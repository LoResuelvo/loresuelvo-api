package evals

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net"
	"net/http"
	"strings"
	"sync/atomic"

	"github.com/LoResuelvo/loresuelvo-api/internal/adapters/chatbot"
	"google.golang.org/genai"
)

var ErrRequestBudget = errors.New("request budget exhausted")

type GeminiExecutor struct {
	dataset   *Dataset
	plan      *Plan
	limits    ExecutionLimits
	apiKey    string
	transport http.RoundTripper
	requests  atomic.Int64
	budget    *CampaignBudgetGuard
}

// NewGeminiExecutor validates opt-in before constructing any live dependency.
func NewGeminiExecutor(dataset *Dataset, plan *Plan, limits ExecutionLimits, allowLive bool, apiKey string) (*GeminiExecutor, error) {
	return newGeminiExecutor(dataset, plan, limits, allowLive, apiKey, http.DefaultTransport)
}
func newGeminiExecutor(dataset *Dataset, plan *Plan, limits ExecutionLimits, allowLive bool, apiKey string, transport http.RoundTripper) (*GeminiExecutor, error) {
	if !allowLive {
		return nil, fmt.Errorf("live execution requires --allow-live")
	}
	if strings.TrimSpace(apiKey) == "" {
		return nil, fmt.Errorf("CHATBOT_API_KEY is required for live execution")
	}
	if err := limits.Validate(); err != nil {
		return nil, err
	}
	if plan == nil || dataset == nil || plan.RequestLimit <= 0 {
		return nil, fmt.Errorf("validated execution plan is required")
	}
	if plan.UsesHoldout && !plan.HoldoutAuthorized {
		return nil, fmt.Errorf("reserve execution requires --allow-holdout")
	}
	return &GeminiExecutor{dataset: dataset, plan: plan, limits: limits, apiKey: strings.TrimSpace(apiKey), transport: transport}, nil
}

type limitedTransport struct {
	requestID *string
	executor  *GeminiExecutor
	used      atomic.Bool
	count     int
	status    int
}

func (t *limitedTransport) RoundTrip(request *http.Request) (*http.Response, error) {
	if err := request.Context().Err(); err != nil {
		return nil, err
	}
	if t.used.Swap(true) {
		return nil, fmt.Errorf("implicit SDK retry or redirect blocked")
	}
	if t.executor.requests.Add(1) > int64(t.executor.plan.RequestLimit) {
		return nil, ErrRequestBudget
	}
	t.count++
	response, err := t.executor.transport.RoundTrip(request)
	if response != nil {
		t.status = response.StatusCode
		if id := response.Header.Get("x-request-id"); id != "" {
			t.requestID = &id
		}
	}
	return response, err
}

type singleRequestTransport struct {
	base http.RoundTripper
	used atomic.Bool
}

func (t *singleRequestTransport) RoundTrip(request *http.Request) (*http.Response, error) {
	if t.used.Swap(true) {
		return nil, fmt.Errorf("implicit SDK retry blocked during token counting")
	}
	return t.base.RoundTrip(request)
}

// CountInputTokens counts the exact prompt for one planned case, including
// image bytes for prediagnosis. It uses the provider CountTokens endpoint and
// never generates a response.
func (e *GeminiExecutor) CountInputTokens(ctx context.Context, caseID string) (int64, error) {
	if e == nil || e.dataset == nil || e.plan == nil {
		return 0, fmt.Errorf("validated execution plan is required")
	}
	baseCaseID := caseID
	var variant *MetamorphicVariant
	for _, planned := range e.plan.Cases {
		if planned.CaseID == caseID {
			variant = planned.Variant
			if variant != nil {
				baseCaseID = variant.BaseCaseID
			}
			break
		}
	}
	bot, err := chatbot.NewGeminiChatbotWithOptions(e.plan.Model, e.apiKey, chatbot.GeminiOptions{HTTPClient: &http.Client{Transport: &singleRequestTransport{base: e.transport}, CheckRedirect: func(*http.Request, []*http.Request) error { return fmt.Errorf("redirects disabled for token counting") }}})
	if err != nil {
		return 0, err
	}
	for _, c := range e.dataset.PD {
		if c.ID != baseCaseID {
			continue
		}
		question, categories, mapErr := e.dataset.MapPD(c.Input)
		if mapErr != nil {
			return 0, mapErr
		}
		return bot.CountAnswerInputTokens(ctx, question, categories)
	}
	for _, c := range e.dataset.RK {
		if c.ID != baseCaseID {
			continue
		}
		input := c.Input
		if variant != nil {
			transformed, transformErr := TransformRanking(input, variant.Transformation, variant.Seed)
			if transformErr != nil {
				return 0, transformErr
			}
			input = transformed.Input
		}
		request, mapErr := input.DomainRequest()
		if mapErr != nil {
			return 0, mapErr
		}
		return bot.CountRankingInputTokens(ctx, request)
	}
	return 0, fmt.Errorf("unknown case %q", caseID)
}

