package mapper

import (
	"log"

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
		Doors:          doorMapToDTO(m.GetDoors()),
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
		}
	}

	return d
}

func MapFromDTO(d *dto.MapDTO) *model.Map {
	if d == nil {
		return nil
	}

	tileGrid := make([][]model.Cell, d.Height)
	for y := 0; y < d.Height; y++ {
		tileGrid[y] = make([]model.Cell, d.Width)
		for x := 0; x < d.Width; x++ {
			tileType, err := model.TileTypeString(d.TileGrid[y][x].Type)
			if err != nil {
				log.Printf("[WARN] mapper: unknown tile type %q at (%d,%d)\n", d.TileGrid[y][x].Type, x, y)
			}
			tileGrid[y][x] = model.Cell{
				Type:   tileType,
				RoomId: model.RoomId(d.TileGrid[y][x].RoomId),
			}
		}
	}

	rooms := make([]*model.Room, len(d.Rooms))
	for i, r := range d.Rooms {
		rooms[i] = &model.Room{
			Id:     model.RoomId(r.Id),
			Pos:    pointFromDTO(r.Pos),
			Width:  r.Width,
			Height: r.Height,
			Center: pointFromDTO(r.Center),
		}
	}

	blueprint := &model.MapBlueprint{
		Width:          d.Width,
		Height:         d.Height,
		TileGrid:       tileGrid,
		Rooms:          rooms,
		Doors:          doorMapFromDTO(d.Doors),
		EntranceRoomId: model.RoomId(d.EntranceRoomId),
		ExitRoomId:     model.RoomId(d.ExitRoomId),
		ExitPoint:      pointFromDTO(d.ExitPoint),
	}

	m := model.NewMapFromBlueprint(blueprint)

	// Restore actor and item positions; Hydrate() already rebuilt room caches
	// with empty grids, so SetActor/SetItem will correctly mark cells as occupied.
	for y, row := range d.ActorGrid {
		for x, id := range row {
			if id != 0 {
				m.SetActor(geometry.Point{X: x, Y: y}, id)
			}
		}
	}
	for y, row := range d.ItemGrid {
		for x, id := range row {
			if id != 0 {
				m.SetItem(geometry.Point{X: x, Y: y}, id)
			}
		}
	}

	return m
}

func doorMapToDTO(src map[geometry.Point]*model.DoorMetadata) []*dto.DoorMetadataDTO {
	if src == nil {
		return nil
	}
	res := make([]*dto.DoorMetadataDTO, 0, len(src))
	for _, d := range src {
		res = append(res, &dto.DoorMetadataDTO{
			Pos:     pointToDTO(d.Pos),
			Locked:  d.Locked,
			Keyhole: int(d.Keyhole),
		})
	}
	return res
}

func doorMapFromDTO(src []*dto.DoorMetadataDTO) map[geometry.Point]*model.DoorMetadata {
	res := make(map[geometry.Point]*model.DoorMetadata, len(src))
	for _, d := range src {
		pos := pointFromDTO(d.Pos)
		res[pos] = &model.DoorMetadata{
			Pos:     pos,
			Locked:  d.Locked,
			Keyhole: model.Keyhole(d.Keyhole),
		}
	}
	return res
}
