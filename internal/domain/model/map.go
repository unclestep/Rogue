// Package model within:
// - Map stores information about game landscape, actors and items;
package model

import (
	"log"
	"math"
	"math/rand"
	"strings"

	"github.com/unclestep/Rogue/pkg/geometry"
	"github.com/unclestep/Rogue/pkg/utils"
)

// Map - structure for gameboard
type Map struct {
	Width          int                              // Map width
	Height         int                              // Map height
	tileGrid       [][]Cell                         // Layer 1: Game landscape
	itemGrid       [][]int64                        // Layer 2: Location of items
	actorGrid      [][]int64                        // Layer 3: Location of actors
	rooms          []*Room                          // Pointers to all level rooms
	doors          map[geometry.Point]*DoorMetadata // Doors do not belong to the rooms
	EntranceRoomId RoomId                           // Pointer to room with spawn point
	ExitRoomId     RoomId                           // Pointer to room with exit point
	ExitPoint      geometry.Point                   // End of level point

	// Following attributes are not serialized to JSON: must be recalculated on load
	roomMap map[RoomId]*Room // For swift access
}

type MapBlueprint struct {
	Width          int
	Height         int
	TileGrid       [][]Cell
	Rooms          []*Room
	Doors          map[geometry.Point]*DoorMetadata
	EntranceRoomId RoomId
	ExitRoomId     RoomId
	ExitPoint      geometry.Point
}

func NewMapBlueprint(width, height int) *MapBlueprint {
	return &MapBlueprint{
		Width:    width,
		Height:   height,
		TileGrid: utils.CreateMatrix[Cell](height, width),
		Rooms:    make([]*Room, 0),
		Doors:    make(map[geometry.Point]*DoorMetadata),
	}
}

