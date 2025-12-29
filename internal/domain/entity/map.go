package entity

import (
	"github.com/Nikolay-Yakunin/gouge/internal/pkg/geometry"
	"log"
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
	width, height int
	tiles         [][]Cell
	actorGrid     [][]int
	itemGrid      [][]int
	entrance      geometry.Point
	exit          geometry.Point
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
		width:     width,
		height:    height,
		tiles:     createMatrix[Cell](height, width),
		actorGrid: createMatrix[int](height, width),
		itemGrid:  createMatrix[int](height, width),
		entrance:  geometry.Point{X: 0, Y: 0},
		exit:      geometry.Point{X: 0, Y: 0},
	}
}

func NewDefaultMap() *Map {
	return NewCustomMap(80, 24)
}

func (m *Map) GetEntrance() geometry.Point {
	return m.entrance
}

func (m *Map) GetExit() geometry.Point {
	return m.exit
}

func (m *Map) InBounds(pos geometry.Point) bool {
	return pos.X >= 0 && pos.X < m.width && pos.Y >= 0 && pos.Y < m.height
}

func (m *Map) IsWalkable(pos geometry.Point) bool {
	if !m.InBounds(pos) {
		return false
	}
	return m.tiles[pos.Y][pos.X].Type == Floor || m.tiles[pos.Y][pos.X].Type == Corridor || m.tiles[pos.Y][pos.X].Type == Door
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
