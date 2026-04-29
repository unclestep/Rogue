package service

import (
	"container/heap"
	"math"

	"github.com/unclestep/Rogue/internal/domain/model"
	"github.com/unclestep/Rogue/pkg/geometry"
)

// Observation layout.
//
// The vector is laid out as [channels... | scalars]:
//
//	channels: ObservationChannels × ObservationCropSize × ObservationCropSize
//	          flattened in CHW row-major order
//	          (i.e. obs[c*H*W + y*W + x])
//	scalars : ObservationScalars floats appended after the channel block
//
// Keep this layout in lock-step with the Python Gymnasium env
// (rl/pursuer_env.py). Any reordering or resizing breaks parity.
const (
	ObservationCropSize   = 11
	ObservationHalfCrop   = ObservationCropSize / 2
	ObservationChannels   = 14 // +ChInterceptPath, +ChVisitedCorridorPath, +ChFuturePlayerPos
	ObservationScalars    = 20 // +Map_Position_X/Y, +Distance_polar, +Angle_polar
	ObservationGridFloats = ObservationChannels * ObservationCropSize * ObservationCropSize // 14*121=1694
	ObservationSize       = ObservationGridFloats + ObservationScalars                      // 1714

	ChWalkable            = 0
	ChClosedDoor          = 1
	ChOtherMonster        = 2
	ChSelfTrail           = 3
	ChPlayerVisNow        = 4
	ChPlayerMemory        = 5
	ChPlayerCone          = 6
	ChScent               = 7  // Dijkstra chase-scent gradient: hot near player, fades with distance
	ChAgeLastSeen         = 8  // Gaussian-spread decay around last-known player position
	ChCovertPath          = 9  // cone-weighted Dijkstra from pursuer — stealth-aware gradient
	ChPlayerTrail         = 10 // recent player positions, exponential decay per turn
	ChInterceptPath       = 11 // BFS from optimal intercept cell — gradient toward ambush point
	ChVisitedCorridorPath = 12 // weighted BFS, visited-corridor cells discounted
	ChFuturePlayerPos     = 13 // predicted player trajectory (greedy descent), decayed per step

	// ScentNormFactor is the BFS-distance scale for ChScent normalisation.
	ScentNormFactor = 30.0
	// AgeLastSeenSigma is the Gaussian spread (cells) for ChAgeLastSeen.
	AgeLastSeenSigma = 2.5
	// CovertConePenalty is the step cost of entering a cone cell in the
	// weighted-BFS that produces ChCovertPath.
	CovertConePenalty = 10
	// CovertNormFactor is the distance scale for ChCovertPath normalisation.
	CovertNormFactor = 50.0
	// PlayerTrailCapacity bounds the player-trail ring buffer.
	PlayerTrailCapacity = 30
	// PlayerTrailDecay is the per-turn intensity multiplier for older entries.
	PlayerTrailDecay = 0.9
	// InterceptNormFactor is the distance scale for ChInterceptPath normalisation.
	InterceptNormFactor = 20.0
	// VisitedCorridorDiscount is the step cost for walkable cells that are NOT
	// in the player's visited-corridor set. Visited corridor cells cost 1.
	// Ratio is ~3× so pathing prefers known corridors.
	VisitedCorridorDiscount = 3
	// VisitedCorridorNormFactor is the distance scale for ChVisitedCorridorPath.
	VisitedCorridorNormFactor = 60.0
	// FuturePosHorizon controls how far ahead predictPlayerPath looks.
	FuturePosHorizon = 5
	// FuturePosDecay is the per-step intensity multiplier for future positions.
	FuturePosDecay = 0.8

	// PursuerTrailCapacity bounds the self-trail channel window.
	// Oldest entry has weight 0, newest has weight 1.
	PursuerTrailCapacity = 20

	// PursuerMemoryHorizon bounds the memory decay channel. Past this many
	// turns without LOS the chPlayerMemory channel reads 0.
	PursuerMemoryHorizon = 20
)

// PursuerMemory is per-Pursuer state preserved across turns. The map that
// owns it lives inside PursuerBehavior; PursuerMemory itself is a leaf value
// struct. Exported so the observation layout can be verified from tests and
// from the Python Gymnasium env.
type PursuerMemory struct {
	LastSeen      geometry.Point // NewInvalidPoint when the player has never been spotted
	TurnsSinceLOS int            // Capped at PursuerMemoryHorizon for stability
	Trail         []geometry.Point
	PlayerTrail   []geometry.Point // Ring buffer of recent player positions, newest last

	// Director/Alien two-tier state (not part of the ONNX observation).
	FrustrationCount int  // turns in RL mode without LOS or hot scent
	RLActive         bool // true = ONNX policy controls; false = macro/Director
}

