---
description: "Explain the ralph plugin and its commands"
---

# /ralph-help

Explain to the user:

**Disambiguation** (if the official Anthropic `ralph-loop` marketplace
plugin is also installed — unrelated, easy to confuse by name):
- Official `ralph-loop` (`/ralph-loop`, `/cancel-ralph`): re-feeds the same
  prompt to *this session* via a `Stop` hook; stops when the session ends.
- This plugin (`ralph`: `/ralph`, `/ralph-status`, `/ralph-cancel`):
  launches the standalone `ralph-loop` CLI as a real background process — a
  fresh, non-continued `claude -p` per iteration, real git commits — but
  bound to *this* session's lifecycle: it stops automatically when this
  session ends, so it can never silently keep running/costing money after
  you leave.

Then list all four commands with a one-line description each:
- `/ralph <prompt>` — start or reattach, then watch live progress.
- `/ralph-status` — one-shot status check.
- `/ralph-cancel` — stop the active loop.
- `/ralph-help` — this command.

Also mention: the state file location (`.claude/ralph/state`, git-excluded
via `.git/info/exclude`), the log dir convention (same default the bare CLI
uses: `<OS cache dir>/ralph-loop/<repo-basename>`, deliberately outside the
repo), the one-loop-per-repo constraint, the session-bound lifecycle, and
point to the bundled `ralph-loop` skill for CLI flag-level detail.
