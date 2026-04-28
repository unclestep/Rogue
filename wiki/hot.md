# Hot - current status (2026-04-28 session)

## Active branch
`feature/advanced_pipeline` (off `develop`). Anchor branch
`qrdqn5-baseline` pins QRDQN_5 weights so any QRDQN_6 outcome is
recoverable with a single `git checkout`.

## Theme of the branch
RL observation pipeline: 14 channels x 11x11 + 20 scalars = 1714 floats.
PursuerResCNN. 5-stage topology-aware curriculum. Cosine LR.

**Goal**: beat Dijkstra on `ambush_ratio` (higher) and `in_cone` (lower) on
`eval_pool/v1`. **Achieved by QRDQN_5** (ambush 0.929, in_cone 0.127 vs
baseline 0.607 / 0.167). One additional retrain QRDQN_6 in flight to push
`in_cone < 0.10` and recover `catch >= 0.70`.

## Status: QRDQN_6 code prepared, awaiting training

Code in place. Tests 59/59 green. User confirmed the package; awaiting
the local 3-4h training run.

### Plan QRDQN_6 — combined B + C (single retrain)

Reward (`rl/pursuer_env.py:_compute_reward`):
- **B.1** — visible-kill terminal +8.0 -> +12.0. Softens 20:8 ratio that
  starved catch in QRDQN_5; ambush still preferred but visible kill is
  not economically punishing anymore.
- **B.2** — `+0.05` per step when `not in_cone AND chebyshev <= 3`.
  Encourages "creep up close while unseen", not just final cone-entry.
- **B.3** — approach shaping `0.02 * delta_chebyshev` replaced by
  `0.02 * clip(delta_covert_dist, -2, 2)`. Covert-Dijkstra (cone_penalty
  =10 per cone-cell) selects detours through dark corridors. Single
  Dijkstra per `step()`, reused by `_observe`.

Optimisation (`rl/pursuer_training.ipynb`):
- **C.1** — `lr_final` 1e-5 -> 3e-5. **Correction during this session**:
  earlier wiki claimed QRDQN_5's late-stage LR was "effectively zero" —
  wrong, it was 10% of init. Bumping floor to 30% of init gives ~3x more
  late-stage adaptation budget, which is the actual thing we want.
- **C.2** — `max_grad_norm=1.0` (was SB3 default 10). Caps gradient-spike
  step size 10x; symptomatic damping for stage-3 loss divergence.

### Frozen revert criteria (decided BEFORE training)

| metric | revert | ship |
|---|---|---|
| `ambush_ratio` | < 0.85 | >= 0.85 |
| `in_cone` | > 0.13 | <= 0.13 |
| `stealth` | < 0.75 | >= 0.78 |
| `catch` | < 0.65 | >= 0.70 |

ANY metric in revert column -> snap back to QRDQN_5 via
`git checkout qrdqn5-baseline -- rl/models/`. Loss is informational, not
a revert trigger. Probability estimates: ~25-30% strict-improve all 4 /
~50% net-positive ship / ~20-25% revert.

### Reference: QRDQN_5 baseline (200 ep, eval_pool/v1, sha256[:12]=0d940a0bfd82)

| metric | baseline (Dijkstra) | QRDQN_5 |
|---|---|---|
| `return` | 10.41 +/- 12.76 | 11.32 +/- 16.05 |
| `catch` | 0.835 | 0.685 |
| `in_cone` | 0.167 | 0.127 |
| `ambush_ratio` | 0.607 | **0.929** |
| `stealth` | 0.440 | **0.802** |

## Test status
- `rl/tests/test_reward_and_player.py`: 18/18 PASS (4 new tests for
  B.1/B.2/B.3: `test_terminal_reward_player_caught_visible` updated to
  +12.0; `test_approach_shaping_covert_delta`,
  `test_approach_shaping_covert_clipped`,
  `test_stealth_approach_bonus_close_and_unseen`,
  `test_stealth_approach_bonus_suppressed_in_cone`).
- Full RL suite: 59/59 PASS.
- Parity (Go<->Python): 5/5 PASS (verified 2026-04-27).
- Go: not re-run; no Go code changed.

## Uncommitted working-tree changes (post B+C prep)
- `rl/pursuer_env.py` — B.1+B.2+B.3 reward changes; covert-map cache
  pre-computed in `step()` and reused by `_observe`.
- `rl/pursuer_training.ipynb` — C.1 (lr_final 3e-5), C.2 (max_grad_norm
  =1.0), plus all the docs/run-history/ONNX-fix work from the prior
  session.
- `rl/tests/test_reward_and_player.py` — 4 new tests, _isolate updated.
- `rl/models/pursuer.onnx{,.data}` — still QRDQN_5 weights (will be
  rewritten by ONNX-export cell after QRDQN_6 finishes).
- `rl/README.md` — Plan A reward semantics (will need a small bump after
  QRDQN_6 if it ships).
- `rl/runs_summary/QRDQN_{1..5}.csv` — historical CSVs, tracked.
- `rl/tools/dump_run_summary.py` — TB->CSV exporter.
- `wiki/plan.md` — Plan QRDQN_6 with frozen revert criteria, plus
  Plans A (closed), B/C (now folded into QRDQN_6).
- `wiki/hot.md`, `wiki/logs.md`, `wiki/index.md`, `wiki/rl.md`,
  `CLAUDE.md`.
- `.gitignore` — `.pylocal/`, `.claude/`.
- `.obsidian/workspace.json` — IDE noise.

## Key invariants to preserve
1. `BuildObservation` (Go) = `PursuerEnv._observe` (Python),
   byte-identical. Verified 2026-04-27.
2. `ObservationChannels=14`, `ObservationScalars=20`, `ObservationSize=
   1714`. **Unchanged in QRDQN_6** (no observation layout changes).
3. ONNX input name `"input"`, output `"logits"`.
4. Flashlight: `FlashlightHalfFOV=45 deg`, `FlashlightRange=3` cells.
5. **Eval benchmark**: only `eval_pool/v1` (sha256[:12]=`0d940a0bfd82`).
6. Director->RL handoff: `rlTriggerRadius=8`, `rlTriggerScent=0.25`,
   `rlFrustrationLimit=15`.
7. **Anchor**: branch `qrdqn5-baseline` pins QRDQN_5 weights for revert.

## Next steps (waiting on user)
1. Run training: open `rl/pursuer_training.ipynb`, run all cells (3-4h).
2. Eval cell will print the comparison table.
3. Compare against the revert criteria above.
4. **If ship**: update [[hot]] + [[logs]] + [[rl]] + run history; commit
   QRDQN_6 weights + code.
5. **If revert**: `git checkout qrdqn5-baseline -- rl/models/`, log the
   experiment in [[logs]] with the actual numbers and a post-mortem
   against the recorded probability estimates, ship QRDQN_5 from where
   we paused.
