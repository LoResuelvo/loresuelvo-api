package evals

import (
	"encoding/json"
	"errors"
	"fmt"
	"math"
	"slices"
)

var ErrInvalidSummary = errors.New("invalid evaluation summary")

// MetricSummary weights base cases equally after averaging their available
// trials. Missing/undefined numerical metrics remain visible, never zero-filled.
type MetricSummary struct {
	ExpectedObservations   int      `json:"expected_observations"`
	UnassessedObservations int      `json:"unassessed_observations"`
	Mean                   *float64 `json:"base_case_mean"`
	Minimum                *float64 `json:"minimum_observation"`
	Maximum                *float64 `json:"maximum_observation"`
	Observations           int      `json:"observations"`
	EvaluatedBaseCases     int      `json:"evaluated_base_cases"`
	UnassessedBaseCases    int      `json:"unassessed_base_cases"`
}
type TokenSummary struct {
	ObservedTotal   *int64 `json:"observed_total"`
	KnownRequests   int    `json:"known_requests"`
	UnknownRequests int    `json:"unknown_requests"`
}
type OperationalSummary struct {
	Requests               int          `json:"requests"`
	LatencyP50Millis       *float64     `json:"latency_p50_ms"`
	LatencyP95Millis       *float64     `json:"latency_p95_ms"`
	LatencyObservations    int          `json:"latency_observations"`
	LatencyUnknownRequests int          `json:"latency_unknown_requests"`
	InputTokens            TokenSummary `json:"input_tokens"`
	OutputTokens           TokenSummary `json:"output_tokens"`
	ThoughtTokens          TokenSummary `json:"thought_tokens"`
	TotalTokens            TokenSummary `json:"total_tokens"`
	Cost                   *float64     `json:"cost"`
	CostReason             string       `json:"cost_reason"`
}
type CriticalSummary struct {
	RequiredCases     int      `json:"required_cases"`
	ObservedCases     int      `json:"observed_cases"`
	MissingCaseIDs    []string `json:"missing_case_ids"`
	FailedCaseIDs     []string `json:"failed_case_ids"`
	UnassessedCaseIDs []string `json:"unassessed_case_ids"`
}
type Summary struct {
	VariantMetrics     map[string]MetricSummary `json:"variant_metrics"`
	VariantTrials      int                      `json:"variant_terminal_executions"`
	BaseCases          int                      `json:"base_cases"`
	Families           int                      `json:"families"`
	Trials             int                      `json:"terminal_trials"`
	Attempts           int                      `json:"attempts_including_retries"`
	ExecutionCounts    map[string]int           `json:"execution_counts"`
	Metrics            map[string]MetricSummary `json:"metrics"`
	SingletonMacroF1   *float64                 `json:"singleton_macro_f1"`
	SingletonBaseCases int                      `json:"singleton_base_cases"`
	SingletonTrials    int                      `json:"singleton_trials"`
	SemanticCounts     map[string]int           `json:"semantic_criterion_counts"`
	Critical           CriticalSummary          `json:"critical"`
	Operations         OperationalSummary       `json:"operations"`
	ReleaseApproved    bool                     `json:"release_approved"`
	Warning            string                   `json:"warning"`
}

