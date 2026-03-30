# This file is licensed under the Business Source License 1.1. See the LICENSE file for details.
# Copyright (c) 2026 Uniforge GmbH. All rights reserved.

FROM ubuntu:24.04

# Prevent interactive prompts during package install
ENV DEBIAN_FRONTEND=noninteractive

# Install base tools
RUN apt-get update && apt-get install -y \
    curl \
    git \
    zsh \
    ripgrep \
    ca-certificates \
    gnupg \
    && rm -rf /var/lib/apt/lists/*

# Install Node.js LTS via NodeSource
RUN curl -fsSL https://deb.nodesource.com/setup_lts.x | bash - \
    && apt-get install -y nodejs \
    && rm -rf /var/lib/apt/lists/*

# Create non-root user
# Ubuntu 24.04 already has uid 1000 ('ubuntu') — use a high uid to avoid conflict
RUN useradd --uid 9999 --no-log-init --create-home --shell /bin/zsh sandbox

# Install Claude Code globally (as root, into system npm)
RUN npm install -g @anthropic-ai/claude-code

# Switch to non-root user
USER sandbox

# Set HOME explicitly so ~/.claude resolves correctly
ENV HOME=/home/sandbox

WORKDIR /workspace
