package provider_test

import (
	"testing"

	"github.com/LoResuelvo/loresuelvo-api/internal/domain/provider"
	"github.com/stretchr/testify/assert"
)

func TestRatingStatsReturnsZeroSummaryWithoutRatings(t *testing.T) {
	summary := provider.RatingStats{}.Summary()

	assert.Equal(t, provider.RatingSummary{}, summary)
}

func TestRatingStatsRoundsAverageToOneDecimal(t *testing.T) {
	summary := provider.RatingStats{Total: 14, Count: 3}.Summary()

	assert.Equal(t, provider.RatingSummary{Average: 4.7, Count: 3}, summary)
}