// Summarize consumes already evaluated, identity-matched attempts. The terminal
// retry represents a trial; all retry failures remain in execution counts and
// critical worst-case reporting. It does not select the best-scoring response.
func Summarize(dataset *Dataset, plan *Plan, attempts []Attempt, evaluations []EvaluatedAttempt) (Summary, error) {
	summary := Summary{VariantMetrics: map[string]MetricSummary{}, ExecutionCounts: map[string]int{}, Metrics: map[string]MetricSummary{}, SemanticCounts: map[string]int{}, Warning: "Base cases are descriptive units, not independent population samples. Base and variant quality are reported separately. Numerical means exclude undefined scores with coverage shown; missing/invalid outputs fail outcome/category accuracy. Critical worst-case checks include all retries; no release approval is inferred."}
	if dataset == nil || plan == nil || len(attempts) != len(evaluations) {
		return summary, fmt.Errorf("%w: dataset and matched attempts required", ErrInvalidSummary)
	}
	if plan.Trials <= 0 || plan.MaxRetries < 0 || len(plan.Cases) == 0 {
		return summary, fmt.Errorf("%w: nonempty plan with positive trials required", ErrInvalidSummary)
	}
	if plan.Trials > len(attempts)/len(plan.Cases) {
		return summary, fmt.Errorf("%w: missing planned case/trial; record not_executed explicitly", ErrInvalidSummary)
	}
	baseFor := map[string]string{}
	variants := map[string]bool{}
	for _, planned := range plan.Cases {
		if _, exists := baseFor[planned.CaseID]; exists {
			return summary, fmt.Errorf("%w: duplicate planned case", ErrInvalidSummary)
		}
		baseFor[planned.CaseID] = planned.CaseID
		if planned.Variant != nil {
			baseFor[planned.CaseID] = planned.Variant.BaseCaseID
			variants[planned.CaseID] = true
		}
	}
	meta := map[string]CaseMetadata{}
	pd := map[string]pdExpected{}
	pdCount, rkCount := 0, 0
	for _, c := range dataset.PD {
		meta[c.ID] = c.CaseMetadata
		var expected pdExpected
		if err := json.Unmarshal(c.Expected, &expected); err != nil {
			return summary, fmt.Errorf("decode expected %s: %w", c.ID, err)
		}
		pd[c.ID] = expected
	}
	for _, c := range dataset.RK {
		meta[c.ID] = c.CaseMetadata
	}
	evaluated := map[string]EvaluatedAttempt{}
	for _, item := range evaluations {
		key := summaryAttemptKey(item.CaseID, item.Trial, item.Retry)
		if _, ok := evaluated[key]; ok {
			return summary, fmt.Errorf("%w: duplicate evaluation", ErrInvalidSummary)
		}
		evaluated[key] = item
	}
	terminal := map[string]Attempt{}
	seenAttempts := map[string]bool{}
	baseIDs := map[string]bool{}
	families := map[string]bool{}
	criticalSeen := map[string]bool{}
	criticalFailed := map[string]bool{}
	criticalPending := map[string]bool{}
	for _, a := range attempts {
		key := summaryAttemptKey(a.CaseID, a.Trial, a.Retry)
		item, ok := evaluated[key]
		if !ok || seenAttempts[key] || item.ExecutionStatus != a.Status || item.Evaluation.CaseID != a.CaseID {
			return summary, fmt.Errorf("%w: unmatched or duplicate attempt %s", ErrInvalidSummary, key)
		}
		baseID, planned := baseFor[a.CaseID]
		metadata, known := meta[baseID]
		if !known || !planned {
			return summary, fmt.Errorf("%w: unknown case %s", ErrInvalidSummary, a.CaseID)
		}
		if a.Trial < 1 || a.Trial > plan.Trials || a.Retry < 0 || a.Retry > plan.MaxRetries || !slices.Contains([]string{"executed", "execution_error", "asset_error", "not_executed"}, a.Status) {
			return summary, fmt.Errorf("%w: invalid trial, retry or status", ErrInvalidSummary)
		}
		seenAttempts[key] = true
		baseIDs[baseID] = true
		families[metadata.FamilyID] = true
		summary.ExecutionCounts[a.Status]++
		trialKey := attemptKey(a.CaseID, a.Trial)
		previous, exists := terminal[trialKey]
		if (!exists && a.Retry != 0) || (exists && (a.Retry != previous.Retry+1 || previous.Status != "execution_error")) {
			return summary, fmt.Errorf("%w: invalid retry sequence for %s", ErrInvalidSummary, trialKey)
		}
		terminal[trialKey] = a
		for _, criterion := range item.Evaluation.SemanticChecks {
			summary.SemanticCounts[criterion.Result]++
		}
		if slices.Contains(dataset.Suites["critical_all"], baseID) {
			if a.Status != "not_executed" {
				criticalSeen[baseID] = true
			}
			if a.Status != "executed" || item.Evaluation.DeterministicStatus != "passed" {
				criticalFailed[baseID] = true
			}
			if len(item.Evaluation.SemanticChecks) == 0 {
				criticalPending[baseID] = true
			}
			for _, criterion := range item.Evaluation.SemanticChecks {
				if criterion.Result == "fail" {
					criticalFailed[baseID] = true
				}
				if criterion.Result == "unassessed" || criterion.Result == "" {
					criticalPending[baseID] = true
				}
			}
		}
	}
	if len(terminal) != len(plan.Cases)*plan.Trials {
		return summary, fmt.Errorf("%w: missing planned case/trial; record not_executed explicitly", ErrInvalidSummary)
	}
	summary.BaseCases, summary.Families, summary.Trials, summary.Attempts = len(baseIDs), len(families), len(terminal), len(attempts)
	for id := range baseIDs {
		if _, ok := pd[id]; ok {
			pdCount++
		} else {
			rkCount++
		}
	}
	observations := map[string]map[string][]float64{}
	variantObservations := map[string]map[string][]float64{}
	variantBases := map[string]bool{}
	singleton := map[string][]outcomeObservation{}
	pdTrials, rkTrials, categoryTrials := 0, 0, 0
	categoryCases := map[string]bool{}
	terminalKeys := make([]string, 0, len(terminal))
	for key := range terminal {
		terminalKeys = append(terminalKeys, key)
	}
	slices.Sort(terminalKeys)
	for _, key := range terminalKeys {
		a := terminal[key]
		evaluation := evaluated[summaryAttemptKey(a.CaseID, a.Trial, a.Retry)].Evaluation
		baseID := baseFor[a.CaseID]
		if variants[a.CaseID] {
			if _, isPD := pd[baseID]; isPD {
				return summary, fmt.Errorf("%w: ranking variants require ranking base cases", ErrInvalidSummary)
			}
			summary.VariantTrials++
			variantBases[baseID] = true
			if a.Status == "executed" {
				collectRankingMetrics(variantObservations, baseID, evaluation)
			}
			continue
		}
		expected, isPD := pd[baseID]
		if !isPD {
			rkTrials++
			if a.Status == "executed" {
				collectRankingMetrics(observations, baseID, evaluation)
			}
			continue
		}
		pdTrials++
		accepted := 0.0
		if a.Status == "executed" {
			if value, ok := evaluation.Metrics["accepted_outcome"].(bool); ok && value {
				accepted = 1
			}
		}
		addMetric(observations, "accepted_outcome_accuracy", a.CaseID, accepted)
		var output pdOutput
		parsed := a.Status == "executed" && json.Unmarshal([]byte(a.RawOutput), &output) == nil && !hasSchemaError(evaluation.Errors)
		if len(expected.Outcomes) == 1 && slices.Contains(expected.Actions, "replace") {
			predicted := ""
			if parsed && output.Assessment.Action == "replace" {
				predicted = output.Assessment.Outcome
			}
			singleton[a.CaseID] = append(singleton[a.CaseID], outcomeObservation{Expected: expected.Outcomes[0], Predicted: predicted})
		}
		if len(expected.Outcomes) == 1 && expected.Outcomes[0] == "professional_required" {
			categoryCases[a.CaseID] = true
			categoryTrials++
			correct := 0.0
			if parsed && output.Assessment.Action == "replace" && slices.Contains(expected.CategoryNames, output.Assessment.Category) && output.Assessment.Category != "" {
				correct = 1
			}
			addMetric(observations, "category_accuracy_when_required", a.CaseID, correct)
		}
	}
	for _, name := range rankingMetricNames() {
		summary.Metrics[name] = summarizeMetric(observations[name], rkCount, rkTrials)
		summary.VariantMetrics[name] = summarizeMetric(variantObservations[name], len(variantBases), summary.VariantTrials)
	}
	summary.Metrics["accepted_outcome_accuracy"] = summarizeMetric(observations["accepted_outcome_accuracy"], pdCount, pdTrials)
	summary.Metrics["category_accuracy_when_required"] = summarizeMetric(observations["category_accuracy_when_required"], len(categoryCases), categoryTrials)
	summary.SingletonMacroF1, summary.SingletonTrials = singletonMacroF1(singleton)
	summary.SingletonBaseCases = len(singleton)
	summary.Critical = CriticalSummary{RequiredCases: len(dataset.Suites["critical_all"]), MissingCaseIDs: []string{}, FailedCaseIDs: []string{}, UnassessedCaseIDs: []string{}}
	for _, id := range dataset.Suites["critical_all"] {
		if criticalSeen[id] {
			summary.Critical.ObservedCases++
		} else {
			summary.Critical.MissingCaseIDs = append(summary.Critical.MissingCaseIDs, id)
		}
		if criticalFailed[id] {
			summary.Critical.FailedCaseIDs = append(summary.Critical.FailedCaseIDs, id)
		}
		if criticalPending[id] {
			summary.Critical.UnassessedCaseIDs = append(summary.Critical.UnassessedCaseIDs, id)
		}
	}
	summary.Operations = summarizeOperations(attempts)
	return summary, nil
}
func summaryAttemptKey(id string, trial, retry int) string {
	return fmt.Sprintf("%s/%d/%d", id, trial, retry)
}
func hasSchemaError(codes []string) bool {
	for _, code := range codes {
		if len(code) >= 13 && code[:13] == "output_schema" {
			return true
		}
	}
	return false
}
func rankingMetricNames() []string {
	return []string{"ndcg_at_3", "precision_at_3_relevance_ge_2", "pairwise_satisfaction", "pairwise_coverage"}
}
func collectRankingMetrics(observations map[string]map[string][]float64, id string, evaluation Evaluation) {
	for _, name := range []string{"ndcg_at_3", "precision_at_3_relevance_ge_2"} {
		if value, ok := summaryNumber(evaluation.Metrics[name]); ok {
			addMetric(observations, name, id, value)
		}
	}
	if raw, ok := evaluation.Metrics["pairwise"]; ok {
		data, err := json.Marshal(raw)
		if err != nil {
			return
		}
		var pairs PairwiseMetrics
		if json.Unmarshal(data, &pairs) != nil {
			return
		}
		if pairs.Satisfaction != nil {
			addMetric(observations, "pairwise_satisfaction", id, *pairs.Satisfaction)
		}
		if pairs.Coverage != nil {
			addMetric(observations, "pairwise_coverage", id, *pairs.Coverage)
		}
	}
}
func summaryNumber(value any) (float64, bool) {
	if value == nil {
		return 0, false
	}
	raw, err := json.Marshal(value)
	if err != nil {
		return 0, false
	}
	var number *float64
	if json.Unmarshal(raw, &number) != nil || number == nil || math.IsNaN(*number) || math.IsInf(*number, 0) {
		return 0, false
	}
	return *number, true
}
func addMetric(observations map[string]map[string][]float64, name, id string, value float64) {
	if observations[name] == nil {
		observations[name] = map[string][]float64{}
	}
	observations[name][id] = append(observations[name][id], value)
}
func summarizeMetric(values map[string][]float64, total, expectedObservations int) MetricSummary {
	result := MetricSummary{UnassessedBaseCases: total, ExpectedObservations: expectedObservations}
	sum := 0.0
	low, high := math.Inf(1), math.Inf(-1)
	ids := make([]string, 0, len(values))
	for id := range values {
		ids = append(ids, id)
	}
	slices.Sort(ids)
	for _, id := range ids {
		samples := values[id]
		if len(samples) == 0 {
			continue
		}
		caseSum := 0.0
		for _, sample := range samples {
			caseSum += sample
			low = min(low, sample)
			high = max(high, sample)
		}
		sum += caseSum / float64(len(samples))
		result.Observations += len(samples)
		result.EvaluatedBaseCases++
	}
	result.UnassessedBaseCases -= result.EvaluatedBaseCases
	result.UnassessedObservations = expectedObservations - result.Observations
	if result.EvaluatedBaseCases > 0 {
		mean := sum / float64(result.EvaluatedBaseCases)
		result.Mean = &mean
		result.Minimum = &low
		result.Maximum = &high
	}
	return result
}

