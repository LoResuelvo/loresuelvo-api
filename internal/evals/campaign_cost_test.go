package evals

import (
	"encoding/json"
	"github.com/stretchr/testify/require"
	"testing"
)

func TestCampaignCostUsesModelPriceAndDoesNotDoubleCountCacheOrThought(t *testing.T) {
	cache := .03
	prices := []CampaignPrice{{Model: "a", InputUSDPerMillion: .3, OutputUSDPerMillion: 2.5, CacheUSDPerMillion: &cache, Source: "pricing", VerifiedOn: "2026-10-13", Basis: "standard_paid", Currency: "USD", Verified: true}, {Model: "b", InputUSDPerMillion: 10, OutputUSDPerMillion: 20, Source: "pricing", VerifiedOn: "2026-10-13", Basis: "standard_paid", Currency: "USD", Verified: true}}
	attempts := []Attempt{{RequestCount: 1, Status: "execution_error", ProviderUsage: json.RawMessage(`{"promptTokenCount":100,"cachedContentTokenCount":20,"candidatesTokenCount":5,"thoughtsTokenCount":7,"totalTokenCount":112}`)}}
	cost := estimateCampaignCost(attempts, prices, "a")
	require.NotNil(t, cost.EstimatedUSD)
	require.InDelta(t, (80*.3+20*.03+12*2.5)/1_000_000, *cost.EstimatedUSD, 1e-12)
	require.Nil(t, cost.BilledUSD)
	require.Equal(t, 1, cost.KnownRequests)
	prices[0].Basis = ""
	unknown := estimateCampaignCost(attempts, prices, "a")
	require.Nil(t, unknown.EstimatedUSD)
	require.Equal(t, 1, unknown.UnknownRequests)
	prices[0].Basis = "standard_paid"
	attempts[0].ProviderUsage = json.RawMessage(`{"promptTokenCount":100,"candidatesTokenCount":5,"totalTokenCount":105}`)
	unknown = estimateCampaignCost(attempts, prices, "a")
	require.Nil(t, unknown.EstimatedUSD)
	require.Equal(t, 1, unknown.UnknownRequests)
}
