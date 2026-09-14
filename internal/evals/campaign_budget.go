package evals

import (
	"context"
	"errors"
	"fmt"
	"math"
	"strings"
	"sync"
)

var ErrInvalidCampaignBudget = errors.New("invalid campaign budget")
var ErrCampaignBudgetExceeded = errors.New("campaign budget exceeded")

const (
	CampaignBudgetCeilingUSD      = 10.0
	CampaignBudgetMaxRequests     = 360
	CampaignBudgetMaxInputTokens  = 20000
	CampaignBudgetMaxOutputTokens = 4096
)

// CampaignPrice is a provider price verified for this campaign execution date.
// Prices are USD per million tokens. Unverified or missing prices fail closed.
type CampaignPrice struct {
	Model               string
	InputUSDPerMillion  float64
	OutputUSDPerMillion float64
	Verified            bool
}

// InputTokenEvidence is the input-token count for one effective provider
// request. CountTokens should include all text and image parts; a conservative
// bound is accepted only when the caller has included every part.
type InputTokenEvidence struct {
	Tokens int64
	Source string // count_tokens or conservative_bound
}

type CampaignBudgetPlan struct {
	Plan        *Plan
	InputTokens []InputTokenEvidence
}

type CampaignBudgetSpec struct {
	Plans           []CampaignBudgetPlan
	Prices          []CampaignPrice
	HardCeilingUSD  float64
	MaxOutputTokens int64
	PricingSource   string
}

type CampaignBudgetEstimate struct {
	RequestCount         int               `json:"request_count"`
	InputTokens          int64             `json:"counted_input_tokens"`  // observed CountTokens sum
	ReservedInputTokens  int64             `json:"reserved_input_tokens"` // hard bound reserved for every request
	ReservedOutputTokens int64             `json:"reserved_output_tokens"`
	InputUSDPerMillion   float64           `json:"input_usd_per_million"`
	OutputUSDPerMillion  float64           `json:"output_usd_per_million"`
	ReservedInputUSD     float64           `json:"reserved_input_usd"`
	ReservedOutputUSD    float64           `json:"reserved_output_usd"`
	ReservedTotalUSD     float64           `json:"reserved_total_usd"`
	HardCeilingUSD       float64           `json:"hard_ceiling_usd"`
	HeadroomUSD          float64           `json:"headroom_usd"`
	inputBounds          []int64           // aggregate per-request bounds
	planBounds           map[*Plan][]int64 // bounds keyed to the exact planned execution
}

