#!/usr/bin/env bash
set -euo pipefail

pip install --no-cache-dir -r rl/requirements.txt jupyterlab ipykernel pytest

go mod download

echo
echo "Dev container ready."
echo "  Jupyter:     jupyter lab --ip=0.0.0.0 --no-browser --allow-root  (port 8888)"
echo "  TensorBoard: tensorboard --logdir rl/runs --host 0.0.0.0         (port 6006)"
echo "  Go tests:    go test ./...      (ORT loaded via ROGUE_ONNX_LIB_PATH)"
echo "  Python:      python -m pytest rl/tests"