type outcomeObservation struct{ Expected, Predicted string }

func singletonMacroF1(cases map[string][]outcomeObservation) (*float64, int) {
	type counts struct{ TP, FP, FN float64 }
	classes := map[string]*counts{}
	trials := 0
	get := func(label string) *counts {
		if classes[label] == nil {
			classes[label] = &counts{}
		}
		return classes[label]
	}
	ids := make([]string, 0, len(cases))
	for id := range cases {
		ids = append(ids, id)
	}
	slices.Sort(ids)
	for _, id := range ids {
		items := cases[id]
		if len(items) == 0 {
			continue
		}
		weight := 1 / float64(len(items))
		for _, item := range items {
			trials++
			if item.Predicted == item.Expected {
				get(item.Expected).TP += weight
			} else {
				get(item.Expected).FN += weight
				if item.Predicted != "" {
					get(item.Predicted).FP += weight
				}
			}
		}
	}
	if len(classes) == 0 {
		return nil, trials
	}
	labels := make([]string, 0, len(classes))
	for label := range classes {
		labels = append(labels, label)
	}
	slices.Sort(labels)
	sum := 0.0
	for _, label := range labels {
		c := classes[label]
		sum += 2 * c.TP / (2*c.TP + c.FP + c.FN)
	}
	score := sum / float64(len(classes))
	return &score, trials
}
func summarizeOperations(attempts []Attempt) OperationalSummary {
	result := OperationalSummary{CostReason: "Verified pricing is not configured; observed token subtotals are not complete totals when requests lack usage metadata."}
	latencies := []float64{}
	for _, a := range attempts {
		if a.RequestCount <= 0 {
			continue
		}
		result.Requests += a.RequestCount
		if !a.StartedOn.IsZero() && a.LatencyMillis >= 0 {
			latencies = append(latencies, float64(a.LatencyMillis))
		} else {
			result.LatencyUnknownRequests += a.RequestCount
		}
		var response struct {
			Usage *struct {
				Input    *int64 `json:"promptTokenCount"`
				Output   *int64 `json:"candidatesTokenCount"`
				Thoughts *int64 `json:"thoughtsTokenCount"`
				Total    *int64 `json:"totalTokenCount"`
			} `json:"usageMetadata"`
		}
		if json.Unmarshal(a.ProviderResponse, &response) != nil || response.Usage == nil {
			addTokenUsage(&result.InputTokens, nil, a.RequestCount)
			addTokenUsage(&result.OutputTokens, nil, a.RequestCount)
			addTokenUsage(&result.ThoughtTokens, nil, a.RequestCount)
			addTokenUsage(&result.TotalTokens, nil, a.RequestCount)
			continue
		}
		addTokenUsage(&result.InputTokens, response.Usage.Input, a.RequestCount)
		addTokenUsage(&result.OutputTokens, response.Usage.Output, a.RequestCount)
		addTokenUsage(&result.ThoughtTokens, response.Usage.Thoughts, a.RequestCount)
		addTokenUsage(&result.TotalTokens, response.Usage.Total, a.RequestCount)
	}
	slices.Sort(latencies)
	result.LatencyObservations = len(latencies)
	if len(latencies) > 0 {
		p50 := latencies[int(math.Ceil(0.5*float64(len(latencies))))-1]
		p95 := latencies[int(math.Ceil(0.95*float64(len(latencies))))-1]
		result.LatencyP50Millis = &p50
		result.LatencyP95Millis = &p95
	}
	return result
}
func addTokenUsage(summary *TokenSummary, count *int64, requests int) {
	if count == nil || *count < 0 {
		summary.UnknownRequests += requests
		return
	}
	if summary.ObservedTotal == nil {
		summary.ObservedTotal = new(int64)
	}
	*summary.ObservedTotal += *count
	summary.KnownRequests += requests
}
