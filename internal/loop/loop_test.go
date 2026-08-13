package loop

import (
	"errors"
	"testing"

	"github.com/dmipeck/ralph-loop/internal/claude"
	"github.com/dmipeck/ralph-loop/internal/config"
)

func baseCfg() *config.Config {
	return &config.Config{
		CompletionPromise:    "ALL DONE",
		MaxIterations:        5,
		MaxStalledIterations: 3,
		MaxTotalCostUSD:      10.0,
	}
}

func resultOutcome(iteration int, text string, cost float64, committed bool) Outcome {
	return Outcome{
		Iteration: iteration,
		Result:    &claude.ResultEvent{Result: text, TotalCostUSD: cost},
		Committed: committed,
	}
}

func TestDecide_PromiseMatchStopsImmediately(t *testing.T) {
	c := NewController(baseCfg())
	d := c.Decide(resultOutcome(1, "<promise>ALL DONE</promise>", 0.1, false))
	if !d.Stop {
		t.Fatal("expected Stop=true on promise match")
	}
	if d.Reason == "" {
		t.Error("expected a non-empty reason")
	}
}

func TestDecide_WrongPromisePhraseDoesNotStop(t *testing.T) {
	c := NewController(baseCfg())
	d := c.Decide(resultOutcome(1, "<promise>WRONG PHRASE</promise>", 0.1, false))
	if d.Stop {
		t.Fatal("did not expect Stop on a promise tag with the wrong phrase")
	}
}

func TestDecide_CommitResetsStalledCounter(t *testing.T) {
	cfg := baseCfg()
	cfg.MaxStalledIterations = 2
	c := NewController(cfg)

	c.Decide(resultOutcome(1, "no promise", 0.1, false)) // stalled=1
	c.Decide(resultOutcome(2, "no promise", 0.1, true))  // committed -> stalled=0
	if c.StalledCount() != 0 {
		t.Fatalf("StalledCount = %d, want 0 after a committed iteration", c.StalledCount())
	}
	d := c.Decide(resultOutcome(3, "no promise", 0.1, false)) // stalled=1
	if d.Stop {
		t.Fatal("did not expect Stop: only 1 stalled iteration since the last commit, threshold is 2")
	}
}

func TestDecide_StalledStopsExactlyAtThreshold(t *testing.T) {
	cfg := baseCfg()
	cfg.MaxStalledIterations = 3
	c := NewController(cfg)

	var d Decision
	for i := 1; i <= 2; i++ {
		d = c.Decide(resultOutcome(i, "no promise", 0.1, false))
		if d.Stop {
			t.Fatalf("did not expect Stop at stalled=%d (threshold=3)", i)
		}
	}
	d = c.Decide(resultOutcome(3, "no promise", 0.1, false)) // stalled=3, hits threshold
	if !d.Stop {
		t.Fatal("expected Stop once stalled count reaches the threshold")
	}
}

func TestDecide_RunErrorAndNilResultCountAsStalled(t *testing.T) {
	cfg := baseCfg()
	cfg.MaxStalledIterations = 2
	c := NewController(cfg)

	c.Decide(Outcome{Iteration: 1, RunErr: errors.New("boom")})
	d := c.Decide(Outcome{Iteration: 2, Result: nil})
	if !d.Stop {
		t.Fatal("expected Stop: two consecutive hard failures should count toward the stall threshold")
	}
}

func TestDecide_MaxIterationsStopsExactlyAtLimit(t *testing.T) {
	cfg := baseCfg()
	cfg.MaxIterations = 3
	cfg.MaxStalledIterations = 0 // disabled, so only max-iterations can trigger
	c := NewController(cfg)

	// Commit every time so the stall counter never factors in.
	d := c.Decide(resultOutcome(1, "no promise", 0.1, true))
	if d.Stop {
		t.Fatalf("did not expect Stop at iteration 1 (max=3)")
	}
	d = c.Decide(resultOutcome(2, "no promise", 0.1, true))
	if d.Stop {
		t.Fatalf("did not expect Stop at iteration 2 (max=3)")
	}
	d = c.Decide(resultOutcome(3, "no promise", 0.1, true))
	if !d.Stop {
		t.Fatal("expected Stop at iteration 3 (max=3)")
	}
}

func TestDecide_TotalCostBudgetStopsOnceReached(t *testing.T) {
	cfg := baseCfg()
	cfg.MaxStalledIterations = 0
	cfg.MaxIterations = 0
	cfg.MaxTotalCostUSD = 1.0
	c := NewController(cfg)

	d := c.Decide(resultOutcome(1, "no promise", 0.6, true))
	if d.Stop {
		t.Fatalf("did not expect Stop: cumulative cost 0.6 < budget 1.0")
	}
	d = c.Decide(resultOutcome(2, "no promise", 0.4, true)) // cumulative = 1.0
	if !d.Stop {
		t.Fatal("expected Stop once cumulative cost reaches the budget")
	}
}

func TestDecide_ZeroDisablesStallAndIterationLimits(t *testing.T) {
	cfg := baseCfg()
	cfg.MaxStalledIterations = 0
	cfg.MaxIterations = 0
	cfg.MaxTotalCostUSD = 0
	c := NewController(cfg)

	for i := 1; i <= 20; i++ {
		d := c.Decide(resultOutcome(i, "no promise", 100, false))
		if d.Stop {
			t.Fatalf("did not expect Stop at iteration %d: all limits disabled (0)", i)
		}
	}
}
