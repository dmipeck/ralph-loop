// Package config holds the fully-resolved settings for one ralph-loop run.
package config

import (
	"fmt"
	"os"
	"path/filepath"
	"time"
)

// Config is built from CLI flags by cmd/run.go and passed to loop.Controller.
// Every field is expected to already be resolved (defaults applied, paths
// made absolute) by the time Validate is called.
type Config struct {
	RepoDir string

	// Exactly one of PromptFile/PromptText is set. PromptFile is re-read
	// fresh at the start of every iteration; PromptText is a fixed string
	// used as-is for every iteration.
	PromptFile string
	PromptText string

	MaxIterations        int // 0 = unlimited
	MaxStalledIterations int // 0 = disabled
	CompletionPromise    string
	Sleep                time.Duration
	Model                string

	PermissionMode  string
	AllowedTools    []string
	DisallowedTools []string
	SkipPermissions bool

	MaxIterationCostUSD float64 // 0 = unset, passed through to `claude -p --max-budget-usd`
	MaxTotalCostUSD     float64 // 0 = unset, enforced by us across iterations

	PreflightCmd string // arbitrary shell command; must exit 0 before each iteration

	Branch      string // optional: checked out/created before the loop starts
	TagOnCommit bool   // optional: tag every successful commit at HEAD
	LogDir      string

	DryRun bool
}

// Validate checks the config is internally consistent and that referenced
// paths actually exist. It does not mutate Config — callers apply defaults
// before calling this.
func (c *Config) Validate() error {
	if c.PromptFile == "" && c.PromptText == "" {
		return fmt.Errorf("a prompt is required (--prompt or --prompt-file)")
	}
	if c.PromptFile != "" && c.PromptText != "" {
		return fmt.Errorf("--prompt and --prompt-file are mutually exclusive")
	}
	if c.PromptFile != "" {
		info, err := os.Stat(c.PromptFile)
		if err != nil {
			return fmt.Errorf("prompt file %q: %w", c.PromptFile, err)
		}
		if info.IsDir() {
			return fmt.Errorf("prompt file %q is a directory", c.PromptFile)
		}
	}

	if c.CompletionPromise == "" {
		return fmt.Errorf("completion promise must be resolved before validation")
	}

	if c.RepoDir == "" {
		return fmt.Errorf("repo dir could not be resolved")
	}
	repoInfo, err := os.Stat(c.RepoDir)
	if err != nil {
		return fmt.Errorf("repo dir %q: %w", c.RepoDir, err)
	}
	if !repoInfo.IsDir() {
		return fmt.Errorf("repo dir %q is not a directory", c.RepoDir)
	}
	if _, err := os.Stat(filepath.Join(c.RepoDir, ".git")); err != nil {
		return fmt.Errorf("repo dir %q does not look like a git repository (no .git) — run ralph-loop from inside the target repo: %w", c.RepoDir, err)
	}

	if c.MaxIterations < 0 {
		return fmt.Errorf("--max-iterations must be >= 0 (0 = unlimited), got %d", c.MaxIterations)
	}
	if c.MaxStalledIterations < 0 {
		return fmt.Errorf("--max-stalled-iterations must be >= 0 (0 = disabled), got %d", c.MaxStalledIterations)
	}
	if c.Sleep < 0 {
		return fmt.Errorf("--sleep must be >= 0, got %s", c.Sleep)
	}
	if c.MaxIterationCostUSD < 0 {
		return fmt.Errorf("--max-iteration-cost-usd must be >= 0, got %v", c.MaxIterationCostUSD)
	}
	if c.MaxTotalCostUSD < 0 {
		return fmt.Errorf("--max-total-cost-usd must be >= 0, got %v", c.MaxTotalCostUSD)
	}

	if c.SkipPermissions {
		if c.PermissionMode != "" {
			return fmt.Errorf("--dangerously-skip-permissions cannot be combined with --permission-mode")
		}
		if len(c.AllowedTools) > 0 || len(c.DisallowedTools) > 0 {
			return fmt.Errorf("--dangerously-skip-permissions cannot be combined with --allowed-tool/--disallowed-tool")
		}
	}

	if c.LogDir == "" {
		return fmt.Errorf("log dir must be resolved before validation")
	}

	return nil
}

// ReadPrompt resolves the effective prompt text for one iteration: if
// PromptFile is set it's read fresh (so edits between iterations are
// picked up), otherwise the fixed PromptText is returned as-is.
func (c *Config) ReadPrompt() (string, error) {
	if c.PromptFile != "" {
		b, err := os.ReadFile(c.PromptFile)
		if err != nil {
			return "", fmt.Errorf("read prompt file: %w", err)
		}
		return string(b), nil
	}
	return c.PromptText, nil
}
