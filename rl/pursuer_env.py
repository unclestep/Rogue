"""
Pursuer training environment (Gymnasium).

This module is the Python mirror of Go's
internal/domain/service/rl_observation.go, logic_monster_pursuer.go and
sys_raycaster.go. Observation layout, action enum, raycaster algorithm and
memory decay must stay byte-for-byte identical to the Go side — the ONNX
model trained here is loaded by ONNXPolicy in production and will produce
garbage if the feature vector drifts.

Dungeon topologies are **not** generated here. They come from JSON files
produced by `cmd/dump-topology` (see PR 3.5), so the BSP layout used during
training is exactly what the game ships.

Reward shaping follows the plan in
/home/codespace/.claude/plans/binary-hugging-turing.md (PR 4 section).
"""

from __future__ import annotations

import dataclasses
import glob
import json
import math
import os
import random
from typing import Callable, Optional

import gymnasium as gym
import numpy as np
from gymnasium import spaces

# ---------------------------------------------------------------------------
# Constants mirrored from Go.
# ---------------------------------------------------------------------------

# Observation geometry — see rl_observation.go:22-44.
OBSERVATION_CROP_SIZE = 11
OBSERVATION_HALF_CROP = OBSERVATION_CROP_SIZE // 2
OBSERVATION_CHANNELS = 7
OBSERVATION_SCALARS = 12
OBSERVATION_GRID_FLOATS = (
    OBSERVATION_CHANNELS * OBSERVATION_CROP_SIZE * OBSERVATION_CROP_SIZE
)
OBSERVATION_SIZE = OBSERVATION_GRID_FLOATS + OBSERVATION_SCALARS  # 859

CH_WALKABLE = 0
CH_CLOSED_DOOR = 1
CH_OTHER_MONSTER = 2
CH_SELF_TRAIL = 3
CH_PLAYER_VIS_NOW = 4
CH_PLAYER_MEMORY = 5
CH_PLAYER_CONE = 6

PURSUER_TRAIL_CAPACITY = 20
PURSUER_MEMORY_HORIZON = 20

# Action enum — see rl_policy.go:10-18.
ACTION_UP = 0
ACTION_RIGHT = 1
ACTION_DOWN = 2
ACTION_LEFT = 3
ACTION_WAIT = 4
ACTION_COUNT = 5

ACTION_VECTORS = {
    ACTION_UP: (0, -1),
    ACTION_RIGHT: (1, 0),
    ACTION_DOWN: (0, 1),
    ACTION_LEFT: (-1, 0),
    ACTION_WAIT: (0, 0),
}

# Tile ordinals — must match model.TileType iota in map.go.
TILE_EMPTY = 0
TILE_WALL = 1
TILE_FLOOR = 2
TILE_OPEN_DOOR = 3
TILE_CLOSED_DOOR = 4
TILE_CORRIDOR = 5
TILE_EXIT = 6

FLASHLIGHT_HALF_FOV_DEG = 45.0
FLASHLIGHT_RANGE = 15.0


# ---------------------------------------------------------------------------
# Domain structs.
# ---------------------------------------------------------------------------


@dataclasses.dataclass
class Topology:
    """Deserialised output of cmd/dump-topology."""

    width: int
    height: int
    tiles: np.ndarray  # shape (H, W), int8
    rooms: list[dict]
    entrance_room: int
    exit_room: int
    exit_point: tuple[int, int]

    @classmethod
    def from_json(cls, path: str) -> "Topology":
        with open(path, "r", encoding="utf-8") as fh:
            raw = json.load(fh)
        tiles = np.asarray(raw["tiles"], dtype=np.int8)
        return cls(
            width=raw["width"],
            height=raw["height"],
            tiles=tiles,
            rooms=raw["rooms"],
            entrance_room=raw["entrance_room"],
            exit_room=raw["exit_room"],
            exit_point=(raw["exit_point"]["x"], raw["exit_point"]["y"]),
        )

    def in_bounds(self, x: int, y: int) -> bool:
        return 0 <= x < self.width and 0 <= y < self.height

    def is_walkable(self, x: int, y: int) -> bool:
        if not self.in_bounds(x, y):
            return False
        t = self.tiles[y, x]
        return t != TILE_EMPTY and t != TILE_WALL and t != TILE_CLOSED_DOOR

    def is_closed_door(self, x: int, y: int) -> bool:
        return self.in_bounds(x, y) and self.tiles[y, x] == TILE_CLOSED_DOOR

    def walkable_points(self) -> list[tuple[int, int]]:
        out: list[tuple[int, int]] = []
        ys, xs = np.where(
            (self.tiles != TILE_EMPTY)
            & (self.tiles != TILE_WALL)
            & (self.tiles != TILE_CLOSED_DOOR)
        )
        for x, y in zip(xs.tolist(), ys.tolist()):
            out.append((x, y))
        return out

    def room_walkable_points(self, room: dict) -> list[tuple[int, int]]:
        out: list[tuple[int, int]] = []
        for y in range(room["y"], room["y"] + room["h"]):
            for x in range(room["x"], room["x"] + room["w"]):
                if self.is_walkable(x, y):
                    out.append((x, y))
        return out


