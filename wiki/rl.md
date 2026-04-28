# RL subsystem (`rl/`)

Python reinforcement learning pipeline for the Pursuer monster.
Produces `rl/models/pursuer.onnx`, loaded by Go via `ROGUE_PURSUER_MODEL_PATH`.

---

## File map

| File / directory | Purpose |
|-----------------|---------|
| `pursuer_env.py` | Gymnasium env — 1:1 mirror of Go's `BuildObservation` |
| `scripted_player.py` | Scripted opponent (goal-directed with 6 behavioural profiles) |
| `pursuer_training.ipynb` | Main notebook: architecture, training, eval, ONNX export |
| `tools/gen_tiny_onnx.py` | Generates minimal ONNX fixture for Go smoke tests |
| `fixtures/parity/*.json` | Committed hand-crafted scenarios for Go↔Python byte-exact tests |
| `fixtures/topologies/` | 2000 full maps (4×4 rooms, 80×24) — committed, regen if BSP changes |
| `fixtures/topologies_small/` | 500 maps (2×2 rooms) — committed, curriculum warmup |
| `fixtures/topologies_medium/` | 500 maps (3×3 rooms) — committed, curriculum bridge |
| `models/pursuer.onnx` | Trained policy (gitignored; `.data` tracked) |
| `checkpoints/` | SB3 eval checkpoints + `evaluations.npz` |
| `runs/QRDQN_*/` | TensorBoard event files — 3 training runs |
| `tests/` | Python unit tests (observation invariants, env mechanics, parity) |
| `__init__.py` | Package marker |

---

## Technologies

| Component | Library | Version |
|-----------|---------|---------|
| Gymnasium env | `gymnasium` | 1.2.3 |
| RL algorithm | `stable-baselines3` + `sb3_contrib.QRDQN` | SB3 2.8.0 |
| Neural network | `torch` (PyTorch) | 2.11.0 |
| ONNX export | `torch.onnx.export` + `onnxruntime` | opset 17→18 |
| Metrics | `tensorboard` | 2.20.0 |
| Topology deserialisation | `numpy` | 2.4.4 |

**Algorithm: QR-DQN** (Quantile Regression DQN, off-policy, discrete actions).
Chosen over PPO because it is naturally suited to the sparse-reward catch task —
quantile regression provides full distributional estimates, smoothing out the
binary catch/escape terminal signals.

---

## Environment (`PursuerEnv`)

One pursuer, one scripted player, one topology per episode.
No items, no multi-level runs, no door-opening — minimal slice that still
forces stealth and ambush reasoning.

### Episode mechanics

```
reset():
  - Load random topology JSON from pool
  - Place player randomly; place pursuer within spawn_radius (3..8 Chebyshev cells)
  - Pre-compute exit_scent (BFS from exit, cached for episode)
  - Apply curriculum knobs (profile, distractor, topology pool)

step(action):
  1. Pursuer moves (ACTION_UP/RIGHT/DOWN/LEFT/WAIT)
  2. Pursuer attacks if bump into player or WAIT while adjacent
  3. Scripted player moves + counter-attacks
  4. Recompute chase_scent (BFS from player, every step)
  5. Advance distractor (random walk)
  6. Compute reward
  → terminated if player HP=0, pursuer HP=0, or player reaches exit
  → truncated if step_count >= max_episode_steps
```

### Reward structure

Current shape (post-Plan-A and pre-QRDQN_6 implementation; [[plan]] tracks
the per-iteration changes):

```
Terminal (QRDQN_5: 20/8 split; QRDQN_6: 20/12 split — see [[plan]] B.1):
  +20.0   player killed AND pursuer was NOT visible last step (ambush)
  + 8.0   player killed AND pursuer WAS     visible last step (visible) — QRDQN_5
  +12.0   ditto — QRDQN_6 (loaded from current pursuer_env.py)
  -20.0   player reached exit
   -2.0   pursuer dies

Ambush effect (fires when pursuer enters cone from outside):
  +3.0    if Chebyshev dist to player <= 2  (sudden close-range)
  -1.0    if Chebyshev dist >= 5            (premature far detection)

Dense shaping:
  +-0.02 * clip(delta_covert_dist, -2, 2)   covert-Dijkstra approach signal — QRDQN_6 (was Chebyshev pre-Plan-B)
  +0.05   per step if NOT in cone AND chebyshev<=3 — QRDQN_6 stealth-approach bonus
  -0.05   per step if pursuer is in player's cone (any action) — Plan A
  -0.1 * (1 - player_to_exit/max_dist)   exit-blocking pressure
  -0.5    crowd: distractor already near player AND pursuer moves in
  -0.05*k anti-camp: k consecutive turns within 3 cells of exit AND far from player
  -0.02   time penalty per step

WAIT penalties:
  -0.1    if in player cone (freeze in spotlight; stacks on the -0.05 cone-presence)
  -0.02   if TurnsSinceLOS > 10 (wandering without purpose)
```

