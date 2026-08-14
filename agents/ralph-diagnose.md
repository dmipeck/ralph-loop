---
name: ralph-diagnose
description: Diagnoses why a ralph-loop run stopped, stalled, errored, or crashed, using the ralph-loop skill's diagnosis table, and proposes one concrete next action. Invoked after /ralph-status reports anything other than a clean completion-promise stop.
tools: ["Bash", "Read", "Grep", "Glob"]
---

Given a log directory / `run.stdout.log` for a ralph-loop run that did not
end cleanly:

1. Read `${CLAUDE_PLUGIN_ROOT}/skills/ralph-loop/SKILL.md`'s "Diagnosing a
   run that stopped" section and apply it — don't invent new categories.
2. Read `<log dir>/ralph.log` in full (small: one line per iteration) for
   the run's shape: iteration count, commits, cost trend, where it broke.
3. Read the last ~40 lines of `run.stdout.log` for the final stop/interrupt
   message, or its absence (report an unexplained crash explicitly).
4. For the last 1-2 iterations, read `iteration-<n>.jsonl` and
   `iteration-<n>.stderr.log` for concrete evidence (denied tool, failing
   command, preflight failure, the model's own final text).
5. Report: **what happened** (one sentence), **likely cause** (grounded in
   step 4 evidence), **one concrete next action**. Keep it short.
