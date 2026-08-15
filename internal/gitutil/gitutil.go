// Package gitutil wraps the handful of plain `git` invocations ralph-loop
// needs to do itself, directly, with the invoking user's own permissions —
// never routed through claude's tool-permission layer. That's deliberate:
// branch isolation only means something if it happens before claude ever
// gets a chance to touch the repo.
package gitutil

import (
	"context"
	"fmt"
	"os/exec"
	"strings"
)

func run(ctx context.Context, repoDir string, args ...string) (string, error) {
	cmd := exec.CommandContext(ctx, "git", args...)
	cmd.Dir = repoDir
	out, err := cmd.CombinedOutput()
	if err != nil {
		return "", fmt.Errorf("git %s: %w: %s", strings.Join(args, " "), err, strings.TrimSpace(string(out)))
	}
	return strings.TrimSpace(string(out)), nil
}

// HeadSHA returns the full SHA of HEAD in repoDir.
func HeadSHA(ctx context.Context, repoDir string) (string, error) {
	return run(ctx, repoDir, "rev-parse", "HEAD")
}

// LastCommitShort returns the abbreviated SHA of HEAD, for display.
func LastCommitShort(ctx context.Context, repoDir string) (string, error) {
	return run(ctx, repoDir, "log", "-1", "--format=%h")
}

// CommitSubject returns the subject line (first line) of HEAD's commit
// message, for display.
func CommitSubject(ctx context.Context, repoDir string) (string, error) {
	return run(ctx, repoDir, "log", "-1", "--format=%s")
}

// DiffStat returns a one-line `--shortstat` summary (files changed,
// insertions, deletions) of everything between before and after, for
// display. Empty (with no error) if the two refs are identical.
func DiffStat(ctx context.Context, repoDir, before, after string) (string, error) {
	return run(ctx, repoDir, "diff", "--shortstat", before, after)
}

// CreateTag creates a lightweight (non-annotated) tag named name
// pointing at HEAD in repoDir. Best-effort by design: callers should
// treat a non-nil error as non-fatal, not abort the iteration.
func CreateTag(ctx context.Context, repoDir, name string) error {
	_, err := run(ctx, repoDir, "tag", name)
	return err
}

// CurrentBranch returns the currently checked-out branch name in repoDir.
func CurrentBranch(ctx context.Context, repoDir string) (string, error) {
	return run(ctx, repoDir, "branch", "--show-current")
}

// branchExists reports whether branch already exists locally in repoDir.
func branchExists(ctx context.Context, repoDir, branch string) bool {
	cmd := exec.CommandContext(ctx, "git", "rev-parse", "--verify", "refs/heads/"+branch)
	cmd.Dir = repoDir
	return cmd.Run() == nil
}

// CheckoutBranch checks out branch in repoDir, creating it from the current
// HEAD first if it doesn't exist yet. Idempotent: calling it again for a
// branch ralph-loop already created and is already on is a no-op.
func CheckoutBranch(ctx context.Context, repoDir, branch string) error {
	current, err := CurrentBranch(ctx, repoDir)
	if err == nil && current == branch {
		return nil
	}

	if branchExists(ctx, repoDir, branch) {
		_, err := run(ctx, repoDir, "checkout", branch)
		return err
	}

	_, err = run(ctx, repoDir, "checkout", "-b", branch)
	return err
}
