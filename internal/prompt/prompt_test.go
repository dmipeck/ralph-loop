package prompt

import (
	"strings"
	"testing"
)

// TestCompose_SubstitutesCompletionPromise is the regression test for the
// duplication this package removes: previously, the completion phrase in
// the prompt text and the --completion-promise flag value had to be kept
// in sync by hand. Now Compose derives the phrase claude is told to
// promise directly from the flag value passed in.
func TestCompose_SubstitutesCompletionPromise(t *testing.T) {
	got := Compose("ALL DONE XYZ", "user's own project prompt")

	if !strings.Contains(got, "<promise>ALL DONE XYZ</promise>") {
		t.Errorf("expected composed prompt to contain the substituted promise tag, got: %s", got)
	}
	if strings.Contains(got, "{{COMPLETION_PROMISE}}") {
		t.Errorf("placeholder leaked into composed prompt: %s", got)
	}
}

func TestCompose_IncludesUserPrompt(t *testing.T) {
	got := Compose("PHRASE", "this project's very own goal and plan")

	if !strings.Contains(got, "this project's very own goal and plan") {
		t.Errorf("expected composed prompt to include the user's own prompt text, got: %s", got)
	}
}

// TestHarnessTemplate_SearchBeforeAssuming guards the section 2 guardrail
// telling each iteration to search the whole tree before concluding
// something isn't implemented yet, since the prompt/specs can lag behind
// what's already been built.
func TestHarnessTemplate_SearchBeforeAssuming(t *testing.T) {
	if !strings.Contains(harnessTemplate, "Before concluding something isn't implemented yet, search for it") {
		t.Errorf("expected harness template to instruct searching before assuming something isn't implemented, got: %s", harnessTemplate)
	}
}

// TestHarnessTemplate_NoPlaceholdersOrStubs guards the section 4 guardrail
// against faking green tests/builds with placeholder or stub implementations.
func TestHarnessTemplate_NoPlaceholdersOrStubs(t *testing.T) {
	if !strings.Contains(harnessTemplate, "placeholder") {
		t.Errorf("expected harness template to mention avoiding placeholder implementations, got: %s", harnessTemplate)
	}
	if !strings.Contains(harnessTemplate, "stub") {
		t.Errorf("expected harness template to mention avoiding stub implementations, got: %s", harnessTemplate)
	}
}

// TestHarnessTemplate_StaticAnalysis guards the section 4 broadening of
// verification beyond the test suite to also cover static analysis, so a
// slice isn't considered done until a type-checker/linter is clean too.
func TestHarnessTemplate_StaticAnalysis(t *testing.T) {
	if !strings.Contains(harnessTemplate, "type-checker, linter, or other static-analysis") {
		t.Errorf("expected harness template to instruct running static analysis alongside the test suite, got: %s", harnessTemplate)
	}
}

// TestHarnessTemplate_SubagentGuidance guards the section 4 guidance that
// subagents are fine for research/search-style work, but that build/test/
// verification commands must run one at a time, never concurrently.
func TestHarnessTemplate_SubagentGuidance(t *testing.T) {
	if !strings.Contains(harnessTemplate, "Task tool") {
		t.Errorf("expected harness template to reference the Task tool for subagent use, got: %s", harnessTemplate)
	}
	if !strings.Contains(harnessTemplate, "serially") {
		t.Errorf("expected harness template to require running verification commands serially, got: %s", harnessTemplate)
	}
}

// TestHarnessTemplate_StandingInstructionsDoc guards the section 2 bullet
// telling each iteration to treat a project's own CLAUDE.md/AGENT.md as
// authoritative and append newly discovered learnings to it.
func TestHarnessTemplate_StandingInstructionsDoc(t *testing.T) {
	if !strings.Contains(harnessTemplate, "AGENT.md") && !strings.Contains(harnessTemplate, "CLAUDE.md") {
		t.Errorf("expected harness template to reference AGENT.md or CLAUDE.md, got: %s", harnessTemplate)
	}
	if !strings.Contains(harnessTemplate, "learnings") {
		t.Errorf("expected harness template to mention appending learnings to a standing-instructions doc, got: %s", harnessTemplate)
	}
}

// TestHarnessTemplate_FreshWorktreeAndBranch guards the new section 1
// requirement that each iteration starts its work in a fresh git worktree
// on a new branch, rather than committing directly on the branch the loop
// started on.
func TestHarnessTemplate_FreshWorktreeAndBranch(t *testing.T) {
	if !strings.Contains(harnessTemplate, "worktree") {
		t.Errorf("expected harness template to instruct starting work in a fresh git worktree, got: %s", harnessTemplate)
	}
	if !strings.Contains(harnessTemplate, "new branch") {
		t.Errorf("expected harness template to instruct starting work on a new branch, got: %s", harnessTemplate)
	}
}

// TestHarnessTemplate_OpensPRBeforeWork guards the section 1 instruction
// to push the new branch and open a draft PR/MR for it before starting
// implementation, whenever a matching CLI/remote is available — and to
// skip that step silently otherwise, rather than blocking the iteration.
func TestHarnessTemplate_OpensPRBeforeWork(t *testing.T) {
	if !strings.Contains(harnessTemplate, "gh pr create") {
		t.Errorf("expected harness template to reference gh pr create, got: %s", harnessTemplate)
	}
	if !strings.Contains(harnessTemplate, "glab mr create") {
		t.Errorf("expected harness template to reference glab mr create, got: %s", harnessTemplate)
	}
	if !strings.Contains(harnessTemplate, "skip this step silently") {
		t.Errorf("expected harness template to allow silently skipping PR/MR creation when unavailable, got: %s", harnessTemplate)
	}
}

// TestHarnessTemplate_PushesEachCommit guards the section 5 requirement
// that each iteration pushes its commit immediately, rather than leaving
// it local-only for a future iteration to push.
func TestHarnessTemplate_PushesEachCommit(t *testing.T) {
	if !strings.Contains(harnessTemplate, "`git push`") {
		t.Errorf("expected harness template to instruct running git push after each commit, got: %s", harnessTemplate)
	}
}

// TestHarnessTemplate_PromiseOnlyAfterCommitAndPush guards the section 9
// requirement that the completion promise may only be reported after this
// iteration's changes have actually been committed and pushed.
func TestHarnessTemplate_PromiseOnlyAfterCommitAndPush(t *testing.T) {
	if !strings.Contains(harnessTemplate, "must not** output this promise") {
		t.Errorf("expected harness template to forbid reporting completion before commit/push, got: %s", harnessTemplate)
	}
	if !strings.Contains(harnessTemplate, "committed and pushed") {
		t.Errorf("expected harness template to require commit and push before the completion promise, got: %s", harnessTemplate)
	}
}
