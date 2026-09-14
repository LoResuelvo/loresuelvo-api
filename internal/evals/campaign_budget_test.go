package evals

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"math"
	"net/http"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func budgetPlan(model string, requests int, tokens int64, retries int) CampaignBudgetPlan {
	inputs := make([]InputTokenEvidence, requests)
	for i := range inputs {
		inputs[i] = InputTokenEvidence{Tokens: tokens, Source: "count_tokens"}
	}
	return CampaignBudgetPlan{Plan: &Plan{Model: model, MaximumRequests: requests, MaxRetries: retries}, InputTokens: inputs}
}

func validBudgetSpec() CampaignBudgetSpec {
	return CampaignBudgetSpec{
		Plans:          []CampaignBudgetPlan{budgetPlan("gemini-3.1-flash-lite", 2, 100, 0), budgetPlan("gemini-3.5-flash-lite", 2, 200, 0)},
		Prices:         []CampaignPrice{{Model: "gemini-3.1-flash-lite", InputUSDPerMillion: 0.25, OutputUSDPerMillion: 1.5, Verified: true}, {Model: "gemini-3.5-flash-lite", InputUSDPerMillion: 0.30, OutputUSDPerMillion: 2.50, Verified: true}},
		HardCeilingUSD: 10, MaxOutputTokens: CampaignBudgetMaxOutputTokens, PricingSource: "https://ai.google.dev/gemini-api/docs/pricing",
	}
}

func TestPreflightCampaignBudgetReservesHighestVerifiedRates(t *testing.T) {
	estimate, err := PreflightCampaignBudget(validBudgetSpec())
	require.NoError(t, err)
	require.Equal(t, 4, estimate.RequestCount)
	require.Equal(t, int64(600), estimate.InputTokens)
	require.Equal(t, int64(80000), estimate.ReservedInputTokens)
	require.Equal(t, int64(16384), estimate.ReservedOutputTokens)
	require.InDelta(t, 0.06496, estimate.ReservedTotalUSD, 0.0000001)
	require.InDelta(t, 9.93504, estimate.HeadroomUSD, 0.0000001)
	require.Equal(t, 0.30, estimate.InputUSDPerMillion)
	require.Equal(t, 2.50, estimate.OutputUSDPerMillion)
}

func TestPreflightCampaignBudgetRejectsUnknownOrUnsafeInputs(t *testing.T) {
	tests := []struct {
		name   string
		mutate func(*CampaignBudgetSpec)
	}{
		{"missing token evidence", func(s *CampaignBudgetSpec) { s.Plans[0].InputTokens = nil }},
		{"unknown token source", func(s *CampaignBudgetSpec) { s.Plans[0].InputTokens[0].Source = "provider_guess" }},
		{"input bound exceeded", func(s *CampaignBudgetSpec) { s.Plans[0].InputTokens[0].Tokens = CampaignBudgetMaxInputTokens + 1 }},
		{"unverified price", func(s *CampaignBudgetSpec) { s.Prices[0].Verified = false }},
		{"unknown model price", func(s *CampaignBudgetSpec) { s.Plans[0].Plan.Model = "new-model" }},
		{"retries", func(s *CampaignBudgetSpec) { s.Plans[0].Plan.MaxRetries = 1 }},
		{"nonfinite ceiling", func(s *CampaignBudgetSpec) { s.HardCeilingUSD = math.NaN() }},
		{"wrong output bound", func(s *CampaignBudgetSpec) { s.MaxOutputTokens = 4095 }},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			spec := validBudgetSpec()
			test.mutate(&spec)
			_, err := PreflightCampaignBudget(spec)
			require.ErrorIs(t, err, ErrInvalidCampaignBudget)
		})
	}
}

func TestPreflightCampaignBudgetRejectsOverCeilingAndRequestLimit(t *testing.T) {
	spec := validBudgetSpec()
	spec.HardCeilingUSD = 0.01
	_, err := PreflightCampaignBudget(spec)
	require.ErrorIs(t, err, ErrCampaignBudgetExceeded)

	spec = validBudgetSpec()
	spec.Plans = []CampaignBudgetPlan{budgetPlan("gemini-3.1-flash-lite", CampaignBudgetMaxRequests+1, 1, 0)}
	_, err = PreflightCampaignBudget(spec)
	require.ErrorIs(t, err, ErrInvalidCampaignBudget)
}

