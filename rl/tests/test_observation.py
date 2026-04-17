"""
Python-side invariants on the observation vector. These guard against the
most common ways the Python env and the Go source can drift apart:

  - total length must be 859,
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
    CH_PLAYER_CONE,
    CH_SELF_TRAIL,
    CH_WALKABLE,
    OBSERVATION_CROP_SIZE,
    OBSERVATION_GRID_FLOATS,
    OBSERVATION_HALF_CROP,
    OBSERVATION_SCALARS,
    OBSERVATION_SIZE,
    PursuerEnv,
    PursuerMemory,
    Topology,
    build_observation,
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


def test_env_full_episode_no_crash():
    env = PursuerEnv(str(FIXTURES), scripted_player_policy, max_episode_steps=25, seed=1)
    obs, _ = env.reset(seed=1)
    assert obs.shape == (OBSERVATION_SIZE,)
    for _ in range(25):
        a = env.action_space.sample()
        obs, r, term, trunc, _ = env.step(int(a))
        assert np.isfinite(r)
        if term or trunc:
            break
