# Gouge — Index

Go implementation of Rogue (roguelike). Module: `github.com/unclestep/Rogue`.

---

## Entry points (`cmd/`)

| Binary | File | Purpose |
|--------|------|---------|
| Main game | `cmd/main.go` | Starts TUI + Server, loads ONNX policy if env set |
| Topology dumper | `cmd/dump-topology/main.go` | Dumps N BSP maps to JSON for RL training |
| Observation dumper | `cmd/dump-observation/main.go` | Dumps pursuer observation tensors for parity tests |

---

## Domain layer (`internal/domain/`)

### Models (`model/`)

| File | Key types |
|------|-----------|
| `actor.go` | `Actor`, `ActorType` (Player/Zombie/Vampire/Ghost/Ogre/SnakeMage/Mimic/Pursuer), `AttrType`, `VitalType`, `BehaviorType`, constructors `NewDefault*` |
| `map.go` | `Map`, `Room`, `Cell`, `TileType`, `DoorMetadata` — 3-layer grid (tile/item/actor) |
| `playthrough.go` | `Playthrough` — game session state, `GameState`, `GameStats`, `LevelMetrics`, dynamic difficulty |
| `rules.go` | `GameRules`, `DungParams`, `DifficultyCurve` — config for dungeon generation and difficulty |
| `session_context.go` | `SessionContext` — per-turn runtime (scent maps, RNG, exit scent cache) |
| `item.go` | `Item`, `ItemType`, `ItemLabel`, `Backpack` |
| `backpack.go` | `Backpack`, slot management |
| `effect.go` | `Effect`, `EffectType` — temporary/permanent buffs/debuffs |
| `event.go` | `Event` — turn events for view broadcasting |
| `impact.go` | `Impact` — combat/effect application data |
| `intent.go` | `Intent` — player/monster action queue entry |
| `fow.go` | `VisibleArea`, `FogType` — fog of war state per player |
| `enums_enumer.go` | Auto-generated stringer implementations |

### Services (`service/`)

**Generation**

| File | Purpose |
|------|---------|
| `gen_topology.go` | BSP topology generation — sectorize, create rooms, corridors, entrance/exit |
| `gen_dungeon.go` | `DungeonGenerator` — spawns actors and items using `DungParams` |
| `gen_object_spawner.go` | Object spawner helpers |
| `gen_door_locker.go` | Randomly locks doors with colored keys |

**Gameplay**

| File | Purpose |
|------|---------|
| `play_movement.go` | `Movement` — validates/executes actor movement |
| `play_combat.go` | `Combat` — hit resolution, damage, counter-attacks |
| `play_interact.go` | `Interactor` — door open/close, exit interaction |
| `play_item_pickup.go` | `Pickup` — item pick-up logic |
| `play_item_usage.go` | `ItemUsage` — consuming items from backpack |
| `logic_monster.go` | `MonsterController`, `ChaseBehavior`, `WanderBehavior` |
| `logic_monster_pursuer.go` | `PursuerBehavior` — two-tier Director/Alien AI |

**Resolution**

| File | Purpose |
|------|---------|
| `res_movement.go` | `MoveResolver` — movement side-effects (stamina, FoW update) |
| `res_impact.go` | `ImpactResolver` — applies stat changes, effects, reactions |

**Systems**

| File | Purpose |
|------|---------|
| `sys_pathfinder.go` | `Pathfinder` — A* and Dijkstra path-finding |
| `sys_raycaster.go` | `Raycaster` — DDA flashlight FOV (half-FOV=45°, range=3) |

**RL subsystem**

| File | Purpose |
|------|---------|
| `rl_policy.go` | `Policy` interface, `FallbackPolicy`, `ScriptedPolicy`, action↔vector mapping |
| `rl_onnx.go` | `ONNXPolicy` — ONNX Runtime inference session |
| `rl_observation.go` | `BuildObservation` — 1714-float tensor (14ch×11×11 + 20 scalars) |

---

## Application layer (`internal/application/`)

