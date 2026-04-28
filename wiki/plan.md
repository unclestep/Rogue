# Plan

Active plans and proposals. Each entry: status (proposed / in-progress / done /
deferred / dropped), goal, concrete steps with verification criteria. Move
completed plans to [[logs]] when finished.

---

## Plan QRDQN_6 — combined B + C retrain (in-progress, 2026-04-28)

**Status**: code in place, awaiting training. This is a single-shot retrain
that bundles every deferred change from Plans B and C below into one run,
so we pay one 3-4h compute cost rather than two. User-approved. Anchor
branch `qrdqn5-baseline` already pins QRDQN_5 weights; revert is one
checkout away if any criterion below trips.

### What changed in the codebase

Reward (`rl/pursuer_env.py:_compute_reward`):
- **B.1** — visible-kill terminal raised from `+8.0` to `+12.0`. The
  20:8 ratio in QRDQN_5 starved catch (-7pp); 20:12 keeps ambush as the
  preferred outcome but lets the agent close out uncertain visible kills
  it had been declining.
- **B.2** — per-step `+0.05` stealth-approach bonus when
  `not in_cone AND chebyshev_to_player <= 3`. Encourages "creep up close
  while unseen" rather than only rewarding the final cone-entry.
- **B.3** — approach shaping `0.02 * delta_chebyshev` replaced by
  `0.02 * clip(delta_covert_dist, -2, 2)`. Covert-Dijkstra from pursuer
  with cone_penalty=10 produces a gradient that selects detours through
  dark corridors when those exist, instead of straight-line chase.
- Covert map is now computed once in `step()` after cone update, used
  by both `_compute_reward` (B.3 lookup) and `_observe` (ChCovertPath
  channel). Single Dijkstra per step, two consumers.

Optimisation (`rl/pursuer_training.ipynb`):
- **C.1** — `lr_final` raised from `1e-5` to `3e-5`. Earlier wiki text
  claimed QRDQN_5's late-stage LR was "effectively zero" — wrong: it was
  10% of init. The actual issue is that 10% of init is still small
  enough that stage-3/4 distribution shift cannot be absorbed; bumping to
  30% gives ~3x the late-stage adaptation budget.
- **C.2** — `max_grad_norm=1.0` (was SB3 default 10.0). Caps the
  effective LR step on gradient spikes by 10x; symptomatic damping on
  the stage-3 loss divergence (peak 56.86 in QRDQN_5).

### Frozen revert criteria

Recorded BEFORE training so the ship/revert decision is deterministic.

| metric | revert (snap back to QRDQN_5) | ship as QRDQN_6 |
|---|---|---|
| `ambush_ratio` | < 0.85 | >= 0.85 |
| `in_cone` | > 0.13 | <= 0.13 (project-goal win if < 0.10) |
| `stealth` (composite) | < 0.75 | >= 0.78 |
| `catch` | < 0.65 | >= 0.70 |
| `loss tail-mean` (last 10%) | (informational only) | < 25 = ship + close Plan C |

**Revert if ANY of `ambush_ratio`, `in_cone`, `stealth`, `catch` falls in
the revert column.** No partial wins. Loss is informational — even if
loss is still divergent, we ship if the four behavioral metrics all clear.

Revert command (one-liner):
```
git checkout qrdqn5-baseline -- rl/models/pursuer.onnx rl/models/pursuer.onnx.data
```

Then close this Plan as "experiment, did not ship" and add a [[logs]]
entry with the QRDQN_6 metrics for posterity.

### Probability estimates (recorded for honest post-mortem)

Forecast by Claude before training:

| outcome | probability |
|---|---|
| strict improvement on all 4 primary metrics | ~25-30% |
| net-positive (most improve, none cross revert thresholds) | ~50% |
| meaningful regression -> revert | ~20-25% |

Test these against the actual outcome in [[logs]] post-eval.

---

## Plan A — Stealth-shift reward — DONE (2026-04-27, QRDQN_5)

Closed: ambush 92.9% (target > 65%), in_cone 0.127 (target < 0.10, missed by
2.7pp but well below baseline 0.167), stealth composite 0.802 (target > 0.55).
Catch 68.5% (gate was >= 70%, missed by 1.5pp — not a primary metric).
Full result + decision in [[logs]] entry "QRDQN_5 evaluation". Steps 3 and 4
of the original plan were not needed and are folded into Plan B below as
deferred levers.