func TestCampaignBudgetGuardStopsBeforeNextRequestWhenObservedSpendExceedsCeiling(t *testing.T) {
	spec := validBudgetSpec()
	spec.HardCeilingUSD = 0.1
	estimate, err := PreflightCampaignBudget(spec)
	require.NoError(t, err)
	guard, err := NewCampaignBudgetGuard(estimate)
	require.NoError(t, err)
	require.NoError(t, guard.BeforeRequest(spec.Plans[0].Plan))
	// The observed input is deliberately much larger than the CountTokens bound;
	// the guard refuses to certify the next request rather than hiding the drift.
	require.NoError(t, guard.RecordObserved(spec.Plans[0].Plan, 100, 100))
	require.NoError(t, guard.BeforeRequest(spec.Plans[0].Plan))
	require.ErrorIs(t, guard.RecordObserved(spec.Plans[0].Plan, 1_000_000, 4096), ErrCampaignBudgetExceeded)
	used, spent := guard.Snapshot()
	require.Equal(t, 1, used)
	require.Greater(t, spent, 0.0)
}

func TestCampaignBudgetGuardIsConcurrencySafe(t *testing.T) {
	spec := validBudgetSpec()
	spec.Plans = []CampaignBudgetPlan{budgetPlan("gemini-3.5-flash-lite", 4, 1, 0)}
	estimate, err := PreflightCampaignBudget(spec)
	require.NoError(t, err)
	guard, err := NewCampaignBudgetGuard(estimate)
	require.NoError(t, err)
	var wg sync.WaitGroup
	var mu sync.Mutex
	var errs []error
	for i := 0; i < 4; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			if err := guard.BeforeRequest(spec.Plans[0].Plan); err != nil {
				mu.Lock()
				errs = append(errs, err)
				mu.Unlock()
				return
			}
			if err := guard.RecordObserved(spec.Plans[0].Plan, 1, 1); err != nil {
				mu.Lock()
				errs = append(errs, err)
				mu.Unlock()
			}
		}()
	}
	wg.Wait()
	require.Empty(t, errs)
	used, _ := guard.Snapshot()
	require.Equal(t, 4, used)
	require.True(t, errors.Is(guard.BeforeRequest(spec.Plans[0].Plan), ErrCampaignBudgetExceeded))
}

type budgetCountTransport struct{}

func (budgetCountTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	body := `{"totalTokens":123}`
	return &http.Response{StatusCode: http.StatusOK, Body: io.NopCloser(strings.NewReader(body)), Header: make(http.Header), Request: req}, nil
}

type campaignBudgetEndpointTransport struct {
	countTokens atomic.Int32
	generate    atomic.Int32
}

func (t *campaignBudgetEndpointTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	if strings.Contains(req.URL.Path, "countTokens") {
		t.countTokens.Add(1)
		return &http.Response{StatusCode: http.StatusOK, Body: io.NopCloser(strings.NewReader(`{"totalTokens":123}`)), Header: make(http.Header), Request: req}, nil
	}
	if strings.Contains(req.URL.Path, "generateContent") {
		t.generate.Add(1)
	}
	return &http.Response{StatusCode: http.StatusInternalServerError, Body: io.NopCloser(strings.NewReader(`{}`)), Header: make(http.Header), Request: req}, nil
}

