#!/bin/bash
# The primary lifecycle-binding mechanism: fires when this session ends and
# stops the ralph-loop run this session owns, so it can never keep running
# (and costing money) unnoticed after the user thinks they've left.
#
# Reads session_id from the hook's stdin JSON and compares it against the
# state file's SESSION_ID, so a *different*, still-open session's loop is
# left alone — but fails safe TOWARD KILLING when either id is unknown,
# since leaking a running process silently is worse than an extra kill.
set -uo pipefail

INPUT=$(cat) # hook JSON on stdin
HOOK_SESSION_ID=$(printf '%s' "$INPUT" | sed -n 's/.*"session_id"[[:space:]]*:[[:space:]]*"\([^"]*\)".*/\1/p')

[[ -e "$PWD/.git" ]] || exit 0
STATE="$PWD/.claude/ralph/state"
[[ -f "$STATE" ]] || exit 0

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
source "${CLAUDE_PLUGIN_ROOT:-$SCRIPT_DIR/..}/scripts/ralph-lib.sh"

PID=$(ralph_state_get PID "$STATE")
OWNER_SESSION=$(ralph_state_get SESSION_ID "$STATE")

# Only skip if BOTH ids are known and clearly differ.
if [[ -n "$OWNER_SESSION" && -n "$HOOK_SESSION_ID" && "$OWNER_SESSION" != "$HOOK_SESSION_ID" ]]; then
  exit 0
fi

if ! ralph_pid_alive "$PID"; then
  rm -f "$STATE"
  exit 0
fi

kill -TERM "$PID" 2>/dev/null || true
for _ in 1 2 3 4 5 6; do # ~3s grace period
  ralph_pid_alive "$PID" || break
  sleep 0.5
done
if ralph_pid_alive "$PID"; then
  kill -KILL "$PID" 2>/dev/null || true
fi
rm -f "$STATE"
exit 0