Source of truth: `rl/pursuer_env.py:_compute_reward`. The doc-string in
that function pins the exact constants for the loaded weights.

### Scripted player profiles

| Profile | flee_prob | wait_prob | goal_prob | panic_boost | Notes |
|---------|-----------|-----------|-----------|-------------|-------|
| stone | 0.0 | 1.0 | 0.0 | 0.0 | stationary — curriculum warmup |
| training_dummy | 0.0 | 0.85 | 0.15 | 0.0 | barely moves |
| default | 0.5 | 0.1 | 0.75 | 1.0 | balanced |
| aggressive | 0.3 | 0.02 | 0.55 | 0.8 | ignores exit, hunts |
| cautious | 0.7 | 0.3 | 0.85 | 1.0 | camps near exit |
| timid | 0.95 | 0.1 | 0.95 | 1.0 | always flees |

The player uses a goal-directed policy: most turns it descends the BFS
exit-scent gradient. `panic_boost` multiplies `goal_prob` when the pursuer
is visible, simulating a realistic sprint for the exit.

---

## Topology generation

Topologies are generated by the Go side (`cmd/dump-topology`) so the BSP layout
during training matches the real game exactly. The Python env loads them as JSON.

### All three pools

```bash
# Full (4x4 rooms, 80x24 map) — production
go run ./cmd/dump-topology -n 2000 -seed 42 \
  -rooms-h 4 -rooms-v 4 -extra-connections 2 \
  -out rl/fixtures/topologies

# Medium (3x3 rooms, 80x24 map) — curriculum stage 1-2
go run ./cmd/dump-topology -n 500 -seed 100 \
  -rooms-h 3 -rooms-v 3 -extra-connections 1 \
  -out rl/fixtures/topologies_medium

# Small (2x2 rooms, 80x24 map) — curriculum warmup
go run ./cmd/dump-topology -n 500 -seed 200 \
  -rooms-h 2 -rooms-v 2 -extra-connections 0 \
  -out rl/fixtures/topologies_small
```

Map dimensions (all three pools share 80×24):
- `topologies_small`:  4 rooms (2×2 grid)
- `topologies_medium`: 9 rooms (3×3 grid)
- `topologies_full`:  16 rooms (4×4 grid)

All three pools are committed (3000 maps total). Regenerate only if the Go BSP
generator changes; parity fixtures live separately under `fixtures/parity/` with
their own pinned topology under `fixtures/parity/topologies/`.

### CLI flags for `dump-topology`

| Flag | Default | Meaning |
|------|---------|---------|
| `-n` | 1000 | Number of maps |
| `-seed` | UnixNano | Base RNG seed; map `i` uses `seed+i` |
| `-out` | `rl/topologies` | Output directory |
| `-width` | 80 | Map width in tiles |
| `-height` | 24 | Map height in tiles |
| `-rooms-h` | 4 | Horizontal room count |
| `-rooms-v` | 4 | Vertical room count |
| `-extra-connections` | 2 | Extra corridors beyond spanning tree |

---

## Neural architecture (`PursuerResCNN`)

Split-stream feature extractor for QR-DQN.

```
Input: 1714-float flat vector

Grid stream (14 ch × 11×11):
  reshape → (B, 14, 11, 11)
  append CoordConv channels (+2) → (B, 16, 11, 11)
  ResBlock(16→32, dilation=1)
  ResBlock(32→64, stride=2, dilation=2)   [11×11 → 6×6, receptive field 5×5]
  SEBlock(64, reduction=8)
  AdaptiveAvgPool(3×3)
  Flatten → 64×9 = 576
  Linear(576 → 192) + ReLU

Scalar stream (20 floats):
  Linear(20 → 32) + ReLU

Fusion:
  Concat(192+32=224) → Linear(224 → 256) + ReLU

QR-DQN head: quantile_net → mean over 51 quantiles → argmax(5 actions)
```

**CoordConv**: appends normalised (x, y) coordinate grids to the input
tensor, giving the network an explicit sense of relative position within
the 11×11 crop — the pursuer always sits at the center.

**SE (Squeeze-Excitation)**: channel attention after the second ResBlock.
With 14 channels carrying very different tactical signal (covert-path early,
cone-contact during hunt), SE lets the network re-weight channels per input.

**Dilated convolutions**: dilation=2 in ResBlock 2 triples the effective
receptive field without adding parameters or reducing spatial resolution
before the pooling.

**Total parameters**: ~30K–40K (lightweight; fast inference on CPU).

---

## Training configuration (current — QRDQN_6 prep)

Full `CONFIG` dict (top of `pursuer_training.ipynb`):

