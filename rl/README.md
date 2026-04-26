# Pursuer RL Training Pipeline

Reinforcement learning pipeline for the Pursuer monster in Gouge.
Produces `rl/models/pursuer.onnx`, which the Go game loads via `ROGUE_PURSUER_MODEL_PATH`.

---

## Contents

```
rl/
  pursuer_env.py           Gymnasium environment — mirrors Go's BuildObservation exactly
  scripted_player.py       Scripted opponent with 6 behavioural profiles
  pursuer_training.ipynb   Main notebook: architecture, training, evaluation, ONNX export
  tools/
    gen_tiny_onnx.py       Generates a minimal ONNX fixture for Go smoke tests
  fixtures/
    parity/                Committed hand-crafted scenarios for Go <-> Python byte-exact tests
    topologies/            2000 full maps (4x4 rooms, 80x24) — committed, regen if BSP changes
    topologies_medium/     500 maps (3x3 rooms) — committed, curriculum stage 1-2
    topologies_small/      500 maps (2x2 rooms) — committed, curriculum warmup
  models/pursuer.onnx      Trained policy weights (gitignored)
  checkpoints/             SB3 eval checkpoints and evaluations.npz
  runs/                    TensorBoard event files (gitignored)
  tests/                   Python unit tests: observation invariants, env mechanics, parity
```

---

## Quick start

The notebook installs its own dependencies in the first cell. Run it from the `rl/` directory:

```bash
cd rl
jupyter notebook pursuer_training.ipynb
```

To watch training metrics live, run TensorBoard alongside the notebook:

```bash
tensorboard --logdir rl/runs
```

### Regenerating topology fixtures

Topology pools are committed. Regenerate them only if the Go BSP generator changes:

```bash
make dump-topologies            # all three pools
make dump-topologies-full       # 2000 maps, 4x4 rooms
make dump-topologies-medium     # 500 maps, 3x3 rooms
make dump-topologies-small      # 500 maps, 2x2 rooms
```

---

## Technologies

| Component        | Library / Tool              | Notes                                       |
|------------------|-----------------------------|---------------------------------------------|
| Environment      | gymnasium                   | PursuerEnv implements Env interface          |
| Parallelism      | stable-baselines3 SubprocVecEnv | 8 envs in separate processes             |
| RL algorithm     | sb3_contrib.QRDQN           | QR-DQN, off-policy, discrete actions        |
| Neural network   | PyTorch                     | Custom PursuerResCNN feature extractor       |
| ONNX export      | torch.onnx + onnxruntime    | opset 18, input "input", output "logits"    |
| Metrics          | TensorBoard                 | Logged via SB3 callbacks                    |
| Topology input   | numpy                       | Deserialise Go-generated JSON maps          |

---

## Why QR-DQN instead of PPO

The initial approach used PPO (Proximal Policy Optimization), an on-policy actor-critic algorithm.
It was replaced by QR-DQN for four concrete reasons.

**Off-policy replay vs. sparse terminal rewards.**
The catch event (+15) and escape event (-20) are rare terminal signals spread across episodes of up to 70 steps. PPO discards all collected experience after each update, so each terminal event is seen exactly once. QR-DQN stores transitions in a 300K-step replay buffer and re-samples the same rare events hundreds of times, dramatically improving signal-to-noise for the sparse reward.

**Distributional RL for high-variance outcomes.**
The outcome of a pursuit episode is bimodal: either the pursuer catches the player or the player
escapes. QR-DQN estimates the full return distribution via 51 quantiles rather than a single
expected value. This stabilises training because the network does not average two completely
different futures into one number.

**Discrete actions are a natural fit for Q-learning.**
The pursuer has exactly 5 actions. Q-learning computes a value for each action explicitly and
picks the argmax — exact and cheap. PPO with a discrete policy requires extra care to prevent
entropy collapse and premature convergence to a deterministic policy.

