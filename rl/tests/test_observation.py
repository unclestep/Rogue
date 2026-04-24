"""
Python-side invariants on the observation vector. These guard against the
most common ways the Python env and the Go source can drift apart:

  - total length must be 1101 (9 channels × 11×11 + 12 scalars),
  - channel block is contiguous CHW,
  - scalar slice is 12 floats,
  - walls outside the map are encoded as 0 in CH_WALKABLE,
  - self-trail decays monotonically with age,
  - cone mask lights the origin cell.

These are NOT byte-exact Go↔Python parity — that requires a Go helper to
dump reference observations (deferred to a later PR). But they do catch
silent stride/ordering mistakes introduced when either side is edited in
isolation.
"""

from __future__ import annotations

import math
import os
from pathlib import Path

import numpy as np
import pytest

from rl.pursuer_env import (
    CH_AGE_LAST_SEEN,
    CH_COVERT_PATH,
    CH_PLAYER_CONE,
    CH_PLAYER_TRAIL,
    CH_SCENT,
    CH_SELF_TRAIL,
    CH_WALKABLE,
    COVERT_CONE_PENALTY,
    COVERT_NORM_FACTOR,
    OBSERVATION_CROP_SIZE,
    OBSERVATION_GRID_FLOATS,
    OBSERVATION_HALF_CROP,
    OBSERVATION_SCALARS,
    OBSERVATION_SIZE,
    PLAYER_TRAIL_DECAY,
    PursuerEnv,
    PursuerMemory,
    Topology,
    build_observation,
    chase_scent_map,
    compute_intercept_stats,
    covert_path_map,
    flashlight_mask,
)
from rl.scripted_player import scripted_player_policy


FIXTURES = Path(__file__).resolve().parents[1] / "fixtures" / "topologies"


@pytest.fixture(scope="module")
def topology() -> Topology:
    paths = sorted(FIXTURES.glob("*.json"))
    assert paths, f"no fixtures in {FIXTURES}; run `make dump-topologies` first"
    return Topology.from_json(str(paths[0]))


def test_observation_length_and_dtype(topology):
    mem = PursuerMemory()
    obs = build_observation(
        topology=topology,
        pursuer_pos=(topology.rooms[0]["center"]["x"], topology.rooms[0]["center"]["y"]),
        pursuer_hp_frac=1.0,
        pursuer_stamina_frac=1.0,
        memory=mem,
        player_pos=None,
        player_angle_rad=0.0,
        cone_mask=None,
    )
    assert obs.shape == (OBSERVATION_SIZE,)
    assert obs.dtype == np.float32
    assert OBSERVATION_GRID_FLOATS + OBSERVATION_SCALARS == OBSERVATION_SIZE


def test_walkable_channel_matches_topology(topology):
    center = topology.rooms[0]["center"]
    pursuer = (center["x"], center["y"])
    obs = build_observation(
        topology=topology,
        pursuer_pos=pursuer,
        pursuer_hp_frac=1.0,
        pursuer_stamina_frac=1.0,
        memory=PursuerMemory(),
        player_pos=None,
        player_angle_rad=0.0,
        cone_mask=None,
    )
    # Reshape the CHW block and pick channel 0.
    grid = obs[:OBSERVATION_GRID_FLOATS].reshape(
        -1, OBSERVATION_CROP_SIZE, OBSERVATION_CROP_SIZE
    )
    walkable = grid[CH_WALKABLE]
    for dy in range(-OBSERVATION_HALF_CROP, OBSERVATION_HALF_CROP + 1):
        for dx in range(-OBSERVATION_HALF_CROP, OBSERVATION_HALF_CROP + 1):
            px, py = pursuer[0] + dx, pursuer[1] + dy
            expected = 1.0 if topology.is_walkable(px, py) else 0.0
            got = walkable[dy + OBSERVATION_HALF_CROP, dx + OBSERVATION_HALF_CROP]
            assert got == expected, f"walkable mismatch at (dx,dy)=({dx},{dy})"


