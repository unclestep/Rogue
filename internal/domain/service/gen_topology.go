package service

import (
	"log"
	"math"
	"math/rand"
	"slices"

	"github.com/unclestep/Rogue/internal/domain/model"
	"github.com/unclestep/Rogue/pkg/algorithm"
	"github.com/unclestep/Rogue/pkg/geometry"
	"github.com/unclestep/Rogue/pkg/utils"
)

type TopologyGenerator struct{}

func NewTopologyGenerator() *TopologyGenerator {
	return &TopologyGenerator{}
}

const (
	SectorMargin = 1 // Right margin of one room + left margin of second room
	MinRoomSize  = 3 // Wall + Floor + Wall

)

// sector - structure for sector model
type sector struct {
	xMin, yMin, xMax, yMax int
}

func (t *TopologyGenerator) Gen(ctx *model.SessionContext, mapWidth, mapHeight, roomCountHorizontal, roomCountVertical int) {
	if !t.canFitGrid(mapWidth, mapHeight, roomCountHorizontal, roomCountVertical) {
		return
	}

	blueprint := model.NewMapBlueprint(mapWidth, mapHeight)
	sectors := t.sectorize(mapWidth, mapHeight, roomCountHorizontal, roomCountVertical, ctx.Rng())
	t.createRooms(blueprint, sectors, ctx.Rng())
	t.createEntranceAndExit(blueprint, ctx.Rng())
	t.connectRooms(blueprint)
	ctx.Playthrough.Map = model.NewMapFromBlueprint(blueprint)
}

// canFitGrid - checks the possibility to generate given number of rooms horizontally and vertically.
func (t *TopologyGenerator) canFitGrid(mapWidth, mapHeight, roomCountHorizontal, roomCountVertical int) bool {
	if roomCountHorizontal <= 0 || roomCountVertical <= 0 {
		return false
	}

	numberWidthMargins, numberHeightMargins := roomCountHorizontal-1, roomCountVertical-1
	minAllowedMapWidth := roomCountHorizontal*MinRoomSize + SectorMargin*numberWidthMargins
	minAllowedMapHeight := roomCountVertical*MinRoomSize + SectorMargin*numberHeightMargins

	if mapWidth < minAllowedMapWidth || mapHeight < minAllowedMapHeight {
		return false
	}

	return true
}

// Function a grid of sector such that each sector has different sizes.
// gridWidth - number of rooms horizontally, gridHeight - number of rooms vertically.
// Returns matrix of sectors.
func (t *TopologyGenerator) sectorize(mapWidth, mapHeight, gridWidth, gridHeight int, rng *rand.Rand) [][]sector {
	sectors := utils.CreateMatrix[sector](gridHeight, gridWidth)
	sectorWidth, sectorWidthRem := mapWidth/gridWidth, mapWidth%gridWidth
	sectorHeight, sectorHeightRem := mapHeight/gridHeight, mapHeight%gridHeight

	heightRemLimit := make([]int, gridWidth) // Height remainder for each column
	for h := range heightRemLimit {
		heightRemLimit[h] = sectorHeightRem
	}
	yMins := make([]int, gridWidth) // Last sector's yMin for each column

	// Create sectors
	for row := range gridHeight {
		widthRemLimit := sectorWidthRem
		xMin := 0
		for col := range gridWidth {
			var randHeightRemAdd, yMax int
			if row == gridHeight-1 { // For last row just add rest limit's remainder without randomization
				randHeightRemAdd = heightRemLimit[col]
				yMax = yMins[col] + (sectorHeight - 1) + randHeightRemAdd
			} else {
				randHeightRemAdd = rng.Intn(heightRemLimit[col] + 1)
				yMax = yMins[col] + (sectorHeight - 1) + randHeightRemAdd - SectorMargin
			}
			heightRemLimit[col] -= randHeightRemAdd

			var randWidthRemAdd, xMax int
			if col == gridWidth-1 { // For last col just add limit's remainder without randomization
				randWidthRemAdd = widthRemLimit
				xMax = xMin + (sectorWidth - 1) + randWidthRemAdd
			} else {
				randWidthRemAdd = rng.Intn(widthRemLimit + 1)
				xMax = xMin + (sectorWidth - 1) + randWidthRemAdd - SectorMargin
			}
			widthRemLimit -= randWidthRemAdd

			sectors[row][col] = sector{xMin: xMin, xMax: xMax, yMin: yMins[col], yMax: yMax}
			xMin = xMax + SectorMargin + 1
			yMins[col] = yMax + SectorMargin + 1
		}
	}

	return sectors
}

