// Map stores information about game landscape, actors and items
package entity

import (
	"github.com/Nikolay-Yakunin/gouge/internal/pkg/algorithm"
	"github.com/Nikolay-Yakunin/gouge/internal/pkg/geometry"
	"log"
	"math/rand"
	"strings"
)

// DATA STRUCTURES
type Map struct {
	width, height int            // Cols and rows
	tiles         [][]Cell       // Layer 1: Game landscape
	actorGrid     [][]int        // Layer 2: Location of actors
	itemGrid      [][]int        // Layer 3: Location of items
	entrancePoint geometry.Point // Spawn point
	exitPoint     geometry.Point // End of level point
	rooms         []*Room        // Pointers to all level rooms
	entranceRoom  *Room          // Pointer to room with spawn point
	exitRoom      *Room          // Pointer to room with exit point
	rndm          *rand.Rand     // Local random generator (for representative tests)
	seed          int64          // Seed of local random generator
}

type TileType int

const (
	Empty TileType = iota
	Wall
	Floor
	Door
	Corridor
	Exit
)

type VisibilityState int

const (
	Unexplored VisibilityState = iota
	Explored
	Visible
)

type Cell struct {
	Type       TileType
	Visibility VisibilityState
}

type Sector struct {
	xMin, yMin, xMax, yMax int
}

const (
	MarginBetweenSectors = 2 // Right margin of one room + left margin of second room
)

type Room struct {
	id            int
	pos           geometry.Point
	width, height int
	center        geometry.Point
}

const (
	MinRoomSize = 3 // Wall + Floor + Wall
)

// API

func (m *Map) String() string {
	var sb strings.Builder

	for row := range m.tiles {
		for col := range m.tiles[row] {
			var ch byte
			tile := m.tiles[row][col].Type

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
				ch = 'E'
			default:
				ch = ' '
			}

			if row == m.entrancePoint.Y && col == m.entrancePoint.X {
				ch = 'S'
			}

			sb.WriteByte(ch)
		}
		sb.WriteByte('\n')
	}

	return sb.String()
}

// Custom map constructor
func NewCustomMap(width, height int) *Map {
	m := &Map{
		width:         width,
		height:        height,
		tiles:         createMatrix[Cell](height, width),
		actorGrid:     createMatrix[int](height, width),
		itemGrid:      createMatrix[int](height, width),
		entrancePoint: geometry.Point{X: 0, Y: 0},
		exitPoint:     geometry.Point{X: 0, Y: 0},
	}
	m.SetSeed(rand.Int63())

	return m
}

// Map constructor with predefined width = 80 and height = 24
func NewDefaultMap() *Map {
	return NewCustomMap(80, 24)
}

func (m *Map) GetEntrancePoint() geometry.Point {
	return m.entrancePoint
}

func (m *Map) GetExitPoint() geometry.Point {
	return m.exitPoint
}

func (m *Map) GetEntranceRoom() *Room {
	return m.entranceRoom
}

func (m *Map) GetExitRoom() *Room {
	return m.exitRoom
}

func (m *Map) InBounds(p geometry.Point) bool {
	return p.X >= 0 && p.X < m.width && p.Y >= 0 && p.Y < m.height
}

func (m *Map) IsWalkable(p geometry.Point) bool {
	if !m.InBounds(p) {
		return false
	}
	t := m.tiles[p.Y][p.X].Type
	return t != Empty && t != Wall
}

func (m *Map) GetActorID(p geometry.Point) (int, bool) {
	if !m.InBounds(p) {
		return 0, false
	}

	id := m.actorGrid[p.Y][p.X]
	return id, id != 0
}

func (m *Map) GetItemID(pos geometry.Point) (int, bool) {
	if !m.InBounds(pos) {
		return 0, false
	}

	id := m.itemGrid[pos.Y][pos.X]
	return id, id != 0
}

// Pentalty function for A* algorithm
func (m *Map) GetCost(p geometry.Point) float64 {
	tile := m.tiles[p.Y][p.X].Type
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
				nt := m.tiles[n.Y][n.X].Type
				if nt == Wall || nt == Corridor {
					cost += 5.0
				}
			}
		}
	}

	return cost
}

// Returns points within the boundaries to the right, below, left and above given point
// Digging = false does not return walls
func (m *Map) GetNeighbors(p geometry.Point, digging bool) []geometry.Point {
	valid := make([]geometry.Point, 0, 4)

	for _, n := range getNeighborList(p) {
		if m.InBounds(n) {
			following := !digging && m.IsWalkable(n)

			if digging || following {
				valid = append(valid, n)
			}
		}
	}

	return valid
}

func (m *Map) SetActor(p geometry.Point, id int) {
	if !m.IsWalkable(p) {
		log.Printf("[ERROR] Can't set actor at %v: tile is not walkable\n", p)
		return
	}
	m.actorGrid[p.Y][p.X] = id
}

func (m *Map) SetItem(p geometry.Point, id int) {
	if !m.IsWalkable(p) {
		log.Printf("[ERROR] Can't set item at %v: tile is not walkable\n", p)
		return
	}
	m.itemGrid[p.Y][p.X] = id
}