def test_self_trail_decays_with_age(topology):
    center = topology.rooms[0]["center"]
    pursuer = (center["x"], center["y"])
    mem = PursuerMemory()
    # Push three cells in a row — newest has highest weight.
    mem.push_trail((pursuer[0] - 2, pursuer[1]))
    mem.push_trail((pursuer[0] - 1, pursuer[1]))
    mem.push_trail(pursuer)

    obs = build_observation(
        topology=topology,
        pursuer_pos=pursuer,
        pursuer_hp_frac=1.0,
        pursuer_stamina_frac=1.0,
        memory=mem,
        player_pos=None,
        player_angle_rad=0.0,
        cone_mask=None,
    )
    grid = obs[:OBSERVATION_GRID_FLOATS].reshape(-1, OBSERVATION_CROP_SIZE, OBSERVATION_CROP_SIZE)
    trail = grid[CH_SELF_TRAIL]
    center_w = trail[OBSERVATION_HALF_CROP, OBSERVATION_HALF_CROP]
    older_w = trail[OBSERVATION_HALF_CROP, OBSERVATION_HALF_CROP - 1]
    oldest_w = trail[OBSERVATION_HALF_CROP, OBSERVATION_HALF_CROP - 2]
    assert center_w > older_w > oldest_w >= 0.0


def test_cone_channel_lights_origin(topology):
    # Place pursuer at player so player's own cell is in the cone.
    rx = topology.rooms[0]["center"]["x"]
    ry = topology.rooms[0]["center"]["y"]
    player = (rx, ry)
    cone = flashlight_mask(topology, player, angle_rad=0.0)
    assert cone[ry, rx], "origin cell must be lit"

    # At the pursuer==player position the ChPlayerCone channel must be set.
    obs = build_observation(
        topology=topology,
        pursuer_pos=player,
        pursuer_hp_frac=1.0,
        pursuer_stamina_frac=1.0,
        memory=PursuerMemory(),
        player_pos=player,
        player_angle_rad=0.0,
        cone_mask=cone,
    )
    grid = obs[:OBSERVATION_GRID_FLOATS].reshape(-1, OBSERVATION_CROP_SIZE, OBSERVATION_CROP_SIZE)
    assert grid[CH_PLAYER_CONE, OBSERVATION_HALF_CROP, OBSERVATION_HALF_CROP] == 1.0


def test_observation_all_grid_values_in_unit_range(topology):
    """Every cell in the spatial channel block must be in [0, 1]."""
    center = topology.rooms[0]["center"]
    pursuer = (center["x"], center["y"])
    player = (pursuer[0] - 2, pursuer[1])
    obs = build_observation(
        topology=topology,
        pursuer_pos=pursuer,
        pursuer_hp_frac=1.0,
        pursuer_stamina_frac=1.0,
        memory=PursuerMemory(),
        player_pos=player,
        player_angle_rad=0.0,
        cone_mask=None,
        scent_map=chase_scent_map(topology, player),
    )
    grid = obs[:OBSERVATION_GRID_FLOATS]
    assert np.all(grid >= 0.0) and np.all(grid <= 1.0), (
        f"grid out of [0,1]: min={grid.min():.4f} max={grid.max():.4f}"
    )


def test_scent_channel_hot_at_player_position(topology):
    """CH_SCENT must be 1.0 at the player's own cell (BFS distance = 0)."""
    center = topology.rooms[0]["center"]
    pursuer = (center["x"], center["y"])
    player = (pursuer[0] - 2, pursuer[1])  # within 11×11 crop
    scent = chase_scent_map(topology, player)

    obs = build_observation(
        topology=topology,
        pursuer_pos=pursuer,
        pursuer_hp_frac=1.0,
        pursuer_stamina_frac=1.0,
        memory=PursuerMemory(),
        player_pos=player,
        player_angle_rad=0.0,
        cone_mask=None,
        scent_map=scent,
    )
    grid = obs[:OBSERVATION_GRID_FLOATS].reshape(-1, OBSERVATION_CROP_SIZE, OBSERVATION_CROP_SIZE)
    # player dx=-2, dy=0 → crop (3, 5)
    cx, cy = OBSERVATION_HALF_CROP - 2, OBSERVATION_HALF_CROP
    val = grid[CH_SCENT, cy, cx]
    assert abs(val - 1.0) < 1e-5, f"CH_SCENT at player cell expected 1.0, got {val}"