@dataclasses.dataclass
class PursuerMemory:
    """Mirror of service.PursuerMemory (Go). Invalid LastSeen is (-1, -1)."""

    last_seen: tuple[int, int] = (-1, -1)
    turns_since_los: int = PURSUER_MEMORY_HORIZON
    trail: list[tuple[int, int]] = dataclasses.field(default_factory=list)

    def push_trail(self, p: tuple[int, int]) -> None:
        self.trail.append(p)
        if len(self.trail) > PURSUER_TRAIL_CAPACITY:
            self.trail = self.trail[-PURSUER_TRAIL_CAPACITY:]


# ---------------------------------------------------------------------------
# Raycaster port.
# ---------------------------------------------------------------------------


def _blocks_light(tile: int) -> bool:
    # Mirrors sys_raycaster.go blocksLight.
    return (
        tile == TILE_WALL
        or tile == TILE_EMPTY
        or tile == TILE_CLOSED_DOOR
        or tile == TILE_OPEN_DOOR
    )


def flashlight_mask(
    topology: Topology,
    origin: tuple[int, int],
    angle_rad: float,
    half_fov_deg: float = FLASHLIGHT_HALF_FOV_DEG,
    max_range: float = FLASHLIGHT_RANGE,
) -> np.ndarray:
    """
    Port of service.Raycaster.Flashlight. Returns a bool array of shape
    (H, W) marking cells currently inside the player's cone. The algorithm
    is deliberately identical to Go's DDA version — changing it breaks the
    ChPlayerCone channel, which the trained policy relies on.
    """
    mask = np.zeros((topology.height, topology.width), dtype=bool)

    pos_x = origin[0] + 0.5
    pos_y = origin[1] + 0.5
    dir_x = math.cos(angle_rad)
    dir_y = math.sin(angle_rad)

    # Go's math.Tan(pi/4) rounds to 1.0 exactly; Python's libm math.tan
    # returns 0.9999999999999999 (one ULP below). That 1-ULP gap inverts the
    # DDA tie-break on perfectly-diagonal cone edges — four boundary cells
    # flip between the two implementations. Special-case the 45° production
    # value to keep observation parity bit-exact.
    if half_fov_deg == 45.0:
        plane_scale = 1.0
    else:
        plane_scale = math.tan(math.radians(half_fov_deg))
    plane_x = -math.sin(angle_rad) * plane_scale
    plane_y = math.cos(angle_rad) * plane_scale

    # Origin cell is always lit.
    ox, oy = int(pos_x), int(pos_y)
    if topology.in_bounds(ox, oy):
        mask[oy, ox] = True

    plane_len = math.hypot(plane_x, plane_y)
    if plane_len < 1e-9:
        return mask

    step = 0.5 / (plane_len * max_range)
    cam_x = -1.0
    while cam_x <= 1.0:
        _cast_ray(mask, topology, pos_x, pos_y, dir_x, dir_y, plane_x, plane_y, cam_x, max_range)
        cam_x += step
    _cast_ray(mask, topology, pos_x, pos_y, dir_x, dir_y, plane_x, plane_y, 1.0, max_range)

    return mask


