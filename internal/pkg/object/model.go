// Package object содержит базовую можель обьекта с координатами.
// Координаты и размер обьекта представлены структурой Coords.
// Обьект реализует интерфейс.
// - SetCoords(x, y int) - установка координат обьекта.
// - SetSize(sizeX, sizeY int) - установка размера обьекта.
package object

import "fmt"

// Coords - структура координат обьекта на игровом поле.
// В оригинале реализуется через enum и массив, для "динамического" набора координат.
type Coords struct {
	X int `json:"x"`
	Y int `json:"y"`
	// CoordsNum int // Количество координат, для добавления большего числа измерений.
}

// Object - базовая модель обьекта с координатами и размером.
type Object struct {
	Pos  Coords `json:"Pos"`  // Верхний левый угол обьекта.
	Size Coords `json:"Size"` // Размер обьекта по X и Y.
}

// New - создание нового обьекта с координатами и размером.
func New(x, y, sizeX, sizeY int) *Object {
	return &Object{
		Pos:  Coords{X: x, Y: y},
		Size: Coords{X: sizeX, Y: sizeY},
	}
}

// Interface - интерфейс обьекта с координатами и размером.
type Interface interface {
	SetCoords(x, y int)
	SetSize(sizeX, sizeY int) // Потенциально бесполезное, но можно попробовать добавить зелье изменениея размера, к примеру.
}

// SetCoords - установка координат обьекта.
func (o *Object) SetCoords(x, y int) {
	o.Pos.X = x
	o.Pos.Y = y
}

// SetSize - установка размера обьекта.
func (o *Object) SetSize(sizeX, sizeY int) {
	o.Size.X = sizeX
	o.Size.Y = sizeY
}

// String - строковое представление координат для Sprintf.
func (c Coords) String() string {
	return fmt.Sprintf("[%v, %v]", c.X, c.Y)
}

// String - строковое представление обьекта для Sprintf.
func (o *Object) String() string {
	return fmt.Sprintf("Object{Pos: %v, Size: %v}", o.Pos, o.Size)
}
