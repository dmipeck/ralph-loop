package cmd

import (
	"reflect"
	"testing"
)

// TestTagOnCommitFlag_RegisteredWithDefaultFalse guards the --tag-on-commit
// flag's registration: opt-in git tagging must default to off, consistent
// with --branch/--preflight-cmd already being explicit opt-ins for anything
// that shapes git history.
func TestTagOnCommitFlag_RegisteredWithDefaultFalse(t *testing.T) {
	f := rootCmd.Flags().Lookup("tag-on-commit")
	if f == nil {
		t.Fatal("--tag-on-commit flag not registered")
	}
	if f.DefValue != "false" {
		t.Errorf("--tag-on-commit default = %q, want %q", f.DefValue, "false")
	}
}

// TestMergeUnique_PreservesBaselineAndDedupes is a regression test for a
// real bug found during manual smoke testing: pflag's StringArrayVar
// *replaces* its default value on the first occurrence of a repeatable
// flag rather than appending to it, so relying on
// `StringArrayVar(&allowedTools, "allowed-tool", defaultAllowedTools, ...)`
// silently threw away the safety baseline the moment a user passed even one
// --allowed-tool. mergeUnique is the fix — the merge happens explicitly in
// code instead.
func TestMergeUnique_PreservesBaselineAndDedupes(t *testing.T) {
	cases := []struct {
		name  string
		base  []string
		extra []string
		want  []string
	}{
		{
			name:  "user addition appends after the baseline",
			base:  []string{"Read", "Write", "Bash(git *)"},
			extra: []string{"Bash(npm test*)"},
			want:  []string{"Read", "Write", "Bash(git *)", "Bash(npm test*)"},
		},
		{
			name:  "re-specifying a baseline pattern does not duplicate it",
			base:  []string{"Read", "Write", "Bash(git *)"},
			extra: []string{"Bash(git *)"},
			want:  []string{"Read", "Write", "Bash(git *)"},
		},
		{
			name:  "no user additions leaves the baseline untouched",
			base:  []string{"Read", "Write", "Bash(git *)"},
			extra: nil,
			want:  []string{"Read", "Write", "Bash(git *)"},
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := mergeUnique(tc.base, tc.extra)
			if !reflect.DeepEqual(got, tc.want) {
				t.Errorf("mergeUnique(%v, %v) = %v, want %v", tc.base, tc.extra, got, tc.want)
			}
		})
	}
}
