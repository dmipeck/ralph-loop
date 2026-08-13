package cmd

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/spf13/cobra"
)

var (
	initOutput string
	initForce  bool
)

var initCmd = &cobra.Command{
	Use:   "init",
	Short: "Scaffold a starter prompt file for ralph-loop in --repo-dir",
	RunE:  runInit,
}

func init() {
	rootCmd.AddCommand(initCmd)
	flags := initCmd.Flags()
	flags.StringVar(&initOutput, "output", "PROMPT.md", "Path (relative to --repo-dir) to write the starter prompt file")
	flags.BoolVar(&initForce, "force", false, "Overwrite an existing file")
}

func runInit(cmd *cobra.Command, _ []string) error {
	absRepoDir, err := filepath.Abs(repoDir)
	if err != nil {
		return fmt.Errorf("resolve --repo-dir: %w", err)
	}
	target := filepath.Join(absRepoDir, initOutput)

	if _, err := os.Stat(target); err == nil && !initForce {
		return fmt.Errorf("%s already exists (pass --force to overwrite)", target)
	}

	if err := os.MkdirAll(filepath.Dir(target), 0o755); err != nil {
		return fmt.Errorf("create %s: %w", filepath.Dir(target), err)
	}
	if err := os.WriteFile(target, []byte(starterPromptTemplate), 0o644); err != nil {
		return fmt.Errorf("write %s: %w", target, err)
	}

	out := cmd.OutOrStdout()
	fmt.Fprintf(out, "Wrote starter prompt to %s\n", target)
	fmt.Fprintln(out, "Edit it to describe this project's own plan and verification steps, pick a completion")
	fmt.Fprintln(out, "phrase, then run something like:")
	fmt.Fprintf(out, "\n  ralph-loop run --repo-dir %s --prompt-file %s --completion-promise \"YOUR PHRASE HERE\"\n", repoDir, initOutput)
	return nil
}

const starterPromptTemplate = `# Ralph iteration prompt

You are one iteration of an unattended loop. You have no memory of any
previous iteration — this process started fresh. This repository's own
plan/spec document (e.g. PLAN.md — replace with whatever this project
actually calls it) and the current state of the git repository are the
*only* sources of truth for what has already been done and what remains.
Do not assume anything about prior progress beyond what you can verify by
reading files and git history.

## 1. Orient yourself

- Read the project's plan/spec document end to end.
- Run ` + "`git log --oneline -30`" + `, ` + "`git status`" + `, and ` + "`git diff`" + ` to see exactly what
  earlier iterations have already committed.
- Cross-reference the two: figure out which parts of the plan are already
  done in the working tree, and which are not.

## 2. Pick the smallest next slice

From what remains, choose the single smallest next piece of work that is:

- A **complete, independently useful increment** — once committed, the
  codebase is left in a working state with tests passing, not halfway
  through a change.
- Correctly **ordered** — respect whatever dependency order the plan lays
  out, but feel free to break any single step into smaller sub-steps if it's
  too big for one commit.

## 3. Implement it with strict TDD

1. Write or extend a failing test first, matching this project's existing
   test style and location.
2. Run the project's test command and confirm the new test fails for the
   reason you expect — not for an unrelated reason (typo, missing build
   step, environment issue).
3. Write the minimal implementation needed to make that test pass.
4. Re-run the **full** relevant test suite, not just the new test, and
   confirm everything is green.
5. Refactor only as much as needed to keep the change minimal and
   consistent with the surrounding code, while keeping tests green.

## 4. Commit exactly one thing

The commit you make this iteration must contain exactly **one** thing: one
feature, one fix, or one focused refactor — never a bundle of unrelated
edits. If you notice unrelated cleanup worth doing, leave it for a future
iteration; do not touch it now.

- ` + "`git add`" + ` only the files that belong to this slice.
- Write a commit message describing that one change and referencing the
  relevant part of the plan.
- ` + "`git commit`" + `.

## 5. If something is blocked, don't thrash — report it

If a command you need is denied by permissions, fails for an environment
reason (a database isn't reachable, a service isn't running, ...), or is
otherwise blocked: **try at most one alternative approach**, then stop and
report the blocker plainly rather than retrying many variations. Burning
dozens of turns re-attempting a blocked command wastes real cost for no
benefit — a clear, early report is more useful than an exhausted retry
storm. Leave the working tree clean (revert any partial edits) rather than
leaving broken or half-finished work behind.

## 6. Stop

The moment a commit is made (or you've reported a blocker), stop. Do not
start the next slice, do not keep editing, do not run further exploratory
commands. This process exits right after your response anyway, but end
your turn deliberately rather than trailing off mid-task.

## 7. Report, and only promise completion when it's actually true

As the last line of your reply, output exactly one of:

- A one-line summary of the commit you just made (or, if you made none, why
  not).
- ` + "`<promise>YOUR COMPLETION PHRASE HERE</promise>`" + ` — but **only** if every
  part of the plan is finished *and* verified (tests green, build clean,
  whatever this project's own verification steps call for). A single
  successful commit is never grounds to output this. Do not output it to
  end the loop early, and do not output it speculatively "because the next
  iteration will probably finish it" — only when it is unequivocally true
  right now.
`
