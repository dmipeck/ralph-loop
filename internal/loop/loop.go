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
	Iteration int
	Result    *claude.ResultEvent // nil if the process crashed / never emitted a result event
	RunErr    error               // non-nil if the claude subprocess itself failed
	Committed bool
	CommitRef string // short SHA, set iff Committed
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

		outcome, err := c.runOneIteration(ctx, claudeBinary, iteration, out, renderer)
		if err != nil && outcome.RunErr == nil {
			// A logging/IO-level failure distinct from the subprocess itself.
			return err
		}

		isError := outcome.Result != nil && outcome.Result.IsError
		cost := 0.0
		if outcome.Result != nil {
			cost = outcome.Result.TotalCostUSD
		}
		committed := "no"
		if outcome.Committed {
			committed = fmt.Sprintf("yes (%s)", outcome.CommitRef)
		}
		line := logging.FormatSummaryLine(time.Now(), iteration, isError, cost, committed)
		fmt.Fprintln(out, line)
		if err := summary.Append(line); err != nil {
			fmt.Fprintf(out, "warning: failed to write summary log: %v\n", err)
		}

		decision := c.Decide(outcome)
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
func (c *Controller) runOneIteration(ctx context.Context, claudeBinary string, iteration int, out io.Writer, renderer claude.Renderer) (Outcome, error) {
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

	var parseWarnings int
	onParseError := func(line string, perr error) {
		parseWarnings++
		fmt.Fprintf(out, "  ! could not parse a stream-json line (%v), see iteration-%d.jsonl\n", perr, iteration)
	}

	result, runErr := claude.RunIteration(ctx, claudeBinary, args, c.cfg.RepoDir, logs.Raw, logs.Stderr, renderer, onParseError)

	outcome := Outcome{Iteration: iteration, Result: result, RunErr: runErr}

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
