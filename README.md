# Gouge

A turn-based multiplayer roguelike written in Go, featuring a trained RL-powered Pursuer monster.

## Game

Players explore procedurally generated BSP dungeons, collect items, fight enemies, and reach the exit.
Up to N players share one dungeon in-process — no network sockets, communication is channel-based.

The Pursuer is a special monster: a two-tier Director/Alien AI that switches between a Dijkstra-based wander phase and a neural policy. When the player comes within range (Chebyshev <= 8 cells) or a scent threshold is crossed, the RL policy takes over and attempts a stealth ambush.

## Running

```bash
# No model — Dijkstra fallback (default, works out of the box)
go run ./cmd

# With trained ONNX model
export ROGUE_PURSUER_MODEL_PATH=/path/to/rl/models/pursuer.onnx
export ROGUE_ONNX_LIB_PATH=/path/to/libonnxruntime.so   # if not on default loader path
go run ./cmd
```

Both env vars are optional. A missing or broken model soft-fails to FallbackPolicy with a log warning.

## Policy interface

Three implementations share the `service.Policy` interface (`internal/domain/service/rl_policy.go`):

| Implementation  | What it does                                                                 |
|-----------------|------------------------------------------------------------------------------|
| ONNXPolicy      | Runs the QR-DQN model via yalue/onnxruntime_go; loaded from ROGUE_PURSUER_MODEL_PATH |
| FallbackPolicy  | Wraps ChaseBehavior's Dijkstra gradient; default when no model is present    |
| ScriptedPolicy  | Replays a fixed action sequence; test-only                                   |

## Tooling

```bash
make test             # Go tests (no ONNX needed)
make test-rl          # Python parity + env tests
make dump-topologies  # regenerate RL training maps from Go BSP
make run-notebook     # open training notebook
make lint             # go vet + gofumpt
make build            # cross-compile for 8 platforms
```

## Architecture

Clean/Hexagonal layering: domain -> application -> presentation/infrastructure, wired by Google Wire.

```
cmd/              entry points
internal/
  domain/         pure business logic — no I/O, no frameworks
  application/    use cases and game loop
  bootstrap/      dependency injection (Wire)
  presentation/   TUI (BubbleTea) and in-process server
  infrastructure/ storage adapters (JSON, in-memory)
  dto/            cross-layer data transfer objects
pkg/              reusable project-agnostic libraries
rl/               Python QR-DQN training pipeline
tests/            black-box tests by layer
```

See `wiki/architecture.md` for the full architecture reference.

## Go <-> Python parity

`BuildObservation` (Go) and `PursuerEnv._observe` (Python) must produce byte-identical tensors.
Two guards:

- `rl/tests/test_parity.py` — runs `cmd/dump-observation` on committed fixtures and compares
  with the Python env output.
- Final notebook cell — runs the trained PyTorch policy and the exported ONNX through 1000
  random states and asserts max(abs(diff)) < 1e-5.

Any change to the observation layout must be mirrored on both sides and parity fixtures regenerated.

## RL training

See [rl/README.md](rl/README.md) for the full training pipeline: environment design, neural
architecture, curriculum, training configuration, and Go integration details.
