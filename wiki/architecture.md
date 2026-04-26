# Architecture

## Overview

Gouge is a turn-based multiplayer roguelike built in Go with a Python RL training pipeline.
The codebase follows **Clean/Hexagonal Architecture** with a strict domain-first approach.

```
cmd/            — entry points (no business logic)
internal/
  domain/       — pure business logic (no I/O, no framework)
  application/  — use cases, game loop, view mapping
  bootstrap/    — dependency injection (Google Wire)
  presentation/ — TUI (BubbleTea), in-process network layer
  infrastructure/ — storage adapters (JSON files, in-memory)
  dto/          — cross-layer data transfer objects
pkg/            — reusable, project-agnostic libraries
rl/             — Python QR-DQN training pipeline
tests/          — black-box tests organised by layer
```

---

## Dependency flow

```
presentation ──► application ──► domain ◄── infrastructure
       │              │               ▲
       └──────────────┴───► bootstrap ┘
```

- `domain` imports only `pkg/geometry`, `pkg/utils` and stdlib — zero framework deps.
- `application` imports `domain` + `dto`; it defines the `port` interfaces that
  `infrastructure` satisfies.
- `presentation` imports `application` (game loop, use cases) and `dto`.
- `bootstrap` wires everything together with Google Wire.

---

## Game loop

```
Player input                Monster tick (1s)
     │                             │
     ▼                             ▼
SubmitIntent ──► PendingIntents ◄── GameLoop.Run
                      │
                ResolveTurn.Exec
                      │
        ┌─────────────┼─────────────┐
        ▼             ▼             ▼
   Movement       Combat      ItemUsage/Interact
        │             │
        ▼             ▼
   MoveResolver  ImpactResolver
        │
   FoW update (Raycaster)
   ScentMaps rebuild
        │
   broadcastState → per-player GameView
```

One "turn" is driven by `ResolveTurn.Exec`:
1. Collect all pending player intents + AI decisions.
2. Resolve each action (movement, combat, item, interaction) in priority order.
3. Apply effects/reactions via `ImpactResolver`.
4. Update each player's fog-of-war via `Raycaster`.
5. Rebuild `ScentMaps` (wander + chase + exit lazy) in `SessionContext`.
6. Persist playthrough to `PlaythroughRepository`.
7. Broadcast per-player snapshots to TUI subscribers.

---

## Map representation

Three stacked integer grids, all indexed `[y][x]`:

| Grid | Type | Content |
|------|------|---------|
| `tileGrid` | `[][]Cell` | Tile type + room ID |
| `itemGrid` | `[][]int64` | Item ID (0 = empty) |
| `actorGrid` | `[][]int64` | Actor ID (0 = empty) |

Rooms cache two indexed lists of empty points (`emptyItemPoints`, `emptyActorPoints`)
that are updated incrementally on every spawn/pickup/kill — no full scan needed.

---

## BSP dungeon generation

1. **Sectorize** — divide map into a grid of variable-size sectors.
2. **Create rooms** — place one room randomly inside each sector.
3. **Entrance / exit** — assign opposite-corner rooms.
4. **Connect rooms** — spanning tree of L-shaped corridors.
5. **Extra connections** — optional additional corridors for loops.
6. **Spawn objects** — `DungeonGenerator` fills rooms with monsters and items
   according to `DungParams` (weights, min/max counts).
7. **Lock doors** — `DoorLocker` randomly locks doors with colored keys.

Difficulty is interpolated along `DifficultyCurve` from `Start` to `End` DungParams
as depth increases, then scaled by `DynamicDifficulty` (set by `AdaptDifficulty()`
based on average player HP and food count).

---

## Fog of War

`Raycaster` uses a **DDA (digital differential analysis)** algorithm to cast rays
within a cone (half-FOV = 45°, range = 3 cells). Each player has a per-turn
`VisibleArea` stored in `Playthrough.PlayersFoW`. The view mapper clips the
`GameView` to the visible area before broadcasting.

---

## Actor / Effect system

```
Actor
 ├─ BaseAttrs   (strength, max-HP, …)
 ├─ DerivedAttrs (= BaseAttrs + sum of active Effects)
 ├─ Vitals      (current HP, stamina)
 ├─ Effects     map[EffectType]*Effect
 ├─ EquippedGear map[ItemType]*Item   ← gear effects also feed DerivedAttrs
 └─ Traits      map[TriggerType][]*Reaction
```

`RecomputeStats()` resets `DerivedAttrs` to `BaseAttrs` then re-applies all
active effects (from `Effects` + `EquippedGear`).

Effects stack by extending duration / charges, never by multiplying stats.
Reactions are triggered by events (`TriggerOnHit`, `TriggerOnDamage`,
`TriggerEffectOnExpire`, `TriggerOnPreHit`) and processed by `ImpactResolver`.

