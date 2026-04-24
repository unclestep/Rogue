# Pursuer RL training

Python training harness for the Pursuer monster. Produces
`rl/models/pursuer.onnx`, which the Go game loads through
`internal/domain/service/rl_onnx.go` (set `ROGUE_PURSUER_MODEL_PATH=…`).

## Layout

```
rl/
  pursuer_env.py         # Gymnasium env, 1:1 with Go's BuildObservation
  scripted_player.py     # random wander + flee-on-adjacency opponent
  pursuer_training.ipynb # main notebook (runs in Colab or local Jupyter)
  tools/
    gen_tiny_onnx.py     # test fixture generator (PR 3)
  fixtures/
    parity/*.json        # committed Go-vs-Python parity scenarios
    topologies/*.json    # bulk training dump, 2000 maps (gitignored, regen)
  models/pursuer.onnx    # trained policy (gitignored)
  runs/                  # TensorBoard logs (gitignored)
  checkpoints/           # QR-DQN checkpoints (gitignored)
```

## Quick start — Colab

Open `pursuer_training.ipynb` in Colab (there's a badge inside). The notebook
installs dependencies, writes the Python modules via `%%writefile`, trains
PPO, evaluates, and exports ONNX — no local setup needed.

## Quick start — local

```
cd /workspaces/Rogue
pip install -r rl/requirements.txt
make dump-topologies          # generate rl/topologies/*.json (Go-side)
jupyter lab rl/pursuer_training.ipynb
```

Or run a short training pass without Jupyter:

```
python -m rl.pursuer_env --smoke   # not implemented — use the notebook
```

## Parity with Go

Observation layout, action indices, raycaster DDA, LOS Bresenham and memory
decay are mirrored from `internal/domain/service/` one-to-one. Any change on
either side must update the other — there is no auto-generated contract.
The notebook's final cell runs 1000 random observations through both the
PyTorch policy and the exported ONNX graph and asserts `max(abs(diff)) < 1e-5`.

## Training topologies

The env loads dungeons produced by `cmd/dump-topology` so the BSP layout
during training matches the real game. The training fixture pool is
**gitignored**; regenerate before training on a fresh checkout:

```
go run ./cmd/dump-topology -n 2000 -seed 42 \
  -rooms-h 4 -rooms-v 4 -extra-connections 2 \
  -out rl/fixtures/topologies
```

Parameters:
- `-n 2000` — pool size. More variety reduces overfit risk across 3M training steps.
- `-rooms-h 4 -rooms-v 4` — 4x4 room grid (PR B upgrade). Default in rules.
- `-extra-connections 2` — extra random inter-room corridors beyond spanning tree, creates loops.

Parity fixtures under `rl/fixtures/parity/` ARE committed — those are small,
hand-crafted scenarios needed for Python<->Go byte-exact tests.
