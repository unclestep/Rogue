"""
Visualization utilities for the Pursuer QR-DQN agent.

Key feature: Q-value heatmap — scans all walkable cells with a fixed player
position, runs the Q-network, and renders a 2-D spatial map of max Q-values
with arrows showing the best action at each cell.  This lets you see at a
glance whether the agent has learned spatial coverage vs. a "follow the
shortest path" collapse.

Usage example (inside a training callback or notebook):

    from rl.visualize import render_q_heatmap, plot_reward_decomposition
    img = render_q_heatmap(model, env.topology, env.player_pos)
    writer.add_image("q_value_heatmap", img, global_step)

All render_* functions return (C, H, W) uint8 tensors suitable for
torch.utils.tensorboard.SummaryWriter.add_image.
"""

from __future__ import annotations

from typing import Optional, TYPE_CHECKING

import math
import numpy as np

try:
    import matplotlib
    matplotlib.use("Agg")
    import matplotlib.pyplot as plt
    import matplotlib.patches as mpatches
    from matplotlib.colors import Normalize
    _MPL_OK = True
except ImportError:
    _MPL_OK = False

if TYPE_CHECKING:
    import torch

try:
    from .pursuer_env import (
        Topology,
        PursuerMemory,
        build_observation,
        chase_scent_map,
        ACTION_VECTORS,
        ACTION_COUNT,
        OBSERVATION_SIZE,
    )
except ImportError:
    from pursuer_env import (
        Topology,
        PursuerMemory,
        build_observation,
        chase_scent_map,
        ACTION_VECTORS,
        ACTION_COUNT,
        OBSERVATION_SIZE,
    )


# ---------------------------------------------------------------------------
# Q-value heatmap (mandatory per plan)
# ---------------------------------------------------------------------------


def render_q_heatmap(
    model,
    topology: Topology,
    player_pos: tuple[int, int],
    player_angle: float = 0.0,
    dpi: int = 80,
    figsize: tuple[int, int] = (8, 8),
) -> np.ndarray:
    """Return a (3, H_px, W_px) uint8 array showing Q-value heatmap.

    For every walkable cell in the topology we build an observation (pursuer
    at that cell, player at player_pos) and query the Q-network.  We render:
      • background: max Q-value colour-coded (viridis)
      • arrows: best action direction at each cell
      • white square: player position
    """
    if not _MPL_OK:
        raise ImportError("matplotlib is required for visualization")

    import torch

    device = next(model.policy.parameters()).device
    walkable = topology.walkable_points()
    memory = PursuerMemory()
    scent_map = chase_scent_map(topology, player_pos)

    obs_list = []
    for px, py in walkable:
        obs = build_observation(
            topology=topology,
            pursuer_pos=(px, py),
            pursuer_hp_frac=1.0,
            pursuer_stamina_frac=1.0,
            memory=memory,
            player_pos=player_pos,
            player_angle_rad=player_angle,
            cone_mask=None,
            scent_map=scent_map,
        )
        obs_list.append(obs)

    obs_tensor = torch.tensor(np.stack(obs_list), dtype=torch.float32, device=device)

    with torch.no_grad():
        quantiles = model.policy.quantile_net(obs_tensor)  # (N, n_actions, n_q)
        q_values = quantiles.mean(dim=-1).cpu().numpy()    # (N, n_actions)

    max_q = q_values.max(axis=1)
    best_a = q_values.argmax(axis=1)

    # Build spatial grid.
    q_grid = np.full((topology.height, topology.width), float("nan"))
    action_grid = np.full((topology.height, topology.width), -1, dtype=int)
    for i, (px, py) in enumerate(walkable):
        q_grid[py, px] = max_q[i]
        action_grid[py, px] = best_a[i]

    # Plot.
    fig, ax = plt.subplots(figsize=figsize, dpi=dpi)
    vmin = np.nanmin(q_grid) if not np.all(np.isnan(q_grid)) else 0
    vmax = np.nanmax(q_grid) if not np.all(np.isnan(q_grid)) else 1
    im = ax.imshow(
        q_grid,
        cmap="viridis",
        vmin=vmin,
        vmax=vmax,
        origin="upper",
        interpolation="nearest",
    )
    plt.colorbar(im, ax=ax, label="max Q-value")

    # Draw action arrows.
    arrow_kwargs = dict(head_width=0.3, head_length=0.2, fc="white", ec="white", alpha=0.7)
    for i, (px, py) in enumerate(walkable):
        a = best_a[i]
        dx, dy = ACTION_VECTORS.get(a, (0, 0))
        if dx != 0 or dy != 0:
            ax.arrow(px, py, dx * 0.4, dy * 0.4, **arrow_kwargs)

    # Mark player.
    ax.plot(player_pos[0], player_pos[1], "ws", markersize=8, label="player")
    # Mark exit.
    ex, ey = topology.exit_point
    ax.plot(ex, ey, "r*", markersize=10, label="exit")

    ax.set_title(f"Q-value heatmap | player=({player_pos[0]},{player_pos[1]})")
    ax.legend(loc="upper right", fontsize=7)
    ax.axis("off")
    plt.tight_layout()

    # Convert to (3, H, W) uint8.
    fig.canvas.draw()
    w_px, h_px = fig.canvas.get_width_height()
    img = np.frombuffer(fig.canvas.tostring_rgb(), dtype=np.uint8).reshape(h_px, w_px, 3)
    plt.close(fig)
    return img.transpose(2, 0, 1)  # (3, H, W)


