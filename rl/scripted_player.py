"""
Scripted opponent for PursuerEnv.

The player is goal-directed — most turns it descends an exit-scent gradient
toward `topology.exit_point`, which is what real players do. That gives the
pursuer a consistent target of movement to stalk and intercept, instead of the
random flailing the previous version produced. We still randomize around the
goal to keep training diverse:

  * flee_prob    — chance of stepping away when pursuer is adjacent.
  * wait_prob    — chance of standing still on a normal turn.
  * goal_prob    — chance of taking the scent-descending move vs a random step
                   (imperfection so pursuer doesn't overfit a perfect player).
  * panic_boost  — multiplier applied to goal_prob when the pursuer is visible
                   in the player's cone (realistic "oh god run for the exit").
  * angle_jitter — per-step cone rotation magnitude.

Profiles:

  * default    — balanced.
  * aggressive — hunts: low flee, low wait, ignores the exit more often.
  * cautious   — camps: medium flee, high wait, sticks to the exit heavily.
  * timid      — flees: high flee, full goal focus under panic.

All randomness flows through the RNG the env passes in, so episodes stay
reproducible under `env.reset(seed=...)`.
"""

from __future__ import annotations

import math
import random
from typing import TYPE_CHECKING

try:
    from .pursuer_env import (
        ACTION_DOWN,
        ACTION_LEFT,
        ACTION_RIGHT,
        ACTION_UP,
        ACTION_VECTORS,
        ACTION_WAIT,
    )
except ImportError:  # Colab: %%writefile drops us as a top-level module
    from pursuer_env import (
        ACTION_DOWN,
        ACTION_LEFT,
        ACTION_RIGHT,
        ACTION_UP,
        ACTION_VECTORS,
        ACTION_WAIT,
    )

if TYPE_CHECKING:
    try:
        from .pursuer_env import PursuerEnv
    except ImportError:
        from pursuer_env import PursuerEnv

CARDINAL_ACTIONS = [ACTION_UP, ACTION_RIGHT, ACTION_DOWN, ACTION_LEFT]

_PROFILES: dict[str, dict[str, float]] = {
    "training_dummy": {"flee_prob": 0.0,  "wait_prob": 0.85, "goal_prob": 0.15, "panic_boost": 0.0, "angle_jitter": 0.1},
    
    "default":    {"flee_prob": 0.5,  "wait_prob": 0.1,  "goal_prob": 0.75, "panic_boost": 1.0, "angle_jitter": 0.6},
    "aggressive": {"flee_prob": 0.3,  "wait_prob": 0.02, "goal_prob": 0.55, "panic_boost": 0.8, "angle_jitter": 1.0},
    "cautious":   {"flee_prob": 0.7,  "wait_prob": 0.3,  "goal_prob": 0.85, "panic_boost": 1.0, "angle_jitter": 0.2},
    "timid":      {"flee_prob": 0.95, "wait_prob": 0.1,  "goal_prob": 0.95, "panic_boost": 1.0, "angle_jitter": 0.9},
}


def scripted_player_policy(env: "PursuerEnv", rng: random.Random) -> tuple[int, float]:
    """Goal-directed player. Returns (action, new_aim_angle_rad)."""
    assert env.topology is not None

    profile = _PROFILES.get(
        getattr(env, "player_profile", "default"), _PROFILES["default"]
    )
    flee_prob = profile["flee_prob"]
    wait_prob = profile["wait_prob"]
    goal_prob = profile["goal_prob"]
    panic_boost = profile["panic_boost"]
    angle_jitter = profile["angle_jitter"]

    dx = env.pursuer_pos[0] - env.player_pos[0]
    dy = env.pursuer_pos[1] - env.player_pos[1]

    # Panic mode: pursuer is in our cone right now. Sprint for the exit, cut
    # wait probability, widen angle jitter (player looking around wildly).
    pursuer_visible_now = (
        env._cone_mask_cache is not None
        and env._cone_mask_cache[env.pursuer_pos[1], env.pursuer_pos[0]]
    )
    if pursuer_visible_now:
        goal_prob = min(1.0, goal_prob * (1.0 + panic_boost))
        wait_prob = wait_prob * 0.2
        angle_jitter = min(math.pi, angle_jitter * 1.5)

    if abs(dx) + abs(dy) == 1 and rng.random() < flee_prob:
        action = _flee_action(dx, dy)
    elif rng.random() < wait_prob:
        action = ACTION_WAIT
    elif rng.random() < goal_prob:
        action = _goal_directed_action(env, rng)
    else:
        action = _safe_random_step(env, rng)

    angle_delta = rng.uniform(-angle_jitter, angle_jitter)
    new_angle = _wrap_angle(env.player_angle + angle_delta)
    return action, new_angle


def _flee_action(dx: int, dy: int) -> int:
    """Step away from (dx, dy) — the vector *from* player *to* pursuer."""
    if abs(dx) >= abs(dy):
        return ACTION_LEFT if dx > 0 else ACTION_RIGHT
    return ACTION_UP if dy > 0 else ACTION_DOWN


def _goal_directed_action(env: "PursuerEnv", rng: random.Random) -> int:
    """Step down the exit-scent gradient; break ties randomly.

    Falls back to a random cardinal if no neighbour reduces scent (e.g. player
    is already on the exit tile, or boxed in by non-walkable cells)."""
    scent = env.exit_scent_map()
    px, py = env.player_pos
    current = int(scent[py, px])
    INF = 10**8
    best = current
    best_actions: list[int] = []
    for a in CARDINAL_ACTIONS:
        vx, vy = ACTION_VECTORS[a]
        nx, ny = px + vx, py + vy
        if env.topology is None or not env.topology.is_walkable(nx, ny):
            continue
        v = int(scent[ny, nx])
        if v >= INF:
            continue
        if v < best:
            best = v
            best_actions = [a]
        elif v == best:
            best_actions.append(a)
    if not best_actions or best == current:
        return _safe_random_step(env, rng)
    return rng.choice(best_actions)


def _safe_random_step(env: "PursuerEnv", rng: random.Random) -> int:
    """Pick a walkable cardinal if possible, otherwise any cardinal."""
    assert env.topology is not None
    candidates = []
    for a in CARDINAL_ACTIONS:
        vx, vy = ACTION_VECTORS[a]
        if env.topology.is_walkable(env.player_pos[0] + vx, env.player_pos[1] + vy):
            candidates.append(a)
    if candidates:
        return rng.choice(candidates)
    return rng.choice(CARDINAL_ACTIONS)


def _wrap_angle(a: float) -> float:
    while a > math.pi:
        a -= 2 * math.pi
    while a < -math.pi:
        a += 2 * math.pi
    return a
