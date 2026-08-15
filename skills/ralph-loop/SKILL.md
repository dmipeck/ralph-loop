---
name: ralph-loop
description: Drive the ralph-loop CLI, which runs the Ralph Wiggum technique — repeatedly invoking a fresh, non-continued `claude -p` against a fixed prompt until a plan is done or a safety limit trips. Use this whenever the user wants to run an unattended/autonomous Claude Code loop against a repo, supply a prompt via --prompt or --prompt-file, pick ralph-loop flags (allowed tools, budgets, branch, preflight), interpret a ralph-loop run's output or logs, or debug why a run stalled/aborted. Trigger on "ralph loop", "ralph-loop", "ralph wiggum technique", "unattended claude loop", "run claude in a loop until done", or requests to automate a multi-iteration Claude Code plan.
---

# ralph-loop

A CLI that runs the [Ralph Wiggum technique](https://ghuntley.com/ralph/): loop
a fresh `claude -p` process against one fixed prompt (a literal string or a
file), with no `--continue`/session state, until the work is done. Everything that
"remembers" progress between iterations is external to the process — the
target repo's files and `git log`. `ralph-loop` is that loop plus the
bookkeeping needed to run it unattended: live streaming, per-iteration logs,
commit detection, a completion-promise check, and stall/cost/iteration
limits.

This skill is about *driving the CLI* — running it against some target
project. It is not about editing ralph-loop's own Go source (that's just
normal code work in this repo).

## Mental model before touching flags

Every iteration is a brand-new `claude -p` process with zero memory of the
last one. The only things that persist are:

1. The target repo's **git history** (what got committed).
2. Whatever the target repo's **own plan/spec doc** (e.g. `PLAN.md`) says is
   done vs. remaining — the prompt should tell each iteration to read this
   and cross-reference it against `git log`.

So the prompt file matters more than any flag. If iterations don't converge,
the fix is almost always the prompt (ordering, scope-per-commit, what counts
as "done"), not a flag.

## Recommended project structure (optional)

ralph-loop doesn't require any particular file layout — the `--prompt`/
`--prompt-file` text and git history are the only hard requirements. Two
optional, purely-convention docs make longer runs easier for both the loop
and the human watching it, if the target project already uses (or wants to
adopt) them:

- **A tracked plan/todo doc** (e.g. `docs/ROADMAP.md`, `PLAN.md`) distinct
  from `--prompt-file` itself — a living, prioritized todo list. If the
  project prompt references one, the harness already tells each iteration
  to update it (mark off finished work, append newly discovered
  follow-up) alongside its commit.
- **A standing build/test-commands + learnings doc** (`CLAUDE.md`,
  `AGENT.md`, or similar) — durable facts a future iteration should know
  (how to build, quirks, gotchas), never a status/progress report. If
  present, the harness tells each iteration to treat it as authoritative
  and append new learnings to it.

Neither doc is scaffolded by ralph-loop itself; both are conventions the
harness knows to look for and update if present.

## Workflow

Run it from inside the target repo — there's no `--repo-dir`, it always
operates on the current directory (`cd` into the target project first).
Supply the prompt either as a literal string (`--prompt`) or a path to a
file (`--prompt-file`, re-read fresh at the start of every iteration, so it
can be edited between iterations). No separate scaffolding step is needed —
just write or paste the project's own goal/plan/verification text and go.

### 1. Dry-run before spending anything

```
cd /path/to/project
ralph-loop --prompt-file PLAN.md --dry-run
# or, for a short one-off prompt:
ralph-loop --prompt "Implement X, verify with \`go test ./...\`" --dry-run
```

Prints the config summary and the exact `claude ...` argv that would be
spawned, without invoking claude. Always do this first when flags are
non-trivial (custom allowed-tools, a preflight command, a budget) — it's
free and catches typos/contradictions before they cost anything.

### 2. Run for real, with limits set

```
ralph-loop --prompt-file PLAN.md \
  --max-iterations 40 \
  --max-stalled-iterations 3 \
  --max-total-cost-usd 20 \
  --branch ralph/my-feature \
  --preflight-cmd "mysqladmin ping"
```

`--prompt`/`--prompt-file` (exactly one of the two) is the only required
input. Everything else — including the completion-promise phrase claude is
told to output when genuinely done — has a built-in default (see below);
start from the defaults and only add what the target project actually
needs.

