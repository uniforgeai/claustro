---
title: Installation
weight: 1
---

# Installation

## Homebrew (macOS / Linux)

```bash
brew tap uniforgeai/tap
brew install claustro
```

## Binary Download

Download the latest release from [GitHub Releases](https://github.com/uniforgeai/claustro/releases).

Available platforms:
- `linux/amd64`
- `linux/arm64`
- `darwin/amd64` (Intel Mac)
- `darwin/arm64` (Apple Silicon)

## From Source

```bash
go install github.com/uniforgeai/claustro/cmd/claustro@latest
```

## Prerequisites

- **Docker Engine** or **Docker Desktop** must be installed and running
- Run `claustro doctor` to verify your environment
