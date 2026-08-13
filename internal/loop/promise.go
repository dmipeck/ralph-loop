package loop

import "regexp"
import "strings"

// promiseRe finds the FIRST <promise>...</promise> tag in a block of text.
// The non-greedy capture group means that with multiple tags present, only
// the first is ever extracted — deterministic, matching the original bash
// prototype's perl-based extraction.
var promiseRe = regexp.MustCompile(`(?s)<promise>(.*?)</promise>`)

var whitespaceRe = regexp.MustCompile(`\s+`)

// ExtractPromise returns the trimmed, whitespace-collapsed contents of the
// first <promise>...</promise> tag in text, or "" if there is none.
func ExtractPromise(text string) string {
	m := promiseRe.FindStringSubmatch(text)
	if m == nil {
		return ""
	}
	return whitespaceRe.ReplaceAllString(strings.TrimSpace(m[1]), " ")
}