func TestPreflightGeminiCampaignBudgetCountsExactInputsAcrossFourExecutorsWithoutGeneration(t *testing.T) {
	dataset := rankingEvaluationFixture(t)
	transport := &campaignBudgetEndpointTransport{}
	limits := ExecutionLimits{Concurrency: 1, AttemptTimeout: time.Second, GlobalTimeout: time.Minute, MinInterval: time.Second, MaxOutputTokens: CampaignBudgetMaxOutputTokens}
	executors := make([]*GeminiExecutor, 0, 4)
	for i := 0; i < 4; i++ {
		plan := &Plan{Model: "gemini-3.5-flash-lite", Cases: []PlannedCase{{CaseID: "RK-test"}}, Trials: 90, MaxRetries: 0, MaximumRequests: 90, RequestLimit: 90}
		executor, err := newGeminiExecutor(dataset, plan, limits, true, "test-key", transport)
		require.NoError(t, err)
		executors = append(executors, executor)
	}
	estimate, err := PreflightGeminiCampaignBudget(context.Background(), executors, []CampaignPrice{{Model: "gemini-3.5-flash-lite", InputUSDPerMillion: .30, OutputUSDPerMillion: 2.50, Verified: true}}, 10, "pricing-source")
	require.NoError(t, err)
	require.Equal(t, CampaignBudgetMaxRequests, estimate.RequestCount)
	require.Equal(t, int32(4), transport.countTokens.Load())
	require.Zero(t, transport.generate.Load())
	for _, executor := range executors {
		used, spent, ok := executor.CampaignBudgetSnapshot()
		require.True(t, ok)
		require.Zero(t, used)
		require.Zero(t, spent)
	}
}

func TestPreflightGeminiCampaignBudgetCountsExactExecutorInputs(t *testing.T) {
	dataset := rankingEvaluationFixture(t)
	plan := &Plan{Model: "gemini-3.5-flash-lite", Cases: []PlannedCase{{CaseID: "RK-test"}}, Trials: 2, MaxRetries: 0, MaximumRequests: 2, RequestLimit: 2}
	limits := ExecutionLimits{Concurrency: 1, AttemptTimeout: time.Second, GlobalTimeout: time.Minute, MinInterval: time.Second, MaxOutputTokens: CampaignBudgetMaxOutputTokens}
	executor, err := newGeminiExecutor(dataset, plan, limits, true, "test-key", budgetCountTransport{})
	require.NoError(t, err)
	estimate, err := PreflightGeminiCampaignBudget(context.Background(), []*GeminiExecutor{executor}, []CampaignPrice{{Model: plan.Model, InputUSDPerMillion: .30, OutputUSDPerMillion: 2.50, Verified: true}}, 10, "pricing-source")
	require.NoError(t, err)
	require.Equal(t, 2, estimate.RequestCount)
	require.Equal(t, int64(246), estimate.InputTokens)
	require.NotNil(t, executor.budget)
}

func TestObservedUsageIncludesThoughtTokens(t *testing.T) {
	input, output, err := observedUsage(json.RawMessage(`{"usageMetadata":{"promptTokenCount":12,"candidatesTokenCount":3,"thoughtsTokenCount":5}}`))
	require.NoError(t, err)
	require.Equal(t, int64(12), input)
	require.Equal(t, int64(8), output)
	_, _, err = observedUsage(json.RawMessage(`{"usageMetadata":{"promptTokenCount":12,"candidatesTokenCount":3}}`))
	require.Error(t, err)
}

func TestPreflightCampaignBudgetMatchesProtocolUpperBound(t *testing.T) {
	spec := CampaignBudgetSpec{
		Plans:           []CampaignBudgetPlan{budgetPlan("gemini-3.5-flash-lite", CampaignBudgetMaxRequests, 1, 0)},
		Prices:          []CampaignPrice{{Model: "gemini-3.5-flash-lite", InputUSDPerMillion: .30, OutputUSDPerMillion: 2.50, Verified: true}},
		HardCeilingUSD:  10,
		MaxOutputTokens: CampaignBudgetMaxOutputTokens,
		PricingSource:   "pricing-source",
	}
	estimate, err := PreflightCampaignBudget(spec)
	require.NoError(t, err)
	require.Equal(t, int64(7_200_000), estimate.ReservedInputTokens)
	require.Equal(t, int64(1_474_560), estimate.ReservedOutputTokens)
	require.InDelta(t, 5.8464, estimate.ReservedTotalUSD, 0.0000001)
}

func TestCampaignBudgetGuardConsumesMaximumForUnknownUsageAndContinues(t *testing.T) {
	spec := validBudgetSpec()
	estimate, err := PreflightCampaignBudget(spec)
	require.NoError(t, err)
	guard, err := NewCampaignBudgetGuard(estimate)
	require.NoError(t, err)
	plan := spec.Plans[0].Plan
	require.NoError(t, guard.BeforeRequest(plan))
	require.NoError(t, guard.RecordUnknown(plan))
	used, spent := guard.Snapshot()
	require.Equal(t, 1, used)
	require.InDelta(t, 0.01624, spent, 0.0000001)
	require.NoError(t, guard.BeforeRequest(plan))
}