func (m *Map) SetSeed(seed int64) {
	m.seed = seed
	m.rndm = rand.New(rand.NewSource(seed))
}

func (m *Map) GetSeed() int64 {
	return m.seed
}

// Generates rooms and corridors between them using given grid
// k=cols, n=rows, k*n=rooms
// Grid must be positive
// Too big grid lead to error
func (m *Map) GenerateLevel(k, n int) bool {
	if !m.canFitGrid(k, n) {
		return false
	}
	sectors := m.sectorization(k, n)
	m.createRooms(sectors)
	m.connectRooms()
	return true
}

func (m *Map) GetRoomsCount() int {
	return len(m.rooms)
}

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

func (r *Room) GetID() int {
	return r.id
}

func (r *Room) GetPos() geometry.Point {
	return r.pos
}

func (r *Room) GetHW() (int, int) {
	return r.height, r.width
}

func (r *Room) GetCenter() geometry.Point {
	return r.center
}

// PRIVATE
func createMatrix[T any](rows, cols int) [][]T {
	matrix := make([][]T, rows)
	for i := range matrix {
		matrix[i] = make([]T, cols)
	}
	return matrix
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
				randHeightRemAdd = m.rndm.Intn(heightRemLimit[col] + 1)
				yMax = yMins[col] + sectorHeight + randHeightRemAdd - MarginBetweenSectors
			}
			heightRemLimit[col] -= randHeightRemAdd

			var randWidthRemAdd, xMax int
			if col == k-1 { // For last col just add limit's remainder without randomization
				randWidthRemAdd = widthRemLimit
				xMax = xMin + sectorWidth + randWidthRemAdd
			} else {
				randWidthRemAdd = m.rndm.Intn(widthRemLimit + 1)
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

	for row := range rows {
		for col := range cols {
			yMax, yMin := sectors[row][col].yMax, sectors[row][col].yMin
			xMax, xMin := sectors[row][col].xMax, sectors[row][col].xMin

			// Pick random height and width
			sectorHeight := yMax - yMin
			sectorWidth := xMax - xMin
			roomHeight := minHeight + m.rndm.Intn(sectorHeight-minHeight+1)
			roomWidth := minWidth + m.rndm.Intn(sectorWidth-minWidth+1)

			// Pick random pos (left-upper corner)
			allowedYMax := yMax - roomHeight
			allowedXMax := xMax - roomWidth
			posX := xMin + m.rndm.Intn(allowedXMax-xMin+1)
			posY := yMin + m.rndm.Intn(allowedYMax-yMin+1)

			// Assemble
			m.rooms[row*cols+col] = &Room{
				id:     row*cols + col,
				pos:    geometry.Point{X: posX, Y: posY},
				width:  roomWidth,
				height: roomHeight,
				center: geometry.Point{
					X: posX + roomWidth/2,
					Y: posY + roomHeight/2,
				},
			}
		}
	}

	m.drawRooms()
}

func (m *Map) drawRooms() {
	for _, room := range m.rooms {
		for y := room.pos.Y; y < room.pos.Y+room.height; y++ {
			for x := room.pos.X; x < room.pos.X+room.width; x++ {
				if y == room.pos.Y || y == room.pos.Y+room.height-1 || x == room.pos.X || x == room.pos.X+room.width-1 {
					m.tiles[y][x].Type = Wall
				} else {
					m.tiles[y][x].Type = Floor
				}
			}
		}
	}
}

func (m *Map) connectRooms() {
	// Entrance and exit room position on the map is always random
	m.rndm.Shuffle(len(m.rooms), func(i, j int) {
		m.rooms[i], m.rooms[j] = m.rooms[j], m.rooms[i]
	})

	m.entranceRoom, m.exitRoom = m.rooms[0], m.rooms[len(m.rooms)-1]
	m.entrancePoint = m.pickRandomPointInRoom(*m.entranceRoom)
	m.exitPoint = m.pickRandomPointInRoom(*m.exitRoom)
	m.tiles[m.exitPoint.Y][m.exitPoint.X].Type = Exit

	for i := 0; i < len(m.rooms)-1; i++ {
		path := algorithm.FindPath(m, m.rooms[i].center, m.rooms[i+1].center, true)
		m.drawCorridors(path)
	}
}

func (m *Map) drawCorridors(path []geometry.Point) {
	for _, p := range path {
		x, y := p.X, p.Y
		tile := &m.tiles[y][x]

		switch tile.Type {
		case Empty:
			tile.Type = Corridor
		case Wall:
			tile.Type = Door
		}
	}
}

func (m *Map) pickRandomPointInRoom(room Room) geometry.Point {
	innerX := room.pos.X + 1
	innerWidth := room.width - 2
	innerY := room.pos.Y + 1
	innerHeight := room.height - 2

	x := innerX + m.rndm.Intn(innerWidth)
	y := innerY + m.rndm.Intn(innerHeight)

	return geometry.Point{X: x, Y: y}
}