**Better sample efficiency at low FPS.**
The environment is expensive: each step runs 5 BFS/Dijkstra maps in Python. At 8 parallel envs
throughput is roughly 1000 env steps/sec. Off-policy replay reuses every sample multiple times,
making effective learning steps per wall-clock second significantly higher than on-policy collection.

---

## Environment design

`PursuerEnv` is a single-agent Gymnasium env. One pursuer, one scripted player, one topology per episode. Items, multi-level runs, and door mechanics are excluded — the env is a minimal slice that forces stealth and ambush reasoning without the complexity of full game state.

### Episode flow

```
reset():
  load random topology JSON from pool
  place player at random walkable cell
  place pursuer within spawn_radius (3..8 Chebyshev cells from player)
  pre-compute exit_scent map (BFS from exit, cached for full episode)
  apply curriculum settings (player profile, distractor, topology pool)

step(action):
  1. pursuer moves (ACTION_UP / RIGHT / DOWN / LEFT / WAIT)
  2. pursuer attacks if adjacent to player or WAIT while adjacent
  3. scripted player moves according to its profile
  4. player counter-attacks if pursuer is adjacent
  5. recompute chase_scent (BFS from player — updates every step)
  6. advance distractor (random walk)
  7. compute reward

terminated: player HP = 0  OR  pursuer HP = 0  OR  player reaches exit
truncated:  step_count >= max_episode_steps (randomised per episode: 40..70)
```

### Actions

| Index | Name         |
|-------|--------------|
| 0     | ACTION_UP    |
| 1     | ACTION_RIGHT |
| 2     | ACTION_DOWN  |
| 3     | ACTION_LEFT  |
| 4     | ACTION_WAIT  |

### Reward structure

```
Terminal rewards:
  +15.0   player killed (pursuer wins)
  -20.0   player reached exit (player wins)
   -2.0   pursuer dies from player counter-attack

Ambush bonus/penalty (fires when pursuer enters player flashlight cone from outside):
  +1.5    Chebyshev distance to player <= 2  (sudden close-range ambush)
  -1.0    Chebyshev distance >= 5            (detected from far away)

Dense shaping (every step):
  +-0.02 * delta_chebyshev   reward approach, penalise retreat
  -0.1 * (1 - player_to_exit / max_dist)   pressure to intercept before exit
  -0.5    crowding: distractor already near player AND pursuer moves in
  -0.05*k anti-camp: k turns within 3 cells of exit AND far from player
  -0.02   time penalty per step

WAIT penalties:
  -0.1    if standing inside player flashlight cone
  -0.02   if TurnsSinceLOS > 10 (aimless waiting)
```

### Scripted player profiles

Six profiles control how the opponent behaves. The player uses a goal-directed base policy
(BFS descent toward exit) modulated by these probability parameters:

| Profile        | flee_prob | wait_prob | goal_prob | panic_boost | Role in curriculum     |
|----------------|-----------|-----------|-----------|-------------|------------------------|
| stone          | 0.0       | 1.0       | 0.0       | 0.0         | stationary, stage 0    |
| training_dummy | 0.0       | 0.85      | 0.15      | 0.0         | barely moves, stage 0  |
| default        | 0.5       | 0.1       | 0.75      | 1.0         | balanced, stages 3-4   |
| aggressive     | 0.3       | 0.02      | 0.55      | 0.8         | ignores exit, hunts    |
| cautious       | 0.7       | 0.3       | 0.85      | 1.0         | camps near exit        |
| timid          | 0.95      | 0.1       | 0.95      | 1.0         | always flees           |

`panic_boost` multiplies `goal_prob` when the pursuer is within line of sight, simulating
a sprint for the exit when the player notices danger.

---

## Observation tensor

1714 floats in a flat vector. Layout: CHW row-major grid block followed by scalar block.

```
obs[0 .. 1693]     grid block: 14 channels x 11 x 11 = 1694 floats
obs[1694 .. 1713]  scalar block: 20 floats
```

The 11x11 crop is pursuer-centered (pursuer always at cell 5,5).
Index within a channel: c * 121 + y * 11 + x.

### Channels

