package entity

import (
	"github.com/Nikolay-Yakunin/gouge/internal/pkg/geometry"
	"log"
	"math/rand"
)

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

type Map struct {
	width, height  int
	tiles          [][]Cell
	actorGrid      [][]int
	itemGrid       [][]int
	entrance_point geometry.Point
	exit_point     geometry.Point
	rooms          [][]Room
	entrance_room  *Room
	exit_room      *Room
}

func createMatrix[T any](rows, cols int) [][]T {
	matrix := make([][]T, rows)
	for i := range matrix {
		matrix[i] = make([]T, cols)
	}
	return matrix
}

func NewCustomMap(width, height int) *Map {
	return &Map{
		width:          width,
		height:         height,
		tiles:          createMatrix[Cell](height, width),
		actorGrid:      createMatrix[int](height, width),
		itemGrid:       createMatrix[int](height, width),
		entrance_point: geometry.Point{X: 0, Y: 0},
		exit_point:     geometry.Point{X: 0, Y: 0},
	}
}

func NewDefaultMap() *Map {
	return NewCustomMap(80, 24)
}

func (m *Map) GetEntrance() geometry.Point {
	return m.entrance_point
}

func (m *Map) GetExit() geometry.Point {
	return m.exit_point
}

func (m *Map) InBounds(pos geometry.Point) bool {
	return pos.X >= 0 && pos.X < m.width && pos.Y >= 0 && pos.Y < m.height
}

func (m *Map) IsWalkable(pos geometry.Point) bool {
	if !m.InBounds(pos) {
		return false
	}
	t := m.tiles[pos.Y][pos.X].Type
	return t != Empty && t != Wall
}

func (m *Map) GetActorID(pos geometry.Point) (int, bool) {
	if !m.InBounds(pos) {
		return 0, false
	}

	id := m.actorGrid[pos.Y][pos.X]
	return id, id != 0
}

func (m *Map) GetItemID(pos geometry.Point) (int, bool) {
	if !m.InBounds(pos) {
		return 0, false
	}

	id := m.itemGrid[pos.Y][pos.X]
	return id, id != 0
}

func (m *Map) SetActor(pos geometry.Point, id int) {
	if !m.IsWalkable(pos) {
		log.Printf("[ERROR] Can't set actor at %v: tile is not walkable\n", pos)
		return
	}
	m.actorGrid[pos.Y][pos.X] = id
}

func (m *Map) SetItem(pos geometry.Point, id int) {
	if !m.IsWalkable(pos) {
		log.Printf("[ERROR] Can't set item at %v: tile is not walkable\n", pos)
		return
	}
	m.itemGrid[pos.Y][pos.X] = id
}

type Sector struct {
	xMin, yMin, xMax, yMax int
}

type Room struct {
	id            int
	pos           geometry.Point
	width, height int
	center        geometry.Point
}

// Function creates puzzle layout: every sector has different size
// k stands for x-dimension, n stands for y-dimension
func (m *Map) sectorization(k, n int) [][]Sector {
	sectors := createMatrix[Sector](n, k)
	sector_width, sector_width_rem := m.width/k, m.width%k
	sector_height, sector_height_rem := m.height/n, m.height%n

	height_rem_limit := make([]int, k)
	for h := range height_rem_limit {
		height_rem_limit[h] = sector_height_rem
	}
	y_mins := make([]int, k)

	margin := 2

	for row := range n {
		width_rem_limit := sector_width_rem
		x_min := 0
		for col := range k {
			var rand_height_rem_add, y_max int
			if row == n-1 { // For last row just add limit's remainder without randomization
				rand_height_rem_add = height_rem_limit[col]
				y_max = y_mins[col] + sector_height + rand_height_rem_add
			} else {
				rand_height_rem_add = rand.Intn(height_rem_limit[col] + 1)
				y_max = y_mins[col] + sector_height + rand_height_rem_add - margin
			}
			height_rem_limit[col] -= rand_height_rem_add

			var rand_width_rem_add, x_max int
			if col == k-1 { // For last col just add limit's remainder without randomization
				rand_width_rem_add = width_rem_limit
				x_max = x_min + sector_width + rand_width_rem_add
			} else {
				rand_width_rem_add = rand.Intn(width_rem_limit + 1)
				x_max = x_min + sector_width + rand_width_rem_add - margin
			}
			width_rem_limit -= rand_width_rem_add

			sectors[row][col] = Sector{xMin: x_min, xMax: x_max, yMin: y_mins[col], yMax: y_max}
			x_min = x_max + margin
			y_mins[col] = y_max + margin
		}
	}

	return sectors
}

