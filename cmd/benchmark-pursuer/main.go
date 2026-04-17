// Command benchmark-pursuer runs head-to-head episodes between two Pursuer
// policies (FallbackPolicy = ChaseBehavior, and optionally a loaded
// ONNXPolicy) against a scripted fleeing player on real dumped topologies.
// It reports turns-to-first-hit, hit-rate, kill-rate, and in-cone-ratio so
// PR 5 can quantify whether a trained model meaningfully improves on the
// hand-coded chase baseline.
//
// Usage:
//
//	go run ./cmd/benchmark-pursuer \
//	    -topologies rl/fixtures/topologies \
//	    -episodes 50 \
//	    -max-steps 200 \
//	    -seed 42 \
//	    -onnx rl/models/pursuer.onnx   # optional
//
// Without -onnx, only the baseline (FallbackPolicy) runs — useful when the
// trained model doesn't exist yet; the tiny.onnx testdata works too.
package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"math/rand"
	"os"
	"path/filepath"
	"sort"

	"github.com/unclestep/Rogue/internal/domain/model"
	"github.com/unclestep/Rogue/internal/domain/service"
	"github.com/unclestep/Rogue/internal/dto"
	"github.com/unclestep/Rogue/pkg/geometry"
)

const (
	playerID  = model.ActorId(1)
	pursuerID = model.ActorId(1000)

	attackDamage = 2
)

type metrics struct {
	policy      string
	episodes    int
	catches     int // pursuer reduced player HP to 0
	totalHits   int
	ambushHits  int
	inConeSteps int
	totalSteps  int
	firstHitSum int
	firstHitEps int
}

func (m metrics) print() {
	hitRate := float64(m.totalHits) / float64(maxInt(m.totalSteps, 1))
	ambushRatio := float64(m.ambushHits) / float64(maxInt(m.totalHits, 1))
	inConeRatio := float64(m.inConeSteps) / float64(maxInt(m.totalSteps, 1))
	firstHit := -1.0
	if m.firstHitEps > 0 {
		firstHit = float64(m.firstHitSum) / float64(m.firstHitEps)
	}
	catchRate := float64(m.catches) / float64(maxInt(m.episodes, 1))
	fmt.Printf(
		"%-24s  catch=%5.1f%%  hit_rate=%5.1f%%  ambush=%5.1f%%  in_cone=%5.1f%%  1st_hit=%6.1f\n",
		m.policy,
		catchRate*100, hitRate*100, ambushRatio*100, inConeRatio*100, firstHit,
	)
}

func main() {
	topoDir := flag.String("topologies", "rl/fixtures/topologies", "directory with topology JSONs")
	episodes := flag.Int("episodes", 30, "episodes per policy")
	maxSteps := flag.Int("max-steps", 200, "max turns per episode")
	seed := flag.Int64("seed", 42, "RNG seed (same seed → same trajectories across policies)")
	onnxPath := flag.String("onnx", "", "optional ONNX model path for the RL policy")
	flag.Parse()

	topos, err := loadTopologies(*topoDir)
	if err != nil {
		die("loading topologies: %v", err)
	}
	if len(topos) == 0 {
		die("no topologies found under %q", *topoDir)
	}
	fmt.Fprintf(os.Stderr, "loaded %d topologies from %s\n", len(topos), *topoDir)

	pathfinder := service.NewPathfinderService()
	raycaster := service.NewRaycaster()

	runs := []struct {
		name   string
		policy service.Policy
	}{
		{"fallback (chase-scent)", service.NewFallbackPolicy(pathfinder)},
	}
	if *onnxPath != "" {
		p, err := service.NewONNXPolicy(*onnxPath)
		if err != nil {
			fmt.Fprintf(os.Stderr, "WARN: ONNX load failed (%v) — skipping RL policy\n", err)
		} else {
			runs = append(runs, struct {
				name   string
				policy service.Policy
			}{"onnx (" + filepath.Base(*onnxPath) + ")", p})
		}
	}

	for _, r := range runs {
		m := runPolicy(r.name, r.policy, topos, raycaster, *episodes, *maxSteps, *seed)
		m.print()
	}
}

func runPolicy(
	name string,
	policy service.Policy,
	topos []*dto.TopologyDump,
	rc *service.Raycaster,
	episodes, maxSteps int,
	seed int64,
) metrics {
	m := metrics{policy: name, episodes: episodes}

	rng := rand.New(rand.NewSource(seed))

	for ep := 0; ep < episodes; ep++ {
		topo := topos[rng.Intn(len(topos))]
		em := runEpisode(policy, topo, rc, maxSteps, rng.Int63())
		m.catches += boolToInt(em.caught)
		m.totalHits += em.hits
		m.ambushHits += em.ambush
		m.inConeSteps += em.inCone
		m.totalSteps += em.steps
		if em.firstHit > 0 {
			m.firstHitSum += em.firstHit
			m.firstHitEps++
		}
	}
	return m
}