def _cast_ray(
    mask: np.ndarray,
    topology: Topology,
    pos_x: float,
    pos_y: float,
    dir_x: float,
    dir_y: float,
    plane_x: float,
    plane_y: float,
    camera_x: float,
    max_range: float,
) -> None:
    ray_dir_x = dir_x + plane_x * camera_x
    ray_dir_y = dir_y + plane_y * camera_x

    map_x = int(pos_x)
    map_y = int(pos_y)

    delta_x = math.inf if ray_dir_x == 0 else abs(1.0 / ray_dir_x)
    delta_y = math.inf if ray_dir_y == 0 else abs(1.0 / ray_dir_y)

    if ray_dir_x < 0:
        step_x = -1
        side_x = (pos_x - map_x) * delta_x
    else:
        step_x = 1
        side_x = (map_x + 1.0 - pos_x) * delta_x
    if ray_dir_y < 0:
        step_y = -1
        side_y = (pos_y - map_y) * delta_y
    else:
        step_y = 1
        side_y = (map_y + 1.0 - pos_y) * delta_y

    while True:
        if side_x < side_y:
            side_x += delta_x
            map_x += step_x
            side_flag = 0
        else:
            side_y += delta_y
            map_y += step_y
            side_flag = 1

        if side_flag == 0:
            perp = side_x - delta_x
        else:
            perp = side_y - delta_y
        if perp > max_range:
            break
        if not topology.in_bounds(map_x, map_y):
            break

        tile = int(topology.tiles[map_y, map_x])
        mask[map_y, map_x] = True
        if _blocks_light(tile):
            break


def has_los(topology: Topology, a: tuple[int, int], b: tuple[int, int]) -> bool:
    """Port of hasLOS in rl_observation.go — Bresenham with open-door passable."""
    if a == b:
        return True
    x0, y0 = a
    x1, y1 = b
    dx = abs(x1 - x0)
    dy = abs(y1 - y0)
    sx = 1 if x0 < x1 else -1
    sy = 1 if y0 < y1 else -1
    err = dx - dy
    x, y = x0, y0
    while True:
        if (x, y) == (x1, y1):
            return True
        e2 = 2 * err
        if e2 > -dy:
            err -= dy
            x += sx
        if e2 < dx:
            err += dx
            y += sy
        if (x, y) == (x1, y1):
            return True
        if not topology.in_bounds(x, y):
            return False
        t = int(topology.tiles[y, x])
        if t == TILE_WALL or t == TILE_EMPTY or t == TILE_CLOSED_DOOR:
            return False


# ---------------------------------------------------------------------------
# Observation builder.
# ---------------------------------------------------------------------------


def _set_channel(obs: np.ndarray, channel: int, x: int, y: int, v: float) -> None:
    if x < 0 or x >= OBSERVATION_CROP_SIZE or y < 0 or y >= OBSERVATION_CROP_SIZE:
        return
    obs[
        channel * OBSERVATION_CROP_SIZE * OBSERVATION_CROP_SIZE
        + y * OBSERVATION_CROP_SIZE
        + x
    ] = v


