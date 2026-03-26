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

# Install Claude Code globally
RUN npm install -g @anthropic-ai/claude-code

# Create non-root sandbox user
# Ubuntu 24.04 already has uid 1000 ('ubuntu') — use uid 9999
RUN useradd --uid 9999 --no-log-init --create-home --shell /bin/zsh sandbox

# Copy and install the claustro init entrypoint
COPY claustro-init /usr/local/bin/claustro-init
RUN chmod +x /usr/local/bin/claustro-init

# Rust for sandbox user
RUN mkdir -p /home/sandbox/.cargo /home/sandbox/.rustup \
    && cp -r /root/.cargo /home/sandbox/.cargo \
    && chown -R sandbox:sandbox /home/sandbox

ENV HOME=/home/sandbox
ENV PATH="/home/sandbox/.cargo/bin:/usr/local/go/bin:${PATH}"

WORKDIR /workspace

ENTRYPOINT ["/usr/local/bin/claustro-init"]
CMD ["sleep", "infinity"]