## Flag reference

Required — exactly one of these two:
- `--prompt` — the fixed prompt fed to every iteration, as a literal string.
- `--prompt-file` — path to a file containing that prompt; read fresh at the
  start of every iteration, so it can double as a live plan doc.

Stopping conditions (any one stops the loop; the config summary before the
run and `ralph.log` after show which one fired):
- `--max-iterations` (default 40, `0` = unlimited)
- `--max-stalled-iterations` (default 3, `0` = disabled) — aborts after this
  many iterations *in a row* with no commit and no completion promise. This
  is the guard against silently burning money on a stuck loop; don't set it
  to 0 unless something else is bounding cost.
- `--max-total-cost-usd` (default unset) — cumulative cost across
  iterations, tracked by ralph-loop itself from claude's reported cost.
- `--max-iteration-cost-usd` (default unset) — passed straight through to
  `claude -p --max-budget-usd`, a per-iteration cap enforced by claude.
- A genuine completion promise always wins over every other check, even one
  computed from the same iteration (e.g. it also happened to commit).

Permissions — pick **one** of these two modes (they're mutually exclusive
with each other and with `--dangerously-skip-permissions`):
- Default: `--permission-mode auto` plus `--allowed-tool` (repeatable),
  which is *added to* a generic baseline (`Read`, `Write`, `Edit`, `Glob`,
  `Grep`, `Bash(git *)`) — it does not replace it. Pass project-specific
  commands here: a test runner, a build tool, a linter. E.g.
  `--allowed-tool "Bash(npm test)" --allowed-tool "Bash(go build ./...)"`.
- `--disallowed-tool` (repeatable) to explicitly block something even if a
  pattern above would otherwise allow it.
- `--dangerously-skip-permissions` bypasses claude's permission system
  entirely. Only reach for this if `--permission-mode`/`--allowed-tool`
  prove too restrictive for a given project — it removes a real safety net
  on an unattended run, so prefer scoping `--allowed-tool` first.

Safety / environment:
- `--preflight-cmd` — shell command that must exit 0 before *every*
  iteration, or the run aborts before spawning claude at all (no cost
  spent). Use this for anything the plan depends on being reachable — a dev
  database, a local service.
- `--branch` — checks out (creating if needed) this branch before the loop
  starts, using your own git permissions, not claude's. Always set this for
  an unattended run so commits don't land straight on `main`/`master`.
- `--tag-on-commit` (default off) — after every successful commit, create a
  lightweight git tag (`ralph/iter-<n>-<sha>`) at HEAD — free, cheap rollback
  points. Purely additive and best-effort: a tag-creation failure never
  aborts the iteration or the run, it just prints a warning.

Other:
- `--completion-promise` (default: an internal phrase) — the exact phrase
  expected inside `<promise>...</promise>` in claude's final reply. Not
  something you normally need to set — ralph-loop's own harness (prepended
  to every prompt automatically) already tells claude to use the matching
  phrase. Only override this if a project's own text is likely to naturally
  contain the default phrase.
- `--model` — passed through to `claude -p --model`.
- `--sleep` (default 5s) — pause between iterations.
- `--log-dir` — default is a subdirectory of the OS cache dir keyed by repo
  name, deliberately *outside* the target repo so logs can't accidentally
  get committed into it. Override if you want logs somewhere specific.
- `-v` / `--verbose`.
- There is no `--repo-dir` — ralph-loop always operates on the current
  working directory, so `cd` into the target repo before running it.

Run `ralph-loop --help` for the authoritative, current list — flags can
drift from this summary.

## Reading output and logs

While running, ralph-loop streams what each iteration is doing live, then
prints one summary line per iteration and, at the end, why it stopped and
the total reported cost.

In `--log-dir` (printed in the config summary at the start of every run):
- `ralph.log` — one line per iteration: timestamp, error/ok, per-iteration
  cost, commit-or-not, running cost total across the whole run, and (if a
  commit was made) its subject line. Read this first to get the shape of a
  run at a glance.
- `iteration-<n>.jsonl` — the full raw NDJSON stream-json output for
  iteration `n`. Read this when a summary line looks wrong or a parse
  warning was printed, to see exactly what claude did/said.
