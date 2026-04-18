FROM mcr.microsoft.com/devcontainers/python:3.12

RUN apt-get update \
 && apt-get install -y --no-install-recommends \
      curl ca-certificates build-essential \
 && rm -rf /var/lib/apt/lists/*

# ONNX Runtime shared library — required by github.com/yalue/onnxruntime_go
# for Go tests (tests/application_test/pursuer_onnx_smoke_test.go and
# tests/service_test/rl_onnx_test.go). Version must match go.mod's
# onnxruntime_go v1.23.x.
ARG ORT_VERSION=1.23.0
RUN set -eux; \
    case "$(dpkg --print-architecture)" in \
      arm64) ORT_ARCH=aarch64 ;; \
      amd64) ORT_ARCH=x64 ;; \
      *) echo "unsupported arch: $(dpkg --print-architecture)"; exit 1 ;; \
    esac; \
    curl -fsSL "https://github.com/microsoft/onnxruntime/releases/download/v${ORT_VERSION}/onnxruntime-linux-${ORT_ARCH}-${ORT_VERSION}.tgz" \
      | tar -xz -C /opt; \
    ln -s "/opt/onnxruntime-linux-${ORT_ARCH}-${ORT_VERSION}" /opt/onnxruntime
