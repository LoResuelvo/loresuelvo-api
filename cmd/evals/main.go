// Command evals runs explicitly selected local evaluations.
package main

import (
	"context"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"os/exec"
	"os/signal"
	"path/filepath"
	"strings"
	"syscall"
	"time"

	"github.com/LoResuelvo/loresuelvo-api/internal/evals"
	"github.com/google/uuid"
)

const defaultDataset = "evals/datasets/LoResuelvo_US60_evals_v1.0.0"

func main() {
	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer cancel()
	os.Exit(runContext(ctx, os.Args[1:], os.Stdout, os.Stderr))
}
func run(args []string, stdout, stderr io.Writer) int {
	return runContext(context.Background(), args, stdout, stderr)
}

func runContext(ctx context.Context, args []string, stdout, stderr io.Writer) int {
	if len(args) == 0 {
		usage(stderr)
		return 2
	}
	if args[0] == "help" || args[0] == "--help" || args[0] == "-h" {
		usage(stdout)
		return 0
	}
	command := args[0]
	switch command {
	case "validate", "plan", "contract", "live", "replay", "compare", "baselines", "summary", "review-template", "review", "metamorphic-report":
	default:
		fmt.Fprintf(stderr, "unsupported command %q; no model calls were made\n", command)
		return 2
	}
	flags := flag.NewFlagSet(command, flag.ContinueOnError)
	flags.SetOutput(stderr)
	root := flags.String("dataset", defaultDataset, "immutable dataset directory")
	var options evals.PlanOptions
	var comparisonOptions evals.CompareOptions
	var limits evals.ExecutionLimits
	var allowLive, dryRun bool
	var tokens int
	var output, runDirectory, left, right, caseIDs, reviewPath string
	if command == "plan" || command == "live" {
		flags.StringVar(&options.Suite, "suite", "", "explicit suite: smoke, development, holdout, critical_all")
		flags.StringVar(&options.Model, "model", "", "explicit requested model identifier")
		flags.IntVar(&options.Trials, "trials", 0, "repetitions per base case (default: frozen experiment configuration)")
		flags.IntVar(&options.MaxRetries, "max-retries", 0, "maximum retries per execution")
		flags.IntVar(&options.MaxRequests, "max-requests", 0, "positive request ceiling, including retries")
		flags.BoolVar(&options.AllowHoldout, "allow-holdout", false, "explicitly authorize reserve cases")
		flags.BoolVar(&options.Metamorphic, "metamorphic", false, "explicitly include frozen ranking transformations; requires frozen repeat count")
		flags.StringVar(&caseIDs, "cases", "", "optional comma-separated case subset of the selected suite")
	}
	if command == "live" {
		flags.BoolVar(&allowLive, "allow-live", false, "authorize provider calls for this invocation")
		flags.BoolVar(&dryRun, "dry-run", false, "print plan and limits without credentials or provider calls")
		flags.IntVar(&limits.Concurrency, "concurrency", 1, "maximum simultaneous executions (currently 1 only)")
		flags.DurationVar(&limits.AttemptTimeout, "attempt-timeout", 0, "positive timeout per attempt")
		flags.DurationVar(&limits.GlobalTimeout, "global-timeout", 0, "positive timeout for the run")
		flags.DurationVar(&limits.MinInterval, "min-interval", 0, "positive minimum interval between attempts")
		flags.IntVar(&tokens, "max-output-tokens", 0, "positive maximum output tokens per generation")
		flags.StringVar(&output, "out", "", "new run directory (must not exist)")
	} else if command == "replay" || command == "summary" || command == "review-template" || command == "review" || command == "metamorphic-report" {
		flags.StringVar(&runDirectory, "run", "", "completed run directory")
		if command == "review" {
			flags.StringVar(&reviewPath, "reviews", "", "semantic review JSON file")
		}
	} else if command == "compare" {
		flags.StringVar(&left, "left", "", "left completed run directory")
		flags.StringVar(&right, "right", "", "right completed run directory")
		flags.BoolVar(&comparisonOptions.AllowSourceChange, "allow-source-change", false, "declare intentional source commit change")
		flags.BoolVar(&comparisonOptions.AllowPromptChange, "allow-prompt-change", false, "declare intentional effective prompt/input change")
		flags.BoolVar(&comparisonOptions.AllowGenerationConfigChange, "allow-generation-config-change", false, "declare intentional generation configuration change")
		flags.StringVar(&comparisonOptions.ChangeDescription, "change-description", "", "explain deliberate comparison changes")
	}
	if command == "baselines" {
		flags.StringVar(&options.Suite, "suite", "", "explicit suite")
		flags.BoolVar(&options.AllowHoldout, "allow-holdout", false, "authorize reserve scoring")
	}
	if err := flags.Parse(args[1:]); err != nil {
		if errors.Is(err, flag.ErrHelp) {
			return 0
		}
		return 2
	}
	if flags.NArg() != 0 {
		fmt.Fprintln(stderr, "unexpected positional arguments")
		return 2
	}
	if command == "live" {
		tokenValue := tokens
		if tokenValue <= 0 || int64(tokenValue) > int64(1<<31-1) {
			fmt.Fprintln(stderr, "max-output-tokens must be a positive int32")
			return 2
		}
		limits.MaxOutputTokens = int32(tokenValue)
		if !dryRun && !allowLive {
			fmt.Fprintln(stderr, "live execution requires --allow-live; no model calls were made")
			return 2
		}
		if err := limits.Validate(); err != nil {
			fmt.Fprintln(stderr, err)
			return 2
		}
	}
	dataset, err := evals.LoadDataset(*root)
	if err != nil {
		fmt.Fprintln(stderr, err)
		return 2
	}
	var result any
	code := 0
	switch command {
	case "validate":
		result = map[string]any{"status": "passed", "scope": "manifest integrity, local JSON schemas, harness references and domain mapping; editorial policy checks also use evalpack.py", "dataset_version": dataset.Version, "dataset_manifest_sha256": dataset.ManifestSHA256, "prediagnosis_cases": len(dataset.PD), "ranking_cases": len(dataset.RK), "contract_specifications": len(dataset.CT), "live_model_calls": 0}
	case "plan", "live":
		trialsProvided := false
		flags.Visit(func(f *flag.Flag) {
			if f.Name == "trials" {
				trialsProvided = true
			}
		})
		if !trialsProvided {
			key := "release"
			if options.Suite == "smoke" {
				key = "smoke"
			} else if options.Suite == "development" {
				key = "baseline_development"
			}
			options.Trials = dataset.ExperimentTrials[key]
		}
		if caseIDs != "" {
			options.CaseIDs = strings.Split(caseIDs, ",")
			for i := range options.CaseIDs {
				options.CaseIDs[i] = strings.TrimSpace(options.CaseIDs[i])
			}
		}
		plan, planErr := evals.BuildPlan(dataset, options)
		if planErr != nil {
			fmt.Fprintln(stderr, planErr)
			return 2
		}
		if command == "plan" {
			result = plan
			break
		}
		if dryRun {
			result = map[string]any{"plan": plan, "limits": limits, "live_model_calls": 0}
			break
		}
		result, code, err = executeLive(ctx, dataset, plan, limits, allowLive, output)
	case "contract":
		var contracts []evals.ContractResult
		contracts, err = evals.ExecuteContracts(ctx, dataset)
		result = map[string]any{"contracts": contracts, "live_model_calls": 0, "release_approved": false}
		for _, c := range contracts {
			if c.Status == "fail" || c.Status == "not_implemented" {
				code = 1
			}
		}
	case "replay":
		if runDirectory == "" {
			fmt.Fprintln(stderr, "--run is required")
			return 2
		}
		_, report, replayErr := evals.Replay(dataset, runDirectory)
		result = report
		err = replayErr
		if report.DeterministicFailures > 0 {
			code = 1
		}
	case "baselines":
		options.Model = "offline-baselines"
		options.Trials = 1
		options.MaxRequests = len(dataset.PD) + len(dataset.RK)
		var plan *evals.Plan
		plan, err = evals.BuildPlan(dataset, options)
		if err == nil {
			ids := make([]string, 0)
			for _, c := range plan.Cases {
				if strings.HasPrefix(c.CaseID, "RK-") {
					ids = append(ids, c.CaseID)
				}
			}
			result, err = evals.EvaluateRankingBaselines(dataset, ids)
		}
	case "summary":
		result, err = summarizeRun(dataset, runDirectory)
	case "review-template":
		result, err = evals.NewSemanticReviewTemplate(dataset, runDirectory)
	case "review":
		if reviewPath == "" {
			fmt.Fprintln(stderr, "--reviews is required")
			return 2
		}
		var document evals.SemanticReviewDocument
		document, err = evals.ReadSemanticReviews(reviewPath)
		if err == nil {
			var reviewed evals.SemanticReviewReport
			reviewed, err = evals.ApplySemanticReviews(dataset, runDirectory, document)
			result = reviewed
			if reviewed.ResultCounts["fail"] > 0 || reviewed.Report.DeterministicFailures > 0 {
				code = 1
			}
		}
	case "metamorphic-report":
		var transformed evals.MetamorphicReport
		transformed, err = evals.ReplayMetamorphic(dataset, runDirectory)
		result = transformed
		for _, observation := range transformed.Observations {
			if observation.Comparison.DeterministicStatus == "failed" {
				code = 1
			}
		}
	case "compare":
		if left == "" || right == "" {
			fmt.Fprintln(stderr, "--left and --right are required")
			return 2
		}
		result, err = evals.CompareWithOptions(dataset, left, right, comparisonOptions)
	}
	if err != nil {
		fmt.Fprintln(stderr, err)
		return 2
	}
	encoder := json.NewEncoder(stdout)
	encoder.SetIndent("", "  ")
	if err = encoder.Encode(result); err != nil {
		fmt.Fprintln(stderr, err)
		return 2
	}
	return code
}

