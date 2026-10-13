package evals

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"strings"
	"sync"
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
	apiKeys   []string
	lanes     []*projectLane
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
		return nil, fmt.Errorf("Gemini API key is required for live execution")
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

// NewGeminiExecutorWithKeys binds one independent project credential to each
// worker lane; the application CHATBOT_API_KEY setting is not read or changed.
func NewGeminiExecutorWithKeys(dataset *Dataset, plan *Plan, limits ExecutionLimits, allowLive bool, keys []string) (*GeminiExecutor, error) {
	return newGeminiExecutorWithKeys(dataset, plan, limits, allowLive, keys, http.DefaultTransport)
}
func newGeminiExecutorWithKeys(dataset *Dataset, plan *Plan, limits ExecutionLimits, allowLive bool, keys []string, transport http.RoundTripper) (*GeminiExecutor, error) {
	if len(keys) == 0 || limits.Concurrency > len(keys) {
		return nil, fmt.Errorf("evaluation concurrency requires at least one distinct key per lane")
	}
	normalized, err := ParseEvaluationAPIKeys(strings.Join(keys, ","))
	if err != nil {
		return nil, err
	}
	executor, err := newGeminiExecutor(dataset, plan, limits, allowLive, normalized[0], transport)
	if err != nil {
		return nil, err
	}
	executor.apiKeys = normalized
	executor.lanes = make([]*projectLane, len(normalized))
	for i := range executor.lanes {
		executor.lanes[i] = &projectLane{interval: limits.MinInterval}
	}
	return executor, nil
}
func (e *GeminiExecutor) LaneCount() int {
	if len(e.apiKeys) > 0 {
		return len(e.apiKeys)
	}
	return 1
}

// LaneAvailable reports local shared 429 state, not provider health.
func (e *GeminiExecutor) LaneAvailable(lane int) bool {
	return lane >= 0 && lane < e.LaneCount() && (len(e.lanes) == 0 || e.lanes[lane].available())
}

func (e *GeminiExecutor) ExecuteLane(ctx context.Context, caseID string, lane int) (ExecutionOutput, error) {
	if lane < 0 || lane >= e.LaneCount() {
		return ExecutionOutput{}, fmt.Errorf("invalid evaluation lane")
	}
	if !e.LaneAvailable(lane) {
		return ExecutionOutput{}, &ExecutionError{Kind: "rate_limited", LaneStop: true, NotExecuted: true, Cause: ErrProjectLaneStopped}
	}
	key := e.apiKey
	if len(e.apiKeys) > 0 {
		key = e.apiKeys[lane]
	}
	out, err := e.execute(ctx, caseID, key, lane)
	var classified *ExecutionError
	if errors.As(err, &classified) && classified.Kind == "rate_limited" {
		classified.Stop = false
		classified.LaneStop = true
	}
	return out, err
}

type limitedTransport struct {
	requestID   *string
	executor    *GeminiExecutor
	used        atomic.Bool
	count       int
	status      int
	usage       json.RawMessage
	usageSource string
	reservation *CampaignBudgetReservation
	lane        *projectLane
	refusal     error
}

func (t *limitedTransport) RoundTrip(request *http.Request) (*http.Response, error) {
	if err := request.Context().Err(); err != nil {
		t.refusal = err
		return nil, err
	}
	if t.used.Swap(true) {
		return nil, fmt.Errorf("implicit SDK retry or redirect blocked")
	}
	if t.lane != nil {
		if err := t.lane.wait(request.Context()); err != nil {
			t.refusal = err
			return nil, err
		}
	}
	for {
		used := t.executor.requests.Load()
		if used >= int64(t.executor.plan.RequestLimit) {
			t.refusal = ErrRequestBudget
			return nil, ErrRequestBudget
		}
		if t.executor.requests.CompareAndSwap(used, used+1) {
			break
		}
	}
	if t.executor.budget != nil {
		reservation, err := t.executor.budget.Reserve(t.executor.plan)
		if err != nil {
			t.executor.requests.Add(-1)
			t.refusal = err
			return nil, err
		}
		t.reservation = reservation
	}
	t.count++
	response, err := t.executor.transport.RoundTrip(request)
	if response != nil {
		t.status = response.StatusCode
		if t.lane != nil && response.StatusCode == http.StatusTooManyRequests {
			t.lane.stop()
		}
		if id := response.Header.Get("x-request-id"); id != "" {
			t.requestID = &id
		}
		if response.Body != nil {
			response.Body = &usageCaptureBody{ReadCloser: response.Body, capture: func(raw []byte) {
				var envelope struct {
					Usage json.RawMessage `json:"usageMetadata"`
				}
				if json.Unmarshal(raw, &envelope) == nil && len(envelope.Usage) > 0 && string(envelope.Usage) != "null" {
					t.usage = append(json.RawMessage(nil), envelope.Usage...)
					t.usageSource = "http_usage_metadata"
				}
			}}
		}
	}
	return response, err
}

// usageCaptureBody retains only usageMetadata after the SDK consumes the body.
// HTTP headers and the original HTTP envelope are never persisted.
type usageCaptureBody struct {
	io.ReadCloser
	buffer  bytes.Buffer
	once    sync.Once
	capture func([]byte)
}