---

## Plan B — Recover catch without losing stealth (deferred)

**Status**: deferred. Apply only if a play-test or new product requirement
flags QRDQN_5's 68.5% catch as too low. Do NOT run as a "while we're here"
polish — see "Pros vs cons" below.

### Goal

Pull catch back into 0.72-0.78 range while keeping ambush_ratio > 0.85 and
in_cone < 0.13. The shipped QRDQN_5 already beats Dijkstra on every primary
metric, so this is a tradeoff knob, not a bug fix.

### Hypothesis

Two shaping signals over-favour ambush at the cost of close-range visible
kills:

1. Catch terminal split is 20.0 (ambush) vs 8.0 (visible) — ratio 2.5:1.
   Agent appears to abandon visible-kill opportunities that would still
   succeed, hoping for an ambush window that doesn't always come.
2. There is no per-step bonus for "close and unseen". The agent gets +3.0
   only on the cone-entry tick, nothing for sustained covert approach.

### Steps (apply both at once; one retrain)

#### B.1 — Soften terminal split to 20:12

`pursuer_env.py:_compute_reward` — change visible-kill terminal from `+8.0`
to `+12.0`. Ratio drops from 2.5:1 to 1.67:1. Ambush still preferred but
visible-kill stops being economically punishing.

Update `tests/test_reward_and_player.py::test_terminal_reward_player_caught_visible`
expected value from 7.98 to 11.98.

#### B.2 — Stealth-approach per-step bonus (was Plan A step 4)

Add to dense reward path in `_compute_reward`:

```
+0.05  per step if (not in_cone_now) AND (chebyshev_distance(pursuer,player) <= 3)
```

Bounded by `<= 3` so it cannot be farmed at distance. Encourages "creep up
close while unseen" rather than only rewarding the final cone-entry.

Add `tests/test_reward_and_player.py::test_stealth_approach_bonus`.

### Pros vs cons (weighted)

| factor | for | against |
|---|---|---|
| Recovers catch | likely +5-10pp (modeled by 1.67:1 ratio + close-and-unseen pull) | stochastic — could land anywhere in catch ∈ [0.62, 0.78] |
| Hurts ambush | bonus is conditional on `not in_cone`, so it does not directly oppose ambush | small risk: dropping ratio to 20:12 may reduce ambush from 92.9% to ~85-88% |
| Hurts in_cone | per-step `+0.05` requires `not in_cone`, so direction is correct | none expected |
| Compute | 3-4h wall + ~30 min code/tests | 3-4h compute spent on already-met targets |
| Risk to QRDQN_5 baseline | none (we keep the snapshot via `qrdqn5-baseline` branch) | regressing primary metrics is real RL risk; we are at a verified local optimum |

**EV verdict**: negative unless catch is independently identified as a
problem. Shipping QRDQN_5 already beats Dijkstra on every metric the user
cares about. Each retrain is a lottery against a verified-good model.

### Trigger conditions (when to actually run B)

Run B only if at least one of these holds:

- Play-test in real game shows pursuer "feels too easy to escape" or fights
  feel anticlimactic because of low close-range hit rate.
- Product requirement adds a strict catch floor (e.g. >= 0.75) for game
  balance reasons.
- We are already paying for a retrain for an unrelated reason (new
  observation channel, new architecture) — fold B in for free.

### Success criteria (if run)

| metric | QRDQN_5 | target after B |
|---|---|---|
| `catch` | 0.685 | **>= 0.72** |
| `ambush_ratio` | 0.929 | **>= 0.85** (no more than -8pp) |
| `in_cone` | 0.127 | **<= 0.13** (no regression) |
| `stealth` | 0.802 | **>= 0.70** |

If catch hits >= 0.72 and the other three hold, ship as QRDQN_6. If
ambush_ratio falls below 0.85, B.1 was too aggressive — revert to 20:10 or
abandon B and ship QRDQN_5.

---

## Plan C — Stage-3 loss divergence (deferred, technical-debt)

**Status**: deferred. Apply only when bundled with another retrain (B,
observation change, etc.). Never run C alone — see cons.

### Goal

