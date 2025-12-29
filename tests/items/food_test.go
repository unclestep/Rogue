package items_test

import (
	"testing"

	"github.com/Nikolay-Yakunin/gouge/internal/domain/entity"
)

func TestFoodSetters(t *testing.T) {
	food := entity.NewFood("Test Food", 10)

	food.SetName("Updated Food")
	if food.Name != "Updated Food" {
		t.Errorf("Expected Name 'Updated Food', got '%s'", food.Name)
	}

	food.SetToRegen(20)
	if food.ToRegen != 20 {
		t.Errorf("Expected ToRegen 20, got %d", food.ToRegen)
	}
}

func TestFoodObjectSetters(t *testing.T) {
	foodObj := entity.NewFoodObject(1, 2, 3, 4, 10, "Test Food")

	foodObj.SetCoords(5, 6)
	if foodObj.Pos.X != 5 || foodObj.Pos.Y != 6 {
		t.Errorf("Expected Coords (5,6), got (%d,%d)", foodObj.Pos.X, foodObj.Pos.Y)
	}

	foodObj.SetSize(7, 8)
	if foodObj.Size.X != 7 || foodObj.Size.Y != 8 {
		t.Errorf("Expected Size (7,8), got (%d,%d)", foodObj.Size.X, foodObj.Size.Y)
	}

	foodObj.SetName("Updated Food")
	if foodObj.Name != "Updated Food" {
		t.Errorf("Expected Name 'Updated Food', got '%s'", foodObj.Name)
	}

	foodObj.SetToRegen(20)
	if foodObj.ToRegen != 20 {
		t.Errorf("Expected ToRegen 20, got %d", foodObj.ToRegen)
	}
}

func TestGenerateFood(t *testing.T) {
	food := entity.GenerateFood(500)
	if food.ToRegen < 5 || food.ToRegen > 100 {
		t.Errorf("Expected ToRegen between 5 and 100, got %d", food.ToRegen)
	}
	if food.Name == "" {
		t.Error("Expected non-empty Name")
	}
}

func TestGenerateFoodObject(t *testing.T) {
	foodObj := entity.GenerateFoodObject(10, 20, 2, 3, 500)
	if foodObj.Pos.X != 10 || foodObj.Pos.Y != 20 {
		t.Errorf("Expected Coords (10,20), got (%d,%d)", foodObj.Pos.X, foodObj.Pos.Y)
	}
	if foodObj.Size.X != 2 || foodObj.Size.Y != 3 {
		t.Errorf("Expected Size (2,3), got (%d,%d)", foodObj.Size.X, foodObj.Size.Y)
	}
}
