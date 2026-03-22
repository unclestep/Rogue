package mapper

import (
	"log"

	"github.com/unclestep/Rogue/internal/domain/model"
	"github.com/unclestep/Rogue/internal/infrastructure/storage/json/dto"
	"github.com/unclestep/Rogue/pkg/geometry"
)

func pointToDTO(p geometry.Point) dto.PointDTO {
	return dto.PointDTO{X: p.X, Y: p.Y}
}

func pointFromDTO(p dto.PointDTO) geometry.Point {
	return geometry.Point{X: p.X, Y: p.Y}
}

func pointsToDTO(src []geometry.Point) []dto.PointDTO {
	if src == nil {
		return nil
	}
	res := make([]dto.PointDTO, len(src))
	for i, p := range src {
		res[i] = pointToDTO(p)
	}
	return res
}

func pointsFromDTO(src []dto.PointDTO) []geometry.Point {
	if src == nil {
		return nil
	}
	res := make([]geometry.Point, len(src))
	for i, p := range src {
		res[i] = pointFromDTO(p)
	}
	return res
}

// vitalsToDTO converts map[VitalType]int to map[string]int using enum names.
func vitalsToDTO(src map[model.VitalType]int) map[string]int {
	if src == nil {
		return nil
	}
	res := make(map[string]int, len(src))
	for k, v := range src {
		res[k.String()] = v
	}
	return res
}

func vitalsFromDTO(src map[string]int) map[model.VitalType]int {
	if src == nil {
		return nil
	}
	res := make(map[model.VitalType]int, len(src))
	for k, v := range src {
		key, err := model.VitalTypeString(k)
		if err != nil {
			log.Printf("[WARNING] mapper: unknown vital type %q, skipping\n", k)
			continue
		}
		res[key] = v
	}
	return res
}

// attrsToDTO converts map[AttrType]int to map[string]int using enum names.
func attrsToDTO(src map[model.AttrType]int) map[string]int {
	if src == nil {
		return nil
	}
	res := make(map[string]int, len(src))
	for k, v := range src {
		res[k.String()] = v
	}
	return res
}

func attrsFromDTO(src map[string]int) map[model.AttrType]int {
	if src == nil {
		return nil
	}
	res := make(map[model.AttrType]int, len(src))
	for k, v := range src {
		key, err := model.AttrTypeString(k)
		if err != nil {
			log.Printf("[WARN] mapper: unknown attr type %q, skipping\n", k)
			continue
		}
		res[key] = v
	}
	return res
}

// statusesToDTO converts map[StatusType]int to map[string]int.
// StatusType is already a string alias, so this is a direct cast.
func statusesToDTO(src map[model.StatusType]int) map[string]int {
	if src == nil {
		return nil
	}
	res := make(map[string]int, len(src))
	for k, v := range src {
		res[string(k)] = v
	}
	return res
}

func statusesFromDTO(src map[string]int) map[model.StatusType]int {
	if src == nil {
		return nil
	}
	res := make(map[model.StatusType]int, len(src))
	for k, v := range src {
		res[model.StatusType(k)] = v
	}
	return res
}