# ---------------------------------------------------------------------------
# Quantile spread (uncertainty)
# ---------------------------------------------------------------------------


def compute_quantile_spread(model, obs_batch: np.ndarray) -> np.ndarray:
    """Return std of quantile estimates across actions for each obs.

    Shape: (N,) — high std means the network is uncertain.
    """
    import torch

    device = next(model.policy.parameters()).device
    tensor = torch.tensor(obs_batch, dtype=torch.float32, device=device)
    with torch.no_grad():
        quantiles = model.policy.quantile_net(tensor)  # (N, n_actions, n_q)
    return quantiles.std(dim=-1).mean(dim=-1).cpu().numpy()  # (N,)


# ---------------------------------------------------------------------------
# Reward decomposition (stacked area chart)
# ---------------------------------------------------------------------------


def plot_reward_decomposition(
    component_history: dict[str, list[float]],
    window: int = 1000,
) -> np.ndarray:
    """Stacked area chart of reward components over training steps.

    component_history: {name: [value_per_episode, ...]}
    Returns (3, H, W) uint8.
    """
    if not _MPL_OK:
        raise ImportError("matplotlib required")

    fig, ax = plt.subplots(figsize=(10, 4), dpi=80)
    labels = list(component_history.keys())
    colors = plt.cm.tab10(np.linspace(0, 1, len(labels)))

    bottoms = np.zeros(window)
    for label, color in zip(labels, colors):
        values = component_history[label]
        smoothed = np.convolve(values[-window:], np.ones(window) / window, mode="valid") if len(values) >= window else np.array(values)
        x = np.arange(len(smoothed))
        ax.fill_between(x, bottoms[: len(smoothed)], bottoms[: len(smoothed)] + smoothed, label=label, color=color, alpha=0.7)
        bottoms[: len(smoothed)] += smoothed

    ax.set_xlabel("Episode (rolling avg)")
    ax.set_ylabel("Reward contribution")
    ax.set_title("Reward decomposition")
    ax.legend(loc="upper left", fontsize=7, ncol=2)
    plt.tight_layout()

    fig.canvas.draw()
    w_px, h_px = fig.canvas.get_width_height()
    img = np.frombuffer(fig.canvas.tostring_rgb(), dtype=np.uint8).reshape(h_px, w_px, 3)
    plt.close(fig)
    return img.transpose(2, 0, 1)


# ---------------------------------------------------------------------------
# TensorBoard visualization callback
# ---------------------------------------------------------------------------


class QValueVisualizationCallback:
    """Integrates visualization into SB3 training via BaseCallback.

    Usage:
        from rl.visualize import QValueVisualizationCallback
        cb = QValueVisualizationCallback(eval_env, writer, freq=25_000)
        model.learn(..., callback=[cb, ...])
    """

    def __init__(self, eval_env, writer, freq: int = 25_000):
        self._env = eval_env
        self._writer = writer
        self._freq = freq
        self._step = 0

    def __call__(self, locals_: dict, globals_: dict) -> bool:
        self._step += 1
        if self._step % self._freq != 0:
            return True

        model = locals_.get("self")
        if model is None:
            return True

        try:
            obs, _ = self._env.reset()
            # Unwrap VecEnv if needed.
            topology = getattr(self._env, "topology", None) or getattr(self._env.envs[0], "topology", None)
            player_pos = getattr(self._env, "player_pos", None) or getattr(self._env.envs[0], "player_pos", (5, 5))
            if topology is None:
                return True

            img = render_q_heatmap(model, topology, player_pos)
            self._writer.add_image("q_value_heatmap", img, self._step)

            # Quantile spread.
            obs_batch = np.array([obs] if obs.ndim == 1 else obs, dtype=np.float32)
            spread = compute_quantile_spread(model, obs_batch)
            self._writer.add_scalar("metrics/quantile_spread_mean", float(spread.mean()), self._step)
        except Exception as e:
            print(f"[visualize] callback error: {e}")

        return True
