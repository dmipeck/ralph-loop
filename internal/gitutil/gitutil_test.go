package gitutil

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
)

// initRepo creates a real git repo in a temp dir with one empty commit, and
// returns its path. Shells out to the real `git` binary deliberately — fast
// and fully sandboxed to t.TempDir(), no fixture-faking needed.
func initRepo(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	run := func(args ...string) {
		t.Helper()
		cmd := exec.Command("git", args...)
		cmd.Dir = dir
		cmd.Env = append(os.Environ(),
			"GIT_AUTHOR_NAME=ralph-loop-test", "GIT_AUTHOR_EMAIL=test@example.com",
			"GIT_COMMITTER_NAME=ralph-loop-test", "GIT_COMMITTER_EMAIL=test@example.com",
		)
		out, err := cmd.CombinedOutput()
		if err != nil {
			t.Fatalf("git %v: %v\n%s", args, err, out)
		}
	}
	run("init", "-q")
	run("commit", "--allow-empty", "-q", "-m", "init")
	return dir
}

func TestHeadSHA(t *testing.T) {
	dir := initRepo(t)
	sha, err := HeadSHA(context.Background(), dir)
	if err != nil {
		t.Fatalf("HeadSHA: %v", err)
	}
	if len(sha) != 40 {
		t.Errorf("HeadSHA = %q, want a 40-char SHA", sha)
	}
}

func TestHeadSHA_ChangesAfterCommit(t *testing.T) {
	dir := initRepo(t)
	ctx := context.Background()

	before, err := HeadSHA(ctx, dir)
	if err != nil {
		t.Fatalf("HeadSHA (before): %v", err)
	}

	if err := os.WriteFile(filepath.Join(dir, "file.txt"), []byte("hello"), 0o644); err != nil {
		t.Fatal(err)
	}
	for _, args := range [][]string{{"add", "file.txt"}, {"commit", "-q", "-m", "add file"}} {
		cmd := exec.Command("git", args...)
		cmd.Dir = dir
		cmd.Env = append(os.Environ(), "GIT_AUTHOR_NAME=t", "GIT_AUTHOR_EMAIL=t@t.com", "GIT_COMMITTER_NAME=t", "GIT_COMMITTER_EMAIL=t@t.com")
		if out, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("git %v: %v\n%s", args, err, out)
		}
	}

	after, err := HeadSHA(ctx, dir)
	if err != nil {
		t.Fatalf("HeadSHA (after): %v", err)
	}
	if after == before {
		t.Error("expected HEAD to change after a new commit")
	}
}

func TestCommitSubject(t *testing.T) {
	dir := initRepo(t)
	ctx := context.Background()

	if err := os.WriteFile(filepath.Join(dir, "file.txt"), []byte("hello"), 0o644); err != nil {
		t.Fatal(err)
	}
	for _, args := range [][]string{{"add", "file.txt"}, {"commit", "-q", "-m", "add file\n\nmore body text"}} {
		cmd := exec.Command("git", args...)
		cmd.Dir = dir
		cmd.Env = append(os.Environ(), "GIT_AUTHOR_NAME=t", "GIT_AUTHOR_EMAIL=t@t.com", "GIT_COMMITTER_NAME=t", "GIT_COMMITTER_EMAIL=t@t.com")
		if out, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("git %v: %v\n%s", args, err, out)
		}
	}

	subject, err := CommitSubject(ctx, dir)
	if err != nil {
		t.Fatalf("CommitSubject: %v", err)
	}
	if subject != "add file" {
		t.Errorf("CommitSubject = %q, want %q", subject, "add file")
	}
}

func TestDiffStat(t *testing.T) {
	dir := initRepo(t)
	ctx := context.Background()

	before, err := HeadSHA(ctx, dir)
	if err != nil {
		t.Fatalf("HeadSHA (before): %v", err)
	}

	if err := os.WriteFile(filepath.Join(dir, "file.txt"), []byte("hello\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	for _, args := range [][]string{{"add", "file.txt"}, {"commit", "-q", "-m", "add file"}} {
		cmd := exec.Command("git", args...)
		cmd.Dir = dir
		cmd.Env = append(os.Environ(), "GIT_AUTHOR_NAME=t", "GIT_AUTHOR_EMAIL=t@t.com", "GIT_COMMITTER_NAME=t", "GIT_COMMITTER_EMAIL=t@t.com")
		if out, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("git %v: %v\n%s", args, err, out)
		}
	}

	after, err := HeadSHA(ctx, dir)
	if err != nil {
		t.Fatalf("HeadSHA (after): %v", err)
	}

	stat, err := DiffStat(ctx, dir, before, after)
	if err != nil {
		t.Fatalf("DiffStat: %v", err)
	}
	if stat == "" {
		t.Error("expected a non-empty diffstat for a changed file")
	}
}

func TestCreateTag(t *testing.T) {
	dir := initRepo(t)
	ctx := context.Background()

	head, err := HeadSHA(ctx, dir)
	if err != nil {
		t.Fatalf("HeadSHA: %v", err)
	}

	if err := CreateTag(ctx, dir, "ralph/iter-1-abc1234"); err != nil {
		t.Fatalf("CreateTag: %v", err)
	}

	tagSHA, err := run(ctx, dir, "rev-parse", "ralph/iter-1-abc1234")
	if err != nil {
		t.Fatalf("rev-parse tag: %v", err)
	}
	if tagSHA != head {
		t.Errorf("tag points at %q, want HEAD %q", tagSHA, head)
	}
}

func TestCreateTag_DuplicateFails(t *testing.T) {
	dir := initRepo(t)
	ctx := context.Background()

	if err := CreateTag(ctx, dir, "ralph/iter-1-abc1234"); err != nil {
		t.Fatalf("CreateTag (first): %v", err)
	}
	if err := CreateTag(ctx, dir, "ralph/iter-1-abc1234"); err == nil {
		t.Error("CreateTag (duplicate) = nil error, want non-nil")
	}
}

func TestCheckoutBranch_CreatesThenReuses(t *testing.T) {
	dir := initRepo(t)
	ctx := context.Background()

	if err := CheckoutBranch(ctx, dir, "ralph/test-branch"); err != nil {
		t.Fatalf("CheckoutBranch (create): %v", err)
	}
	got, err := CurrentBranch(ctx, dir)
	if err != nil {
		t.Fatalf("CurrentBranch: %v", err)
	}
	if got != "ralph/test-branch" {
		t.Fatalf("CurrentBranch = %q, want ralph/test-branch", got)
	}

	// Switch to a second branch, then ask for the first one again — should
	// reuse (checkout) it rather than erroring on "already exists".
	if err := CheckoutBranch(ctx, dir, "some-other-branch"); err != nil {
		t.Fatalf("CheckoutBranch (second branch): %v", err)
	}
	if err := CheckoutBranch(ctx, dir, "ralph/test-branch"); err != nil {
		t.Fatalf("CheckoutBranch (idempotent re-checkout): %v", err)
	}
	got, err = CurrentBranch(ctx, dir)
	if err != nil {
		t.Fatalf("CurrentBranch: %v", err)
	}
	if got != "ralph/test-branch" {
		t.Fatalf("CurrentBranch = %q, want ralph/test-branch", got)
	}
}
