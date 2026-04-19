"""
QR-DQN training script for the Pursuer agent.

Replaces the PPO notebook (pursuer_training.ipynb).
Run from the repo root:

    python -m rl.train_qrdqn --topologies rl/fixtures/topologies --steps 3000000

The trained model is saved to rl/models/pursuer_qrdqn.zip and exported to
rl/models/pursuer.onnx (compatible with the Go ONNXPolicy consumer).
"""

from __future__ import annotations

import argparse
import math
import os
from pathlib import Path

import numpy as np
import torch
import torch.nn as nn
from stable_baselines3.common.callbacks import BaseCallback, EvalCallback
from stable_baselines3.common.env_util import make_vec_env
from stable_baselines3.common.vec_env import SubprocVecEnv

try:
    from sb3_contrib import QRDQN
    from sb3_contrib.common.wrappers import ActionMasker
except ImportError:
    raise SystemExit(
        "sb3_contrib is required: pip install sb3-contrib"
    )

try:
    from .features import PursuerResCNN
    from .pursuer_env import OBSERVATION_SIZE, ACTION_COUNT, PursuerEnv
    from .scripted_player import scripted_player_policy
except ImportError:
    from features import PursuerResCNN
    from pursuer_env import OBSERVATION_SIZE, ACTION_COUNT, PursuerEnv
    from scripted_player import scripted_player_policy


# ---------------------------------------------------------------------------
# N-step wrapper
# ---------------------------------------------------------------------------


class NStepWrapper(PursuerEnv):
    """Accumulates n-step returns before passing to the replay buffer."""

    def __init__(self, *args, n_steps: int = 4, gamma: float = 0.99, **kwargs):
        super().__init__(*args, **kwargs)
        self._n = n_steps
        self._gamma = gamma
        self._buf_obs: list = []
        self._buf_rewards: list[float] = []
        self._buf_done: list[bool] = []
        self._pending_obs = None

    def step(self, action):
        obs, reward, terminated, truncated, info = super().step(action)
        self._buf_rewards.append(reward)
        done = terminated or truncated
        self._buf_done.append(done)

        if len(self._buf_rewards) >= self._n or done:
            # Compute n-step return.
            g = 0.0
            for r in reversed(self._buf_rewards):
                g = r + self._gamma * g
            self._buf_rewards.clear()
            self._buf_done.clear()
            return obs, g, terminated, truncated, info

        # Buffer not full yet and not done — return zero reward (buffered).
        return obs, 0.0, terminated, truncated, info

    def reset(self, **kwargs):
        self._buf_rewards.clear()
        self._buf_done.clear()
        return super().reset(**kwargs)


# ---------------------------------------------------------------------------
# TensorBoard metrics callback
# ---------------------------------------------------------------------------


class MetricsCallback(BaseCallback):
    """Logs catch rate, episode length, and Q-value stats to TensorBoard."""

    def __init__(self, eval_freq: int = 5000):
        super().__init__()
        self._eval_freq = eval_freq
        self._episode_rewards: list[float] = []
        self._episode_lengths: list[int] = []
        self._caught: list[bool] = []
        self._ep_reward = 0.0
        self._ep_len = 0

    def _on_step(self) -> bool:
        rewards = self.locals.get("rewards", [])
        dones = self.locals.get("dones", [])
        infos = self.locals.get("infos", [])

        for r, done, info in zip(rewards, dones, infos):
            self._ep_reward += float(r)
            self._ep_len += 1
            if done:
                self._episode_rewards.append(self._ep_reward)
                self._episode_lengths.append(self._ep_len)
                self._caught.append(bool(info.get("caught", False)))
                self._ep_reward = 0.0
                self._ep_len = 0

        if self.n_calls % self._eval_freq == 0 and self._caught:
            catch_rate = sum(self._caught) / len(self._caught)
            mean_len = sum(self._episode_lengths) / len(self._episode_lengths)
            mean_rew = sum(self._episode_rewards) / len(self._episode_rewards)
            self.logger.record("metrics/catch_rate", catch_rate)
            self.logger.record("metrics/ep_len_mean", mean_len)
            self.logger.record("metrics/ep_rew_mean", mean_rew)
            self._episode_rewards.clear()
            self._episode_lengths.clear()
            self._caught.clear()

        return True


# ---------------------------------------------------------------------------
# Training entry point
# ---------------------------------------------------------------------------


