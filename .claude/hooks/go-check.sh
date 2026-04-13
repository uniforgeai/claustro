#!/usr/bin/env bash
# This file is licensed under the Business Source License 1.1. See the LICENSE file for details.
# Copyright (c) 2026 Uniforge GmbH. All rights reserved.
#
# PostToolUse hook: runs Go build, test, and lint after .go file edits.
# Returns errors as additionalContext so Claude can auto-fix them.

set -euo pipefail

FILE=$(jq -r '.tool_input.file_path // .tool_response.filePath // empty' < /dev/stdin)

# Skip non-Go files
if [[ ! "$FILE" =~ \.go$ ]]; then
  exit 0
fi

cd /workspace

ERRORS=""

# Run go build
BUILD_OUT=$(go build ./... 2>&1) || ERRORS="${ERRORS}

## go build errors:
${BUILD_OUT}"

# Run go test (only if build passed — tests won't compile otherwise)
if [[ -z "$ERRORS" ]]; then
  TEST_OUT=$(go test ./... 2>&1) || ERRORS="${ERRORS}

## go test errors:
${TEST_OUT}"
fi

# Run golangci-lint (if available)
if command -v golangci-lint &>/dev/null; then
  LINT_OUT=$(golangci-lint run 2>&1) || ERRORS="${ERRORS}

## golangci-lint errors:
${LINT_OUT}"
elif [[ -x "$HOME/go/bin/golangci-lint" ]]; then
  LINT_OUT=$("$HOME/go/bin/golangci-lint" run 2>&1) || ERRORS="${ERRORS}

## golangci-lint errors:
${LINT_OUT}"
fi

if [[ -n "$ERRORS" ]]; then
  # Output JSON that Claude Code will parse — additionalContext gets injected into the model context
  jq -n --arg ctx "$ERRORS" '{
    "hookSpecificOutput": {
      "hookEventName": "PostToolUse",
      "additionalContext": ("Go checks failed. Fix these errors before continuing:\n" + $ctx)
    }
  }'
fi
