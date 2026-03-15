package mapper

import (
	"fmt"
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

func enumMapToDTO[K comparable, V any](src map[K]V) map[string]V {
	res := make(map[string]V, len(src))
	for k, v := range src {
		res[fmt.Sprint(k)] = v
	}
	return res
}
