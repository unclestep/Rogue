// Package entity - сущности доменной области игры.
// Food - сущность еды в игре.
// FoodObject - обьект еды на игровом поле.
package entity

import (
	"math/rand/v2"

	"github.com/Nikolay-Yakunin/gouge/internal/pkg/object"
)

// MaxFoodRegenPercent - максимальный процент восстановления здоровья едой.
const MaxFoodRegenPercent = 20

// FoodNames - названия еды в игре.
// Потом можно заменить на чтение из файла или тип того.
// TODO: Перенести в data layer.
var FoodNames = []string{
	"Ration of the Ironclad",
	"Crimson Berry Cluster",
	"Loaf of the Forgotten Baker",
	"Smoked Wyrm Jerky",
	"Golden Apple of Vitality",
	"Hardtack of the Endless March",
	"Spiced Venison Strips",
	"Honeyed Nectar Bread",
	"Dried Mushrooms of the Deep",
}

// Food - сущность еды в игре.
type Food struct {
	ToRegen int    `json:"toRegen"` // Количество здоровья, которое восстанавливает еда.
	Name    string `json:"name"`    // Название еды.
}

// FoodInterface - интерфейс сущности еды.
type FoodInterface interface {
	SetToRegen(toRegen int)
	SetName(name string)
}

// NewFood - создание новой сущности еды.
func NewFood(name string, toRegen int) *Food {
	return &Food{
		ToRegen: toRegen,
		Name:    name,
	}
}

// GenerateFood - генерация еды.
func GenerateFood(maxHealth int) *Food {
	// Рандомное имя из слайса FoodNames.
	// #nosec G404
	name := FoodNames[rand.IntN(len(FoodNames))]
	// Рандомный максимальный реген на основе максимального здоровья персонажа и защита от отрицательных значений?.
	maxRegen := max(maxHealth*MaxFoodRegenPercent/100, 1)
	// Рандомное количество регена от 1 до maxRegen.
	// #nosec G404
	toRegen := rand.IntN(maxRegen) + 1
	return NewFood(name, toRegen)
}

// SetToRegen - установка количества здоровья, которое восстанавливает еда.
func (f *Food) SetToRegen(toRegen int) {
	f.ToRegen = toRegen
}

// SetName - установка названия еды.
func (f *Food) SetName(name string) {
	f.Name = name
}

// String - строковое представление еды.
func (f *Food) String() string {
	return f.Name + " (Regen: " + string(rune(f.ToRegen)) + ")"
}

// FoodObject - обьект еды на игровом поле.
type FoodObject struct {
	*object.Object `json:"object"` // Обьект с координатами и размером.
	*Food          `json:"food"`   // Сущность еды.
}

// FoodObjectInterface - интерфейс обьекта еды.
type FoodObjectInterface interface {
	object.Interface
	FoodInterface
}

// NewFoodObject - создание нового обьекта еды на игровом поле.
func NewFoodObject(x, y, sizeX, sizeY, toRegen int, name string) *FoodObject {
	return &FoodObject{
		Object: object.New(x, y, sizeX, sizeY),
		Food:   NewFood(name, toRegen),
	}
}

// GenerateFoodObject - генерация обьекта с рандомной едой.
func GenerateFoodObject(x, y, sizeX, sizeY, maxHealth int) *FoodObject {
	food := GenerateFood(maxHealth)
	return NewFoodObject(x, y, sizeX, sizeY, food.ToRegen, food.Name)
}
