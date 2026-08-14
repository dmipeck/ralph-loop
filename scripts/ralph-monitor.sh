#!/bin/bash
# Run by the Monitor tool's `command` (see commands/ralph.md step 3). Each
# stdout line becomes a chat notification, so this tails only structural
# markers — iteration-start banners, the FormatSummaryLine summary shape,
# and the loop's own terminal/error strings — never the full per-tool-call
# renderer stream. Explicitly covers crash/hang paths, not just the happy
# path: silent process death with no clean stop message is reported as a
# crash, and a long silence while still running gets a heartbeat line.
#
# Arg: path to the ralph state file.
set -uo pipefail

STATE_FILE="$1"
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
source "$SCRIPT_DIR/ralph-lib.sh"

PID=$(ralph_state_get PID "$STATE_FILE")
STDOUT_LOG=$(ralph_state_get STDOUT_LOG "$STATE_FILE")
if [[ -z "$PID" || -z "$STDOUT_LOG" ]]; then
  echo "ERROR: could not read $STATE_FILE"
  exit 1
fi

echo "Watching ralph loop (pid $PID) — $STDOUT_LOG"
for _ in $(seq 1 20); do
  [[ -f "$STDOUT_LOG" ]] && break
  sleep 0.25
done

tail -n +1 -F "$STDOUT_LOG" 2>/dev/null | grep -E --line-buffered \
  '^── Iteration [0-9]+ ──|^[0-9-]+T[0-9:]+Z \| iter=|^Ralph loop stopped after|^Interrupted (before|after) iteration|claude exited with an error|produced no result event|Preflight command failed' &
TAIL_PID=$!

LAST_HEARTBEAT=$(date +%s)
while true; do
  if ! kill -0 "$PID" 2>/dev/null; then
    sleep 1 # let the tail|grep pipeline flush any trailing buffered lines
    kill "$TAIL_PID" 2>/dev/null || true
    CLASS=$(ralph_classify_terminal "$STDOUT_LOG")
    if [[ "$CLASS" == "CRASHED" ]]; then
      echo "Ralph loop (pid $PID) is no longer running with no clean stop message — likely a crash. Last lines:"
      tail -n 5 "$STDOUT_LOG"
    else
      echo "Ralph loop (pid $PID) finished ($CLASS)."
    fi
    echo "RALPH_MONITOR_DONE=1"
    exit 0
  fi
  NOW=$(date +%s)
  MTIME=$(ralph_mtime "$STDOUT_LOG")
  if [[ -n "$MTIME" ]] && ((NOW - MTIME > 600)) && ((NOW - LAST_HEARTBEAT > 600)); then
    echo "Still running (pid $PID), no new iteration events in ~10m — check $STDOUT_LOG if this seems hung."
    LAST_HEARTBEAT=$NOW
  fi
  sleep 2
done
