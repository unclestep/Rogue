# Hot — current status (2026-04-26)

## Active branch
`feature/advanced_pipeline` (off `develop`)

## Recent work (last commits)
- `d73a967` feat: default topologies for learning, but can be regenerated
- `2b9cd31` weights after learning with channel attention and other minor improvements
- `531ea73` feat: CNN with dilated conv + SE, cosine LR schedule, 5-step topology-aware curriculum (small-medium-full)
- `41f45e9` feat: new channels and scalars
- `ef3acbf` new weights

**Theme**: RL observation pipeline upgrade — new channels (ChInterceptPath=11, ChVisitedCorridorPath=12, ChFuturePlayerPos=13), new scalars (Map_Position_X/Y at s[16-17], polar distance/angle at s[18-19]). Total: 14 channels, 20 scalars → ObservationSize=1714.

## Uncommitted changes
- `rl/models/pursuer.onnx` (modified) — new trained weights
- `rl/models/pursuer.onnx.data` (modified) — new trained weights data  
- `rl/pursuer_training.ipynb` (modified) — training notebook changes (CNN with dilated conv + SE, cosine LR, 5-step curriculum)

## Current state of RL training
- Architecture: PursuerResCNN (CoordConv + ResBlock + SEBlock + dilated conv)
- LR schedule: cosine annealing 1e-4 → 1e-5 over 3M steps
- Curriculum: 5-stage topology-aware (small 2x2 → medium 3x3 → full 4x4)
- Observation: 14ch × 11×11 + 20 scalars = 1714 floats
- Algorithm: QR-DQN (sb3_contrib), 51 quantiles, AdamW optimizer

## Training run summary (from TensorBoard analysis)
| Run | Obs size | Steps | Catch rate | Reward | Notes |
|-----|----------|-------|------------|--------|-------|
| QRDQN_1 | 1101 (9ch+12s) | 2M | 0.751 | 12.8 | Loss diverges (0.67→6.92), partial curriculum |
| QRDQN_2 | unknown (scalars added) | 3M | 0.812 | 13.083 | Best reward, stable loss, no curriculum advancement |
| QRDQN_3 | 1714 (14ch+20s) | 3M | 0.799 | ~12.7 | CNN+SE+dilated+cosine, loss diverges (2.34→8.11), 4 curriculum stages |

Final evaluation (QRDQN_3):
- QR-DQN: 93.3% catch, 12.65 return, 49.1% ambush
- Dijkstra baseline: 93.3% catch, 12.77 return, 52.6% ambush
- Per-profile: stone 86.7%, dummy 86.7%, default 76.7%, aggressive 80.0%, cautious 73.3%, timid 70.0%

## Known bugs (found in session 2026-04-25)
1. **ONNX parity failure**: Export uses opset_version=17 but torch silently upgrades to 18,
   causing `max|PyTorch - ONNX| = 1.14e-04`. Fix: set `opset_version=18` explicitly in export call.
2. **`rl/tools/gen_tiny_onnx.py` stale**: `OBS_SIZE = 1101` (9ch×121+12s). Should be 1714.
   Go CI uses this to generate fake ONNX for tests — wrong size breaks tests when ONNX path is set.
3. **`rl/tests/test_observation.py` stale docstring**: says "total length must be 1101" (actual: 1714).
4. **Final evaluation uses wrong topology pool**: notebook eval cell uses `topologies_small`
   instead of `topologies_full` — explains the tie with Dijkstra (small maps favor simple chasing).
5. **QR-DQN loss divergence in QRDQN_3**: loss climbs 2.34→8.11 in second half of training,
   suggesting LR too high after cosine decay or the new channels overwhelm the current buffer/batch config.

## Key invariants to maintain
1. `BuildObservation` (Go) ↔ `PursuerEnv._observe` (Python) must stay byte-identical.
   After any observation change: regenerate parity fixtures + `make test-rl`.
2. `ObservationChannels=14`, `ObservationScalars=20`, `ObservationSize=1714` are
   shared constants — changing them requires updating both Go and Python.
3. ONNX input name = `"input"`, output name = `"logits"` (see `rl_onnx.go`).
4. Flashlight: half-FOV=45°, range=3 cells (tuned for stealth-horror feel in PR B).

## Nothing blocking
- Go tests pass with FallbackPolicy (no ONNX needed for CI).
- CI installs ORT 1.23.0 dynamically via `ROGUE_ONNX_LIB_PATH`.

## Docs updated (2026-04-26)
- `README.md` — rewritten: brief game description, policy table, architecture overview, parity summary
- `rl/README.md` — rewritten as full technical writeup: why QR-DQN over PPO, env design, reward
  structure, observation tensor, PursuerResCNN architecture, training config, curriculum, topology
  generation, Go<->Python parity, ONNX export, testing
- `Makefile` — dump-topologies split into three separate targets (full/medium/small) with correct
  paths, seeds and room grid parameters; old single target replaced

## Next likely tasks
- Fix the 5 known bugs above (especially ONNX opset and gen_tiny_onnx OBS_SIZE).
- Re-run final evaluation on `topologies_full` to get a meaningful QR-DQN vs Dijkstra comparison.
- Commit/PR the new ONNX weights and updated training notebook to `feature/advanced_pipeline`.
- Potentially merge `feature/advanced_pipeline` -> `develop` once evaluation passes.
