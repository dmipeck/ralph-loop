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