def test_scent_channel_zero_when_no_scent_map(topology):
    """CH_SCENT must stay 0 when scent_map=None (the common offline-render case)."""
    center = topology.rooms[0]["center"]
    pursuer = (center["x"], center["y"])
    obs = build_observation(
        topology=topology,
        pursuer_pos=pursuer,
        pursuer_hp_frac=1.0,
        pursuer_stamina_frac=1.0,
        memory=PursuerMemory(),
        player_pos=None,
        player_angle_rad=0.0,
        cone_mask=None,
        scent_map=None,
    )
    grid = obs[:OBSERVATION_GRID_FLOATS].reshape(-1, OBSERVATION_CROP_SIZE, OBSERVATION_CROP_SIZE)
    assert np.all(grid[CH_SCENT] == 0.0), "CH_SCENT must be all-zero with no scent_map"


def test_age_last_seen_peaks_at_last_seen_cell(topology):
    """CH_AGE_LAST_SEEN must be 1.0 at the last-seen cell with turns_since_los=0."""
    center = topology.rooms[0]["center"]
    pursuer = (center["x"], center["y"])
    last_seen = (pursuer[0] - 1, pursuer[1])  # one cell left of pursuer

    mem = PursuerMemory()
    mem.last_seen = last_seen
    mem.turns_since_los = 0  # amplitude = 1.0

    obs = build_observation(
        topology=topology,
        pursuer_pos=pursuer,
        pursuer_hp_frac=1.0,
        pursuer_stamina_frac=1.0,
        memory=mem,
        player_pos=None,
        player_angle_rad=0.0,
        cone_mask=None,
    )
    grid = obs[:OBSERVATION_GRID_FLOATS].reshape(-1, OBSERVATION_CROP_SIZE, OBSERVATION_CROP_SIZE)
    # last_seen dx=-1, dy=0 → crop (4, 5)
    peak = grid[CH_AGE_LAST_SEEN, OBSERVATION_HALF_CROP, OBSERVATION_HALF_CROP - 1]
    at_pursuer = grid[CH_AGE_LAST_SEEN, OBSERVATION_HALF_CROP, OBSERVATION_HALF_CROP]
    assert abs(peak - 1.0) < 1e-5, f"peak amplitude must be 1.0, got {peak:.6f}"
    assert peak > at_pursuer, (
        f"CH_AGE_LAST_SEEN must peak at last-seen ({peak:.4f}) > at pursuer ({at_pursuer:.4f})"
    )


def test_age_last_seen_zero_at_memory_horizon(topology):
    """CH_AGE_LAST_SEEN must be all-zero when turns_since_los == PURSUER_MEMORY_HORIZON."""
    from rl.pursuer_env import PURSUER_MEMORY_HORIZON

    center = topology.rooms[0]["center"]
    pursuer = (center["x"], center["y"])

    mem = PursuerMemory()
    mem.last_seen = (pursuer[0] - 1, pursuer[1])
    mem.turns_since_los = PURSUER_MEMORY_HORIZON  # amplitude = 0

    obs = build_observation(
        topology=topology,
        pursuer_pos=pursuer,
        pursuer_hp_frac=1.0,
        pursuer_stamina_frac=1.0,
        memory=mem,
        player_pos=None,
        player_angle_rad=0.0,
        cone_mask=None,
    )
    grid = obs[:OBSERVATION_GRID_FLOATS].reshape(-1, OBSERVATION_CROP_SIZE, OBSERVATION_CROP_SIZE)
    assert np.all(grid[CH_AGE_LAST_SEEN] == 0.0), (
        "CH_AGE_LAST_SEEN must be all-zero at memory horizon"
    )


