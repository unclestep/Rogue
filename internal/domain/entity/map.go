// Package entity within:
// - Map stores information about game landscape, actors and items;
package entity

import (
	"log"
	"maps"
	"math"
	"math/rand"
	"slices"
	"strings"

	"github.com/Nikolay-Yakunin/gouge/internal/pkg/algorithm"
	"github.com/Nikolay-Yakunin/gouge/internal/pkg/geometry"
)

// Map - structure for gameboard
type Map struct {
	width, height int            // Cols and rows
	tileGrid      [][]Cell       // Layer 1: Game landscape
	itemGrid      [][]int        // Layer 2: Location of items
	actorGrid     [][]int        // Layer 3: Location of actors
	entrancePoint geometry.Point // Spawn point
	exitPoint     geometry.Point // End of level point
	rooms         []*Room        // Pointers to all level rooms
	entranceRoom  *Room          // Pointer to room with spawn point
	exitRoom      *Room          // Pointer to room with exit point
	rand          *rand.Rand     // Local random generator (for representative tests)
	seed          int64          // Seed of local random generator
	idGen         func() int
}

type NavMap map[*Room][]Edge

type Edge struct {
	src     *Room
	srcDoor geometry.Point
	dst     *Room
	dstDoor geometry.Point
	weight  int
}

// TileType - enumeration of cell tile types
type TileType int

// Tile types
const (
	Empty TileType = iota
	Wall
	Floor
	Door
	Corridor
	Exit
)

// VisibilityState - enumeration of cell visibility states
type VisibilityState int

// Visibility states
const (
	Unexplored VisibilityState = iota
	Explored
	Visible
)

// Cell - structure for cell entity
type Cell struct {
	Type       TileType
	Visibility VisibilityState
}

// Sector - structure for sector entity
type Sector struct {
	xMin, yMin, xMax, yMax int
}

// Map constants
const (
	MarginBetweenSectors  = 2 // Right margin of one room + left margin of second room
	PrimaryPoolMultiplier = 0.5
)

// Room - structure for room entity
type Room struct {
	id                    int
	pos                   geometry.Point // Left-upper corner
	width, height         int
	center                geometry.Point
	doors                 []geometry.Point
	emptyItemPoints       []geometry.Point
	emptyItemPointsIndex  map[geometry.Point]int
	emptyActorPoints      []geometry.Point
	emptyActorPointsIndex map[geometry.Point]int
}

// Room constants
const (
	MinRoomSize = 3 // Wall + Floor + Wall
)

//
//
// --- CONSTRUCTORS ---
//
//

// NewCustomMap - map constructor
func NewCustomMap(width, height int) *Map {
	m := &Map{
		width:         width,
		height:        height,
		tileGrid:      createMatrix[Cell](height, width),
		actorGrid:     createMatrix[int](height, width),
		itemGrid:      createMatrix[int](height, width),
		entrancePoint: geometry.Point{X: -1, Y: -1},
		exitPoint:     geometry.Point{X: -1, Y: -1},
	}

	// Configure generators
	m.idGen = m.generateID(0)
	m.SetSeed(rand.Int63())

	return m
}

// NewDefaultMap - generate default map with predefined width = 80 and height = 24
func NewDefaultMap() *Map {
	return NewCustomMap(80, 24)
}

//
//
// --- GETTERS ---
//
//

// GetEntrancePoint - returns entrance point
func (m *Map) GetEntrancePoint() geometry.Point {
	return m.entrancePoint
}

// GetExitPoint - returns exit point
func (m *Map) GetExitPoint() geometry.Point {
	return m.exitPoint
}

// GetEntranceRoom - returns entrance room pointer
func (m *Map) GetEntranceRoom() *Room {
	return m.entranceRoom
}

// GetExitRoom - returns exit room pointer
func (m *Map) GetExitRoom() *Room {
	return m.exitRoom
}

func (m *Map) GetTileType(p geometry.Point) (TileType, bool) {
	if !m.InBounds(p) {
		return Empty, false
	}

	t := m.tileGrid[p.Y][p.X].Type
	return t, true
}

// GetItemID - returns item ID at given position
func (m *Map) GetItemID(p geometry.Point) (int, bool) {
	if !m.InBounds(p) {
		return 0, false
	}

	id := m.itemGrid[p.Y][p.X]
	return id, true
}

