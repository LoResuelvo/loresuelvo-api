package evals

import (
	"encoding/json"
	"fmt"
	"reflect"
	"sort"
	"strings"
)

type EvaluatedAttempt struct {
	CaseID          string     `json:"case_id"`
	Trial           int        `json:"trial"`
	Retry           int        `json:"retry"`
	ExecutionStatus string     `json:"execution_status"`
	Evaluation      Evaluation `json:"evaluation"`
}

type Report struct {
	RunID                 string             `json:"run_id"`
	Mode                  string             `json:"mode"`
	Attempts              []EvaluatedAttempt `json:"attempts"`
	ExecutionCounts       map[string]int     `json:"execution_counts"`
	Requests              int                `json:"model_requests"`
	DeterministicFailures int                `json:"deterministic_failures"`
	SemanticStatus        string             `json:"semantic_status"`
	ReleaseApproved       bool               `json:"release_approved"`
	Warning               string             `json:"warning"`
}

func Replay(dataset *Dataset, directory string) (RunRecord, Report, error) {
	record, attempts, err := ReadRun(directory)
	if err != nil {
		return record, Report{}, err
	}
	if err = validateRun(dataset, record, attempts); err != nil {
		return record, Report{}, err
	}
	report := Report{RunID: record.RunID, Mode: "replay", ExecutionCounts: map[string]int{}, SemanticStatus: "unassessed", Warning: "Historical outputs only; replay does not test the current model or prompt. Semantic review is required."}
	for _, a := range attempts {
		evaluation, evalErr := evaluatePlannedAttempt(dataset, record.Plan, a)
		if evalErr != nil {
			return record, report, evalErr
		}
		if a.Status != "executed" {
			evaluation.Errors = append(evaluation.Errors, a.Status)
			evaluation.DeterministicStatus = "failed"
			evaluation.OverallStatus = "failed"
		}
		if evaluation.DeterministicStatus == "failed" {
			report.DeterministicFailures++
		}
		report.ExecutionCounts[a.Status]++
		report.Requests += a.RequestCount
		report.Attempts = append(report.Attempts, EvaluatedAttempt{CaseID: a.CaseID, Trial: a.Trial, Retry: a.Retry, ExecutionStatus: a.Status, Evaluation: evaluation})
	}
	return record, report, nil
}

func validateRun(dataset *Dataset, record RunRecord, attempts []Attempt) error {
	if record.Plan.DatasetVersion != dataset.Version || record.Plan.DatasetSHA256 != dataset.ManifestSHA256 {
		return fmt.Errorf("run dataset version or manifest mismatch")
	}
	plan, err := BuildPlan(dataset, PlanOptions{Suite: record.Plan.Suite, Model: record.Plan.Model, Trials: record.Plan.Trials, MaxRetries: record.Plan.MaxRetries, MaxRequests: record.Plan.RequestLimit, AllowHoldout: record.Plan.HoldoutAuthorized, Metamorphic: record.Plan.Metamorphic, CaseIDs: record.Plan.SelectedCaseIDs})
	if err != nil {
		return err
	}
	if !reflect.DeepEqual(plan.Cases, record.Plan.Cases) || plan.MaximumRequests != record.Plan.MaximumRequests {
		return fmt.Errorf("recorded plan does not match dataset suite")
	}
	expected := map[string]bool{}
	for _, c := range plan.Cases {
		for trial := 1; trial <= plan.Trials; trial++ {
			expected[attemptKey(c.CaseID, trial)] = true
		}
	}
	previous := map[string]Attempt{}
	for _, a := range attempts {
		key := attemptKey(a.CaseID, a.Trial)
		if !expected[key] || a.Retry < 0 || a.Retry > plan.MaxRetries {
			return fmt.Errorf("unknown case, trial or retry in journal: %s", key)
		}
		prior, exists := previous[key]
		if (!exists && a.Retry != 0) || (exists && (a.Retry != prior.Retry+1 || prior.Status != "execution_error")) {
			return fmt.Errorf("duplicate or invalid retry sequence for %s", key)
		}
		switch a.Status {
		case "executed", "execution_error", "asset_error", "not_executed":
		default:
			return fmt.Errorf("unknown execution status %q", a.Status)
		}
		if a.Status == "executed" && (len(a.Input) == 0 || a.InputSHA256 == "" || a.PromptSHA256 == "" || len(a.GenerationConfig) == 0 || a.RequestCount != 1) {
			return fmt.Errorf("executed attempt lacks generation evidence")
		}
		if a.RequestCount < 0 || a.RequestCount > 1 {
			return fmt.Errorf("invalid request count")
		}
		if a.Status == "not_executed" && a.RequestCount != 0 {
			return fmt.Errorf("unexecuted case has requests")
		}
		if len(a.Input) > 0 && digest(a.Input) != a.InputSHA256 {
			return fmt.Errorf("input hash mismatch for %s", key)
		}
		if len(a.Input) > 0 {
			hash, err := promptHash(a.Input)
			if err != nil || hash != a.PromptSHA256 {
				return fmt.Errorf("prompt hash mismatch for %s", key)
			}
		}
		previous[key] = a
	}
	if len(previous) != len(expected) {
		return fmt.Errorf("journal omits planned trials")
	}
	requests := 0
	for _, a := range attempts {
		requests += a.RequestCount
	}
	if requests > plan.RequestLimit {
		return fmt.Errorf("journal exceeds request limit")
	}
	return nil
}
func attemptKey(caseID string, trial int) string { return fmt.Sprintf("%s/%d", caseID, trial) }