---

## Pursuer RL subsystem

### Two-tier Director / Alien pattern

```
Every turn:
  if not RLActive:
    if player within rlTriggerRadius(8) OR ChScent > rlTriggerScent(0.25):
      → activate RL, reset FrustrationCount
    else:
      → Dijkstra wander (FallbackPolicy)
  if RLActive:
    → obs = BuildObservation(...)
    → action = policy.Predict(obs)   ← ONNXPolicy / FallbackPolicy / ScriptedPolicy
    if no LOS to player:
      FrustrationCount++
    if FrustrationCount >= rlFrustrationLimit(15):
      → deactivate RL, hand back to Director
```

### Observation tensor (1714 floats)

```
[channels: 14 × 11 × 11 = 1694 floats] + [scalars: 20 floats]
```

Layout is CHW row-major: `obs[c * H*W + y*W + x]`.

**Channels:**

| # | Name | Description |
|---|------|-------------|
| 0 | ChWalkable | 1.0 if tile is walkable |
| 1 | ChClosedDoor | 1.0 if closed door |
| 2 | ChOtherMonster | 1.0 if another monster present |
| 3 | ChSelfTrail | Pursuer's recent positions, linear decay |
| 4 | ChPlayerVisNow | 1.0 if player visible + LOS |
| 5 | ChPlayerMemory | Spike at last-known player pos, decays with TurnsSinceLOS |
| 6 | ChPlayerCone | 1.0 if inside player's flashlight cone |
| 7 | ChScent | Dijkstra distance from player, normalised by 30 |
| 8 | ChAgeLastSeen | Gaussian decay (σ=2.5) around last-seen pos |
| 9 | ChCovertPath | Cone-weighted Dijkstra (penalty=10 in cone), stealth gradient |
| 10 | ChPlayerTrail | Recent player positions, exponential decay (α=0.9) |
| 11 | ChInterceptPath | BFS from optimal intercept cell |
| 12 | ChVisitedCorridorPath | Dijkstra biased to player-visited corridors |
| 13 | ChFuturePlayerPos | Predicted player path (greedy exit descent), decay 0.8/step |

**Scalars (20):**

| # | Name |
|---|------|
| 0 | HP ratio |
| 1 | Stamina ratio |
| 2 | TurnsSinceLOS / horizon |
| 3 | Distance to last-seen (normalised diagonal) |
| 4 | Last-seen dx / map width |
| 5 | Last-seen dy / map height |
| 6 | Nearest pursuer dx / map width |
| 7 | Nearest pursuer dy / map height |
| 8 | In player cone flag |
| 9 | Player dx / map width |
| 10 | Player dy / map height |
| 11 | Cone dot product (pursuer dir · player-to-pursuer) |
| 12 | Player-to-exit distance (norm) |
| 13 | Pursuer-to-player distance (norm) |
| 14 | Intercept advantage (signed, norm) |
| 15 | Player in corridor flag |
| 16 | Map position X (norm) |
| 17 | Map position Y (norm) |
| 18 | Polar distance to player (norm) |
| 19 | Polar angle to player / π |

### Go ↔ Python parity

`BuildObservation` (Go) and `PursuerEnv._observe` (Python) must produce
byte-identical tensors. Two guards:

1. `rl/tests/test_parity.py` — dumps observations via `cmd/dump-observation`
   on committed fixtures and compares with the Python env.
2. Notebook final cell — runs PyTorch policy and exported ONNX through 1000
   random states and asserts `max(abs(diff)) < 1e-5`.

---

## Storage

| Adapter | Interface | Notes |
|---------|-----------|-------|
| `MemorySessionRepo` | `PlaythroughRepository` | In-process, no persistence |
| `JsonPlaythroughRepo` | `PlaythroughRepository` | JSON files in `PlaythroughsDir` |
| `CachedSessionRepo` | `PlaythroughRepository` | In-memory cache wrapping JSON repo |
| `JsonRulesRepo` | `RulesRepository` | JSON files in `RulesDir` |

---

## Multiplayer (in-process)

The game supports multiple simultaneous players within a single OS process.
`Server` owns the `GameLoop` and exposes `Subscribe(uuid)` + `CommandChan()`.
Each player session is a `Client` that writes to `CommandChan` and reads from
its private `<-chan GameView`. All communication is channel-based — no network sockets.

---

## CI

GitHub Actions (`go.yaml`) runs on pushes to `main` / `develop`:
1. `lint` — `go vet` + `gofumpt` formatting check.
2. `test` — installs ORT 1.23.0, runs `go test ./...` with `ROGUE_ONNX_LIB_PATH` set.
