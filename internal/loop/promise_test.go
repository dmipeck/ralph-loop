package loop

import "testing"

func TestExtractPromise(t *testing.T) {
	cases := []struct {
		name string
		text string
		want string
	}{
		{"exact match", "<promise>DONE</promise>", "DONE"},
		{"surrounded by other text", "Some summary.\n<promise>PLAN COMPLETE</promise>\n", "PLAN COMPLETE"},
		{"internal whitespace collapsed", "<promise>PLAN\n  COMPLETE</promise>", "PLAN COMPLETE"},
		{"leading/trailing whitespace trimmed", "<promise>  DONE  </promise>", "DONE"},
		{"absent", "No promise here, just a status update.", ""},
		{"empty tag", "<promise></promise>", ""},
		{"multiple tags: first wins", "<promise>FIRST</promise> text <promise>SECOND</promise>", "FIRST"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := ExtractPromise(tc.text); got != tc.want {
				t.Errorf("ExtractPromise(%q) = %q, want %q", tc.text, got, tc.want)
			}
		})
	}
}