```python
CONFIG = {
    # Topology pools (4 pools - the active one for stage 0 is topologies_dir)
    'topologies_small':      'fixtures/topologies_small',   # 2x2 rooms
    'topologies_medium':     'fixtures/topologies_medium',  # 3x3
    'topologies_full':       'fixtures/topologies',         # 4x4 - production
    'topologies_dir':        'fixtures/topologies_small',   # initial pool

    # Episode
    'total_timesteps':       3_000_000,
    'n_envs':                8,                  # SubprocVecEnv
    'max_episode_steps':     50,
    'max_episode_steps_range': (40, 70),         # domain randomisation
    'seed':                  42,

    # Output paths
    'model_out':             'models/pursuer.onnx',
    'checkpoint_dir':        'checkpoints',
    'tensorboard_log':       'runs',

    # Net + algorithm
    'features_dim':          256,
    'n_quantiles':           51,
    'buffer_size':           300_000,
    'learning_starts':       50_000,
    'batch_size':            512,
    'gamma':                 0.99,
    'lr_initial':            1e-4,               # cosine decay -> lr_final
    'lr_final':              3e-5,                # QRDQN_6 (Plan C.1): 1e-5 -> 3e-5, ~3x late-stage budget
    'exploration_fraction':  0.50,
    'exploration_final_eps': 0.02,
    'train_freq':            4,
    'target_update_interval': 2_000,

    # Eval
    'eval_freq':             10_000,
    'eval_episodes':         20,
    'metrics_log_freq':      5_000,

    # Curriculum thresholds
    'curriculum_check_freq':     10_000,
    'curriculum_stone_rew':      10.0,
    'curriculum_stone_catch':     0.85,
    'curriculum_dummy_rew':       8.0,
    'curriculum_dummy_catch':     0.50,
    'curriculum_default_rew':     9.0,
    'curriculum_default_catch':   0.65,

    # Early stopping (currently disabled in callback wiring)
    'es_patience':  5,
    'es_min_delta': 0.5,
}
```

**Cosine LR schedule**: half-cosine decay from `lr_initial` to `lr_final`
over the full 3M-step run. Smoother tail convergence than step decay.
The QRDQN_6 floor (3e-5 = 30% of `lr_initial`) is intentionally higher
than QRDQN_5's (1e-5 = 10%) — see [[plan]] Plan QRDQN_6 C.1.

**Gradient clipping**: `max_grad_norm=1.0` on the QRDQN constructor (was
SB3 default 10.0). Symptomatic damping for stage-3 loss spikes — see
[[plan]] Plan QRDQN_6 C.2.

### Notebook cell order (`pursuer_training.ipynb`)

| # | Type | Purpose |
|---|------|---------|
| 1 | md | "Pursuer - QR-DQN training pipeline" header |
| 2 | code | `%pip install` deps |
| 3 | md | "Hyperparameters" |
| 4 | code | `CONFIG` dict (above) |
| 5 | code | `%load_ext tensorboard` + `%tensorboard --logdir runs` (inline UI) |
| 6 | md | "Environment" |
| 7 | code | imports + 30-step env sanity rollout |
| 8 | md | "Architecture" |
| 9 | code | `PursuerResCNN` definition (CoordConv + ResBlocks + SE + dilated) |
| 10 | md | "Training" |
| 11 | code | `cosine_lr_schedule`, `MetricsCallback`, `CurriculumCallback`, `make_env` |
| 12 | code | training loop - `SubprocVecEnv` + `QRDQN` + `model.learn` |
| 13 | md | "Learning curves" |
| 14 | code | matplotlib plots from metrics history (reward, len, catch, epsilon) |
| 15 | md | "Q-value heatmap" |
| 16 | code | 3x3 grid of Q-heatmaps via `render_q_heatmap` (uses `buffer_rgba`) |
| 17 | md | "Evaluation vs baseline" |
| 18 | code | `BaselinePolicy` (Dijkstra) + rollout vs QR-DQN on `topologies_full` |
| 19 | code | RNG determinism sanity test (baseline byte-equal across seeds) |
| 20 | md | "ONNX export + parity check" |
| 21 | code | `_QRDQNActor` wrapper + `torch.onnx.export(opset_version=18)` |
| 22 | code | parity loop - 1000 random samples; **assertion fails at 1.14e-4** ([[bugs]] B1) |
| 23 | code | per-profile rollout (stone / dummy / default / aggressive / cautious / timid) |

---

## Curriculum (5 stages)

| Stage | Profile | Topology pool | Spawn radius | Promote condition |
|-------|---------|---------------|--------------|-------------------|
| 0 | stone | topologies_small | (3, 8) | reward > 9.0 AND catch > 70% |
| 1 | stone | topologies_medium | (3, 8) | same |
| 2 | training_dummy | topologies_medium | (3, 10) | reward > 8.0 AND catch > 50% |
| 3 | default | topologies_full | (3, 15) | reward > 9.0 AND catch > 65% |
| 4 | default + distractors + randomised | topologies_full | (3, 15) | (final, no promotion) |

