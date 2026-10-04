package readmodel

import (
	"github.com/stretchr/testify/require"
	"math"
	"testing"
	"time"
)

func TestConversionCalculateRatios(t *testing.T) {
	for _, tc := range []struct {
		name                 string
		stages               ConversionStages
		requests             ConversionRequestCounts
		contracted, accepted *float64
	}{
		{"half up", ConversionStages{Issued: 32, Contracted: 1}, ConversionRequestCounts{Received: 3, Accepted: 2, Pending: 1}, ptr(3.13), ptr(66.67)},
		{"max integer", ConversionStages{Issued: math.MaxInt64, Contracted: math.MaxInt64, Reported: math.MaxInt64, Paid: math.MaxInt64}, ConversionRequestCounts{Received: math.MaxInt64, Accepted: math.MaxInt64}, ptr(100), ptr(100)},
		{"zero numerator", ConversionStages{Issued: 3}, ConversionRequestCounts{Received: 1, Pending: 1}, ptr(0), ptr(0)},
		{"empty", ConversionStages{}, ConversionRequestCounts{}, nil, nil},
	} {
		t.Run(tc.name, func(t *testing.T) {
			now := time.Now().UTC()
			result, err := (ConversionSnapshot{Stages: tc.stages, Requests: tc.requests}).Calculate(ConversionPeriod{From: now.Add(-time.Hour), To: now}, now)
			require.NoError(t, err)
			require.Equal(t, tc.contracted, result.Proposals.Rates.Contracted.Cohort.Percentage)
			require.Equal(t, tc.accepted, result.Requests.AcceptanceRate.Percentage)
			require.Equal(t, tc.stages.Issued-tc.stages.Contracted, result.Proposals.Uncontracted)
			require.Equal(t, "America/Argentina/Buenos_Aires", result.TimeZone)
		})
	}
}
func ptr(v float64) *float64 { return &v }
func TestConversionCalculateNestedRates(t *testing.T) {
	result, err := (ConversionSnapshot{Stages: ConversionStages{8, 4, 2, 1}}).Calculate(ConversionPeriod{}, time.Time{})
	require.NoError(t, err)
	require.Equal(t, ConversionRatio{1, 8, ptr(12.5)}, result.Proposals.Rates.Paid.Cohort)
	require.Equal(t, ConversionRatio{1, 2, ptr(50)}, result.Proposals.Rates.Paid.PreviousStage)
	require.Equal(t, ConversionRatio{2, 4, ptr(50)}, result.Proposals.Rates.Reported.PreviousStage)
}
func TestConversionCalculateRejectsInvalidCounts(t *testing.T) {
	for _, snapshot := range []ConversionSnapshot{
		{Stages: ConversionStages{Issued: -1}}, {Stages: ConversionStages{Issued: 1, Contracted: 2}}, {Stages: ConversionStages{Issued: 2, Contracted: 1, Reported: 2}}, {Stages: ConversionStages{Issued: 2, Contracted: 2, Reported: 1, Paid: 2}}, {Stages: ConversionStages{Issued: 2, Contracted: -1}}, {Requests: ConversionRequestCounts{Received: 1, Accepted: 1, Pending: 1}}, {Requests: ConversionRequestCounts{Received: math.MaxInt64, Accepted: math.MaxInt64, Pending: math.MaxInt64}}, {Requests: ConversionRequestCounts{Received: 1, Accepted: -1, Pending: 2}}, {Requests: ConversionRequestCounts{Received: 1}},
	} {
		result, err := snapshot.Calculate(ConversionPeriod{}, time.Time{})
		require.Error(t, err)
		require.Nil(t, result)
	}
}

func TestConversionCalculateZeroDenominatorsAreAbsent(t *testing.T) {
	result, err := (ConversionSnapshot{Stages: ConversionStages{Issued: 3}}).Calculate(ConversionPeriod{}, time.Time{})
	require.NoError(t, err)
	require.Equal(t, ptr(0), result.Proposals.Rates.Reported.Cohort.Percentage)
	require.Equal(t, ptr(0), result.Proposals.Rates.Paid.Cohort.Percentage)
	require.Nil(t, result.Proposals.Rates.Reported.PreviousStage.Percentage)
	require.Nil(t, result.Proposals.Rates.Paid.PreviousStage.Percentage)
	require.Nil(t, result.Requests.AcceptanceRate.Percentage)
}
func TestConversionCalculateRoundsNearMaxIntegerExactly(t *testing.T) {
	result, err := (ConversionSnapshot{Stages: ConversionStages{Issued: math.MaxInt64, Contracted: math.MaxInt64 / 2}, Requests: ConversionRequestCounts{Received: math.MaxInt64, Accepted: 1, Pending: math.MaxInt64 - 1}}).Calculate(ConversionPeriod{}, time.Time{})
	require.NoError(t, err)
	require.Equal(t, ptr(50), result.Proposals.Rates.Contracted.Cohort.Percentage)
	require.Equal(t, ptr(0), result.Requests.AcceptanceRate.Percentage)
}
