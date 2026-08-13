package cmd

import (
	"context"
	"fmt"
	"io"
	"os"
	"os/signal"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"
	"time"

	"github.com/dmipeck/ralph-loop/internal/claude"
	"github.com/dmipeck/ralph-loop/internal/config"
	"github.com/dmipeck/ralph-loop/internal/loop"
	"github.com/spf13/cobra"
)

var (
	promptFile           string
	maxIterations        int
	maxStalledIterations int
	completionPromise    string
	sleepDuration        time.Duration
	model                string
	permissionMode       string
	allowedTools         []string
	disallowedTools      []string
	skipPermissions      bool
	maxIterationCostUSD  float64
	maxTotalCostUSD      float64
	preflightCmd         string
	branch               string
	logDir               string
	dryRun               bool
)

// defaultAllowedTools is a generic, project-agnostic safety baseline:
// file-edit tools plus ordinary git usage. Anything project-specific (a
// test runner, a build command, ...) is additive via --allowed-tool —
// pflag's StringArray always appends to the default rather than replacing
// it, which is exactly the "baseline + additions" behavior wanted here.
var defaultAllowedTools = []string{"Read", "Write", "Edit", "Glob", "Grep", "Bash(git *)"}

var runCmd = &cobra.Command{
	Use:   "run",
	Short: "Run the loop until completion, max iterations, or a stall/budget limit",
	RunE:  runRun,
}

func init() {
	rootCmd.AddCommand(runCmd)

	flags := runCmd.Flags()
	flags.StringVar(&promptFile, "prompt-file", "", "Prompt fed to claude every iteration (required)")
	flags.IntVar(&maxIterations, "max-iterations", 40, "Stop after this many iterations, 0 = unlimited")
	flags.IntVar(&maxStalledIterations, "max-stalled-iterations", 3, "Abort after this many iterations in a row with no commit and no completion promise, 0 = disabled")
	flags.StringVar(&completionPromise, "completion-promise", "", "Exact phrase expected inside <promise>...</promise> (required)")
	flags.DurationVar(&sleepDuration, "sleep", 5*time.Second, "Pause between iterations")
	flags.StringVar(&model, "model", "", "Passed through to claude -p --model")
	flags.StringVar(&permissionMode, "permission-mode", "auto", "Passed through to claude -p --permission-mode")
	flags.StringArrayVar(&allowedTools, "allowed-tool", nil, "Tool pattern to allow, IN ADDITION to the built-in baseline (repeatable)")
	flags.StringArrayVar(&disallowedTools, "disallowed-tool", nil, "Tool pattern to disallow (repeatable)")
	flags.BoolVar(&skipPermissions, "dangerously-skip-permissions", false, "Use claude -p --dangerously-skip-permissions instead of --permission-mode/--allowed-tool")
	flags.Float64Var(&maxIterationCostUSD, "max-iteration-cost-usd", 0, "Passed through to claude -p --max-budget-usd, 0 = unset")
	flags.Float64Var(&maxTotalCostUSD, "max-total-cost-usd", 0, "Stop once cumulative reported cost across iterations reaches this, 0 = unset")
	flags.StringVar(&preflightCmd, "preflight-cmd", "", "Shell command that must exit 0 before each iteration, or the run aborts before spawning claude at all")
	flags.StringVar(&branch, "branch", "", "Checkout/create this branch in --repo-dir before the loop starts")
	flags.StringVar(&logDir, "log-dir", "", "Directory for iteration logs and the summary log (default: a subdirectory of the OS cache dir, deliberately outside --repo-dir)")
	flags.BoolVar(&dryRun, "dry-run", false, "Print what would run and exit")

	_ = runCmd.MarkFlagRequired("prompt-file")
	_ = runCmd.MarkFlagRequired("completion-promise")
	runCmd.MarkFlagsMutuallyExclusive("dangerously-skip-permissions", "permission-mode")
	runCmd.MarkFlagsMutuallyExclusive("dangerously-skip-permissions", "allowed-tool")
	runCmd.MarkFlagsMutuallyExclusive("dangerously-skip-permissions", "disallowed-tool")
}

