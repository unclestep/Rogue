"""
CNN feature extractor for the Pursuer PPO agent.

The observation is a flat 859-float vector:

  * grid block: first 7 * 11 * 11 = 847 floats, channels-first layout
                (matches Go's `rl_observation.go`: walkable, closed_door,
                other_monster, self_trail, player_visible_now, player_memory,
                player_cone)
  * scalars:    trailing 12 floats (hp, stamina, los-age, dx/dy to last-seen,
                in_cone, dx/dy to player, cos of cone axis, etc.)

We split the flat vector into the two streams, run Conv2d on the grid and a
small MLP on the scalars, then concatenate. This is strictly a superset of
the default `FlattenExtractor` — if Conv learns nothing, the scalar stream
alone can still drive policy.
"""

from __future__ import annotations

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


class PursuerCNN(BaseFeaturesExtractor):
    """Split-stream extractor.

    Grid path: Conv2d(7→16, 3×3, pad=1, stride=1) → ReLU
             → Conv2d(16→32, 3×3, pad=1, stride=2) → ReLU
             → AdaptiveAvgPool2d(3)
             → Flatten → Linear(32·3·3 → 96) → ReLU

    Scalar path: Linear(12 → 32) → ReLU

    Fusion: Linear(96+32 → features_dim) → ReLU

    The `stride=2 + AdaptiveAvgPool` combo replaces the previous dumb
    `Linear(3872 → 96)` on the full flattened conv output, which was ~370K
    parameters of unstructured weights.
    """

    def __init__(self, observation_space: gym.spaces.Box, features_dim: int = 128):
        super().__init__(observation_space, features_dim=features_dim)
        assert observation_space.shape == (OBSERVATION_GRID_FLOATS + OBSERVATION_SCALARS,), (
            f"expected flat obs of length {OBSERVATION_GRID_FLOATS + OBSERVATION_SCALARS}, "
            f"got {observation_space.shape}"
        )

        self.conv = nn.Sequential(
            nn.Conv2d(OBSERVATION_CHANNELS, 16, kernel_size=3, padding=1, stride=1),
            nn.ReLU(inplace=True),
            nn.Conv2d(16, 32, kernel_size=3, padding=1, stride=2),
            nn.ReLU(inplace=True),
            nn.AdaptiveAvgPool2d(3),
            nn.Flatten(),
        )
        with torch.no_grad():
            dummy = torch.zeros(
                1, OBSERVATION_CHANNELS, OBSERVATION_CROP_SIZE, OBSERVATION_CROP_SIZE
            )
            conv_out_dim = self.conv(dummy).shape[1]

        self.grid_head = nn.Sequential(nn.Linear(conv_out_dim, 96), nn.ReLU(inplace=True))
        self.scalar_head = nn.Sequential(nn.Linear(OBSERVATION_SCALARS, 32), nn.ReLU(inplace=True))
        self.fuse = nn.Sequential(
            nn.Linear(96 + 32, features_dim),
            nn.ReLU(inplace=True),
        )

    def forward(self, observations: torch.Tensor) -> torch.Tensor:
        # observations: (batch, 859). Slice strictly by the published
        # constants — if obs layout drifts in Go or Python, the parity
        # tests catch it before we hit this code.
        grid_flat = observations[:, :OBSERVATION_GRID_FLOATS]
        scalars = observations[:, OBSERVATION_GRID_FLOATS:]
        grid = grid_flat.view(
            -1, OBSERVATION_CHANNELS, OBSERVATION_CROP_SIZE, OBSERVATION_CROP_SIZE
        )
        g = self.grid_head(self.conv(grid))
        s = self.scalar_head(scalars)
        return self.fuse(torch.cat([g, s], dim=1))
