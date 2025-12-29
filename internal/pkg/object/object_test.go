package object

import (
	"fmt"
	"testing"
)

// Тупые проверки
func TestConstructor(t *testing.T) {
	obj := New(1, 2, 3, 4)

	if obj.Pos.X != 1 || obj.Pos.Y != 2 {
		t.Errorf("Expected position [1,2], got %v", obj.Pos)
	}

	if obj.Size.X != 3 || obj.Size.Y != 4 {
		t.Errorf("Expected size [3,4], got %v", obj.Size)
	}
}

func TestSetters(t *testing.T) {
	obj := New(0, 0, 0, 0)

	obj.SetCoords(9, 10)
	if obj.Pos.X != 9 || obj.Pos.Y != 10 {
		t.Errorf("Expected Coords [9,10], got %v", obj.Pos)
	}

	obj.SetSize(11, 12)
	if obj.Size.X != 11 || obj.Size.Y != 12 {
		t.Errorf("Expected Size [11,12], got %v", obj.Size)
	}
}

func TestObjectString(t *testing.T) {
	obj := New(1, 2, 3, 4)
	expected := "Object{Pos: [1, 2], Size: [3, 4]}"
	result := obj.String()
	if result != expected {
		t.Errorf("Expected %s, got %s", expected, result)
	}
}

func TestCoordsString(t *testing.T) {
	coords := Coords{X: 5, Y: 6}
	expected := "[5, 6]"
	result := coords.String()
	if result != expected {
		t.Errorf("Expected %s, got %s", expected, result)
	}
}

func TestObjectStringSprintf(t *testing.T) {
	obj := New(1, 2, 3, 4)
	expected := obj.String()
	result := fmt.Sprintf("%v", obj)
	if result != expected {
		t.Errorf("Expected %s, got %s", expected, result)
	}
}

func TestCoordsStringSprintf(t *testing.T) {
	coords := Coords{X: 5, Y: 6}
	expected := coords.String()
	result := fmt.Sprintf("%v", coords)
	if result != expected {
		t.Errorf("Expected %s, got %s", expected, result)
	}
}