func runRun(cmd *cobra.Command, _ []string) error {
	// --dangerously-skip-permissions wins outright: null out the other
	// permission fields regardless of their (possibly still-default)
	// values, so config.Validate never sees a spurious contradiction.
	if skipPermissions {
		permissionMode = ""
		allowedTools = nil
		disallowedTools = nil
	} else {
		// pflag's StringArrayVar *replaces* its default on the first
		// occurrence of the flag rather than appending to it, so the
		// "baseline + additions" merge has to happen here in code, not by
		// relying on defaultAllowedTools as the flag's default value.
		allowedTools = mergeUnique(defaultAllowedTools, allowedTools)
	}

	absRepoDir, err := filepath.Abs(repoDir)
	if err != nil {
		return fmt.Errorf("resolve --repo-dir: %w", err)
	}

	resolvedLogDir := logDir
	if resolvedLogDir == "" {
		cacheDir, err := os.UserCacheDir()
		if err != nil {
			cacheDir = os.TempDir()
		}
		resolvedLogDir = filepath.Join(cacheDir, "ralph-loop", filepath.Base(absRepoDir))
	}

	cfg := &config.Config{
		RepoDir:              absRepoDir,
		PromptFile:           promptFile,
		MaxIterations:        maxIterations,
		MaxStalledIterations: maxStalledIterations,
		CompletionPromise:    completionPromise,
		Sleep:                sleepDuration,
		Model:                model,
		PermissionMode:       permissionMode,
		AllowedTools:         allowedTools,
		DisallowedTools:      disallowedTools,
		SkipPermissions:      skipPermissions,
		MaxIterationCostUSD:  maxIterationCostUSD,
		MaxTotalCostUSD:      maxTotalCostUSD,
		PreflightCmd:         preflightCmd,
		Branch:               branch,
		LogDir:               resolvedLogDir,
		DryRun:               dryRun,
	}

	if err := cfg.Validate(); err != nil {
		return err
	}
	// Past this point, any failure is a runtime condition (bad prompt file
	// content, claude missing, budget exceeded, ...), not a flag mistake —
	// don't dump a Cobra usage block for it.
	cmd.SilenceUsage = true

	out := cmd.OutOrStdout()

	if dryRun {
		promptBytes, err := os.ReadFile(cfg.PromptFile)
		if err != nil {
			return fmt.Errorf("read prompt file: %w", err)
		}
		argv := claude.BuildArgs(cfg, string(promptBytes))
		fmt.Fprintln(out, "Ralph loop starting (dry run — not invoking claude)")
		printConfigSummary(out, cfg)
		fmt.Fprintf(out, "\nclaude %s\n", shellJoinForDisplay(argv))
		return nil
	}

	printConfigSummary(out, cfg)

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	controller := loop.NewController(cfg)
	return controller.Run(ctx, out)
}

func printConfigSummary(w io.Writer, cfg *config.Config) {
	fmt.Fprintln(w, "Ralph loop starting")
	fmt.Fprintf(w, "  repo dir:               %s\n", cfg.RepoDir)
	fmt.Fprintf(w, "  prompt file:            %s\n", cfg.PromptFile)

	maxIter := "unlimited"
	if cfg.MaxIterations > 0 {
		maxIter = strconv.Itoa(cfg.MaxIterations)
	}
	fmt.Fprintf(w, "  max iterations:         %s\n", maxIter)

	maxStalled := "disabled"
	if cfg.MaxStalledIterations > 0 {
		maxStalled = strconv.Itoa(cfg.MaxStalledIterations)
	}
	fmt.Fprintf(w, "  max stalled iterations: %s\n", maxStalled)
	fmt.Fprintf(w, "  completion promise:     %s\n", cfg.CompletionPromise)

	if cfg.SkipPermissions {
		fmt.Fprintln(w, "  permissions:            dangerously-skip-permissions")
	} else {
		fmt.Fprintf(w, "  permissions:            mode=%s allowed=%v disallowed=%v\n",
			cfg.PermissionMode, cfg.AllowedTools, cfg.DisallowedTools)
	}
	if cfg.Branch != "" {
		fmt.Fprintf(w, "  branch:                 %s\n", cfg.Branch)
	}
	if cfg.PreflightCmd != "" {
		fmt.Fprintf(w, "  preflight command:      %s\n", cfg.PreflightCmd)
	}
	fmt.Fprintf(w, "  log dir:                %s\n", cfg.LogDir)
	fmt.Fprintln(w)
}

// mergeUnique concatenates base and extra, dropping exact duplicates while
// preserving first-seen order — so passing a pattern already in the
// baseline (e.g. --allowed-tool 'Bash(git *)') doesn't produce a
// doubled-up --allowedTools argv.
func mergeUnique(base, extra []string) []string {
	seen := make(map[string]bool, len(base)+len(extra))
	out := make([]string, 0, len(base)+len(extra))
	for _, s := range append(append([]string{}, base...), extra...) {
		if !seen[s] {
			seen[s] = true
			out = append(out, s)
		}
	}
	return out
}

// shellJoinForDisplay renders argv for human eyes only (--dry-run output) —
// never fed back into a shell, so this only needs to be readable, not
// round-trippable.
func shellJoinForDisplay(argv []string) string {
	parts := make([]string, len(argv))
	for i, a := range argv {
		if strings.ContainsAny(a, " \t\n") {
			parts[i] = "'" + a + "'"
		} else {
			parts[i] = a
		}
	}
	return strings.Join(parts, " ")
}