// GetActorID - returns actor ID at given position
func (m *Map) GetActorID(p geometry.Point) (int, bool) {
	if !m.InBounds(p) {
		return 0, false
	}

	id := m.actorGrid[p.Y][p.X]
	return id, true
}

// GetSeed - returns current map seed
func (m *Map) GetSeed() int64 {
	return m.seed
}

//
//
// --- PREDICATES ---
//
//

// InBounds - return true if point in map boundaries
func (m *Map) InBounds(p geometry.Point) bool {
	return p.X >= 0 && p.X < m.width && p.Y >= 0 && p.Y < m.height
}

// IsWalkable - returns true if tile at given position is walkable
func (m *Map) IsWalkable(p geometry.Point) bool {
	if !m.InBounds(p) {
		return false
	}
	t := m.tileGrid[p.Y][p.X].Type
	return t != Empty && t != Wall
}

func (m *Map) IsItem(p geometry.Point) bool {
	return m.InBounds(p) && m.itemGrid[p.Y][p.X] != 0
}

func (m *Map) IsActor(p geometry.Point) bool {
	return m.InBounds(p) && m.actorGrid[p.Y][p.X] != 0
}

//
//
// --- SETTERS ---
//
//

// SetItem - sets item position on the map
func (m *Map) SetItem(p geometry.Point, id int) bool {
	if !m.IsWalkable(p) {
		log.Printf("[ERROR] Can't set item at %v: tile is not walkable\n", p)
		return false
	}

	m.itemGrid[p.Y][p.X] = id
	room, _ := m.GetRoomByPoint(p)

	if id == 0 {
		room.addItemPoint(p)
	} else {
		room.removeItemPoint(p)
	}

	return true
}

// SetActor - sets actor position on the map
func (m *Map) SetActor(p geometry.Point, id int) bool {
	if !m.IsWalkable(p) {
		log.Printf("[ERROR] Can't set actor at %v: tile is not walkable\n", p)
		return false
	}

	m.actorGrid[p.Y][p.X] = id
	room, _ := m.GetRoomByPoint(p)

	if id == 0 {
		room.addItemPoint(p)
	} else {
		room.removeItemPoint(p)
	}

	return true
}

// SetSeed - set seed and generate new random generator
func (m *Map) SetSeed(seed int64) {
	m.seed = seed
	m.rand = rand.New(rand.NewSource(seed))
}

//
//
// --- MUTATORS ---
//
//

func (m *Map) GenID() int {
	return m.idGen()
}

func (m *Map) RemoveItem(p geometry.Point) bool {
	return m.SetItem(p, 0)
}

func (m *Map) RemoveActor(p geometry.Point) bool {
	return m.SetActor(p, 0)
}

func (m *Map) TakeRandomItemPoint(r *Room) (geometry.Point, bool) {
	return m.takeRandomPoint(r.emptyItemPoints, r.removeItemPoint)
}

func (m *Map) TakeRandomActorPoint(r *Room) (geometry.Point, bool) {
	return m.takeRandomPoint(r.emptyActorPoints, r.removeActorPoint)
}

func (m *Map) takeRandomPoint(points []geometry.Point, removeFunc func(p geometry.Point)) (geometry.Point, bool) {
	if len(points) == 0 {
		return geometry.Point{}, false
	}

	point := points[m.rand.Intn(len(points))]
	removeFunc(point)

	return point, true
}

func (r *Room) addItemPoint(p geometry.Point) {
	addPoint(p, &r.emptyItemPoints, r.emptyItemPointsIndex)
}

func (r *Room) addActorPoint(p geometry.Point) {
	addPoint(p, &r.emptyActorPoints, r.emptyActorPointsIndex)
}

func addPoint(p geometry.Point, s *[]geometry.Point, m map[geometry.Point]int) {
	if _, ok := m[p]; ok {
		return
	}

	*s = append(*s, p)
	m[p] = len(*s) - 1
}

func (r *Room) removeItemPoint(p geometry.Point) {
	removePoint(p, &r.emptyItemPoints, r.emptyItemPointsIndex)
}

func (r *Room) removeActorPoint(p geometry.Point) {
	removePoint(p, &r.emptyActorPoints, r.emptyActorPointsIndex)
}