def make_env(topologies_dir: str, seed: int | None = None):
    def _init():
        env = NStepWrapper(
            topologies_dir=topologies_dir,
            player_policy=scripted_player_policy,
            max_episode_steps=200,
            n_steps=4,
            gamma=0.99,
            seed=seed,
        )
        return env
    return _init


def train(
    topologies_dir: str,
    total_steps: int = 3_000_000,
    n_envs: int = 8,
    output_dir: str = "rl/models",
    tb_log: str = "rl/tb_logs",
):
    os.makedirs(output_dir, exist_ok=True)
    os.makedirs(tb_log, exist_ok=True)

    env = make_vec_env(
        make_env(topologies_dir),
        n_envs=n_envs,
        vec_env_cls=SubprocVecEnv,
    )

    eval_env = make_vec_env(make_env(topologies_dir), n_envs=4)

    policy_kwargs = dict(
        features_extractor_class=PursuerResCNN,
        features_extractor_kwargs=dict(features_dim=256),
        n_quantiles=51,
        optimizer_class=torch.optim.AdamW,
        optimizer_kwargs=dict(weight_decay=1e-4),
    )

    model = QRDQN(
        policy="MlpPolicy",
        env=env,
        policy_kwargs=policy_kwargs,
        buffer_size=300_000,
        learning_starts=50_000,
        batch_size=256,
        gamma=0.99,
        learning_rate=3e-4,
        exploration_fraction=0.30,
        exploration_final_eps=0.05,
        train_freq=4,
        target_update_interval=1_000,
        optimize_memory_usage=False,
        verbose=1,
        tensorboard_log=tb_log,
        device="auto",
    )

    eval_callback = EvalCallback(
        eval_env,
        best_model_save_path=output_dir,
        log_path=output_dir,
        eval_freq=max(25_000 // n_envs, 1),
        n_eval_episodes=10,
        deterministic=True,
        render=False,
    )
    metrics_callback = MetricsCallback(eval_freq=5_000)

    model.learn(
        total_timesteps=total_steps,
        callback=[eval_callback, metrics_callback],
        progress_bar=True,
    )

    zip_path = os.path.join(output_dir, "pursuer_qrdqn.zip")
    model.save(zip_path)
    print(f"Model saved → {zip_path}")

    onnx_path = os.path.join(output_dir, "pursuer.onnx")
    _export_onnx(model, onnx_path)
    print(f"ONNX exported → {onnx_path}")

    env.close()
    eval_env.close()
    return model


# ---------------------------------------------------------------------------
# ONNX export (also available standalone via export_onnx.py)
# ---------------------------------------------------------------------------


class _QRDQNActor(nn.Module):
    """Wraps QR-DQN q_net to output mean Q-values (shape: batch × n_actions).

    QR-DQN internally stores quantile estimates of shape (batch, n_actions,
    n_quantiles). We average over quantiles before argmax — this is exactly
    what QRDQN.predict() does at inference time.
    """

    def __init__(self, q_net: nn.Module):
        super().__init__()
        self.q_net = q_net

    def forward(self, obs: torch.Tensor) -> torch.Tensor:
        quantiles = self.q_net(obs)          # (B, n_actions, n_quantiles)
        return quantiles.mean(dim=-1)         # (B, n_actions)


def _export_onnx(model: QRDQN, path: str) -> None:
    actor = _QRDQNActor(model.policy.quantile_net).eval()
    dummy = torch.zeros(1, OBSERVATION_SIZE, dtype=torch.float32)
    with torch.no_grad():
        torch.onnx.export(
            actor,
            dummy,
            path,
            input_names=["input"],
            output_names=["logits"],
            opset_version=17,
            dynamic_axes=None,
        )


# ---------------------------------------------------------------------------
# CLI
# ---------------------------------------------------------------------------


if __name__ == "__main__":
    parser = argparse.ArgumentParser(description="Train Pursuer QR-DQN")
    parser.add_argument(
        "--topologies",
        default="rl/fixtures/topologies",
        help="Path to topology JSON directory",
    )
    parser.add_argument("--steps", type=int, default=3_000_000)
    parser.add_argument("--envs", type=int, default=8)
    parser.add_argument("--output", default="rl/models")
    parser.add_argument("--tb-log", default="rl/tb_logs")
    args = parser.parse_args()

    train(
        topologies_dir=args.topologies,
        total_steps=args.steps,
        n_envs=args.envs,
        output_dir=args.output,
        tb_log=args.tb_log,
    )
