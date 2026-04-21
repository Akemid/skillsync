#!/usr/bin/env bash
# go-review-on-commit.sh
#
# PreToolUse hook: intercepts git commit attempts and runs Go quality checks.
# Blocks the commit if go vet or go fmt issues are found.
#
# Input:  JSON on stdin (agent hook payload)
# Output: JSON on stdout (permissionDecision)
# Exit 0: allow  |  Exit 2: block

set -euo pipefail

# ── Read hook payload ──────────────────────────────────────────────────────────
PAYLOAD=$(cat)
TOOL_NAME=$(echo "$PAYLOAD" | python3 -c "import sys,json; d=json.load(sys.stdin); print(d.get('toolName',''))" 2>/dev/null || echo "")
TOOL_INPUT=$(echo "$PAYLOAD" | python3 -c "import sys,json; d=json.load(sys.stdin); print(json.dumps(d.get('toolInput','')))" 2>/dev/null || echo "{}")

# ── Only act on shell/terminal tool calls that contain "git commit" ────────────
is_git_commit() {
  local input="$1"
  echo "$input" | grep -qE '"(git commit|git -c|git --no-pager commit)' 2>/dev/null && return 0
  # Also check command field directly
  echo "$input" | python3 -c "
import sys, json
d = json.loads(sys.stdin.read())
cmd = d.get('command', d.get('cmd', ''))
if isinstance(cmd, list):
    cmd = ' '.join(cmd)
print('yes' if 'git commit' in cmd else 'no')
" 2>/dev/null | grep -q "^yes$" && return 0
  return 1
}

# Not a git commit — allow immediately
if ! is_git_commit "$TOOL_INPUT"; then
  echo '{"continue": true}'
  exit 0
fi

# ── Run Go quality checks ──────────────────────────────────────────────────────
ISSUES=""

# 1. go vet
VET_OUTPUT=$(go vet ./... 2>&1) || {
  ISSUES="${ISSUES}\n[go vet]\n${VET_OUTPUT}"
}

# 2. go fmt (detect unformatted files without modifying them)
UNFORMATTED=$(gofmt -l . 2>/dev/null)
if [ -n "$UNFORMATTED" ]; then
  ISSUES="${ISSUES}\n[go fmt] The following files are not formatted:\n$(echo "$UNFORMATTED" | sed 's/^/  /')"
fi

# 3. go build (compile check — catches type errors and missing symbols)
BUILD_OUTPUT=$(go build ./... 2>&1) || {
  ISSUES="${ISSUES}\n[go build]\n${BUILD_OUTPUT}"
}

# ── Decide ─────────────────────────────────────────────────────────────────────
if [ -n "$ISSUES" ]; then
  REASON=$(printf "Go quality checks failed — fix these issues before committing:\n%b\n\nRun the @Go Code Reviewer agent for a detailed review." "$ISSUES")
  python3 -c "
import json, sys
print(json.dumps({
  'hookSpecificOutput': {
    'hookEventName': 'PreToolUse',
    'permissionDecision': 'deny',
    'permissionDecisionReason': sys.argv[1]
  }
}))
" "$REASON"
  exit 2
fi

# All checks passed — allow the commit
python3 -c "
import json
print(json.dumps({
  'hookSpecificOutput': {
    'hookEventName': 'PreToolUse',
    'permissionDecision': 'allow',
    'permissionDecisionReason': 'go vet, go fmt, and go build passed.'
  }
}))
"
exit 0
