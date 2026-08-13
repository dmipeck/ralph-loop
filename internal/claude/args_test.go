package claude

import (
	"testing"

	"github.com/dmipeck/ralph-loop/internal/config"
)

// contains reports whether s appears as an exact element of args.
func contains(args []string, s string) bool {
	for _, a := range args {
		if a == s {
			return true
		}
	}
	return false
}

// TestBuildArgs_AllowedToolPatternSurvivesIntact is the regression test for
// the bug that motivated this whole rewrite: the bash prototype built its
// --allowedTools argument from an unquoted shell string, and bash word-split
// "Bash(git *)" on its internal space into two garbage tokens before it
// ever reached argv, silently disabling every git-allow rule. Here, args are
// built as a real []string with no shell involved — this test asserts that
// structural guarantee directly, so it fails loudly if anyone ever
// "helpfully" refactors BuildArgs to join patterns into a single
// space-joined string again.
func TestBuildArgs_AllowedToolPatternSurvivesIntact(t *testing.T) {
	cfg := &config.Config{
		PermissionMode: "acceptEdits",
		AllowedTools:   []string{"Read", "Bash(git *)", "Bash(vendor/bin/phpunit*)"},
	}

	args := BuildArgs(cfg, "prompt text")

	for _, want := range cfg.AllowedTools {
		if !contains(args, want) {
			t.Errorf("expected %q to survive as one unsplit argv element, got args: %#v", want, args)
		}
	}
	if contains(args, "Bash(git") {
		t.Errorf("found the split-bug's garbage token \"Bash(git\" in args: %#v", args)
	}
}

func TestBuildArgs_SkipPermissions(t *testing.T) {
	cfg := &config.Config{SkipPermissions: true}
	args := BuildArgs(cfg, "prompt")

	if !contains(args, "--dangerously-skip-permissions") {
		t.Errorf("expected --dangerously-skip-permissions in args: %#v", args)
	}
	if contains(args, "--allowedTools") || contains(args, "--permission-mode") {
		t.Errorf("did not expect permission-mode/allowedTools flags alongside skip-permissions: %#v", args)
	}
}

func TestBuildArgs_PromptPassedAsSingleArg(t *testing.T) {
	cfg := &config.Config{}
	prompt := "line one\nline two with spaces and \"quotes\" and $vars"
	args := BuildArgs(cfg, prompt)

	if !contains(args, prompt) {
		t.Errorf("expected prompt text to appear as one unmodified argv element")
	}
}

func TestBuildArgs_ModelAndBudgetPassthrough(t *testing.T) {
	cfg := &config.Config{Model: "claude-sonnet-5", MaxIterationCostUSD: 1.5}
	args := BuildArgs(cfg, "prompt")

	if !contains(args, "--model") || !contains(args, "claude-sonnet-5") {
		t.Errorf("expected --model claude-sonnet-5 in args: %#v", args)
	}
	if !contains(args, "--max-budget-usd") || !contains(args, "1.5") {
		t.Errorf("expected --max-budget-usd 1.5 in args: %#v", args)
	}
}

func TestBuildArgs_AlwaysNonInteractivePrintMode(t *testing.T) {
	args := BuildArgs(&config.Config{}, "prompt")
	for _, want := range []string{"-p", "--output-format", "stream-json", "--verbose", "--no-session-persistence"} {
		if !contains(args, want) {
			t.Errorf("expected %q in args: %#v", want, args)
		}
	}
}
