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

	"github.com/unclestep/Rogue/internal/pkg/algorithm"
	"github.com/unclestep/Rogue/internal/pkg/geometry"
	"github.com/unclestep/Rogue/internal/pkg/idgen"
)

// Map - structure for gameboard
type Map struct {
	Cols           int              `json:"cols"`             // Map width
	Rows           int              `json:"rows"`             // Map height
	TileGrid       [][]Cell         `json:"tile_grid"`        // Layer 1: Game landscape
	ItemGrid       [][]int          `json:"item_grid"`        // Layer 2: Location of items
	ActorGrid      [][]int          `json:"actor_grid"`       // Layer 3: Location of actors
	Rooms          []*Room          `json:"rooms"`            // Pointers to all level rooms
	EntranceRoomId RoomId           `json:"entrance_room_id"` // Pointer to room with spawn point
	ExitRoomId     RoomId           `json:"exit_room_id"`     // Pointer to room with exit point
	EntrancePoint  geometry.Point   `json:"entrance_point"`   // Spawn point
	ExitPoint      geometry.Point   `json:"exit_point"`       // End of level point
	VisibleCells   []geometry.Point `json:"visible_cells"`    // For swift access

	// Following attributes are not serialized to JSON: must be recalculated on load

	doors   map[geometry.Point]*DoorMetadata // For swift access
	roomMap map[RoomId]*Room                 // For swift access
}

// Cell - structure for cell entity
type Cell struct {
	Type       TileType        `json:"type"`
	Visibility VisibilityState `json:"visibility"`
	RoomId     RoomId          `json:"room_id"`
}

// TileType - enumeration of cell tile types
type TileType int