Stop `train/loss` from blowing up at curriculum stage transitions. Cosmetic
and compute-efficiency win, not a metric win.

### Observed pattern

| run | stage-3 transition step | loss before | loss peak | recovered? |
|---|---|---|---|---|
| QRDQN_3 | ~2.7M | ~5 | ~13.6 | partial |
| QRDQN_4 | 2.4M | ~6 | 39.76 @ 2.25M | no |
| QRDQN_5 | 2.4M | ~7 | 56.86 @ 2.58M | no |

Loss divergence has gotten worse with each iteration even though eval
metrics have improved. Eval reward kept rising to step 2.99M in QRDQN_5,
so policy is not broken — Q-regression is.

### Hypotheses (most likely first)

1. **Cosine LR decays to ~5% of init by stage-3 transition**, leaving no
   adaptation budget when the topology pool jumps from medium/dummy/(3,10)
   to full/default/(3,15).
2. **B3 buffer flush to 30%** at curriculum-transition leaves the buffer
   briefly noisy: stale samples from previous stage + sparse new-stage
   samples. TD targets on under-represented support spike.
3. **Reward magnitudes have grown** (+20 ambush, +-20 exit). Larger TD
   targets mean larger absolute loss; partly cosmetic.
4. **QR-DQN quantile head with 51 quantiles** is sensitive to OOD obs
   because Huber loss is summed across all quantiles.

### Steps (apply both at once)

#### C.1 — LR floor instead of decay-to-zero

`pursuer_training.ipynb` cosine LR schedule — clamp at `lr_initial * 0.1`
instead of decaying to `lr_final` (which equals `1e-5` and effectively zero
relative to `lr_initial=1e-4`). Single-line change in the schedule callable.

Verify: post-change, LR at step 2.4M >= 1e-5; loss tail-mean (last 10% of
training) < 30.

#### C.2 — Tighter gradient clipping

Pass `max_grad_norm=1.0` to `QRDQN(...)` constructor (default is `10.0`).
Symptomatic, dampens loss spikes without changing semantics.

Verify: loss peak < 30 (vs 56.86 in QRDQN_5).

### Deferred levers (apply if C.1+C.2 insufficient)

- **C.3 — Per-stage LR warm-restart**. On curriculum-advance, reset cosine
  to `0.5 * lr_init` and restart the schedule. Larger code change in
  `MetricsCallback`. SGDR-style.
- **C.4 — Soft buffer flush to 60%** instead of 30%. Reduces noise mix
  during transition. Cheap (one constant) but changes B3 semantics.

### Pros vs cons (weighted)

| factor | for | against |
|---|---|---|
| Eval-metric improvement | none expected — fixes loss not policy | cannot justify retrain on its own |
| Compute waste | saves ~20-30% of last-stage compute (loss-bound updates currently produce small gradient signal) | only matters if we expect more retrains |
| Reproducibility | cleaner TB curves; better artifact for wiki/papers | nobody outside us is reading these curves |
| Implementation | C.1 + C.2 are 2 lines combined | C.3 requires callback rewrite (~20 LOC) |
| Risk | very low — both changes are tiny and well-understood | clamp at `lr_initial * 0.1` could over-train and hurt last-stage policy slightly |

**EV verdict**: positive ONLY when bundled with another retrain. Negative
in isolation (3-4h compute for zero metric improvement).

### Trigger conditions

Run C only when bundled with one of:

- Plan B retrain (B.1 + B.2 + C.1 + C.2 in a single QRDQN_6).
- New observation channel or scalar.
- Architecture change (e.g. larger features_dim, more ResBlocks).

---

## Future-retrain decision matrix

Use this checklist before initiating any QRDQN_6+:

```
[ ] Primary metric (ambush_ratio, in_cone, stealth) is below target on shipped model
    OR
[ ] Product requirement explicitly demands a metric we don't currently meet
    OR
[ ] Observation/architecture/curriculum change makes retrain unavoidable
```

If none check, do not retrain. The shipped QRDQN_5 is a verified-good local
optimum; each retrain has nontrivial regression risk against a stochastic
3-4h compute budget. Default action is "keep snapshot".

When a retrain IS justified, fold deferred fixes (Plan B, Plan C) into the
same run rather than running them serially. One retrain = one lottery
ticket; we should not buy more than necessary.