func NewPursuerMemory() *PursuerMemory {
	return &PursuerMemory{
		LastSeen:      model.NewInvalidPoint(),
		TurnsSinceLOS: PursuerMemoryHorizon,
		Trail:         make([]geometry.Point, 0, PursuerTrailCapacity),
		PlayerTrail:   make([]geometry.Point, 0, PlayerTrailCapacity),
	}
}

// BuildObservation assembles a flat float32 tensor for the Pursuer policy.
// All spatial features are relative to the Pursuer's current position. Scalars
// are normalised into [-1, 1] where reasonable. The tensor always has length
// ObservationSize; cells outside the map are left at 0 (treated as walls).
// Pure function — all state is read from ctx and mem; nothing is mutated.
func BuildObservation(ctx *model.SessionContext, actor *model.Actor, mem *PursuerMemory) []float32 {
	obs := make([]float32, ObservationSize)
	play := ctx.Playthrough
	m := play.Map
	if m == nil {
		return obs
	}

	player, playerAngle, haveTarget := nearestAlivePlayer(play, actor.Pos)

	playerCone := playerConeArea(play, player, haveTarget)
	seesPlayer := haveTarget && hasLOS(m, actor.Pos, player.Pos)

	writeChannels(obs, m, actor, mem, play, player, playerCone, haveTarget, seesPlayer)
	writeChScent(obs, ctx.ScentMaps[model.ScentMapChase], actor.Pos, m)
	writeChAgeLastSeen(obs, mem, actor.Pos)
	writeChCovertPath(obs, m, actor.Pos, playerCone)
	writeChPlayerTrail(obs, mem, actor.Pos)
	writeChInterceptPath(obs, m, ctx, actor.Pos, player, haveTarget)
	writeChVisitedCorridorPath(obs, m, actor.Pos, mem)
	writeChFuturePlayerPos(obs, m, ctx, actor.Pos, player, haveTarget)
	writeScalars(obs, ctx, m, actor, mem, player, playerAngle, haveTarget, seesPlayer, play)
	return obs
}

func writeChannels(
	obs []float32,
	m *model.Map,
	actor *model.Actor,
	mem *PursuerMemory,
	play *model.Playthrough,
	player *model.Actor,
	playerCone *model.VisibleArea,
	haveTarget bool,
	seesPlayer bool,
) {
	origin := actor.Pos
	for dy := -ObservationHalfCrop; dy <= ObservationHalfCrop; dy++ {
		for dx := -ObservationHalfCrop; dx <= ObservationHalfCrop; dx++ {
			p := geometry.Point{X: origin.X + dx, Y: origin.Y + dy}
			x := dx + ObservationHalfCrop
			y := dy + ObservationHalfCrop

			if !m.InBounds(p) {
				continue
			}

			if m.IsWalkable(p) {
				setChannel(obs, ChWalkable, x, y, 1.0)
			}
			if m.IsClosedDoor(p) {
				setChannel(obs, ChClosedDoor, x, y, 1.0)
			}
			if id, _ := m.GetActorID(p); id != 0 && model.ActorId(id) != actor.Id {
				if _, isMonster := play.Monsters[model.ActorId(id)]; isMonster {
					setChannel(obs, ChOtherMonster, x, y, 1.0)
				}
			}
			if haveTarget && p == player.Pos && seesPlayer {
				setChannel(obs, ChPlayerVisNow, x, y, 1.0)
			}
			if playerCone != nil && playerCone.Area[p.Y][p.X] == model.Visible {
				setChannel(obs, ChPlayerCone, x, y, 1.0)
			}
		}
	}

	// ChSelfTrail — fresh positions dominate; oldest has weight ~1/cap.
	for i, tp := range mem.Trail {
		age := len(mem.Trail) - 1 - i
		weight := 1.0 - float32(age)/float32(PursuerTrailCapacity)
		if weight <= 0 {
			continue
		}
		dx := tp.X - origin.X
		dy := tp.Y - origin.Y
		if dx < -ObservationHalfCrop || dx > ObservationHalfCrop ||
			dy < -ObservationHalfCrop || dy > ObservationHalfCrop {
			continue
		}
		setChannel(obs, ChSelfTrail, dx+ObservationHalfCrop, dy+ObservationHalfCrop, weight)
	}

	// ChPlayerMemory — single spike at LastSeen, amplitude decays with TurnsSinceLOS.
	if mem.LastSeen != model.NewInvalidPoint() {
		dx := mem.LastSeen.X - origin.X
		dy := mem.LastSeen.Y - origin.Y
		if dx >= -ObservationHalfCrop && dx <= ObservationHalfCrop &&
			dy >= -ObservationHalfCrop && dy <= ObservationHalfCrop {
			weight := 1.0 - float32(mem.TurnsSinceLOS)/float32(PursuerMemoryHorizon)
			if weight > 0 {
				setChannel(obs, ChPlayerMemory, dx+ObservationHalfCrop, dy+ObservationHalfCrop, weight)
			}
		}
	}
}

