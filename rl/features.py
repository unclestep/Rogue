"""
CNN feature extractor for the Pursuer PPO agent.

The observation is a flat 859-float vector:
  * first  7*11*11 = 847 floats — CHW grid crop around the pursuer,
  * last          12 floats — scalar sensor readings (hp, los-age, ...).

A CNN over the grid block exploits spatial locality (walls, player cone,
self-trail, memory-decay) far better than the default MLP, which has to
relearn translation invariance through fully-connected layers.

The scalar tail is pushed through a tiny MLP and concatenated before the
shared policy/value heads — this matches the split in the Go observation
builder (internal/domain/service/rl_observation.go) 1:1, so the two sides
stay in the same mental model.
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
    """Split-stream extractor: Conv2d tower on the grid, MLP on the scalars."""

    def __init__(self, observation_space: gym.spaces.Box, features_dim: int = 128):
        super().__init__(observation_space, features_dim=features_dim)

        self.conv = nn.Sequential(
            nn.Conv2d(OBSERVATION_CHANNELS, 16, kernel_size=3, padding=1),
            nn.ReLU(inplace=True),
            nn.Conv2d(16, 32, kernel_size=3, padding=1),
            nn.ReLU(inplace=True),
            nn.Flatten(),
        )
        with torch.no_grad():
            dummy = torch.zeros(
                1, OBSERVATION_CHANNELS, OBSERVATION_CROP_SIZE, OBSERVATION_CROP_SIZE
            )
            conv_out_dim = self.conv(dummy).shape[1]

        self.grid_head = nn.Sequential(
            nn.Linear(conv_out_dim, 96),
            nn.ReLU(inplace=True),
        )
        self.scalar_head = nn.Sequential(
            nn.Linear(OBSERVATION_SCALARS, 32),
            nn.ReLU(inplace=True),
        )
        self.fuse = nn.Sequential(
            nn.Linear(96 + 32, features_dim),
            nn.ReLU(inplace=True),
        )

    def forward(self, observations: torch.Tensor) -> torch.Tensor:
        grid = observations[:, :OBSERVATION_GRID_FLOATS].view(
            -1, OBSERVATION_CHANNELS, OBSERVATION_CROP_SIZE, OBSERVATION_CROP_SIZE
        )
        scalars = observations[:, OBSERVATION_GRID_FLOATS:]
        g = self.grid_head(self.conv(grid))
        s = self.scalar_head(scalars)
        return self.fuse(torch.cat([g, s], dim=1))
