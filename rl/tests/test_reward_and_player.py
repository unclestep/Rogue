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
    ACTION_UP,
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


def test_exit_camp_escalating_penalty_when_parked_on_exit(topology):
    """Anti-camp penalty escalates: −0.05 × k each turn (turn 1 → -0.05,
    turn 2 → -0.10, …). This is intentionally NOT flat — we want increasing
    pressure to leave the exit area, bounded by the episode length.
    """
    env = _make_env()
    env.reset(seed=12)

    env.pursuer_pos = env.topology.exit_point
    env.player_pos = _pick_far_cell(env, env.topology.exit_point, min_dist=10)
    env._prev_scent_val = None
    env._exit_camp_counter = 0
    env._was_visible_before_step = False
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
    # Each subsequent reward must be more negative than the previous one.
    for a, b in zip(rewards, rewards[1:]):
        assert b < a, f"camp penalty must escalate, got {rewards}"


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
# Terminal reward exact magnitudes.
# All components except the target are isolated:
#   - prev_pos == pursuer_pos  → approach shaping = 0
#   - _exit_scent_cache = None  → no exit-blocking penalty
#   - distractor_pos = None     → no crowd penalty
#   - _first_detection_done = True → no ambush bonus
#   - pursuer far from exit     → anti-camp counter stays 0
#   - cone = zeros              → no freeze penalty in wait tests
# ---------------------------------------------------------------------------


def _isolate(env: PursuerEnv) -> None:
    """Null out all optional reward components."""
    env._exit_scent_cache = None
    env.distractor_pos = None
    env._was_visible_before_step = True
    env._exit_camp_counter = 0
    env._cone_mask_cache = np.zeros(
        (env.topology.height, env.topology.width), dtype=bool
    )


def test_terminal_reward_player_caught_visible():
    """+8.0 when player HP reaches 0 while pursuer was visible (visible kill)."""
    env = _make_env()
    env.reset(seed=42)
    env.player_hp = 0
    env.pursuer_hp = 1
    env.pursuer_pos = _pick_far_cell(env, env.topology.exit_point, min_dist=10)
    env.player_pos = _pick_far_cell(env, env.topology.exit_point, min_dist=8)
    _isolate(env)
    env._was_visible_before_step = True

    r = env._compute_reward(action=ACTION_UP, prev_pos=env.pursuer_pos, pursuer_hit=False)
    assert abs(r - 7.98) < 1e-6, f"Expected +8.0 - 0.02 = 7.98, got {r}"


def test_terminal_reward_player_caught_ambush():
    """+20.0 when player HP reaches 0 from stealth (pursuer not visible last step)."""
    env = _make_env()
    env.reset(seed=42)
    env.player_hp = 0
    env.pursuer_hp = 1
    env.pursuer_pos = _pick_far_cell(env, env.topology.exit_point, min_dist=10)
    env.player_pos = _pick_far_cell(env, env.topology.exit_point, min_dist=8)
    _isolate(env)
    env._was_visible_before_step = False

    r = env._compute_reward(action=ACTION_UP, prev_pos=env.pursuer_pos, pursuer_hit=False)
    assert abs(r - 19.98) < 1e-6, f"Expected +20.0 - 0.02 = 19.98, got {r}"


def test_terminal_reward_player_escaped():
    """-20.0 when player reaches exit; only time penalty on top."""
    env = _make_env()
    env.reset(seed=42)
    env.player_hp = 1
    env.pursuer_hp = 1
    env.pursuer_pos = _pick_far_cell(env, env.topology.exit_point, min_dist=10)
    env.player_pos = env.topology.exit_point
    _isolate(env)

    r = env._compute_reward(action=ACTION_UP, prev_pos=env.pursuer_pos, pursuer_hit=False)
    assert abs(r - (-20.02)) < 1e-6, f"Expected -20.0 - 0.02 = -20.02, got {r}"


def test_terminal_reward_pursuer_death():
    """-2.0 when pursuer HP reaches 0; only time penalty on top."""
    env = _make_env()
    env.reset(seed=42)
    env.pursuer_hp = 0
    env.player_hp = 1
    env.pursuer_pos = _pick_far_cell(env, env.topology.exit_point, min_dist=10)
    env.player_pos = _pick_far_cell(env, env.topology.exit_point, min_dist=8)
    _isolate(env)

    r = env._compute_reward(action=ACTION_UP, prev_pos=env.pursuer_pos, pursuer_hit=False)
    assert abs(r - (-2.02)) < 1e-6, f"Expected -2.0 - 0.02 = -2.02, got {r}"