type episodeMetrics struct {
	hits     int
	ambush   int
	inCone   int
	steps    int
	firstHit int // 1-indexed; 0 means no hit
	caught   bool
}

func runEpisode(
	policy service.Policy,
	topo *dto.TopologyDump,
	rc *service.Raycaster,
	maxSteps int,
	episodeSeed int64,
) episodeMetrics {
	mp := dto.FromTopologyDump(*topo)
	rng := rand.New(rand.NewSource(episodeSeed))

	playerPos, pursuerPos, ok := pickSpawnPoints(mp, topo, rng)
	if !ok {
		return episodeMetrics{}
	}

	player := model.NewDefaultPlayer(playerID, playerPos, 8)
	pursuer := model.NewDefaultPursuer(pursuerID, pursuerPos)

	play := &model.Playthrough{
		Seed:            episodeSeed,
		Map:             mp,
		Players:         map[model.ActorId]*model.Actor{playerID: player},
		Monsters:        map[model.ActorId]*model.Actor{pursuerID: pursuer},
		PlayersFoW:      make(map[model.ActorId]*model.VisibleArea),
		PlayerAimAngles: map[model.ActorId]float64{playerID: rng.Float64()*6.28 - 3.14},
	}
	mp.SetActor(player.Pos, int64(playerID))
	mp.SetActor(pursuer.Pos, int64(pursuerID))

	behavior := service.NewPursuerBehavior(rc, policy)
	maxHP := player.DerivedAttrs[model.AttrMaxHP]

	var em episodeMetrics
	wasVisible := false
	for step := 1; step <= maxSteps; step++ {
		em.steps++

		// Player FoW uses this step's player position/angle.
		rc.RefreshPlayerFoW(play, playerID, service.FlashlightHalfFOV, service.FlashlightRange)

		// Pursuer decides and moves.
		ctx := model.NewSessionContext(play)
		_, decision := behavior.Update(ctx, pursuer)
		if decision != nil && (decision.MoveVector.X != 0 || decision.MoveVector.Y != 0) {
			next := pursuer.Pos.Add(decision.MoveVector)
			if mp.IsWalkable(next) && !isOccupiedBy(mp, next, playerID) {
				mp.SetActor(pursuer.Pos, 0)
				pursuer.Pos = next
				mp.SetActor(pursuer.Pos, int64(pursuerID))
			}
		}

		// Adjacency = hit.
		if manhattan(pursuer.Pos, player.Pos) == 1 {
			em.hits++
			if !wasVisible {
				em.ambush++
			}
			if em.firstHit == 0 {
				em.firstHit = step
			}
			player.Vitals[model.VitalHP] -= attackDamage
			if player.Vitals[model.VitalHP] <= 0 {
				em.caught = true
				break
			}
		}

		// Scripted player flees / wanders; rotates aim.
		stepScriptedPlayer(mp, play, player, pursuer, rng)

		// Measure in-cone after player & pursuer both settled this turn.
		rc.RefreshPlayerFoW(play, playerID, service.FlashlightHalfFOV, service.FlashlightRange)
		wasVisible = isInFoW(play, playerID, pursuer.Pos)
		if wasVisible {
			em.inCone++
		}

		if player.Vitals[model.VitalHP] <= 0 {
			em.caught = true
			break
		}
		if maxHP > 0 && player.Vitals[model.VitalHP] > maxHP {
			player.Vitals[model.VitalHP] = maxHP
		}
	}
	return em
}

func stepScriptedPlayer(
	mp *model.Map,
	play *model.Playthrough,
	player, pursuer *model.Actor,
	rng *rand.Rand,
) {
	// Flee if adjacent (50/50), otherwise random cardinal walk / wait.
	adjacent := manhattan(player.Pos, pursuer.Pos) == 1
	dirs := []geometry.Point{
		{X: 0, Y: -1}, {X: 1, Y: 0}, {X: 0, Y: 1}, {X: -1, Y: 0},
	}
	var move geometry.Point
	switch {
	case adjacent && rng.Float64() < 0.5:
		// Flee along dominant axis of (player - pursuer).
		dx, dy := player.Pos.X-pursuer.Pos.X, player.Pos.Y-pursuer.Pos.Y
		if absInt(dx) >= absInt(dy) {
			if dx >= 0 {
				move = geometry.Point{X: 1}
			} else {
				move = geometry.Point{X: -1}
			}
		} else {
			if dy >= 0 {
				move = geometry.Point{Y: 1}
			} else {
				move = geometry.Point{Y: -1}
			}
		}
	case rng.Float64() < 0.2:
		// Wait.
		move = geometry.Point{}
	default:
		move = dirs[rng.Intn(len(dirs))]
	}
	next := player.Pos.Add(move)
	if (move != geometry.Point{}) && mp.IsWalkable(next) && !isOccupiedBy(mp, next, pursuerID) {
		mp.SetActor(player.Pos, 0)
		player.Pos = next
		mp.SetActor(player.Pos, int64(playerID))
	}
	play.PlayerAimAngles[playerID] += rng.Float64()*1.2 - 0.6
}

