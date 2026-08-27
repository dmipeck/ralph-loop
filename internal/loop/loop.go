// Package loop implements the ralph-loop iteration state machine: run a
// fresh `claude -p` process, look at what happened, decide whether to keep
// going. The decision logic (Decide) is pure and independently testable;
// Run wires it to the real subprocess/git/logging machinery.
package loop

import (
	"context"
	"fmt"
	"io"
	"os/exec"
	"time"

	"github.com/dmipeck/ralph-loop/internal/claude"
	"github.com/dmipeck/ralph-loop/internal/config"
	"github.com/dmipeck/ralph-loop/internal/display"
	"github.com/dmipeck/ralph-loop/internal/gitutil"
	"github.com/dmipeck/ralph-loop/internal/logging"
	"github.com/dmipeck/ralph-loop/internal/prompt"
)

// Outcome is what one iteration produced, as input to Decide.
type Outcome struct {
	Iteration     int
	Result        *claude.ResultEvent   // nil if the process crashed / never emitted a result event
	RunErr        error                 // non-nil if the claude subprocess itself failed
	RateLimit     *claude.RateLimitInfo // last rate_limit_event seen this iteration, if any
	Committed     bool
	CommitRef     string // short SHA, set iff Committed
	CommitSubject string // commit message subject line, best-effort, set iff Committed
	DiffStat      string // `git diff --shortstat` summary, best-effort, set iff Committed
	Tag           string // set iff a tag was created this iteration (only possible if Committed)
}

// rateLimited reports whether o represents an iteration that never produced
// a real answer because the API rejected it for being rate-limited, as
// opposed to any other kind of failure (a bug, a crash, max-turns, ...).
func rateLimited(o Outcome) bool {
	return (o.RunErr != nil || o.Result == nil) && o.RateLimit != nil && o.RateLimit.Rejected()
}

const (
	// rateLimitBuffer is added on top of the API-reported reset time to
	// absorb clock skew between this machine and the API, so we don't come
	// back online a few seconds early and get rejected again immediately.
	rateLimitBuffer = 15 * time.Second

	// rateLimitFallbackWait is used when a rejected rate_limit_event
	// carried no parseable reset time. Should be rare — resetsAt is
	// normally present whenever status is "rejected" — but backing off and
	// retrying beats spinning on every claude invocation.
	rateLimitFallbackWait = 5 * time.Minute

	// maxConsecutiveRateLimitRetries caps how many times in a row we'll
	// wait out a reported reset and retry the same iteration before giving
	// up entirely. A transient rate limit clears after one wait; still
	// being rejected after several full reset windows have passed means
	// something else is wrong (suspended account, org spend cap, ...) that
	// waiting can't fix.
	maxConsecutiveRateLimitRetries = 5
)

// rateLimitWait computes how long to sleep before retrying a rate-limited
// iteration: until the API-reported reset time (plus a buffer for clock
// skew), or a fixed fallback if no reset time was reported at all.
func rateLimitWait(now time.Time, info *claude.RateLimitInfo) time.Duration {
	resetsAt, ok := info.ResetsAtTime()
	if !ok {
		return rateLimitFallbackWait
	}
	wait := resetsAt.Sub(now) + rateLimitBuffer
	if wait < 0 {
		wait = rateLimitBuffer
	}
	return wait
}

// tagNameForCommit builds this iteration's git tag name.
func tagNameForCommit(iteration int, commitRef string) string {
	return fmt.Sprintf("ralph/iter-%d-%s", iteration, commitRef)
}

// Decision is what Decide concluded after folding in one Outcome.
type Decision struct {
	Stop   bool
	Reason string
}

// Controller holds cumulative loop state (stall count, running cost total)
// across iterations. Decide is pure and safe to drive with synthetic
// Outcomes in tests; Run is the real orchestration loop.
type Controller struct {
	cfg          *config.Config
	stalledCount int
	totalCostUSD float64
}

func NewController(cfg *config.Config) *Controller {
	return &Controller{cfg: cfg}
}

func (c *Controller) TotalCostUSD() float64 { return c.totalCostUSD }
func (c *Controller) StalledCount() int     { return c.stalledCount }