func (e *GeminiExecutor) Execute(ctx context.Context, caseID string) (ExecutionOutput, error) {
	var result ExecutionOutput
	allowed := false
	var variant *MetamorphicVariant
	for _, c := range e.plan.Cases {
		if c.CaseID == caseID {
			allowed = true
			variant = c.Variant
			break
		}
	}
	if !allowed {
		return result, &ExecutionError{Kind: "unauthorized_case", Stop: true, Cause: fmt.Errorf("case %q is outside authorized plan", caseID)}
	}
	baseCaseID := caseID
	if variant != nil {
		baseCaseID = variant.BaseCaseID
	}
	transport := &limitedTransport{executor: e}
	client := &http.Client{Transport: transport, CheckRedirect: func(*http.Request, []*http.Request) error { return fmt.Errorf("redirects disabled for evaluation") }}
	var trace chatbot.GenerationTrace
	bot, err := chatbot.NewGeminiChatbotWithOptions(e.plan.Model, e.apiKey, chatbot.GeminiOptions{HTTPClient: client, MaxOutputTokens: e.limits.MaxOutputTokens, Observer: func(_ context.Context, t chatbot.GenerationTrace) { trace = t }})
	if err != nil {
		return result, err
	}
	if e.budget != nil {
		if err := e.budget.BeforeRequest(e.plan); err != nil {
			return result, &ExecutionError{Kind: "budget_exhausted", Stop: true, Cause: err}
		}
	}
	var parsed any
	found := false
	for _, c := range e.dataset.PD {
		if c.ID == baseCaseID {
			found = true
			question, categories, mapErr := e.dataset.MapPD(c.Input)
			if mapErr != nil {
				return result, &ExecutionError{Kind: "asset_error", Cause: mapErr}
			}
			parsed, err = bot.AnswerHomeProblemQuestion(ctx, question, categories)
			break
		}
	}
	if !found {
		for _, c := range e.dataset.RK {
			if c.ID == baseCaseID {
				found = true
				input := c.Input
				if variant != nil {
					transformed, transformErr := TransformRanking(input, variant.Transformation, variant.Seed)
					if transformErr != nil {
						return result, transformErr
					}
					input = transformed.Input
				}
				request, mapErr := input.DomainRequest()
				if mapErr != nil {
					return result, mapErr
				}
				parsed, err = bot.RankProviders(ctx, request)
				break
			}
		}
	}
	if !found {
		return result, fmt.Errorf("unknown case %q", caseID)
	}
	result = ExecutionOutput{RequestID: transport.requestID, Input: trace.Contents, GenerationConfig: trace.Config, RawOutput: trace.RawText, ProviderResponse: trace.Response, RequestCount: transport.count}
	if e.budget != nil && transport.count > 0 {
		inputTokens, outputTokens, usageErr := observedUsage(trace.Response)
		if usageErr != nil {
			// The response and its original error remain intact. Consume the
			// preflight maximum instead of converting a valid model result into
			// an execution failure solely because usage metadata is absent.
			if reserveErr := e.budget.RecordUnknown(e.plan); reserveErr != nil {
				return result, &ExecutionError{Kind: "budget_exhausted", Stop: true, Cause: reserveErr}
			}
		} else if usageErr = e.budget.RecordObserved(e.plan, inputTokens, outputTokens); usageErr != nil {
			return result, &ExecutionError{Kind: "budget_exhausted", Stop: true, Cause: usageErr}
		}
	}
	if len(trace.Contents) > 0 {
		hash, hashErr := promptHash(trace.Contents)
		if hashErr != nil {
			return result, hashErr
		}
		result.PromptSHA256 = hash
	}

	if err == nil {
		result.ParsedOutput, err = json.Marshal(parsed)
	}
	if err != nil {
		return result, classifyExecutionError(err, transport.status)
	}
	return result, nil
}
func observedUsage(raw json.RawMessage) (int64, int64, error) {
	var response struct {
		Usage *struct {
			Input    *int64 `json:"promptTokenCount"`
			Output   *int64 `json:"candidatesTokenCount"`
			Thoughts *int64 `json:"thoughtsTokenCount"`
		} `json:"usageMetadata"`
	}
	if len(raw) == 0 || json.Unmarshal(raw, &response) != nil || response.Usage == nil || response.Usage.Input == nil || response.Usage.Output == nil {
		return 0, 0, fmt.Errorf("provider token usage is missing")
	}
	if *response.Usage.Input < 0 || *response.Usage.Output < 0 || response.Usage.Thoughts == nil || *response.Usage.Thoughts < 0 {
		return 0, 0, fmt.Errorf("provider token usage is invalid or incomplete")
	}
	return *response.Usage.Input, *response.Usage.Output + *response.Usage.Thoughts, nil
}

func classifyExecutionError(err error, status int) error {
	if errors.Is(err, ErrRequestBudget) {
		return &ExecutionError{Kind: "budget_exhausted", Stop: true, Cause: err}
	}
	var apiErr genai.APIError
	if status == 0 && errors.As(err, &apiErr) {
		status = apiErr.Code
	}
	if status == http.StatusTooManyRequests {
		return &ExecutionError{Kind: "rate_limited", Stop: true, Cause: err}
	}
	if status == http.StatusBadRequest || status == http.StatusUnprocessableEntity || status == http.StatusUnauthorized || status == http.StatusForbidden || status == http.StatusNotFound {
		return &ExecutionError{Kind: "provider_configuration_error", Stop: true, Cause: err}
	}
	var networkError net.Error
	retryable := status >= 500 || errors.Is(err, context.DeadlineExceeded) || errors.As(err, &networkError)
	return &ExecutionError{Kind: "execution_error", Retryable: retryable, Cause: err}
}

// CampaignBudgetSnapshot exposes the final shared guard counters without
// exposing the guard or credentials. ok is false when this executor was not
// attached to a campaign preflight guard.
func (e *GeminiExecutor) CampaignBudgetSnapshot() (used int, spentUSD float64, ok bool) {
	if e == nil || e.budget == nil {
		return 0, 0, false
	}
	used, spentUSD = e.budget.Snapshot()
	return used, spentUSD, true
}
