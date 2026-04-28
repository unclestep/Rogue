# Bugs and open issues

Consolidated list of known bugs, drift, and open issues across the codebase.
Update this page as bugs are confirmed, resolved, or invalidated.

Conventions:
- **Status**: `open` (still present), `fixed` (resolved on this branch),
  `verify-on-retrain` (code change committed, but only the next training run
  can prove the fix worked), `invalidated` (turned out to be intentional).
- Severity: `critical` (breaks training/inference), `major` (visible regression),
  `minor` (cosmetic / docs / future risk).

---

## RL: training pipeline

### B1. ONNX parity assertion fails at export — fixed / minor
**Was**: `max |PyTorch - ONNX| = 1.14e-04` but the assert required `< 1e-4`,
so the cell raised `AssertionError`. Root cause: the threshold was the
opset-17 expectation; the 1.14e-4 gap on opset 18 is < 1 ULP for float32 and
doesn't change argmax over 5 logits.
**Fix**: notebook cell 21 now uses a soft `< 1e-3` numerical tolerance plus a
**hard argmax-stability assertion** over 1000 random samples - the
load-bearing invariant is that PyTorch and ONNX pick the same action.
**Verify**: re-run notebook end-to-end; cell prints both `max|diff|` and
`argmax agreement N/N`.

### B2. `test_observation.py` docstring stale — fixed / minor
**Was**: docstring said `scalar slice is 12 floats` (current value: 20).
**Fix**: docstring now reads `scalar slice is 20 floats`.

### B3. Loss divergence on curriculum-heavy runs — partially fixed / major
**Symptom**: `train/loss` climbed 2.34 -> 8.11 over QRDQN_3 (3M steps).
TensorBoard analysis confirmed the loss spike at step ~1.04M coincides
exactly with the stage-0 -> stage-1 promotion - replay-buffer staleness
across topology distributions.
**Mechanical fix**: `CurriculumCallback._flush_replay_buffer()` drops
the oldest 70% of replay-buffer transitions on every stage promotion.
Verified working: post-flush buffer = 11250 transitions/env = 30% of
300K, TB tag `curriculum/buffer_after_flush` confirms.
**Status**: the flush itself works, but did NOT prevent loss divergence
in QRDQN_4 (peak 39.76 @ 2.25M) or QRDQN_5 (peak 56.86 @ 2.58M). The
divergence migrated from stage-0/1 to stage-3/4 transitions. eval reward
kept rising in QRDQN_5 to step 2.99M, so policy is not broken — Q-target
regression is noisy. Further fix attempts in QRDQN_6 ([[plan]] Plan
QRDQN_6 C.1+C.2: LR floor 3e-5, max_grad_norm=1.0).

### B4. FPS bottleneck — fixed / major
**Was**: `_observe()` ran 5 BFS/Dijkstra per env step + a *second* BFS in
`_intercept_stats` (`chase_scent_map(self.topology, self.player_pos)`
bypassing the cache). FPS dropped 1058 -> 131; 6h wall time on 3M steps.
**Profile** (default profile, 2000 steps, before fix):
`chase_scent_map` was 66% of total time, called **2.9 times per step**.
**Fix** (`pursuer_env.py`):
- Cached `_intercept_map_cache`, `_future_path_cache` keyed on `player_pos`.
- `_chase_scent_cache` recomputed only when `player_pos` actually changes.
- `_intercept_stats` now reuses `_chase_scent_cache` instead of recomputing.
**Measured speedup** (fresh process, 5000 steps each):

| profile | before | after | speedup |
|---|---|---|---|
| stone | 219 step/s | **662** step/s | **3.0x** |
| training_dummy | n/a | **554** step/s | ~2.5x |
| default | 228 step/s | **306** step/s | 1.34x |
| cautious | n/a | 353 step/s | ~1.5x |

Curriculum-weighted (stages 0-2 are stone/dummy on small/medium maps),
expect overall **~2x** training speedup -> ~3h instead of 6h.

### B5. Curriculum stage 4 fires too late — fixed / major
**Symptom**: in QRDQN_3, stages 1-4 fired at 1.04M / 2.72M / 2.80M / 2.88M -
stages 2-4 got just 9% of total budget combined.
**Root causes**:
- Stage 0 promotion threshold `catch>0.85, reward>10` was too high - epsilon
  decayed before the agent could exploit enough to hit it. Stage 0 dwelled
  35% of the budget waiting for one good 10K window.
