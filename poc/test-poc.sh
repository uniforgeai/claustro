#!/usr/bin/env bash
set -euo pipefail

CONTAINER_NAME="claustro-poc"
IMAGE_NAME="claustro-poc"
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
PROJECT_DIR="$(dirname "$SCRIPT_DIR")"

GREEN='\033[0;32m'
RED='\033[0;31m'
YELLOW='\033[1;33m'
NC='\033[0m'

pass() { echo -e "  ${GREEN}✓${NC} $1"; }
fail() { echo -e "  ${RED}✗${NC} $1"; }
info() { echo -e "  ${YELLOW}→${NC} $1"; }

echo ""
echo "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"
echo "  claustro POC — Docker + Claude Code spike"
echo "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"
echo ""

# --- Build image ---
echo "[ 1/3 ] Building image $IMAGE_NAME ..."
docker build -t "$IMAGE_NAME" "$SCRIPT_DIR"
echo ""

# --- Clean up any previous run ---
if docker ps -a --format '{{.Names}}' | grep -q "^${CONTAINER_NAME}$"; then
    info "Removing previous container $CONTAINER_NAME"
    docker rm -f "$CONTAINER_NAME" >/dev/null
fi

# --- Start container ---
echo "[ 2/3 ] Starting container $CONTAINER_NAME ..."
docker run -d \
    --name "$CONTAINER_NAME" \
    --mount "type=bind,source=${PROJECT_DIR},target=/workspace" \
    --mount "type=bind,source=${HOME}/.claude,target=/home/sandbox/.claude" \
    --entrypoint sleep \
    "$IMAGE_NAME" infinity
echo ""

# --- Run checks ---
echo "[ 3/3 ] Automated checks:"
echo ""

run_check() {
    local label="$1"
    local cmd="$2"
    local expected="$3"

    result=$(docker exec "$CONTAINER_NAME" sh -c "$cmd" 2>&1 || true)
    if echo "$result" | grep -q "$expected"; then
        pass "$label → $result"
    else
        fail "$label → got: '$result' (expected to contain: '$expected')"
    fi
}

run_check "user is 'sandbox' (uid 9999)" "id"                    "9999"
run_check "HOME is /home/sandbox"     "echo \$HOME"               "/home/sandbox"
run_check "Node.js installed"         "node --version"            "v"
run_check "npm installed"             "npm --version"             "."
run_check "Claude Code installed"     "claude --version"          "."
run_check "/workspace mounted"        "ls /workspace"             "poc"
run_check "~/.claude mounted"         "ls /home/sandbox/.claude"  "."
run_check "git installed"             "git --version"             "git"
run_check "ripgrep installed"         "rg --version"              "ripgrep"
run_check "zsh installed"             "zsh --version"             "zsh"

echo ""
echo "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"
echo "  Manual tests — container is still running"
echo "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"
echo ""
echo "  Open a shell:"
echo "    docker exec -it $CONTAINER_NAME zsh"
echo ""
echo "  Inside the container, test:"
echo "    claude --version               # Claude Code works"
echo "    ls ~/.claude                   # config visible"
echo "    ls ~/.claude/projects/         # project state visible"
echo "    echo \$HOME                     # should be /home/sandbox"
echo ""
echo "  Start Claude Code (uses your active subscription):"
echo "    claude"
echo ""
echo "  Project path mapping test:"
echo "    # Note the absolute path inside container is /workspace"
echo "    # On the host it's ${PROJECT_DIR}"
echo "    # Check if Claude Code recognizes plans/sessions from host"
echo "    ls ~/.claude/projects/         # look for matching entry"
echo ""
echo "  Persistence test:"
echo "    # 1. Create a plan inside the container: claude (then /plan ...)"
echo "    # 2. Exit Claude, exit shell"
echo "    # 3. docker rm -f $CONTAINER_NAME"
echo "    # 4. Re-run this script"
echo "    # 5. Check if plan is still in ~/.claude/projects/"
echo ""
echo "  When done:"
echo "    docker rm -f $CONTAINER_NAME"
echo ""