// Decide folds one iteration's Outcome into the controller's running state
// and returns whether the loop should stop, and why. Order of checks
// matters: a genuine completion promise always wins over stall/budget/max-
// iteration bookkeeping computed from the very same outcome.
func (c *Controller) Decide(o Outcome) Decision {
	if o.RunErr != nil || o.Result == nil {
		c.stalledCount++
	} else {
		c.totalCostUSD += o.Result.TotalCostUSD

		if o.Committed {
			c.stalledCount = 0
		} else {
			c.stalledCount++
		}

		if promise := ExtractPromise(o.Result.Result); promise != "" && promise == c.cfg.CompletionPromise {
			return Decision{Stop: true, Reason: fmt.Sprintf("completion promise detected: %s", promise)}
		}
	}

	if c.cfg.MaxStalledIterations > 0 && c.stalledCount >= c.cfg.MaxStalledIterations {
		return Decision{Stop: true, Reason: fmt.Sprintf(
			"%d iteration(s) in a row made no commit and no completion promise — likely stuck", c.stalledCount)}
	}

	if c.cfg.MaxTotalCostUSD > 0 && c.totalCostUSD >= c.cfg.MaxTotalCostUSD {
		return Decision{Stop: true, Reason: fmt.Sprintf(
			"cumulative cost $%.4f reached the $%.4f budget", c.totalCostUSD, c.cfg.MaxTotalCostUSD)}
	}

	if c.cfg.MaxIterations > 0 && o.Iteration >= c.cfg.MaxIterations {
		return Decision{Stop: true, Reason: fmt.Sprintf("max iterations (%d) reached", c.cfg.MaxIterations)}
	}

	return Decision{Stop: false}
}

// Run drives the real loop: preflight check, spawn+stream, git commit
// detection, Decide, sleep, repeat — until Decide says stop or ctx is
// cancelled.
func (c *Controller) Run(ctx context.Context, out io.Writer) error {
	if c.cfg.Branch != "" {
		if err := gitutil.CheckoutBranch(ctx, c.cfg.RepoDir, c.cfg.Branch); err != nil {
			return fmt.Errorf("checkout branch %q: %w", c.cfg.Branch, err)
		}
		fmt.Fprintf(out, "Checked out branch %q\n", c.cfg.Branch)
	}

	claudeBinary, err := claude.LookPath()
	if err != nil {
		return err
	}

	summary, err := logging.NewSummaryLogger(c.cfg.LogDir)
	if err != nil {
		return err
	}

	renderer := &display.Renderer{Out: out}

	// Note: the max-iterations stop condition is enforced by Decide, called
	// at the end of each completed iteration below — there's no separate
	// pre-check here, since Decide always catches the boundary first.
	iteration := 0
	rateLimitRetries := 0
	for {
		iteration++

		select {
		case <-ctx.Done():
			fmt.Fprintf(out, "\nInterrupted before iteration %d. Resume any time — the prompt file and git history are the only state.\n", iteration)
			return ctx.Err()
		default:
		}

		if c.cfg.PreflightCmd != "" {
			if err := runPreflight(ctx, c.cfg.PreflightCmd, c.cfg.RepoDir); err != nil {
				fmt.Fprintf(out, "Preflight command failed, aborting before spawning claude: %v\n", err)
				return fmt.Errorf("preflight command failed: %w", err)
			}
		}

		fmt.Fprintf(out, "── Iteration %d ──────────────────────────────────────\n", iteration)
		fmt.Fprintf(out, "RALPH_ITERATION_START iter=%d total_cost_so_far=$%.4f\n", iteration, c.totalCostUSD)

		outcome, err := c.runOneIteration(ctx, claudeBinary, iteration, out, renderer)
		if err != nil && outcome.RunErr == nil {
			// A logging/IO-level failure distinct from the subprocess itself.
			return err
		}

		// A rate-limit rejection never reaches Decide: it isn't a stall (the
		// model didn't get a chance to do anything) and hammering claude
		// again in --sleep seconds would just get rejected again. Wait out
		// the reported reset instead and retry this same iteration number.
		if rateLimited(outcome) {
			rateLimitRetries++
			if rateLimitRetries > maxConsecutiveRateLimitRetries {
				fmt.Fprintf(out, "\nGiving up after %d consecutive rate-limit rejections in a row — this no longer looks like a transient rate limit. Check your Claude account's usage/billing status.\n", rateLimitRetries-1)
				return fmt.Errorf("rate limited %d times in a row, giving up", rateLimitRetries-1)
			}

			wait := rateLimitWait(time.Now(), outcome.RateLimit)
			resetDesc := "an unknown time (no reset time reported)"
			if t, ok := outcome.RateLimit.ResetsAtTime(); ok {
				resetDesc = t.Local().Format(time.RFC1123)
			}
			fmt.Fprintf(out, "Rate limited by the Claude API (%s) — resets around %s. Sleeping %s instead of retrying immediately (attempt %d/%d).\n",
				outcome.RateLimit.RateLimitType, resetDesc, wait.Round(time.Second), rateLimitRetries, maxConsecutiveRateLimitRetries)

			select {
			case <-ctx.Done():
				fmt.Fprintf(out, "\nInterrupted while waiting for the rate limit to lift.\n")
				return ctx.Err()
			case <-time.After(wait):
			}

			iteration-- // this attempt never really ran; retry the same iteration number
			continue
		}
		rateLimitRetries = 0

		// Decide first: it accumulates c.totalCostUSD as a side effect, and
		// the summary line below wants that running total, not just this
		// iteration's own cost.
		decision := c.Decide(outcome)

		isError := outcome.Result != nil && outcome.Result.IsError
		cost := 0.0
		if outcome.Result != nil {
			cost = outcome.Result.TotalCostUSD
		}
		committed := "no"
		if outcome.Committed {
			committed = fmt.Sprintf("yes (%s)", outcome.CommitRef)
		}
		line := logging.FormatSummaryLine(time.Now(), iteration, isError, cost, committed, c.totalCostUSD, outcome.CommitSubject)
		fmt.Fprintln(out, line)
		if err := summary.Append(line); err != nil {
			fmt.Fprintf(out, "warning: failed to write summary log: %v\n", err)
		}

		if decision.Stop {
			fmt.Fprintf(out, "\nRalph loop stopped after %d iteration(s): %s\n", iteration, decision.Reason)
			fmt.Fprintf(out, "Total reported cost: $%.4f\n", c.totalCostUSD)
			return nil
		}

		if c.cfg.Sleep > 0 {
			select {
			case <-ctx.Done():
				fmt.Fprintf(out, "\nInterrupted after iteration %d.\n", iteration)
				return ctx.Err()
			case <-time.After(c.cfg.Sleep):
			}
		}
	}
}

