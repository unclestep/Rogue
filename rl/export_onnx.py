"""
Standalone ONNX export for a trained QR-DQN Pursuer model.

Usage:
    python -m rl.export_onnx --model rl/models/pursuer_qrdqn.zip \
                             --output rl/models/pursuer.onnx

The exported graph:
    input  : "input"  — float32 (1, 1101)
    output : "logits" — float32 (1, 5)   (mean Q-values over quantiles)

The Go ONNXPolicy consumes this with argmax(logits[0]) → action in [0,4].
No hidden state — every call is stateless.
"""

from __future__ import annotations

import argparse

import numpy as np
import torch
import torch.nn as nn

try:
    from sb3_contrib import QRDQN
except ImportError:
    raise SystemExit("sb3_contrib is required: pip install sb3-contrib")

try:
    from .pursuer_env import OBSERVATION_SIZE
    from .features import PursuerResCNN
except ImportError:
    from pursuer_env import OBSERVATION_SIZE
    from features import PursuerResCNN


class _QRDQNActor(nn.Module):
    """Mean Q-value exporter. Input (1, OBS) → output (1, n_actions)."""

    def __init__(self, q_net: nn.Module):
        super().__init__()
        self.q_net = q_net

    def forward(self, obs: torch.Tensor) -> torch.Tensor:
        return self.q_net(obs).mean(dim=-1)


def export(model_path: str, output_path: str, verify: bool = True) -> None:
    model = QRDQN.load(model_path, device="cpu")
    actor = _QRDQNActor(model.policy.quantile_net).eval()

    dummy = torch.zeros(1, OBSERVATION_SIZE, dtype=torch.float32)

    with torch.no_grad():
        torch.onnx.export(
            actor,
            dummy,
            output_path,
            input_names=["input"],
            output_names=["logits"],
            opset_version=17,
            dynamic_axes=None,
        )
    print(f"Exported → {output_path}")

    if verify:
        import onnxruntime as ort

        sess = ort.InferenceSession(output_path)
        np_dummy = np.zeros((1, OBSERVATION_SIZE), dtype=np.float32)
        onnx_out = sess.run(["logits"], {"input": np_dummy})[0]

        with torch.no_grad():
            pt_out = actor(dummy).numpy()

        max_diff = float(np.max(np.abs(onnx_out - pt_out)))
        print(f"Parity check max |PyTorch - ONNX| = {max_diff:.2e}")
        assert max_diff < 1e-4, f"Parity check FAILED: {max_diff}"
        print("Parity check PASSED")


if __name__ == "__main__":
    parser = argparse.ArgumentParser()
    parser.add_argument("--model", required=True, help="Path to .zip model file")
    parser.add_argument("--output", default="rl/models/pursuer.onnx")
    parser.add_argument("--no-verify", action="store_true")
    args = parser.parse_args()
    export(args.model, args.output, verify=not args.no_verify)