- `iteration-<n>.stderr.log` — that iteration's stderr.

`run.stdout.log` (the loop's own live output) additionally carries three
greppable structural markers, one per moment worth reporting live — used
by `/ralph`'s Monitor-based live reporting, not just for human reading:
- `RALPH_ITERATION_START iter=N total_cost_so_far=$X` — an iteration just
  started; total cost is everything spent by *prior* iterations.
- `RALPH_PLAN iter=N: <text>` — the first `<plan>...</plan>` tag claude
  output this iteration, i.e. the slice it decided to work on.
- `RALPH_CHANGES iter=N commit=<sha> subject="<msg>" diffstat="<stat>"` —
  printed the moment a commit is detected, before the iteration's summary
  line.

## Diagnosing a run that stopped

Match the stop reason (printed on the last line, and derivable from
`ralph.log`) to the fix:

- **"N iteration(s) in a row made no commit and no completion promise"** —
  the stall guard fired. Read the last couple of `iteration-<n>.jsonl` logs:
  is claude blocked on a denied tool (loosen `--allowed-tool` or check
  `--preflight-cmd`), stuck re-attempting something (the prompt's own "if
  blocked, report and stop" instruction may need to be stronger/more
  prominent), or is the plan doc actually already fully done (in which case
  claude isn't outputting the promise when it should)? The harness prompt
  ralph-loop prepends automatically already tells claude the exact phrase
  to use, so a mismatch here is rare unless `--completion-promise` was
  explicitly overridden — if it was, check the override was applied
  consistently.
- **"cumulative cost ... reached the ... budget" / max iterations reached**
  — expected budget exhaustion, not necessarily a bug. Check `ralph.log` for
  how much progress (commit count) happened per dollar/iteration to decide
  whether to raise the limit or fix inefficiency in the prompt first.
- **claude exited with an error / produced no result event** — a
  process-level failure, not a loop-logic one. Check the matching
  `iteration-<n>.stderr.log` and `.jsonl` directly.
- **Preflight command failed, aborting before spawning claude** — the
  environment isn't ready (service down, etc.); nothing was spent. Fix the
  environment or the preflight check itself.
- A completion promise that never fires despite the plan looking done
  usually means the model output the phrase without the
  `<promise>...</promise>` wrapper, or (rarely, since the harness supplies
  the phrase automatically) a custom `--completion-promise` override wasn't
  applied consistently — check `iteration-<n>.jsonl`'s final assistant
  message text directly.

### Recovering from a run that's gone sideways

Not every problem shows up as a stop reason — sometimes iterations are
individually "successful" (a commit, no error) but collectively drifting: a
tracked plan/todo doc getting thrashed or contradicted, quality visibly
degrading commit over commit. This isn't something ralph-loop detects for
you; it's a human judgment call. If `--tag-on-commit` was set, `git tag -l
'ralph/*'` lists a tag per past commit — `git reset --hard <last-good-tag>`
gets you back to a known-good state fast, without re-deriving it from `git
log` by hand. Without tagging enabled, fall back to `git log --oneline` and
`git reset --hard <sha>` directly.

## Using this from a Claude Code session (the `ralph` plugin)

Everything above is about driving the CLI directly (a terminal, CI, or an
assistant shelling out to it). If you're inside an interactive Claude Code
console and this repo's `ralph` plugin is installed, you don't need to do
any of that by hand — `/ralph <prompt>` starts (or reattaches to) a run and
reports progress live, `/ralph-status` gives a one-shot check, and
`/ralph-cancel` stops it. See `/ralph-help` for the full picture.

Two things worth knowing up front:
- The plugin is named `ralph`, deliberately distinct from the unrelated,
  official Anthropic `ralph-loop` marketplace plugin (an in-session `Stop`
  hook loop) that may also be installed — `/ralph-help` explains the
  difference.
- A loop started via `/ralph` is bound to that Claude Code session's
  lifecycle: it's stopped automatically when the session ends, so it never
  silently keeps running (and costing money) after you've left. This is
  different from running the bare CLI directly in a terminal, which keeps
  going until it stops itself or you kill it yourself.

## Build

If `ralph-loop` isn't already on `PATH` or built in this repo:

```
nix develop   # or: nix shell nixpkgs#go
go build -o ralph-loop .
```
