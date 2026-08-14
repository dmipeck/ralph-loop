#!/bin/bash
# Non-destructive backstop for anything the SessionEnd hook didn't catch
# (hard crash, force-quit, machine sleep). A fresh session can't reliably
# tell "orphan left by a crash" apart from "legitimately owned by another,
# still-open session" from PID liveness alone, so this only ever REPORTS
# and points at /ralph-status or /ralph-cancel — it never auto-kills.
set -uo pipefail

cat >/dev/null # drain stdin, must not block

[[ -e "$PWD/.git" ]] || exit 0
STATE="$PWD/.claude/ralph/state"
[[ -f "$STATE" ]] || exit 0

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
source "${CLAUDE_PLUGIN_ROOT:-$SCRIPT_DIR/..}/scripts/ralph-lib.sh"

PID=$(ralph_state_get PID "$STATE")
LOG_DIR=$(ralph_state_get LOG_DIR "$STATE")
STARTED=$(ralph_state_get STARTED_AT "$STATE")

if ralph_pid_alive "$PID"; then
  MSG="A ralph-loop process appears to still be running (pid $PID, started $STARTED, logs $LOG_DIR) — from another still-open session, or left over from one that didn't shut down cleanly. Run /ralph-status to check, or /ralph-cancel to stop it."
else
  rm -f "$STATE"
  MSG="A leftover ralph-loop state file from a previous session was cleared (its process had already stopped)."
fi

esc() {
  local s=$1
  s=${s//\\/\\\\}
  s=${s//\"/\\\"}
  printf '%s' "$s"
}
printf '{"hookSpecificOutput":{"hookEventName":"SessionStart","additionalContext":"%s"}}\n' "$(esc "$MSG")"
exit 0