def build_observation(
    topology: Topology,
    pursuer_pos: tuple[int, int],
    pursuer_hp_frac: float,
    pursuer_stamina_frac: float,
    memory: PursuerMemory,
    player_pos: Optional[tuple[int, int]],
    player_angle_rad: float,
    cone_mask: Optional[np.ndarray],
    other_pursuer_pos: Optional[tuple[int, int]] = None,
) -> np.ndarray:
    """
    Mirror of BuildObservation in rl_observation.go. See that file for the
    authoritative spec — any divergence here is a bug.
    """
    obs = np.zeros(OBSERVATION_SIZE, dtype=np.float32)
    ox, oy = pursuer_pos

    have_target = player_pos is not None
    sees_player = False
    if have_target:
        sees_player = has_los(topology, pursuer_pos, player_pos)

    for dy in range(-OBSERVATION_HALF_CROP, OBSERVATION_HALF_CROP + 1):
        for dx in range(-OBSERVATION_HALF_CROP, OBSERVATION_HALF_CROP + 1):
            px, py = ox + dx, oy + dy
            cx, cy = dx + OBSERVATION_HALF_CROP, dy + OBSERVATION_HALF_CROP

            if not topology.in_bounds(px, py):
                continue
            if topology.is_walkable(px, py):
                _set_channel(obs, CH_WALKABLE, cx, cy, 1.0)
            if topology.is_closed_door(px, py):
                _set_channel(obs, CH_CLOSED_DOOR, cx, cy, 1.0)
            if other_pursuer_pos is not None and other_pursuer_pos == (px, py):
                _set_channel(obs, CH_OTHER_MONSTER, cx, cy, 1.0)
            if have_target and (px, py) == player_pos and sees_player:
                _set_channel(obs, CH_PLAYER_VIS_NOW, cx, cy, 1.0)
            if cone_mask is not None and cone_mask[py, px]:
                _set_channel(obs, CH_PLAYER_CONE, cx, cy, 1.0)

    # Self-trail — newest has weight 1, oldest has weight 1/cap.
    trail = memory.trail
    for i, tp in enumerate(trail):
        age = len(trail) - 1 - i
        weight = 1.0 - age / PURSUER_TRAIL_CAPACITY
        if weight <= 0:
            continue
        dx = tp[0] - ox
        dy = tp[1] - oy
        if abs(dx) > OBSERVATION_HALF_CROP or abs(dy) > OBSERVATION_HALF_CROP:
            continue
        _set_channel(obs, CH_SELF_TRAIL, dx + OBSERVATION_HALF_CROP, dy + OBSERVATION_HALF_CROP, weight)

    # Player memory spike.
    if memory.last_seen != (-1, -1):
        dx = memory.last_seen[0] - ox
        dy = memory.last_seen[1] - oy
        if abs(dx) <= OBSERVATION_HALF_CROP and abs(dy) <= OBSERVATION_HALF_CROP:
            weight = 1.0 - memory.turns_since_los / PURSUER_MEMORY_HORIZON
            if weight > 0:
                _set_channel(
                    obs,
                    CH_PLAYER_MEMORY,
                    dx + OBSERVATION_HALF_CROP,
                    dy + OBSERVATION_HALF_CROP,
                    weight,
                )

    # Scalars — see writeScalars in rl_observation.go.
    s = obs[OBSERVATION_GRID_FLOATS:]
    s[0] = float(np.clip(pursuer_hp_frac, 0.0, 1.0))
    s[1] = float(np.clip(pursuer_stamina_frac, 0.0, 1.0))
    s[2] = min(memory.turns_since_los / PURSUER_MEMORY_HORIZON, 1.0)

    w, h = float(topology.width), float(topology.height)
    if memory.last_seen != (-1, -1):
        dx = memory.last_seen[0] - ox
        dy = memory.last_seen[1] - oy
        dist = math.hypot(dx, dy)
        s[3] = dist / math.hypot(w, h)
        s[4] = dx / w
        s[5] = dy / h

    if other_pursuer_pos is not None:
        s[6] = (other_pursuer_pos[0] - ox) / w
        s[7] = (other_pursuer_pos[1] - oy) / h

    if have_target:
        cone_dx = float(player_pos[0] - ox)
        cone_dy = float(player_pos[1] - oy)
        dist = math.hypot(cone_dx, cone_dy)
        in_cone = False
        if dist <= FLASHLIGHT_RANGE and dist > 1e-9:
            cos_half = math.cos(math.radians(FLASHLIGHT_HALF_FOV_DEG))
            cos_theta = (
                (ox - player_pos[0]) * math.cos(player_angle_rad)
                + (oy - player_pos[1]) * math.sin(player_angle_rad)
            ) / dist
            in_cone = cos_theta >= cos_half
        elif (ox, oy) == player_pos:
            in_cone = True
        if in_cone:
            s[8] = 1.0
        s[9] = (player_pos[0] - ox) / w
        s[10] = (player_pos[1] - oy) / h

        n = math.hypot(ox - player_pos[0], oy - player_pos[1])
        if n > 0:
            s[11] = (
                (ox - player_pos[0]) * math.cos(player_angle_rad)
                + (oy - player_pos[1]) * math.sin(player_angle_rad)
            ) / n

    return obs


