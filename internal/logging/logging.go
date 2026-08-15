// Package logging manages per-iteration raw/stderr log files and the
// running plaintext summary log.
package logging

import (
	"fmt"
	"os"
	"path/filepath"
	"time"
)

// IterationLogs holds the two files a single iteration writes to: the full
// untruncated NDJSON stream, and anything claude wrote to stderr.
type IterationLogs struct {
	Raw    *os.File
	Stderr *os.File
}

// OpenIterationLogs creates (or truncates) iteration-<n>.jsonl and
// iteration-<n>.stderr.log under logDir, creating logDir if needed.
func OpenIterationLogs(logDir string, iteration int) (*IterationLogs, error) {
	if err := os.MkdirAll(logDir, 0o755); err != nil {
		return nil, fmt.Errorf("create log dir %q: %w", logDir, err)
	}

	raw, err := os.Create(filepath.Join(logDir, fmt.Sprintf("iteration-%d.jsonl", iteration)))
	if err != nil {
		return nil, fmt.Errorf("create raw log: %w", err)
	}

	stderr, err := os.Create(filepath.Join(logDir, fmt.Sprintf("iteration-%d.stderr.log", iteration)))
	if err != nil {
		raw.Close()
		return nil, fmt.Errorf("create stderr log: %w", err)
	}

	return &IterationLogs{Raw: raw, Stderr: stderr}, nil
}

// Close closes both files, returning the first error encountered (if any)
// after attempting both.
func (l *IterationLogs) Close() error {
	rawErr := l.Raw.Close()
	stderrErr := l.Stderr.Close()
	if rawErr != nil {
		return rawErr
	}
	return stderrErr
}

// SummaryLogger appends one line per iteration to a running plaintext log.
type SummaryLogger struct {
	path string
}

// NewSummaryLogger prepares a summary logger writing to logDir/ralph.log,
// creating logDir if needed.
func NewSummaryLogger(logDir string) (*SummaryLogger, error) {
	if err := os.MkdirAll(logDir, 0o755); err != nil {
		return nil, fmt.Errorf("create log dir %q: %w", logDir, err)
	}
	return &SummaryLogger{path: filepath.Join(logDir, "ralph.log")}, nil
}

// Append writes one line (a trailing newline is added) to the summary log.
func (s *SummaryLogger) Append(line string) error {
	f, err := os.OpenFile(s.path, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o644)
	if err != nil {
		return fmt.Errorf("open summary log: %w", err)
	}
	defer f.Close()
	if _, err := f.WriteString(line + "\n"); err != nil {
		return fmt.Errorf("write summary log: %w", err)
	}
	return nil
}

// FormatSummaryLine builds one summary-log line in the same shape the
// original bash prototype used, for continuity when reading old and new
// logs side by side, extended with the running cost total across the whole
// run and (when a commit was made) its subject line.
func FormatSummaryLine(ts time.Time, iteration int, isError bool, costUSD float64, committed string, totalCostUSD float64, commitSubject string) string {
	line := fmt.Sprintf(
		"%s | iter=%d | is_error=%v | cost=$%.6f | committed=%s | total_cost=$%.6f",
		ts.UTC().Format(time.RFC3339), iteration, isError, costUSD, committed, totalCostUSD,
	)
	if commitSubject != "" {
		line += fmt.Sprintf(" | subject=%q", commitSubject)
	}
	return line
}
