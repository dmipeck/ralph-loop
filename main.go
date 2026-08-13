// Command ralph-loop runs the Ralph Wiggum technique loop: repeatedly
// invoking a fresh, non-continued `claude -p` process against a fixed
// prompt until the work is done.
package main

import (
	"os"

	"github.com/dmipeck/ralph-loop/cmd"
)

func main() {
	// cmd.Execute prints its own error (via Cobra's default error
	// handling) — just set the exit code here, don't print again.
	if err := cmd.Execute(); err != nil {
		os.Exit(1)
	}
}
