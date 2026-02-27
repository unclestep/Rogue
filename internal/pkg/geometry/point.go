package geometry

import (
	"fmt"
)

type Point struct {
	X, Y int
}

//
//
// --- POINT CONSTRUCTORS ---
//
//

func NewDefaultPoint() Point {
	return Point{X: -1, Y: -1}
}

//
// -- CARDINAL DIRECTIONS --
//

func GetDirUp() Point {
	return Point{X: 0, Y: -1}
}

func GetDirDown() Point {
	return Point{X: 0, Y: 1}
}

func GetDirRight() Point {
	return Point{X: 1, Y: 0}
}

func GetDirLeft() Point {
	return Point{X: -1, Y: 0}
}

func GetCardinalDirs() []Point {
	return []Point{GetDirUp(), GetDirRight(), GetDirDown(), GetDirLeft()}
}

//
// -- DIAGONAL DIRECTIONS --
//

func GetDirUpRight() Point {
	return Point{X: 1, Y: -1}
}

func GetDirUpLeft() Point {
	return Point{X: -1, Y: -1}
}

func GetDirDownRight() Point {
	return Point{X: 1, Y: 1}
}

func GetDirDownLeft() Point {
	return Point{X: -1, Y: 1}
}

func GetDiagonalDirs() []Point {
	return []Point{GetDirUpRight(), GetDirDownRight(), GetDirDownLeft(), GetDirUpLeft()}
}

//
// -- ALL DIRECTIONS --
//

func GetAllDirs() []Point {
	return append(GetCardinalDirs(), GetDiagonalDirs()...)
}

//
//
// --- OTHER METHODS ---
//
//

func (p Point) Add(add Point) Point {
	return Point{X: p.X + add.X, Y: p.Y + add.Y}
}

func (p Point) Sub(sub Point) Point {
	return Point{X: p.X - sub.X, Y: p.Y - sub.Y}
}

func (p Point) Equal(other Point) bool {
	return p.X == other.X && p.Y == other.Y
}

func (p *Point) String() string {
	return fmt.Sprintf("(%v, %v)", p.X, p.Y)
}