// PreflightCampaignBudget verifies all planned requests before any generation
// call. It reserves the maximum configured output for every request and the
// highest verified rate across the requested models, so it is conservative.
func PreflightCampaignBudget(spec CampaignBudgetSpec) (CampaignBudgetEstimate, error) {
	var result CampaignBudgetEstimate
	result.planBounds = make(map[*Plan][]int64)
	if spec.HardCeilingUSD <= 0 || !finite(spec.HardCeilingUSD) {
		return result, fmt.Errorf("%w: positive finite hard ceiling required", ErrInvalidCampaignBudget)
	}
	if spec.MaxOutputTokens != CampaignBudgetMaxOutputTokens {
		return result, fmt.Errorf("%w: max output tokens must be %d", ErrInvalidCampaignBudget, CampaignBudgetMaxOutputTokens)
	}
	if strings.TrimSpace(spec.PricingSource) == "" || len(spec.Prices) == 0 {
		return result, fmt.Errorf("%w: verified pricing source and prices are required", ErrInvalidCampaignBudget)
	}
	prices := make(map[string]CampaignPrice, len(spec.Prices))
	var inputRate, outputRate float64
	for _, price := range spec.Prices {
		if strings.TrimSpace(price.Model) == "" || !price.Verified || !finite(price.InputUSDPerMillion) || !finite(price.OutputUSDPerMillion) || price.InputUSDPerMillion <= 0 || price.OutputUSDPerMillion <= 0 {
			return result, fmt.Errorf("%w: unverified or invalid price for %q", ErrInvalidCampaignBudget, price.Model)
		}
		if _, exists := prices[price.Model]; exists {
			return result, fmt.Errorf("%w: duplicate price for %q", ErrInvalidCampaignBudget, price.Model)
		}
		prices[price.Model] = price
		inputRate = max(inputRate, price.InputUSDPerMillion)
		outputRate = max(outputRate, price.OutputUSDPerMillion)
	}
	if len(spec.Plans) == 0 {
		return result, fmt.Errorf("%w: at least one plan is required", ErrInvalidCampaignBudget)
	}
	for _, item := range spec.Plans {
		if item.Plan == nil || strings.TrimSpace(item.Plan.Model) == "" || item.Plan.MaximumRequests <= 0 || item.Plan.MaximumRequests > CampaignBudgetMaxRequests || item.Plan.MaxRetries != 0 {
			return result, fmt.Errorf("%w: plan must have bounded requests and zero retries", ErrInvalidCampaignBudget)
		}

		if _, ok := prices[item.Plan.Model]; !ok {
			return result, fmt.Errorf("%w: no verified price for model %q", ErrInvalidCampaignBudget, item.Plan.Model)
		}
		if len(item.InputTokens) != item.Plan.MaximumRequests {
			return result, fmt.Errorf("%w: input token evidence count for %q is %d, want %d", ErrInvalidCampaignBudget, item.Plan.Model, len(item.InputTokens), item.Plan.MaximumRequests)
		}
		startBounds := len(result.inputBounds)
		for _, evidence := range item.InputTokens {
			if evidence.Tokens < 0 || evidence.Tokens > CampaignBudgetMaxInputTokens || evidence.Source != "count_tokens" && evidence.Source != "conservative_bound" {
				return result, fmt.Errorf("%w: every input token count must be known and sourced", ErrInvalidCampaignBudget)
			}
			if result.InputTokens > math.MaxInt64-evidence.Tokens {
				return result, fmt.Errorf("%w: input token count overflows", ErrInvalidCampaignBudget)
			}
			result.InputTokens += evidence.Tokens
			result.inputBounds = append(result.inputBounds, CampaignBudgetMaxInputTokens)
		}
		if result.RequestCount > CampaignBudgetMaxRequests-item.Plan.MaximumRequests {
			return result, fmt.Errorf("%w: request count exceeds %d", ErrInvalidCampaignBudget, CampaignBudgetMaxRequests)
		}
		result.RequestCount += item.Plan.MaximumRequests
		result.planBounds[item.Plan] = append([]int64(nil), result.inputBounds[startBounds:]...)
	}
	if result.RequestCount <= 0 {
		return result, fmt.Errorf("%w: no requests selected", ErrInvalidCampaignBudget)
	}
	if int64(result.RequestCount) > math.MaxInt64/spec.MaxOutputTokens {
		return result, fmt.Errorf("%w: output token reservation overflows", ErrInvalidCampaignBudget)
	}
	result.ReservedInputTokens = int64(result.RequestCount) * CampaignBudgetMaxInputTokens
	result.ReservedOutputTokens = int64(result.RequestCount) * spec.MaxOutputTokens
	result.InputUSDPerMillion, result.OutputUSDPerMillion = inputRate, outputRate
	result.ReservedInputUSD = float64(result.ReservedInputTokens) * inputRate / 1_000_000
	result.ReservedOutputUSD = float64(result.ReservedOutputTokens) * outputRate / 1_000_000
	result.ReservedTotalUSD = result.ReservedInputUSD + result.ReservedOutputUSD
	result.HardCeilingUSD = spec.HardCeilingUSD
	result.HeadroomUSD = spec.HardCeilingUSD - result.ReservedTotalUSD
	if !finite(result.ReservedTotalUSD) || result.ReservedTotalUSD > spec.HardCeilingUSD {
		return result, fmt.Errorf("%w: reserved upper bound %.6f exceeds ceiling %.6f", ErrCampaignBudgetExceeded, result.ReservedTotalUSD, spec.HardCeilingUSD)
	}
	return result, nil
}

// CampaignBudgetGuard enforces the same ceiling while requests execute. It is
// safe for a runner that records requests from multiple workers.
type CampaignBudgetGuard struct {
	mu       sync.Mutex
	estimate CampaignBudgetEstimate
	used     int
	spentUSD float64
	unknown  int
	planUsed map[*Plan]int
}