func removePoint(p geometry.Point, s *[]geometry.Point, m map[geometry.Point]int) {
	removable, exists := m[p]

	if !exists {
		return
	}

	// Get last
	lastInd := len(*s) - 1
	lastPoint := (*s)[lastInd]

	(*s)[removable] = lastPoint // Overwrite removable point by last point
	m[lastPoint] = removable    // Update index in the map

	// Reduce slice size and remove point from map
	*s = (*s)[:lastInd]
	delete(m, p)
}

//
//
// --- LEVEL GENERATION LOGIC ---
//
//

func (m *Map) GenerateLevel(items, actors int) {
	m.ClearLevel()
	m.GenerateTopology(3, 3)
	// m.GenerateKeysAndDoors()
	m.GenerateItems(items)
	m.GenerateActors(actors)
}

//
// -- Clearing --
//

func (m *Map) ClearLevel() {
	m.ClearTopology()
	m.ClearItems()
	m.ClearActors()
	m.idGen = m.generateID(0)
}

func (m *Map) ClearTopology() {
	clearMatrix(m.tileGrid)
	m.entranceRoom = nil
	m.exitRoom = nil
	m.entrancePoint = geometry.Point{X: -1, Y: -1}
	m.exitPoint = geometry.Point{X: -1, Y: -1}
}

func (m *Map) ClearItems() {
	clearMatrix(m.itemGrid)

	for _, room := range m.rooms {
		m.updateRoomEmptyPoints(room, room.addItemPoint, m.itemGrid)
	}

	if m.entranceRoom != nil {
		delete(m.entranceRoom.emptyItemPointsIndex, m.entrancePoint)
	}

	if m.exitRoom != nil {
		delete(m.exitRoom.emptyItemPointsIndex, m.exitPoint)
	}
}

func (m *Map) ClearActors() {
	clearMatrix(m.actorGrid)

	for _, room := range m.rooms {
		m.updateRoomEmptyPoints(room, room.addActorPoint, m.actorGrid)
	}

	if m.entranceRoom != nil {
		delete(m.entranceRoom.emptyActorPointsIndex, m.entrancePoint)
	}

	if m.exitRoom != nil {
		delete(m.exitRoom.emptyActorPointsIndex, m.exitPoint)
	}
}

//
// -- Generation --
//

// GenerateTopology - generates rooms and corridors between them using given grid
// k=cols, n=rows, k*n=rooms
// Grid must be positive
// Too big grid leads to error
func (m *Map) GenerateTopology(k, n int) bool {
	if !m.canFitGrid(k, n) {
		return false
	}
	sectors := m.sectorization(k, n)
	m.createRooms(sectors)
	m.createEntranceAndExit()
	m.connectRooms()
	return true
}

func (m *Map) canFitGrid(k, n int) bool {
	if k <= 0 || n <= 0 {
		return false
	}

	widthMargins, heightMargins := k-1, n-1
	minAllowedMapWidth := k*MinRoomSize + MarginBetweenSectors*widthMargins
	minAllowedMapHeight := n*MinRoomSize + MarginBetweenSectors*heightMargins

	if m.width < minAllowedMapWidth || m.height < minAllowedMapHeight {
		return false
	}

	return true
}

// Function creates puzzle layout so every sector has different size
// k stands for x-dimension, n stands for y-dimension
func (m *Map) sectorization(k, n int) [][]Sector {
	sectors := createMatrix[Sector](n, k)
	sectorWidth, sectorWidthRem := m.width/k, m.width%k
	sectorHeight, sectorHeightRem := m.height/n, m.height%n

	heightRemLimit := make([]int, k)
	for h := range heightRemLimit {
		heightRemLimit[h] = sectorHeightRem
	}
	yMins := make([]int, k)

	for row := range n {
		widthRemLimit := sectorWidthRem
		xMin := 0
		for col := range k {
			var randHeightRemAdd, yMax int
			if row == n-1 { // For last row just add limit's remainder without randomization
				randHeightRemAdd = heightRemLimit[col]
				yMax = yMins[col] + sectorHeight + randHeightRemAdd
			} else {
				randHeightRemAdd = m.rand.Intn(heightRemLimit[col] + 1)
				yMax = yMins[col] + sectorHeight + randHeightRemAdd - MarginBetweenSectors
			}
			heightRemLimit[col] -= randHeightRemAdd

			var randWidthRemAdd, xMax int
			if col == k-1 { // For last col just add limit's remainder without randomization
				randWidthRemAdd = widthRemLimit
				xMax = xMin + sectorWidth + randWidthRemAdd
			} else {
				randWidthRemAdd = m.rand.Intn(widthRemLimit + 1)
				xMax = xMin + sectorWidth + randWidthRemAdd - MarginBetweenSectors
			}
			widthRemLimit -= randWidthRemAdd

			sectors[row][col] = Sector{xMin: xMin, xMax: xMax, yMin: yMins[col], yMax: yMax}
			xMin = xMax + MarginBetweenSectors
			yMins[col] = yMax + MarginBetweenSectors
		}
	}

	return sectors
}

