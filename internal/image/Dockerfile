# This file is licensed under the Business Source License 1.1. See the LICENSE file for details.
# Copyright (c) 2026 Uniforge GmbH. All rights reserved.

FROM ubuntu:24.04

ENV DEBIAN_FRONTEND=noninteractive

# Install base tools
RUN apt-get update && apt-get install -y \
    curl \
    git \
    zsh \
    ripgrep \
    fd-find \
    fzf \
    jq \
    tree \
    tmux \
    htop \
    make \
    ca-certificates \
    gnupg \
    build-essential \
    pkg-config \
    libssl-dev \
    iptables \
    && rm -rf /var/lib/apt/lists/*

# Install Node.js LTS
RUN curl -fsSL https://deb.nodesource.com/setup_lts.x | bash - \
    && apt-get install -y nodejs \
    && rm -rf /var/lib/apt/lists/*

# Install Go
RUN curl -fsSL https://go.dev/dl/go1.24.2.linux-$(dpkg --print-architecture).tar.gz \
    | tar -C /usr/local -xz
ENV PATH="/usr/local/go/bin:${PATH}"

# Install Rust
RUN curl --proto '=https' --tlsv1.2 -sSf https://sh.rustup.rs | sh -s -- -y --no-modify-path
ENV PATH="/root/.cargo/bin:${PATH}"

# Install Python 3 + pip
RUN apt-get update && apt-get install -y python3 python3-pip python3-venv \
    && rm -rf /var/lib/apt/lists/*

# Install GitHub CLI (gh)
RUN curl -fsSL https://cli.github.com/packages/githubcli-archive-keyring.gpg \
    | dd of=/usr/share/keyrings/githubcli-archive-keyring.gpg \
    && chmod go+r /usr/share/keyrings/githubcli-archive-keyring.gpg \
    && echo "deb [arch=$(dpkg --print-architecture) signed-by=/usr/share/keyrings/githubcli-archive-keyring.gpg] https://cli.github.com/packages stable main" \
    | tee /etc/apt/sources.list.d/github-cli.list > /dev/null \
    && apt-get update && apt-get install -y gh \
    && rm -rf /var/lib/apt/lists/*

# Install MCP servers (system-wide via npm)
RUN npm install -g \
    @modelcontextprotocol/server-filesystem \
    @modelcontextprotocol/server-memory

# Install MCP fetch server (Python-based)
RUN pip3 install --break-system-packages mcp-server-fetch

# Install ccstatusline (optional — native build may fail on some architectures)
RUN npm install -g ccstatusline || true

# Install gopls (Go language server for LSP support in Claude Code)
RUN GOPATH=/tmp/gopls-build go install golang.org/x/tools/gopls@latest \
    && mv /tmp/gopls-build/bin/gopls /usr/local/bin/gopls \
    && rm -rf /tmp/gopls-build

# Install clipboard shims (xclip and wl-paste) that bridge to the host clipboard
# via the claustro Unix socket mounted at /run/claustro/clipboard.sock.
COPY xclip-shim /usr/local/bin/xclip
COPY wl-paste-shim /usr/local/bin/wl-paste
RUN chmod +x /usr/local/bin/xclip /usr/local/bin/wl-paste \
    && mkdir -p /run/claustro

# Create non-root sandbox user at uid 1000
# Ubuntu 24.04 ships with uid 1000 ('ubuntu') — delete it first
RUN userdel -r ubuntu 2>/dev/null || true \
    && useradd --uid 1000 --no-log-init --create-home --shell /bin/zsh sandbox

# Copy and install the claustro init entrypoint
COPY claustro-init /usr/local/bin/claustro-init
RUN chmod +x /usr/local/bin/claustro-init

# Rust for sandbox user
RUN mkdir -p /home/sandbox/.cargo /home/sandbox/.rustup \
    && cp -r /root/.cargo /home/sandbox/.cargo \
    && cp -r /root/.rustup /home/sandbox/.rustup \
    && chown -R sandbox:sandbox /home/sandbox

# Install Claude Code via native installer (recommended method).
# Installs to ~/.local/bin/claude -> ~/.local/share/claude/versions/<ver>.
# Native install auto-updates in the background — no npm or sudo needed.
RUN su sandbox -c "curl -fsSL https://claude.ai/install.sh | sh"

# Pre-create npm and pip cache dirs owned by sandbox
RUN mkdir -p /home/sandbox/.npm /home/sandbox/.cache/pip \
    && chown -R sandbox:sandbox /home/sandbox/.npm /home/sandbox/.cache/pip

ENV HOME=/home/sandbox
ENV PATH="/home/sandbox/.local/bin:/home/sandbox/.cargo/bin:/usr/local/go/bin:${PATH}"

WORKDIR /workspace

ENTRYPOINT ["/usr/local/bin/claustro-init"]
CMD ["sleep", "infinity"]
