package cmd

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/dmipeck/ralph-loop/internal/prompt"
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
	if err := os.WriteFile(target, []byte(prompt.Starter()), 0o644); err != nil {
		return fmt.Errorf("write %s: %w", target, err)
	}

	out := cmd.OutOrStdout()
	fmt.Fprintf(out, "Wrote starter prompt to %s\n", target)
	fmt.Fprintln(out, "Edit it to describe this project's own goal, plan, and verification steps —")
	fmt.Fprintln(out, "ralph-loop wraps it with its own TDD/commit/stop-condition harness automatically,")
	fmt.Fprintln(out, "so none of that needs to be repeated here. Pick a completion phrase, then run")
	fmt.Fprintln(out, "something like:")
	fmt.Fprintf(out, "\n  ralph-loop run --repo-dir %s --prompt-file %s --completion-promise \"YOUR PHRASE HERE\"\n", repoDir, initOutput)
	return nil
}