// writeChScent writes the normalised Dijkstra chase-scent gradient into ChScent.
// Hot (≈1.0) near the player, fading to 0 at ScentNormFactor BFS steps away.
// Unreachable cells are left at 0.
func writeChScent(obs []float32, scentMap *model.ScentMap, origin geometry.Point, m *model.Map) {
	if scentMap == nil {
		return
	}
	for dy := -ObservationHalfCrop; dy <= ObservationHalfCrop; dy++ {
		for dx := -ObservationHalfCrop; dx <= ObservationHalfCrop; dx++ {
			p := geometry.Point{X: origin.X + dx, Y: origin.Y + dy}
			if !m.InBounds(p) {
				continue
			}
			dist := scentMap.Scent[p.Y][p.X]
			if dist == math.MaxInt {
				continue // unreachable cell
			}
			v := 1.0 - float32(dist)/ScentNormFactor
			if v <= 0 {
				continue
			}
			setChannel(obs, ChScent, dx+ObservationHalfCrop, dy+ObservationHalfCrop, v)
		}
	}
}

// writeChAgeLastSeen writes a Gaussian-spread decay centred on the last known
// player position into ChAgeLastSeen. Amplitude decays linearly with
// TurnsSinceLOS; spatial spread uses σ=AgeLastSeenSigma cells. This gives the
// network a directional gradient even when the player is out of the 11×11 crop.
func writeChAgeLastSeen(obs []float32, mem *PursuerMemory, origin geometry.Point) {
	if mem.LastSeen == model.NewInvalidPoint() {
		return
	}
	amplitude := 1.0 - float32(mem.TurnsSinceLOS)/float32(PursuerMemoryHorizon)
	if amplitude <= 0 {
		return
	}
	const sigma = AgeLastSeenSigma
	const twoSigmaSq = 2.0 * sigma * sigma
	for dy := -ObservationHalfCrop; dy <= ObservationHalfCrop; dy++ {
		for dx := -ObservationHalfCrop; dx <= ObservationHalfCrop; dx++ {
			wx := origin.X + dx
			wy := origin.Y + dy
			ddx := float64(wx - mem.LastSeen.X)
			ddy := float64(wy - mem.LastSeen.Y)
			d2 := ddx*ddx + ddy*ddy
			v := amplitude * float32(math.Exp(-d2/twoSigmaSq))
			if v < 0.01 {
				continue
			}
			setChannel(obs, ChAgeLastSeen, dx+ObservationHalfCrop, dy+ObservationHalfCrop, v)
		}
	}
}

// writeChCovertPath writes the normalised cone-weighted Dijkstra distance from
// the pursuer into ChCovertPath. Cells inside the player's cone cost
// CovertConePenalty to enter; free cells cost 1. Normalised by
// CovertNormFactor and clipped to [0, 1]. Unreachable cells (incl. cells
// outside the map) read 1.0. Gives the policy a stealth-aware gradient that
// the policy can use to plan approaches that avoid the cone when any
// alternative exists.
func writeChCovertPath(
	obs []float32,
	m *model.Map,
	origin geometry.Point,
	playerCone *model.VisibleArea,
) {
	dist := weightedBFSFromPursuer(m, origin, playerCone, CovertConePenalty)
	for dy := -ObservationHalfCrop; dy <= ObservationHalfCrop; dy++ {
		for dx := -ObservationHalfCrop; dx <= ObservationHalfCrop; dx++ {
			p := geometry.Point{X: origin.X + dx, Y: origin.Y + dy}
			if !m.InBounds(p) {
				continue
			}
			d := dist[p.Y][p.X]
			cx := dx + ObservationHalfCrop
			cy := dy + ObservationHalfCrop
			if d == math.MaxInt {
				setChannel(obs, ChCovertPath, cx, cy, 1.0)
				continue
			}
			v := float32(d) / float32(CovertNormFactor)
			if v > 1.0 {
				v = 1.0
			}
			setChannel(obs, ChCovertPath, cx, cy, v)
		}
	}
}