type unknownUsageTransport struct {
	countTokens atomic.Int32
	generate    atomic.Int32
	invalid     bool
}

func (t *unknownUsageTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	if strings.Contains(req.URL.Path, "countTokens") {
		t.countTokens.Add(1)
		return &http.Response{StatusCode: http.StatusOK, Body: io.NopCloser(strings.NewReader(`{"totalTokens":123}`)), Header: make(http.Header), Request: req}, nil
	}
	t.generate.Add(1)
	body := `{"candidates":[{"content":{"parts":[{"text":"{\"recommendations\":[{\"reference\":\"a\",\"reason\":\"a\"},{\"reference\":\"b\",\"reason\":\"b\"},{\"reference\":\"c\",\"reason\":\"c\"}]}"}]}}]}`
	if t.invalid {
		body = `{"candidates":[{"content":{"parts":[{"text":"not json"}]}}]}`
	}
	return &http.Response{StatusCode: http.StatusOK, Body: io.NopCloser(strings.NewReader(body)), Header: make(http.Header), Request: req}, nil
}

func TestGeminiExecutorPreservesValidResultWhenUsageIsUnknown(t *testing.T) {
	dataset := rankingEvaluationFixture(t)
	plan := &Plan{Model: "gemini-3.5-flash-lite", Cases: []PlannedCase{{CaseID: "RK-test"}}, Trials: 1, MaxRetries: 0, MaximumRequests: 1, RequestLimit: 1}
	limits := ExecutionLimits{Concurrency: 1, AttemptTimeout: time.Second, GlobalTimeout: time.Minute, MinInterval: time.Second, MaxOutputTokens: CampaignBudgetMaxOutputTokens}
	transport := &unknownUsageTransport{}
	executor, err := newGeminiExecutor(dataset, plan, limits, true, "test-key", transport)
	require.NoError(t, err)
	_, err = PreflightGeminiCampaignBudget(context.Background(), []*GeminiExecutor{executor}, []CampaignPrice{{Model: plan.Model, InputUSDPerMillion: .30, OutputUSDPerMillion: 2.50, Verified: true}}, 10, "pricing-source")
	require.NoError(t, err)
	output, err := executor.Execute(context.Background(), "RK-test")
	require.NoError(t, err)
	require.Equal(t, 1, output.RequestCount)
	require.Equal(t, int32(1), transport.generate.Load())
	used, _, ok := executor.CampaignBudgetSnapshot()
	require.True(t, ok)
	require.Equal(t, 1, used)
}

func TestGeminiExecutorPreservesQualityErrorWhenUsageIsUnknown(t *testing.T) {
	dataset := rankingEvaluationFixture(t)
	plan := &Plan{Model: "gemini-3.5-flash-lite", Cases: []PlannedCase{{CaseID: "RK-test"}}, Trials: 1, MaxRetries: 0, MaximumRequests: 1, RequestLimit: 1}
	limits := ExecutionLimits{Concurrency: 1, AttemptTimeout: time.Second, GlobalTimeout: time.Minute, MinInterval: time.Second, MaxOutputTokens: CampaignBudgetMaxOutputTokens}
	transport := &unknownUsageTransport{invalid: true}
	executor, err := newGeminiExecutor(dataset, plan, limits, true, "test-key", transport)
	require.NoError(t, err)
	_, err = PreflightGeminiCampaignBudget(context.Background(), []*GeminiExecutor{executor}, []CampaignPrice{{Model: plan.Model, InputUSDPerMillion: .30, OutputUSDPerMillion: 2.50, Verified: true}}, 10, "pricing-source")
	require.NoError(t, err)
	output, err := executor.Execute(context.Background(), "RK-test")
	require.Error(t, err) // parser/quality error is preserved
	require.NotContains(t, err.Error(), "budget_usage_unknown")
	require.Equal(t, 1, output.RequestCount)
	used, _, ok := executor.CampaignBudgetSnapshot()
	require.True(t, ok)
	require.Equal(t, 1, used)
}
