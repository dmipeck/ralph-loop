---
description: "Stop the active ralph-loop run for this repo"
allowed-tools: ["Bash", "TaskStop"]
---

# /ralph-cancel

Run:
```
bash "${CLAUDE_PLUGIN_ROOT}/scripts/ralph-cancel.sh"
```

Parse `RALPH_*=value`, then report:
- `RALPH_STATUS=NONE` — "No active ralph loop found — nothing to cancel."
- `RALPH_STATUS=ALREADY_STOPPED` — already finished; report last summary.
- `RALPH_STATUS=CANCELLED` — stopped gracefully (SIGTERM).
- `RALPH_STATUS=CANCEL_PENDING` — signal sent, hadn't exited within the
  grace period yet (normal mid-iteration). Suggest re-running
  `/ralph-cancel` shortly. Only mention force-killing if the user
  explicitly asks — never do it unprompted.

If a `Monitor` from an earlier `/ralph` is still active this session, stop
it with `TaskStop` once cancellation is confirmed.
