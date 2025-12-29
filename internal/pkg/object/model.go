// Package object содержит базовую можель обьекта с координатами.
// Координаты и размер обьекта представлены структурой Coords.
// Обьект реализует интерфейс ObjectInterface.
// - Coords() Coords - получение координат обьекта.
// - Size() Coords - получение размера обьекта.
// - SetCoords(x, y int) - установка координат обьекта.
// - SetSize(sizeX, sizeY int) - установка размера обьекта.
package object

// Coords - структура координат обьекта на игровом поле.
// В оригинале реализуется через enum и массив, для "динамического" набора координат.
type Coords struct {
	X int
	Y int
	// CoordsNum int // Количество координат, для добавления большего числа измерений.
}

// Object - базовая модель обьекта с координатами и размером.
type Object struct {
	pos  Coords // Верхний левый угол обьекта.
	size Coords // Размер обьекта по X и Y.
}

// New - создание нового обьекта с координатами и размером.
func New(x, y, sizeX, sizeY int) *Object {
	return &Object{
		pos:  Coords{X: x, Y: y},
		size: Coords{X: sizeX, Y: sizeY},
	}
}

// Interface - интерфейс обьекта с координатами и размером.
type Interface interface {
	Coords() Coords
	Size() Coords
	SetCoords(x, y int)
	SetSize(sizeX, sizeY int) // Потенциально бесполезное, но можно попробовать добавить зелье изменениея размера, к примеру.
}

// Coords - получение координат обьекта.
func (o *Object) Coords() Coords {
	return o.pos
}

// Size - получение размера обьекта.
func (o *Object) Size() Coords {
	return o.size
}

// SetCoords - установка координат обьекта.
func (o *Object) SetCoords(x, y int) {
	o.pos.X = x
	o.pos.Y = y
}

// SetSize - установка размера обьекта.
func (o *Object) SetSize(sizeX, sizeY int) {
	o.size.X = sizeX
	o.size.Y = sizeY
}
