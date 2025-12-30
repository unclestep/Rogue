// Package entity содержит сущности, используемые в доменной логике приложения.
// - map - сущность карты игрового мира, содержащая информацию о тайлах, акторах и предметах на карте.
package entity

import (
	"github.com/Nikolay-Yakunin/gouge/internal/pkg/geometry"
	"log"
	"math/rand"
)

// Map - сущность карты игрового мира.
// - width, height - размеры карты.
// - tiles - двумерный слайс ячеек карты.
// - actorGrid - двумерный слайс идентификаторов акторов на карте.
// - itemGrid - двумерный слайс идентификаторов предметов на карте.
// - entrance - координаты входа на карту.
// - exit - координаты выхода с карты.
type Map struct {
	width, height int
	tiles         [][]Cell
	actorGrid     [][]int
	itemGrid      [][]int
	entrancePoint geometry.Point
	exitPoint     geometry.Point
	rooms         []*Room
	entranceRoom  *Room
	exitRoom      *Room
}

// TileType - тип кода тайла на карте.
type TileType int

// Определение типов тайлов.
const (
	Empty TileType = iota
	Wall
	Floor
	Door
	Corridor
	Exit
)

// VisibilityState - тип кода видимости тайла на карте.
type VisibilityState int

// Определение состояний видимости.
const (
	Unexplored VisibilityState = iota
	Explored
	Visible
)

// Cell - структура ячейки карты, содержащая тип тайла и состояние видимости.
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
// NewCustomMap - конструктор карты.
func NewCustomMap(width, height int) *Map {
	return &Map{
		width:         width,
		height:        height,
		tiles:         createMatrix[Cell](height, width),
		actorGrid:     createMatrix[int](height, width),
		itemGrid:      createMatrix[int](height, width),
		entrancePoint: geometry.Point{X: 0, Y: 0},
		exitPoint:     geometry.Point{X: 0, Y: 0},
	}
}

// NewDefaultMap - конструктор карты с размерами по умолчанию.
func NewDefaultMap() *Map {
	return NewCustomMap(80, 24)
}

// GetEntrance - получение координат входа на карту.
func (m *Map) GetEntrancePoint() geometry.Point {
	return m.entrancePoint
}

// GetExit - получение координат выхода с карты.
func (m *Map) GetExitPoint() geometry.Point {
	return m.exitPoint
}

func (m *Map) GetEntranceRoom() *Room {
	return m.entranceRoom
}

func (m *Map) GetExitRoom() *Room {
	return m.exitRoom
}

// InBounds - проверка, находятся ли координаты внутри границ карты.
func (m *Map) InBounds(pos geometry.Point) bool {
	return pos.X >= 0 && pos.X < m.width && pos.Y >= 0 && pos.Y < m.height
}

// IsWalkable - проверка, можно ли пройти по тайлу на заданных координатах.
func (m *Map) IsWalkable(pos geometry.Point) bool {
	if !m.InBounds(pos) {
		return false
	}
	t := m.tiles[pos.Y][pos.X].Type
	return t != Empty && t != Wall
}

// GetActorID - получение идентификатора актора на заданных координатах.
func (m *Map) GetActorID(pos geometry.Point) (int, bool) {
	if !m.InBounds(pos) {
		return 0, false
	}

	id := m.actorGrid[pos.Y][pos.X]
	return id, id != 0
}

// GetItemID - получение идентификатора предмета на заданных координатах.
func (m *Map) GetItemID(pos geometry.Point) (int, bool) {
	if !m.InBounds(pos) {
		return 0, false
	}

	id := m.itemGrid[pos.Y][pos.X]
	return id, id != 0
}

// SetActor - установка идентификатора актора на заданных координатах.
func (m *Map) SetActor(pos geometry.Point, id int) {
	if !m.IsWalkable(pos) {
		log.Printf("[ERROR] Can't set actor at %v: tile is not walkable\n", pos)
		return
	}
	m.actorGrid[pos.Y][pos.X] = id
}

// SetItem - установка идентификатора предмета на заданных координатах.
func (m *Map) SetItem(pos geometry.Point, id int) {
	if !m.IsWalkable(pos) {
		log.Printf("[ERROR] Can't set item at %v: tile is not walkable\n", pos)
		return
	}
	m.itemGrid[pos.Y][pos.X] = id
}

func (m *Map) GenerateLevel(k, n int) bool {
	if !m.canFitGrid(k, n) {
		return false
	}
	sectors := m.sectorization(k, n)
	m.createRooms(sectors)
	m.connectRooms()
	return true
}

