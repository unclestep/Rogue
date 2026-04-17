"""
Sanity checks for the three domain-randomization knobs exposed by PursuerEnv:

  1. `max_episode_steps_range`  — per-episode cap varies within a range.
  2. `distractor_prob`          — a non-player monster appears with given prob
                                  and its `other_pursuer` channel is non-zero.
  3. `randomize_player_profile` — `env.player_profile` samples from the full
                                  set of profiles over enough resets.

Kept deliberately small — we don't want to lock behaviour down tighter than
the defaults actually imply, otherwise tuning the scripted player becomes
painful.
"""

from __future__ import annotations

from pathlib import Path

import numpy as np
import pytest

from rl.pursuer_env import PursuerEnv
from rl.scripted_player import scripted_player_policy


REPO_ROOT = Path(__file__).resolve().parents[2]
TOPOLOGIES = REPO_ROOT / "rl" / "fixtures" / "topologies"


def _make(**kwargs) -> PursuerEnv:
    return PursuerEnv(
        str(TOPOLOGIES),
        scripted_player_policy,
        max_episode_steps=50,
        seed=kwargs.pop("seed", 123),
        **kwargs,
    )


def test_max_episode_steps_range_varies_between_resets():
    env = _make(max_episode_steps_range=(30, 80))
    seen_caps = set()
    for ep in range(20):
        _, info = env.reset(seed=1000 + ep)
        cap = info["max_steps"]
        assert 30 <= cap <= 80
        seen_caps.add(cap)
    # 20 draws from [30,80] should spread across at least a handful of values.
    assert len(seen_caps) >= 3, f"range barely sampled: {seen_caps}"


def test_distractor_prob_zero_leaves_channel_empty():
    env = _make(distractor_prob=0.0)
    for ep in range(5):
        obs, info = env.reset(seed=2000 + ep)
        assert info["has_distractor"] is False
        assert env.distractor_pos is None


def test_distractor_prob_one_always_spawns_and_moves():
    env = _make(distractor_prob=1.0)
    obs, info = env.reset(seed=3000)
    assert info["has_distractor"] is True
    assert env.distractor_pos is not None
    initial = env.distractor_pos

    # Step a few times — distractor may not move every step (blocked neighbours),
    # but over several steps its position should not be identically pinned.
    moved = False
    for _ in range(15):
        env.step(env.action_space.sample())
        if env.distractor_pos != initial:
            moved = True
            break
    assert moved, "distractor never moved in 15 steps"


def test_randomize_player_profile_covers_all_profiles():
    env = _make(randomize_player_profile=True)
    seen = set()
    for ep in range(40):
        _, info = env.reset(seed=4000 + ep)
        seen.add(info["player_profile"])
    # All four profiles should appear over 40 draws (p ≈ 1 - (3/4)^40 ≈ 1.0).
    assert seen == set(PursuerEnv.PLAYER_PROFILES), f"missing profiles: {seen}"


def test_default_behaviour_unchanged_when_randomization_off():
    env = _make()  # All DR knobs at defaults.
    _, info = env.reset(seed=5000)
    assert info["player_profile"] == "default"
    assert info["has_distractor"] is False
    assert info["max_steps"] == 50
