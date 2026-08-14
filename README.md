# ralph-loop

A small CLI implementing the [Ralph Wiggum technique](https://ghuntley.com/ralph/):
repeatedly run a fresh, non-continued `claude -p` process against a fixed
prompt until the work is done. Each iteration has zero memory of the last —
the target git repository's files and commit history are the only state
that persists between iterations.

```
while :; do
  claude -p "$(cat PROMPT.md)"
  # look at what changed, decide whether to keep going
done
```

`ralph-loop` is that loop, plus the bookkeeping needed to run it unattended:
live streaming of what claude is doing, per-iteration logs, commit
detection, a completion-promise check, and stall/cost/iteration limits so a
stuck run doesn't burn money forever.

## Build

```
go build -o ralph-loop .
```

## Usage

Run it from inside the target git repo — there's no `--repo-dir`, it always
operates on the current directory. Supply the prompt either as a literal
string or a path to a file:

```
cd /path/to/project

# See what would run without spending anything
ralph-loop --prompt "Implement X, verify with \`go test ./...\`" --dry-run

# ...or with a prompt already written to a file, re-read fresh each iteration
ralph-loop --prompt-file PLAN.md --dry-run

# Run for real, with sensible safety limits
ralph-loop --prompt-file PLAN.md \
  --max-iterations 40 \
  --max-stalled-iterations 3 \
  --max-total-cost-usd 20 \
  --branch ralph/my-feature \
  --preflight-cmd "mysqladmin ping"
```

`--prompt`/`--prompt-file` are the only required flags (exactly one of the
two). Everything else — including the completion-promise phrase claude is
told to output when done — has a built-in default, so no other setup is
needed. Run `ralph-loop --help` for the full flag list. Notable ones:

- `--allowed-tool` (repeatable) adds to a generic baseline (`Read`, `Write`,
  `Edit`, `Glob`, `Grep`, `Bash(git *)`) — pass project-specific commands
  (a test runner, a build tool, ...) here rather than expecting the tool to
  know about your stack.
- `--preflight-cmd` runs before every iteration and must exit 0, or the run
  aborts before spending anything — e.g. checking a dev database is
  reachable.
- `--branch` checks out (creating if needed) a dedicated branch before the
  loop starts, done directly with your own git permissions, so an unattended
  run never lands commits straight onto `main`.
- `--dangerously-skip-permissions` bypasses claude's permission system
  entirely — the documented escape hatch if `--permission-mode`/
  `--allowed-tool` prove too restrictive for a given project.

## Logs

Each iteration writes `iteration-<n>.jsonl` (the full raw NDJSON stream) and
`iteration-<n>.stderr.log` to `--log-dir` (default: a subdirectory of the OS
cache dir, deliberately *outside* the target repo so logs never risk being
accidentally committed into it), plus one line per iteration to
`ralph.log` there.
