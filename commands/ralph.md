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

`ralph-loop` is always started with `--prompt-file .claude/ralph/prompt.md`
— never a raw `--prompt` literal, and never a `--prompt-file` pointing
anywhere else. Every input path below ends by getting the prompt content
into that one file (`mkdir -p .claude/ralph` first if needed).

- If the text after `/ralph` starts with `--`, treat it as **raw flag
  passthrough** to `ralph-loop` (e.g. `--prompt-file PLAN.md
  --max-iterations 10`) — quote each value correctly yourself, then:
  - If the flags include `--prompt-file PATH`: copy that file's contents
    into the canonical location (`cp PATH .claude/ralph/prompt.md`), then
    drop `--prompt-file PATH` from the flags you'll pass in step 2 and
    substitute `--prompt-file .claude/ralph/prompt.md`.
  - If the flags include `--prompt "..."` (a literal inline prompt
    string): write that text verbatim into `.claude/ralph/prompt.md` via
    the quoted heredoc below, then likewise drop `--prompt "..."` from the
    flags and substitute `--prompt-file .claude/ralph/prompt.md`.
  - Any other flags (`--max-iterations`, `--branch`, etc.) pass through to
    step 2 unchanged.
- Otherwise, if the text after `/ralph` is non-empty, treat the **entire**
  text as free-form prompt text. Write it verbatim, unmodified, via a
  quoted heredoc so nothing in it gets shell-expanded:
  ```
  mkdir -p .claude/ralph
  cat > .claude/ralph/prompt.md <<'RALPH_PROMPT_EOF'
  <exact user text>
  RALPH_PROMPT_EOF
  ```
  then pass `--prompt-file .claude/ralph/prompt.md` in step 2.
- No argument text at all (bare `/ralph`, or only non-prompt flags like
  `--max-iterations 10`): if this session already produced an approved
  implementation plan before `/ralph` was invoked (e.g. via plan mode),
  write that plan's full text into `.claude/ralph/prompt.md` using the
  same quoted heredoc, and pass `--prompt-file .claude/ralph/prompt.md` in
  step 2.
- Still nothing to use as a prompt → ask the user for a prompt or
  `--prompt-file` path before continuing; don't guess.

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
