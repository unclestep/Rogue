"""
CNN feature extractor for the Pursuer QR-DQN agent.

The observation is a flat 1101-float vector:

  * grid block: first 9 * 11 * 11 = 1089 floats, channels-first layout
                (walkable, closed_door, other_monster, self_trail,
                 player_visible_now, player_memory, player_cone,
                 scent, age_of_last_seen)
  * scalars:    trailing 12 floats (hp, stamina, los-age, dx/dy to last-seen,
                in_cone, dx/dy to player, cos of cone axis, etc.)

Architecture: CoordConv → ResNet backbone → scalar MLP → fused head.
Designed to work with QR-DQN (no dueling head here — dueling is implemented
in the custom QNetwork passed to QRDQN via policy_kwargs).
"""

from __future__ import annotations

import math

import gymnasium as gym
import torch
import torch.nn as nn
from stable_baselines3.common.torch_layers import BaseFeaturesExtractor

try:
    from .pursuer_env import (
        OBSERVATION_CHANNELS,
        OBSERVATION_CROP_SIZE,
        OBSERVATION_GRID_FLOATS,
        OBSERVATION_SCALARS,
    )
except ImportError:  # Colab: loaded as top-level module
    from pursuer_env import (
        OBSERVATION_CHANNELS,
        OBSERVATION_CROP_SIZE,
        OBSERVATION_GRID_FLOATS,
        OBSERVATION_SCALARS,
    )


# ---------------------------------------------------------------------------
# CoordConv helper
# ---------------------------------------------------------------------------


def _make_coord_channels(crop: int) -> torch.Tensor:
    """Returns (2, crop, crop) with normalised x/y coordinates in [-1, 1]."""
    xs = torch.linspace(-1.0, 1.0, crop)
    ys = torch.linspace(-1.0, 1.0, crop)
    grid_y, grid_x = torch.meshgrid(ys, xs, indexing="ij")
    return torch.stack([grid_x, grid_y], dim=0)  # (2, H, W)


# ---------------------------------------------------------------------------
# ResBlock
# ---------------------------------------------------------------------------


class _ResBlock(nn.Module):
    """Pre-activation ResBlock with LayerNorm and optional projection skip."""

    def __init__(self, in_ch: int, out_ch: int, stride: int = 1):
        super().__init__()
        self.norm1 = nn.GroupNorm(num_groups=min(8, in_ch), num_channels=in_ch)
        self.conv1 = nn.Conv2d(in_ch, out_ch, 3, padding=1, stride=stride, bias=False)
        self.norm2 = nn.GroupNorm(num_groups=min(8, out_ch), num_channels=out_ch)
        self.conv2 = nn.Conv2d(out_ch, out_ch, 3, padding=1, bias=False)

        self.skip = (
            nn.Conv2d(in_ch, out_ch, 1, stride=stride, bias=False)
            if (in_ch != out_ch or stride != 1)
            else nn.Identity()
        )

    def forward(self, x: torch.Tensor) -> torch.Tensor:
        out = self.conv1(torch.relu(self.norm1(x)))
        out = self.conv2(torch.relu(self.norm2(out)))
        return out + self.skip(x)


# ---------------------------------------------------------------------------
# Main extractor
# ---------------------------------------------------------------------------


class PursuerResCNN(BaseFeaturesExtractor):
    """Split-stream extractor for QR-DQN.

    Grid path (CoordConv → ResNet):
      Input: (batch, 9, 11, 11)
      +CoordConv → (batch, 11, 11, 11)
      ResBlock(11→32, stride=1)
      ResBlock(32→64, stride=2)  [11×11 → 6×6]
      AdaptiveAvgPool(3)          [6×6 → 3×3]
      Flatten → Linear(64·9 → 192) → ReLU

    Scalar path:
      Linear(12 → 32) → ReLU

    Fusion:
      Concat(192+32=224) → Linear(224 → features_dim) → ReLU
    """

    def __init__(self, observation_space: gym.spaces.Box, features_dim: int = 256):
        super().__init__(observation_space, features_dim=features_dim)
        obs_len = OBSERVATION_GRID_FLOATS + OBSERVATION_SCALARS
        assert observation_space.shape == (obs_len,), (
            f"expected flat obs of length {obs_len}, got {observation_space.shape}"
        )

        coord_ch = 2
        total_ch = OBSERVATION_CHANNELS + coord_ch  # 11

        self.register_buffer(
            "_coord",
            _make_coord_channels(OBSERVATION_CROP_SIZE).unsqueeze(0),  # (1, 2, H, W)
        )

        self.backbone = nn.Sequential(
            _ResBlock(total_ch, 32, stride=1),
            _ResBlock(32, 64, stride=2),
            nn.AdaptiveAvgPool2d(3),
            nn.Flatten(),
        )
        with torch.no_grad():
            dummy = torch.zeros(1, total_ch, OBSERVATION_CROP_SIZE, OBSERVATION_CROP_SIZE)
            grid_out_dim = self.backbone(dummy).shape[1]

        self.grid_head = nn.Sequential(
            nn.Linear(grid_out_dim, 192),
            nn.ReLU(inplace=True),
        )
        self.scalar_head = nn.Sequential(
            nn.Linear(OBSERVATION_SCALARS, 32),
            nn.ReLU(inplace=True),
        )
        self.fuse = nn.Sequential(
            nn.Linear(192 + 32, features_dim),
            nn.ReLU(inplace=True),
        )

        self._init_weights()

    def _init_weights(self):
        for m in self.modules():
            if isinstance(m, nn.Conv2d):
                nn.init.kaiming_normal_(m.weight, mode="fan_out", nonlinearity="relu")
            elif isinstance(m, nn.Linear):
                nn.init.orthogonal_(m.weight, gain=math.sqrt(2))
                if m.bias is not None:
                    nn.init.zeros_(m.bias)

    def forward(self, observations: torch.Tensor) -> torch.Tensor:
        grid_flat = observations[:, :OBSERVATION_GRID_FLOATS]
        scalars = observations[:, OBSERVATION_GRID_FLOATS:]

        grid = grid_flat.view(-1, OBSERVATION_CHANNELS, OBSERVATION_CROP_SIZE, OBSERVATION_CROP_SIZE)
        coord = self._coord.expand(grid.shape[0], -1, -1, -1)
        grid = torch.cat([grid, coord], dim=1)  # (B, 11, 11, 11)

        g = self.grid_head(self.backbone(grid))
        s = self.scalar_head(scalars)
        return self.fuse(torch.cat([g, s], dim=1))


# Keep old name as alias for backward compat with any external references.
PursuerCNN = PursuerResCNN
