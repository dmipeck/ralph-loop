package logging

import (
	"strings"
	"testing"
	"time"
)

func TestFormatSummaryLine_NoCommit(t *testing.T) {
	ts := time.Date(2026, 1, 2, 3, 4, 5, 0, time.UTC)
	line := FormatSummaryLine(ts, 5, false, 0.5, "no", 2.5, "")
	if strings.Contains(line, "subject=") {
		t.Errorf("FormatSummaryLine with no commit should omit subject=, got %q", line)
	}
	for _, want := range []string{"iter=5", "is_error=false", "cost=$0.500000", "committed=no", "total_cost=$2.500000"} {
		if !strings.Contains(line, want) {
			t.Errorf("FormatSummaryLine = %q, want it to contain %q", line, want)
		}
	}
}

func TestFormatSummaryLine_WithCommitSubject(t *testing.T) {
	ts := time.Date(2026, 1, 2, 3, 4, 5, 0, time.UTC)
	line := FormatSummaryLine(ts, 5, false, 0.5, "yes (abc123)", 2.5, "Add X")
	if !strings.Contains(line, `subject="Add X"`) {
		t.Errorf("FormatSummaryLine = %q, want it to contain subject=%q", line, `"Add X"`)
	}
	if !strings.Contains(line, "committed=yes (abc123)") {
		t.Errorf("FormatSummaryLine = %q, want it to contain committed=yes (abc123)", line)
	}
}
