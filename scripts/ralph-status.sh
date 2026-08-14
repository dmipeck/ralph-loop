#!/bin/bash
# One-shot, read-only status check for the ralph-loop run in this repo.
# Never starts, stops, or modifies anything (including the state file).
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
STDOUT_LOG=$(ralph_state_get STDOUT_LOG "$STATE")
STARTED_AT=$(ralph_state_get STARTED_AT "$STATE")
LOG_BASELINE=$(ralph_state_get LOG_BASELINE "$STATE")
[[ -z "$LOG_BASELINE" ]] && LOG_BASELINE=0

if ralph_pid_alive "$PID"; then
  ITERATIONS_COMPLETED=0
  LAST_SUMMARY=""
  if [[ -f "$LOG_DIR/ralph.log" ]]; then
    # ralph.log is append-only across every historical run against this
    # repo, so subtract back out the line count recorded at this run's
    # start (LOG_BASELINE) to get iterations completed by THIS run.
    TOTAL_LINES=$(wc -l <"$LOG_DIR/ralph.log" | tr -d ' ')
    ITERATIONS_COMPLETED=$((TOTAL_LINES - LOG_BASELINE))
    LAST_SUMMARY=$(tail -1 "$LOG_DIR/ralph.log")
  fi

  CURRENT_ITERATION=""
  if [[ -f "$STDOUT_LOG" ]]; then
    LAST_STARTED=$(grep -oE '^── Iteration [0-9]+ ──' "$STDOUT_LOG" 2>/dev/null | grep -oE '[0-9]+' | tail -1)
    if [[ -n "$LAST_STARTED" && "$LAST_STARTED" -gt "$ITERATIONS_COMPLETED" ]]; then
      CURRENT_ITERATION="$LAST_STARTED"
    fi
  fi

  ralph_emit STATUS RUNNING
  ralph_emit PID "$PID"
  ralph_emit STARTED_AT "$STARTED_AT"
  ralph_emit ITERATIONS_COMPLETED "$ITERATIONS_COMPLETED"
  ralph_emit CURRENT_ITERATION "$CURRENT_ITERATION"
  ralph_emit LAST_SUMMARY "$LAST_SUMMARY"
  ralph_emit LOG_DIR "$LOG_DIR"
  ralph_emit STDOUT_LOG "$STDOUT_LOG"
else
  CLASS=$(ralph_classify_terminal "$STDOUT_LOG")
  LAST_SUMMARY=""
  [[ -f "$LOG_DIR/ralph.log" ]] && LAST_SUMMARY=$(tail -1 "$LOG_DIR/ralph.log")

  if [[ "$CLASS" == "CRASHED" ]]; then
    ralph_emit STATUS CRASHED
  else
    ralph_emit STATUS STOPPED
    ralph_emit STOP_CLASS "$CLASS"
  fi
  ralph_emit LAST_SUMMARY "$LAST_SUMMARY"
  ralph_emit LOG_DIR "$LOG_DIR"
  ralph_emit STDOUT_LOG "$STDOUT_LOG"
fi
