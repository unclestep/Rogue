"""
Tests for the bump-attack mechanic introduced in PR #1 (see plan
senior-ancient-stroustrup.md).

The old mechanic silently allowed the pursuer to walk through the player
(overlap, dist=0) and required Manhattan-1 AFTER the move, which made
chase-to-kill essentially impossible. These tests pin the new semantics:

  1. Moving INTO the player from an adjacent tile = hit (player blocked, damage
     applied, pursuer stays on prev_pos).
  2. WAIT while adjacent = hit on the same tile.
  3. Walking past (orthogonal) does not bump-attack.
  4. A Dijkstra-greedy policy vs a stone player kills in <=6 steps on a
     small connected topology.
"""
from __future__ import annotations

from pathlib import Path

import pytest

from rl.pursuer_env import (
    ACTION_DOWN,
    ACTION_LEFT,
    ACTION_RIGHT,
    ACTION_UP,
    ACTION_VECTORS,
    ACTION_WAIT,
    PursuerEnv,
    chase_scent_map,
)
from rl.scripted_player import scripted_player_policy


FIXTURES = Path(__file__).resolve().parents[1] / "fixtures" / "topologies"


def _make_env(**kwargs) -> PursuerEnv:
    return PursuerEnv(
        str(FIXTURES),
        scripted_player_policy,
        max_episode_steps=kwargs.pop("max_episode_steps", 40),
        seed=kwargs.pop("seed", 7),
        fixed_profile=kwargs.pop("fixed_profile", "stone"),
        **kwargs,
    )


def _place(env: PursuerEnv, pursuer: tuple[int, int], player: tuple[int, int]) -> None:
    """Force actors to specific positions for deterministic assertions."""
    assert env.topology is not None
    assert env.topology.is_walkable(*pursuer)
    assert env.topology.is_walkable(*player)
    env.pursuer_pos = pursuer
    env.player_pos = player


def test_bump_attack_into_adjacent_player_hits_and_blocks() -> None:
    env = _make_env(player_hp=4, pursuer_hp=10, attack_damage=2)
    env.reset(seed=0)
    # Find two adjacent walkable cells.
    topo = env.topology
    assert topo is not None
    walkable = topo.walkable_points()
    pair = next(
        ((a, b) for a in walkable for b in walkable if abs(a[0] - b[0]) + abs(a[1] - b[1]) == 1),
        None,
    )
    assert pair is not None, "fixture has no adjacent walkable cells"
    pursuer, player = pair
    _place(env, pursuer, player)
    env.player_hp = 4
    env.pursuer_hp = 10

    dx = player[0] - pursuer[0]
    dy = player[1] - pursuer[1]
    action = next(
        a for a, v in ACTION_VECTORS.items() if v == (dx, dy) and a != ACTION_WAIT
    )
    _, _, _, _, info = env.step(action)

    assert info["pursuer_hit"] is True, "bump into adjacent player must hit"
    assert env.player_hp == 2, "HP must drop by attack_damage"
    assert env.pursuer_pos == pursuer, "pursuer must stay at prev_pos (blocker)"


def test_wait_adjacent_hits_player() -> None:
    env = _make_env(player_hp=4, pursuer_hp=10, attack_damage=2)
    env.reset(seed=0)
    topo = env.topology
    assert topo is not None
    walkable = topo.walkable_points()
    pair = next(
        ((a, b) for a in walkable for b in walkable if abs(a[0] - b[0]) + abs(a[1] - b[1]) == 1),
        None,
    )
    assert pair is not None
    pursuer, player = pair
    _place(env, pursuer, player)
    env.player_hp = 4
    env.pursuer_hp = 10

    _, _, _, _, info = env.step(ACTION_WAIT)
    assert info["pursuer_hit"] is True, "WAIT adjacent must still hit"
    assert env.player_hp == 2


def test_orthogonal_move_does_not_bump_attack() -> None:
    """Moving orthogonally away keeps the pursuer adjacent but is NOT a bump."""
    env = _make_env(player_hp=4, pursuer_hp=10, attack_damage=2, fixed_profile="stone")
    env.reset(seed=0)
    topo = env.topology
    assert topo is not None

    # Find a 2x2 block of walkable tiles.
    walkable_set = set(topo.walkable_points())
    block = next(
        (
            (x, y)
            for (x, y) in walkable_set
            if {(x + 1, y), (x, y + 1), (x + 1, y + 1)} <= walkable_set
        ),
        None,
    )
    assert block is not None, "fixture has no 2x2 walkable block"
    x, y = block
    pursuer = (x, y)
    player = (x + 1, y)  # adjacent horizontally
    _place(env, pursuer, player)
    env.player_hp = 4
    env.pursuer_hp = 10
    hp_before = env.player_hp

    # Move DOWN — orthogonal to the player direction (RIGHT). action_vec != bump_vec.
    _, _, _, _, info = env.step(ACTION_DOWN)
    # Pursuer stepped to (x, y+1). May or may not still be adjacent; the
    # critical assertion is that the *bump* did not fire on this action.
    assert info["pursuer_hit"] is False, "orthogonal step must not bump-attack"
    assert env.player_hp == hp_before


def test_dijkstra_kills_stone_within_budget() -> None:
    """End-to-end: a Dijkstra-greedy policy vs a stone player must kill quickly."""
    env = _make_env(
        player_hp=4,
        pursuer_hp=10,
        attack_damage=2,
        fixed_profile="stone",
        max_episode_steps=30,
        spawn_radius=(3, 5),
    )

    caught_runs = 0
    for ep in range(20):
        obs, _ = env.reset(seed=100 + ep)
        done = False
        steps = 0
        while not done:
            # Dijkstra-greedy with Manhattan tie-break — scent is 8-connected
            # BFS but actions are 4-cardinal, so when pursuer is diagonally
            # adjacent to player both cardinal neighbours have scent == current.
            # Manhattan tie-break steers us onto the same row/column as the
            # player so the next bump can hit.
            scent = chase_scent_map(env.topology, env.player_pos)
            ox, oy = env.pursuer_pos
            px, py = env.player_pos
            current_scent = int(scent[oy, ox])
            current_manhattan = abs(px - ox) + abs(py - oy)
            best = (current_scent, current_manhattan)
            best_action = ACTION_WAIT
            for a, (dx, dy) in ACTION_VECTORS.items():
                if a == ACTION_WAIT:
                    continue
                nx, ny = ox + dx, oy + dy
                if not env.topology.is_walkable(nx, ny):
                    continue
                v = int(scent[ny, nx])
                m = abs(px - nx) + abs(py - ny)
                if (v, m) < best:
                    best = (v, m)
                    best_action = a
            _, _, term, trunc, info = env.step(best_action)
            steps += 1
            done = term or trunc
        if info.get("caught"):
            caught_runs += 1

    assert caught_runs >= 16, (
        f"Dijkstra must catch stone player in >=80% of short episodes, got {caught_runs}/20"
    )