// createRooms - creates one room in every sector.
func (t *TopologyGenerator) createRooms(blueprint *model.MapBlueprint, sectors [][]sector, rng *rand.Rand) {
	minHeight, minWidth := MinRoomSize, MinRoomSize
	rows, cols := len(sectors), len(sectors[0])
	blueprint.Rooms = make([]*model.Room, rows*cols)

	// Create rooms
	for row := range rows {
		for col := range cols {
			yMax, yMin := sectors[row][col].yMax, sectors[row][col].yMin
			xMax, xMin := sectors[row][col].xMax, sectors[row][col].xMin

			// Pick random height and width
			sectorHeight := yMax - yMin + 1
			sectorWidth := xMax - xMin + 1
			roomHeight := minHeight + rng.Intn(sectorHeight-minHeight+1)
			roomWidth := minWidth + rng.Intn(sectorWidth-minWidth+1)

			// Interior space excluding the walls
			utilHeight := roomHeight - 2
			utilWidth := roomWidth - 2

			// Pick random pos (left-upper corner)
			maxPossibleX := xMax - (roomWidth - 1)       //  Max possible X to take so that the room fits into the sector
			maxPossibleY := yMax - (roomHeight - 1)      // Max possible Y to take so that the room fits into the sector
			posX := xMin + rng.Intn(maxPossibleX-xMin+1) // Final X
			posY := yMin + rng.Intn(maxPossibleY-yMin+1) // Final Y
			utilX, utilY := posX+1, posY+1               // left-upper corner in the room (floor cell)

			// Assemble
			room := &model.Room{
				Id:     model.RoomId(row*cols + col + 1),
				Pos:    geometry.Point{X: utilX, Y: utilY},
				Width:  utilWidth,
				Height: utilHeight,
				Center: geometry.Point{
					X: utilX + utilWidth/2,
					Y: utilY + utilHeight/2,
				},
			}
			blueprint.Rooms[row*cols+col] = room
		}
	}

	// Add rooms to the tile grid
	for _, room := range blueprint.Rooms {
		wallYMin, wallXMin := room.Pos.Y-1, room.Pos.X-1
		wallYMax, wallXMax := wallYMin+room.Height+1, wallXMin+room.Width+1

		for y := wallYMin; y <= wallYMax; y++ {
			for x := wallXMin; x <= wallXMax; x++ {
				tile := &blueprint.TileGrid[y][x]

				if y == wallYMin || y == wallYMax || x == wallXMin || x == wallXMax {
					tile.Type = model.Wall
				} else {
					tile.Type = model.Floor
					tile.RoomId = room.Id
				}
			}
		}
	}

	// Make the order of rooms random
	utils.Shuffle(rng, blueprint.Rooms)
}

// createEntranceAndExit - chooses one random room to be entrance and one room to be exited. Generates there entrance and exit points.
func (t *TopologyGenerator) createEntranceAndExit(blueprint *model.MapBlueprint, rng *rand.Rand) {
	rooms := slices.Clone(blueprint.Rooms)

	roomMaxSquare := rooms[0]
	maxSquare := roomMaxSquare.Height * roomMaxSquare.Width
	roomMaxSquareSliceInd := 0

	for i, room := range rooms {
		square := room.Height * room.Width
		if square > maxSquare {
			maxSquare = square
			roomMaxSquare = room
			roomMaxSquareSliceInd = i
		}
	}

	blueprint.EntranceRoomId = roomMaxSquare.Id
	rooms = algorithm.Remove(rooms, roomMaxSquareSliceInd)

	exitRoom := roomMaxSquare
	if len(rooms) > 0 {
		exitRoom = rooms[0]
	}
	blueprint.ExitRoomId = exitRoom.Id

	exitPointX := exitRoom.Pos.X + rng.Intn(exitRoom.Width)
	exitPointY := exitRoom.Pos.Y + rng.Intn(exitRoom.Height)

	blueprint.ExitPoint = geometry.Point{X: exitPointX, Y: exitPointY}
	blueprint.TileGrid[exitPointY][exitPointX].Type = model.Exit
}

