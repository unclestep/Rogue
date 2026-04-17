package dto

import "github.com/unclestep/Rogue/internal/domain/model"

// TopologyDump is the JSON-serialisable snapshot of a generated dungeon.
// Runtime-only state (actors, items, FoW, hydration caches) is intentionally
// dropped — the Python training environment only needs static topology.
//
// Tiles are the raw model.TileType ordinals (Empty=0, Wall=1, Floor=2,
// OpenDoor=3, ClosedDoor=4, Corridor=5, Exit=6). Any reshuffling of that
// enum must be mirrored in the Python loader.
type TopologyDump struct {
	Width        int            `json:"width"`
	Height       int            `json:"height"`
	Tiles        [][]int        `json:"tiles"`
	Rooms        []TopologyRoom `json:"rooms"`
	EntranceRoom int64          `json:"entrance_room"`
	ExitRoom     int64          `json:"exit_room"`
	ExitPoint    TopologyPoint  `json:"exit_point"`
}

type TopologyRoom struct {
	Id     int64         `json:"id"`
	X      int           `json:"x"`
	Y      int           `json:"y"`
	Width  int           `json:"w"`
	Height int           `json:"h"`
	Center TopologyPoint `json:"center"`
}

type TopologyPoint struct {
	X int `json:"x"`
	Y int `json:"y"`
}

// ToTopologyDump flattens a live Map into the dump format. No deep copies of
// slices are made — callers must not mutate the source map after calling.
func ToTopologyDump(m *model.Map) TopologyDump {
	grid := m.GetTileGrid()
	tiles := make([][]int, m.Height)
	for y := range grid {
		row := make([]int, m.Width)
		for x := range grid[y] {
			row[x] = int(grid[y][x].Type)
		}
		tiles[y] = row
	}

	rooms := m.GetRooms()
	out := make([]TopologyRoom, 0, len(rooms))
	for _, r := range rooms {
		out = append(out, TopologyRoom{
			Id:     int64(r.Id),
			X:      r.Pos.X,
			Y:      r.Pos.Y,
			Width:  r.Width,
			Height: r.Height,
			Center: TopologyPoint{X: r.Center.X, Y: r.Center.Y},
		})
	}

	return TopologyDump{
		Width:        m.Width,
		Height:       m.Height,
		Tiles:        tiles,
		Rooms:        out,
		EntranceRoom: int64(m.EntranceRoomId),
		ExitRoom:     int64(m.ExitRoomId),
		ExitPoint:    TopologyPoint{X: m.ExitPoint.X, Y: m.ExitPoint.Y},
	}
}