func (m *Map) createRooms(sectors [][]Sector) {
	minHeight, minWidth := MinRoomSize, MinRoomSize
	rows, cols := len(sectors), len(sectors[0])
	m.rooms = make([]*Room, rows*cols)

	// Create rooms
	for row := range rows {
		for col := range cols {
			yMax, yMin := sectors[row][col].yMax, sectors[row][col].yMin
			xMax, xMin := sectors[row][col].xMax, sectors[row][col].xMin

			// Pick random height and width
			sectorHeight := yMax - yMin
			sectorWidth := xMax - xMin
			roomHeight := minHeight + m.rand.Intn(sectorHeight-minHeight+1)
			roomWidth := minWidth + m.rand.Intn(sectorWidth-minWidth+1)
			utilSquare := (roomHeight - 2) * (roomWidth - 2)

			// Pick random pos (left-upper corner)
			allowedYMax := yMax - roomHeight
			allowedXMax := xMax - roomWidth
			posX := xMin + m.rand.Intn(allowedXMax-xMin+1)
			posY := yMin + m.rand.Intn(allowedYMax-yMin+1)

			// Assemble
			room := &Room{
				id:     row*cols + col,
				pos:    geometry.Point{X: posX, Y: posY},
				width:  roomWidth,
				height: roomHeight,
				center: geometry.Point{
					X: posX + roomWidth/2,
					Y: posY + roomHeight/2,
				},
				emptyItemPoints:       make([]geometry.Point, 0, utilSquare),
				emptyItemPointsIndex:  make(map[geometry.Point]int, utilSquare),
				emptyActorPoints:      make([]geometry.Point, 0, utilSquare),
				emptyActorPointsIndex: make(map[geometry.Point]int, utilSquare),
			}
			m.updateRoomEmptyPoints(room, room.addItemPoint, m.itemGrid)
			room.emptyActorPoints = slices.Clone(room.emptyItemPoints)
			room.emptyActorPointsIndex = maps.Clone(room.emptyItemPointsIndex)
			m.rooms[row*cols+col] = room
		}
	}

	// Add rooms to the tile grid
	for _, room := range m.rooms {
		for y := room.pos.Y; y < room.pos.Y+room.height; y++ {
			for x := room.pos.X; x < room.pos.X+room.width; x++ {
				if y == room.pos.Y || y == room.pos.Y+room.height-1 || x == room.pos.X || x == room.pos.X+room.width-1 {
					m.tileGrid[y][x].Type = Wall
				} else {
					m.tileGrid[y][x].Type = Floor
				}
			}
		}
	}

	// Make the order of rooms random
	shuffle(m.rand, m.rooms)
}

func (m *Map) createEntranceAndExit() {
	entryInd, exitInd := 0, 0 // If we have only one room

	if len(m.rooms) > 1 {
		entryInd = m.rand.Intn(len(m.rooms) - 1)
		exitInd = entryInd + 1
	}

	m.entranceRoom, m.exitRoom = m.rooms[entryInd], m.rooms[exitInd]
	m.entrancePoint, _ = m.TakeRandomItemPoint(m.entranceRoom)
	m.exitPoint, _ = m.TakeRandomItemPoint(m.exitRoom)
	m.tileGrid[m.exitPoint.Y][m.exitPoint.X].Type = Exit

	// Prohibit actor spawn at entrance and exit point
	m.entranceRoom.removeActorPoint(m.entrancePoint)
	m.exitRoom.removeActorPoint(m.exitPoint)
}

func (m *Map) connectRooms() {
	for i := 0; i < len(m.rooms)-1; i++ {
		path, ok := m.findDiggingPath(m.rooms[i], m.rooms[i+1])

		if !ok {
			log.Fatalf("[ERROR] Could not find digging path between room #%d and room #%d", i, i+1)
		}

		for _, p := range path {
			x, y := p.X, p.Y
			tile := &m.tileGrid[y][x]

			switch tile.Type {
			case Empty:
				tile.Type = Corridor
			case Wall:
				tile.Type = Door
			default:
			}
		}
	}
}

