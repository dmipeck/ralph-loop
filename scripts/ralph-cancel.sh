#!/bin/bash
# Stops the active ralph-loop run for this repo, gracefully (SIGTERM only —
# never auto-SIGKILL; a mid-commit `claude` process deserves the chance to
# finish cleanly while the user is right here to check again).
set -uo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
source "$SCRIPT_DIR/ralph-lib.sh"

if ! ralph_require_repo; then
  ralph_emit STATUS NONE
  exit 0
fi

STATE="$(ralph_state_path)"
if [[ ! -f "$STATE" ]]; then
  ralph_emit STATUS NONE
  exit 0
fi

PID=$(ralph_state_get PID "$STATE")
LOG_DIR=$(ralph_state_get LOG_DIR "$STATE")

if ! ralph_pid_alive "$PID"; then
  LAST_SUMMARY=""
  [[ -f "$LOG_DIR/ralph.log" ]] && LAST_SUMMARY=$(tail -1 "$LOG_DIR/ralph.log")
  rm -f "$STATE"
  ralph_emit STATUS ALREADY_STOPPED
  ralph_emit LAST_SUMMARY "$LAST_SUMMARY"
  exit 0
fi

kill -TERM "$PID" 2>/dev/null || true
for _ in 1 2 3 4 5 6 7 8 9 10; do
  ralph_pid_alive "$PID" || break
  sleep 0.5
done

if ralph_pid_alive "$PID"; then
  ralph_emit STATUS CANCEL_PENDING
  ralph_emit PID "$PID"
else
  rm -f "$STATE"
  ralph_emit STATUS CANCELLED
  ralph_emit PID "$PID"
fi
