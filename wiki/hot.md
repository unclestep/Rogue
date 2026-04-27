# Hot - current status (2026-04-27 session, evening)

## Active branch
`feature/advanced_pipeline` (off `develop`). Anchor branch
`qrdqn5-baseline` created at the same commit so QRDQN_5 weights remain
recoverable regardless of subsequent work.

## Theme of the branch
RL observation pipeline: 14 channels x 11x11 + 20 scalars = 1714 floats.
PursuerResCNN. 5-stage topology-aware curriculum. Cosine LR.
Stealth-shifted reward (Plan A).

**Goal**: beat Dijkstra on `ambush_ratio` (higher) and `in_cone` (lower) on
`eval_pool/v1`. **Achieved.** See [[plan]] Plan A (now closed) and [[logs]].

## Status: Plan A closed, ready to ship

QRDQN_5 evaluation (200 episodes, eval_pool/v1, sha256[:12]=`0d940a0bfd82`):

| metric         | baseline (Dijkstra) | QRDQN_5 | target | result |
|---|---|---|---|---|
| `return`       | 10.41 +/- 12.76     | 11.32 +/- 16.05 | — | over baseline |
| `catch`        | 0.835               | 0.685   | >= 0.70 | -1.5pp under gate |
| `in_cone`      | 0.167               | 0.127   | < 0.10  | < baseline, 2.7pp over target |
| `ambush_ratio` | 0.607               | **0.929** | > 0.65 | +32pp |
| `stealth`      | 0.440               | **0.802** | > 0.55 | +0.36 |
| `len`          | 19.3                | 25.4    | — | longer creep approaches |

Primary stealth metrics beat baseline with large margin. Catch dropped 7pp
relative to QRDQN_4 — accepted as a tradeoff (see [[logs]] decision entry).

## What changed in this session

### Plan A retrain (QRDQN_5)
- Reward shaped per [[plan]] Plan A steps 1+2 (catch terminal split, cone
  presence penalty). Step 5 added composite `stealth` column.
- 3M training steps, ~7h wall on local box.
- TB run `runs/QRDQN_5`. New tags `metrics/ambush_ratio` and
  `metrics/in_cone_ratio` confirmed: tail-mean ambush 0.915, in_cone 0.087
  in-distribution (eval-pool slightly harder).
- Loss divergence at stage-3 transition got *worse* (peak 56.86 @ 2.58M,
  vs 39.76 in QRDQN_4). Eval reward kept rising to step 2.99M, so policy
  not broken — Q-regression noisy. Open tech-debt: see [[plan]] Plan C.

### Wiki + docs
- [[plan]] rewritten: Plan A closed, Plans B/C added as deferred with full
  pros-vs-cons tables and trigger conditions, plus a future-retrain
  decision matrix.
- `rl/README.md`: reward structure updated to Plan A semantics
  (+20/+8 split, +3.0 ambush, -0.05 cone-presence). Test list corrected
  (no more nonexistent `test_env.py`). `models/pursuer.onnx` correctly
  marked as force-tracked.
- Root `README.md`: fact-checked, no drift found.
- `.gitignore`: added `.pylocal/` and `.claude/` (local-only artifacts).

### Parity verification
- `rl/tests/test_parity.py` ran clean: 5/5 scenarios PASS. Go module cache
  permission issue resolved on host (`chown -R user:user /home/user/go`).
- `pip install --target /app/.pylocal pytest` for the test runner; runs
  via `PYTHONPATH=/app/.pylocal:/app /app/.pylocal/bin/pytest ...`.

## Test status
- Python (non-parity): 51/51 pass (last verified 2026-04-27 morning).
- Parity (Go<->Python observation): 5/5 PASS this session.
- Go: not re-run this session; no Go code changed.

## Uncommitted working-tree changes
- `rl/pursuer_env.py` — Plan A reward (catch terminal split, cone-presence).
- `rl/pursuer_training.ipynb` — MetricsCallback ambush/in_cone tags,
  eval-cell stealth column.
- `rl/tests/test_reward_and_player.py` — terminal split + cone-presence tests.
- `rl/models/pursuer.onnx{,.data}` — QRDQN_5 weights.
- `rl/README.md` — reward + tests sections.
- `wiki/plan.md`, `wiki/logs.md` (new files), `wiki/hot.md`,
  `wiki/index.md`, `CLAUDE.md`.
- `.gitignore` — `.pylocal/`, `.claude/`.
- `.obsidian/workspace.json` — IDE noise.

## Key invariants to preserve
1. `BuildObservation` (Go) = `PursuerEnv._observe` (Python), byte-identical.
   Parity verified 2026-04-27.
2. `ObservationChannels=14`, `ObservationScalars=20`, `ObservationSize=1714`.
3. ONNX input name `"input"`, output `"logits"`.
4. Flashlight: `FlashlightHalfFOV=45 deg`, `FlashlightRange=3` cells.
5. **Eval benchmark**: only `eval_pool/v1` (sha256[:12]=`0d940a0bfd82`).
6. Director->RL handoff: `rlTriggerRadius=8`, `rlTriggerScent=0.25`,
   `rlFrustrationLimit=15`.
7. **Anchor**: branch `qrdqn5-baseline` pins QRDQN_5 weights for revert.

## Next steps
1. Commit current state (code + weights + wiki) on `feature/advanced_pipeline`.
2. Open PR `feature/advanced_pipeline -> develop`.
3. Plans B and C remain deferred ([[plan]]). Trigger only under specific
   conditions documented there. Default action is "do not retrain".
4. Open tech-debt: stage-3 loss divergence (Plan C). Bundle with any
   future retrain.