def test_env_pursuer_can_move_toward_player():
    """Pursuer must actually change position in at least one of several steps."""
    env = PursuerEnv(str(FIXTURES), scripted_player_policy, max_episode_steps=30, seed=3)
    env.reset(seed=3)
    start_pos = env.pursuer_pos
    moved = False
    for _ in range(30):
        _, _, term, trunc, _ = env.step(int(env.action_space.sample()))
        if env.pursuer_pos != start_pos:
            moved = True
            break
        if term or trunc:
            break
    assert moved, "Pursuer never moved from starting position in 30 steps"


def test_env_episode_terminates_or_truncates():
    """An episode must end (term or trunc) within max_episode_steps steps."""
    env = PursuerEnv(str(FIXTURES), scripted_player_policy, max_episode_steps=50, seed=5)
    env.reset(seed=5)
    ended = False
    for _ in range(50):
        _, _, term, trunc, _ = env.step(int(env.action_space.sample()))
        if term or trunc:
            ended = True
            break
    assert ended, "Episode did not terminate or truncate within 50 steps"


def test_observation_size_matches_spec():
    # Guards against drift: 11 channels × 121 + 16 scalars = 1347.
    assert OBSERVATION_SIZE == 11 * OBSERVATION_CROP_SIZE * OBSERVATION_CROP_SIZE + 16
    assert OBSERVATION_GRID_FLOATS + OBSERVATION_SCALARS == OBSERVATION_SIZE
    assert OBSERVATION_SCALARS == 16


def test_covert_path_weighted_bfs_penalizes_cone():
    # Construct a tiny-enough topology from the fixture, then assert that
    # covert BFS from one end prefers cells outside the cone mask.
    paths = sorted(FIXTURES.glob("*.json"))
    topology = Topology.from_json(str(paths[0]))

    walkable = set(topology.walkable_points())
    pursuer = sorted(walkable)[0]
    neighbour = (pursuer[0] + 1, pursuer[1])
    if neighbour not in walkable:
        pytest.skip("fixture has no adjacent walkable cell next to origin")

    cone_free = covert_path_map(topology, pursuer, None)
    cone = np.zeros((topology.height, topology.width), dtype=bool)
    cone[neighbour[1], neighbour[0]] = True
    cone_penalty = covert_path_map(topology, pursuer, cone)

    # Entering a cone cell must cost COVERT_CONE_PENALTY instead of 1.
    assert cone_free[neighbour[1], neighbour[0]] == 1
    assert cone_penalty[neighbour[1], neighbour[0]] == COVERT_CONE_PENALTY


def test_covert_path_channel_writes_normalized_distance(topology):
    # ChCovertPath must hold min(d / COVERT_NORM_FACTOR, 1.0).
    center = topology.rooms[0]["center"]
    pursuer = (center["x"], center["y"])

    obs = build_observation(
        topology=topology,
        pursuer_pos=pursuer,
        pursuer_hp_frac=1.0,
        pursuer_stamina_frac=1.0,
        memory=PursuerMemory(),
        player_pos=None,
        player_angle_rad=0.0,
        cone_mask=None,
        covert_map=covert_path_map(topology, pursuer, None),
    )
    grid = obs[:OBSERVATION_GRID_FLOATS].reshape(
        -1, OBSERVATION_CROP_SIZE, OBSERVATION_CROP_SIZE
    )
    # Origin distance = 0 → normalized 0.
    assert (
        grid[CH_COVERT_PATH, OBSERVATION_HALF_CROP, OBSERVATION_HALF_CROP] == 0.0
    )