# ---------------------------------------------------------------------------
# Scent gradient (for FallbackPolicy-equivalent reward shaping).
# ---------------------------------------------------------------------------


def chase_scent_map(topology: Topology, player_pos: tuple[int, int]) -> np.ndarray:
    """BFS distance from player over walkable cells. Used for potential shaping."""
    INF = 10**9
    scent = np.full((topology.height, topology.width), INF, dtype=np.int32)
    scent[player_pos[1], player_pos[0]] = 0
    q = [player_pos]
    head = 0
    dirs = [(0, -1), (1, 0), (0, 1), (-1, 0), (1, -1), (1, 1), (-1, 1), (-1, -1)]
    while head < len(q):
        cx, cy = q[head]
        head += 1
        for dx, dy in dirs:
            nx, ny = cx + dx, cy + dy
            if not topology.is_walkable(nx, ny):
                continue
            if scent[ny, nx] > scent[cy, cx] + 1:
                scent[ny, nx] = scent[cy, cx] + 1
                q.append((nx, ny))
    return scent


# ---------------------------------------------------------------------------
# Environment.
# ---------------------------------------------------------------------------


PlayerPolicyFn = Callable[["PursuerEnv", random.Random], tuple[int, float]]
"""
Callable returning (action_index, new_aim_angle_rad) for the scripted player.
action_index uses the same ACTION_* enum as the pursuer.
"""


