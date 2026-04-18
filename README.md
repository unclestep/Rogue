# Gouge

Gouge - Golang implementation of Rouge.

## Pursuer and RL integration

The Pursuer monster picks its next action through a `service.Policy`
interface (`internal/domain/service/rl_policy.go`). Three implementations
share the interface:

- `ONNXPolicy` — runs the PPO-trained model via `yalue/onnxruntime_go`.
  Loaded lazily when `ROGUE_PURSUER_MODEL_PATH` points at a valid
  `.onnx` file and the ONNX Runtime shared library resolves.
- `FallbackPolicy` — wraps `ChaseBehavior`'s Dijkstra gradient so the game
  keeps running without a model. This is what starts by default and what
  CI/tests use when ORT is absent.
- `ScriptedPolicy` — replays a fixed action sequence; test-only.

Switching policies at runtime:

```
# Fallback (default, no model)
go run ./cmd

# ONNX
export ROGUE_PURSUER_MODEL_PATH=/absolute/path/to/pursuer.onnx
# ORT shared library — if not on the default loader path
export ROGUE_ONNX_LIB_PATH=/absolute/path/to/onnxruntime.so
go run ./cmd
```

Both envs are read in `cmd/main.go`; missing/broken model or library
soft-fail back to `FallbackPolicy` with a warning rather than crashing.

### Training

Training lives under `rl/`. See `rl/README.md` for the full workflow.
The short version: dump real BSP topologies from Go with
`make dump-topologies`, then run `rl/pursuer_training.ipynb`
(Colab or local Jupyter) to train PPO and export
`rl/models/pursuer.onnx`.

### Parity Go <-> Python

`BuildObservation` (Go) and `PursuerEnv._observe` (Python) must produce
byte-identical tensors. Two checks guard this:

- `rl/tests/test_parity.py` dumps observations via `cmd/dump-observation`
  on committed fixtures and compares with the Python env.
- The final notebook cell runs the trained PyTorch policy and its
  exported ONNX through 1000 random states and asserts
  `max(abs(diff)) < 1e-5`.

### Evaluation

Policy quality is measured in Python — `rl/pursuer_training.ipynb`
cell 15 rolls out PPO and the scent-chase baseline through the same
env used for training and prints a comparison table.

`tests/application_test/pursuer_onnx_smoke_test.go` covers the Go side:
it wires `ONNXPolicy` through `MonsterControllerService` and verifies
the stack produces valid intents.
