// Command dump-observation produces a byte-exact reference observation for a
// given canned scenario. The Python Gymnasium env (rl/pursuer_env.py) loads
// the same scenario JSON and asserts that its locally-built observation
// matches the one produced here.
//
// Scenario schema (JSON):
//
//	{
//	  "topology":   "rl/fixtures/topologies/0000.json",
//	  "pursuer":    {"pos": {"x":10,"y":5}, "hp_frac":0.8, "stamina_frac":0.6},
//	  "player":     {"pos": {"x":15,"y":7}, "aim_angle_rad":0.0},   // nullable
//	  "memory":     {"last_seen":{"x":14,"y":7}, "turns_since_los":3,
//	                 "trail":[{"x":9,"y":5},{"x":10,"y":5}]},
//	  "other_monsters": [{"x":20,"y":10}]
//	}
//
// Omit "player" (or set it to null) to simulate "no target" / haven't seen
// the player yet. Set memory.last_seen to {-1,-1} to mark memory empty.
package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"path/filepath"

	"github.com/unclestep/Rogue/internal/domain/model"
	"github.com/unclestep/Rogue/internal/domain/service"
	"github.com/unclestep/Rogue/internal/dto"
	"github.com/unclestep/Rogue/pkg/geometry"
)

type point struct {
	X int `json:"x"`
	Y int `json:"y"`
}

type pursuerSpec struct {
	Pos         point   `json:"pos"`
	HPFrac      float32 `json:"hp_frac"`
	StaminaFrac float32 `json:"stamina_frac"`
}

type playerSpec struct {
	Pos         point   `json:"pos"`
	AimAngleRad float64 `json:"aim_angle_rad"`
}

type memorySpec struct {
	LastSeen      point   `json:"last_seen"`
	TurnsSinceLOS int     `json:"turns_since_los"`
	Trail         []point `json:"trail"`
	PlayerTrail   []point `json:"player_trail"`
}

type scenario struct {
	Topology      string      `json:"topology"`
	Pursuer       pursuerSpec `json:"pursuer"`
	Player        *playerSpec `json:"player"`
	Memory        memorySpec  `json:"memory"`
	OtherMonsters []point     `json:"other_monsters"`
}

type output struct {
	Observation []float32 `json:"observation"`
}

func main() {
	scenarioPath := flag.String("scenario", "", "path to scenario JSON")
	outPath := flag.String("out", "", "path to write observation JSON (default: stdout)")
	flag.Parse()

	if *scenarioPath == "" {
		fmt.Fprintln(os.Stderr, "usage: dump-observation -scenario PATH [-out PATH]")
		os.Exit(2)
	}

	sc, err := loadScenario(*scenarioPath)
	if err != nil {
		die("loading scenario: %v", err)
	}
	topoPath := resolveTopologyPath(*scenarioPath, sc.Topology)
	topo, err := loadTopology(topoPath)
	if err != nil {
		die("loading topology %q: %v", topoPath, err)
	}

	m := dto.FromTopologyDump(*topo)

	const (
		pursuerID = model.ActorId(1000)
		playerID  = model.ActorId(1)
	)

	pursuer := model.NewDefaultPursuer(pursuerID, geometry.Point{X: sc.Pursuer.Pos.X, Y: sc.Pursuer.Pos.Y})
	applyFracs(pursuer, sc.Pursuer.HPFrac, sc.Pursuer.StaminaFrac)

	play := &model.Playthrough{
		Map:             m,
		Players:         make(map[model.ActorId]*model.Actor),
		Monsters:        make(map[model.ActorId]*model.Actor),
		PlayersFoW:      make(map[model.ActorId]*model.VisibleArea),
		PlayerAimAngles: make(map[model.ActorId]float64),
	}
	play.Monsters[pursuerID] = pursuer

	for i, mp := range sc.OtherMonsters {
		id := model.ActorId(2000 + i)
		other := model.NewDefaultPursuer(id, geometry.Point{X: mp.X, Y: mp.Y})
		play.Monsters[id] = other
		m.SetActor(other.Pos, int64(id))
	}
	m.SetActor(pursuer.Pos, int64(pursuerID))

	if sc.Player != nil {
		player := model.NewDefaultPlayer(playerID, geometry.Point{X: sc.Player.Pos.X, Y: sc.Player.Pos.Y}, 8)
		play.Players[playerID] = player
		play.PlayerAimAngles[playerID] = sc.Player.AimAngleRad
		m.SetActor(player.Pos, int64(playerID))

		rc := service.NewRaycaster()
		rc.RefreshPlayerFoW(play, playerID, service.FlashlightHalfFOV, service.FlashlightRange)
	}

	mem := buildMemory(sc.Memory)

	play.Seed = 42 // arbitrary seed for wander maps
	ctx := model.NewSessionContext(play)
	obs := service.BuildObservation(ctx, pursuer, mem)

	body, err := json.Marshal(output{Observation: obs})
	if err != nil {
		die("marshalling output: %v", err)
	}
	if *outPath == "" {
		os.Stdout.Write(body)
		os.Stdout.Write([]byte("\n"))
		return
	}
	if err := os.WriteFile(*outPath, body, 0o644); err != nil {
		die("writing output: %v", err)
	}
}

func applyFracs(a *model.Actor, hpFrac, stamFrac float32) {
	hp := int(float32(a.DerivedAttrs[model.AttrMaxHP]) * hpFrac)
	stam := int(float32(a.DerivedAttrs[model.AttrMaxStamina]) * stamFrac)
	a.Vitals[model.VitalHP] = hp
	a.Vitals[model.VitalStamina] = stam
}

func buildMemory(spec memorySpec) *service.PursuerMemory {
	mem := service.NewPursuerMemory()
	if spec.LastSeen.X >= 0 && spec.LastSeen.Y >= 0 {
		mem.LastSeen = geometry.Point{X: spec.LastSeen.X, Y: spec.LastSeen.Y}
	}
	mem.TurnsSinceLOS = spec.TurnsSinceLOS
	for _, p := range spec.Trail {
		mem.Trail = append(mem.Trail, geometry.Point{X: p.X, Y: p.Y})
	}
	for _, p := range spec.PlayerTrail {
		mem.PlayerTrail = append(mem.PlayerTrail, geometry.Point{X: p.X, Y: p.Y})
	}
	return mem
}

func loadScenario(path string) (*scenario, error) {
	raw, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	sc := &scenario{}
	if err := json.Unmarshal(raw, sc); err != nil {
		return nil, err
	}
	return sc, nil
}

func loadTopology(path string) (*dto.TopologyDump, error) {
	raw, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	out := &dto.TopologyDump{}
	if err := json.Unmarshal(raw, out); err != nil {
		return nil, err
	}
	return out, nil
}

// resolveTopologyPath treats topology as either absolute, or relative to the
// scenario file's directory, whichever resolves to an existing file.
func resolveTopologyPath(scenarioPath, topology string) string {
	if filepath.IsAbs(topology) {
		return topology
	}
	rel := filepath.Join(filepath.Dir(scenarioPath), topology)
	if _, err := os.Stat(rel); err == nil {
		return rel
	}
	return topology
}

func die(format string, args ...any) {
	fmt.Fprintf(os.Stderr, format+"\n", args...)
	os.Exit(1)
}
