#!/usr/bin/env python3
"""
Generate a minimal ONNX model for the Pursuer integration test.

The model has the same input/output contract as the real trained policy
produced by PR 4 (rl/pursuer_training.ipynb):

    input  : float32 [1, 859]   (named "input")
    output : float32 [1, 5]     (named "logits")

It is deliberately trivial — a single Gemm with zero weights and a biased
constant — so the output is deterministic (argmax == 0, i.e. ActionUp)
regardless of the observation. That lets the Go test assert a known answer
while still exercising the full load-and-infer path through onnxruntime_go.

Usage:

    pip install onnx
    python rl/tools/gen_tiny_onnx.py

Output is written to internal/domain/service/testdata/pursuer_tiny.onnx and
is committed to the repo — no CI-side regeneration needed.
"""

from __future__ import annotations

import pathlib

import numpy as np
import onnx
from onnx import TensorProto, helper, numpy_helper

OBS_SIZE = 859
ACTION_COUNT = 5

# Bias chosen so that argmax(logits) == 0 (ActionUp) — lets the test assert
# a known action without relying on trained weights.
BIAS = np.array([0.9, 0.1, 0.1, 0.1, 0.1], dtype=np.float32)


def build_model() -> onnx.ModelProto:
    weight = numpy_helper.from_array(
        np.zeros((OBS_SIZE, ACTION_COUNT), dtype=np.float32), name="W"
    )
    bias = numpy_helper.from_array(BIAS.reshape(1, ACTION_COUNT), name="B")

    node = helper.make_node(
        "Gemm",
        inputs=["input", "W", "B"],
        outputs=["logits"],
        alpha=1.0,
        beta=1.0,
        transA=0,
        transB=0,
    )

    graph = helper.make_graph(
        [node],
        "pursuer_tiny",
        inputs=[
            helper.make_tensor_value_info("input", TensorProto.FLOAT, [1, OBS_SIZE])
        ],
        outputs=[
            helper.make_tensor_value_info(
                "logits", TensorProto.FLOAT, [1, ACTION_COUNT]
            )
        ],
        initializer=[weight, bias],
    )

    model = helper.make_model(
        graph,
        producer_name="rogue-rl-gen",
        opset_imports=[helper.make_opsetid("", 17)],
    )
    model.ir_version = 9
    onnx.checker.check_model(model)
    return model


def main() -> None:
    out = (
        pathlib.Path(__file__).resolve().parents[2]
        / "internal"
        / "domain"
        / "service"
        / "testdata"
        / "pursuer_tiny.onnx"
    )
    out.parent.mkdir(parents=True, exist_ok=True)
    onnx.save(build_model(), out)
    print(f"Wrote {out} ({out.stat().st_size} bytes)")


if __name__ == "__main__":
    main()