// writeChPlayerTrail lays the recent player positions into ChPlayerTrail,
// with the most recent entry at intensity 1.0 and each older entry multiplied
// by PlayerTrailDecay. Older entries overwrite only if they dominate (max
// rather than sum) so a visited cell keeps the strongest observation. Mirrors
// build_observation's handling in Python.
func writeChPlayerTrail(obs []float32, mem *PursuerMemory, origin geometry.Point) {
	if len(mem.PlayerTrail) == 0 {
		return
	}
	n := len(mem.PlayerTrail)
	start := 0
	if n > PlayerTrailCapacity {
		start = n - PlayerTrailCapacity
	}
	window := mem.PlayerTrail[start:]
	for i := len(window) - 1; i >= 0; i-- {
		age := len(window) - 1 - i
		intensity := float32(math.Pow(PlayerTrailDecay, float64(age)))
		if intensity < 1e-3 {
			break
		}
		dx := window[i].X - origin.X
		dy := window[i].Y - origin.Y
		cx := dx + ObservationHalfCrop
		cy := dy + ObservationHalfCrop
		if cx < 0 || cx >= ObservationCropSize || cy < 0 || cy >= ObservationCropSize {
			continue
		}
		cur := obs[ChPlayerTrail*ObservationCropSize*ObservationCropSize+cy*ObservationCropSize+cx]
		if intensity > cur {
			setChannel(obs, ChPlayerTrail, cx, cy, intensity)
		}
	}
}

// weightedBFSFromPursuer runs a 4-cardinal Dijkstra from origin where cells
// inside `cone` cost conePenalty to enter and other walkable cells cost 1.
// Returns a distance grid sized (m.Height × m.Width). Unreachable cells are
// math.MaxInt. Out-of-bounds origin returns an all-MaxInt grid.
func weightedBFSFromPursuer(
	m *model.Map,
	origin geometry.Point,
	cone *model.VisibleArea,
	conePenalty int,
) [][]int {
	dist := make([][]int, m.Height)
	for y := range dist {
		row := make([]int, m.Width)
		for x := range row {
			row[x] = math.MaxInt
		}
		dist[y] = row
	}
	if !m.InBounds(origin) {
		return dist
	}
	dist[origin.Y][origin.X] = 0
	pq := &weightedBFSQueue{}
	heap.Init(pq)
	heap.Push(pq, weightedBFSNode{d: 0, x: origin.X, y: origin.Y})
	dirs := [4][2]int{{0, -1}, {1, 0}, {0, 1}, {-1, 0}}
	for pq.Len() > 0 {
		cur := heap.Pop(pq).(weightedBFSNode)
		if cur.d > dist[cur.y][cur.x] {
			continue
		}
		for _, d := range dirs {
			nx := cur.x + d[0]
			ny := cur.y + d[1]
			np := geometry.Point{X: nx, Y: ny}
			if !m.IsWalkable(np) {
				continue
			}
			cost := 1
			if cone != nil && cone.Area[ny][nx] == model.Visible {
				cost = conePenalty
			}
			nd := cur.d + cost
			if nd < dist[ny][nx] {
				dist[ny][nx] = nd
				heap.Push(pq, weightedBFSNode{d: nd, x: nx, y: ny})
			}
		}
	}
	return dist
}

type weightedBFSNode struct {
	d int
	x int
	y int
}

type weightedBFSQueue []weightedBFSNode

func (q weightedBFSQueue) Len() int           { return len(q) }
func (q weightedBFSQueue) Less(i, j int) bool { return q[i].d < q[j].d }
func (q weightedBFSQueue) Swap(i, j int)      { q[i], q[j] = q[j], q[i] }
func (q *weightedBFSQueue) Push(v any)        { *q = append(*q, v.(weightedBFSNode)) }
func (q *weightedBFSQueue) Pop() any {
	old := *q
	n := len(old)
	v := old[n-1]
	*q = old[:n-1]
	return v
}

// computeInterceptStats mirrors PursuerEnv._intercept_stats in Python. Returns
// (player_to_exit_norm, pursuer_to_player_norm, intercept_advantage_signed).
// Falls back to zeros when scent maps are missing.
// findOptimalInterceptCell locates the cell minimising max(pursuer_time,
// player_arrival_time) among cells on the player→exit route. Returns (-1,-1)
// for the cell if no valid intercept exists. Also returns pursuer_to_intercept
// and max_dist used for normalisation. Mirrors _find_optimal_intercept_cell
// in pursuer_env.py byte-for-byte.
func findOptimalInterceptCell(
	exitScent *model.ScentMap,
	chaseScent *model.ScentMap,
	player geometry.Point,
) (geometry.Point, int, int) {
	none := geometry.Point{X: -1, Y: -1}
	if exitScent == nil || chaseScent == nil {
		return none, 0, 1
	}
	exit := exitScent.Scent
	chase := chaseScent.Scent
	if len(exit) == 0 || len(chase) == 0 {
		return none, 0, 1
	}
	if player.Y < 0 || player.Y >= len(exit) || player.X < 0 || player.X >= len(exit[0]) {
		return none, 0, 1
	}

	maxDist := 1
	for y := range exit {
		for x := range exit[y] {
			v := exit[y][x]
			if v != math.MaxInt && v > maxDist {
				maxDist = v
			}
		}
	}

	playerToExit := exit[player.Y][player.X]
	if playerToExit == math.MaxInt {
		playerToExit = maxDist
	}

	bestCell := none
	minMeet := math.MaxInt
	for y := range exit {
		for x := range exit[y] {
			ev := exit[y][x]
			cv := chase[y][x]
			if ev == math.MaxInt || cv == math.MaxInt {
				continue
			}
			if ev > playerToExit {
				continue
			}
			playerTime := max(playerToExit-ev, 0)
			meet := max(cv, playerTime)
			if meet < minMeet {
				minMeet = meet
				bestCell = geometry.Point{X: x, Y: y}
			}
		}
	}
	if bestCell == none {
		return none, maxDist, maxDist
	}
	return bestCell, minMeet, maxDist
}

