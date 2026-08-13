package claude

import (
	"os"
	"path/filepath"
	"testing"
)

func readFixture(t *testing.T, name string) []byte {
	t.Helper()
	// testdata lives at the module root, three levels up from this package.
	path := filepath.Join("..", "..", "testdata", "ndjson", name)
	b, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read fixture %s: %v", name, err)
	}
	return trimTrailingNewline(b)
}

func trimTrailingNewline(b []byte) []byte {
	if len(b) > 0 && b[len(b)-1] == '\n' {
		return b[:len(b)-1]
	}
	return b
}

func TestParseLine_AssistantText(t *testing.T) {
	ev, err := ParseLine(readFixture(t, "assistant_text.jsonl"))
	if err != nil {
		t.Fatalf("ParseLine: %v", err)
	}
	if ev.Type != "assistant" || ev.Assistant == nil {
		t.Fatalf("expected assistant event, got %+v", ev)
	}
	if len(ev.Assistant.Message.Content) != 1 || ev.Assistant.Message.Content[0].Type != "text" {
		t.Fatalf("expected one text block, got %+v", ev.Assistant.Message.Content)
	}
	if got := ev.Assistant.Message.Content[0].Text; got != "OK" {
		t.Errorf("text = %q, want %q", got, "OK")
	}
}

func TestParseLine_AssistantToolUse(t *testing.T) {
	ev, err := ParseLine(readFixture(t, "assistant_tool_use.jsonl"))
	if err != nil {
		t.Fatalf("ParseLine: %v", err)
	}
	block := ev.Assistant.Message.Content[0]
	if block.Type != "tool_use" || block.Name != "Bash" {
		t.Fatalf("expected Bash tool_use block, got %+v", block)
	}
	if len(block.Input) == 0 {
		t.Error("expected non-empty raw Input")
	}
}

func TestParseLine_UserToolResult(t *testing.T) {
	ev, err := ParseLine(readFixture(t, "user_tool_result.jsonl"))
	if err != nil {
		t.Fatalf("ParseLine: %v", err)
	}
	if ev.Type != "user" || ev.User == nil {
		t.Fatalf("expected user event, got %+v", ev)
	}
	block := ev.User.Message.Content[0]
	if block.Type != "tool_result" {
		t.Fatalf("expected tool_result block, got %+v", block)
	}
	if got := ToolResultText(block.Content); got != "hello\n" {
		t.Errorf("ToolResultText = %q, want %q", got, "hello\n")
	}
}

func TestParseLine_ResultSuccess(t *testing.T) {
	ev, err := ParseLine(readFixture(t, "result_success.jsonl"))
	if err != nil {
		t.Fatalf("ParseLine: %v", err)
	}
	if ev.Type != "result" || ev.Result == nil {
		t.Fatalf("expected result event, got %+v", ev)
	}
	if ev.Result.IsError {
		t.Error("expected IsError=false")
	}
	if ev.Result.Result != "OK" {
		t.Errorf("Result = %q, want %q", ev.Result.Result, "OK")
	}
	if ev.Result.TotalCostUSD <= 0 {
		t.Errorf("TotalCostUSD = %v, want > 0", ev.Result.TotalCostUSD)
	}
}

func TestParseLine_ResultError(t *testing.T) {
	ev, err := ParseLine(readFixture(t, "result_error.jsonl"))
	if err != nil {
		t.Fatalf("ParseLine: %v", err)
	}
	if !ev.Result.IsError {
		t.Error("expected IsError=true")
	}
}

func TestParseLine_UnknownType(t *testing.T) {
	ev, err := ParseLine([]byte(`{"type":"rate_limit_event","rate_limit_info":{}}`))
	if err != nil {
		t.Fatalf("ParseLine: %v", err)
	}
	if ev.Type != "rate_limit_event" {
		t.Errorf("Type = %q, want rate_limit_event", ev.Type)
	}
	if ev.Assistant != nil || ev.User != nil || ev.Result != nil {
		t.Errorf("expected no payload for an unrendered event type, got %+v", ev)
	}
}

func TestParseLine_Malformed(t *testing.T) {
	cases := []string{
		``,
		`not json at all`,
		`{"type":"assistant"`, // truncated
		`{"type":"assistant","message":{"content":[{"type":"tool_use","input":not-json}]}}`,
	}
	for _, line := range cases {
		if _, err := ParseLine([]byte(line)); err == nil {
			t.Errorf("ParseLine(%q): expected error, got nil", line)
		}
	}
}

func TestToolResultText_ArrayShape(t *testing.T) {
	raw := []byte(`[{"type":"text","text":"first"},{"type":"text","text":"second"}]`)
	got := ToolResultText(raw)
	if want := "first second"; got != want {
		t.Errorf("ToolResultText = %q, want %q", got, want)
	}
}

func TestToolResultText_Empty(t *testing.T) {
	if got := ToolResultText(nil); got != "" {
		t.Errorf("ToolResultText(nil) = %q, want empty", got)
	}
}
