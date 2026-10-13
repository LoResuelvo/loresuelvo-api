package evals

import (
	"time"
)

// CampaignCostEstimate is a token-based quote, never a provider invoice or a
// conservative budget reservation. Coverage counts physical provider calls.
type CampaignCostEstimate struct {
	EstimatedUSD    *float64        `json:"estimated_usd"`
	BilledUSD       *float64        `json:"billed_usd"`
	KnownRequests   int             `json:"known_requests"`
	UnknownRequests int             `json:"unknown_requests"`
	Reason          string          `json:"reason"`
	Prices          []CampaignPrice `json:"prices"`
}

func estimateCampaignCost(attempts []Attempt, prices []CampaignPrice, model string) CampaignCostEstimate {
	result := CampaignCostEstimate{Reason: "pricing_or_usage_incomplete", Prices: prices}
	var price *CampaignPrice
	for i := range prices {
		if prices[i].Model == model {
			if price != nil {
				price = nil
				break
			}
			price = &prices[i]
		}
	}
	valid := price != nil
	if valid {
		_, dateErr := time.Parse(time.DateOnly, price.VerifiedOn)
		valid = price.Verified && price.Source != "" && price.Basis != "" && price.Basis != "unknown" && price.Currency == "USD" && dateErr == nil && finite(price.InputUSDPerMillion) && price.InputUSDPerMillion >= 0 && finite(price.OutputUSDPerMillion) && price.OutputUSDPerMillion >= 0
	}
	var total float64
	for _, attempt := range attempts {
		if attempt.RequestCount <= 0 {
			continue
		}
		usage := campaignExportAttemptUsage(attempt)
		if !valid || attempt.RequestCount != 1 || usage.InputTokens == nil || usage.OutputTokens == nil || usage.ThinkingTokens == nil || usage.CachedInputTokens == nil || *usage.InputTokens < 0 || *usage.OutputTokens < 0 || *usage.ThinkingTokens < 0 || *usage.CachedInputTokens < 0 || *usage.CachedInputTokens > *usage.InputTokens {
			result.UnknownRequests += attempt.RequestCount
			continue
		}
		cached := *usage.CachedInputTokens
		if cached > 0 && (price.CacheUSDPerMillion == nil || !finite(*price.CacheUSDPerMillion) || *price.CacheUSDPerMillion < 0) {
			result.UnknownRequests++
			continue
		}
		estimate := float64(*usage.InputTokens-cached)*price.InputUSDPerMillion/1_000_000 + (float64(*usage.OutputTokens)+float64(*usage.ThinkingTokens))*price.OutputUSDPerMillion/1_000_000
		if cached > 0 {
			estimate += float64(cached) * *price.CacheUSDPerMillion / 1_000_000
		}
		if !finite(estimate) || estimate < 0 {
			result.UnknownRequests++
			continue
		}
		total += estimate
		result.KnownRequests++
	}
	if result.UnknownRequests == 0 && result.KnownRequests > 0 && finite(total) {
		result.EstimatedUSD = &total
		result.Reason = "observed_tokens_at_declared_verified_rates_not_billed_cost"
	}
	return result
}