func (m *Map) GetRoom(pos geometry.Point) (*Room, bool) {
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

func (m *Map) GetRoomPos(room *Room) (geometry.Point, bool) {
	if room == nil {
		return geometry.Point{}, false
	}
	return room.pos, true
}

func (m *Map) GetRoomHW(room *Room) (int, int, bool) {
	if room == nil {
		return 0, 0, false
	}
	return room.height, room.width, true
}

// PRIVATE
// createMatrix - инициализация слайса заданного размера и типа.
func createMatrix[T any](rows, cols int) [][]T {
	matrix := make([][]T, rows)
	for i := range matrix {
		matrix[i] = make([]T, cols)
	}
	return matrix
}

func (m *Map) canFitGrid(k, n int) bool {
	if k <= 0 || n <= 0 {
		return false
	}

	widthMarigns, heightMargins := k-1, n-1
	minAllowedMapWidth := k*MinRoomSize + MarginBetweenSectors*widthMarigns
	minAllowedMapHeight := n*MinRoomSize + MarginBetweenSectors*heightMargins

	if m.width < minAllowedMapWidth || m.height < minAllowedMapHeight {
		return false
	}

	return true
}

// Function creates puzzle layout: every sector has different size
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
				randHeightRemAdd = rand.Intn(heightRemLimit[col] + 1)
				yMax = yMins[col] + sectorHeight + randHeightRemAdd - MarginBetweenSectors
			}
			heightRemLimit[col] -= randHeightRemAdd

			var randWidthRemAdd, xMax int
			if col == k-1 { // For last col just add limit's remainder without randomization
				randWidthRemAdd = widthRemLimit
				xMax = xMin + sectorWidth + randWidthRemAdd
			} else {
				randWidthRemAdd = rand.Intn(widthRemLimit + 1)
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
			roomHeight := minHeight + rand.Intn(sectorHeight-minHeight+1)
			roomWidth := minWidth + rand.Intn(sectorWidth-minWidth+1)

			// Pick random pos (left-upper corner)
			allowedYMax := yMax - roomHeight
			allowedXMax := xMax - roomWidth
			posX := xMin + rand.Intn(allowedXMax-xMin+1)
			posY := yMin + rand.Intn(allowedYMax-yMin+1)

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

	m.fillTiles()
}

func (m *Map) fillTiles() {
	for _, room := range m.rooms {
		m.drawRoom(*room)
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

func (m *Map) connectRooms() {
	// Entrance and exit room position on the map is always random
	rand.Shuffle(len(m.rooms), func(i, j int) {
		m.rooms[i], m.rooms[j] = m.rooms[j], m.rooms[i]
	})

	m.entranceRoom, m.exitRoom = m.rooms[0], m.rooms[len(m.rooms)-1]
	m.entrancePoint = m.pickRandomPointInRoom(*m.entranceRoom)
	m.exitPoint = m.pickRandomPointInRoom(*m.exitRoom)
	m.tiles[m.exitPoint.Y][m.exitPoint.X].Type = Exit

	for i := 0; i < len(m.rooms)-1; i++ {
		m.digCorridor(m.rooms[i].center, m.rooms[i+1].center)
	}
}

func (m *Map) pickRandomPointInRoom(room Room) geometry.Point {
	innerX := room.pos.X + 1
	innerWidth := room.width - 2
	innerY := room.pos.Y + 1
	innerHeight := room.height - 2

	x := innerX + rand.Intn(innerWidth)
	y := innerY + rand.Intn(innerHeight)

	return geometry.Point{X: x, Y: y}
}

func (m *Map) digCorridor(point1, point2 geometry.Point) {
	distanceX, distanceY := point2.X-point1.X, point2.Y-point1.Y
	stepX, stepY := 1, 1
	if distanceX < 0 {
		stepX = -1
		distanceX = -distanceX
	}
	if distanceY < 0 {
		stepY = -1
		distanceY = -distanceY
	}

	if rand.Intn(2) == 0 {
		m.digCorridorX(point1, stepX, distanceX)
		m.digCorridorY(geometry.Point{X: point2.X, Y: point1.Y}, stepY, distanceY)
	} else {
		m.digCorridorY(point1, stepY, distanceY)
		m.digCorridorX(geometry.Point{X: point1.X, Y: point2.Y}, stepX, distanceX)
	}
}

func (m *Map) digCorridorX(initial geometry.Point, step, distance int) {
	x, y := initial.X, initial.Y

	for i := 0; i <= distance; i++ {
		tile := &m.tiles[y][x]

		if tile.Type == Empty {
			tile.Type = Corridor
		} else if tile.Type == Wall {
			prevX := x - step
			nextX := x + step

			if (m.InBounds(geometry.Point{X: prevX, Y: y}) && m.tiles[y][prevX].Type == Floor) || (m.InBounds(geometry.Point{X: nextX, Y: y}) && m.tiles[y][nextX].Type == Floor) {
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
			prevY := y - step
			nextY := y + step

			if (m.InBounds(geometry.Point{X: x, Y: prevY}) && m.tiles[prevY][x].Type == Floor) || (m.InBounds(geometry.Point{X: x, Y: nextY}) && m.tiles[nextY][x].Type == Floor) {
				tile.Type = Door
			}
		}

		y += step
	}
}