func promptHash(input json.RawMessage) (string, error) {
	var contents []struct {
		Parts []struct {
			Text string `json:"text"`
		} `json:"parts"`
	}
	if err := json.Unmarshal(input, &contents); err != nil {
		return "", err
	}
	var prompts []string
	for _, content := range contents {
		for _, part := range content.Parts {
			if part.Text != "" {
				prompts = append(prompts, part.Text)
			}
		}
	}
	data, err := json.Marshal(prompts)
	if err != nil {
		return "", err
	}
	return digest(data), nil
}

type CaseComparison struct {
	CaseID string     `json:"case_id"`
	Trial  int        `json:"trial"`
	Left   Evaluation `json:"left"`
	Right  Evaluation `json:"right"`
}
type CompareOptions struct {
	AllowSourceChange           bool   `json:"allow_source_change"`
	AllowPromptChange           bool   `json:"allow_prompt_change"`
	AllowGenerationConfigChange bool   `json:"allow_generation_config_change"`
	ChangeDescription           string `json:"change_description"`
}

type Comparison struct {
	Changes         CompareOptions   `json:"declared_changes"`
	Differences     []string         `json:"observed_differences"`
	LeftRunID       string           `json:"left_run_id"`
	RightRunID      string           `json:"right_run_id"`
	LeftModel       string           `json:"left_model"`
	RightModel      string           `json:"right_model"`
	Pairs           []CaseComparison `json:"pairs"`
	ReleaseApproved bool             `json:"release_approved"`
	Warning         string           `json:"warning"`
}

// Compare requires identical evidence and execution settings; a model identifier
// change is the sole intentional difference supported in this first comparison.
func Compare(dataset *Dataset, leftDirectory, rightDirectory string) (Comparison, error) {
	return CompareWithOptions(dataset, leftDirectory, rightDirectory, CompareOptions{})
}

