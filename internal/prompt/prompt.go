// Package prompt supplies the harness prompt ralph-loop wraps around every
// project's own prompt file. TDD discipline, one-commit-per-iteration, the
// stop condition, and the completion-promise reporting format are the same
// for every project, so they live here rather than being something each
// user has to copy into their own prompt file.
package prompt

import (
	_ "embed"
	"strings"
)

//go:embed templates/harness.md
var harnessTemplate string

//go:embed templates/starter.md
var starterTemplate string

// completionPromisePlaceholder is substituted with the run's actual
// --completion-promise value, so the phrase claude is told to promise
// always matches the phrase the loop is watching for — the user never
// has to keep two copies of it in sync.
const completionPromisePlaceholder = "{{COMPLETION_PROMISE}}"

// Starter returns the starter template written by `ralph-loop init` — a
// project-specific goal/plan/verification skeleton. It intentionally does
// not contain the harness mechanics; those are injected by Compose at run
// time instead.
func Starter() string {
	return starterTemplate
}

// Compose builds the full prompt sent to claude for one iteration: the
// built-in harness (with completionPromise substituted in), followed by
// the project's own prompt file content.
func Compose(completionPromise, userPrompt string) string {
	harness := strings.ReplaceAll(harnessTemplate, completionPromisePlaceholder, completionPromise)
	return harness + "\n" + userPrompt
}
