package display

import (
	"bytes"
	"strings"
	"testing"

	"github.com/dmipeck/ralph-loop/internal/claude"
)

func TestExtractPlan(t *testing.T) {
	cases := []struct {
		name string
		text string
		want string
	}{
		{"exact match", "<plan>Add the widget</plan>", "Add the widget"},
		{"surrounded by other text", "Looking at this now.\n<plan>Add the widget</plan>\nOK.", "Add the widget"},
		{"internal whitespace collapsed", "<plan>Add\n  the widget</plan>", "Add the widget"},
		{"leading/trailing whitespace trimmed", "<plan>  Add the widget  </plan>", "Add the widget"},
		{"absent", "No plan tag here.", ""},
		{"empty tag", "<plan></plan>", ""},
		{"multiple tags: first wins", "<plan>FIRST</plan> text <plan>SECOND</plan>", "FIRST"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := ExtractPlan(tc.text); got != tc.want {
				t.Errorf("ExtractPlan(%q) = %q, want %q", tc.text, got, tc.want)
			}
		})
	}
}

func textEvent(text string) claude.Event {
	return claude.Event{
		Type: "assistant",
		Assistant: &claude.AssistantEvent{
			Type:    "assistant",
			Message: claude.Message{Content: []claude.ContentBlock{{Type: "text", Text: text}}},
		},
	}
}

func TestRender_EmitsPlanMarkerOnce(t *testing.T) {
	var buf bytes.Buffer
	r := &Renderer{Out: &buf}
	r.BeginIteration(7)

	r.Render(textEvent("Looking around first.\n<plan>Add the widget</plan>"))
	r.Render(textEvent("<plan>A different plan</plan>")) // should not fire again

	out := buf.String()
	if got := strings.Count(out, "RALPH_PLAN"); got != 1 {
		t.Fatalf("expected exactly one RALPH_PLAN line, got %d in:\n%s", got, out)
	}
	if !strings.Contains(out, "RALPH_PLAN iter=7: Add the widget") {
		t.Errorf("expected RALPH_PLAN iter=7: Add the widget, got:\n%s", out)
	}
}

func TestRender_NoPlanTagEmitsNoMarker(t *testing.T) {
	var buf bytes.Buffer
	r := &Renderer{Out: &buf}
	r.BeginIteration(1)

	r.Render(textEvent("Just some ordinary text."))

	if strings.Contains(buf.String(), "RALPH_PLAN") {
		t.Errorf("did not expect a RALPH_PLAN line, got:\n%s", buf.String())
	}
}

func TestRender_PlanMarkerResetsOnNewIteration(t *testing.T) {
	var buf bytes.Buffer
	r := &Renderer{Out: &buf}
	r.BeginIteration(1)
	r.Render(textEvent("<plan>First</plan>"))

	r.BeginIteration(2)
	r.Render(textEvent("<plan>Second</plan>"))

	out := buf.String()
	if got := strings.Count(out, "RALPH_PLAN"); got != 2 {
		t.Fatalf("expected two RALPH_PLAN lines across two iterations, got %d in:\n%s", got, out)
	}
	if !strings.Contains(out, "RALPH_PLAN iter=2: Second") {
		t.Errorf("expected RALPH_PLAN iter=2: Second, got:\n%s", out)
	}
}