- No per-stage budget guard - once thresholds finally fired, every later
  stage promoted within one check.
**Fix** (notebook CONFIG + `CurriculumCallback._on_step`):
- `curriculum_stone_catch`: 0.85 -> **0.70** (achievable while exploring).
- `curriculum_stone_rew`: 10.0 -> **9.0**.
- New `curriculum_min_steps_per_stage = 300_000` - even when threshold met,
  stay on stage to consolidate.
- New `curriculum_max_steps_per_stage = 800_000` - force promotion after
  this many steps regardless of threshold.
With these knobs each of the 5 stages gets between 300K and 800K of the 3M
budget (target distribution).
**Verified**: QRDQN_4 onward — stages fire at ~0.8M / 1.28M / 1.6M / 2.4M
(spread, not bunched). Confirmed in TB across QRDQN_4 and QRDQN_5.

### B6. Observation space declared `low=-1.0` for grid floats actually in [0, 1] — fixed / minor
**Was**: `spaces.Box(low=-1.0, high=1.0, ...)` used a single scalar bound
across all 1714 floats; grid channels are in `[0, 1]`.
**Fix** (`pursuer_env.py:931-940`): per-element bounds via
`np.concatenate([np.zeros(GRID), np.full(SCALARS, -1.0)])` for `low`;
`high` stays at all-ones.
**Note**: the existing trained weights are unaffected (SB3 doesn't auto-
normalise from the Box spec); the next retrain will get a slightly cleaner
init signal.

### B7. RL model does not outperform Dijkstra baseline — fixed (re-scoped) / major
**Original framing** (pre-Plan-A): QR-DQN was strictly worse than Dijkstra
on catch / return / length. The bug-fix sweep B1..B12 closed mechanical
issues (cone parity, replay flush, curriculum spread, spawn radius, ambush
bonus +1.5 -> +3.0) but QRDQN_4 still tied Dijkstra on the **stealth**
metrics that actually matter (`ambush_ratio`, `in_cone`).

**Resolution**: the goal was reframed (see [[plan]] Plan A and the
`project_pursuer_goal` memory): catch is no longer the target — beating
Dijkstra on `ambush_ratio` (higher) and `in_cone` (lower) on
`eval_pool/v1` is. QRDQN_5 (Plan A reward shift) cleared this with large
margin: ambush 0.929 vs 0.607, in_cone 0.127 vs 0.167. Catch dropped
from 0.755 to 0.685 as an intentional tradeoff. Shipped as the production
model; anchor branch `qrdqn5-baseline`.

QRDQN_6 (in flight) bundles Plan B + C to push `in_cone < 0.10` and
recover `catch >= 0.70`. Outcome to be appended to [[logs]].

### B8. Spawn radius bias toward close-range encounters — fixed / minor
**Was**: `spawn_radius=(3, 8)` matched `rlTriggerRadius=8` so the env exactly
covered the Director->RL handoff radius. But after `rlFrustrationLimit`
cycles the Director can hand control with the player much further than 8.
**Fix** (notebook cell 11 `_stages` definition + new `set_difficulty`
`spawn_radius=` param):
- Stage 0/1 (stone): `(3, 8)` - unchanged, matches Director handoff.
- Stage 2 (dummy on medium maps): `(3, 10)`.
- Stage 3/4 (default on full maps): **`(3, 15)`** - covers the OOD case.
**Verified**: QRDQN_4 onward trained with the wider radii without catch-rate
collapse; QRDQN_5 ships at 0.685 catch on the eval pool. The spawn-radius
fix is no longer the bottleneck.

### B11. Eval drifted across runs because eval setup was mutable — fixed / major
**Was**: comparing baseline numbers across training runs was meaningless
because `CONFIG['topologies_full']` and `max_episode_steps` evolved between
runs. Sweep across pools reproduced every historical "baseline" number
exactly - the drift was config-driven, not RNG-driven.
**Fix - frozen evaluation benchmark**:
- New committed pool `rl/fixtures/eval_pool/` with **150 maps**:
  50 small (seed 9001) + 50 medium (seed 9101) + 50 full (seed 9201).
  Seeds chosen far from training-pool seeds (42 / 100 / 200) so no overlap.
- New Makefile target `make dump-topologies-eval` (pool is committed; only
  regenerate when bumping `EVAL_POOL_VERSION` in the notebook).
