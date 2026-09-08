package evals

import (
	"fmt"
	"math"
	"strings"
)

// PlanOptions describes an offline preview, never permission to contact a model.
type PlanOptions struct {
	Suite        string
	Model        string
	Trials       int
	MaxRetries   int
	MaxRequests  int
	AllowHoldout bool
}

type PlannedCase struct {
	CaseID string `json:"case_id"`
	Split  string `json:"split"`
}

type Plan struct {
	DatasetVersion    string        `json:"dataset_version"`
	DatasetSHA256     string        `json:"dataset_manifest_sha256"`
	Suite             string        `json:"suite"`
	Model             string        `json:"requested_model"`
	Cases             []PlannedCase `json:"cases"`
	Trials            int           `json:"trials"`
	MaxRetries        int           `json:"max_retries"`
	BaseExecutions    int           `json:"base_executions"`
	MaximumRequests   int           `json:"maximum_requests"`
	RequestLimit      int           `json:"request_limit"`
	UsesHoldout       bool          `json:"uses_holdout"`
	HoldoutAuthorized bool          `json:"holdout_authorized"`
	Cost              *float64      `json:"estimated_cost"`
	CostReason        string        `json:"cost_reason"`
	LiveCalls         int           `json:"live_model_calls"`
}

// BuildPlan counts only the explicitly selected base PD/RK cases. It never
// adds contracts, transformations, holdout, or retries implicitly.
func BuildPlan(dataset *Dataset, options PlanOptions) (*Plan, error) {
	if dataset == nil {
		return nil, fmt.Errorf("dataset is required")
	}
	if options.Suite == "" || options.Suite == "all" {
		return nil, fmt.Errorf("select an explicit suite: smoke, development, holdout, or critical_all")
	}
	ids, ok := dataset.Suites[options.Suite]
	if !ok || len(ids) == 0 {
		return nil, fmt.Errorf("unknown or empty suite %q", options.Suite)
	}
	if strings.TrimSpace(options.Model) == "" {
		return nil, fmt.Errorf("requested model is required")
	}
	if options.Trials <= 0 || options.MaxRetries < 0 || options.MaxRequests <= 0 {
		return nil, fmt.Errorf("trials and request limit must be positive; retries cannot be negative")
	}
	splits := make(map[string]string, len(dataset.PD)+len(dataset.RK))
	for _, c := range dataset.PD {
		splits[c.ID] = c.Split
	}
	for _, c := range dataset.RK {
		splits[c.ID] = c.Split
	}
	plan := &Plan{DatasetVersion: dataset.Version, DatasetSHA256: dataset.ManifestSHA256, Suite: options.Suite, Model: strings.TrimSpace(options.Model), Trials: options.Trials, MaxRetries: options.MaxRetries, RequestLimit: options.MaxRequests, HoldoutAuthorized: options.AllowHoldout, CostReason: "verified pricing is not configured", Cases: make([]PlannedCase, 0, len(ids))}
	seen := make(map[string]bool, len(ids))
	for _, id := range ids {
		split, exists := splits[id]
		if !exists || seen[id] {
			return nil, fmt.Errorf("suite %q contains unknown or duplicate case %q", options.Suite, id)
		}
		seen[id] = true
		if split != "development" && split != "holdout" {
			return nil, fmt.Errorf("case %q has invalid split %q", id, split)
		}
		if split == "holdout" {
			plan.UsesHoldout = true
		}
		plan.Cases = append(plan.Cases, PlannedCase{CaseID: id, Split: split})
	}
	if (plan.UsesHoldout || options.Suite == "critical_all" || options.Suite == "holdout") && !options.AllowHoldout {
		return nil, fmt.Errorf("suite %q requires --allow-holdout", options.Suite)
	}
	if len(ids) > math.MaxInt/options.Trials {
		return nil, fmt.Errorf("execution count overflows")
	}
	plan.BaseExecutions = len(ids) * options.Trials
	if options.MaxRetries == math.MaxInt || plan.BaseExecutions > math.MaxInt/(options.MaxRetries+1) {
		return nil, fmt.Errorf("request count overflows")
	}
	plan.MaximumRequests = plan.BaseExecutions * (options.MaxRetries + 1)
	if plan.MaximumRequests > options.MaxRequests {
		return nil, fmt.Errorf("plan requires up to %d requests, exceeding limit %d", plan.MaximumRequests, options.MaxRequests)
	}
	return plan, nil
}