func (m *Map) findDiggingPath(r1, r2 *Room) ([]geometry.Point, bool) {
	if r1 == nil || r2 == nil {
		return nil, false
	}

	path := algorithm.FindPath[geometry.Point](m, r1.center, r2.center, m.getDiggingCost)
	if path == nil {
		return nil, false
	}

	return path, true
}

func (m *Map) getDiggingCost(p geometry.Point) float64 {
	tile := m.tileGrid[p.Y][p.X].Type
	cost := 0.0

	switch tile {
	case Empty:
		cost = 1.0
	case Corridor:
		cost = 0.5
	case Floor, Door:
		cost = 3.0
	case Wall:
		cost = 5.0
	default:
		return 1.0
	}

	if tile == Empty {
		for _, n := range getNeighborList(p) {
			if m.InBounds(n) {
				nt := m.tileGrid[n.Y][n.X].Type
				if nt == Wall || nt == Corridor {
					cost += 5.0
				}
			}
		}
	}

	return cost
}

func (m *Map) GenerateItems(n int) map[int]geometry.Point {
	return m.generateObjects(n, m.itemGrid, func(r *Room) int { return len(r.emptyItemPointsIndex) }, m.TakeRandomItemPoint)
}

func (m *Map) GenerateActors(n int) map[int]geometry.Point {
	return m.generateObjects(n, m.actorGrid, func(r *Room) int { return len(r.emptyActorPointsIndex) }, m.TakeRandomActorPoint)
}

func (m *Map) generateObjects(n int, grid [][]int, getRoomCapacity func(*Room) int, getRandomPoint func(*Room) (geometry.Point, bool)) map[int]geometry.Point {
	if n <= 0 {
		log.Printf("[INFO] No object was generated due to n=%v\n", n)
		return nil
	}

	points := make(map[int]geometry.Point, n)
	primaryRoomList := make([]*Room, 0, len(m.rooms))
	availableRoomsPrime := make(map[*Room]int, len(m.rooms)) // Map stores number of points in primary pool
	availableRoomsRes := make(map[*Room]int, len(m.rooms))   // Map stores number of points in reserve pool

	for _, room := range m.rooms {
		if room == m.entranceRoom {
			continue
		}

		total := getRoomCapacity(room)
		primePoints := int(float64(total) * PrimaryPoolMultiplier)
		reservePoints := total - primePoints

		if primePoints > 0 {
			primaryRoomList = append(primaryRoomList, room)
			availableRoomsPrime[room] = primePoints
		}

		if reservePoints > 0 {
			availableRoomsRes[room] = reservePoints
		}
	}

	var spawned int

	for len(availableRoomsPrime) > 0 && spawned < n {
		roomInd := m.rand.Intn(len(primaryRoomList))
		room := primaryRoomList[roomInd]

		rest := availableRoomsPrime[room]

		if spawned >= n {
			break
		}

		point, _ := getRandomPoint(room)
		id := m.idGen()

		points[id] = point
		grid[point.Y][point.X] = id

		spawned++
		rest--
		availableRoomsPrime[room] = rest

		if rest == 0 {
			last := len(primaryRoomList) - 1
			primaryRoomList[roomInd] = primaryRoomList[last]
			primaryRoomList = primaryRoomList[:last]

			delete(availableRoomsPrime, room)
			continue
		}
	}

	for len(availableRoomsRes) > 0 && spawned < n {
		for room := range availableRoomsRes {
			rest := availableRoomsRes[room]

			if spawned >= n {
				break
			}

			point, _ := getRandomPoint(room)
			id := m.idGen()

			points[id] = point
			grid[point.Y][point.X] = id

			spawned++
			rest--
			availableRoomsRes[room] = rest

			if rest == 0 {
				delete(availableRoomsRes, room)
				continue
			}
		}
	}

	return points
}

//
//
// --- LOOT MANAGEMENT ---
//
//

func (m *Map) SpawnLoot(p geometry.Point, rad int) (int, geometry.Point, bool) {
	lootPoint, ok := m.FindLootPoint(p, rad)

	if !ok {
		return 0, geometry.Point{}, false
	}

	id := m.idGen()
	m.itemGrid[lootPoint.Y][lootPoint.X] = id

	return id, lootPoint, true
}

