package service

import (
	"math"

	"github.com/unclestep/Rogue/internal/domain/model"
	"github.com/unclestep/Rogue/pkg/geometry"
)

// FlashlightHalfFOV is the default half-cone angle in degrees.
// FlashlightRange is the default visibility radius in grid cells.
const (
	FlashlightHalfFOV = 45
	FlashlightRange   = 15.0
)

// Raycaster handles field-of-view calculations using camera-plane DDA raycasting.
type Raycaster struct{}

func NewRaycaster() *Raycaster { return &Raycaster{} }

//
// --- CAMERA ---
//

// Camera represents the virtual flashlight emitter.
// Dir is the unit direction vector; Plane is the camera plane perpendicular
// to Dir whose magnitude equals tan(halfFOV).
type Camera struct {
	PosX, PosY     float64
	DirX, DirY     float64
	PlaneX, PlaneY float64
}

// NewCameraWithHalfFOV creates a camera whose cone spans 2·halfFOVDeg.
// Examples:  halfFOVDeg=22.5 -> 45 cone,  halfFOVDeg=45 -> 90 cone.
func NewCameraWithHalfFOV(x, y, angle, halfFOVDeg float64) Camera {
	// |plane| / |dir| = tan(halfFOV)  ->  |plane| = tan(halfFOV) when |dir|=1.
	planeScale := math.Tan(halfFOVDeg * math.Pi / 180.0)
	return Camera{
		PosX: x, PosY: y,
		DirX:   math.Cos(angle),
		DirY:   math.Sin(angle),
		PlaneX: -math.Sin(angle) * planeScale,
		PlaneY: math.Cos(angle) * planeScale,
	}
}

//
// --- FOW FLASHLIGHT ---
//

func (rc *Raycaster) Flashlight(
	area *model.VisibleArea,
	m *model.Map,
	cam Camera,
	maxRange float64,
) {
	// Reset: previously visible cells become Explored (memory is retained).
	for _, p := range area.VisibleCells {
		if area.Area[p.Y][p.X] == model.Visible {
			area.Area[p.Y][p.X] = model.Explored
		}
	}
	area.VisibleCells = area.VisibleCells[:0]

	// The player's own cell is always lit.
	origin := geometry.Point{X: int(cam.PosX), Y: int(cam.PosY)}
	markVisible(area, origin)

	// Camera-plane magnitude |plane| = tan(halfFOV).
	planeLen := math.Sqrt(cam.PlaneX*cam.PlaneX + cam.PlaneY*cam.PlaneY)
	if planeLen < 1e-9 {
		return // degenerate zero-width cone
	}

	// Minimal angular step: guarantees <= 0.5-cell gap between adjacent rays
	// at maximum range. Derived directly from geometry - not a magic constant.
	step := 0.5 / (planeLen * maxRange)

	for cx := -1.0; cx <= 1.0; cx += step {
		castFOWRay(area, m, &cam, cx, maxRange)
	}
	// Always cast the exact right-edge ray in case the loop ended just short.
	castFOWRay(area, m, &cam, 1.0, maxRange)
}

// castFOWRay fires one DDA ray at camera column cameraX in [-1, +1].
// It marks every grid cell the ray crosses as Visible, then stops at (and
// reveals) the first light-blocking tile or when maxRange is exceeded.
func castFOWRay(area *model.VisibleArea, m *model.Map, cam *Camera, cameraX, maxRange float64) {
	rayDirX := cam.DirX + cam.PlaneX*cameraX
	rayDirY := cam.DirY + cam.PlaneY*cameraX

	mapX := int(cam.PosX)
	mapY := int(cam.PosY)

	// Delta distances: how far to travel in world-space to cross one grid line.
	var deltaDistX, deltaDistY float64
	if rayDirX == 0 {
		deltaDistX = math.MaxFloat64
	} else {
		deltaDistX = math.Abs(1.0 / rayDirX)
	}
	if rayDirY == 0 {
		deltaDistY = math.MaxFloat64
	} else {
		deltaDistY = math.Abs(1.0 / rayDirY)
	}

	// Side distances: distance from camera position to the very first grid boundary.
	var stepX, stepY int
	var sideDistX, sideDistY float64
	if rayDirX < 0 {
		stepX = -1
		sideDistX = (cam.PosX - float64(mapX)) * deltaDistX
	} else {
		stepX = 1
		sideDistX = (float64(mapX) + 1.0 - cam.PosX) * deltaDistX
	}
	if rayDirY < 0 {
		stepY = -1
		sideDistY = (cam.PosY - float64(mapY)) * deltaDistY
	} else {
		stepY = 1
		sideDistY = (float64(mapY) + 1.0 - cam.PosY) * deltaDistY
	}

	var side int
	for {
		// Advance to the next grid cell.
		if sideDistX < sideDistY {
			sideDistX += deltaDistX
			mapX += stepX
			side = 0
		} else {
			sideDistY += deltaDistY
			mapY += stepY
			side = 1
		}

		// Perpendicular (fish-eye-corrected) distance to the current cell boundary.
		var perpDist float64
		if side == 0 {
			perpDist = sideDistX - deltaDistX
		} else {
			perpDist = sideDistY - deltaDistY
		}

		if perpDist > maxRange {
			break
		}

		p := geometry.Point{X: mapX, Y: mapY}
		if !m.InBounds(p) {
			break
		}

		tile, _ := m.GetTileType(p)

		// Always reveal the cell — the player sees the wall/door that stops them.
		markVisible(area, p)

		if blocksLight(tile) {
			break
		}
	}
}

// RefreshPlayerFoW recomputes the flashlight cone for a single player using
// their current position and stored aim angle.
// It lazily allocates a VisibleArea for players who joined after map creation.
func (rc *Raycaster) RefreshPlayerFoW(play *model.Playthrough, id model.ActorId, halfFOVDeg, maxRange float64) {
	player := play.Players[id]
	if player == nil || play.Map == nil {
		return
	}
	if play.PlayersFoW[id] == nil {
		play.PlayersFoW[id] = model.NewVisibleArea(play.Map.Height, play.Map.Width)
	}
	// PlayerAimAngles may be nil after deserialisation (field is intentionally
	// not persisted — defaults to 0 = pointing right on first load).
	if play.PlayerAimAngles == nil {
		play.PlayerAimAngles = make(map[model.ActorId]float64)
	}
	angle := play.PlayerAimAngles[id]
	cam := NewCameraWithHalfFOV(
		float64(player.Pos.X)+0.5,
		float64(player.Pos.Y)+0.5,
		angle,
		halfFOVDeg,
	)
	rc.Flashlight(play.PlayersFoW[id], play.Map, cam, maxRange)
}

// blocksLight reports whether a tile stops the flashlight beam.
// Solid walls, void cells, and closed doors all occlude light.
// Corridor tiles are transparent.
func blocksLight(t model.TileType) bool {
	return t == model.Wall || t == model.Empty || t == model.ClosedDoor || t == model.OpenDoor
}

// markVisible transitions a cell to Visible and registers it in the cache.
// The guard prevents double-appending when multiple rays converge on one cell.
func markVisible(area *model.VisibleArea, p geometry.Point) {
	if area.Area[p.Y][p.X] != model.Visible {
		area.Area[p.Y][p.X] = model.Visible
		area.VisibleCells = append(area.VisibleCells, p)
	}
}
