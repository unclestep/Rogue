package service_test

import (
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/unclestep/Rogue/internal/domain/model"
	"github.com/unclestep/Rogue/internal/domain/service"
	"github.com/unclestep/Rogue/pkg/geometry"
)

func bfsForLockedDoors(ctx *model.SessionContext) (map[model.Keyhole]bool, map[geometry.Point]bool) {
	m := ctx.Playthrough.Map
	inventory := make(map[model.Keyhole]bool)
	visited := make(map[geometry.Point]bool)

	start := m.GetEntranceRoom().Center
	queue := []geometry.Point{start}

	for len(queue) > 0 {
		cur := queue[0]
		queue = queue[1:]

		if key, ok := ctx.Playthrough.GetKey(cur); ok {
			if _, exists := inventory[key.Keyhole]; !exists {
				ctx.Playthrough.RemoveItem(key)
				inventory[key.Keyhole] = true
				clear(visited)
				queue = []geometry.Point{start}
				continue
			}
		}

		for _, dir := range geometry.GetCardinalDirs() {
			n := cur.Add(dir)

			if m.IsClosedDoor(n) {
				for key := range inventory {
					if m.TryOpenDoor(n, key) {
						break
					}
				}
			}

			if !m.IsWalkable(n) {
				continue
			}

			if !visited[n] {
				visited[n] = true
				queue = append(queue, n)
			}
		}
	}

	return inventory, visited
}

const (
	ColorReset  = "\033[0m"
	ColorRed    = "\033[31m"
	ColorGreen  = "\033[32m"
	ColorYellow = "\033[33m"
	ColorBlue   = "\033[34m"
	ColorPurple = "\033[35m"
	ColorCyan   = "\033[36m"
)

// String - returns string representation of the map
func stringMap(play *model.Playthrough, m *model.Map) string {
	colors := []string{ColorRed, ColorGreen, ColorYellow, ColorBlue, ColorPurple, ColorCyan}
	var sb strings.Builder

	for row := range m.GetTileGrid() {
		for col := range m.GetTileGrid()[row] {
			var ch rune
			tile := m.GetTileGrid()[row][col].Type

			switch tile {
			case model.Wall:
				ch = '#'
			case model.Floor:
				ch = '.'
			case model.OpenDoor:
				ch = '/'
			case model.ClosedDoor:
				keyhole := int(m.GetDoors()[geometry.Point{X: col, Y: row}].Keyhole)
				colorIndex := keyhole % len(colors)
				sb.WriteString(colors[colorIndex])
				ch = '%'
			case model.Corridor:
				ch = '*'
			case model.Exit:
				ch = '>'
			default:
				ch = ' '
			}

			p := geometry.Point{X: col, Y: row}
			if p == m.GetEntranceRoom().Center {
				ch = '@'
			}

			if key, ok := play.GetKey(p); ok {
				colorIndex := int(key.Keyhole) % len(colors)
				sb.WriteString(colors[colorIndex])
				ch = 'K'
			} else if m.IsItem(p) {
				ch = 'I'
			}

			if m.IsActor(p) {
				ch = 'A'
			}

			sb.WriteRune(ch)
			sb.WriteString(ColorReset)
		}
		sb.WriteRune('\n')
	}

	return sb.String()
}

func TestDoorLockerLockDoors(t *testing.T) {
	tg := service.NewTopologyGenerator()
	dl := service.NewDoorLocker()

	setupLockerEnv := func(seed int64, w, h int) *model.SessionContext {
		ctx := createTestContext(seed)
		tg.Gen(ctx, 80, 24, w, h, 0)
		return ctx
	}

	t.Run("Default (3 doors, 3 keys)", func(t *testing.T) {
		for i := 0; i < 100; i++ {
			seed := time.Now().UnixNano() + int64(i)
			ctx := setupLockerEnv(seed, 3, 3)
			m := ctx.Playthrough.Map

			dl.LockDoors(ctx, 3, 3)

			lockedDoorsCount := 0
			for _, door := range m.GetDoors() {
				if door.Locked {
					lockedDoorsCount++
				}
			}

			if lockedDoorsCount != 3 {
				t.Errorf("Seed: %v, iter: %v, Expected 3 locked doors, got %d", seed, i, lockedDoorsCount)
			}

			inventory, visited := bfsForLockedDoors(ctx)

			if len(inventory) != 3 {
				fmt.Println(stringMap(ctx.Playthrough, m))
				t.Errorf("Seed: %v, iter: %v, Expected 3 keys collected, got %d", seed, i, len(inventory))
			}

			if !visited[m.ExitPoint] {
				t.Errorf("Seed: %v, Exit was not reached from the entrance after collecting keys", seed)
			}
		}
	})

	t.Run("MoreDoorsThanKeys (5 doors, 3 keys)", func(t *testing.T) {
		for i := 0; i < 100; i++ {
			seed := time.Now().UnixNano() + int64(i)
			ctx := setupLockerEnv(seed, 3, 3)
			m := ctx.Playthrough.Map

			dl.LockDoors(ctx, 5, 3)

			inventory, visited := bfsForLockedDoors(ctx)

			if len(inventory) != 3 {
				fmt.Println(stringMap(ctx.Playthrough, m))
				t.Errorf("Seed: %v, iter: %v, Expected 3 keys collected, got %d", seed, i, len(inventory))
			}

			if !visited[m.ExitPoint] {
				t.Errorf("Seed: %v, Exit was not reachable", seed)
			}
		}
	})

	t.Run("MoreKeysThanDoors (3 doors, 5 keys)", func(t *testing.T) {
		for i := 0; i < 100; i++ {
			seed := time.Now().UnixNano() + int64(i)
			ctx := setupLockerEnv(seed, 3, 3)
			m := ctx.Playthrough.Map

			dl.LockDoors(ctx, 3, 5)

			inventory, visited := bfsForLockedDoors(ctx)

			if len(inventory) != 3 {
				fmt.Println(stringMap(ctx.Playthrough, m))
				t.Errorf("Seed: %v, Expected 3 keys collected (validKeysCount=min(3,5)), got %d", seed, len(inventory))
			}

			if !visited[m.ExitPoint] {
				t.Errorf("Seed: %v, Exit was not reachable", seed)
			}
		}
	})

	t.Run("TooManyDoors (100 doors, 1 key)", func(t *testing.T) {
		for i := 0; i < 100; i++ {
			seed := time.Now().UnixNano() + int64(i)
			ctx := setupLockerEnv(seed, 3, 3)
			m := ctx.Playthrough.Map

			dl.LockDoors(ctx, 100, 1)

			inventory, visited := bfsForLockedDoors(ctx)

			if len(inventory) != 1 {
				fmt.Println(stringMap(ctx.Playthrough, m))
				t.Errorf("Seed: %v, Expected 1 key collected, got %d", seed, len(inventory))
			}

			if !visited[m.ExitPoint] {
				t.Errorf("Seed: %v, Exit was not reachable", seed)
			}
		}
	})

	t.Run("OneRoomOnTheMap", func(t *testing.T) {
		seed := time.Now().UnixNano()
		ctx := setupLockerEnv(seed, 1, 1)
		m := ctx.Playthrough.Map

		dl.LockDoors(ctx, 3, 3)

		lockedDoorsCount := 0
		for _, door := range m.GetDoors() {
			if door.Locked {
				lockedDoorsCount++
			}
		}

		if lockedDoorsCount != 0 {
			fmt.Println(stringMap(ctx.Playthrough, m))
			t.Errorf("Seed: %v\nExpected 0 locked doors on 1x1 map, got %d", seed, lockedDoorsCount)
		}
	})
}