| Index | Name                  | Encoding                                                        |
|-------|-----------------------|-----------------------------------------------------------------|
| 0     | ChWalkable            | 1.0 if tile is walkable                                         |
| 1     | ChClosedDoor          | 1.0 if tile is a closed door                                    |
| 2     | ChOtherMonster        | 1.0 if another monster occupies the cell                        |
| 3     | ChSelfTrail           | Pursuer's recent positions, linear decay over trail length      |
| 4     | ChPlayerVisNow        | 1.0 if player is visible with direct LOS                        |
| 5     | ChPlayerMemory        | Spike at last-known player position, decays with TurnsSinceLOS  |
| 6     | ChPlayerCone          | 1.0 if cell is inside player flashlight cone (FOV 45 deg, r=3)  |
| 7     | ChScent               | Dijkstra BFS distance from player, normalised by 30             |
| 8     | ChAgeLastSeen         | Gaussian decay (sigma=2.5) around last-seen position            |
| 9     | ChCovertPath          | Cone-weighted Dijkstra (penalty=10 inside cone), stealth grad   |
| 10    | ChPlayerTrail         | Recent player positions, exponential decay (alpha=0.9)          |
| 11    | ChInterceptPath       | BFS from optimal intercept cell                                 |
| 12    | ChVisitedCorridorPath | Dijkstra biased toward player-visited corridors                 |
| 13    | ChFuturePlayerPos     | Predicted player path (greedy exit descent), decay 0.8/step    |

### Scalars

| Index | Name                                               |
|-------|----------------------------------------------------|
| 0     | HP ratio                                           |
| 1     | Stamina ratio                                      |
| 2     | TurnsSinceLOS / horizon                            |
| 3     | Distance to last-seen (norm diagonal)              |
| 4     | Last-seen dx / map width                           |
| 5     | Last-seen dy / map height                          |
| 6     | Nearest pursuer dx / map width                     |
| 7     | Nearest pursuer dy / map height                    |
| 8     | In player cone flag                                |
| 9     | Player dx / map width                              |
| 10    | Player dy / map height                             |
| 11    | Cone dot product (pursuer dir . player-to-pursuer) |
| 12    | Player-to-exit distance (norm)                     |
| 13    | Pursuer-to-player distance (norm)                  |
| 14    | Intercept advantage (signed, norm)                 |
| 15    | Player in corridor flag                            |
| 16    | Map position X (norm)                              |
| 17    | Map position Y (norm)                              |
| 18    | Polar distance to player (norm)                    |
| 19    | Polar angle to player / pi                         |

---

## Neural architecture — PursuerResCNN

A split-stream feature extractor used as the QR-DQN policy network.

```
Input: 1714-float flat vector

Grid stream:
  reshape -> (B, 14, 11, 11)
  CoordConv: append normalised x,y coordinate grids -> (B, 16, 11, 11)
  ResBlock(in=16, out=32, dilation=1)
  ResBlock(in=32, out=64, stride=2, dilation=2)   11x11 -> 6x6, receptive field 5x5
  SEBlock(channels=64, reduction=8)
  AdaptiveAvgPool(3x3)
  Flatten -> 576
  Linear(576 -> 192) + ReLU

Scalar stream:
  Linear(20 -> 32) + ReLU

Fusion:
  Concat(192 + 32 = 224) -> Linear(224 -> 256) + ReLU

QR-DQN head: quantile_net -> mean over 51 quantiles -> argmax over 5 actions
```

**CoordConv** appends two channels containing normalised (x, y) coordinate grids to the CNN input. Because the pursuer is always at center (5,5) of the crop, CoordConv gives the network an explicit sense of offset from self — crucial for reading directional channels like ChPlayerMemory and ChScent.

**Squeeze-and-Excitation (SE)** is channel attention applied after the second ResBlock.
The 14 input channels carry very different tactical signals (covert-path for stealth routing,
cone-contact for detection timing). SE learns to re-weight channels per input rather than blending them uniformly.

