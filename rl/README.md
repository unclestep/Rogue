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
    topologies/*.json    # committed BSP dungeons — parity across machines
  topologies/*.json      # bulk training dump (gitignored)
  models/pursuer.onnx    # trained policy — committed
  runs/                  # TensorBoard logs (gitignored)
  checkpoints/           # PPO checkpoints (gitignored)
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
during training matches the real game. Commit or regenerate at will:

```
make dump-topologies                 # 1000 maps → rl/topologies/ (gitignored)
```

A small deterministic subset under `rl/fixtures/topologies/` is committed so
CI and local tests always have something to load.
