// Package entity содержит сущности, используемые в доменной логике приложения.
// - map - сущность карты игрового мира, содержащая информацию о тайлах, акторах и предметах на карте.
package entity

import (
	"log"

	"github.com/Nikolay-Yakunin/gouge/internal/pkg/geometry"
)

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
	entrance      geometry.Point
	exit          geometry.Point
}

// createMatrix - инициализация слайса заданного размера и типа.
func createMatrix[T any](rows, cols int) [][]T {
	matrix := make([][]T, rows)
	for i := range matrix {
		matrix[i] = make([]T, cols)
	}
	return matrix
}

// NewCustomMap - конструктор карты.
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

// NewDefaultMap - конструктор карты с размерами по умолчанию.
func NewDefaultMap() *Map {
	return NewCustomMap(80, 24)
}

// GetEntrance - получение координат входа на карту.
func (m *Map) GetEntrance() geometry.Point {
	return m.entrance
}

// GetExit - получение координат выхода с карты.
func (m *Map) GetExit() geometry.Point {
	return m.exit
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
	// TODO: Пофикси этот хардкод:)
	return m.tiles[pos.Y][pos.X].Type == Floor || m.tiles[pos.Y][pos.X].Type == Corridor || m.tiles[pos.Y][pos.X].Type == Door || m.tiles[pos.Y][pos.X].Type == Exit
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
