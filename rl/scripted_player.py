"""
Scripted opponent for PursuerEnv.

The player behaviour is intentionally crude — we need the pursuer to learn
stealth/ambush tactics, not to counter a sophisticated policy. Profiles keep
training diverse enough to transfer to the real Go game, which has different
player styles in different moments:

  * default    — balanced: moderate flee, some waits, medium angle jitter.
  * aggressive — high flee probability, low wait, wide angle sweeps (simulates
                 a player actively hunting).
  * cautious   — low flee, high wait, narrow angle jitter (simulates a player
                 standing guard in a chokepoint).
  * timid      — very high flee, medium wait, wide angle sweeps (simulates a
                 player trying to escape at all costs).

All randomness flows through the RNG the env passes in, so episodes stay
reproducible under `env.reset(seed=...)`.
"""

from __future__ import annotations

import math
import random
from typing import TYPE_CHECKING

from .pursuer_env import (
    ACTION_DOWN,
    ACTION_LEFT,
    ACTION_RIGHT,
    ACTION_UP,
    ACTION_VECTORS,
    ACTION_WAIT,
)

if TYPE_CHECKING:
    from .pursuer_env import PursuerEnv

CARDINAL_ACTIONS = [ACTION_UP, ACTION_RIGHT, ACTION_DOWN, ACTION_LEFT]

# Per-profile behavioural knobs. Keep the "default" row identical to the
# pre-randomization defaults so existing tests/baselines stay unchanged.
_PROFILES: dict[str, dict[str, float]] = {
    "default":    {"flee_prob": 0.5, "wait_prob": 0.2, "angle_jitter": 0.6},
    "aggressive": {"flee_prob": 0.8, "wait_prob": 0.05, "angle_jitter": 1.0},
    "cautious":   {"flee_prob": 0.2, "wait_prob": 0.5, "angle_jitter": 0.2},
    "timid":      {"flee_prob": 0.95, "wait_prob": 0.2, "angle_jitter": 0.9},
}


def scripted_player_policy(env: "PursuerEnv", rng: random.Random) -> tuple[int, float]:
    """Default player for training. Returns (action, new_aim_angle_rad)."""
    assert env.topology is not None

    profile = _PROFILES.get(getattr(env, "player_profile", "default"), _PROFILES["default"])
    flee_prob = profile["flee_prob"]
    wait_prob = profile["wait_prob"]
    angle_jitter = profile["angle_jitter"]

    dx = env.pursuer_pos[0] - env.player_pos[0]
    dy = env.pursuer_pos[1] - env.player_pos[1]

    # Flee if adjacent.
    if abs(dx) + abs(dy) == 1 and rng.random() < flee_prob:
        action = _flee_action(env, dx, dy)
    elif rng.random() < wait_prob:
        action = ACTION_WAIT
    else:
        action = rng.choice(CARDINAL_ACTIONS)
        # Prefer walkable moves — re-roll once if we're about to bump.
        vec = ACTION_VECTORS[action]
        tgt = (env.player_pos[0] + vec[0], env.player_pos[1] + vec[1])
        if not env.topology.is_walkable(*tgt):
            action = rng.choice(CARDINAL_ACTIONS)

    # Rotate cone by a random small angle every step.
    angle_delta = rng.uniform(-angle_jitter, angle_jitter)
    new_angle = _wrap_angle(env.player_angle + angle_delta)
    return action, new_angle


def _flee_action(env: "PursuerEnv", dx: int, dy: int) -> int:
    """Step away from (dx, dy) — the vector *from* player *to* pursuer."""
    # Negate so we move opposite to the pursuer.
    if abs(dx) >= abs(dy):
        return ACTION_LEFT if dx > 0 else ACTION_RIGHT
    return ACTION_UP if dy > 0 else ACTION_DOWN


def _wrap_angle(a: float) -> float:
    while a > math.pi:
        a -= 2 * math.pi
    while a < -math.pi:
        a += 2 * math.pi
    return a