**Dilated convolutions** with dilation=2 in ResBlock 2 triple the effective receptive field without
adding parameters or reducing spatial resolution before pooling. A 3x3 kernel at dilation=2
covers a 5x5 area, capturing corridor-level context across the 11x11 crop.

**Residual connections** in each ResBlock stabilise gradients. With the shallow depth (2 blocks)
residuals primarily guard against degradation during the channel-reweighting task learned by SE.

Total parameters: roughly 30K-40K. Intentionally lightweight for fast CPU inference at runtime.

---

## Training configuration

```python
CONFIG = {
    'total_timesteps':         3_000_000,
    'n_envs':                  8,            # SubprocVecEnv — separate OS processes
    'max_episode_steps':       50,           # base value
    'max_episode_steps_range': (40, 70),     # randomised per episode reset
    'features_dim':            256,          # fusion layer output -> QR-DQN head
    'n_quantiles':             51,           # QR-DQN distributional head
    'buffer_size':             300_000,      # replay buffer capacity (transitions)
    'learning_starts':         50_000,       # random exploration before first update
    'batch_size':              512,
    'gamma':                   0.99,
    'lr_initial':              1e-4,         # cosine LR schedule start
    'lr_final':                1e-5,         # cosine LR schedule end
    'exploration_fraction':    0.50,         # epsilon greedy decay over first 50% of steps
    'exploration_final_eps':   0.02,         # minimum epsilon
    'train_freq':              4,            # gradient update every 4 env steps
    'target_update_interval':  2_000,        # hard target network copy interval (steps)
}
```

**Cosine LR schedule** decays the learning rate from `lr_initial` to `lr_final` following a
half-cosine curve over the full training run. Implemented as a custom `MetricsCallback` that
updates `model.policy.optimizer.param_groups` each step. Compared to step decay, this gives smoother convergence in the final third of training where the policy is near-optimal and a
high learning rate would destabilise the Q-value estimates.

**SubprocVecEnv** runs 8 environments in separate OS processes to bypass the Python GIL.
Each process has its own topology pool handle and PRNG state so episode resets do not collide.

**Domain randomisation**: `max_episode_steps` is sampled uniformly from (40, 70) at each reset.
This prevents the policy from learning a fixed horizon and forces robustness to variable-length chases.

---

## Curriculum (5 stages)

The curriculum progressively increases difficulty by switching the topology pool and player profile. Both the training VecEnv and the eval VecEnv are updated simultaneously via `set_difficulty()`. Each stage is saved as `checkpoints/stage{n}_final.zip` on promotion.

| Stage | Player profile        | Topology pool     | Promotion condition                      |
|-------|-----------------------|-------------------|------------------------------------------|
| 0     | stone                 | topologies_small  | eval reward > 10.0 AND catch_rate > 85%  |
| 1     | stone                 | topologies_medium | eval reward > 10.0 AND catch_rate > 85%  |
| 2     | training_dummy        | topologies_medium | eval reward > 8.0  AND catch_rate > 50%  |
| 3     | default               | topologies_full   | eval reward > 9.0  AND catch_rate > 65%  |
| 4     | default + distractors | topologies_full   | (final stage, no promotion)              |

Stage 4 adds a random-walking distractor monster. When both the distractor and the pursuer are near the player simultaneously, a crowding penalty (-0.5) fires in the reward, discouraging
passive shadowing and encouraging the pursuer to act decisively.

---

## Topology generation

Dungeon maps for training are generated by the Go-side `cmd/dump-topology` binary. Using real BSP
maps ensures the training distribution matches the production game exactly — walls, corridors,
room shapes, and extra connection loops are identical to what the Pursuer faces at runtime.

Topology pools are gitignored. Regenerate before training on a fresh checkout.

### Commands