// pickSpawnPoints picks a player cell and a pursuer cell both in the same
// room, 2–4 cells apart. Same-room keeps them inside ChaseBehavior's
// hostility radius (1.5 × Hostility = 6 cells for a Pursuer), otherwise the
// FallbackPolicy transitions out to Wander and the benchmark runs a silent
// no-chase scenario. PR 5 will add a multi-room curriculum once state
// transitions are wired through MonsterController.Tick here too.
func pickSpawnPoints(mp *model.Map, topo *dto.TopologyDump, rng *rand.Rand) (geometry.Point, geometry.Point, bool) {
	rooms := topo.Rooms
	if len(rooms) == 0 {
		return geometry.Point{}, geometry.Point{}, false
	}
	// Prefer rooms with enough walkable cells for 2 distinct spawns.
	perm := rng.Perm(len(rooms))
	for _, idx := range perm {
		candidates := roomWalkables(mp, rooms[idx])
		if len(candidates) < 2 {
			continue
		}
		rng.Shuffle(len(candidates), func(i, j int) { candidates[i], candidates[j] = candidates[j], candidates[i] })
		for i := 0; i < len(candidates); i++ {
			for j := i + 1; j < len(candidates); j++ {
				d := manhattan(candidates[i], candidates[j])
				if d >= 2 && d <= 5 {
					return candidates[i], candidates[j], true
				}
			}
		}
		// Fallback: just take the first two if no 2–5 pair exists.
		return candidates[0], candidates[1], true
	}
	return geometry.Point{}, geometry.Point{}, false
}

func roomWalkables(mp *model.Map, r dto.TopologyRoom) []geometry.Point {
	out := make([]geometry.Point, 0, r.Width*r.Height)
	for y := r.Y; y < r.Y+r.Height; y++ {
		for x := r.X; x < r.X+r.Width; x++ {
			p := geometry.Point{X: x, Y: y}
			if mp.IsWalkable(p) {
				out = append(out, p)
			}
		}
	}
	return out
}

func isOccupiedBy(mp *model.Map, p geometry.Point, by model.ActorId) bool {
	id, _ := mp.GetActorID(p)
	return model.ActorId(id) == by
}

func isInFoW(play *model.Playthrough, id model.ActorId, p geometry.Point) bool {
	area := play.PlayersFoW[id]
	if area == nil {
		return false
	}
	if p.Y < 0 || p.Y >= len(area.Area) || p.X < 0 || p.X >= len(area.Area[0]) {
		return false
	}
	return area.Area[p.Y][p.X] == model.Visible
}

func manhattan(a, b geometry.Point) int {
	return absInt(a.X-b.X) + absInt(a.Y-b.Y)
}

func absInt(x int) int {
	if x < 0 {
		return -x
	}
	return x
}

func maxInt(a, b int) int {
	if a > b {
		return a
	}
	return b
}

func boolToInt(b bool) int {
	if b {
		return 1
	}
	return 0
}

func loadTopologies(dir string) ([]*dto.TopologyDump, error) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil, err
	}
	paths := make([]string, 0, len(entries))
	for _, e := range entries {
		if !e.IsDir() && filepath.Ext(e.Name()) == ".json" {
			paths = append(paths, filepath.Join(dir, e.Name()))
		}
	}
	sort.Strings(paths)
	out := make([]*dto.TopologyDump, 0, len(paths))
	for _, p := range paths {
		raw, err := os.ReadFile(p)
		if err != nil {
			return nil, fmt.Errorf("read %s: %w", p, err)
		}
		d := &dto.TopologyDump{}
		if err := json.Unmarshal(raw, d); err != nil {
			return nil, fmt.Errorf("decode %s: %w", p, err)
		}
		out = append(out, d)
	}
	return out, nil
}

func die(format string, args ...any) {
	fmt.Fprintf(os.Stderr, format+"\n", args...)
	os.Exit(1)
}