Each stage runs for at least `curriculum_min_steps_per_stage = 300_000`
steps even if the threshold is met sooner (consolidate the skill), and at
most `curriculum_max_steps_per_stage = 800_000` steps even if the threshold
is never met (force-promote to give later stages budget).

Promotion side-effects:
1. The current model is saved as `stage{n}_final.zip`.
2. Training and eval VecEnvs are advanced via `env_method('set_difficulty', ...)`.
3. **Replay buffer flush** (B3 fix): the oldest
   `curriculum_buffer_flush_frac = 0.7` of the replay buffer is dropped, the
   most-recent 30% physically rotated to the front. Prevents TD-target
   divergence from stale (small-map) transitions.

---

## Training runs — short index

The notebook (`pursuer_training.ipynb`) "Run history" section is the
canonical source of run-by-run write-ups, including pre-Plan-A (QRDQN_1/2
/3) era observation/architecture changes. CSVs in `rl/runs_summary/` feed
the in-notebook comparison plot. [[logs]] holds the chronological decision
log. This page only carries the one-line index.

| Run | Steps | Goal | Eval (eval_pool/v1, where applicable) | Status |
|---|---|---|---|---|
| QRDQN_1 | 2M | Pipeline sanity, 2-channel obs (`ChScent` + `ChWalkable`) | (no eval pool) | superseded |
| QRDQN_2 | 3M | Observation enrichment to 11 channels + SE channel attention | (no eval pool) | superseded; `curriculum_stage` tag missing in TB events was a callback bug, training itself ran with curriculum |
| QRDQN_3 | 3M | Spatial promotion (intercept/visited/future as channels), cosine LR, dilated conv | (no eval pool) | superseded |
| QRDQN_4 | 3M | B1..B12 bug-fix sweep | catch 0.755, ambush 0.607, in_cone 0.156 — strictly worse than Dijkstra on catch, equal on stealth | superseded |
| QRDQN_5 | 3M | Plan A: stealth-shifted reward (+20/+8 split, -0.05 cone-presence) | catch 0.685, ambush **0.929**, in_cone 0.127, stealth **0.802** | **shipped** (anchor branch `qrdqn5-baseline`) |
| QRDQN_6 | 3M | Plan B+C bundled (recover catch + push in_cone <0.10 + fix loss divergence) | TBD | code in place, training pending; revert criteria frozen in [[plan]] |

**Open items inherited from QRDQN_5**:
- Stage-3 loss divergence (peak 56.86) — addressed in QRDQN_6 by C.1+C.2.
- `in_cone` 0.127 vs target 0.10 — addressed in QRDQN_6 by B.3 covert-path
  approach shaping.
- `catch` 0.685 vs gate 0.70 — addressed in QRDQN_6 by B.1 (20:12 split) +
  B.2 (close-and-unseen bonus).

---

## Reward function — source of truth

The exact reward constants live in `rl/pursuer_env.py:_compute_reward`
docstring; that doc-string is updated alongside the code on every Plan
change. Wiki-side summary is in the *Reward structure* block earlier
in this page (post-Plan-A, pre-QRDQN_6 dual annotation). [[plan]] tracks
the per-iteration deltas. Don't expect this page to know what's loaded
in `pursuer.onnx` — that depends on which run produced it.

---

## Known bugs and open issues

Tracked in [[bugs]]. Highlights affecting the RL pipeline:

- **B1** - ONNX parity assertion fails at threshold `1e-4` (actual `1.14e-4`).
- **B2** - `test_observation.py` docstring says "scalar slice is 12 floats" - now 20.
- **B3** - `train/loss` diverges (replay-buffer staleness across curriculum stages).
- **B4** - `_observe()` runs 5 BFS/Dijkstra per step, FPS dropped 1058 -> 131.
- **B5** - Curriculum stage 4 fires at step 2.88M of 3M (no time to learn).
- **B6** - `observation_space` declared `low=-1` for grid floats actually in [0, 1].
- **B7** - QR-DQN strictly worse than Dijkstra (-10pp catch, -2.4 return, +10pp ambush).
- **B8** - `spawn_radius=(3,8)` matches `rlTriggerRadius=8`, but post-frustration
  handoffs from Director can be out-of-distribution.
- **B11** - Eval is byte-deterministic per-config but cross-run comparisons
  are unreliable (eval pool / max_steps / env code change between runs).
  Recommendation: frozen `fixtures/eval_pool/`, n_episodes >= 200, eval
  knobs decoupled from CONFIG.

Resolved (no longer in code): `gen_tiny_onnx.py OBS_SIZE`, eval-pool mismatch,
`tostring_rgb` matplotlib API, broken `from rl.*` notebook imports.