// Cell - structure for cell model
type Cell struct {
	Type   TileType `json:"type"`
	RoomId RoomId   `json:"room_id"`
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

// Room - structure for room model
type Room struct {
	Id     RoomId
	Pos    geometry.Point // Left-upper walkable corner
	Width  int            // Room width
	Height int            // Room height
	Center geometry.Point // Room center

	// Following attributes should not be serialized: must be recalculated on load

	emptyItemPoints  []geometry.Point // Cached empty points for items. Must be synchronized with the map's itemGrid
	emptyActorPoints []geometry.Point // Cached empty points for actors. Must be synchronized with the map's actorGrid

	emptyItemPointsIndex  map[geometry.Point]int // For swift access
	emptyActorPointsIndex map[geometry.Point]int // For swift access
}

// RoomId - room identification type
type RoomId int64

// InvalidRoomId constant
const (
	InvalidRoomId = 0
)

func NewInvalidPoint() geometry.Point {
	return geometry.Point{X: -1, Y: -1}
}

// DoorMetadata - data of all doors
type DoorMetadata struct {
	Pos     geometry.Point `json:"pos"`     // Door position
	Locked  bool           `json:"locked"`  // Lock state
	Keyhole Keyhole        `json:"keyhole"` // Key color which opens this door
}

//
//
// --- CONSTRUCTORS ---
//
//

func NewMapFromBlueprint(data *MapBlueprint) *Map {
	m := &Map{
		Width:          data.Width,
		Height:         data.Height,
		tileGrid:       data.TileGrid,
		actorGrid:      utils.CreateMatrix[int64](data.Height, data.Width),
		itemGrid:       utils.CreateMatrix[int64](data.Height, data.Width),
		rooms:          data.Rooms,
		doors:          data.Doors,
		EntranceRoomId: data.EntranceRoomId,
		ExitRoomId:     data.ExitRoomId,
		ExitPoint:      data.ExitPoint,
	}

	m.Hydrate()

	return m
}

func (m *Map) Hydrate() {
	m.roomMap = make(map[RoomId]*Room)

	for _, room := range m.rooms {
		m.roomMap[room.Id] = room
		roomSquare := room.Width * room.Height

		room.emptyItemPoints = make([]geometry.Point, 0, roomSquare)
		room.emptyItemPointsIndex = make(map[geometry.Point]int, roomSquare)
		m.RefreshEmptyItemPoints(room)

		room.emptyActorPoints = make([]geometry.Point, 0, roomSquare)
		room.emptyActorPointsIndex = make(map[geometry.Point]int, roomSquare)
		m.RefreshEmptyActorPoints(room)
	}

	exitRoom := m.roomMap[m.ExitRoomId]
	removePoint(m.ExitPoint, &exitRoom.emptyItemPoints, exitRoom.emptyItemPointsIndex)
}

//
//
// --- GETTERS ---
//
//

func (m *Map) GetDimensions() geometry.Point {
	return geometry.Point{X: m.Width, Y: m.Height}
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

	t := m.tileGrid[p.Y][p.X].Type
	return t, true
}

// GetItemID - returns item ID at given position and true if point is in bounds.
func (m *Map) GetItemID(p geometry.Point) (int64, bool) {
	if !m.InBounds(p) {
		return 0, false
	}

	id := m.itemGrid[p.Y][p.X]
	return id, true
}

// GetActorID - returns actor ID at given position and true if point is in bounds.
func (m *Map) GetActorID(p geometry.Point) (int64, bool) {
	if !m.InBounds(p) {
		return 0, false
	}

	id := m.actorGrid[p.Y][p.X]
	return id, true
}

func (m *Map) GetRooms() []*Room {
	return m.rooms
}

func (m *Map) GetRoomCount() int {
	return len(m.rooms)
}

func (m *Map) GetDoors() map[geometry.Point]*DoorMetadata {
	return m.doors
}

func (m *Map) GetDoorKeyhole(door geometry.Point) (Keyhole, bool) {
	if !m.IsClosedDoor(door) {
		return 0, false
	}

	doorMetadata, ok := m.doors[door]
	if !ok {
		return MasterKeyhole, true
	}

	return doorMetadata.Keyhole, true
}

// GetRoomByPoint - returns room by point (like by coordinates)
// Returns (pointer, true) to the room where the given point is located
// Returns (nil, false) if point is outside any room
func (m *Map) GetRoomByPoint(pos geometry.Point) (*Room, bool) {
	if !m.InBounds(pos) {
		return nil, false
	}

	roomId := m.tileGrid[pos.Y][pos.X].RoomId
	if roomId == 0 {
		return nil, false
	}
	return m.roomMap[roomId], true
}

// GetRoomByID - returns room by id
// Returns (pointer, true) to the room with given id
// Returns (nil, false) if the room with given id is not found
func (m *Map) GetRoomByID(id RoomId) (*Room, bool) {
	for _, room := range m.rooms {
		if room.Id == id {
			return room, true
		}
	}

	return nil, false
}

func (r *Room) GetItemCapacity() int {
	return len(r.emptyItemPoints)
}

func (r *Room) GetActorCapacity() int {
	return len(r.emptyActorPoints)
}

//
//
// --- PREDICATES ---
//
//

// InBounds - return true if point in map boundaries.
func (m *Map) InBounds(p geometry.Point) bool {
	return p.X >= 0 && p.X < m.Width && p.Y >= 0 && p.Y < m.Height
}

// IsWalkable - returns true if tile at given position is walkable.
func (m *Map) IsWalkable(p geometry.Point) bool {
	if !m.InBounds(p) {
		return false
	}
	t := m.tileGrid[p.Y][p.X].Type
	return t != Empty && t != Wall && t != ClosedDoor
}

// CanMoveTo - returns true if given p is walkable and free of actors
func (m *Map) CanMoveTo(p geometry.Point) bool {
	room, ok := m.GetRoomByPoint(p)
	if !ok {
		log.Printf("[WARNING] CanMoveTo: No room found for point %v\n", p)
		return false
	}
	_, acceptActors := room.emptyActorPointsIndex[p]
	return m.IsWalkable(p) && !m.IsActor(p) && acceptActors
}

// IsItem - returns true if there is an item in given position and point is in bounds.
func (m *Map) IsItem(p geometry.Point) bool {
	return m.InBounds(p) && m.itemGrid[p.Y][p.X] != 0
}

// IsActor - returns true if there is an actor in given position and point is in bounds.
func (m *Map) IsActor(p geometry.Point) bool {
	return m.InBounds(p) && m.actorGrid[p.Y][p.X] != 0
}

// IsDoor - returns true if the there is an open or closed door in given position and point is in bounds.
func (m *Map) IsDoor(p geometry.Point) bool {
	return m.InBounds(p) && (m.tileGrid[p.Y][p.X].Type == OpenDoor || m.tileGrid[p.Y][p.X].Type == ClosedDoor)
}

// IsOpenDoor - returns true if there is an open door in given position and point is in bounds.
func (m *Map) IsOpenDoor(p geometry.Point) bool {
	return m.InBounds(p) && m.tileGrid[p.Y][p.X].Type == OpenDoor
}

func (m *Map) IsClosedDoor(p geometry.Point) bool {
	return m.InBounds(p) && m.tileGrid[p.Y][p.X].Type == ClosedDoor
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
func (m *Map) SetItem(p geometry.Point, id int64) bool {
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

// SetActor - sets actor position on the map.
// Returns false if it is impossible to set actor in given position (tile is not walkable).
// If given id != 0 also reduces room capacity to locate other actors (affects generation algorithms).
// If given id == 0 increases room capacity so more actors can be generated in that room.
func (m *Map) SetActor(p geometry.Point, id int64) bool {
	if m == nil {
		log.Printf("[ERROR] Can't set actor at %v: map is nil\n", p)
		return false
	}

	if !m.IsWalkable(p) {
		log.Printf("[ERROR] Can't set actor at %v: tile is not walkable\n", p)
		return false
	}

	m.actorGrid[p.Y][p.X] = id
	room, _ := m.GetRoomByPoint(p)

	if id == 0 {
		room.addActorPoint(p)
	} else {
		room.removeActorPoint(p)
	}

	return true
}

// Move - moves actor from src to dst point if possible.
// Returns success or failure of operation.
func (m *Map) Move(src, dst geometry.Point) bool {
	if !m.IsWalkable(src) || !m.IsWalkable(dst) {
		log.Printf("[ERROR] Can't move actor from %v to %v: tile(s) is(are) not walkable\n", src, dst)
		return false
	}

	if m.actorGrid[src.Y][src.X] == 0 {
		log.Printf("[ERROR] Can't move actor from %v to %v: %v is empty\n", src, dst, src)
		return false
	}

	if m.actorGrid[dst.Y][dst.X] != 0 {
		log.Printf("[ERROR] Can't move actor from %v to %v: destination point %v is not empty\n", src, dst, src)
		return false
	}

	m.SetActor(dst, m.actorGrid[src.Y][src.X])
	m.RemoveActor(src)

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
	return m.takeRandomPoint(&r.emptyItemPoints, r.removeItemPoint, rng)
}

// TakeRandomActorPoint - consumes random empty point for actor from given room (item and actor can be together on one cell).
// Returns (random point for actor in room, is there available cell for new actor).
// Generated position are not repeated until they become empty again.
// This function reduces room capacity.
// NOTE: the function does not generate or set id for actor in global map actor grid,
// so the function IsActor(genPoint) returns false until you manually set the ID for generated point through SetActor(genPoint, id).
func (m *Map) TakeRandomActorPoint(r *Room, rng *rand.Rand) (geometry.Point, bool) {
	return m.takeRandomPoint(&r.emptyActorPoints, r.removeActorPoint, rng)
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
	addPoint(p, &r.emptyItemPoints, r.emptyItemPointsIndex)
}

// addActorPoint - adds empty point for actor.
func (r *Room) addActorPoint(p geometry.Point) {
	addPoint(p, &r.emptyActorPoints, r.emptyActorPointsIndex)
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
	removePoint(p, &r.emptyItemPoints, r.emptyItemPointsIndex)
}

// removeActorPoint - removes empty point for actors.
func (r *Room) removeActorPoint(p geometry.Point) {
	removePoint(p, &r.emptyActorPoints, r.emptyActorPointsIndex)
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

func (m *Map) RefreshEmptyItemPoints(room *Room) {
	m.updateRoomEmptyPoints(room, &room.emptyItemPoints, room.emptyItemPointsIndex, m.itemGrid)
}

func (m *Map) RefreshEmptyActorPoints(room *Room) {
	m.updateRoomEmptyPoints(room, &room.emptyActorPoints, room.emptyActorPointsIndex, m.actorGrid)
}

// updateRoomEmptyPoints - finds empty points in the given room
func (m *Map) updateRoomEmptyPoints(room *Room, roomPoints *[]geometry.Point, roomPointsIndex map[geometry.Point]int, grid [][]int64) {
	*roomPoints = (*roomPoints)[:0]
	clear(roomPointsIndex)

	for i := room.Pos.Y; i < room.Pos.Y+room.Height; i++ {
		for j := room.Pos.X; j < room.Pos.X+room.Width; j++ {
			if grid[i][j] == 0 {
				addPoint(geometry.Point{X: j, Y: i}, roomPoints, roomPointsIndex)
			}
		}
	}
}

// TryOpenDoor - tries to open the door by passing door position and available key color.
// Returns false if the given point is not a door or the key is not convenient.
// Returns true if the key color matches with door color or door is already unlocked or always was open.
func (m *Map) TryOpenDoor(p geometry.Point, keyhole Keyhole) bool {
	door, exists := m.doors[p]
	if !exists {
		return false
	}

	if door.Keyhole == 0 {
		return true
	}

	if door.Keyhole == keyhole {
		m.tileGrid[p.Y][p.X].Type = OpenDoor
		door.Locked = false
		door.Keyhole = KeyholeNone
		return true
	}

	return false
}

func (m *Map) OpenDoor(p geometry.Point) {
	if doorMeta, exists := m.doors[p]; exists {
		m.tileGrid[p.Y][p.X].Type = OpenDoor
		doorMeta.Keyhole = KeyholeNone
		doorMeta.Locked = false
	}
}

func (m *Map) LockDoor(p geometry.Point, keyhole Keyhole) {
	if door, exists := m.doors[p]; exists {
		door.Locked = true
		door.Keyhole = keyhole
		m.tileGrid[p.Y][p.X].Type = ClosedDoor
	}
}

//
//
// --- DUNGEON CLEARING ---
//
//

// ClearDungeon  - clears topology, items, actors, doors, keys.
func (m *Map) ClearDungeon() {
	m.ClearTopology()
	m.ClearItems()
	m.ClearActors()
}

// ClearTopology - clears topology and tile grid. Removes entrance and exit.
func (m *Map) ClearTopology() {
	utils.ClearMatrix(m.tileGrid)

	clear(m.rooms)
	m.rooms = m.rooms[:0]
	m.EntranceRoomId = InvalidRoomId
	m.ExitRoomId = InvalidRoomId
	m.ExitPoint = NewInvalidPoint()

	clear(m.doors)
}

// ClearItems - clears all items and updates capacity of all rooms making them empty again.
// Does not remove entrance and exit point.
// Entrance room points will not be available for participation in random point extractions or procedural generations after this function.
func (m *Map) ClearItems() {
	utils.ClearMatrix(m.itemGrid)

	for _, room := range m.rooms {
		if room.Id != m.EntranceRoomId {
			m.updateRoomEmptyPoints(room, &room.emptyItemPoints, room.emptyItemPointsIndex, m.itemGrid)
		} else {
			room.emptyItemPoints = room.emptyItemPoints[:0]
			clear(room.emptyItemPointsIndex)
		}
	}
}

// ClearActors - clears all actors and updates capacity of all rooms making them empty again.
// Entrance room points will not be available for participation in random point extractions or procedural generations after this function anyway.
func (m *Map) ClearActors() {
	utils.ClearMatrix(m.actorGrid)

	for _, room := range m.rooms {
		if room.Id != m.EntranceRoomId {
			m.updateRoomEmptyPoints(room, &room.emptyActorPoints, room.emptyActorPointsIndex, m.actorGrid)
		} else {
			room.emptyActorPoints = room.emptyActorPoints[:0]
			clear(room.emptyActorPointsIndex)
		}
	}
}

//
//
// --- FOR ACTORS AI ---
//
//

func (m *Map) GenerateScentMap(points []geometry.Point) [][]int {
	scentMap := utils.CreateMatrix[int](m.Height, m.Width)
	utils.FillMatrix(scentMap, math.MaxInt)

	queue := points

	for _, p := range points {
		scentMap[p.Y][p.X] = 0
	}

	for len(queue) > 0 {
		cur := queue[0]
		queue = queue[1:]

		for _, dir := range geometry.GetAllDirs() {
			n := cur.Add(dir)

			if m.IsWalkable(n) && scentMap[n.Y][n.X] == math.MaxInt {
				scentMap[n.Y][n.X] = scentMap[cur.Y][cur.X] + 1
				queue = append(queue, n)
			}
		}
	}

	return scentMap
}

//
//
// --- LOOT MANAGEMENT ---
//
//

// FindEmptyPoint - searches for the nearest empty floor tile within a given radius to spawn loot.
// If rad < 0, searches the entire room where the center is located.
// If rad == 0, checks only the specified center point.
// If rad > 0, searches in concentric squares but stays within the room boundaries.
func (m *Map) FindEmptyPoint(center geometry.Point, rad int) (geometry.Point, bool) {
	// Basic validation: center must be walkable and belong to a room
	if !m.IsWalkable(center) {
		log.Printf("[ERROR] FindEmptyPoint - center is not walkable\n")
		return geometry.Point{}, false
	}

	room, ok := m.GetRoomByPoint(center)

	if !ok {
		log.Printf("[ERROR] FindEmptyPoint - given point %v belongs to no room\n", center)
		return geometry.Point{}, false
	}

	// Helper to check if a specific tile is valid for loot
	isValid := func(x, y int) bool {
		return m.tileGrid[y][x].Type == Floor && m.itemGrid[y][x] == 0
	}

	if isValid(center.X, center.Y) {
		return center, true
	}
	if rad == 0 {
		return geometry.Point{}, false
	}

	roomMinX, roomMaxX := room.Pos.X, room.Pos.X+room.Width-1
	roomMinY, roomMaxY := room.Pos.Y, room.Pos.Y+room.Height-1

	limit := rad
	if rad < 0 {
		limit = max(room.Width, room.Height)
	}

	// Concentric square search
	for r := 1; r <= limit; r++ {
		// Calculate boundaries for the current frame (radius)
		curTop, curBot := center.Y-r, center.Y+r
		curLeft, curRight := center.X-r, center.X+r

		minX, maxX := max(curLeft, roomMinX), min(curRight, roomMaxX)
		minY, maxY := max(curTop, roomMinY), min(curBot, roomMaxY)

		// Top and Bottom edges (including corners)
		for i := minX; i <= maxX; i++ {
			if curTop >= roomMinY && isValid(i, curTop) {
				return geometry.Point{X: i, Y: curTop}, true
			}
			if curBot <= roomMaxY && isValid(i, curBot) {
				return geometry.Point{X: i, Y: curBot}, true
			}
		}

		// Left and Right edges
		for j := minY; j <= maxY; j++ {
			if curLeft >= roomMinX && isValid(curLeft, j) {
				return geometry.Point{X: curLeft, Y: j}, true
			}
			if curRight <= roomMaxX && isValid(curRight, j) {
				return geometry.Point{X: curRight, Y: j}, true
			}
		}

		// Optimization: if we are out of room bounds on all sides, stop searching
		if minX == roomMinX && maxX == roomMaxX && minY == roomMinY && maxY == roomMaxY {
			break
		}
	}

	log.Printf("[INFO] FindEmptyPoint - no empty space within radius %v from %v\n", rad, center)
	return geometry.Point{}, false
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

	entranceCenter := m.GetEntranceRoom().Center

	for row := range m.tileGrid {
		for col := range m.tileGrid[row] {
			var ch rune
			tile := m.tileGrid[row][col].Type

			switch tile {
			case Wall:
				ch = '#'
			case Floor:
				ch = '.'
			case OpenDoor:
				ch = '/'
			case ClosedDoor:
				ch = '%'
			case Corridor:
				ch = '*'
			case Exit:
				ch = '>'
			default:
				ch = ' '
			}

			itemID := m.itemGrid[row][col]
			if itemID != 0 {
				ch = 'I'
			} else if m.actorGrid[row][col] != 0 {
				ch = 'A'
			}

			p := geometry.Point{X: col, Y: row}
			if p == entranceCenter {
				ch = '@'
			}

			sb.WriteRune(ch)
		}
		sb.WriteRune('\n')
	}

	return sb.String()
}

func (r *Room) GetEmptyItemPoints() []geometry.Point {
	return r.emptyItemPoints
}

//
//
// --- GETTERS FOR DTO ---
//
//

func (m *Map) GetTileGrid() [][]Cell {
	return m.tileGrid
}

func (m *Map) GetItemGrid() [][]int64 {
	return m.itemGrid
}

func (m *Map) GetActorGrid() [][]int64 {
	return m.actorGrid
}