func computeInterceptStats(
	exitScent *model.ScentMap,
	chaseScent *model.ScentMap,
	pursuer geometry.Point,
	player geometry.Point,
) (float32, float32, float32) {
	if exitScent == nil || chaseScent == nil {
		return 0, 0, 0
	}
	exit := exitScent.Scent
	chase := chaseScent.Scent
	if len(exit) == 0 || len(chase) == 0 {
		return 0, 0, 0
	}
	if player.Y < 0 || player.Y >= len(exit) || player.X < 0 || player.X >= len(exit[0]) {
		return 0, 0, 0
	}
	if pursuer.Y < 0 || pursuer.Y >= len(chase) || pursuer.X < 0 || pursuer.X >= len(chase[0]) {
		return 0, 0, 0
	}

	cell, pursuerToIntercept, maxDist := findOptimalInterceptCell(exitScent, chaseScent, player)

	playerToExit := exit[player.Y][player.X]
	if playerToExit == math.MaxInt {
		playerToExit = maxDist
	}
	pursuerToPlayer := chase[pursuer.Y][pursuer.X]
	if pursuerToPlayer == math.MaxInt {
		pursuerToPlayer = maxDist
	}
	none := geometry.Point{X: -1, Y: -1}
	if cell == none {
		pursuerToIntercept = pursuerToPlayer
	}

	p2e := float32(playerToExit) / float32(maxDist)
	p2p := float32(pursuerToPlayer) / float32(maxDist)
	if p2p > 1 {
		p2p = 1
	}
	adv := float32(playerToExit-pursuerToIntercept) / float32(maxDist)
	if adv > 1 {
		adv = 1
	}
	if adv < -1 {
		adv = -1
	}
	return p2e, p2p, adv
}

// bfsFromCell runs 8-connected BFS from origin over walkable cells (diagonals
// count as 1 step, Chebyshev-distance style) and returns the distance grid
// (math.MaxInt for unreachable). Matches Python chase_scent_map byte-for-byte
// — both use the same 8-dir set to guarantee parity of the intercept channel.
func bfsFromCell(m *model.Map, origin geometry.Point) [][]int {
	dist := make([][]int, m.Height)
	for y := range dist {
		row := make([]int, m.Width)
		for x := range row {
			row[x] = math.MaxInt
		}
		dist[y] = row
	}
	if !m.InBounds(origin) {
		return dist
	}
	dist[origin.Y][origin.X] = 0
	queue := []geometry.Point{origin}
	dirs := [8][2]int{
		{0, -1}, {1, 0}, {0, 1}, {-1, 0},
		{1, -1}, {1, 1}, {-1, 1}, {-1, -1},
	}
	for head := 0; head < len(queue); head++ {
		cur := queue[head]
		for _, d := range dirs {
			nx, ny := cur.X+d[0], cur.Y+d[1]
			np := geometry.Point{X: nx, Y: ny}
			if !m.IsWalkable(np) {
				continue
			}
			if dist[ny][nx] > dist[cur.Y][cur.X]+1 {
				dist[ny][nx] = dist[cur.Y][cur.X] + 1
				queue = append(queue, np)
			}
		}
	}
	return dist
}

// predictPlayerPath mirrors Python predict_player_path: greedy descent of
// exit_scent from player for `horizon` steps. Returns [player, step1, ..., step_h].
func predictPlayerPath(
	m *model.Map,
	exitScent *model.ScentMap,
	player geometry.Point,
	horizon int,
) []geometry.Point {
	path := []geometry.Point{player}
	if exitScent == nil || !m.InBounds(player) {
		return path
	}
	scent := exitScent.Scent
	cur := player
	dirs := [4][2]int{{0, -1}, {1, 0}, {0, 1}, {-1, 0}}
	for range horizon {
		if !m.InBounds(cur) {
			break
		}
		bestVal := scent[cur.Y][cur.X]
		if bestVal == math.MaxInt {
			break
		}
		bestNext := geometry.Point{X: -1, Y: -1}
		for _, d := range dirs {
			nx, ny := cur.X+d[0], cur.Y+d[1]
			np := geometry.Point{X: nx, Y: ny}
			if !m.IsWalkable(np) {
				continue
			}
			v := scent[ny][nx]
			if v == math.MaxInt {
				continue
			}
			if v < bestVal {
				bestVal = v
				bestNext = np
			}
		}
		if bestNext == (geometry.Point{X: -1, Y: -1}) {
			break
		}
		path = append(path, bestNext)
		cur = bestNext
	}
	return path
}