```bash
# Full pool — 16 rooms (4x4 grid) — production maps
go run ./cmd/dump-topology \
  -n 2000 -seed 42 -rooms-h 4 -rooms-v 4 -extra-connections 2 \
  -out rl/fixtures/topologies

# Medium pool — 9 rooms (3x3 grid) — curriculum stages 1-2
go run ./cmd/dump-topology \
  -n 500 -seed 100 -rooms-h 3 -rooms-v 3 -extra-connections 1 \
  -out rl/fixtures/topologies_medium

# Small pool — 4 rooms (2x2 grid) — curriculum stage 0 warmup
go run ./cmd/dump-topology \
  -n 500 -seed 200 -rooms-h 2 -rooms-v 2 -extra-connections 0 \
  -out rl/fixtures/topologies_small

# Makefile shortcut for the full pool only:
make dump-topologies
```

### CLI flags

| Flag               | Default       | Meaning                                         |
|--------------------|---------------|-------------------------------------------------|
| -n                 | 1000          | Number of maps to generate                      |
| -seed              | UnixNano      | Base RNG seed; map i uses seed+i                |
| -out               | rl/topologies | Output directory                                |
| -width             | 80            | Map width in tiles                              |
| -height            | 24            | Map height in tiles                             |
| -rooms-h           | 4             | Horizontal room count                           |
| -rooms-v           | 4             | Vertical room count                             |
| -extra-connections | 2             | Extra corridors beyond the minimum spanning tree|

`-extra-connections` adds random inter-room corridors on top of the spanning tree, creating loops
and reducing dead-end density. Higher values make the map more open with more pursuer approach routes.

---

## Go <-> Python parity

`BuildObservation` (Go, `internal/domain/service/rl_observation.go`) and `PursuerEnv._observe`
(Python) must produce byte-identical float32 tensors. There is no auto-generated contract —
parity is maintained manually.

What is mirrored between Go and Python:
- Channel layout and index constants (Ch0..Ch13)
- Crop size (11x11), channel count (14), scalar count (20), total size (1714)
- All BFS/Dijkstra algorithms: chase scent, covert path, intercept path, visited corridor path
- LOS Bresenham line
- Raycaster DDA (FOV 45 deg, range 3 cells)
- Memory decay formulas: Gaussian (sigma=2.5), exponential (alpha=0.9), linear trail decay
- All scalar normalisation factors

### Parity tests

`rl/tests/test_parity.py` performs byte-exact comparison on 5 committed hand-crafted scenarios:

```bash
make test-rl
# or:
python -m pytest rl/tests/test_parity.py -v
```

The test calls `go run ./cmd/dump-observation` on each fixture, then runs `PursuerEnv._observe`
on the same state and asserts `max(abs(go - py)) < 1e-6`.

After any observation change, regenerate the fixtures:

```bash
go run ./cmd/dump-observation -out rl/fixtures/parity/
make test-rl
```

---

## ONNX export

The trained PyTorch model is exported to ONNX for Go-side inference via `yalue/onnxruntime_go`.

```python
torch.onnx.export(
    model,
    dummy_input,
    "rl/models/pursuer.onnx",
    input_names=["input"],
    output_names=["logits"],
    opset_version=18,
)
```

The Go runtime expects:
- Input tensor:  name "input",  shape (1, 1714), dtype float32
- Output tensor: name "logits", shape (1, 5),    dtype float32

`ONNXPolicy.Predict` takes the argmax over the 5 logits to select an action.
Inference is serialised with a sync.Mutex because the ONNX Runtime session is not thread-safe.

The final notebook cell validates the export by running 1000 random observations through both
the PyTorch policy and the ONNX graph and asserting max(abs(pytorch - onnx)) < 1e-5.

---

## Testing

```bash
make test-rl                                        # all Python tests
python -m pytest rl/tests/test_parity.py -v        # Go <-> Python byte-exact
python -m pytest rl/tests/test_observation.py -v   # observation shape and value invariants
python -m pytest rl/tests/test_env.py -v           # environment mechanics
```

The Go smoke test (`tests/application_test/pursuer_onnx_smoke_test.go`) wires `ONNXPolicy`
through `MonsterControllerService` and verifies the full stack produces valid action intents.
It uses a tiny ONNX fixture generated by `rl/tools/gen_tiny_onnx.py`.