func (m *Map) createRooms(sectors [][]Sector) {
	min_height, min_width := 3, 3

	for row := range len(sectors) {
		for col := range len(sectors[row]) {
			y_max, y_min := sectors[row][col].yMax, sectors[row][col].yMin
			x_max, x_min := sectors[row][col].xMax, sectors[row][col].xMin

			// Pick random height and width
			sector_height := y_max - y_min
			sector_width := x_max - x_min
			room_height := min_height + rand.Intn(sector_height-min_height+1)
			room_width := min_width + rand.Intn(sector_width-min_width+1)

			// Pick random pos (left-upper corner)
			allowed_y_max := y_max - room_height
			allowed_x_max := x_max - room_width
			pos_x := x_min + rand.Intn(allowed_x_max-x_min+1)
			pos_y := y_min + rand.Intn(allowed_y_max-y_min+1)

			// Assemble
			m.rooms[row][col] = Room{
				id:     row*len(sectors[row]) + col,
				pos:    geometry.Point{X: pos_x, Y: pos_y},
				width:  room_width,
				height: room_height,
				center: geometry.Point{
					X: pos_x + room_width/2,
					Y: pos_y + room_height/2,
				},
			}
		}
	}

	m.fillTiles()
}

func (m *Map) fillTiles() {
	for _, row := range m.rooms {
		for _, room := range row {
			m.drawRoom(room)
		}
	}
}

func (m *Map) drawRoom(room Room) {
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

func (m *Map) pickRandomPointInRoom(room Room) geometry.Point {
	inner_x := room.pos.X + 1
	inner_width := room.width - 2
	inner_y := room.pos.Y + 1
	inner_height := room.height - 2

	x := inner_x + rand.Intn(inner_width)
	y := inner_y + rand.Intn(inner_height)

	return geometry.Point{X: x, Y: y}
}

func (m *Map) connectRooms() {
	rooms_ptr := make([]*Room, len(m.rooms)*len(m.rooms[0]))

	for row := range m.rooms {
		for col := range m.rooms[row] {
			rooms_ptr = append(rooms_ptr, &m.rooms[row][col])
		}
	}

	// Entrance and exit room position on the map is always random
	rand.Shuffle(len(rooms_ptr), func(i, j int) {
		rooms_ptr[i], rooms_ptr[j] = rooms_ptr[j], rooms_ptr[i]
	})

	m.entrance_room, m.exit_room = rooms_ptr[0], rooms_ptr[len(rooms_ptr)-1]
	m.entrance_point = m.pickRandomPointInRoom(*m.entrance_room)
	m.exit_point = m.pickRandomPointInRoom(*m.exit_room)
	m.tiles[m.exit_point.Y][m.exit_point.X].Type = Exit

	for i := 0; i < len(rooms_ptr)-2; i++ {
		m.digCorridor(rooms_ptr[0].center, rooms_ptr[1].center)
	}
}

func (m *Map) digCorridorX(initial geometry.Point, step, distance int) {
	x, y := initial.X, initial.Y

	for i := 0; i <= distance; i++ {
		tile := &m.tiles[y][x]

		if tile.Type == Empty {
			tile.Type = Corridor
		} else if tile.Type == Wall {
			prev_x := x - step
			next_x := x + step

			if (m.InBounds(geometry.Point{X: prev_x, Y: y}) && m.tiles[y][prev_x].Type == Floor) || (m.InBounds(geometry.Point{X: next_x, Y: y}) && m.tiles[y][next_x].Type == Floor) {
				tile.Type = Door
			}
		}

		x += step
	}
}

func (m *Map) digCorridorY(initial geometry.Point, step, distance int) {
	x, y := initial.X, initial.Y

	for i := 0; i <= distance; i++ {
		tile := &m.tiles[y][x]

		if tile.Type == Empty {
			tile.Type = Corridor
		} else if tile.Type == Wall {
			prev_y := y - step
			next_y := y + step

			if (m.InBounds(geometry.Point{X: x, Y: prev_y}) && m.tiles[prev_y][x].Type == Floor) || (m.InBounds(geometry.Point{X: x, Y: next_y}) && m.tiles[next_y][x].Type == Floor) {
				tile.Type = Door
			}
		}

		y += step
	}
}

func (m *Map) digCorridor(point1, point2 geometry.Point) {
	distance_x, distance_y := point2.X-point1.X, point2.Y-point1.Y
	step_x, step_y := 1, 1
	if distance_x < 0 {
		step_x = -1
		distance_x = -distance_x
	}
	if distance_y < 0 {
		step_y = -1
		distance_y = -distance_y
	}

	if rand.Intn(2) == 0 {
		m.digCorridorX(point1, step_x, distance_x)
		m.digCorridorY(geometry.Point{X: point2.X, Y: point1.Y}, step_y, distance_y)
	} else {
		m.digCorridorY(point1, step_y, distance_y)
		m.digCorridorX(geometry.Point{X: point1.X, Y: point2.Y}, step_x, distance_x)
	}
}

func (m *Map) GenerateLevel(k, n int) {
	sectors := m.sectorization(k, n)
	m.createRooms(sectors)
	m.connectRooms()
}