func writeChInterceptPath(
	obs []float32,
	m *model.Map,
	ctx *model.SessionContext,
	origin geometry.Point,
	player *model.Actor,
	haveTarget bool,
) {
	// No target → channel stays at 0 (parity with Python, which passes
	// intercept_map=None when player_pos is None).
	if !haveTarget {
		return
	}
	exit := ctx.ExitScentMap()
	chase := ctx.ScentMaps[model.ScentMapChase]
	cell, _, _ := findOptimalInterceptCell(exit, chase, player.Pos)
	if cell == (geometry.Point{X: -1, Y: -1}) {
		for dy := -ObservationHalfCrop; dy <= ObservationHalfCrop; dy++ {
			for dx := -ObservationHalfCrop; dx <= ObservationHalfCrop; dx++ {
				cx := dx + ObservationHalfCrop
				cy := dy + ObservationHalfCrop
				setChannel(obs, ChInterceptPath, cx, cy, 1.0)
			}
		}
		return
	}
	dist := bfsFromCell(m, cell)
	for dy := -ObservationHalfCrop; dy <= ObservationHalfCrop; dy++ {
		for dx := -ObservationHalfCrop; dx <= ObservationHalfCrop; dx++ {
			p := geometry.Point{X: origin.X + dx, Y: origin.Y + dy}
			if !m.InBounds(p) {
				continue
			}
			d := dist[p.Y][p.X]
			cx := dx + ObservationHalfCrop
			cy := dy + ObservationHalfCrop
			if d == math.MaxInt {
				setChannel(obs, ChInterceptPath, cx, cy, 1.0)
				continue
			}
			v := float32(d) / float32(InterceptNormFactor)
			if v > 1.0 {
				v = 1.0
			}
			setChannel(obs, ChInterceptPath, cx, cy, v)
		}
	}
}

func writeChVisitedCorridorPath(
	obs []float32,
	m *model.Map,
	origin geometry.Point,
	mem *PursuerMemory,
) {
	// Weighted Dijkstra from pursuer. Corridor cells in PlayerTrail cost 1;
	// other walkable cells cost VisitedCorridorDiscount (=3). Gradient biases
	// routes through player-visited corridors for ambush positioning.
	dist := make([][]int, m.Height)
	for y := range dist {
		row := make([]int, m.Width)
		for x := range row {
			row[x] = math.MaxInt
		}
		dist[y] = row
	}
	if !m.InBounds(origin) {
		return
	}

	visited := make(map[geometry.Point]bool, len(mem.PlayerTrail))
	for _, p := range mem.PlayerTrail {
		if !m.InBounds(p) {
			continue
		}
		tile, _ := m.GetTileType(p)
		if tile == model.Corridor {
			visited[p] = true
		}
	}

	dist[origin.Y][origin.X] = 0
	pq := &weightedBFSQueue{}
	heap.Init(pq)
	heap.Push(pq, weightedBFSNode{d: 0, x: origin.X, y: origin.Y})
	dirs := [4][2]int{{0, -1}, {1, 0}, {0, 1}, {-1, 0}}
	for pq.Len() > 0 {
		cur := heap.Pop(pq).(weightedBFSNode)
		if cur.d > dist[cur.y][cur.x] {
			continue
		}
		for _, d := range dirs {
			nx := cur.x + d[0]
			ny := cur.y + d[1]
			np := geometry.Point{X: nx, Y: ny}
			if !m.IsWalkable(np) {
				continue
			}
			cost := VisitedCorridorDiscount
			if visited[np] {
				cost = 1
			}
			nd := cur.d + cost
			if nd < dist[ny][nx] {
				dist[ny][nx] = nd
				heap.Push(pq, weightedBFSNode{d: nd, x: nx, y: ny})
			}
		}
	}

	for dy := -ObservationHalfCrop; dy <= ObservationHalfCrop; dy++ {
		for dx := -ObservationHalfCrop; dx <= ObservationHalfCrop; dx++ {
			p := geometry.Point{X: origin.X + dx, Y: origin.Y + dy}
			if !m.InBounds(p) {
				continue
			}
			d := dist[p.Y][p.X]
			cx := dx + ObservationHalfCrop
			cy := dy + ObservationHalfCrop
			if d == math.MaxInt {
				setChannel(obs, ChVisitedCorridorPath, cx, cy, 1.0)
				continue
			}
			v := float32(d) / float32(VisitedCorridorNormFactor)
			if v > 1.0 {
				v = 1.0
			}
			setChannel(obs, ChVisitedCorridorPath, cx, cy, v)
		}
	}
}

