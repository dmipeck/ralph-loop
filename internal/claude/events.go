package claude

import (
	"encoding/json"
	"fmt"
	"time"
)

// ContentBlock is one block inside an assistant/user message's "content"
// array. Only the fields relevant to a given block Type are populated —
// e.g. Text for "text", Name+Input for "tool_use", Thinking for "thinking",
// Content for "tool_result".
type ContentBlock struct {
	Type     string          `json:"type"`
	Text     string          `json:"text,omitempty"`
	Name     string          `json:"name,omitempty"`
	Input    json.RawMessage `json:"input,omitempty"`
	Thinking string          `json:"thinking,omitempty"`
	Content  json.RawMessage `json:"content,omitempty"` // tool_result: string OR []ContentBlock-ish
}

type Message struct {
	Content []ContentBlock `json:"content"`
}

// AssistantEvent is a `{"type":"assistant", ...}` NDJSON line.
type AssistantEvent struct {
	Type    string  `json:"type"`
	Message Message `json:"message"`
}

// UserEvent is a `{"type":"user", ...}` NDJSON line — despite the name, this
// is claude's own transcript of tool results being fed back to the model,
// not anything a human typed.
type UserEvent struct {
	Type    string  `json:"type"`
	Message Message `json:"message"`
}

// ResultEvent is the terminal `{"type":"result", ...}` NDJSON line: exactly
// one per `claude -p` invocation, always the last line. This is what the
// loop controller parses for cost, error state, and the completion promise.
type ResultEvent struct {
	Type         string  `json:"type"`
	Subtype      string  `json:"subtype"`
	IsError      bool    `json:"is_error"`
	TotalCostUSD float64 `json:"total_cost_usd"`
	Result       string  `json:"result"`
	NumTurns     int     `json:"num_turns"`
	DurationMS   int     `json:"duration_ms"`
	SessionID    string  `json:"session_id"`
}

// RateLimitInfo is the `rate_limit_info` payload of a `{"type":
// "rate_limit_event", ...}` NDJSON line, emitted whenever claude's view of
// the account's rate-limit status changes. Field names/casing here mirror
// the CLI's own internal wire shape exactly (camelCase, unlike the
// surrounding snake_case envelope) — this is not a documented public API,
// so a future claude release could change it.
type RateLimitInfo struct {
	// Status is "allowed", "allowed_warning", or "rejected". Only
	// "rejected" means a request actually got throttled rather than just
	// warned about an approaching cap.
	Status string `json:"status"`
	// ResetsAt is Unix epoch seconds for when this limit lifts, or nil if
	// claude didn't report one (observed alongside every "rejected"
	// status, but treat it as optional defensively).
	ResetsAt *int64 `json:"resetsAt"`
	// RateLimitType identifies which cap was hit, e.g. "five_hour" or
	// "seven_day".
	RateLimitType string `json:"rateLimitType"`
}

// Rejected reports whether this event means a request was actually
// throttled, as opposed to merely a warning that a cap is approaching.
func (r RateLimitInfo) Rejected() bool {
	return r.Status == "rejected"
}

// ResetsAtTime converts ResetsAt to a time.Time, returning ok=false if no
// reset time was reported.
func (r RateLimitInfo) ResetsAtTime() (t time.Time, ok bool) {
	if r.ResetsAt == nil {
		return time.Time{}, false
	}
	return time.Unix(*r.ResetsAt, 0), true
}

// Event is the discriminated union produced by ParseLine. Exactly one of
// Assistant/User/Result/RateLimit is non-nil, matching Type — the rest
// (e.g. "system") carry no payload we render, so all four are nil.
type Event struct {
	Type      string
	Assistant *AssistantEvent
	User      *UserEvent
	Result    *ResultEvent
	RateLimit *RateLimitInfo
}

// ParseLine decodes one NDJSON line from `claude -p --output-format
// stream-json`. Callers should skip blank lines before calling this; a
// non-empty line that isn't valid JSON, or doesn't match any known shape,
// returns an error — callers should log it and keep reading rather than
// abort the whole stream, since a single stray line should never sink an
// iteration.
func ParseLine(line []byte) (Event, error) {
	var envelope struct {
		Type string `json:"type"`
	}
	if err := json.Unmarshal(line, &envelope); err != nil {
		return Event{}, fmt.Errorf("parse NDJSON line: %w", err)
	}

	ev := Event{Type: envelope.Type}
	switch envelope.Type {
	case "assistant":
		var a AssistantEvent
		if err := json.Unmarshal(line, &a); err != nil {
			return Event{}, fmt.Errorf("parse assistant event: %w", err)
		}
		ev.Assistant = &a
	case "user":
		var u UserEvent
		if err := json.Unmarshal(line, &u); err != nil {
			return Event{}, fmt.Errorf("parse user event: %w", err)
		}
		ev.User = &u
	case "result":
		var r ResultEvent
		if err := json.Unmarshal(line, &r); err != nil {
			return Event{}, fmt.Errorf("parse result event: %w", err)
		}
		ev.Result = &r
	case "rate_limit_event":
		var e struct {
			RateLimitInfo RateLimitInfo `json:"rate_limit_info"`
		}
		if err := json.Unmarshal(line, &e); err != nil {
			return Event{}, fmt.Errorf("parse rate_limit_event: %w", err)
		}
		ev.RateLimit = &e.RateLimitInfo
	default:
		// system and any future event types we don't specifically render —
		// carry no payload, that's fine.
	}
	return ev, nil
}

// ToolResultText best-efforts a human-readable string out of a tool_result
// block's Content, which the CLI emits as either a bare JSON string or an
// array of {"type":"text","text":"..."} blocks.
func ToolResultText(raw json.RawMessage) string {
	if len(raw) == 0 {
		return ""
	}

	var asString string
	if err := json.Unmarshal(raw, &asString); err == nil {
		return asString
	}

	var blocks []ContentBlock
	if err := json.Unmarshal(raw, &blocks); err == nil {
		out := ""
		for _, b := range blocks {
			if b.Text != "" {
				if out != "" {
					out += " "
				}
				out += b.Text
			}
		}
		return out
	}

	return string(raw)
}
