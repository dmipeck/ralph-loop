#!/bin/bash
# Starts (or reports reattachment to) a ralph-loop run for the repo rooted
# at $PWD. Invoked by commands/ralph.md. Args: passed straight through to
# `ralph-loop` (e.g. --prompt-file X --max-iterations 10).
set -uo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
source "$SCRIPT_DIR/ralph-lib.sh"

if ! ralph_require_repo; then
  ralph_emit RESULT FAILED_TO_START
  exit 1
fi

STATE="$(ralph_state_path)"

if [[ -f "$STATE" ]]; then
  PID=$(ralph_state_get PID "$STATE")
  if ralph_pid_alive "$PID"; then
    ralph_emit RESULT ALREADY_RUNNING
    ralph_emit PID "$PID"
    ralph_emit LOG_DIR "$(ralph_state_get LOG_DIR "$STATE")"
    ralph_emit STDOUT_LOG "$(ralph_state_get STDOUT_LOG "$STATE")"
    ralph_emit STATE_FILE "$STATE"
    exit 0
  fi
  # Stale: the recorded process is gone. Report its last known line, then
  # clear the state file and fall through to a fresh launch.
  OLD_LOG_DIR=$(ralph_state_get LOG_DIR "$STATE")
  if [[ -n "$OLD_LOG_DIR" && -f "$OLD_LOG_DIR/ralph.log" ]]; then
    echo "Previous run's last recorded line: $(tail -1 "$OLD_LOG_DIR/ralph.log")"
  fi
  rm -f "$STATE"
  STALE=1
fi

BINARY=$(ralph_find_binary) || { ralph_emit RESULT FAILED_TO_START; exit 1; }

mkdir -p "$RALPH_DIR"
ralph_ensure_exclude

# If the caller already passed --log-dir, respect it and don't add another.
LOG_DIR=""
ARGS=("$@")
for ((i = 0; i < ${#ARGS[@]}; i++)); do
  if [[ "${ARGS[$i]}" == "--log-dir" && $((i + 1)) -lt ${#ARGS[@]} ]]; then
    LOG_DIR="${ARGS[$((i + 1))]}"
    break
  elif [[ "${ARGS[$i]}" == --log-dir=* ]]; then
    LOG_DIR="${ARGS[$i]#--log-dir=}"
    break
  fi
done
if [[ -z "$LOG_DIR" ]]; then
  LOG_DIR="$(ralph_default_log_dir)"
  ARGS+=(--log-dir "$LOG_DIR")
fi

mkdir -p "$LOG_DIR"
STDOUT_LOG="$LOG_DIR/run.stdout.log"

# ralph.log is append-only across every historical run against this repo
# (matching the bare CLI's own behavior — it's never truncated), so its
# total line count is not "iterations completed by THIS run." Record the
# line count as it stands right now as a baseline; ralph-status.sh
# subtracts it back out to get a per-run count.
LOG_BASELINE=0
[[ -f "$LOG_DIR/ralph.log" ]] && LOG_BASELINE=$(wc -l <"$LOG_DIR/ralph.log" | tr -d ' ')

if command -v git >/dev/null 2>&1 && [[ -n "$(git status --porcelain 2>/dev/null)" ]]; then
  echo "Warning: this repo has uncommitted changes — ralph-loop will start iterating on top of them."
fi

"$BINARY" "${ARGS[@]}" >"$STDOUT_LOG" 2>&1 </dev/null &
RALPH_PID=$!

sleep 1
if ! kill -0 "$RALPH_PID" 2>/dev/null || grep -q '^Error:' "$STDOUT_LOG" 2>/dev/null; then
  ralph_emit RESULT FAILED_TO_START
  echo "--- $STDOUT_LOG ---"
  tail -n 20 "$STDOUT_LOG" 2>/dev/null
  exit 1
fi

SESSION_ID="${CLAUDE_CODE_SESSION_ID:-}"
STARTED_AT="$(date -u +%Y-%m-%dT%H:%M:%SZ)"

{
  echo "PID=$RALPH_PID"
  echo "LOG_DIR=$LOG_DIR"
  echo "STDOUT_LOG=$STDOUT_LOG"
  echo "REPO_DIR=$PWD"
  echo "STARTED_AT=$STARTED_AT"
  echo "BINARY=$BINARY"
  echo "SESSION_ID=$SESSION_ID"
  echo "LOG_BASELINE=$LOG_BASELINE"
} >"$STATE.tmp"
mv "$STATE.tmp" "$STATE"

if [[ "${STALE:-0}" == "1" ]]; then
  ralph_emit RESULT STALE_CLEARED_THEN_STARTED
else
  ralph_emit RESULT STARTED
fi
ralph_emit PID "$RALPH_PID"
ralph_emit LOG_DIR "$LOG_DIR"
ralph_emit STDOUT_LOG "$STDOUT_LOG"
ralph_emit STATE_FILE "$STATE"