func writeChFuturePlayerPos(
	obs []float32,
	m *model.Map,
	ctx *model.SessionContext,
	origin geometry.Point,
	player *model.Actor,
	haveTarget bool,
) {
	if !haveTarget {
		return
	}
	path := predictPlayerPath(m, ctx.ExitScentMap(), player.Pos, FuturePosHorizon)
	intensity := float32(1.0)
	for i, p := range path {
		if i > 0 {
			intensity *= float32(FuturePosDecay)
		}
		if intensity < 1e-3 {
			break
		}
		dx := p.X - origin.X
		dy := p.Y - origin.Y
		cx := dx + ObservationHalfCrop
		cy := dy + ObservationHalfCrop
		if cx < 0 || cx >= ObservationCropSize || cy < 0 || cy >= ObservationCropSize {
			continue
		}
		cur := obs[ChFuturePlayerPos*ObservationCropSize*ObservationCropSize+cy*ObservationCropSize+cx]
		if intensity > cur {
			setChannel(obs, ChFuturePlayerPos, cx, cy, intensity)
		}
	}
}

func writeScalars(
	obs []float32,
	ctx *model.SessionContext,
	m *model.Map,
	actor *model.Actor,
	mem *PursuerMemory,
	player *model.Actor,
	playerAngle float64,
	haveTarget bool,
	seesPlayer bool,
	play *model.Playthrough,
) {
	s := obs[ObservationGridFloats:]

	maxHP := float32(actor.DerivedAttrs[model.AttrMaxHP])
	if maxHP > 0 {
		s[0] = float32(actor.Vitals[model.VitalHP]) / maxHP
	}
	maxStamina := float32(actor.DerivedAttrs[model.AttrMaxStamina])
	if maxStamina > 0 {
		s[1] = float32(actor.Vitals[model.VitalStamina]) / maxStamina
	}

	s[2] = float32(mem.TurnsSinceLOS) / float32(PursuerMemoryHorizon)
	if s[2] > 1.0 {
		s[2] = 1.0
	}

	w, h := float32(m.Width), float32(m.Height)
	if mem.LastSeen != model.NewInvalidPoint() {
		dx := mem.LastSeen.X - actor.Pos.X
		dy := mem.LastSeen.Y - actor.Pos.Y
		dist := float32(math.Hypot(float64(dx), float64(dy)))
		s[3] = dist / float32(math.Hypot(float64(w), float64(h)))
		s[4] = float32(dx) / w
		s[5] = float32(dy) / h
	}

	nearest := nearestOtherPursuer(play, actor)
	if nearest != nil {
		s[6] = float32(nearest.Pos.X-actor.Pos.X) / w
		s[7] = float32(nearest.Pos.Y-actor.Pos.Y) / h
	}

	if haveTarget {
		if isInPlayerCone(actor.Pos, player.Pos, playerAngle) {
			s[8] = 1.0
		}
		s[9] = float32(player.Pos.X-actor.Pos.X) / w
		s[10] = float32(player.Pos.Y-actor.Pos.Y) / h

		coneDirX := float32(math.Cos(playerAngle))
		coneDirY := float32(math.Sin(playerAngle))
		dx := float32(actor.Pos.X - player.Pos.X)
		dy := float32(actor.Pos.Y - player.Pos.Y)
		n := float32(math.Hypot(float64(dx), float64(dy)))
		if n > 0 {
			s[11] = (dx*coneDirX + dy*coneDirY) / n
		}
	}

	// Intercept / exit-awareness scalars. Defaults to zero when we cannot
	// compute them (no target, map-not-ready, {-1,-1} ExitPoint, etc.).
	if haveTarget {
		exitScent := ctx.ExitScentMap()
		chaseScent := ctx.ScentMaps[model.ScentMapChase]
		p2e, p2p, adv := computeInterceptStats(exitScent, chaseScent, actor.Pos, player.Pos)
		s[12] = p2e
		s[13] = p2p
		s[14] = adv
		tile, ok := m.GetTileType(player.Pos)
		if ok && tile == model.Corridor {
			s[15] = 1
		}
	}

	// Map_Position_X/Y — global orientation: critical on 80×24 maps because the
	// 11×11 crop hides where the pursuer is on the full dungeon layout.
	if w > 0 {
		s[16] = float32(actor.Pos.X) / w
	}
	if h > 0 {
		s[17] = float32(actor.Pos.Y) / h
	}
	if s[16] < 0 {
		s[16] = 0
	} else if s[16] > 1 {
		s[16] = 1
	}
	if s[17] < 0 {
		s[17] = 0
	} else if s[17] > 1 {
		s[17] = 1
	}

	// Polar distance / angle to player — compresses far-target info into
	// bounded channels. Redundant with Relative_X/Y for near targets, but
	// survives long-distance scenarios where relative coords saturate.
	if haveTarget {
		dx := float64(player.Pos.X - actor.Pos.X)
		dy := float64(player.Pos.Y - actor.Pos.Y)
		diag := math.Hypot(float64(w), float64(h))
		if diag > 0 {
			d := math.Hypot(dx, dy) / diag
			if d > 1 {
				d = 1
			}
			s[18] = float32(d)
		}
		s[19] = float32(math.Atan2(dy, dx) / math.Pi)
	}

	_ = seesPlayer // reserved — seesPlayer already encoded in chPlayerVisNow
}

