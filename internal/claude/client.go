package claude

import (
	"bufio"
	"context"
	"fmt"
	"io"
	"os/exec"
	"strings"
)

// Renderer renders one parsed Event for live display while an iteration is
// still running. Defined here (not in the display package) so this package
// never has to import display — display imports claude for the Event type
// instead, and its Renderer implements this interface structurally.
type Renderer interface {
	Render(ev Event)
}

// LookPath confirms the `claude` binary is reachable, mirroring the bash
// version's upfront `command -v claude` check so a missing binary fails
// fast with a clear message instead of deep inside the loop.
func LookPath() (string, error) {
	path, err := exec.LookPath("claude")
	if err != nil {
		return "", fmt.Errorf("claude CLI not found on PATH: %w", err)
	}
	return path, nil
}

// RunIteration runs one `claude -p` invocation to completion: streams its
// stdout (NDJSON) line-by-line to both rawLog (the full untruncated record)
// and renderer (the live truncated display), streams stderr straight to
// stderrLog, and returns the terminal ResultEvent if one was captured, plus
// the last rate_limit_event seen (if any) so callers can tell a rate-limit
// rejection apart from any other kind of failure.
//
// A non-nil returned error means the process itself failed to start/run/exit
// cleanly. A nil error with a nil ResultEvent means the process exited
// cleanly but never emitted a `type:"result"` line — callers must treat that
// as a failed iteration too; it should not happen in practice but a stray
// crash or truncated output must never be silently treated as success.
func RunIteration(
	ctx context.Context,
	claudeBinary string,
	args []string,
	workDir string,
	rawLog io.Writer,
	stderrLog io.Writer,
	renderer Renderer,
	onParseError func(line string, err error),
) (*ResultEvent, *RateLimitInfo, error) {
	cmd := exec.CommandContext(ctx, claudeBinary, args...)
	cmd.Dir = workDir
	cmd.Stderr = stderrLog

	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return nil, nil, fmt.Errorf("wire stdout pipe: %w", err)
	}

	if err := cmd.Start(); err != nil {
		return nil, nil, fmt.Errorf("start claude: %w", err)
	}

	result, rateLimit, streamErr := streamStdout(stdout, rawLog, renderer, onParseError)
	waitErr := cmd.Wait()

	if waitErr != nil {
		return result, rateLimit, fmt.Errorf("claude exited with error: %w", waitErr)
	}
	if streamErr != nil {
		return result, rateLimit, fmt.Errorf("reading claude output: %w", streamErr)
	}
	return result, rateLimit, nil
}

// streamStdout reads NDJSON lines from r as they arrive, mirroring every
// byte read to rawLog (via io.TeeReader) before parsing, rendering non-result
// events live via renderer, and returning the last "result" event and the
// last "rate_limit_event" seen.
//
// Uses bufio.Reader.ReadString rather than bufio.Scanner deliberately:
// Scanner's default 64KB max token size would choke on (or silently drop) a
// single oversized line, e.g. a tool_result block carrying a large file's
// contents — ReadString has no such ceiling.
func streamStdout(r io.Reader, rawLog io.Writer, renderer Renderer, onParseError func(line string, err error)) (*ResultEvent, *RateLimitInfo, error) {
	br := bufio.NewReader(io.TeeReader(r, rawLog))
	var last *ResultEvent
	var lastRateLimit *RateLimitInfo

	for {
		line, readErr := br.ReadString('\n')

		trimmed := strings.TrimSpace(line)
		if trimmed != "" {
			ev, parseErr := ParseLine([]byte(trimmed))
			if parseErr != nil {
				if onParseError != nil {
					onParseError(trimmed, parseErr)
				}
			} else if ev.Type == "result" {
				last = ev.Result
			} else if ev.Type == "rate_limit_event" {
				lastRateLimit = ev.RateLimit
			} else if renderer != nil {
				renderer.Render(ev)
			}
		}

		if readErr != nil {
			if readErr == io.EOF {
				return last, lastRateLimit, nil
			}
			return last, lastRateLimit, fmt.Errorf("read claude stdout: %w", readErr)
		}
	}
}