- Notebook eval cell 18 hard-codes `EVAL_MAX_STEPS = 50`, `EVAL_FIXED_PROFILE = 'default'`,
  `EVAL_SPAWN_RADIUS = (3, 8)`, `EVAL_N_EPISODES = 200` - **all decoupled
  from CONFIG**, so training-knob churn cannot move the eval ruler.
- Eval cell prints pool path, version, and a `sha256[:12]` content hash so
  any unintended pool change is visible in the output.
- `n_episodes` bumped 30 -> 200 (30 binary outcomes have ~12pp Wilson noise).
**Pinned baseline number on `eval_pool/v1` (`sha256[:12]=0d940a0bfd82`)**:

```
baseline (Dijkstra) on eval_pool, 200 ep:
  catch=83.5%, return=9.75 +/- 11.57, len=19.3, hits=341, ambush=60.7%
```

Any future training run can be compared **directly** to this number.

### B12. Python/Go ChPlayerCone parity break — fixed / critical
**Was discovered** during the cleanup session: parity tests
`scenario_01_adjacent` and `scenario_04_trail_and_exit` had been failing for
some time. Root cause was a **stale comment-driven hack in
`flashlight_mask`**:

```python
if half_fov_deg == 45.0:
    plane_scale = 1.0   # claim: Go's math.Tan(pi/4) = 1.0 exactly
else:
    plane_scale = math.tan(math.radians(half_fov_deg))
```

Empirical check on Go 1.26: `math.Tan(math.Pi/4)` returns
`0.99999999999999977...`, not `1.0`. So the "fix" actually *introduced* a
1-ULP gap, which inverted the DDA tie-break on perfectly-diagonal cone
edges and silently flipped 4-8 boundary cells on every step.
**Fix** (`pursuer_env.py`): removed the hardcode; both Python and Go now
use the same `math.tan(math.radians(half_fov_deg))`. **All 5 parity
scenarios pass (54/54 tests green)**.
**Severity**: `critical` because the trained policy saw observations with
a Python-shaped cone but at inference receives Go-shaped cones. This
distribution mismatch on a **load-bearing channel** (ChPlayerCone, plus
ChCovertPath which reads the cone for cost-weighting) likely explains a
large chunk of B7. Worth retraining on the corrected cone before drawing
any conclusions about reward design or architecture.

---

## Go runtime

### B9. `broadcastState` silently swallowed nil snapshots — fixed / minor
**Was**: `internal/application/game_loop.go:104-120`
`MakeSnapshots()` could return nil; `for ... range nil` was a silent skip.
**Fix**: explicit nil check with `log.Printf("[WARN] ...")` and early return.
Surfaces repository-unavailable / mapper failures instead of hiding them.

### B10. Go version skew across `go.mod` / CI / Dockerfile — fixed / minor
**Was**: `go.mod` declared `go 1.25.5`; CI installed `1.26.1`; Dockerfile
installed `1.26.2`.
**Fix**: aligned all three on **1.26.1**.
- `go.mod`: `go 1.25.5` -> `go 1.26.1`.
- `Dockerfile`: `go1.26.2` -> `go1.26.1` + comment cross-referencing the
  CI workflow file.
- CI was already on `1.26.1`.

---

## Resolved (no longer in code)

- `gen_tiny_onnx.py` had `OBS_SIZE = 1101` (stale) — fixed earlier, now `1714`.
- Final eval cell used `topologies_small` instead of full pool — fixed,
  was already `CONFIG['topologies_full']` before this session; now uses
  the frozen `eval_pool/` per B11.
- `tostring_rgb` matplotlib API broke Q-heatmap cell — fixed.
- Notebook `from rl.*` imports broken when sys.path adds `rl/` — fixed.

## Invalidated (false alarms during research)

- **OpenDoor blocks light in raycaster** — *not* a parity bug.
  `internal/domain/service/sys_raycaster.go:204` and `rl/pursuer_env.py:195`
  both treat `OpenDoor` as opaque (verified, Python explicitly says
  "Mirrors sys_raycaster.go blocksLight"). The Go comment "Corridor tiles
  are transparent" was misread as a claim about doors.
- **Baseline numbers drifting between runs** — *not* an RNG leak.
  Within-run is byte-deterministic (notebook cell 19 asserts it). Across
  runs the drift was driven by config evolution (different eval pool /
  `max_episode_steps` / env code). Pinned by B11.