func (m *Map) FindLootPoint(center geometry.Point, rad int) (geometry.Point, bool) {
	if !m.IsWalkable(center) {
		log.Printf("[ERROR] Can't spawn loot: center is not walkable\n")
		return geometry.Point{}, false
	}

	room, ok := m.GetRoomByPoint(center)

	if !ok {
		log.Printf("[ERROR] Can't spawn loot: point %v does not belong to any room\n", center)
		return geometry.Point{}, false
	}

	x, y := center.X, center.Y

	if m.itemGrid[y][x] == 0 {
		return center, true
	}

	roomYMin, roomYMax := room.pos.Y, room.pos.Y+room.height-1
	roomXMin, roomXMax := room.pos.X, room.pos.X+room.width-1

	limit := rad

	if rad < 0 {
		limit = max(room.width, room.height)
	}

	for r := 1; r <= limit; r++ {
		frameYMin, frameYMax := y-r, y+r
		frameXMax, frameXMin := x+r, x-r

		if frameYMin < roomYMin && frameXMin < roomXMin && frameXMax > roomXMax && frameYMax > roomYMax {
			break
		}

		validYMin, validYMax := max(frameYMin, roomYMin), min(frameYMax, roomYMax)
		validXMin, validXMax := max(frameXMin, roomXMin), min(frameXMax, roomXMax)

		if frameYMin >= roomYMin {
			for i := validXMin; i <= validXMax; i++ {
				if m.tileGrid[frameYMin][i].Type == Floor && m.itemGrid[frameYMin][i] == 0 {
					return geometry.Point{X: i, Y: frameYMin}, true
				}
			}
		}

		if frameYMax <= roomYMax {
			for i := validXMin; i <= validXMax; i++ {
				if m.tileGrid[frameYMax][i].Type == Floor && m.itemGrid[frameYMax][i] == 0 {
					return geometry.Point{X: i, Y: frameYMax}, true
				}
			}
		}

		if frameXMin >= roomXMin {
			for i := validYMin; i <= validYMax; i++ {
				if m.tileGrid[i][frameXMin].Type == Floor && m.itemGrid[i][frameXMin] == 0 {
					return geometry.Point{X: frameXMin, Y: i}, true
				}
			}
		}

		if frameXMax <= roomXMax {
			for i := validYMin; i <= validYMax; i++ {
				if m.tileGrid[i][frameXMax].Type == Floor && m.itemGrid[i][frameXMax] == 0 {
					return geometry.Point{X: frameXMax, Y: i}, true
				}
			}
		}
	}

	log.Printf("[INFO] Can't spawn loot: no place to spawn with rad=%d\n", rad)
	return geometry.Point{}, false
}

//
//
// --- QUERIES ---
//
//

func (m *Map) FindPath(p1, p2 geometry.Point) ([]geometry.Point, bool) {
	if !m.IsWalkable(p1) || !m.IsWalkable(p2) {
		return nil, false
	}

	path := algorithm.FindPath[geometry.Point](m, p1, p2, m.getFollowingCost)
	if path == nil {
		return nil, false
	}

	return path, true
}

func (m *Map) getFollowingCost(p geometry.Point) float64 {
	tile := m.tileGrid[p.Y][p.X].Type
	cost := 0.0

	switch tile {
	case Empty, Wall:
		cost = math.Inf(1)
	case Corridor, Floor, Door:
		cost = 1.0
	default:
		return 1.0
	}

	return cost
}

//
//
// --- A* ALGORITHM INTERFACE ---
//
//

// GetNeighbors - returns a slice of neighboring points
// Returns points within the boundaries to the right, below, left and above given point
func (m *Map) GetNeighbors(p geometry.Point) []geometry.Point {
	valid := make([]geometry.Point, 0, 4)

	for _, n := range getNeighborList(p) {
		// Only points in bounds
		if m.InBounds(n) {
			valid = append(valid, n)
		}
	}

	return valid
}

func (m *Map) CalcHeuristic(p1, p2 geometry.Point) float64 {
	x1, y1 := p1.X, p1.Y
	x2, y2 := p2.X, p2.Y
	return math.Abs(float64(x2-x1)) + math.Abs(float64(y2-y1))
}

