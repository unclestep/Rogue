// Command dump-topology generates N procedurally-generated dungeons with the
// canonical TopologyGenerator and writes them as JSON to disk. Python RL
// training (see rl/) consumes these files so it never has to re-implement
// BSP generation — the Go side stays source of truth, avoiding sim-to-real
// drift in dungeon layout.
//
// Usage:
//
//	go run ./cmd/dump-topology -n 1000 -out rl/topologies
package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/unclestep/Rogue/internal/domain/model"
	"github.com/unclestep/Rogue/internal/domain/service"
	"github.com/unclestep/Rogue/internal/dto"
)

func main() {
	count := flag.Int("n", 1000, "number of topologies to generate")
	seed := flag.Int64("seed", time.Now().UnixNano(), "base seed; i-th map uses seed+i")
	outDir := flag.String("out", "rl/topologies", "output directory for JSON files")
	mapW := flag.Int("width", 80, "map width in tiles")
	mapH := flag.Int("height", 24, "map height in tiles")
	roomCols := flag.Int("rooms-h", 4, "number of room columns (horizontal grid)")
	roomRows := flag.Int("rooms-v", 4, "number of room rows (vertical grid)")
	extraConn := flag.Int("extra-connections", 2, "random inter-room corridors beyond the spanning tree")
	flag.Parse()

	if *count <= 0 {
		fatal("-n must be > 0, got %d", *count)
	}

	if err := os.MkdirAll(*outDir, 0o755); err != nil {
		fatal("create output dir %q: %v", *outDir, err)
	}

	tg := service.NewTopologyGenerator()
	for i := 0; i < *count; i++ {
		s := *seed + int64(i)
		play := model.NewPlaythrough(model.PlaythroughId(fmt.Sprintf("dump-%d", i)), 1, s)
		play.Map = &model.Map{}
		ctx := model.NewSessionContext(play)

		tg.Gen(ctx, *mapW, *mapH, *roomCols, *roomRows, *extraConn)
		if ctx.Playthrough.Map == nil {
			fatal("generation returned nil map at index %d (seed=%d)", i, s)
		}

		dump := dto.ToTopologyDump(ctx.Playthrough.Map)
		path := filepath.Join(*outDir, fmt.Sprintf("%04d.json", i))
		if err := writeJSON(path, dump); err != nil {
			fatal("write %s: %v", path, err)
		}
	}

	fmt.Printf("[dump-topology] wrote %d maps to %s (base seed=%d)\n", *count, *outDir, *seed)
}

func writeJSON(path string, v any) error {
	f, err := os.Create(path)
	if err != nil {
		return err
	}
	defer f.Close()
	enc := json.NewEncoder(f)
	return enc.Encode(v)
}

func fatal(format string, a ...any) {
	fmt.Fprintf(os.Stderr, "[dump-topology] "+format+"\n", a...)
	os.Exit(1)
}