def test_approach_shaping_one_step_closer():
    """Moving one step closer to player adds exactly +0.02 reward."""
    env = _make_env()
    env.reset(seed=42)
    env.player_hp = 1
    env.pursuer_hp = 1
    pursuer = _pick_far_cell(env, env.topology.exit_point, min_dist=10)
    env.pursuer_pos = pursuer
    # Player 5 cells east — no need to be walkable for reward calc.
    env.player_pos = (pursuer[0] + 5, pursuer[1])
    _isolate(env)

    # Baseline: prev_pos == pursuer_pos → approach = 0.
    r_same = env._compute_reward(action=ACTION_UP, prev_pos=pursuer, pursuer_hit=False)
    env._exit_camp_counter = 0

    # One step west means previous position was further from player.
    prev_west = (pursuer[0] - 1, pursuer[1])
    r_closer = env._compute_reward(action=ACTION_UP, prev_pos=prev_west, pursuer_hit=False)

    diff = r_closer - r_same
    assert abs(diff - 0.02) < 1e-6, f"Approach delta expected 0.02, got {diff}"


def test_wait_in_cone_adds_exact_penalty():
    """-0.05 cone-presence + -0.1 freeze when WAIT action while pursuer is in player cone."""
    env = _make_env()
    env.reset(seed=42)
    env.player_hp = 1
    env.pursuer_hp = 1
    env.pursuer_pos = _pick_far_cell(env, env.topology.exit_point, min_dist=10)
    env.player_pos = _pick_far_cell(env, env.pursuer_pos, min_dist=5)
    env._exit_scent_cache = None
    env.distractor_pos = None
    env._was_visible_before_step = True
    env._exit_camp_counter = 0
    env.memory = PursuerMemory()
    env.memory.turns_since_los = 5  # not blind

    # No cone -> no penalties.
    env._cone_mask_cache = np.zeros(
        (env.topology.height, env.topology.width), dtype=bool
    )
    r_out = env._compute_reward(action=ACTION_WAIT, prev_pos=env.pursuer_pos, pursuer_hit=False)

    env._exit_camp_counter = 0
    # Pursuer cell lit -> -0.05 cone-presence + -0.1 freeze stack.
    cone = np.zeros((env.topology.height, env.topology.width), dtype=bool)
    cone[env.pursuer_pos[1], env.pursuer_pos[0]] = True
    env._cone_mask_cache = cone
    r_in = env._compute_reward(action=ACTION_WAIT, prev_pos=env.pursuer_pos, pursuer_hit=False)

    delta = r_in - r_out
    assert abs(delta - (-0.15)) < 1e-6, f"Wait-in-cone delta expected -0.15, got {delta}"


def test_wait_blind_adds_exact_penalty():
    """-0.02 extra when WAIT and blind (turns_since_los > 10) and not in cone."""
    env = _make_env()
    env.reset(seed=42)
    env.player_hp = 1
    env.pursuer_hp = 1
    env.pursuer_pos = _pick_far_cell(env, env.topology.exit_point, min_dist=10)
    env.player_pos = _pick_far_cell(env, env.pursuer_pos, min_dist=5)
    env._exit_scent_cache = None
    env.distractor_pos = None
    env._was_visible_before_step = True
    env._exit_camp_counter = 0
    # No cone: only the blind penalty can fire.
    env._cone_mask_cache = np.zeros(
        (env.topology.height, env.topology.width), dtype=bool
    )

    env.memory = PursuerMemory()
    env.memory.turns_since_los = 5  # stalking (≤10): no blind penalty
    r_stalk = env._compute_reward(action=ACTION_WAIT, prev_pos=env.pursuer_pos, pursuer_hit=False)

    env._exit_camp_counter = 0
    env.memory = PursuerMemory()
    env.memory.turns_since_los = 11  # blind (>10): should add -0.02
    r_blind = env._compute_reward(action=ACTION_WAIT, prev_pos=env.pursuer_pos, pursuer_hit=False)

    delta = r_blind - r_stalk
    assert abs(delta - (-0.02)) < 1e-6, f"Blind-wait penalty expected -0.02, got {delta}"


def test_cone_presence_adds_exact_penalty():
    """-0.05 per step when pursuer is inside player cone (action != WAIT)."""
    env = _make_env()
    env.reset(seed=42)
    env.player_hp = 1
    env.pursuer_hp = 1
    pursuer = _pick_far_cell(env, env.topology.exit_point, min_dist=10)
    env.pursuer_pos = pursuer
    env.player_pos = _pick_far_cell(env, pursuer, min_dist=5)
    _isolate(env)

    # Out of cone baseline.
    r_out = env._compute_reward(action=ACTION_UP, prev_pos=pursuer, pursuer_hit=False)

    env._exit_camp_counter = 0
    # In cone: stack -0.05 cone-presence on top.
    cone = np.zeros((env.topology.height, env.topology.width), dtype=bool)
    cone[pursuer[1], pursuer[0]] = True
    env._cone_mask_cache = cone
    r_in = env._compute_reward(action=ACTION_UP, prev_pos=pursuer, pursuer_hit=False)

    delta = r_in - r_out
    assert abs(delta - (-0.05)) < 1e-6, f"Cone-presence penalty expected -0.05, got {delta}"


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