//
//
// --- HELPERS & UTILITIES ---
//
//

func createMatrix[T any](rows, cols int) [][]T {
	matrix := make([][]T, rows)
	for i := range matrix {
		matrix[i] = make([]T, cols)
	}
	return matrix
}

func clearMatrix[T any](matrix [][]T) {
	for i := range matrix {
		clear(matrix[i])
	}
}

func (m *Map) generateID(startID int) func() int {
	return func() int {
		startID++
		return startID
	}
}

func getNeighborList(p geometry.Point) []geometry.Point {
	neighbors := []geometry.Point{
		{X: p.X, Y: p.Y + 1},
		{X: p.X, Y: p.Y - 1},
		{X: p.X + 1, Y: p.Y},
		{X: p.X - 1, Y: p.Y},
	}
	return neighbors
}

func shuffle[T any](r *rand.Rand, s []T) {
	r.Shuffle(len(s), func(i, j int) {
		s[i], s[j] = s[j], s[i]
	})
}

func (m *Map) updateRoomEmptyPoints(room *Room, addPoint func(p geometry.Point), grid [][]int) {
	utilX, utilY := room.pos.X+1, room.pos.Y+1
	utilHeight, utilWidth := room.height-2, room.width-2

	for i := utilY; i < utilY+utilHeight; i++ {
		for j := utilX; j < utilX+utilWidth; j++ {
			if grid[i][j] == 0 {
				addPoint(geometry.Point{X: j, Y: i})
			}
		}
	}
}

//
//
// --- TESTING HELPERS ---
//
//

//
// -- DEBUG UTILITIES --
//

// String - returns string representation of the map
func (m *Map) String() string {
	var sb strings.Builder

	for row := range m.tileGrid {
		for col := range m.tileGrid[row] {
			var ch byte
			tile := m.tileGrid[row][col].Type

			switch tile {
			case Wall:
				ch = '#'
			case Floor:
				ch = '.'
			case Door:
				ch = '+'
			case Corridor:
				ch = '*'
			case Exit:
				ch = '>'
			default:
				ch = ' '
			}

			if row == m.entrancePoint.Y && col == m.entrancePoint.X {
				ch = '@'
			}

			sb.WriteByte(ch)
		}
		sb.WriteByte('\n')
	}

	return sb.String()
}

//
// -- Room methods --
//

// GetRoomsCount - wrap on len function
func (m *Map) GetRoomsCount() int {
	return len(m.rooms)
}

// GetRoomByPoint - returns room by point (like by coordinates)
// Returns (pointer, true) to the room where the given point is located
// Returns (nil, false) if point is outside any room
func (m *Map) GetRoomByPoint(pos geometry.Point) (*Room, bool) {
	for _, room := range m.rooms {
		x, y := pos.X, pos.Y
		xMin, xMax := room.pos.X, room.pos.X+room.width
		yMin, yMax := room.pos.Y, room.pos.Y+room.height

		if x >= xMin && x < xMax && y >= yMin && y < yMax {
			return room, true
		}
	}

	return nil, false
}

// GetRoomByID - returns room by id
// Returns (pointer, true) to the room with given id
// Returns (nil, false) if the room with given id is not found
func (m *Map) GetRoomByID(id int) (*Room, bool) {
	for _, room := range m.rooms {
		if room.id == id {
			return room, true
		}
	}

	return nil, false
}

// GetID - returns room ID
func (r *Room) GetID() int {
	return r.id
}

// GetPos - returns room position (left-upper corner)
func (r *Room) GetPos() geometry.Point {
	return r.pos
}

// GetHW - returns room height and width
func (r *Room) GetHW() (int, int) {
	return r.height, r.width
}

// GetCenter - returns room center point
func (r *Room) GetCenter() geometry.Point {
	return r.center
}

func (r *Room) GetEmptyItemPoints() []geometry.Point {
	return slices.Clone(r.emptyItemPoints)
}

func (r *Room) GetEmptyItemPointsIndex() map[geometry.Point]int {
	return maps.Clone(r.emptyItemPointsIndex)
}

func (r *Room) GetEmptyActorPoints() []geometry.Point {
	return slices.Clone(r.emptyActorPoints)
}

func (r *Room) GetEmptyActorPointsIndex() map[geometry.Point]int {
	return maps.Clone(r.emptyActorPointsIndex)
}