func (b *usageCaptureBody) Read(p []byte) (int, error) {
	n, err := b.ReadCloser.Read(p)
	b.buffer.Write(p[:n])
	if err == io.EOF {
		b.finish()
	}
	return n, err
}
func (b *usageCaptureBody) finish() {
	b.once.Do(func() { b.capture(b.buffer.Bytes()); b.buffer.Reset() })
}
func (b *usageCaptureBody) Close() error { b.finish(); return b.ReadCloser.Close() }

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

// CountInputTokens counts the shared contents payload, including image bytes.
// It does not claim that CountTokens includes the generation response schema;
// native generation usage is authoritative and the budget retains its hard bound.
func (e *GeminiExecutor) CountInputTokens(ctx context.Context, caseID string) (int64, error) {
	if e == nil || e.dataset == nil || e.plan == nil {
		return 0, fmt.Errorf("validated execution plan is required")
	}
	ctx, cancel := context.WithTimeout(ctx, e.limits.AttemptTimeout)
	defer cancel()
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
	if len(e.lanes) > 0 {
		if err := e.lanes[0].wait(ctx); err != nil {
			return 0, err
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
	for _, c := range e.dataset.SM {
		if c.ID != baseCaseID {
			continue
		}
		messages, mapErr := e.dataset.MapSM(c.Input)
		if mapErr != nil {
			return 0, mapErr
		}
		return bot.CountSummaryInputTokens(ctx, c.Input.PreviousSummary, messages)
	}
	return 0, fmt.Errorf("unknown case %q", caseID)
}

func (e *GeminiExecutor) Execute(ctx context.Context, caseID string) (ExecutionOutput, error) {
	return e.ExecuteLane(ctx, caseID, 0)
}
func (e *GeminiExecutor) execute(ctx context.Context, caseID, apiKey string, lane int) (ExecutionOutput, error) {
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
	if len(e.lanes) > 0 {
		transport.lane = e.lanes[lane]
	}
	client := &http.Client{Transport: transport, CheckRedirect: func(*http.Request, []*http.Request) error { return fmt.Errorf("redirects disabled for evaluation") }}
	var trace chatbot.GenerationTrace
	bot, err := chatbot.NewGeminiChatbotWithOptions(e.plan.Model, apiKey, chatbot.GeminiOptions{HTTPClient: client, MaxOutputTokens: e.limits.MaxOutputTokens, Observer: func(_ context.Context, t chatbot.GenerationTrace) { trace = t }})
	if err != nil {
		return result, err
	}
	found := false
	for _, c := range e.dataset.PD {
		if c.ID == baseCaseID {
			found = true
			question, categories, mapErr := e.dataset.MapPD(c.Input)
			if mapErr != nil {
				return result, &ExecutionError{Kind: "asset_error", Cause: mapErr}
			}
			_, err = bot.AnswerHomeProblemQuestion(ctx, question, categories)
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
				_, err = bot.RankProviders(ctx, request)
				break
			}
		}
	}
	if !found {
		for _, c := range e.dataset.SM {
			if c.ID != baseCaseID {
				continue
			}
			found = true
			messages, mapErr := e.dataset.MapSM(c.Input)
			if mapErr != nil {
				return result, mapErr
			}
			_, err = bot.SummarizeHomeProblemConversation(ctx, c.Input.PreviousSummary, messages)
			break
		}
	}
	if !found {
		return result, fmt.Errorf("unknown case %q", caseID)
	}
	result = ExecutionOutput{Provider: "gemini", ProviderUsage: transport.usage, UsageSource: transport.usageSource, RequestID: transport.requestID, Input: trace.Contents, GenerationConfig: trace.Config, RawOutput: trace.RawText, ProviderResponse: trace.Response, RequestCount: transport.count}
	if transport.status != 0 {
		result.HTTPStatus = &transport.status
	}
	if transport.reservation != nil {
		inputTokens, outputTokens, usageErr := observedUsage(usageEnvelope(result.ProviderUsage, trace.Response))
		if usageErr != nil {
			// The response and its original error remain intact. Consume the
			// preflight maximum instead of converting a valid model result into
			// an execution failure solely because usage metadata is absent.
			if reserveErr := transport.reservation.RecordUnknown(); reserveErr != nil {
				return result, &ExecutionError{Kind: "budget_exhausted", Stop: true, Cause: reserveErr}
			}
		} else if usageErr = transport.reservation.RecordObserved(inputTokens, outputTokens); usageErr != nil {
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
		result.ParsedOutput = json.RawMessage(trace.RawText)
	}
	if err != nil {
		if transport.count == 0 && (transport.refusal != nil || ctx.Err() != nil) {
			cause := transport.refusal
			if cause == nil {
				cause = ctx.Err()
			}
			classified := classifyExecutionError(cause, transport.status).(*ExecutionError)
			classified.NotExecuted = true
			return result, classified
		}
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
	if errors.Is(err, ErrProjectLaneStopped) {
		return &ExecutionError{Kind: "rate_limited", Stop: true, Cause: err}
	}
	if errors.Is(err, ErrRequestBudget) || errors.Is(err, ErrCampaignBudgetExceeded) {
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

// usageEnvelope preserves presence from HTTP when available; SDK-decoded data
// remains a legacy fallback and must not be used to infer omitted zeros.
func usageEnvelope(usage, legacy json.RawMessage) json.RawMessage {
	if len(usage) == 0 {
		return legacy
	}
	return append(append(json.RawMessage(`{"usageMetadata":`), usage...), '}')
}

func observedAttemptUsage(a Attempt) (int64, int64, error) {
	return observedUsage(usageEnvelope(a.ProviderUsage, a.ProviderResponse))
}
