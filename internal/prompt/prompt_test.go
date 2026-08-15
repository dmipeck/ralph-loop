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

// TestHarnessTemplate_UpdatesTrackedPlanDoc guards the living plan-doc
// discipline bullet in section 4: when the project prompt points at a
// tracked plan/todo/roadmap document, each commit should update it too.
func TestHarnessTemplate_UpdatesTrackedPlanDoc(t *testing.T) {
	if !strings.Contains(harnessTemplate, "tracked plan/todo/roadmap") {
		t.Errorf("expected harness template to instruct updating a tracked plan/todo/roadmap doc, got: %s", harnessTemplate)
	}
}

// TestHarnessTemplate_SearchBeforeAssuming guards the section 1 guardrail
// telling each iteration to search the whole tree before concluding
// something isn't implemented yet, since the prompt/specs can lag behind
// what's already been built.
func TestHarnessTemplate_SearchBeforeAssuming(t *testing.T) {
	if !strings.Contains(harnessTemplate, "Before concluding something isn't implemented yet, search for it") {
		t.Errorf("expected harness template to instruct searching before assuming something isn't implemented, got: %s", harnessTemplate)
	}
}

// TestHarnessTemplate_NoPlaceholdersOrStubs guards the section 3 guardrail
// against faking green tests/builds with placeholder or stub implementations.
func TestHarnessTemplate_NoPlaceholdersOrStubs(t *testing.T) {
	if !strings.Contains(harnessTemplate, "placeholder") {
		t.Errorf("expected harness template to mention avoiding placeholder implementations, got: %s", harnessTemplate)
	}
	if !strings.Contains(harnessTemplate, "stub") {
		t.Errorf("expected harness template to mention avoiding stub implementations, got: %s", harnessTemplate)
	}
}
