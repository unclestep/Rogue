"""Reinforcement-learning training harness for the Pursuer monster.

This package holds Python-only code (Gymnasium env, scripted player, ONNX
export helpers). The Go game consumes the exported ONNX model through
internal/domain/service/rl_onnx.go — no Python is required at runtime.
"""
