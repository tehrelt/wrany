#!/usr/bin/env bash

set -euo pipefail

BRANCH="$(git branch --show-current 2>/dev/null || true)"

if [[ "$BRANCH" =~ ^epic/[0-9]{2}-[a-z0-9-]+$ ]]; then
  exit 0
fi

jq -n --arg branch "$BRANCH" '{
  hookSpecificOutput: {
    hookEventName: "PreToolUse",
    permissionDecision: "deny",
    permissionDecisionReason: "Blocked: production code changes are allowed only inside epic branches. Current branch: " + $branch + ". Create or switch to epic/<number>-<name> first."
  }
}'