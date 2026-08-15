// Package display renders claude.Event values as human-readable lines while
// an iteration is still running — a Go port of the jq filter used by the
// original bash prototype, minus its "empty thinking/text block prints a
// bare marker line" cosmetic bug.
package display

import (
	"fmt"
	"io"
	"regexp"
	"strings"

	"github.com/dmipeck/ralph-loop/internal/claude"
)

const defaultTruncateAt = 160

// planRe finds the FIRST <plan>...</plan> tag in a block of text — same
// tagged-span convention as loop.ExtractPromise, but for the "what am I
// about to work on this iteration" signal claude reports mid-turn rather
// than the end-of-run completion promise.
var planRe = regexp.MustCompile(`(?s)<plan>(.*?)</plan>`)

var planWhitespaceRe = regexp.MustCompile(`\s+`)

// ExtractPlan returns the trimmed, whitespace-collapsed contents of the
// first <plan>...</plan> tag in text, or "" if there is none.
func ExtractPlan(text string) string {
	m := planRe.FindStringSubmatch(text)
	if m == nil {
		return ""
	}
	return planWhitespaceRe.ReplaceAllString(strings.TrimSpace(m[1]), " ")
}

// Renderer implements claude.Renderer, writing truncated human-readable
// lines to Out as events arrive. The full untruncated NDJSON is persisted
// separately by the caller (see claude.RunIteration's rawLog parameter) —
// truncation here is purely about not flooding a terminal with, say, an
// entire file written via the Write tool.
type Renderer struct {
	Out        io.Writer
	TruncateAt int // 0 = use defaultTruncateAt

	// Iteration is the current iteration number, set by BeginIteration.
	// Used only to label the RALPH_PLAN marker line below.
	Iteration int

	planReported bool // whether RALPH_PLAN has already fired this iteration
}

// BeginIteration resets the per-iteration state (the current iteration
// number and whether a plan has been reported yet) — call this once before
// each iteration's claude.RunIteration.
func (r *Renderer) BeginIteration(iteration int) {
	r.Iteration = iteration
	r.planReported = false
}

func (r *Renderer) truncateAt() int {
	if r.TruncateAt > 0 {
		return r.TruncateAt
	}
	return defaultTruncateAt
}

func truncate(s string, n int) string {
	if len(s) > n {
		return s[:n] + "..."
	}
	return s
}

// prefixed truncates s and, if anything survives, writes prefix+text+"\n" to
// out. Empty/whitespace-only blocks (a common shape for e.g. a thinking
// block with no content) are silently skipped rather than printing a bare
// prefix on its own line.
func prefixed(out io.Writer, prefix, s string, n int) {
	t := truncate(s, n)
	if t == "" {
		return
	}
	fmt.Fprintf(out, "%s%s\n", prefix, t)
}

// Render implements claude.Renderer.
func (r *Renderer) Render(ev claude.Event) {
	n := r.truncateAt()

	switch ev.Type {
	case "assistant":
		if ev.Assistant == nil {
			return
		}
		for _, block := range ev.Assistant.Message.Content {
			switch block.Type {
			case "text":
				prefixed(r.Out, "  ", block.Text, n)
				if !r.planReported {
					if plan := ExtractPlan(block.Text); plan != "" {
						r.planReported = true
						fmt.Fprintf(r.Out, "RALPH_PLAN iter=%d: %s\n", r.Iteration, plan)
					}
				}
			case "tool_use":
				input := truncate(string(block.Input), n)
				fmt.Fprintf(r.Out, "  > %s %s\n", block.Name, input)
			case "thinking":
				prefixed(r.Out, "  . ", block.Thinking, n)
			}
		}
	case "user":
		if ev.User == nil {
			return
		}
		for _, block := range ev.User.Message.Content {
			if block.Type != "tool_result" {
				continue
			}
			prefixed(r.Out, "  < ", claude.ToolResultText(block.Content), n)
		}
	case "result":
		fmt.Fprintln(r.Out, "  -- turn complete --")
	}
}
