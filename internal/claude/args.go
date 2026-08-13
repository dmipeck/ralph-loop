// Package claude wraps invocations of the `claude` CLI in print mode.
package claude

import (
	"strconv"

	"github.com/dmipeck/ralph-loop/internal/config"
)

// BuildArgs constructs the argv (excluding the "claude" binary name itself)
// for one iteration's `claude -p` invocation. It is a pure function deliberately
// kept separate from exec.Command so it can be unit-tested without spawning
// anything: every element ends up as its own exec.Cmd.Args slot with no shell
// re-parsing in between, so a pattern like "Bash(git *)" survives as a single
// argv element by construction — there is no string to word-split.
func BuildArgs(cfg *config.Config, promptText string) []string {
	args := []string{
		"-p", promptText,
		"--output-format", "stream-json",
		"--verbose",
		"--no-session-persistence",
	}

	if cfg.Model != "" {
		args = append(args, "--model", cfg.Model)
	}

	if cfg.SkipPermissions {
		args = append(args, "--dangerously-skip-permissions")
	} else {
		if cfg.PermissionMode != "" {
			args = append(args, "--permission-mode", cfg.PermissionMode)
		}
		if len(cfg.AllowedTools) > 0 {
			args = append(args, "--allowedTools")
			args = append(args, cfg.AllowedTools...)
		}
		if len(cfg.DisallowedTools) > 0 {
			args = append(args, "--disallowedTools")
			args = append(args, cfg.DisallowedTools...)
		}
	}

	if cfg.MaxIterationCostUSD > 0 {
		args = append(args, "--max-budget-usd", strconv.FormatFloat(cfg.MaxIterationCostUSD, 'f', -1, 64))
	}

	return args
}