// CompareWithOptions requires an explicit description for intentional historical
// source, prompt or generation-setting changes. It never relaxes dataset, cases,
// trial or operational-limit compatibility.
func CompareWithOptions(dataset *Dataset, leftDirectory, rightDirectory string, options CompareOptions) (Comparison, error) {
	if (options.AllowSourceChange || options.AllowPromptChange || options.AllowGenerationConfigChange) && strings.TrimSpace(options.ChangeDescription) == "" {
		return Comparison{}, fmt.Errorf("declared comparison changes require a description")
	}
	differences := []string{}

	left, lr, err := Replay(dataset, leftDirectory)
	if err != nil {
		return Comparison{}, err
	}
	right, rr, err := Replay(dataset, rightDirectory)
	if err != nil {
		return Comparison{}, err
	}
	if left.Commit != right.Commit {
		if !options.AllowSourceChange {
			return Comparison{}, fmt.Errorf("incompatible source commits")
		}
		differences = append(differences, "source_commit")
	}
	if left.Plan.Model != right.Plan.Model {
		differences = append(differences, "requested_model")
	}
	if left.Plan.Suite != right.Plan.Suite || left.Plan.Trials != right.Plan.Trials || left.Plan.MaxRetries != right.Plan.MaxRetries || left.Plan.RequestLimit != right.Plan.RequestLimit || !reflect.DeepEqual(left.Plan.Cases, right.Plan.Cases) || left.Limits != right.Limits {
		return Comparison{}, fmt.Errorf("incompatible commits, suites, trials, retries or execution settings")
	}
	leftObserved, la, err := ReadRun(leftDirectory)
	if err != nil {
		return Comparison{}, err
	}
	rightObserved, ra, err := ReadRun(rightDirectory)
	if err != nil {
		return Comparison{}, err
	}
	if leftObserved.AttemptsSHA256 != left.AttemptsSHA256 || rightObserved.AttemptsSHA256 != right.AttemptsSHA256 {
		return Comparison{}, fmt.Errorf("run changed during comparison")
	}
	inputs := map[string]Attempt{}
	for _, a := range la {
		inputs[attemptKey(a.CaseID, a.Trial)] = a
	}
	for _, a := range ra {
		other, ok := inputs[attemptKey(a.CaseID, a.Trial)]
		if !ok {
			return Comparison{}, fmt.Errorf("unpaired trial")
		}
		if len(a.GenerationConfig) > 0 && len(other.GenerationConfig) > 0 {
			var leftConfig, rightConfig any
			if err := json.Unmarshal(a.GenerationConfig, &leftConfig); err != nil {
				return Comparison{}, err
			}
			if err := json.Unmarshal(other.GenerationConfig, &rightConfig); err != nil {
				return Comparison{}, err
			}
			if !reflect.DeepEqual(leftConfig, rightConfig) {
				if !options.AllowGenerationConfigChange {
					return Comparison{}, fmt.Errorf("generation configurations differ for %s", a.CaseID)
				}
				differences = append(differences, "generation_config:"+a.CaseID)
			}
		}
		if len(a.Input) > 0 && len(other.Input) > 0 && (a.InputSHA256 != other.InputSHA256 || a.PromptSHA256 != other.PromptSHA256) {
			if !options.AllowPromptChange {
				return Comparison{}, fmt.Errorf("effective inputs differ for %s", a.CaseID)
			}
			differences = append(differences, "effective_input:"+a.CaseID)
		}
	}
	lmap := map[string]EvaluatedAttempt{}
	for _, a := range lr.Attempts {
		lmap[attemptKey(a.CaseID, a.Trial)] = a
	}
	rmap := map[string]EvaluatedAttempt{}
	for _, a := range rr.Attempts {
		rmap[attemptKey(a.CaseID, a.Trial)] = a
	}
	result := Comparison{Changes: options, Differences: differences, LeftRunID: left.RunID, RightRunID: right.RunID, LeftModel: left.Plan.Model, RightModel: right.Plan.Model, Warning: "Paired historical base cases; technical failures and unassessed semantics are not wins. All retries remain in source reports."}
	keys := make([]string, 0, len(lmap))
	for key := range lmap {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	for _, key := range keys {
		l := lmap[key]
		r, ok := rmap[key]
		if !ok {
			return result, fmt.Errorf("missing paired case %s", key)
		}
		result.Pairs = append(result.Pairs, CaseComparison{CaseID: l.CaseID, Trial: l.Trial, Left: l.Evaluation, Right: r.Evaluation})
	}
	return result, nil
}
