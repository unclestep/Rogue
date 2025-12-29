// Package geometry содержит обьект точки в 2D пространстве.
// Point - структура точки с координатами X и Y.
// - String() string - строковое представление точки.
package geometry

import (
	"fmt"
)

// Point - структура точки с координатами X и Y.
type Point struct {
	X, Y int
}

// Почему p Point, а не *Point?

// String - строковое представление точки.
func (p Point) String() string {
	return fmt.Sprintf("(%v, %v)", p.X, p.Y)
}