// connectRooms - digs corridor between two room centers using A* algorithm.
// There is a chance that corridor between two rooms can go through third room,
// so map can be very interesting.
func (t *TopologyGenerator) connectRooms(blueprint *model.MapBlueprint) {
	for i := 0; i < len(blueprint.Rooms)-1; i++ {
		path, ok := t.findDiggingPath(blueprint, blueprint.Rooms[i], blueprint.Rooms[i+1]) // A* algorithm

		if !ok {
			log.Fatalf("[ERROR] Could not find digging path between room #%d and room #%d", i, i+1)
		}

		for _, p := range path {
			x, y := p.X, p.Y
			tile := &blueprint.TileGrid[y][x]

			switch tile.Type {
			case model.Empty:
				tile.Type = model.Corridor
			case model.Wall:
				tile.Type = model.OpenDoor
				blueprint.Doors[geometry.Point{X: x, Y: y}] = &model.DoorMetadata{Pos: p}
			default:
			}
		}
	}
}

type blueprintGraph struct {
	bp *model.MapBlueprint
}

// findDiggingPath - wrapper function of algorithm package FindPath function with custom cost function.
func (t *TopologyGenerator) findDiggingPath(blueprint *model.MapBlueprint, r1, r2 *model.Room) ([]geometry.Point, bool) {
	if r1 == nil || r2 == nil {
		return nil, false
	}

	blueprintGraph := &blueprintGraph{bp: blueprint}

	path := algorithm.FindPath(blueprintGraph, r1.Center, r2.Center, blueprintGraph.getDiggingCost)
	if path == nil {
		return nil, false
	}

	return path, true
}

// getDiggingCost - cost function for A* algorithm.
func (b *blueprintGraph) getDiggingCost(p geometry.Point) float64 {
	tile := b.bp.TileGrid[p.Y][p.X].Type
	cost := 0.0

	switch tile {
	case model.Empty:
		cost = 1.0
	case model.Corridor:
		cost = 0.5 // Better use already dug corridors
	case model.Floor, model.OpenDoor:
		cost = 3.0
	case model.Wall:
		cost = 50.0
	default:
		return 1.0
	}

	// Make it more expensive to go along the walls,
	// so that the map is more sparse
	if tile == model.Empty {
		for _, dir := range geometry.GetCardinalDirs() {
			n := p.Add(dir)
			if n.X >= 0 && n.X < b.bp.Width && n.Y >= 0 && n.Y < b.bp.Height {
				nt := b.bp.TileGrid[n.Y][n.X].Type
				if nt == model.Wall || nt == model.Corridor {
					cost += 5.0
				}
			}
		}
	}

	return cost
}

//
//
// --- A* ALGORITHM INTERFACE ---
//
//

// GetNeighbors - returns a slice of neighboring points
// Returns points within the boundaries to the right, below, left and above given point
func (b *blueprintGraph) GetNeighbors(p geometry.Point) []geometry.Point {
	valid := make([]geometry.Point, 0, 4)

	for _, dir := range geometry.GetCardinalDirs() {
		n := p.Add(dir)
		// Only points in bounds
		if n.X >= 0 && n.X < b.bp.Width && n.Y >= 0 && n.Y < b.bp.Height {
			valid = append(valid, n)
		}
	}

	return valid
}

// CalcHeuristic - implements Manhattan's distance
func (b *blueprintGraph) CalcHeuristic(p1, p2 geometry.Point) float64 {
	x1, y1 := p1.X, p1.Y
	x2, y2 := p2.X, p2.Y
	return math.Abs(float64(x2-x1)) + math.Abs(float64(y2-y1))
}
