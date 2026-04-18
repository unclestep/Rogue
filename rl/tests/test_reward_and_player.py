"""
Tests for the anti-camping reward, context-aware wait penalty, and goal-directed
scripted player.

These cover the behaviours the reward shaping is supposed to elicit — if the
Alien:Isolation-style pursuer starts camping the exit again, or the scripted
victim starts ignoring the exit, one of these will fail.
"""

from __future__ import annotations

from pathlib import Path

import numpy as np
import pytest

from rl.pursuer_env import (
    ACTION_WAIT,
    PURSUER_MEMORY_HORIZON,
    PursuerEnv,
    PursuerMemory,
    Topology,
)
from rl.scripted_player import _PROFILES, scripted_player_policy


FIXTURES = Path(__file__).resolve().parents[1] / "fixtures" / "topologies"


@pytest.fixture(scope="module")
def topology() -> Topology:
    paths = sorted(FIXTURES.glob("*.json"))
    assert paths
    return Topology.from_json(str(paths[0]))


def _make_env(**kwargs) -> PursuerEnv:
    return PursuerEnv(
        str(FIXTURES),
        scripted_player_policy,
        max_episode_steps=kwargs.pop("max_episode_steps", 50),
        seed=kwargs.pop("seed", 7),
        **kwargs,
    )


# ---------------------------------------------------------------------------
# Anti-camping reward.
# ---------------------------------------------------------------------------


def test_exit_camp_counter_starts_zero_after_reset(topology):
    env = _make_env()
    env.reset(seed=11)
    assert env._exit_camp_counter == 0


def test_exit_camp_escalates_when_parked_on_exit(topology):
    env = _make_env()
    env.reset(seed=12)

    # Place pursuer at the exit, player far away, force both to sit still.
    env.pursuer_pos = env.topology.exit_point
    env.player_pos = _pick_far_cell(env, env.topology.exit_point, min_dist=10)
    env._prev_scent_val = None
    env._exit_camp_counter = 0
    env._cone_mask_cache = env._compute_cone()

    rewards = []
    for _ in range(5):
        r = env._compute_reward(
            action=ACTION_WAIT,
            prev_pos=env.pursuer_pos,
            pursuer_hit=False,
        )
        rewards.append(r)

    assert env._exit_camp_counter == 5
    # Penalty strictly escalates each turn.
    for a, b in zip(rewards, rewards[1:]):
        assert b < a, f"camp penalty should escalate, got {rewards}"


def test_exit_camp_resets_when_player_approaches(topology):
    env = _make_env()
    env.reset(seed=13)

    # Park near the exit with the player far away — counter builds up.
    env.pursuer_pos = env.topology.exit_point
    env.player_pos = _pick_far_cell(env, env.topology.exit_point, min_dist=10)
    env._prev_scent_val = None
    env._exit_camp_counter = 0
    env._cone_mask_cache = env._compute_cone()
    for _ in range(3):
        env._compute_reward(action=ACTION_WAIT, prev_pos=env.pursuer_pos, pursuer_hit=False)
    assert env._exit_camp_counter == 3

    # Now player steps adjacent to pursuer — intercept mode, penalty must reset.
    env.player_pos = (env.pursuer_pos[0] + 1, env.pursuer_pos[1])
    env._compute_reward(action=ACTION_WAIT, prev_pos=env.pursuer_pos, pursuer_hit=False)
    assert env._exit_camp_counter == 0


def test_no_camp_penalty_when_far_from_exit(topology):
    env = _make_env()
    env.reset(seed=14)

    env.pursuer_pos = _pick_far_cell(env, env.topology.exit_point, min_dist=10)
    env.player_pos = _pick_far_cell(env, env.pursuer_pos, min_dist=8)
    env._prev_scent_val = None
    env._exit_camp_counter = 0
    env._cone_mask_cache = env._compute_cone()
    for _ in range(5):
        env._compute_reward(action=ACTION_WAIT, prev_pos=env.pursuer_pos, pursuer_hit=False)
    assert env._exit_camp_counter == 0


# ---------------------------------------------------------------------------
# Context-aware wait penalty.
# ---------------------------------------------------------------------------


