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
func (e *GeminiExecutor) Execute(ctx context.Context, caseID string) (ExecutionOutput, error) {
	var result ExecutionOutput
	allowed := false
	for _, c := range e.plan.Cases {
		if c.CaseID == caseID {
			allowed = true
			break
		}
	}
	if !allowed {
		return result, &ExecutionError{Kind: "unauthorized_case", Stop: true, Cause: fmt.Errorf("case %q is outside authorized plan", caseID)}
	}
	transport := &limitedTransport{executor: e}
	client := &http.Client{Transport: transport, CheckRedirect: func(*http.Request, []*http.Request) error { return fmt.Errorf("redirects disabled for evaluation") }}
	var trace chatbot.GenerationTrace
	bot, err := chatbot.NewGeminiChatbotWithOptions(e.plan.Model, e.apiKey, chatbot.GeminiOptions{HTTPClient: client, MaxOutputTokens: e.limits.MaxOutputTokens, Observer: func(_ context.Context, t chatbot.GenerationTrace) { trace = t }})
	if err != nil {
		return result, err
	}
	var parsed any
	found := false
	for _, c := range e.dataset.PD {
		if c.ID == caseID {
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
			if c.ID == caseID {
				found = true
				request, mapErr := c.Input.DomainRequest()
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