func NewCampaignBudgetGuard(estimate CampaignBudgetEstimate) (*CampaignBudgetGuard, error) {
	if estimate.RequestCount <= 0 || estimate.RequestCount > CampaignBudgetMaxRequests || estimate.HardCeilingUSD <= 0 || estimate.HeadroomUSD < 0 || !finite(estimate.ReservedTotalUSD) {
		return nil, fmt.Errorf("%w: invalid preflight estimate", ErrInvalidCampaignBudget)
	}
	if len(estimate.inputBounds) != estimate.RequestCount || len(estimate.planBounds) == 0 {
		return nil, fmt.Errorf("%w: estimate lacks per-request input bounds", ErrInvalidCampaignBudget)
	}
	return &CampaignBudgetGuard{estimate: estimate, planUsed: make(map[*Plan]int)}, nil
}

// BeforeRequest must be called immediately before each provider request. It
// reserves the worst-case input/output cost of the remaining scheduled slot.
func (g *CampaignBudgetGuard) BeforeRequest(plan *Plan) error {
	g.mu.Lock()
	defer g.mu.Unlock()
	if plan == nil || g.used >= g.estimate.RequestCount {
		return ErrCampaignBudgetExceeded
	}
	usedForPlan := g.planUsed[plan]
	bounds, ok := g.estimate.planBounds[plan]
	if !ok || usedForPlan >= len(bounds) {
		return fmt.Errorf("%w: plan request sequence exhausted", ErrCampaignBudgetExceeded)
	}
	var remainingInput int64
	for eachPlan, eachBounds := range g.estimate.planBounds {
		start := g.planUsed[eachPlan]
		if start > len(eachBounds) {
			return fmt.Errorf("%w: plan request sequence invalid", ErrCampaignBudgetExceeded)
		}
		for _, bound := range eachBounds[start:] {
			if remainingInput > math.MaxInt64-bound {
				return fmt.Errorf("%w: remaining input bound overflows", ErrCampaignBudgetExceeded)
			}
			remainingInput += bound
		}
	}
	remaining := g.estimate.RequestCount - g.used
	upper := g.spentUSD + float64(remainingInput)*g.estimate.InputUSDPerMillion/1_000_000 + float64(remaining)*float64(CampaignBudgetMaxOutputTokens)*g.estimate.OutputUSDPerMillion/1_000_000
	if !finite(upper) || upper > g.estimate.HardCeilingUSD {
		return fmt.Errorf("%w: remaining upper bound %.6f exceeds ceiling %.6f", ErrCampaignBudgetExceeded, upper, g.estimate.HardCeilingUSD)
	}
	return nil
}

// RecordObserved records provider usage after a request. Unknown usage is
// rejected: the guard cannot certify a hard ceiling without observed or
// preflight-bounded usage.
func (g *CampaignBudgetGuard) RecordObserved(plan *Plan, inputTokens, outputTokens int64) error {
	if inputTokens < 0 || outputTokens < 0 {
		return fmt.Errorf("%w: invalid observed usage", ErrInvalidCampaignBudget)
	}
	if inputTokens > CampaignBudgetMaxInputTokens || outputTokens > CampaignBudgetMaxOutputTokens {
		return ErrCampaignBudgetExceeded
	}
	g.mu.Lock()
	defer g.mu.Unlock()
	if plan == nil || g.used >= g.estimate.RequestCount || g.planUsed[plan] >= len(g.estimate.planBounds[plan]) {
		return ErrCampaignBudgetExceeded
	}
	cost := float64(inputTokens)*g.estimate.InputUSDPerMillion/1_000_000 + float64(outputTokens)*g.estimate.OutputUSDPerMillion/1_000_000
	if !finite(cost) || !finite(g.spentUSD+cost) || g.spentUSD+cost > g.estimate.HardCeilingUSD {
		return ErrCampaignBudgetExceeded
	}
	g.spentUSD += cost
	g.used++
	g.planUsed[plan]++
	return nil
}

// AbortUnknownUsage permanently exhausts the guard when a provider response
// omits usage metadata. Further generation is unsafe to certify.
func (g *CampaignBudgetGuard) AbortUnknownUsage() error {
	g.mu.Lock()
	defer g.mu.Unlock()
	g.used = g.estimate.RequestCount
	return ErrCampaignBudgetExceeded
}

