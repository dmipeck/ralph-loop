// Package cmd wires the Cobra CLI. No business logic lives here — it
// resolves flags into an internal/config.Config and hands off to
// internal/loop.
package cmd

import (
	"fmt"

	"github.com/spf13/cobra"
)

// Set via `-ldflags "-X github.com/dmipeck/ralph-loop/cmd.version=... -X ...commit=... -X ...date=..."`.
var (
	version = "dev"
	commit  = ""
	date    = ""
)

var (
	repoDir string
	verbose bool
)

var rootCmd = &cobra.Command{
	Use:   "ralph-loop",
	Short: "Run the Ralph Wiggum technique loop against a Claude Code project",
	Long: `ralph-loop repeatedly runs a fresh, non-continued "claude -p" process
against a fixed prompt file until it declares the work complete. Each
iteration has no memory of the last — the target repo's files and git
history are the only state that persists between iterations.`,
}

// Execute runs the root command; main.go calls this and exits non-zero on error.
func Execute() error {
	rootCmd.Version = versionString()
	return rootCmd.Execute()
}

func versionString() string {
	if commit == "" && date == "" {
		return version
	}
	return fmt.Sprintf("%s (%s, %s)", version, commit, date)
}

func init() {
	rootCmd.PersistentFlags().StringVar(&repoDir, "repo-dir", ".", "Target git repository to run against")
	rootCmd.PersistentFlags().BoolVarP(&verbose, "verbose", "v", false, "Verbose output")
}