def test_player_trail_channel_decays_over_time(topology):
    # Newest position must read 1.0, one-step-older decays by PLAYER_TRAIL_DECAY.
    center = topology.rooms[0]["center"]
    pursuer = (center["x"], center["y"])
    # Three cells in a row, newest last.
    trail = [
        (pursuer[0] - 2, pursuer[1]),
        (pursuer[0] - 1, pursuer[1]),
        pursuer,
    ]
    obs = build_observation(
        topology=topology,
        pursuer_pos=pursuer,
        pursuer_hp_frac=1.0,
        pursuer_stamina_frac=1.0,
        memory=PursuerMemory(),
        player_pos=None,
        player_angle_rad=0.0,
        cone_mask=None,
        player_trail=trail,
    )
    grid = obs[:OBSERVATION_GRID_FLOATS].reshape(
        -1, OBSERVATION_CROP_SIZE, OBSERVATION_CROP_SIZE
    )
    ch = grid[CH_PLAYER_TRAIL]
    # Newest (dx=0, dy=0) — intensity 1.0
    assert ch[OBSERVATION_HALF_CROP, OBSERVATION_HALF_CROP] == pytest.approx(1.0)
    # Previous (dx=-1) — intensity PLAYER_TRAIL_DECAY ^ 1
    assert ch[OBSERVATION_HALF_CROP, OBSERVATION_HALF_CROP - 1] == pytest.approx(
        PLAYER_TRAIL_DECAY, rel=1e-5
    )
    # Oldest (dx=-2) — intensity PLAYER_TRAIL_DECAY ^ 2
    assert ch[OBSERVATION_HALF_CROP, OBSERVATION_HALF_CROP - 2] == pytest.approx(
        PLAYER_TRAIL_DECAY**2, rel=1e-5
    )


def test_intercept_scalar_signed(topology):
    # Crafted small scenario: stats should be within clamps; fall back to
    # zeros when no stats passed.
    pursuer = (topology.exit_point[0], topology.exit_point[1])
    # Pick a reachable player cell away from exit.
    exit_scent = chase_scent_map(topology, topology.exit_point)
    INF = 10**9
    candidate = None
    for (x, y) in topology.walkable_points():
        if 5 <= int(exit_scent[y, x]) < INF:
            candidate = (x, y)
            break
    assert candidate is not None
    player = candidate
    chase = chase_scent_map(topology, player)
    stats = compute_intercept_stats(exit_scent, chase, pursuer, player)
    assert 0.0 <= stats[0] <= 1.0
    assert 0.0 <= stats[1] <= 1.0
    assert -1.0 <= stats[2] <= 1.0


def test_is_corridor_scalar_reflects_tile_type(topology):
    # Find a corridor tile; is_corridor must come through as 1.0.
    from rl.pursuer_env import TILE_CORRIDOR

    corridor = None
    for y in range(topology.height):
        for x in range(topology.width):
            if topology.tiles[y, x] == TILE_CORRIDOR:
                corridor = (x, y)
                break
        if corridor:
            break
    if corridor is None:
        pytest.skip("fixture has no corridor tile")

    pursuer_pos = (topology.rooms[0]["center"]["x"], topology.rooms[0]["center"]["y"])
    obs = build_observation(
        topology=topology,
        pursuer_pos=pursuer_pos,
        pursuer_hp_frac=1.0,
        pursuer_stamina_frac=1.0,
        memory=PursuerMemory(),
        player_pos=corridor,
        player_angle_rad=0.0,
        cone_mask=None,
        is_corridor=1.0,
    )
    assert obs[OBSERVATION_GRID_FLOATS + 15] == pytest.approx(1.0)

    obs_room = build_observation(
        topology=topology,
        pursuer_pos=pursuer_pos,
        pursuer_hp_frac=1.0,
        pursuer_stamina_frac=1.0,
        memory=PursuerMemory(),
        player_pos=pursuer_pos,
        player_angle_rad=0.0,
        cone_mask=None,
        is_corridor=0.0,
    )
    assert obs_room[OBSERVATION_GRID_FLOATS + 15] == pytest.approx(0.0)