| File | Purpose |
|------|---------|
| `game_loop.go` | `GameLoop` — command fan-in, 1s ticker, broadcast snapshots to subscribers |
| `usecase/uc_resolve_state.go` | `ResolveState` — routes commands to `ResolveTurn` |
| `usecase/uc_resolve_turn.go` | `ResolveTurn` — orchestrates one full turn (intents → physics → FoW → scentmaps) |
| `usecase/uc_submit_intent.go` | `SubmitIntent` — validates and enqueues player intents |
| `view/mapper.go` | `Mapper` — converts `Playthrough` to per-player `GameView` snapshots |
| `port/repository.go` | `PlaythroughRepository`, `RulesRepository` interfaces |

---

## Bootstrap (`internal/bootstrap/`)

| File | Purpose |
|------|---------|
| `config.go` | `Config` — filesystem paths + `PursuerModelPath` |
| `providers.go` | Wire provider functions including `providePursuerPolicy` |
| `wire.go` / `wire_gen.go` | Google Wire dependency injection graph |

---

## Presentation layer (`internal/presentation/`)

### TUI (`tui/`)

| File | Purpose |
|------|---------|
| `ui_model.go` | `UIModel` — BubbleTea root model |
| `ui_behavior.go` | Keyboard event dispatch |
| `ui_render.go` | Map/HUD rendering |
| `ui_states_*.go` | States: Nickname, Menu, Lobby, Playing, GameOver |

### Network (`network/`)

| File | Purpose |
|------|---------|
| `server.go` | `Server` — wraps `GameLoop`, exposes `Subscribe`/`CommandChan` |
| `client.go` | `Client` — connects to Server in same-process multiplayer |
| `app.go` | Wires Server + one or more Clients |

---

## Infrastructure (`internal/infrastructure/storage/`)

| File | Purpose |
|------|---------|
| `json/playthrough_repo.go` | JSON file-based playthrough persistence |
| `json/rules_repo.go` | JSON file-based rules loading |
| `memory_session_repo.go` | In-memory session repository |
| `cached_session_repo.go` | Caching wrapper over the JSON repo |
| `json/dto/`, `json/mapper/` | JSON DTO types and mappers |

---

## DTOs (`internal/dto/`)

| File | Types |
|------|-------|
| `command.go` | `Command`, `ActionType` |
| `snapshot.go` | `GameView`, `ActorView`, `ItemView`, … |
| `topology_dump.go` | `TopologyDump` — serialised dungeon for RL training |

---

## Shared packages (`pkg/`)

| Package | Contents |
|---------|---------|
| `pkg/algorithm/` | A* (`astar.go`), graph (`graph.go`), priority queue, utilities |
| `pkg/geometry/` | `Point` — 2D integer coordinate with cardinal/diagonal directions |
| `pkg/conv/mapconv.go` | Generic map conversions |
| `pkg/printer/printer.go` | Debug/log formatting helpers |
| `pkg/utils/` | Matrix creation, shuffle |

---

## RL subsystem (`rl/`)

| File | Purpose |
|------|---------|
| `pursuer_env.py` | Gymnasium env mirroring Go's `BuildObservation` 1:1 |
| `scripted_player.py` | Random wander + flee opponent for training |
| `pursuer_training.ipynb` | Main PPO training notebook (Colab or local Jupyter) |
| `tools/gen_tiny_onnx.py` | Generates minimal ONNX fixture for Go tests |
| `tests/` | Parity tests (Go↔Python), env tests (attack, spawn, observation, reward) |
| `models/pursuer.onnx` | Trained policy weights (gitignored, except `.data` file) |

---

## Tests (`tests/`)

| Directory | Coverage |
|-----------|---------|
| `model_test/` | Domain model unit tests |
| `service_test/` | Domain service unit tests |
| `usecase_test/` | Use-case integration tests |
| `storage_test/` | Repository tests (memory, JSON, cached) |
| `dto_test/` | DTO serialisation tests |
| `application_test/` | ONNX smoke test, server integration test |
