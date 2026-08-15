---
description: "Start a ralph-loop run against this repo, or reattach to one already running, then begin live progress reporting"
argument-hint: "<free text prompt> | --prompt-file PATH [ralph-loop flags...]"
allowed-tools: ["Bash", "Monitor"]
---

# /ralph

Start (or reattach to) a `ralph-loop` run for the git repository rooted at
the current working directory, then keep reporting its progress live for
the rest of this session.

`ralph-loop` runs as a real background OS process, but its lifecycle is
**bound to this Claude Code session**: if this session ends, the loop (and
any `claude -p` call it currently has in flight) is stopped automatically —
it will not keep running, and will not keep costing money, after you leave.
Only one loop may be active per repo; if one is already running (e.g.
started from another still-open session), this reattaches instead of
starting a second one.

## 1. Resolve arguments

- If the text after `/ralph` starts with `--`, treat it as **raw flag
  passthrough** to `ralph-loop` (e.g. `--prompt-file PLAN.md
  --max-iterations 10`) — quote each value correctly yourself.
- Otherwise treat the **entire** text as free-form prompt text. Write it
  verbatim, unmodified, via a quoted heredoc so nothing in it gets shell-
  expanded:
  ```
  mkdir -p .claude/ralph
  cat > .claude/ralph/pending-prompt.txt <<'RALPH_PROMPT_EOF'
  <exact user text>
  RALPH_PROMPT_EOF
  ```
  then pass `--prompt-file .claude/ralph/pending-prompt.txt`.
- No argument text at all → ask the user for a prompt or `--prompt-file`
  path before continuing; don't guess.

## 2. Run the start script

```
bash "${CLAUDE_PLUGIN_ROOT}/scripts/ralph-start.sh" <flags from step 1>
```

Read the trailing `RALPH_*=value` lines: `RALPH_RESULT`, `RALPH_PID`,
`RALPH_LOG_DIR`, `RALPH_STDOUT_LOG`, `RALPH_STATE_FILE`.

- `RALPH_RESULT=ALREADY_RUNNING` — reattaching to an existing loop; report
  its pid/log dir/started-at, do **not** start a new one. Go to step 3.
- `RALPH_RESULT=STARTED` or `STALE_CLEARED_THEN_STARTED` — fresh launch.
  Relay any uncommitted-changes warning verbatim. Go to step 3.
- `RALPH_RESULT=FAILED_TO_START` — report the printed reason plainly and
  stop; do not proceed to step 3.

## 3. Start live progress reporting

Call `Monitor` with `persistent: true` (a run can take a long time; a
non-persistent Monitor would be killed by its timeout while the loop is
still going) and:
- `description`: e.g. "ralph loop iterations for <repo name>"
- `command`: `bash "${CLAUDE_PLUGIN_ROOT}/scripts/ralph-monitor.sh" "<RALPH_STATE_FILE>"`

Remind the user this loop's lifecycle is tied to this session: it stops
automatically if this session ends — it's not a fire-and-forget background
job that outlives the console.

Each `Monitor` notification is one raw line from the loop's own log —
narrate it in plain language rather than relaying it verbatim. Use this
mapping:

- `RALPH_ITERATION_START iter=N total_cost_so_far=$X` — "Starting
  iteration N (~$X spent so far)."
- `RALPH_PLAN iter=N: <text>` — "Ralph's plan for iteration N: `<text>`"
- `RALPH_CHANGES iter=N commit=<sha> subject="<msg>" diffstat="<stat>" tag="<tag>"` —
  "Iteration N changed: `<msg>` (`<stat>`)", tagged `<tag>` if present.
- The `... | iter=N | is_error=... | cost=$X | committed=... |
  total_cost=$Y | subject="<msg>"` summary line — "Iteration N complete:
  {no errors|error}, cost $X, committed as `<sha>` (`<msg>`). Running
  total: N iterations, ~$Y. Still watching."
- Anything else (crash/heartbeat/stop lines) — relay plainly, no need to
  reformat.