class PursuerEnv(gym.Env):
    """
    Gymnasium env training a Pursuer. One player, one pursuer, one topology
    per episode. Reward shaping is documented inline (see `_compute_reward`).

    The env is deliberately minimal — no items, no multi-level runs, no
    closed-door opening. It's the smallest slice of the real game that still
    exercises stealth/ambush reasoning, which is what the paper gain of RL
    over ChaseBehavior lives in.
    """

    metadata = {"render_modes": []}

    PLAYER_PROFILES = ("default", "aggressive", "cautious", "timid")

    def __init__(
        self,
        topologies_dir: str,
        player_policy: PlayerPolicyFn,
        max_episode_steps: int = 200,
        pursuer_hp: int = 10,
        player_hp: int = 10,
        attack_damage: int = 2,
        seed: Optional[int] = None,
        # --- Domain randomization knobs (all default to off for back-compat) ---
        max_episode_steps_range: Optional[tuple[int, int]] = None,
        distractor_prob: float = 0.0,
        randomize_player_profile: bool = False,
    ):
        super().__init__()
        self._topology_paths = sorted(glob.glob(os.path.join(topologies_dir, "*.json")))
        if not self._topology_paths:
            raise ValueError(f"no topology JSON files found under {topologies_dir!r}")
        self._player_policy = player_policy
        self._max_steps_default = max_episode_steps
        self._max_steps_range = max_episode_steps_range
        self._max_steps = max_episode_steps
        self._pursuer_max_hp = pursuer_hp
        self._player_max_hp = player_hp
        self._attack_damage = attack_damage
        self._distractor_prob = float(distractor_prob)
        self._randomize_player_profile = bool(randomize_player_profile)

        self.action_space = spaces.Discrete(ACTION_COUNT)
        self.observation_space = spaces.Box(
            low=-1.0, high=1.0, shape=(OBSERVATION_SIZE,), dtype=np.float32
        )

        self._rng = random.Random(seed)
        self._np_rng = np.random.default_rng(seed)

        self.topology: Optional[Topology] = None
        self.pursuer_pos: tuple[int, int] = (0, 0)
        self.pursuer_hp = pursuer_hp
        self.pursuer_stamina_frac: float = 1.0
        self.player_pos: tuple[int, int] = (0, 0)
        self.player_hp = player_hp
        self.player_angle = 0.0
        self.player_profile: str = "default"
        self.distractor_pos: Optional[tuple[int, int]] = None
        self.memory = PursuerMemory()
        self.step_count = 0
        self._was_visible_to_player = False
        self._prev_scent_val: Optional[int] = None
        self._cone_mask_cache: Optional[np.ndarray] = None
        self._exit_scent_cache: Optional[np.ndarray] = None
        self._exit_camp_counter = 0

        self.memory = PursuerMemory()
        self.step_count = 0
        self._fixed_profile = "default"

    # ------------------------------------------------------------------
    # Gymnasium API.
    # ------------------------------------------------------------------

    def reset(self, *, seed: Optional[int] = None, options=None):  # noqa: D401
        if seed is not None:
            self._rng = random.Random(seed)
            self._np_rng = np.random.default_rng(seed)

        path = self._rng.choice(self._topology_paths)
        self.topology = Topology.from_json(path)

        self.pursuer_pos, self.player_pos = self._place_actors()
        self.pursuer_hp = self._pursuer_max_hp
        self.player_hp = self._player_max_hp
        self.pursuer_stamina_frac = self._rng.uniform(0.3, 1.0)
        self.player_angle = self._rng.uniform(-math.pi, math.pi)
        self.memory = PursuerMemory()
        self.step_count = 0
        self._was_visible_to_player = False
        self._prev_scent_val = None
        self._exit_camp_counter = 0
        self._exit_scent_cache = chase_scent_map(self.topology, self.topology.exit_point)

        # --- Domain randomization per episode ----------------------------
        if self._max_steps_range is not None:
            lo, hi = self._max_steps_range
            self._max_steps = self._rng.randint(lo, hi)
        else:
            self._max_steps = self._max_steps_default

        self.player_profile = (
            self._rng.choice(self.PLAYER_PROFILES)
            if self._randomize_player_profile
            else self._fixed_profile
        )

        self.distractor_pos = None
        if self._distractor_prob > 0.0 and self._rng.random() < self._distractor_prob:
            self.distractor_pos = self._place_distractor()

        self._cone_mask_cache = self._compute_cone()

        obs = self._observe()
        return obs, {
            "topology": os.path.basename(path),
            "player_profile": self.player_profile,
            "has_distractor": self.distractor_pos is not None,
            "max_steps": self._max_steps,
        }

    def step(self, action: int):
        assert self.topology is not None, "reset() before step()"

        # --- Pursuer moves --------------------------------------------------
        prev_pos = self.pursuer_pos
        self.pursuer_pos = self._try_move(self.pursuer_pos, int(action))

        # Memory bookkeeping (mirrors PursuerBehavior.updateMemory in Go).
        self.memory.push_trail(self.pursuer_pos)
        if has_los(self.topology, self.pursuer_pos, self.player_pos):
            self.memory.last_seen = self.player_pos
            self.memory.turns_since_los = 0
        else:
            if self.memory.turns_since_los < PURSUER_MEMORY_HORIZON:
                self.memory.turns_since_los += 1

        # Pursuer attack — adjacency check.
        pursuer_hit = self._adjacent(self.pursuer_pos, self.player_pos)
        if pursuer_hit and action != ACTION_WAIT:
            self.player_hp -= self._attack_damage

        # --- Scripted player reacts ----------------------------------------
        player_action, new_angle = self._player_policy(self, self._rng)
        self.player_angle = new_angle
        self.player_pos = self._try_move(self.player_pos, player_action)
        # Player counter-attack if adjacent (simulates the real Combat service).
        if self._adjacent(self.pursuer_pos, self.player_pos) and player_action != ACTION_WAIT:
            self.pursuer_hp -= self._attack_damage

        # `_was_visible_to_player` carries *last turn's* visibility into
        # this turn's reward/info — that's the ambush condition.
        was_visible_before_step = self._was_visible_to_player

        # Distractor wanders randomly; it never attacks, but its presence
        # adds noise to the `other_monster` channel.
        self._advance_distractor()

        # Cone snapshot after player moved/rotated — used next tick.
        self._cone_mask_cache = self._compute_cone()

        # --- Reward --------------------------------------------------------
        reward = self._compute_reward(
            action=int(action),
            prev_pos=prev_pos,
            pursuer_hit=pursuer_hit,
        )

        self.step_count += 1
        terminated = self.player_hp <= 0 or self.pursuer_hp <= 0
        truncated = self.step_count >= self._max_steps

        # The "ambush" side-channel updates after this step's reward is computed.
        self._was_visible_to_player = (
            self._cone_mask_cache is not None
            and self._cone_mask_cache[self.pursuer_pos[1], self.pursuer_pos[0]]
        )

        obs = self._observe()
        info = {
            "pursuer_hp": self.pursuer_hp,
            "player_hp": self.player_hp,
            "caught": self.player_hp <= 0,
            "pursuer_hit": pursuer_hit,
            "ambush_hit": pursuer_hit and not was_visible_before_step,
            "in_cone": bool(self._was_visible_to_player),
        }
        return obs, reward, terminated, truncated, info

    # ------------------------------------------------------------------
    # Internals.
    # ------------------------------------------------------------------

    def set_curriculum(self, distractor_prob: float, randomize_profile: bool, fixed_profile: str):
        self._distractor_prob = float(distractor_prob)
        self._randomize_player_profile = bool(randomize_profile)
        self._fixed_profile = fixed_profile

    def _place_actors(self) -> tuple[tuple[int, int], tuple[int, int]]:
        """Place player and pursuer in different rooms when possible."""
        assert self.topology is not None
        rooms = list(self.topology.rooms)
        if len(rooms) >= 2:
            self._rng.shuffle(rooms)
            player_room, pursuer_room = rooms[0], rooms[1]
        else:
            player_room = pursuer_room = rooms[0]

        def pick(r):
            pts = self.topology.room_walkable_points(r)
            return self._rng.choice(pts) if pts else (0, 0)

        # Retry once if collision happens in degenerate single-room topologies.
        player = pick(player_room)
        pursuer = pick(pursuer_room)
        if pursuer == player:
            walkable = self.topology.walkable_points()
            pursuer = next((p for p in walkable if p != player), pursuer)
        return pursuer, player

    def _place_distractor(self) -> Optional[tuple[int, int]]:
        """Pick a walkable cell distinct from player and pursuer."""
        assert self.topology is not None
        walkable = self.topology.walkable_points()
        occupied = {self.player_pos, self.pursuer_pos}
        candidates = [p for p in walkable if p not in occupied]
        if not candidates:
            return None
        return self._rng.choice(candidates)

    def _advance_distractor(self) -> None:
        """Random cardinal walk for the distractor; stays put on collisions."""
        if self.distractor_pos is None or self.topology is None:
            return
        moves = list(ACTION_VECTORS.values())
        self._rng.shuffle(moves)
        for dx, dy in moves:
            nx, ny = self.distractor_pos[0] + dx, self.distractor_pos[1] + dy
            if (nx, ny) in (self.player_pos, self.pursuer_pos):
                continue
            if self.topology.is_walkable(nx, ny):
                self.distractor_pos = (nx, ny)
                return

    def _try_move(self, pos: tuple[int, int], action: int) -> tuple[int, int]:
        dx, dy = ACTION_VECTORS.get(action, (0, 0))
        nx, ny = pos[0] + dx, pos[1] + dy
        if self.topology is not None and self.topology.is_walkable(nx, ny):
            return (nx, ny)
        return pos

    def _adjacent(self, a: tuple[int, int], b: tuple[int, int]) -> bool:
        return abs(a[0] - b[0]) + abs(a[1] - b[1]) == 1

    def exit_scent_map(self) -> np.ndarray:
        """BFS distance from the topology exit over walkable cells.

        Cached per-episode in `reset()`; the exit doesn't move so the map is
        stable. Used by the goal-directed scripted player.
        """
        assert self.topology is not None
        if self._exit_scent_cache is None:
            self._exit_scent_cache = chase_scent_map(
                self.topology, self.topology.exit_point
            )
        return self._exit_scent_cache

    def _compute_cone(self) -> np.ndarray:
        assert self.topology is not None
        return flashlight_mask(self.topology, self.player_pos, self.player_angle)

    def _observe(self) -> np.ndarray:
        assert self.topology is not None
        return build_observation(
            topology=self.topology,
            pursuer_pos=self.pursuer_pos,
            pursuer_hp_frac=self.pursuer_hp / max(self._pursuer_max_hp, 1),
            pursuer_stamina_frac=self.pursuer_stamina_frac,
            memory=self.memory,
            player_pos=self.player_pos,
            player_angle_rad=self.player_angle,
            cone_mask=self._cone_mask_cache,
            other_pursuer_pos=self.distractor_pos,
        )

    def _compute_reward(
        self,
        *,
        action: int,
        prev_pos: tuple[int, int],
        pursuer_hit: bool,
    ) -> float:
        """
        Reward breakdown:

        +10  per hit landed
        +5   ambush bonus if pursuer wasn't visible to the player last turn
        +20  killed player (episode ends)
        -2   pursuer dies
        +0.1 * Δchebyshev  approach shaping — +0.1 per step closer, −0.1 per
                  step away. Replaces Dijkstra scent-delta, which mixed Pursuer
                  and player motion into one noisy signal and let PPO collapse
                  to "never engage".
        -0.1 * k  per turn camping near exit (k = consecutive turns, resets when
                  leaving the camp zone or when player approaches within 4 tiles —
                  intercepting a fleeing player is legitimate, loitering is not)
        -0.001 per step                (time penalty)
        Wait action:
          -0.1  if in the player's cone (freeze-in-spotlight: worst possible wait)
          -0.02 if blind (turns_since_LOS > 10 — no stealth gain from stalling)
           0    otherwise (stalking wait out of cone — allowed)
        """
        assert self.topology is not None
        r = 0.0
        if pursuer_hit:
            r += 10.0
            if not self._was_visible_to_player:
                r += 5.0
        if self.player_hp <= 0:
            r += 20.0
        if self.pursuer_hp <= 0:
            r -= 2.0

        # Chebyshev-approach shaping. Previously Dijkstra potential shaping
        # (0.1 * Δscent), but that signal was noisy — it mixed Pursuer motion
        # with scripted-player motion, so PPO couldn't tell whose move caused
        # the change. Deterministic chebyshev-delta gives +0.1 per approach
        # step and -0.1 per retreat step regardless of player motion, which
        # is the strong exploration gradient PPO needs before it can discover
        # hits/ambush.
        prev_dist = max(
            abs(self.player_pos[0] - prev_pos[0]),
            abs(self.player_pos[1] - prev_pos[1]),
        )
        cur_dist = max(
            abs(self.player_pos[0] - self.pursuer_pos[0]),
            abs(self.player_pos[1] - self.pursuer_pos[1]),
        )
        r += 0.1 * (prev_dist - cur_dist)

        in_cone_now = (
            self._cone_mask_cache is not None
            and self._cone_mask_cache[self.pursuer_pos[1], self.pursuer_pos[0]]
        )
        # Stealth penalty removed: -0.05 per in-cone step biased PPO toward
        # pure hiding (eval at 500K steps: len=220, hits=0, death=0 —
        # cowardice collapse). The cone mask is already in the observation,
        # so the agent can learn when to hide from downstream reward (hit
        # vs counter-attack) instead of a hand-coded prior.

        # Anti-camping: Alien: Isolation-style — the Xenomorph must not just
        # park on the exit. Escalating penalty while within 3 chebyshev tiles
        # of the exit, resets the moment the pursuer steps out or the player
        # closes in (intercept is a legitimate tactic).
        EXIT_CAMP_RADIUS = 3
        EXIT_CAMP_INTERCEPT_RADIUS = 4
        exit_x, exit_y = self.topology.exit_point
        dist_to_exit = max(
            abs(exit_x - self.pursuer_pos[0]),
            abs(exit_y - self.pursuer_pos[1]),
        )
        dist_to_player = max(
            abs(self.player_pos[0] - self.pursuer_pos[0]),
            abs(self.player_pos[1] - self.pursuer_pos[1]),
        )
        if (
            dist_to_exit <= EXIT_CAMP_RADIUS
            and dist_to_player > EXIT_CAMP_INTERCEPT_RADIUS
        ):
            self._exit_camp_counter += 1
            r -= 0.1 * self._exit_camp_counter
        else:
            self._exit_camp_counter = 0

        r -= 0.001

        if action == ACTION_WAIT:
            if in_cone_now:
                r -= 0.1
            elif self.memory.turns_since_los > 10:
                r -= 0.02
            # else: stalking wait out of sight — permitted.
        return r