// Tile types
const (
	Empty TileType = iota
	Wall
	Floor
	OpenDoor
	ClosedDoor
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

// Map constants
const (
	SectorMargin          = 1 // Right margin of one room + left margin of second room
	PrimaryPoolMultiplier = 0.5
)

// Room - structure for room entity
type Room struct {
	Id               RoomId           `json:"id"`
	Pos              geometry.Point   `json:"pos"`                // Left-upper walkable corner
	Cols             int              `json:"cols"`               // Room width
	Rows             int              `json:"rows"`               // Room height
	Center           geometry.Point   `json:"center"`             // Room center
	Doors            []*DoorMetadata  `json:"doors"`              // Info about all doors in the room
	EmptyItemPoints  []geometry.Point `json:"empty_item_points"`  // Cached empty points for items. Must be synchronized with the map's itemGrid
	EmptyActorPoints []geometry.Point `json:"empty_actor_points"` // Cached empty points for actors. Must be synchronized with the map's actorGrid

	// Following attributes are not serialized to JSON: must be recalculated on load

	emptyItemPointsIndex  map[geometry.Point]int // For swift access
	emptyActorPointsIndex map[geometry.Point]int // For swift access
}

// RoomId - room identification type
type RoomId int

// Room constants
const (
	InvalidRoomId  = 0
	MinRoomSize    = 3 // Wall + Floor + Wall
	MaxKeysForRoom = 1
)

func NewInvalidPoint() geometry.Point {
	return geometry.Point{X: -1, Y: -1}
}

// DoorMetadata - data of all doors
type DoorMetadata struct {
	Pos    geometry.Point `json:"pos"`     // Door position
	Locked bool           `json:"locked"`  // Lock state
	Color  DoorColor      `json:"color"`   // Key color which opens this door
	KeyPos geometry.Point `json:"key_pos"` // Position of key which opens this door
}

// DoorColor - special int type for key colors
type DoorColor int

//
//
// --- CONSTRUCTORS ---
//
//

// NewCustomMap - map constructor.
func NewCustomMap(width, height int) *Map {
	m := &Map{
		Cols:          width,
		Rows:          height,
		TileGrid:      createMatrix[Cell](height, width),
		ActorGrid:     createMatrix[int](height, width),
		ItemGrid:      createMatrix[int](height, width),
		EntrancePoint: NewInvalidPoint(),
		ExitPoint:     NewInvalidPoint(),
	}

	return m
}

// NewDefaultMap - generate default map with predefined width=80 and height=24.
func NewDefaultMap() *Map {
	return NewCustomMap(80, 24)
}

//
//
// --- GETTERS ---
//
//

// GetEntrancePoint - returns entrance point.
func (m *Map) GetEntrancePoint() geometry.Point {
	return m.EntrancePoint
}

// GetExitPoint - returns exit point.
func (m *Map) GetExitPoint() geometry.Point {
	return m.ExitPoint
}

// GetEntranceRoom - returns entrance room pointer.
func (m *Map) GetEntranceRoom() *Room {
	return m.roomMap[m.EntranceRoomId]
}

// GetExitRoom - returns exit room pointer.
func (m *Map) GetExitRoom() *Room {
	return m.roomMap[m.ExitRoomId]
}

// GetTileType - returns topologic type of cell.
// All possible outputs: Empty, Wall, Floor, OpenDoor, ClosedDoor, Corridor, Exit.
// If point is out of bounds zero returns.
func (m *Map) GetTileType(p geometry.Point) (TileType, bool) {
	if !m.InBounds(p) {
		return Empty, false
	}

	t := m.TileGrid[p.Y][p.X].Type
	return t, true
}

// GetItemID - returns item ID at given position and true if point is in bounds.
func (m *Map) GetItemID(p geometry.Point) (int, bool) {
	if !m.InBounds(p) {
		return 0, false
	}

	id := m.ItemGrid[p.Y][p.X]
	return id, true
}

// GetActorID - returns actor ID at given position and true if point is in bounds.
func (m *Map) GetActorID(p geometry.Point) (int, bool) {
	if !m.InBounds(p) {
		return 0, false
	}

	id := m.ActorGrid[p.Y][p.X]
	return id, true
}

// GetTileVisibility - returns current tile visibility state: visible, explored but not visible, unexplored
func (m *Map) GetTileVisibility(p geometry.Point) (VisibilityState, bool) {
	if !m.InBounds(p) {
		return Unexplored, false
	}
	return m.TileGrid[p.Y][p.X].Visibility, true
}

//
//
// --- PREDICATES ---
//
//

// InBounds - return true if point in map boundaries.
func (m *Map) InBounds(p geometry.Point) bool {
	return p.X >= 0 && p.X < m.Cols && p.Y >= 0 && p.Y < m.Rows
}

// IsWalkable - returns true if tile at given position is walkable.
func (m *Map) IsWalkable(p geometry.Point) bool {
	if !m.InBounds(p) {
		return false
	}
	t := m.TileGrid[p.Y][p.X].Type
	return t != Empty && t != Wall && t != ClosedDoor
}

// CanMoveTo - returns true if given p is walkable and free of actors
func (m *Map) CanMoveTo(p geometry.Point) bool {
	return m.IsWalkable(p) && !m.IsActor(p)
}

// IsItem - returns true if there is an item in given position and point is in bounds.
func (m *Map) IsItem(p geometry.Point) bool {
	return m.InBounds(p) && m.ItemGrid[p.Y][p.X] != 0
}

// IsActor - returns true if there is an actor in given position and point is in bounds.
func (m *Map) IsActor(p geometry.Point) bool {
	return m.InBounds(p) && m.ActorGrid[p.Y][p.X] != 0
}

// IsDoor - returns true if the there is an open or closed door in given position and point is in bounds.
func (m *Map) IsDoor(p geometry.Point) bool {
	return m.InBounds(p) && (m.TileGrid[p.Y][p.X].Type == OpenDoor || m.TileGrid[p.Y][p.X].Type == ClosedDoor)
}

// IsOpenDoor - returns true if there is an open door in given position and point is in bounds.
func (m *Map) IsOpenDoor(p geometry.Point) bool {
	return m.InBounds(p) && m.TileGrid[p.Y][p.X].Type == OpenDoor
}

// IsVisible - returns true if the given tile is visible
func (m *Map) IsVisible(p geometry.Point) bool {
	return m.InBounds(p) && m.TileGrid[p.Y][p.X].Visibility == Visible
}

//
//
// --- SETTERS ---
//
//

// SetItem - sets item position on the map.
// Returns false if it is impossible to set item in given position (tile is not walkable).
// If given id != 0 also reduces room capacity to locate other items (affects generation algorithms).
// If given id == 0 increases room capacity so more items can be generated in that room.
func (m *Map) SetItem(p geometry.Point, id int) bool {
	if !m.IsWalkable(p) {
		log.Printf("[ERROR] Can't set item at %v: tile is not walkable\n", p)
		return false
	}

	m.ItemGrid[p.Y][p.X] = id
	room, _ := m.GetRoomByPoint(p)

	if id == 0 {
		room.addItemPoint(p)
	} else {
		room.removeItemPoint(p)
	}

	return true
}

// SetActor - sets actor position on the map.
// Returns false if it is impossible to set actor in given position (tile is not walkable).
// If given id != 0 also reduces room capacity to locate other actors (affects generation algorithms).
// If given id == 0 increases room capacity so more actors can be generated in that room.
func (m *Map) SetActor(p geometry.Point, id int) bool {
	if !m.IsWalkable(p) {
		log.Printf("[ERROR] Can't set actor at %v: tile is not walkable\n", p)
		return false
	}

	m.ActorGrid[p.Y][p.X] = id
	room, _ := m.GetRoomByPoint(p)

	if id == 0 {
		room.addActorPoint(p)
	} else {
		room.removeActorPoint(p)
	}

	return true
}

//
//
// --- MUTATORS ---
//
//

// Move - moves actor from src to dst point if possible.
// Returns success or failure of operation.
func (m *Map) Move(src, dst geometry.Point) bool {
	if !m.IsWalkable(src) || !m.IsWalkable(dst) {
		log.Printf("[ERROR] Can't move actor from %v to %v: tile(s) is(are) not walkable\n", src, dst)
		return false
	}

	if m.ActorGrid[src.Y][src.X] == 0 {
		log.Printf("[ERROR] Can't move actor from %v to %v: %v is empty\n", src, dst, src)
		return false
	}

	if m.ActorGrid[dst.Y][dst.X] != 0 {
		log.Printf("[ERROR] Can't move actor from %v to %v: destination point %v is not empty\n", src, dst, src)
		return false
	}

	m.ActorGrid[dst.Y][dst.X] = m.ActorGrid[src.Y][src.X]
	m.ActorGrid[src.Y][src.X] = 0

	return true
}

// RemoveItem - removes item from the map making given point available for future procedural generations or random extractions again.
func (m *Map) RemoveItem(p geometry.Point) bool {
	return m.SetItem(p, 0)
}

// RemoveActor - removes actor from the map making given point available for future procedural generations or random extractions again.
func (m *Map) RemoveActor(p geometry.Point) bool {
	return m.SetActor(p, 0)
}

// TakeRandomItemPoint - consumes random empty point for item from given room (item and actor can be together on one cell).
// Returns (random point for item in room, is there available cell for new item).
// Generated position are not repeated until they become empty again.
// This function reduces room capacity.
// NOTE: the function does not generate or set id for item in global map item grid,
// so the function IsItem(genPoint) returns false until you manually set the ID for generated point through SetItem(genPoint, id).
func (m *Map) TakeRandomItemPoint(r *Room, rng *rand.Rand) (geometry.Point, bool) {
	return m.takeRandomPoint(&r.EmptyItemPoints, r.removeItemPoint, rng)
}

// TakeRandomActorPoint - consumes random empty point for actor from given room (item and actor can be together on one cell).
// Returns (random point for actor in room, is there available cell for new actor).
// Generated position are not repeated until they become empty again.
// This function reduces room capacity.
// NOTE: the function does not generate or set id for actor in global map actor grid,
// so the function IsActor(genPoint) returns false until you manually set the ID for generated point through SetActor(genPoint, id).
func (m *Map) TakeRandomActorPoint(r *Room, rng *rand.Rand) (geometry.Point, bool) {
	return m.takeRandomPoint(&r.EmptyActorPoints, r.removeActorPoint, rng)
}

// takeRandomPoint - takes random point in room and make it unavailable to be generated again.
func (m *Map) takeRandomPoint(pointsPtr *[]geometry.Point, removeFunc func(p geometry.Point), rng *rand.Rand) (geometry.Point, bool) {
	points := *pointsPtr

	// If there is no empty cell in room, can't take any point in room
	if len(points) == 0 {
		return geometry.Point{}, false
	}

	point := points[rng.Intn(len(points))] // Take random
	removeFunc(point)                      // Do not forget to remove it from slice and map of empty points of room

	return point, true
}

// addItemPoint - adds empty point for item.
func (r *Room) addItemPoint(p geometry.Point) {
	addPoint(p, &r.EmptyItemPoints, r.emptyItemPointsIndex)
}

// addActorPoint - adds empty point for actor.
func (r *Room) addActorPoint(p geometry.Point) {
	addPoint(p, &r.EmptyActorPoints, r.emptyActorPointsIndex)
}

// addPoint - adds empty point.
func addPoint(p geometry.Point, s *[]geometry.Point, m map[geometry.Point]int) {
	// If the point is empty, do nothing
	if _, ok := m[p]; ok {
		return
	}

	// If the given point is not yet exists in room slice and map of empty points, add it
	*s = append(*s, p)
	m[p] = len(*s) - 1
}

// removeItemPoint - removes empty point for items.
func (r *Room) removeItemPoint(p geometry.Point) {
	removePoint(p, &r.EmptyItemPoints, r.emptyItemPointsIndex)
}

// removeActorPoint - removes empty point for actors.
func (r *Room) removeActorPoint(p geometry.Point) {
	removePoint(p, &r.EmptyActorPoints, r.emptyActorPointsIndex)
}

// removePoint - removes point.
func removePoint(p geometry.Point, s *[]geometry.Point, m map[geometry.Point]int) {
	removable, exists := m[p]

	// If the point is already not empty, do nothing
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

// TryOpenDoor - tries to open the door by passing door position and available key color.
// Returns false if the given point is not a door or the key is not convenient.
// Returns true if the key color matches with door color or door is already unlocked or always was open.
func (m *Map) TryOpenDoor(p geometry.Point, keyColor DoorColor) bool {
	if !m.IsDoor(p) {
		return false
	}

	if m.TileGrid[p.Y][p.X].Type == OpenDoor {
		return true
	}

	door := m.doors[p]
	if door.Color == keyColor {
		m.TileGrid[p.Y][p.X].Type = OpenDoor
		door.Locked = false
		return true
	}

	return false
}

// UpdateVisibleArea - makes points near the given center within the given radius visible
// using Bresenham's algorithm. Previous visible points become explored if they are not visible now.
// Returns false if the given center is not walkable or the rad < 0.
func (m *Map) UpdateVisibleArea(center geometry.Point, rad int) bool {
	if !m.IsWalkable(center) || rad < 0 {
		return false
	}

	for _, vc := range m.VisibleCells {
		m.TileGrid[vc.Y][vc.X].Visibility = Explored
	}
	m.VisibleCells = m.VisibleCells[:0]

	xc, yc := center.X, center.Y
	x, y := 0, rad
	d := 3 - 2*rad

	m.drawHorizontal(xc-y, xc+y, yc)

	for y >= x {
		m.drawHorizontal(xc-y, xc+y, yc+x)
		m.drawHorizontal(xc-y, xc+y, yc-x)
		m.drawHorizontal(xc-x, xc+x, yc+y)
		m.drawHorizontal(xc-x, xc+x, yc-y)

		x++
		if d > 0 {
			y--
			d = d + 4*(x-y) + 10
		} else {
			d = d + 4*x + 6
		}
	}

	return true
}

// drawHorizontal - draws horizontal line
func (m *Map) drawHorizontal(x1, x2, y int) {
	xStart, xEnd := x1, x2
	if xStart > xEnd {
		xStart, xEnd = xEnd, xStart
	}

	for x := xStart; x <= xEnd; x++ {
		if !m.IsWalkable(geometry.Point{X: x, Y: y}) {
			continue
		}

		m.TileGrid[y][x].Visibility = Visible
		m.VisibleCells = append(m.VisibleCells, geometry.Point{X: x, Y: y})
	}
}

// ClearVisibleArea - clears visible area
func (m *Map) ClearVisibleArea() {
	for _, vc := range m.VisibleCells {
		m.TileGrid[vc.Y][vc.X].Visibility = Unexplored
	}
	m.VisibleCells = m.VisibleCells[:0]
}

//
//
// --- LEVEL GENERATION LOGIC ---
//
//

// GenerateLevel - generates map topology, locks doors and places door keys, spawns items and actors using defaults:
// gridWidth=3, gridHeight=3, doorsCount=3, keysCount=3, itemsCount=3, actorsCount=3.
// Returns (map[itemID]itemPosition, map[actorID]actorPosition).
// Automatically clears the previous level: topology, items, actors, doors, keys.
func (m *Map) GenerateLevel(rng *rand.Rand) (map[int]geometry.Point, map[int]geometry.Point) {
	return m.GenerateCustomLevel(3, 3, 3, 3, 3, 3, rng)
}

// GenerateCustomLevel - generates map topology, lock doors and places door keys, spawns items and actor using custom settings.
// Returns (map[itemID]itemPosition, map[actorID]actorPosition).
// Automatically clears the previous level: topology, items, actors, doors, keys.
func (m *Map) GenerateCustomLevel(gridWidth, gridHeight, doorsCount, keysCount, itemsCount, actorsCount int, rng *rand.Rand) (map[int]geometry.Point, map[int]geometry.Point) {
	items := make(map[int]geometry.Point)
	actors := make(map[int]geometry.Point)

	m.ClearLevel()

	generated := m.GenerateTopology(gridWidth, gridHeight, rng)
	if !generated {
		log.Fatalf("[ERROR] Can't generate level, grid too tight: gridWidth=%v gridHeight=%v\n", gridWidth, gridHeight)
	}

	genKeys := m.GenerateKeysAndDoors(doorsCount, keysCount, rng)
	if genKeys != nil {
		maps.Copy(items, genKeys)
	}

	genItems := m.GenerateItems(itemsCount, rng)
	if genItems != nil {
		maps.Copy(items, genItems)
	}

	genActors := m.GenerateActors(actorsCount, rng)
	if genActors != nil {
		maps.Copy(actors, genActors)
	}

	return items, actors
}

//
// -- Clearing --
//

// ClearLevel - clears topology, items, actors, doors, keys.
func (m *Map) ClearLevel() {
	m.ClearTopology()
	m.ClearItems()
	m.ClearActors()
}

// ClearTopology - clears topology and tile grid. Removes entrance and exit.
func (m *Map) ClearTopology() {
	clearMatrix(m.TileGrid)
	m.EntranceRoomId = InvalidRoomId
	m.ExitRoomId = InvalidRoomId
	m.EntrancePoint = NewInvalidPoint()
	m.ExitPoint = NewInvalidPoint()
}

// ClearItems - clears all items and updates capacity of all rooms making them empty again.
// Does not remove entrance and exit point.
// Entrance room points will not be available for participation in random point extractions or procedural generations after this function.
func (m *Map) ClearItems() {
	clearMatrix(m.ItemGrid)

	for _, room := range m.Rooms {
		room.EmptyItemPoints = room.EmptyItemPoints[:0]
		clear(room.emptyItemPointsIndex)

		m.updateRoomEmptyPoints(room, room.addItemPoint, m.ItemGrid)
	}

	if m.EntranceRoomId != InvalidRoomId {
		entranceRoom := m.roomMap[m.EntranceRoomId]
		entranceRoom.EmptyItemPoints = entranceRoom.EmptyItemPoints[:0]
		clear(entranceRoom.emptyItemPointsIndex)
	}

	if m.ExitRoomId != InvalidRoomId {
		exitRoom := m.roomMap[m.ExitRoomId]
		delete(exitRoom.emptyItemPointsIndex, m.ExitPoint)
	}
}

// ClearActors - clears all actors and updates capacity of all rooms making them empty again.
// Entrance room points will not be available for participation in random point extractions or procedural generations after this function anyway.
func (m *Map) ClearActors() {
	clearMatrix(m.ActorGrid)

	for _, room := range m.Rooms {
		room.EmptyActorPoints = room.EmptyActorPoints[:0]
		clear(room.emptyActorPointsIndex)

		m.updateRoomEmptyPoints(room, room.addActorPoint, m.ActorGrid)
	}

	if m.EntranceRoomId != InvalidRoomId {
		entranceRoom := m.roomMap[m.EntranceRoomId]
		entranceRoom.EmptyActorPoints = entranceRoom.EmptyActorPoints[:0]
		clear(entranceRoom.emptyActorPointsIndex)
	}

	if m.ExitRoomId != InvalidRoomId {
		exitRoom := m.roomMap[m.ExitRoomId]
		delete(exitRoom.emptyActorPointsIndex, m.ExitPoint)
	}
}

//
// -- Generation --
//

// GenerateTopology - generates rooms and corridors between them using given grid.
// gridWidth - number of rooms horizontally, gridHeight - number of rooms vertically.
// Returns true if the topology was generated otherwise false.
// Grid should be positive and satisfies the condition for the min possible room sizes (3x3),
// so too big grid leads to impossibility of generation topology.
func (m *Map) GenerateTopology(gridWidth, gridHeight int, rng *rand.Rand) bool {
	if !m.canFitGrid(gridWidth, gridHeight) {
		return false
	}
	sectors := m.sectorization(gridWidth, gridHeight, rng)
	m.createRooms(sectors, rng)
	m.createEntranceAndExit(rng)
	m.connectRooms()
	return true
}

// canFitGrid - checks the possibility to generate given number of rooms horizontally and vertically.
func (m *Map) canFitGrid(gridWidth, gridHeight int) bool {
	if gridWidth <= 0 || gridHeight <= 0 {
		return false
	}

	numberWidthMargins, numberHeightMargins := gridWidth-1, gridHeight-1
	minAllowedMapWidth := gridWidth*MinRoomSize + SectorMargin*numberWidthMargins
	minAllowedMapHeight := gridHeight*MinRoomSize + SectorMargin*numberHeightMargins

	if m.Cols < minAllowedMapWidth || m.Rows < minAllowedMapHeight {
		return false
	}

	return true
}

// Sector - structure for sector entity
type Sector struct {
	xMin, yMin, xMax, yMax int
}

// Function a grid of sector such that each sector has different sizes.
// gridWidth - number of rooms horizontally, gridHeight - number of rooms vertically.
// Returns matrix of sectors.
func (m *Map) sectorization(gridWidth, gridHeight int, rng *rand.Rand) [][]Sector {
	sectors := createMatrix[Sector](gridHeight, gridWidth)
	sectorWidth, sectorWidthRem := m.Cols/gridWidth, m.Cols%gridWidth
	sectorHeight, sectorHeightRem := m.Rows/gridHeight, m.Rows%gridHeight

	heightRemLimit := make([]int, gridWidth) // Height remainder for each column
	for h := range heightRemLimit {
		heightRemLimit[h] = sectorHeightRem
	}
	yMins := make([]int, gridWidth) // Last sector's yMin for each column

	// Create sectors
	for row := range gridHeight {
		widthRemLimit := sectorWidthRem
		xMin := 0
		for col := range gridWidth {
			var randHeightRemAdd, yMax int
			if row == gridHeight-1 { // For last row just add rest limit's remainder without randomization
				randHeightRemAdd = heightRemLimit[col]
				yMax = yMins[col] + (sectorHeight - 1) + randHeightRemAdd
			} else {
				randHeightRemAdd = rng.Intn(heightRemLimit[col] + 1)
				yMax = yMins[col] + (sectorHeight - 1) + randHeightRemAdd - SectorMargin
			}
			heightRemLimit[col] -= randHeightRemAdd

			var randWidthRemAdd, xMax int
			if col == gridWidth-1 { // For last col just add limit's remainder without randomization
				randWidthRemAdd = widthRemLimit
				xMax = xMin + (sectorWidth - 1) + randWidthRemAdd
			} else {
				randWidthRemAdd = rng.Intn(widthRemLimit + 1)
				xMax = xMin + (sectorWidth - 1) + randWidthRemAdd - SectorMargin
			}
			widthRemLimit -= randWidthRemAdd

			sectors[row][col] = Sector{xMin: xMin, xMax: xMax, yMin: yMins[col], yMax: yMax}
			xMin = xMax + SectorMargin + 1
			yMins[col] = yMax + SectorMargin + 1
		}
	}

	return sectors
}

// createRooms - creates one room in every sector.
func (m *Map) createRooms(sectors [][]Sector, rng *rand.Rand) {
	minHeight, minWidth := MinRoomSize, MinRoomSize
	rows, cols := len(sectors), len(sectors[0])
	m.Rooms = make([]*Room, rows*cols)
	m.roomMap = make(map[RoomId]*Room)

	// Create rooms
	for row := range rows {
		for col := range cols {
			yMax, yMin := sectors[row][col].yMax, sectors[row][col].yMin
			xMax, xMin := sectors[row][col].xMax, sectors[row][col].xMin

			// Pick random height and width
			sectorHeight := yMax - yMin + 1
			sectorWidth := xMax - xMin + 1
			roomHeight := minHeight + rng.Intn(sectorHeight-minHeight+1)
			roomWidth := minWidth + rng.Intn(sectorWidth-minWidth+1)

			// Interior space excluding the walls
			utilHeight := roomHeight - 2
			utilWidth := roomWidth - 2
			utilSquare := utilHeight * utilWidth

			// Pick random pos (left-upper corner)
			maxPossibleX := xMax - (roomWidth - 1)       //  Max possible X to take so that the room fits into the sector
			maxPossibleY := yMax - (roomHeight - 1)      // Max possible Y to take so that the room fits into the sector
			posX := xMin + rng.Intn(maxPossibleX-xMin+1) // Final X
			posY := yMin + rng.Intn(maxPossibleY-yMin+1) // Final Y
			utilX, utilY := posX+1, posY+1               // left-upper corner in the room (floor cell)

			// Assemble
			room := &Room{
				Id:   RoomId(row*cols + col + 1),
				Pos:  geometry.Point{X: utilX, Y: utilY},
				Cols: utilWidth,
				Rows: utilHeight,
				Center: geometry.Point{
					X: utilX + utilWidth/2,
					Y: utilY + utilHeight/2,
				},
				EmptyItemPoints:       make([]geometry.Point, 0, utilSquare),
				emptyItemPointsIndex:  make(map[geometry.Point]int, utilSquare),
				EmptyActorPoints:      make([]geometry.Point, 0, utilSquare),
				emptyActorPointsIndex: make(map[geometry.Point]int, utilSquare),
			}
			m.Rooms[row*cols+col] = room
			m.roomMap[room.Id] = room
		}
	}

	// Add rooms to the tile grid
	for _, room := range m.Rooms {
		wallYMin, wallXMin := room.Pos.Y-1, room.Pos.X-1
		wallYMax, wallXMax := wallYMin+room.Rows+1, wallXMin+room.Cols+1

		for y := wallYMin; y <= wallYMax; y++ {
			for x := wallXMin; x <= wallXMax; x++ {
				tile := &m.TileGrid[y][x]
				tile.RoomId = room.Id

				if y == wallYMin || y == wallYMax || x == wallXMin || x == wallXMax {
					tile.Type = Wall
				} else {
					tile.Type = Floor

					room.EmptyItemPoints = append(room.EmptyItemPoints, geometry.Point{X: x, Y: y})
					room.emptyItemPointsIndex[geometry.Point{X: x, Y: y}] = len(room.EmptyItemPoints) - 1
					room.EmptyActorPoints = append(room.EmptyActorPoints, geometry.Point{X: x, Y: y})
					room.emptyActorPointsIndex[geometry.Point{X: x, Y: y}] = len(room.EmptyActorPoints) - 1
				}
			}
		}
	}
}

// createEntranceAndExit - chooses one random room to be entrance and one room to be exited. Generates there entrance and exit points.
func (m *Map) createEntranceAndExit(rng *rand.Rand) {
	// Make the order of rooms random
	shuffle(rng, m.Rooms)

	entryInd, exitInd := 0, 0 // If we have only one room, same room will be entrance and exit
	entranceRoom, exitRoom := m.Rooms[0], m.Rooms[0]

	if len(m.Rooms) > 1 {
		entryInd = rng.Intn(len(m.Rooms) - 1)
		exitInd = entryInd + 1
		entranceRoom = m.Rooms[entryInd]
		exitRoom = m.Rooms[exitInd]
	}

	m.EntranceRoomId, m.ExitRoomId = entranceRoom.Id, exitRoom.Id
	m.EntrancePoint, _ = m.TakeRandomItemPoint(entranceRoom, rng)
	m.ExitPoint, _ = m.TakeRandomItemPoint(exitRoom, rng)
	m.TileGrid[m.ExitPoint.Y][m.ExitPoint.X].Type = Exit

	// Prohibit actor spawn at entrance and exit point
	entranceRoom.removeActorPoint(m.EntrancePoint)
	exitRoom.removeActorPoint(m.ExitPoint)

	// Delete all empty item points from entrance
	entranceRoom.EmptyItemPoints = entranceRoom.EmptyItemPoints[:0]
	clear(entranceRoom.emptyItemPointsIndex)

	// Delete all empty actor points from entrance
	entranceRoom.EmptyActorPoints = entranceRoom.EmptyActorPoints[:0]
	clear(entranceRoom.emptyActorPointsIndex)
}

// connectRooms - digs corridor between two room centers using A* algorithm.
// There is a chance that corridor between two rooms can go through third room,
// so map can be very interesting.
func (m *Map) connectRooms() {
	m.doors = make(map[geometry.Point]*DoorMetadata)

	for i := 0; i < len(m.Rooms)-1; i++ {
		path, ok := m.findDiggingPath(m.Rooms[i], m.Rooms[i+1]) // A* algorithm

		if !ok {
			log.Fatalf("[ERROR] Could not find digging path between room #%d and room #%d", i, i+1)
		}

		for _, p := range path {
			x, y := p.X, p.Y
			tile := &m.TileGrid[y][x]

			switch tile.Type {
			case Empty:
				tile.Type = Corridor
			case Wall:
				tile.Type = OpenDoor
				door := &DoorMetadata{Pos: p}
				m.doors[p] = door
				doorHolder := m.roomMap[tile.RoomId]
				doorHolder.Doors = append(doorHolder.Doors, door)
			default:
			}
		}
	}
}

// findDiggingPath - wrapper function of algorithm package FindPath function with custom cost function.
func (m *Map) findDiggingPath(r1, r2 *Room) ([]geometry.Point, bool) {
	if r1 == nil || r2 == nil {
		return nil, false
	}

	path := algorithm.FindPath[geometry.Point](m, r1.Center, r2.Center, m.getDiggingCost)
	if path == nil {
		return nil, false
	}

	return path, true
}

// getDiggingCost - cost function for A* algorithm.
func (m *Map) getDiggingCost(p geometry.Point) float64 {
	tile := m.TileGrid[p.Y][p.X].Type
	cost := 0.0

	switch tile {
	case Empty:
		cost = 1.0
	case Corridor:
		cost = 0.5 // Better use already dug corridors
	case Floor, OpenDoor:
		cost = 3.0
	case Wall:
		cost = 5.0
	default:
		return 1.0
	}

	// Make it more expensive to go along the walls,
	// so that the map is more sparse
	if tile == Empty {
		for _, n := range getNeighborList(p) {
			if m.InBounds(n) {
				nt := m.TileGrid[n.Y][n.X].Type
				if nt == Wall || nt == Corridor {
					cost += 5.0
				}
			}
		}
	}

	return cost
}

// NavMap - structure for simplified map representation, where rooms and corridors are nodes, doors are edges
type NavMap map[NavNode][]*Edge

func (nm NavMap) deleteEdge(edge *Edge) {
	srcNode := edge.src
	srcEdges := nm[srcNode]
	removeInd := -1

	for i, e := range srcEdges {
		if e == edge {
			removeInd = i
			break
		}
	}

	if removeInd != -1 {
		srcEdges[removeInd] = srcEdges[len(srcEdges)-1]
		srcEdges = srcEdges[:len(srcEdges)-1]
		nm[srcNode] = srcEdges
	}

	backEdge := edge.back
	dstNode := edge.dst
	dstEdges := nm[dstNode]
	removeInd = -1

	for i, e := range dstEdges {
		if e == backEdge {
			removeInd = i
			break
		}
	}

	if removeInd != -1 {
		dstEdges[removeInd] = dstEdges[len(dstEdges)-1]
		dstEdges = dstEdges[:len(dstEdges)-1]
		nm[dstNode] = dstEdges
	}
}

// NavNode - interface for nodes
type NavNode interface {
	GetID() int
	GetCapacity() int
	GetOpenDoorsCount() int
}

// Edge - structure for graph edges
type Edge struct {
	id   int
	src  NavNode
	door *DoorMetadata
	dst  NavNode
	back *Edge // If graph is indirect, should store the pointer to backward edge
}

// GenerateKeysAndDoors - generates doorsCount doors and keysCount keys.
// Returns map[keyID]keyPos.
// If doorsCount > keysCount, some keys can open more than one door;
// if keysCount > doorsCount, keysCount = doorsCount;
// if doorsCount is greater than total number of doors, algorithm will block all found bridges on the map;
// if doorsCount < 0 || keysCount < 0, nil will be returned.
// Algorithm does not block loop edges.
func (m *Map) GenerateKeysAndDoors(doorsCount, keysCount int, rng *rand.Rand) map[int]geometry.Point {
	if doorsCount <= 0 || keysCount <= 0 {
		log.Printf("[INFO] Given doorsCount=%d, keysCount=%d but expected doorsCount>0 and keysCount>0", doorsCount, keysCount)
		return nil
	}

	entranceRoom := m.roomMap[m.EntranceRoomId]

	usedCells := make(map[NavNode]int)

	nm := m.createNavMap()
	topology := nm.analyzeTopology(entranceRoom, usedCells)

	getAvailableEdges := func(topology *TopologyData) []*Edge {
		availableEdges := make([]*Edge, 0, len(topology.Bridges))
		for edge := range topology.Bridges {
			// Don't block doors in start room
			if edge.src != entranceRoom && edge.dst != entranceRoom {
				availableEdges = append(availableEdges, edge)
			}
		}

		// For determinism of test
		slices.SortFunc(availableEdges, func(a, b *Edge) int {
			if a.id < b.id {
				return 1
			} else if a.id > b.id {
				return -1
			}
			return 0
		})
		// Make the order random using local random generator
		shuffle(rng, availableEdges)
		return availableEdges
	}

	validKeysCount := min(doorsCount, keysCount)
	validDoorsCount := min(validKeysCount, len(m.doors))
	doorsForKey := m.getDoorsForKey(validDoorsCount, validKeysCount, rng)

	spawnedKeys := make(map[int]geometry.Point, validDoorsCount)
	curKeyID := idgen.Next()

	for k := 0; k < validKeysCount; k++ {
		numDoors := doorsForKey[k]          // Get number of doors to be blocked by current key
		keysRemaining := validKeysCount - k // Keys left

		curLockedEdges := make([]*Edge, 0) // Slice of locked edges by current key

		// Start blocking the doors
		for door := 0; door < numDoors; door++ {
			availableEdges := getAvailableEdges(topology)

			totalCapacity := topology.SubtreeCapacity[entranceRoom]
			totalDoors := topology.SubtreeDoors[entranceRoom]

			if len(availableEdges) == 0 {
				break
			}

			// Edge that satisfies all conditions
			var selectedEdge *Edge

			// Edge that does not satisfy all conditions but looks convenient to be locked
			var optimalEdge *Edge
			maxOptimalDoors := -1

			// Find convenient room in the path
			for _, edge := range availableEdges {
				var node NavNode
				// It is integral to correct define the node which will be locked
				// Unavailable node is always the one that's further from the center
				if topology.EntryTime[edge.dst] > topology.EntryTime[edge.src] {
					node = edge.dst
				} else {
					node = edge.src
				}

				cutDoors := topology.SubtreeDoors[node] // Number of doors that we lose after locking this edge
				availableDoors := totalDoors - cutDoors // Number of doors that will be still available

				cutCells := topology.SubtreeCapacity[node] // Number of free cells (to place the key) we lose after locking this edge
				availableCells := totalCapacity - cutCells // Number of cells that will be still available

				// This condition checks if there is enough of remain doors and cells
				// availableCells convince to cut edges with enough remain cells
				if availableDoors >= (keysRemaining-1) && availableCells >= keysRemaining {
					selectedEdge = edge
					break
				}

				// If we can't satisfy some of the conditions (or all of them), take the edge that will be the closest one to satisfy them
				if availableCells >= keysRemaining && availableDoors > maxOptimalDoors {
					maxOptimalDoors = availableDoors
					optimalEdge = edge
				}
			}

			// If we couldn't find the edge that satisfies all the conditions, take the optimal one
			if selectedEdge == nil {
				// If optimal edge is not found either, stop the loop
				if optimalEdge == nil {
					break
				}
				selectedEdge = optimalEdge
			}

			curLockedEdges = append(curLockedEdges, selectedEdge)

			nm.deleteEdge(selectedEdge)
			topology = nm.analyzeTopology(entranceRoom, usedCells)
		}

		// If we couldn't find any edge to lock, stop the locking algorithm:
		// we won't be able to find convenient edges for other doors and keys either
		if len(curLockedEdges) == 0 {
			break
		}

		// Define the rooms where we can the spawn the door key
		availableRooms := make([]*Room, 0, len(m.Rooms))
		for _, r := range m.Rooms {
			// Check if the room is in locked subtree
			if _, reachable := topology.EntryTime[r]; !reachable {
				continue
			}

			// If there is no space or consumed all the available cells
			if r == entranceRoom || usedCells[r] >= MaxKeysForRoom {
				continue
			}

			isCritical := true

			// If key is last
			if keysRemaining == 1 {
				isCritical = false
			} else {
				// We are looking at how the spawn will affect subsequent locks
				usedCells[r]++
				testTopology := nm.analyzeTopology(entranceRoom, usedCells)
				totalCapacity := testTopology.SubtreeCapacity[entranceRoom]

				for bridge := range testTopology.Bridges {
					var node NavNode
					if testTopology.EntryTime[bridge.dst] > testTopology.EntryTime[bridge.src] {
						node = bridge.dst
					} else {
						node = bridge.src
					}

					cutCells := testTopology.SubtreeCapacity[node]
					availableCells := totalCapacity - cutCells

					// If we have found at least one bridge that can be safely blocked, then the room is safe.
					if availableCells >= (keysRemaining - 1) {
						isCritical = false
						break
					}
				}
				usedCells[r]--
			}

			if !isCritical {
				availableRooms = append(availableRooms, r)
			}
		}

		// No reason to continue iterating if there is no more available rooms; won't be able to find for other keys too
		if len(availableRooms) == 0 {
			break
		}

		// Spawn the key in any available room
		keyRoom := availableRooms[rng.Intn(len(availableRooms))]
		keyPos, _ := m.TakeRandomItemPoint(keyRoom, rng)
		m.ItemGrid[keyPos.Y][keyPos.X] = curKeyID
		spawnedKeys[curKeyID] = keyPos

		usedCells[keyRoom]++
		topology = nm.analyzeTopology(entranceRoom, usedCells)

		// Truly lock all the selected edges
		for _, doorEdge := range curLockedEdges {
			door := doorEdge.door

			// Lock the door
			door.Locked = true
			door.Color = DoorColor(curKeyID)
			door.KeyPos = keyPos // Save the door key position
			m.TileGrid[door.Pos.Y][door.Pos.X].Type = ClosedDoor
		}

		curKeyID = idgen.Next()
	}

	return spawnedKeys
}

// getDoorsForKey - returns number of doors which key can block for every key
func (m *Map) getDoorsForKey(doorsCount, keysCount int, rng *rand.Rand) []int {
	doorsForKey := make([]int, keysCount)

	for i := 0; i < keysCount; i++ {
		doorsForKey[i] = doorsCount / keysCount
	}

	perm := rng.Perm(keysCount)

	for i := 0; i < doorsCount%keysCount; i++ {
		doorsForKey[perm[i]]++
	}

	return doorsForKey
}

// CorridorHub - abstraction for connections between rooms
type CorridorHub struct {
	id    int
	cells map[geometry.Point]struct{}
}

// createNavMap - creates simplified map representation where rooms and corridors are nodes, doors are edges in the graph
func (m *Map) createNavMap() NavMap {
	nm := make(NavMap)
	hubs := m.findCorridorHubs()
	edgeID := 0

	for _, room := range m.Rooms {
		for _, door := range room.Doors {
			hub := m.getHubConnectedToDoor(hubs, door.Pos)
			if hub != nil {
				abEdge := &Edge{
					id:   edgeID,
					src:  room,
					door: door,
					dst:  hub,
				}
				edgeID++

				// Graph is indirect, don't forget to create backward edge
				baEdge := &Edge{
					id:   edgeID,
					src:  hub,
					door: door,
					dst:  room,
				}
				edgeID++

				abEdge.back = baEdge
				baEdge.back = abEdge

				nm[room] = append(nm[room], abEdge)
				nm[hub] = append(nm[hub], baEdge)
			}
		}
	}

	return nm
}

// findCorridorHubs - finds all corridors between rooms.
// CorridorHub can be simply connection of two rooms or the main artery of the map that unites many rooms
func (m *Map) findCorridorHubs() []*CorridorHub {
	availableCorridors := m.getCorridorTiles()
	hubs := make([]*CorridorHub, 0)
	id := len(m.Rooms)

	for len(availableCorridors) > 0 {
		point := availableCorridors[0]
		restCorridors := availableCorridors[:0]

		hubPoints := m.floodFill(point)
		for _, p := range availableCorridors {
			if _, exists := hubPoints[p]; !exists {
				restCorridors = append(restCorridors, p)
			}
		}

		hubs = append(hubs, &CorridorHub{
			id:    id,
			cells: hubPoints,
		})

		id++
		availableCorridors = restCorridors
	}

	return hubs
}

func (m *Map) getCorridorTiles() []geometry.Point {
	corridorTiles := make([]geometry.Point, 0, 32)

	for r := 0; r < m.Rows; r++ {
		for c := 0; c < m.Cols; c++ {
			if m.TileGrid[r][c].Type == Corridor {
				corridorTiles = append(corridorTiles, geometry.Point{X: c, Y: r})
			}
		}
	}

	return corridorTiles
}

// floodFill - classic BFS flood fill algorithm
func (m *Map) floodFill(start geometry.Point) map[geometry.Point]struct{} {
	visited := make(map[geometry.Point]bool)
	visited[start] = true
	queue := []geometry.Point{start}
	startTile := m.TileGrid[start.Y][start.X].Type // Fill in only the cells having the type of the first cell

	filled := map[geometry.Point]struct{}{}

	for len(queue) > 0 {
		cur := queue[0]
		queue = queue[1:]
		filled[cur] = struct{}{}

		for _, n := range getNeighborList(cur) {
			if m.InBounds(n) && m.TileGrid[n.Y][n.X].Type == startTile && !visited[n] {
				visited[n] = true
				queue = append(queue, n)
			}
		}
	}

	return filled
}

// getHubConnectedToDoor - returns pointer to CorridorHub that runs into the door
func (m *Map) getHubConnectedToDoor(hubs []*CorridorHub, door geometry.Point) *CorridorHub {
	neighbors := getNeighborList(door)

	for _, n := range neighbors {
		if m.InBounds(n) {
			for _, hub := range hubs {
				if _, exists := hub.cells[n]; exists {
					return hub
				}
			}
		}
	}

	return nil
}

// TopologyData - structure for storing found topology data of map
type TopologyData struct {
	Bridges         map[*Edge]bool      // Slice of pointers to edges that are an only way between rooms on the map
	ParentMap       map[NavNode]NavNode // Hashmap with pointers to the parent for every node
	SubtreeCapacity map[NavNode]int     // Number of empty item cells in the subtree
	SubtreeDoors    map[NavNode]int     // Number of doors in the subtree
	EntryTime       map[NavNode]int     // Use to define what a node is closer to start
	ExitTime        map[NavNode]int     // Need to check if the node is in any subtree
}

func (nm NavMap) analyzeTopology(src NavNode, usedCells map[NavNode]int) *TopologyData {
	data := &TopologyData{
		Bridges:         make(map[*Edge]bool),
		ParentMap:       make(map[NavNode]NavNode),
		SubtreeCapacity: make(map[NavNode]int),
		SubtreeDoors:    make(map[NavNode]int),
		EntryTime:       make(map[NavNode]int),
		ExitTime:        make(map[NavNode]int),
	}

	timer := 0

	discovery := make(map[NavNode]int)
	lowLink := make(map[NavNode]int) // To

	var dfs func(cur NavNode, incomingEdge *Edge)
	dfs = func(cur NavNode, incomingEdge *Edge) {
		discovery[cur] = timer // In what order current node was discovered
		lowLink[cur] = timer   // Which node is the closest to the start we can go to from this node directly
		data.EntryTime[cur] = timer
		timer++

		curCapacity := cur.GetCapacity()
		if cur == src {
			curCapacity = 0
		} else if used, exists := usedCells[cur]; exists && used > 0 {
			curCapacity -= used
			if curCapacity < 0 {
				curCapacity = 0
			}
		}
		data.SubtreeCapacity[cur] = curCapacity

		if cur != src {
			data.SubtreeDoors[cur] = cur.GetOpenDoorsCount() // Returns number of open doors (consider doors as part of the rooms, so for corridor return value will be 0)
		} else {
			data.SubtreeDoors[cur] = 0
		}

		if incomingEdge != nil {
			data.ParentMap[cur] = incomingEdge.src
		} else {
			data.ParentMap[cur] = nil
		}

		for _, edge := range nm[cur] {
			neighbor := edge.dst

			// If checked the backward edge
			if incomingEdge != nil && edge == incomingEdge.back {
				continue
			}

			if _, visited := discovery[neighbor]; visited {
				// Update where we can go from this node
				lowLink[cur] = min(lowLink[cur], discovery[neighbor])
			} else {
				dfs(neighbor, edge)
				data.SubtreeCapacity[cur] += data.SubtreeCapacity[neighbor]
				data.SubtreeDoors[cur] += data.SubtreeDoors[neighbor]
				lowLink[cur] = min(lowLink[cur], lowLink[neighbor])

				// If neighbors node is only connected with current node, its lowLink will be higher
				if lowLink[neighbor] > discovery[cur] {
					data.Bridges[edge] = true
					data.Bridges[edge.back] = true
				}
			}
		}
		data.ExitTime[cur] = timer
	}

	dfs(src, nil)
	return data
}

// GenerateItems - spawns items in all rooms except the start one.
// Returns map[itemID]itemPosition.
// If n is greater than the number of all available cells for items on the map, algorithm will place as many as possible items.
func (m *Map) GenerateItems(n int, rng *rand.Rand) map[int]geometry.Point {
	return m.generateObjects(n, m.ItemGrid, func(r *Room) int { return len(r.emptyItemPointsIndex) }, m.TakeRandomItemPoint, rng)
}

// GenerateActors - spawns actors in all rooms except the start one.
// Returns map[actorID]actorPosition.
// If n is greater than the number of all available cells for actors on the map, algorithm will place as many as possible actors.
func (m *Map) GenerateActors(n int, rng *rand.Rand) map[int]geometry.Point {
	return m.generateObjects(n, m.ActorGrid, func(r *Room) int { return len(r.emptyActorPointsIndex) }, m.TakeRandomActorPoint, rng)
}

// generateObjects - common algorithm for items and actors.
func (m *Map) generateObjects(n int, grid [][]int, getRoomCapacity func(*Room) int, getRandomPoint func(*Room, *rand.Rand) (geometry.Point, bool), rng *rand.Rand) map[int]geometry.Point {
	if n <= 0 {
		log.Printf("[INFO] No object was generated due to n=%v\n", n)
		return nil
	}

	points := make(map[int]geometry.Point, n)
	primaryRoomList := make([]*Room, 0, len(m.Rooms))
	availableRoomsPrime := make(map[*Room]int, len(m.Rooms)) // Map stores number of points in primary pool for each room
	availableRoomsRes := make(map[*Room]int, len(m.Rooms))   // Map stores number of points in reserve pool for each room

	for _, room := range m.Rooms {
		if room == m.roomMap[m.EntranceRoomId] {
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

	// Primarily use the prime pool and for this use normal distribution,
	// so some rooms can have many items, some none of them,
	// but do not use more than PrimaryPoolMultiplier of available cells in the room
	for len(availableRoomsPrime) > 0 && spawned < n {
		roomInd := rng.Intn(len(primaryRoomList))
		room := primaryRoomList[roomInd]

		rest := availableRoomsPrime[room]

		if spawned >= n {
			break
		}

		point, _ := getRandomPoint(room, rng)
		id := idgen.Next()

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

	// If we couldn't spawn all objects using normal distribution,
	// use the uniform distribution to place as many object as possible on the map
	for len(availableRoomsRes) > 0 && spawned < n {
		for room := range availableRoomsRes {
			rest := availableRoomsRes[room]

			if spawned >= n {
				break
			}

			point, _ := getRandomPoint(room, rng)
			id := idgen.Next()

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
// --- FOR ACTORS AI ---
//
//

func (m *Map) GenerateScentMap(points []geometry.Point) [][]int {
	scentMap := createMatrix[int](m.Rows, m.Cols)
	fillMatrix(scentMap, math.MaxInt)

	queue := points

	for _, p := range points {
		scentMap[p.Y][p.X] = 0
	}

	for len(queue) > 0 {
		cur := queue[0]
		queue = queue[1:]

		for _, dir := range geometry.GetAllDirs() {
			n := geometry.Point{X: cur.X + dir.X, Y: cur.Y + dir.Y}

			if m.CanMoveTo(n) && scentMap[n.Y][n.X] == math.MaxInt {
				scentMap[n.Y][n.X] = scentMap[cur.Y][cur.X] + 1
				queue = append(queue, n)
			}
		}
	}

	return scentMap
}

//
//
// --- SAVE/LOAD GAME METHODS ---
//
//

func (m *Map) RebuildCache() {
	m.doors = make(map[geometry.Point]*DoorMetadata)
	m.roomMap = make(map[RoomId]*Room)

	for _, room := range m.Rooms {
		m.roomMap[room.Id] = room

		room.emptyItemPointsIndex = make(map[geometry.Point]int)
		for i, p := range room.EmptyItemPoints {
			room.emptyItemPointsIndex[p] = i
		}

		room.emptyActorPointsIndex = make(map[geometry.Point]int)
		for i, p := range room.EmptyActorPoints {
			room.emptyActorPointsIndex[p] = i
		}

		for _, door := range room.Doors {
			m.doors[door.Pos] = door
		}
	}
}

//
//
// --- LOOT MANAGEMENT ---
//
//

// SpawnLoot - finds the empty cell in radius of given point and spawns there an item.
// If rad < 0, searches across whole room;
// If rad == 0, checks only the given center;
// If rad > 0, checks for empty space in given radius but no further than the original room.
// Returns (itemID, itemPosition, isSuccessfulToFindAnEmptyCell)
func (m *Map) SpawnLoot(p geometry.Point, rad int) (int, geometry.Point, bool) {
	lootPoint, ok := m.FindLootPoint(p, rad)

	if !ok {
		return 0, geometry.Point{}, false
	}

	id := idgen.Next()
	m.ItemGrid[lootPoint.Y][lootPoint.X] = id

	return id, lootPoint, true
}

// FindLootPoint - finds the empty cell in radius of given point but does not spawn there an item.
// If rad < 0, searches across whole room;
// If rad == 0, checks only the given center;
// If rad > 0, checks for empty space in given radius but no further than the original room.
// Returns (itemPosition, isSuccessfulToFindAnEmptyCell)
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

	if m.ItemGrid[y][x] == 0 {
		return center, true
	}

	roomYMin, roomYMax := room.Pos.Y, room.Pos.Y+room.Rows-1
	roomXMin, roomXMax := room.Pos.X, room.Pos.X+room.Cols-1

	limit := rad

	if rad < 0 {
		limit = max(room.Cols, room.Rows)
	}

	for r := 1; r <= limit; r++ {
		frameYMin, frameYMax := y-r, y+r
		frameXMin, frameXMax := x-r, x+r

		if frameYMin < roomYMin && frameXMin < roomXMin && frameXMax > roomXMax && frameYMax > roomYMax {
			break
		}

		validYMin, validYMax := max(frameYMin, roomYMin), min(frameYMax, roomYMax)
		validXMin, validXMax := max(frameXMin, roomXMin), min(frameXMax, roomXMax)

		if frameYMin >= roomYMin {
			for i := validXMin; i <= validXMax; i++ {
				if m.TileGrid[frameYMin][i].Type == Floor && m.ItemGrid[frameYMin][i] == 0 {
					return geometry.Point{X: i, Y: frameYMin}, true
				}
			}
		}

		if frameYMax <= roomYMax {
			for i := validXMin; i <= validXMax; i++ {
				if m.TileGrid[frameYMax][i].Type == Floor && m.ItemGrid[frameYMax][i] == 0 {
					return geometry.Point{X: i, Y: frameYMax}, true
				}
			}
		}

		if frameXMin >= roomXMin {
			for i := validYMin; i <= validYMax; i++ {
				if m.TileGrid[i][frameXMin].Type == Floor && m.ItemGrid[i][frameXMin] == 0 {
					return geometry.Point{X: frameXMin, Y: i}, true
				}
			}
		}

		if frameXMax <= roomXMax {
			for i := validYMin; i <= validYMax; i++ {
				if m.TileGrid[i][frameXMax].Type == Floor && m.ItemGrid[i][frameXMax] == 0 {
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

// FindPath - finds the path between two points in map's topology.
// Algorithm can only pass through corridors, floors and open doors.
// It tries to avoid the cells with items and actors.
// Returns slice of points between two given points including them and bool value, if it could find the way.
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

// getFollowingCost - cost function to find the way between points inside the game map: floors, open doors, corridors.
func (m *Map) getFollowingCost(p geometry.Point) float64 {
	x, y := p.X, p.Y
	tile := m.TileGrid[p.Y][p.X].Type
	cost := 0.0

	switch tile {
	case Corridor, Floor, OpenDoor, Exit:
		cost = 1.0
	default:
		cost = math.Inf(1)
	}

	if m.ItemGrid[y][x] != 0 || m.ActorGrid[y][x] != 0 {
		cost += 5.0
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

// CalcHeuristic - implements Manhattan's distance
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

// createMatrix - creates a matrix with given rows and cols
func createMatrix[T any](rows, cols int) [][]T {
	matrix := make([][]T, rows)
	for i := range matrix {
		matrix[i] = make([]T, cols)
	}
	return matrix
}

// clearMatrix - clears the matrix
func clearMatrix[T any](matrix [][]T) {
	for i := range matrix {
		clear(matrix[i])
	}
}

// fillMatrix - fills matrix using given value
func fillMatrix[T any](matrix [][]T, val T) {
	for i := range len(matrix) {
		for j := range len(matrix[i]) {
			matrix[i][j] = val
		}
	}
}

// generateID - generates uniques IDs
func (m *Map) generateID(startID int) func() int {
	return func() int {
		startID++
		return startID
	}
}

// getNeighborList - returns neighbor points to given point
func getNeighborList(p geometry.Point) []geometry.Point {
	neighbors := []geometry.Point{
		{X: p.X, Y: p.Y + 1},
		{X: p.X, Y: p.Y - 1},
		{X: p.X + 1, Y: p.Y},
		{X: p.X - 1, Y: p.Y},
	}
	return neighbors
}

// shuffle - shuffles the slice
func shuffle[T any](r *rand.Rand, s []T) {
	r.Shuffle(len(s), func(i, j int) {
		s[i], s[j] = s[j], s[i]
	})
}

// updateRoomEmptyPoints - finds empty points in the given room
func (m *Map) updateRoomEmptyPoints(room *Room, addPoint func(p geometry.Point), grid [][]int) {
	for i := room.Pos.Y; i < room.Pos.Y+room.Rows; i++ {
		for j := room.Pos.X; j < room.Pos.X+room.Cols; j++ {
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

// Constants for coloring the text
const (
	ColorReset  = "\033[0m"
	ColorRed    = "\033[31m"
	ColorGreen  = "\033[32m"
	ColorYellow = "\033[33m"
	ColorBlue   = "\033[34m"
	ColorPurple = "\033[35m"
	ColorCyan   = "\033[36m"
)

// String - returns string representation of the map
func (m *Map) String() string {
	colors := []string{ColorRed, ColorGreen, ColorYellow, ColorBlue, ColorPurple, ColorCyan}

	keys := make(map[geometry.Point]struct{})
	for _, door := range m.doors {
		if door.Locked {
			keys[door.KeyPos] = struct{}{}
		}
	}

	var sb strings.Builder

	for row := range m.TileGrid {
		for col := range m.TileGrid[row] {
			var ch rune
			tile := m.TileGrid[row][col].Type

			switch tile {
			case Wall:
				ch = '#'
			case Floor:
				ch = '.'
			case OpenDoor:
				ch = '/'
			case ClosedDoor:
				keyID := int(m.doors[geometry.Point{X: col, Y: row}].Color)
				colorIndex := keyID % len(colors)
				sb.WriteString(colors[colorIndex])
				ch = '%'
			case Corridor:
				ch = '*'
			case Exit:
				ch = '>'
			default:
				ch = ' '
			}

			if row == m.EntrancePoint.Y && col == m.EntrancePoint.X {
				ch = '@'
			}

			itemID := m.ItemGrid[row][col]

			if _, isKeyPos := keys[geometry.Point{X: col, Y: row}]; isKeyPos {
				colorIndex := itemID % len(colors)
				sb.WriteString(colors[colorIndex])
				ch = 'K'
			} else if itemID != 0 {
				ch = 'I'
			}

			if m.ActorGrid[row][col] != 0 {
				ch = 'A'
			}

			sb.WriteRune(ch)
			sb.WriteString(ColorReset)
		}
		sb.WriteRune('\n')
	}

	return sb.String()
}

//
// -- Room methods --
//

// GetRoomByPoint - returns room by point (like by coordinates)
// Returns (pointer, true) to the room where the given point is located
// Returns (nil, false) if point is outside any room
func (m *Map) GetRoomByPoint(pos geometry.Point) (*Room, bool) {
	if !m.InBounds(pos) {
		return nil, false
	}

	roomId := m.TileGrid[pos.Y][pos.X].RoomId
	if roomId == 0 {
		return nil, false
	}
	return m.roomMap[roomId], true
}

// GetRoomByID - returns room by id
// Returns (pointer, true) to the room with given id
// Returns (nil, false) if the room with given id is not found
func (m *Map) GetRoomByID(id RoomId) (*Room, bool) {
	for _, room := range m.Rooms {
		if room.Id == id {
			return room, true
		}
	}

	return nil, false
}

// GetID - returns room ID
func (r *Room) GetID() int {
	return int(r.Id)
}

// GetCapacity - returns room's number of empty points for items
func (r *Room) GetCapacity() int {
	return MaxKeysForRoom
}

// GetPos - returns room position (left-upper corner)
func (r *Room) GetPos() geometry.Point {
	return r.Pos
}

// GetHW - returns room height and width
func (r *Room) GetHW() (int, int) {
	return r.Rows, r.Cols
}

// GetCenter - returns room center point
func (r *Room) GetCenter() geometry.Point {
	return r.Center
}

// GetEmptyItemPoints - returns the copy of slice of empty points for items in the room
func (r *Room) GetEmptyItemPoints() []geometry.Point {
	return slices.Clone(r.EmptyItemPoints)
}

// GetEmptyActorPoints - returns the copy of slice of empty points for actors in the room
func (r *Room) GetEmptyActorPoints() []geometry.Point {
	return slices.Clone(r.EmptyActorPoints)
}

// GetOpenDoorsCount - returns number of open doors in the room
func (r *Room) GetOpenDoorsCount() int {
	openDoors := 0

	for _, d := range r.Doors {
		if !d.Locked {
			openDoors++
		}
	}

	return openDoors
}

// GetID - returns corridor hub's ID
func (ch *CorridorHub) GetID() int {
	return ch.id
}

// GetCapacity - returns number of available points for items in the corridor
func (ch *CorridorHub) GetCapacity() int {
	return 0
}

// GetOpenDoorsCount - returns number of open doors in the corridors
func (ch *CorridorHub) GetOpenDoorsCount() int {
	return 0 // 0, because doors are belong to rooms
}
