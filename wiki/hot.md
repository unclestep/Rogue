# Hot - current status (2026-04-26 session 2)

## Active branch
`feature/advanced_pipeline` (off `develop`).

## Recent commits (unchanged this session - all work below is uncommitted)
- `2a8ea89` fix: actualize readme
- `9aa3d72` weights after 3d successful learning
- `d73a967` feat: default topologies for learning, but can be regenerated
- `2b9cd31` weights after learning with channel attention and other minor improvements
- `531ea73` feat: CNN with dilated conv + SE, cosine LR schedule, 5-step topology-aware curriculum (small-medium-full)

## Theme of the branch
RL observation pipeline: 14 channels x 11x11 + 20 scalars = 1714 floats.
PursuerResCNN. 5-stage topology-aware curriculum. Cosine LR.

## What changed in this session

### Fixed without retrain (effective immediately)
- **B1**: ONNX parity cell now asserts argmax stability over 1000 samples
  + a soft `< 1e-3` numerical bound. Hard invariant, not threshold whack-a-mole.
- **B2**: `test_observation.py` docstring `"12 floats" -> "20 floats"`.
- **B4**: Per-step expensive maps (`chase_scent`, `intercept`, `future_path`)
  now cached on `player_pos`; duplicate `chase_scent_map` call in
  `_intercept_stats` removed. **Measured 3.0x speedup on stone, 1.34x on
  default** -> expect overall ~2x training wall-time reduction (6h -> ~3h).
- **B6**: `observation_space` per-element bounds (grid in [0,1], scalars in
  [-1,1]).
- **B9**: `game_loop.go` nil-snapshot guard with `log.Printf` warning.
- **B10**: Go version aligned at `1.26.1` across `go.mod`, CI, Dockerfile.
- **B11**: Frozen eval benchmark `rl/fixtures/eval_pool/` (150 maps,
  committed). `Makefile` target `dump-topologies-eval`. Notebook eval cell
  refactored: `n_episodes=200`, knobs hard-coded (independent of CONFIG),
  prints pool path + sha256[:12]. **Pinned baseline on `eval_pool/v1` is
  83.5% catch / 9.75 return / 60.7% ambush (Dijkstra, 200 ep)** - any
  future model can be compared directly to this.
- **B12** (newly discovered + fixed): `flashlight_mask` had a "fix" hardcoding
  `plane_scale = 1.0` for `half_fov_deg == 45.0` based on a wrong claim
  that Go's `math.Tan(pi/4)` returns 1.0 exactly (it returns
  `0.99999999999999977...`). The hack actually *introduced* a 1-ULP gap
  that flipped the DDA tie-break vs Go. Removed. **All 5 parity scenarios
  now pass; full Python suite 54/54 green.** This is the first time
  Python<->Go parity has been clean on the cone since at least PR B.

### Fixed in code, verify-on-retrain
- **B3**: `CurriculumCallback._flush_replay_buffer()` drops oldest 70% of
  the replay buffer on each stage promotion (key recent 30% physically
  rotated to front). Should kill the loss spike at stage transitions.
- **B5**: Curriculum thresholds + budgets:
  - `curriculum_stone_catch`: 0.85 -> 0.70
  - `curriculum_stone_rew`: 10.0 -> 9.0
  - new `curriculum_min_steps_per_stage = 300_000` (consolidate skill)
  - new `curriculum_max_steps_per_stage = 800_000` (force promotion)
  Each of 5 stages gets between 300K and 800K of the 3M budget.
- **B7**: Reward rebalance - ambush bonus +1.5 -> +3.0 (overpowers the
  ~1.0 cumulative approach-shaping noise). Single change, conservative.
- **B8**: Stage-3/4 spawn radius widened (3, 8) -> (3, 15) so the policy
  practices longer chases that the Director hands off after frustration.

## Test status (after all fixes)
- Python: **54/54 pass** (incl. the 5 parity scenarios that were red).
- Go: all 6 test packages green.

## Tensorboard analysis (QRDQN_3, used to motivate B3/B5)
- FPS: collapses 657 -> 141 in first 5% of training, then plateaus.
  -> per-step constant cost (= the BFS bottleneck B4 fixed).
- Loss: spikes from ~3 -> **13.6 at step 1.0M**, exactly when stage 1 fires
  (1.04M). Smoking gun for B3.
- Curriculum: stage 0 dwells 35% of budget; stages 2-4 fire in last 9%.
  Smoking gun for B5.
- Catch rate: peaks at 0.863 @ step 1M, regresses to 0.799 by 3M.

## Uncommitted working-tree changes
- `rl/pursuer_env.py` - B4 caches, B6 bounds, B7 reward, B12 cone fix,
  `set_difficulty` `spawn_radius` param.
- `rl/pursuer_training.ipynb` - B1 argmax assertion, B3 buffer flush,
  B5 curriculum tuning, B8 stage spawn radii, B11 eval cell rewrite.
- `rl/tests/test_observation.py` - B2 docstring.
- `internal/application/game_loop.go` - B9 nil guard + log import.
- `go.mod`, `Dockerfile` - B10 Go version alignment.
- `Makefile` - B11 `dump-topologies-eval` target.
- `rl/fixtures/eval_pool/` (new directory, 150 .json maps) - B11 frozen pool.

## Key invariants to preserve
1. `BuildObservation` (Go) = `PursuerEnv._observe` (Python), byte-identical.
   Now true on all 5 parity scenarios. After any observation change:
   `make test-rl` (which runs the Go `dump-observation` cmd internally).
2. `ObservationChannels=14`, `ObservationScalars=20`, `ObservationSize=1714`
   shared between `internal/domain/service/rl_observation.go` and
   `rl/pursuer_env.py`.
3. ONNX input name `"input"`, output `"logits"` (`rl_onnx.go`).
4. Flashlight: `FlashlightHalfFOV=45 deg`, `FlashlightRange=3` cells.
5. **Eval benchmark**: only `eval_pool/v1` (sha256[:12]=`0d940a0bfd82`).
   Bumping the version requires `make dump-topologies-eval` + bumping
   `EVAL_POOL_VERSION` in the notebook.
6. Director->RL handoff: `rlTriggerRadius=8`, `rlTriggerScent=0.25`,
   `rlFrustrationLimit=15` (`logic_monster_pursuer.go`).

## Next steps
1. **Retrain** with the new code. Expected: ~3h wall (vs 6h) thanks to B4.
   First-time evidence on the corrected cone (B12).
2. After retrain, evaluate on `eval_pool/v1`. Compare vs pinned baseline
   83.5% catch / 9.75 return / 60.7% ambush.
3. If catch >= baseline, ship `feature/advanced_pipeline -> develop`.
4. If catch < baseline, **first** check whether B3 buffer-flush actually
   suppresses the loss spike at stage transitions (TB tag
   `curriculum/buffer_after_flush`); **then** consider further reward tuning.
5. Open issues: see [[bugs]] entries B3/B5/B7/B8 (`verify-on-retrain`).
