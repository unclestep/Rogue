"""
Tests for the spawn_radius-based actor placement introduced in PR #1.

Before the fix, _place_actors shuffled rooms without connectivity checks,
so the pursuer could spawn in a room isolated from the player. New logic
places them at Chebyshev distance within [lo, hi] AND verifies the scent
BFS from the player reaches the pursuer cell.
"""
from __future__ import annotations

from pathlib import Path

import pytest

from rl.pursuer_env import PursuerEnv, chase_scent_map
from rl.scripted_player import scripted_player_policy


FIXTURES = Path(__file__).resolve().parents[1] / "fixtures" / "topologies"


def _make_env(spawn_radius=(3, 8), **kwargs) -> PursuerEnv:
    return PursuerEnv(
        str(FIXTURES),
        scripted_player_policy,
        max_episode_steps=40,
        seed=kwargs.pop("seed", 42),
        spawn_radius=spawn_radius,
        **kwargs,
    )


def test_spawn_radius_respected_and_reachable() -> None:
    env = _make_env(spawn_radius=(3, 8))
    misses = 0
    unreachables = 0
    for ep in range(100):
        env.reset(seed=1000 + ep)
        px, py = env.player_pos
        ox, oy = env.pursuer_pos
        cheb = max(abs(px - ox), abs(py - oy))
        if not (3 <= cheb <= 8):
            misses += 1
        scent = chase_scent_map(env.topology, env.player_pos)
        if int(scent[oy, ox]) >= 10**9:
            unreachables += 1
    assert misses == 0, f"spawn outside radius in {misses}/100 resets"
    assert unreachables == 0, f"unreachable spawn in {unreachables}/100 resets"


def test_spawn_radius_none_uses_fallback() -> None:
    env = _make_env(spawn_radius=None)
    for ep in range(20):
        env.reset(seed=2000 + ep)
        assert env.pursuer_pos != env.player_pos
        # Still reachable via BFS.
        scent = chase_scent_map(env.topology, env.player_pos)
        assert int(scent[env.pursuer_pos[1], env.pursuer_pos[0]]) < 10**9


def test_tight_radius_still_finds_candidate() -> None:
    env = _make_env(spawn_radius=(2, 3))
    for ep in range(20):
        env.reset(seed=3000 + ep)
        px, py = env.player_pos
        ox, oy = env.pursuer_pos
        cheb = max(abs(px - ox), abs(py - oy))
        # Either exact radius hit, or fallback kicked in — but must be reachable.
        scent = chase_scent_map(env.topology, env.player_pos)
        assert int(scent[oy, ox]) < 10**9