def test_wait_penalty_blind_vs_stalking(topology):
    env = _make_env()
    env.reset(seed=20)

    env.pursuer_pos = _pick_far_cell(env, env.topology.exit_point, min_dist=10)
    env.player_pos = _pick_far_cell(env, env.pursuer_pos, min_dist=8)
    env._prev_scent_val = None
    env._exit_camp_counter = 0
    # Force pursuer out of the player's cone.
    env._cone_mask_cache = np.zeros(
        (env.topology.height, env.topology.width), dtype=bool
    )

    env.memory = PursuerMemory()
    env.memory.turns_since_los = 5  # stalking: recent sighting
    r_stalk = env._compute_reward(action=ACTION_WAIT, prev_pos=env.pursuer_pos, pursuer_hit=False)

    env._prev_scent_val = None
    env._exit_camp_counter = 0
    env.memory = PursuerMemory()
    env.memory.turns_since_los = PURSUER_MEMORY_HORIZON  # blind
    r_blind = env._compute_reward(action=ACTION_WAIT, prev_pos=env.pursuer_pos, pursuer_hit=False)

    # Blind wait strictly costs more than a stalking wait.
    assert r_blind < r_stalk


def test_wait_penalty_harshest_in_cone(topology):
    env = _make_env()
    env.reset(seed=21)

    env.pursuer_pos = _pick_far_cell(env, env.topology.exit_point, min_dist=10)
    env.player_pos = _pick_far_cell(env, env.pursuer_pos, min_dist=8)
    env._prev_scent_val = None
    env._exit_camp_counter = 0
    env.memory = PursuerMemory()
    env.memory.turns_since_los = 5

    # Pursuer out of cone.
    env._cone_mask_cache = np.zeros(
        (env.topology.height, env.topology.width), dtype=bool
    )
    r_out = env._compute_reward(action=ACTION_WAIT, prev_pos=env.pursuer_pos, pursuer_hit=False)

    env._prev_scent_val = None
    env._exit_camp_counter = 0
    env.memory = PursuerMemory()
    env.memory.turns_since_los = 5
    # Pursuer in cone.
    cone = np.zeros((env.topology.height, env.topology.width), dtype=bool)
    cone[env.pursuer_pos[1], env.pursuer_pos[0]] = True
    env._cone_mask_cache = cone
    r_in = env._compute_reward(action=ACTION_WAIT, prev_pos=env.pursuer_pos, pursuer_hit=False)

    assert r_in < r_out


# ---------------------------------------------------------------------------
# Goal-directed scripted player.
# ---------------------------------------------------------------------------


def test_profiles_have_required_keys():
    required = {"flee_prob", "wait_prob", "goal_prob", "panic_boost", "angle_jitter"}
    for name, profile in _PROFILES.items():
        missing = required - profile.keys()
        assert not missing, f"profile {name!r} missing {missing}"


def test_goal_directed_player_makes_progress_toward_exit():
    """
    Roll out several episodes with the scripted player and check the mean
    distance to exit *decreases* over the episode. Random-walk victim from the
    previous version would show no trend.
    """
    env = _make_env(max_episode_steps=80)
    trend_drops = 0
    for ep in range(5):
        obs, info = env.reset(seed=500 + ep)
        start = _chebyshev(env.player_pos, env.topology.exit_point)
        for _ in range(30):
            _, _, term, trunc, _ = env.step(int(env.action_space.sample()))
            if term or trunc:
                break
        end = _chebyshev(env.player_pos, env.topology.exit_point)
        if end < start:
            trend_drops += 1
    # At least 3 of 5 episodes show progress toward exit.
    assert trend_drops >= 3, f"player didn't trend toward exit: {trend_drops}/5 dropped"


# ---------------------------------------------------------------------------
# Helpers.
# ---------------------------------------------------------------------------


def _chebyshev(a: tuple[int, int], b: tuple[int, int]) -> int:
    return max(abs(a[0] - b[0]), abs(a[1] - b[1]))


def _pick_far_cell(
    env: PursuerEnv, anchor: tuple[int, int], min_dist: int
) -> tuple[int, int]:
    assert env.topology is not None
    for p in env.topology.walkable_points():
        if _chebyshev(p, anchor) >= min_dist:
            return p
    # Fall back to any walkable cell that isn't the anchor.
    for p in env.topology.walkable_points():
        if p != anchor:
            return p
    return anchor
