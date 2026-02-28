package geometry

import (
	"fmt"
	"testing"
)

func TestPointString(t *testing.T) {
	p := Point{X: 3, Y: 4}
	expected := "(3, 4)"
	if p.String() != expected {
		t.Errorf("Expected %s, got %s", expected, p.String())
	}
}

func TestPointWithSprint(t *testing.T) {
	p := Point{X: -1, Y: 10}
	expected := "(-1, 10)"
	result := fmt.Sprintf("%v", p)
	if result != expected {
		t.Errorf("Expected %s, got %s", expected, result)
	}
}
