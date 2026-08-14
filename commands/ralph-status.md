---
description: "One-shot status check of the ralph-loop run for this repo"
allowed-tools: ["Bash", "Agent"]
---

# /ralph-status

Run:
```
bash "${CLAUDE_PLUGIN_ROOT}/scripts/ralph-status.sh"
```

Parse the `RALPH_*=value` lines, then report:
- `RALPH_STATUS=NONE` — "No active ralph loop for this repo."
- `RALPH_STATUS=RUNNING` — pid, iterations completed, current iteration if
  in progress, `RALPH_LAST_SUMMARY`.
- `RALPH_STATUS=STOPPED` with a completion-promise reason — healthy; report
  it plainly, do not invoke the diagnose agent.
- `RALPH_STATUS=STOPPED` with any other reason (stall/budget/error/
  preflight), or `RALPH_STATUS=CRASHED` — invoke the `ralph-diagnose`
  subagent via `Agent` (pass it `RALPH_LOG_DIR`/`RALPH_STDOUT_LOG`) instead
  of dumping raw logs yourself.

Never starts, stops, or modifies anything.
