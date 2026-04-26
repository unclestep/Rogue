FROM python:3.14-slim

WORKDIR /app

# System deps + Node.js 22 LTS
RUN apt-get update && \
    apt-get install -y --no-install-recommends \
    curl \
    git \
    ca-certificates \
    gnupg \
    build-essential && \
    curl -fsSL https://deb.nodesource.com/setup_22.x | bash - && \
    apt-get install -y --no-install-recommends nodejs && \
    apt-get clean && \
    rm -rf /var/lib/apt/lists/*

# Go 1.26.1 (must match .github/workflows/go.yaml + go.mod)
RUN set -eux; \
    case "$(dpkg --print-architecture)" in \
    arm64) ARCH=linux-arm64 ;; \
    amd64) ARCH=linux-amd64 ;; \
    *) echo "unsupported arch: $(dpkg --print-architecture)"; exit 1 ;; \
    esac; \
    curl -fsSL "https://go.dev/dl/go1.26.1.${ARCH}.tar.gz" \
    -o /tmp/go.tar.gz && \
    tar -C /usr/local -xzf /tmp/go.tar.gz && \
    rm /tmp/go.tar.gz

RUN useradd -ms /bin/bash user

ENV HOME="/home/user"
ENV PATH="/usr/local/go/bin:/home/user/go/bin:/home/user/.local/bin:${PATH}"
ENV GOPATH="/home/user/go"
ENV NPM_CONFIG_PREFIX="/usr/local"

# ONNX Runtime
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

# Python dependencies
COPY --chown=user:user rl/requirements.txt rl/
RUN pip install --no-cache-dir --root-user-action=ignore -r rl/requirements.txt && \
    python -m ipykernel install --prefix=/home/user/.local --name=python3

# Go dependencies
COPY --chown=user:user go.mod go.sum* ./
RUN go mod download && \
    go clean -modcache

# Claude Code
ENV NPM_CONFIG_PREFIX="/usr/local"
RUN npm install -g @anthropic-ai/claude-code && \
    npm cache clean --force

# Switch to non-root
RUN chown user:user /app
USER user

# Project sources
COPY --chown=user:user . .

EXPOSE 7777