func executeLive(ctx context.Context, dataset *evals.Dataset, plan *evals.Plan, limits evals.ExecutionLimits, allowLive bool, directory string) (any, int, error) {
	if directory == "" {
		return nil, 2, fmt.Errorf("--out is required")
	}
	if err := checkOutputDirectory(dataset.Root, directory); err != nil {
		return nil, 2, err
	}
	key := strings.TrimSpace(os.Getenv("CHATBOT_API_KEY"))
	executor, err := evals.NewGeminiExecutor(dataset, plan, limits, allowLive, key)
	if err != nil {
		return nil, 2, err
	}
	commit, err := cleanCommit()
	if err != nil {
		return nil, 2, err
	}
	record := evals.RunRecord{UnknownDefaults: "Provider defaults not present in captured requested configuration remain unknown; SDK response metadata is recorded only when supplied.", FormatVersion: "1", RunID: uuid.NewString(), Mode: "live", Commit: commit, StartedOn: time.Now().UTC(), Plan: plan, Limits: limits, Status: "running"}
	journal, err := evals.NewJournal(directory, record)
	if err != nil {
		return nil, 2, err
	}
	_, runErr := evals.Run(ctx, dataset, plan, limits, executor, journal, record, evals.CredentialRedactor(key))
	if err = errors.Join(runErr, journal.Close()); err != nil {
		return nil, 2, err
	}
	_, report, err := evals.Replay(dataset, directory)
	if err != nil {
		return nil, 2, err
	}
	if err = evals.WriteReport(directory, report); err != nil {
		return nil, 2, err
	}
	code := 0
	if report.DeterministicFailures > 0 {
		code = 1
	}
	return report, code, nil
}
func cleanCommit() (string, error) {
	status, err := exec.Command("git", "status", "--porcelain").Output()
	if err != nil {
		return "", fmt.Errorf("inspect repository: %w", err)
	}
	if len(status) > 0 {
		return "", fmt.Errorf("live measurement requires a clean committed working tree")
	}
	commit, err := exec.Command("git", "rev-parse", "HEAD").Output()
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(string(commit)), nil
}
func checkOutputDirectory(datasetRoot, directory string) error {
	// Resolve an existing parent before writing; symlinks must not redirect results
	// into the immutable package. The final run directory must be new.
	absolute, err := filepath.Abs(directory)
	if err != nil {
		return err
	}
	existing := filepath.Dir(absolute)
	for {
		if _, statErr := os.Lstat(existing); statErr == nil {
			break
		} else if !os.IsNotExist(statErr) {
			return statErr
		}
		parent := filepath.Dir(existing)
		if parent == existing {
			return fmt.Errorf("no existing output ancestor")
		}
		existing = parent
	}
	resolved, err := filepath.EvalSymlinks(existing)
	if err != nil {
		return err
	}
	suffix, err := filepath.Rel(existing, filepath.Dir(absolute))
	if err != nil {
		return err
	}
	parent := filepath.Join(resolved, suffix)

	target, err := filepath.Abs(filepath.Join(parent, filepath.Base(directory)))
	if err != nil {
		return err
	}
	source, err := filepath.EvalSymlinks(datasetRoot)
	if err != nil {
		return err
	}
	source, err = filepath.Abs(source)
	if err != nil {
		return err
	}
	rel, err := filepath.Rel(source, target)
	if err != nil {
		return err
	}
	if rel == "." || filepath.IsLocal(rel) {
		return fmt.Errorf("results cannot be written inside frozen dataset")
	}
	return nil
}
func usage(out io.Writer) {
	fmt.Fprintln(out, "Usage: evals validate|contract [--dataset PATH]")
	fmt.Fprintln(out, "       evals plan --suite NAME --model MODEL --max-requests N [--trials N] [--max-retries N] [--allow-holdout]")
	fmt.Fprintln(out, "       evals live [plan flags] --allow-live --attempt-timeout D --global-timeout D --min-interval D --max-output-tokens N --out DIR")
	fmt.Fprintln(out, "       evals live [plan and limit flags] --dry-run")
	fmt.Fprintln(out, "       evals replay --run DIR | evals compare --left DIR --right DIR")
	fmt.Fprintln(out, "       evals baselines --suite NAME [--allow-holdout]")
	fmt.Fprintln(out, "       evals summary|review-template|metamorphic-report --run DIR")
	fmt.Fprintln(out, "       evals review --run DIR --reviews FILE")
	fmt.Fprintln(out, "Plan/live: --cases ID,ID limits the suite; --metamorphic adds frozen transformations with --trials 3.")
	fmt.Fprintln(out, "Compare deliberate changes: --allow-source-change/--allow-prompt-change/--allow-generation-config-change plus --change-description TEXT.")
	fmt.Fprintln(out, "Offline modes never call a model. Live needs explicit opt-in and CHATBOT_API_KEY. Semantic review is never automatically approved.")
}