// RecordUnknown consumes the complete per-request reservation when the
// provider omits usage metadata. The preflight proof guarantees that this
// reservation remains within the campaign ceiling, so a valid response is not
// converted into an execution failure merely because usage is unavailable.
func (g *CampaignBudgetGuard) RecordUnknown(plan *Plan) error {
	g.mu.Lock()
	defer g.mu.Unlock()
	if plan == nil || g.used >= g.estimate.RequestCount || g.planUsed[plan] >= len(g.estimate.planBounds[plan]) {
		return ErrCampaignBudgetExceeded
	}
	reserve := float64(CampaignBudgetMaxInputTokens)*g.estimate.InputUSDPerMillion/1_000_000 + float64(CampaignBudgetMaxOutputTokens)*g.estimate.OutputUSDPerMillion/1_000_000
	if !finite(reserve) || g.spentUSD+reserve > g.estimate.HardCeilingUSD {
		return ErrCampaignBudgetExceeded
	}
	g.spentUSD += reserve
	g.used++
	g.unknown++
	g.planUsed[plan]++
	return nil
}

func (g *CampaignBudgetGuard) Snapshot() (used int, spentUSD float64) {
	g.mu.Lock()
	defer g.mu.Unlock()
	return g.used, g.spentUSD
}

// PreflightGeminiCampaignBudget counts the exact prompts (including image
// parts) for every trial through each executor's CountTokens endpoint. The
// endpoint is used only for token counting; no generation request is made.
// A single shared guard is attached to all phase/model executors so the
// campaign-wide request ceiling cannot be bypassed by splitting plans.
func PreflightGeminiCampaignBudget(ctx context.Context, executors []*GeminiExecutor, prices []CampaignPrice, hardCeiling float64, pricingSource string) (CampaignBudgetEstimate, error) {
	if ctx == nil {
		return CampaignBudgetEstimate{}, fmt.Errorf("%w: context is required", ErrInvalidCampaignBudget)
	}
	if len(executors) == 0 {
		return CampaignBudgetEstimate{}, fmt.Errorf("%w: Gemini executors are required", ErrInvalidCampaignBudget)
	}
	plans := make([]CampaignBudgetPlan, 0, len(executors))
	maxOutput := int64(0)
	for _, executor := range executors {
		if executor == nil || executor.plan == nil || executor.dataset == nil {
			return CampaignBudgetEstimate{}, fmt.Errorf("%w: executor is not initialized", ErrInvalidCampaignBudget)
		}
		if executor.limits.MaxOutputTokens != CampaignBudgetMaxOutputTokens {
			return CampaignBudgetEstimate{}, fmt.Errorf("%w: executor output limit must be %d", ErrInvalidCampaignBudget, CampaignBudgetMaxOutputTokens)
		}
		if maxOutput == 0 {
			maxOutput = int64(executor.limits.MaxOutputTokens)
		}
		if int64(executor.limits.MaxOutputTokens) != maxOutput {
			return CampaignBudgetEstimate{}, fmt.Errorf("%w: executor output limits differ", ErrInvalidCampaignBudget)
		}
		tokens := make([]InputTokenEvidence, 0, executor.plan.MaximumRequests)
		for _, planned := range executor.plan.Cases {
			count, err := executor.CountInputTokens(ctx, planned.CaseID)
			if err != nil {
				return CampaignBudgetEstimate{}, fmt.Errorf("%w: count tokens for %s: %v", ErrInvalidCampaignBudget, planned.CaseID, err)
			}
			for trial := 1; trial <= executor.plan.Trials; trial++ {
				tokens = append(tokens, InputTokenEvidence{Tokens: count, Source: "count_tokens"})
			}
		}
		if len(tokens) != executor.plan.MaximumRequests {
			return CampaignBudgetEstimate{}, fmt.Errorf("%w: plan %q token count does not match maximum requests", ErrInvalidCampaignBudget, executor.plan.Model)
		}
		plans = append(plans, CampaignBudgetPlan{Plan: executor.plan, InputTokens: tokens})
	}
	estimate, err := PreflightCampaignBudget(CampaignBudgetSpec{Plans: plans, Prices: prices, HardCeilingUSD: hardCeiling, MaxOutputTokens: maxOutput, PricingSource: pricingSource})
	if err != nil {
		return estimate, err
	}
	guard, err := NewCampaignBudgetGuard(estimate)
	if err != nil {
		return estimate, err
	}
	for _, executor := range executors {
		executor.budget = guard
	}
	return estimate, nil
}

func finite(value float64) bool { return !math.IsNaN(value) && !math.IsInf(value, 0) }
