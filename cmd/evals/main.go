// Command evals prepares local evaluation runs without contacting a model.
package main

import (
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"

	"github.com/LoResuelvo/loresuelvo-api/internal/evals"
)

const defaultDataset = "evals/datasets/LoResuelvo_US60_evals_v1.0.0"

func main() { os.Exit(run(os.Args[1:], os.Stdout, os.Stderr)) }

func run(args []string, stdout, stderr io.Writer) int {
	if len(args) == 0 {
		usage(stderr)
		return 2
	}
	if args[0] == "help" || args[0] == "--help" || args[0] == "-h" {
		usage(stdout)
		return 0
	}
	if args[0] != "validate" && args[0] != "plan" {
		fmt.Fprintf(stderr, "unsupported command %q; no model calls were made\n", args[0])
		usage(stderr)
		return 2
	}
	flags := flag.NewFlagSet(args[0], flag.ContinueOnError)
	flags.SetOutput(stderr)
	root := flags.String("dataset", defaultDataset, "immutable dataset directory")
	var options evals.PlanOptions
	if args[0] == "plan" {
		flags.StringVar(&options.Suite, "suite", "", "explicit suite: smoke, development, holdout, critical_all")
		flags.StringVar(&options.Model, "model", "", "requested model identifier (offline metadata only)")
		flags.IntVar(&options.Trials, "trials", 0, "positive repetitions per base case (default: frozen experiment configuration)")
		flags.IntVar(&options.MaxRetries, "max-retries", 0, "maximum retries per execution")
		flags.IntVar(&options.MaxRequests, "max-requests", 0, "positive request ceiling, including retries")
		flags.BoolVar(&options.AllowHoldout, "allow-holdout", false, "explicitly authorize planning reserve cases")
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
	dataset, err := evals.LoadDataset(*root)
	if err != nil {
		fmt.Fprintln(stderr, err)
		return 2
	}
	var result any
	if args[0] == "plan" {
		trialsProvided := false
		flags.Visit(func(f *flag.Flag) {
			if f.Name == "trials" {
				trialsProvided = true
			}
		})
		if !trialsProvided {
			trialKey := "release"
			switch options.Suite {
			case "smoke":
				trialKey = "smoke"
			case "development":
				trialKey = "baseline_development"
			}
			options.Trials = dataset.ExperimentTrials[trialKey]
		}
		plan, err := evals.BuildPlan(dataset, options)
		if err != nil {
			fmt.Fprintln(stderr, err)
			return 2
		}
		result = plan
	} else {
		result = struct {
			Status    string `json:"status"`
			Scope     string `json:"scope"`
			Version   string `json:"dataset_version"`
			Hash      string `json:"dataset_manifest_sha256"`
			PD        int    `json:"prediagnosis_cases"`
			RK        int    `json:"ranking_cases"`
			CT        int    `json:"contract_specifications"`
			LiveCalls int    `json:"live_model_calls"`
		}{"passed", "integrity and domain mapping; full schema validation uses evalpack.py", dataset.Version, dataset.ManifestSHA256, len(dataset.PD), len(dataset.RK), len(dataset.CT), 0}
	}
	encoder := json.NewEncoder(stdout)
	encoder.SetIndent("", "  ")
	if err := encoder.Encode(result); err != nil {
		fmt.Fprintln(stderr, err)
		return 2
	}
	return 0
}

func usage(out io.Writer) {
	fmt.Fprintln(out, "Usage: evals validate [--dataset PATH]")
	fmt.Fprintln(out, "       evals plan --suite NAME --model MODEL --max-requests N [--trials N] [--max-retries N] [--allow-holdout] [--dataset PATH]")
	fmt.Fprintln(out, "Offline preparation only. Contracts, live execution, replay, compare, and transformations are not implemented yet.")
}
