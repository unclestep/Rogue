# Code conventions

## Go

### Formatting

- Formatter: **gofumpt** (stricter superset of gofmt). CI enforces it.
- Run locally: `gofmt -w .` (Makefile `fmt` target) or `gofumpt -w .`.
- Linter: `go vet ./...`.

### File naming

Domain services follow consistent prefixes:
- `gen_*` — procedural generators (topology, dungeon, spawner, door locker)
- `play_*` — gameplay action handlers (movement, combat, interact, item pickup/usage)
- `res_*` — resolvers (impact, movement side-effects)
- `sys_*` — cross-cutting systems (pathfinder, raycaster)
- `rl_*` — RL-specific code (observation, ONNX policy, policy interface)
- `logic_monster_*` — monster AI behaviours

### Package structure

- `internal/` — private to this module; never import from outside.
- `pkg/` — reusable, no internal imports; safe to import from anywhere.
- Test files live in `tests/` (separate package, black-box testing), not next to
  the source files. The one exception is `ui_render_test.go` / `ui_states_test.go`
  in the `tui` package (whitebox tests for rendering).

### Dependency injection

Google Wire is used. `bootstrap/wire.go` declares the injectors;
`bootstrap/wire_gen.go` is the generated output — **do not edit manually**.
After modifying providers, regenerate with `go run github.com/google/wire/cmd/wire ./internal/bootstrap/`.

### Error handling

- Constructors that can fail return `(T, error)`.
- Services/use-cases that cannot recover gracefully return errors to callers.
- `ONNXPolicy` soft-fails back to `FallbackPolicy` — no panic on missing model.
- Map setters log `[ERROR]` via `log.Printf` but do not panic.

### Enumerations

Enums are `int`-typed constants with `iota`.
Stringer methods for exported enums are auto-generated via `enumer`; the
output lives in `enums_enumer.go`.

### Cloning / immutability

Domain model objects that are passed across layers (snapshots to view, etc.)
are **deep-cloned** via `Clone()` methods. Maps and slices are always cloned
explicitly with `maps.Clone` / manual loops to prevent aliasing.

### Concurrency

- `GameLoop` uses a `sync.RWMutex` to protect the subscriber map.
- `ONNXPolicy` serialises inference calls with a `sync.Mutex` (session is not
  thread-safe).
- No other shared mutable state; use-cases are effectively single-threaded
  (one active goroutine drives `ResolveTurn`).

---

## Observation / RL parity rules

Any change to the observation layout in `rl_observation.go` **must** be
mirrored in `rl/pursuer_env.py` and vice versa. The constants at the top of
`rl_observation.go` are the single source of truth for channel indices,
crop size, and normalisation factors — the Python env imports them by value
from those constants. After any change:

1. Regenerate parity fixtures: `go run ./cmd/dump-observation ...`.
2. Run `make test-rl` to verify Python↔Go byte-equality.
3. Re-train or at minimum run the notebook's ONNX export cell.

---

## Python (`rl/`)

- Python 3.x; dependencies listed in `rl/requirements.txt`.
- Tests run with `pytest rl/tests -q` (`make test-rl`).
- Notebook (`pursuer_training.ipynb`) is the canonical training entry point.
- Helper tool `rl/tools/gen_tiny_onnx.py` generates a minimal fake ONNX model
  used by Go tests when the real trained model is absent.

---

## Testing conventions

### Go

- Unit tests: `tests/<layer>_test/` — separate package names (`_test` suffix or
  explicit `package foo_test`) to enforce black-box testing.
- Integration / smoke: `tests/application_test/` — wires up real services.
- Test helpers: `tests/usecase_test/helpers_test.go`.
- Use `ScriptedPolicy` to make Pursuer behaviour deterministic in tests.
- Use `MemorySessionRepo` (not JSON) for speed in unit tests.

### Python

- Parity tests (`test_parity.py`) depend on committed JSON fixtures under
  `rl/fixtures/parity/`; do not delete them.
- Domain-randomisation tests exercise BSP map diversity; they use seeded RNGs
  so results are deterministic.

---

## Makefile targets

| Target | Command | Notes |
|--------|---------|-------|
| `make run` | `go run ./cmd` | Start game (FallbackPolicy) |
| `make test` | `go test ./...` | Go tests |
| `make dump-topologies` | regenerates all three pools (small/medium/full) under `rl/fixtures/topologies*` | Run only if BSP generator changes |
| `make dump-topologies-full` | full pool: 2000 maps, 4×4 rooms, seed 42 | Output `rl/fixtures/topologies` |
| `make dump-topologies-medium` | medium pool: 500 maps, 3×3 rooms, seed 100 | Output `rl/fixtures/topologies_medium` |
| `make dump-topologies-small` | small pool: 500 maps, 2×2 rooms, seed 200 | Output `rl/fixtures/topologies_small` |
| `make run-notebook` | `jupyter lab rl/pursuer_training.ipynb` | Open training notebook |
| `make test-rl` | `python -m pytest rl/tests -q` | Python parity + env tests |
| `make build` | cross-compile for 8 platforms | Output in `build/` |
| `make release` | build + zip/tar | Output in `build/dist/` |
| `make fmt` | `gofmt -w .` | Format all Go files |
| `make lint` | `go vet + gofmt check` | CI lint step |

---

## Environment variables

| Variable | Used by | Default | Purpose |
|----------|---------|---------|---------|
| `ROGUE_PURSUER_MODEL_PATH` | `cmd/main.go`, `bootstrap/providers.go` | `""` | Path to `pursuer.onnx`; empty → FallbackPolicy |
| `ROGUE_ONNX_LIB_PATH` | `rl_onnx.go` | OS default | Path to `libonnxruntime.so` / `.dylib` |
