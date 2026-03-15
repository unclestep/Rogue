package mapper

import (
	"fmt"
	"github.com/unclestep/Rogue/internal/domain/model"
	"github.com/unclestep/Rogue/internal/infrastructure/storage/json/dto"
	"github.com/unclestep/Rogue/pkg/geometry"
)

func MapToDTO(m *model.Map) *dto.MapDTO {
	if m == nil {
		return nil
	}

	d := &dto.MapDTO{
		Width:          m.Width,
		Height:         m.Height,
		EntranceRoomId: int64(m.EntranceRoomId),
		ExitRoomId:     int64(m.ExitRoomId),
		ExitPoint:      pointToDTO(m.ExitPoint),
		ItemGrid:       m.GetItemGrid(),
		ActorGrid:      m.GetActorGrid(),
	}

	d.TileGrid = make([][]dto.CellDTO, m.Height)
	for y := 0; y < m.Height; y++ {
		d.TileGrid[y] = make([]dto.CellDTO, m.Width)
		for x := 0; x < m.Width; x++ {
			cell := m.GetTileGrid()[y][x]
			d.TileGrid[y][x] = dto.CellDTO{
				Type:   cell.Type.String(),
				RoomId: int64(cell.RoomId),
			}
		}
	}

	d.Rooms = make([]*dto.RoomDTO, m.GetRoomCount())
	for i, r := range m.GetRooms() {
		d.Rooms[i] = &dto.RoomDTO{
			Id:     int64(r.Id),
			Pos:    pointToDTO(r.Pos),
			Width:  r.Width,
			Height: r.Height,
			Center: pointToDTO(r.Center),
			Doors:  doorsToDTO(r.GetDoors()),
		}
	}

	return d
}

func doorsToDTO(src []*model.DoorMetadata) []*dto.DoorMetadataDTO {
	res := make([]*dto.DoorMetadataDTO, len(src))
	for i, d := range src {
		res[i] = &dto.DoorMetadataDTO{
			Pos:     pointToDTO(d.Pos),
			Locked:  d.Locked,
			Keyhole: int(d.Keyhole),
			KeyPos:  pointToDTO(d.KeyPos),
		}
	}
	return res
}
