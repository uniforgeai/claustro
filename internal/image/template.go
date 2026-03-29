package image

import (
	"bytes"
	"text/template"

	"github.com/uniforgeai/claustro/internal/config"
)

type templateData struct {
	Go, Rust, Python             bool
	DevTools, BuildTools         bool
	MCPFilesystem, MCPMemory, MCPFetch bool
}

const dockerfileTemplate = `FROM ubuntu:24.04

ENV DEBIAN_FRONTEND=noninteractive

# Install base tools
RUN apt-get update && apt-get install -y \
    curl \
    git \
    zsh \
    gnupg \
    iptables \
    ca-certificates \{{if .DevTools}}
    ripgrep \
    fd-find \
    fzf \
    jq \
    tree \
    htop \
    tmux \{{end}}{{if .BuildTools}}
    build-essential \
    make \
    pkg-config \
    libssl-dev \{{end}}
    && rm -rf /var/lib/apt/lists/*

# Install Node.js LTS
RUN curl -fsSL https://deb.nodesource.com/setup_lts.x | bash - \
    && apt-get install -y nodejs \
    && rm -rf /var/lib/apt/lists/*
{{if .Go}}
# Install Go
RUN curl -fsSL https://go.dev/dl/go1.24.2.linux-$(dpkg --print-architecture).tar.gz \
    | tar -C /usr/local -xz
ENV PATH="/usr/local/go/bin:${PATH}"
{{end}}{{if .Rust}}
# Install Rust
RUN curl --proto '=https' --tlsv1.2 -sSf https://sh.rustup.rs | sh -s -- -y --no-modify-path
ENV PATH="/root/.cargo/bin:${PATH}"
{{end}}{{if .Python}}
# Install Python 3 + pip
RUN apt-get update && apt-get install -y python3 python3-pip python3-venv \
    && rm -rf /var/lib/apt/lists/*
{{end}}
# Install GitHub CLI (gh)
RUN curl -fsSL https://cli.github.com/packages/githubcli-archive-keyring.gpg \
    | dd of=/usr/share/keyrings/githubcli-archive-keyring.gpg \
    && chmod go+r /usr/share/keyrings/githubcli-archive-keyring.gpg \
    && echo "deb [arch=$(dpkg --print-architecture) signed-by=/usr/share/keyrings/githubcli-archive-keyring.gpg] https://cli.github.com/packages stable main" \
    | tee /etc/apt/sources.list.d/github-cli.list > /dev/null \
    && apt-get update && apt-get install -y gh \
    && rm -rf /var/lib/apt/lists/*

# Install Claude Code globally via npm (single installation method).
# Do NOT symlink to ~/.local/bin — that would create a second "native" installation
# that conflicts with the npm-global one and breaks auto-update detection.
RUN npm install -g @anthropic-ai/claude-code
{{if or .MCPFilesystem .MCPMemory}}
# Install MCP servers (filesystem, memory)
RUN npm install -g{{if .MCPFilesystem}} \
    @modelcontextprotocol/server-filesystem{{end}}{{if .MCPMemory}} \
    @modelcontextprotocol/server-memory{{end}}
{{end}}{{if .MCPFetch}}
# Install MCP fetch server (Python-based)
RUN pip3 install --break-system-packages mcp-server-fetch
{{end}}
# Install ccstatusline (optional — native build may fail on some architectures)
RUN npm install -g ccstatusline || true
{{if .Go}}
# Install gopls (Go language server for LSP support in Claude Code)
RUN GOPATH=/tmp/gopls-build go install golang.org/x/tools/gopls@latest \
    && mv /tmp/gopls-build/bin/gopls /usr/local/bin/gopls \
    && rm -rf /tmp/gopls-build
{{end}}
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
{{if .Rust}}
# Rust for sandbox user
RUN mkdir -p /home/sandbox/.cargo /home/sandbox/.rustup \
    && cp -r /root/.cargo /home/sandbox/.cargo \
    && cp -r /root/.rustup /home/sandbox/.rustup \
    && chown -R sandbox:sandbox /home/sandbox
{{end}}
# Allow sandbox user to update npm global packages (Claude Code auto-updates).
# npm global prefix is /usr/lib (node_modules) + /usr/bin (binaries).
RUN chown -R sandbox:sandbox /usr/lib/node_modules \
    && chown sandbox:sandbox /usr/bin/claude /usr/bin/npx /usr/bin/npm 2>/dev/null || true

# Pre-create npm and pip cache dirs owned by sandbox
RUN mkdir -p /home/sandbox/.npm /home/sandbox/.cache/pip \
    && chown -R sandbox:sandbox /home/sandbox/.npm /home/sandbox/.cache/pip

ENV HOME=/home/sandbox
ENV PATH="/home/sandbox/.cargo/bin:/usr/local/go/bin:${PATH}"

WORKDIR /workspace

ENTRYPOINT ["/usr/local/bin/claustro-init"]
CMD ["sleep", "infinity"]
`

var parsedDockerfileTemplate = template.Must(template.New("Dockerfile").Parse(dockerfileTemplate))

// RenderDockerfile renders the Dockerfile template using the given ImageBuildConfig.
func RenderDockerfile(cfg *config.ImageBuildConfig) (string, error) {
	data := templateData{
		Go:            cfg.IsLanguageEnabled("go"),
		Rust:          cfg.IsLanguageEnabled("rust"),
		Python:        cfg.IsLanguageEnabled("python"),
		DevTools:      cfg.IsToolGroupEnabled("dev"),
		BuildTools:    cfg.IsToolGroupEnabled("build"),
		MCPFilesystem: cfg.IsMCPServerEnabled("filesystem"),
		MCPMemory:     cfg.IsMCPServerEnabled("memory"),
		MCPFetch:      cfg.IsMCPServerEnabled("fetch"),
	}

	var buf bytes.Buffer
	if err := parsedDockerfileTemplate.Execute(&buf, data); err != nil {
		return "", err
	}
	return buf.String(), nil
}