// runOneIteration performs the actual subprocess spawn + git before/after
// comparison for a single iteration.
func (c *Controller) runOneIteration(ctx context.Context, claudeBinary string, iteration int, out io.Writer, renderer *display.Renderer) (Outcome, error) {
	before, err := gitutil.HeadSHA(ctx, c.cfg.RepoDir)
	if err != nil {
		return Outcome{Iteration: iteration}, fmt.Errorf("git rev-parse HEAD (before): %w", err)
	}

	promptText, err := c.cfg.ReadPrompt()
	if err != nil {
		return Outcome{Iteration: iteration}, err
	}

	logs, err := logging.OpenIterationLogs(c.cfg.LogDir, iteration)
	if err != nil {
		return Outcome{Iteration: iteration}, err
	}
	defer logs.Close()

	fullPrompt := prompt.Compose(c.cfg.CompletionPromise, promptText)
	args := claude.BuildArgs(c.cfg, fullPrompt)

	renderer.BeginIteration(iteration)

	var parseWarnings int
	onParseError := func(line string, perr error) {
		parseWarnings++
		fmt.Fprintf(out, "  ! could not parse a stream-json line (%v), see iteration-%d.jsonl\n", perr, iteration)
	}

	result, rateLimit, runErr := claude.RunIteration(ctx, claudeBinary, args, c.cfg.RepoDir, logs.Raw, logs.Stderr, renderer, onParseError)

	outcome := Outcome{Iteration: iteration, Result: result, RunErr: runErr, RateLimit: rateLimit}

	if runErr != nil {
		fmt.Fprintf(out, "claude exited with an error: %v (see iteration-%d.jsonl / iteration-%d.stderr.log)\n", runErr, iteration, iteration)
		return outcome, nil
	}
	if result == nil {
		fmt.Fprintf(out, "claude produced no result event (see iteration-%d.jsonl / iteration-%d.stderr.log)\n", iteration, iteration)
		return outcome, nil
	}

	after, err := gitutil.HeadSHA(ctx, c.cfg.RepoDir)
	if err != nil {
		return outcome, fmt.Errorf("git rev-parse HEAD (after): %w", err)
	}
	if after != before {
		outcome.Committed = true
		if short, err := gitutil.LastCommitShort(ctx, c.cfg.RepoDir); err == nil {
			outcome.CommitRef = short
		}
		if subject, err := gitutil.CommitSubject(ctx, c.cfg.RepoDir); err == nil {
			outcome.CommitSubject = subject
		}
		if stat, err := gitutil.DiffStat(ctx, c.cfg.RepoDir, before, after); err == nil {
			outcome.DiffStat = stat
		}
		if c.cfg.TagOnCommit {
			tagName := tagNameForCommit(iteration, outcome.CommitRef)
			if err := gitutil.CreateTag(ctx, c.cfg.RepoDir, tagName); err == nil {
				outcome.Tag = tagName
			} else {
				fmt.Fprintf(out, "warning: failed to create tag %q: %v\n", tagName, err)
			}
		}
		fmt.Fprintf(out, "RALPH_CHANGES iter=%d commit=%s subject=%q diffstat=%q tag=%q\n",
			iteration, outcome.CommitRef, outcome.CommitSubject, outcome.DiffStat, outcome.Tag)
	}

	return outcome, nil
}

// runPreflight runs an arbitrary shell command that must succeed before an
// iteration is allowed to spawn claude at all — e.g. "is the dev database
// reachable". Kept generic and project-agnostic: callers supply whatever
// check makes sense for their repo via --preflight-cmd.
func runPreflight(ctx context.Context, command, workDir string) error {
	cmd := exec.CommandContext(ctx, "sh", "-c", command)
	cmd.Dir = workDir
	out, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("%w\n%s", err, out)
	}
	return nil
}
