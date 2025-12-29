package object

import "testing"

// Тупые проверки
func TestConstructor(t *testing.T) {
	obj := New(1, 2, 3, 4)

	if obj.pos.X != 1 || obj.pos.Y != 2 {
		t.Errorf("Expected position (1,2), got (%d,%d)", obj.pos.X, obj.pos.Y)
	}

	if obj.size.X != 3 || obj.size.Y != 4 {
		t.Errorf("Expected size (3,4), got (%d,%d)", obj.size.X, obj.size.Y)
	}
}

func TestGetters(t *testing.T) {
	obj := New(5, 6, 7, 8)

	coords := obj.Coords()
	if coords.X != 5 || coords.Y != 6 {
		t.Errorf("Expected Coords (5,6), got (%d,%d)", coords.X, coords.Y)
	}

	size := obj.Size()
	if size.X != 7 || size.Y != 8 {
		t.Errorf("Expected Size (7,8), got (%d,%d)", size.X, size.Y)
	}
}

func TestSetters(t *testing.T) {
	obj := New(0, 0, 0, 0)

	obj.SetCoords(9, 10)
	coords := obj.Coords()
	if coords.X != 9 || coords.Y != 10 {
		t.Errorf("Expected Coords (9,10), got (%d,%d)", coords.X, coords.Y)
	}

	obj.SetSize(11, 12)
	size := obj.Size()
	if size.X != 11 || size.Y != 12 {
		t.Errorf("Expected Size (11,12), got (%d,%d)", size.X, size.Y)
	}
}