func setChannel(obs []float32, channel, x, y int, v float32) {
	if x < 0 || x >= ObservationCropSize || y < 0 || y >= ObservationCropSize {
		return
	}
	obs[channel*ObservationCropSize*ObservationCropSize+y*ObservationCropSize+x] = v
}

// ObservationCell reads obs[channel, x, y] from a flattened tensor produced
// by BuildObservation. Out-of-range coordinates return 0 instead of panicking.
// Exported so tests and offline debug tools can inspect the layout without
// re-deriving the stride arithmetic.
func ObservationCell(obs []float32, channel, x, y int) float32 {
	if x < 0 || x >= ObservationCropSize || y < 0 || y >= ObservationCropSize {
		return 0
	}
	return obs[channel*ObservationCropSize*ObservationCropSize+y*ObservationCropSize+x]
}

func nearestAlivePlayer(play *model.Playthrough, from geometry.Point) (*model.Actor, float64, bool) {
	var best *model.Actor
	bestDist := math.Inf(1)
	for _, p := range play.Players {
		if p.Vitals[model.VitalHP] <= 0 || p.Pos == model.NewInvalidPoint() {
			continue
		}
		d := from.EuclideanDistance(p.Pos)
		if d < bestDist {
			bestDist = d
			best = p
		}
	}
	if best == nil {
		return nil, 0, false
	}
	angle := play.PlayerAimAngles[best.Id]
	return best, angle, true
}

func nearestOtherPursuer(play *model.Playthrough, self *model.Actor) *model.Actor {
	var best *model.Actor
	bestDist := math.Inf(1)
	for _, other := range play.Monsters {
		if other.Id == self.Id || other.Kind != model.ActorPursuer {
			continue
		}
		d := self.Pos.EuclideanDistance(other.Pos)
		if d < bestDist {
			bestDist = d
			best = other
		}
	}
	return best
}

// playerConeArea returns the player's current flashlight visibility area if
// one has been computed for this turn, else nil. Callers must treat nil as
// "no cone info" (all cells dark).
func playerConeArea(play *model.Playthrough, player *model.Actor, haveTarget bool) *model.VisibleArea {
	if !haveTarget || play.PlayersFoW == nil {
		return nil
	}
	return play.PlayersFoW[player.Id]
}

func isInPlayerCone(target, playerPos geometry.Point, playerAngle float64) bool {
	dx := float64(target.X - playerPos.X)
	dy := float64(target.Y - playerPos.Y)
	dist := math.Hypot(dx, dy)
	if dist < 1e-9 || dist > FlashlightRange {
		return target == playerPos
	}
	// cos(angle between vectors) = (a·b) / (|a||b|)
	coneDirX := math.Cos(playerAngle)
	coneDirY := math.Sin(playerAngle)
	cosTheta := (dx*coneDirX + dy*coneDirY) / dist
	cosHalfFOV := math.Cos(FlashlightHalfFOV * math.Pi / 180.0)
	return cosTheta >= cosHalfFOV
}

// hasLOS runs a Bresenham traversal between two grid cells and returns true
// iff no light-blocking tile sits strictly between them. Walls, Empty and
// ClosedDoor block; OpenDoors, floors and corridors do not (unlike the
// flashlight, which treats open doors as opaque to give the player a visual
// cue — monsters are not under that constraint).
func hasLOS(m *model.Map, from, to geometry.Point) bool {
	if from == to {
		return true
	}
	dx := abs(to.X - from.X)
	dy := abs(to.Y - from.Y)
	sx := 1
	if from.X > to.X {
		sx = -1
	}
	sy := 1
	if from.Y > to.Y {
		sy = -1
	}
	err := dx - dy
	x, y := from.X, from.Y
	for {
		if x == to.X && y == to.Y {
			return true
		}
		e2 := 2 * err
		if e2 > -dy {
			err -= dy
			x += sx
		}
		if e2 < dx {
			err += dx
			y += sy
		}
		if x == to.X && y == to.Y {
			return true
		}
		p := geometry.Point{X: x, Y: y}
		if !m.InBounds(p) {
			return false
		}
		t, _ := m.GetTileType(p)
		if t == model.Wall || t == model.Empty || t == model.ClosedDoor {
			return false
		}
	}
}
