package provider_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/LoResuelvo/loresuelvo-api/internal/domain/provider"
	"github.com/LoResuelvo/loresuelvo-api/internal/domain/provider/read_model"
	"github.com/LoResuelvo/loresuelvo-api/internal/domain/user"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

var activityNow = time.Date(2026, 9, 29, 15, 0, 0, 0, time.UTC)

func TestActivityServiceRejectsInvalidPeriodsBeforeReading(t *testing.T) {
	before := activityNow.Add(-time.Hour)
	after := activityNow.Add(time.Hour)
	tooOld := activityNow.Add(-366 * 24 * time.Hour)
	cases := []struct {
		name  string
		input provider.ActivityQueryInput
	}{
		{"missing to", provider.ActivityQueryInput{From: &before}},
		{"missing from", provider.ActivityQueryInput{To: &before}},
		{"equal bounds", provider.ActivityQueryInput{From: &before, To: &before}},
		{"reversed bounds", provider.ActivityQueryInput{From: &after, To: &before}},
		{"future to", provider.ActivityQueryInput{From: &before, To: &after}},
		{"too long", provider.ActivityQueryInput{From: &tooOld, To: &activityNow}},
		{"bad granularity", provider.ActivityQueryInput{Granularity: "hour"}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			reader := &activityReaderMock{}
			actors := &providerActorFinderMock{}
			clock := &activityClockMock{}
			clock.On("Now").Return(activityNow).Once()
			result, err := provider.NewActivityService(reader, actors, clock).Query(t.Context(), "subject", tc.input)
			require.Nil(t, result)
			require.ErrorIs(t, err, provider.ErrInvalidActivityQuery)
			reader.AssertNotCalled(t, "Read", mock.Anything, mock.Anything, mock.Anything)
			actors.AssertNotCalled(t, "FindByAuthID", mock.Anything, mock.Anything)
			clock.AssertExpectations(t)
		})
	}
}

func TestActivityServiceSeparatesMissingAndForbiddenActors(t *testing.T) {
	cases := []struct {
		name      string
		id        int
		role      string
		finderErr error
		want      error
	}{
		{"missing user", 0, "", user.ErrNotFound, provider.ErrActivityProviderNotFound},
		{"consumer", 12, "consumer", nil, provider.ErrActivityForbidden},
		{"provider missing subtype", 0, provider.Role, nil, provider.ErrActivityProviderNotFound},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			reader := &activityReaderMock{}
			actors := &providerActorFinderMock{}
			actors.On("FindByAuthID", mock.Anything, "subject").Return(tc.id, tc.role, tc.finderErr).Once()
			clock := &activityClockMock{}
			clock.On("Now").Return(activityNow).Once()
			result, err := provider.NewActivityService(reader, actors, clock).Query(t.Context(), "subject", provider.ActivityQueryInput{})
			require.Nil(t, result)
			require.ErrorIs(t, err, tc.want)
			reader.AssertNotCalled(t, "Read", mock.Anything, mock.Anything, mock.Anything)
			actors.AssertExpectations(t)
		})
	}
}

func TestActivityServiceUsesOneClockReadingAndDefaultRange(t *testing.T) {
	reader := &activityReaderMock{}
	actors := &providerActorFinderMock{}
	clock := &activityClockMock{}
	clock.On("Now").Return(activityNow).Once()
	actors.On("FindByAuthID", mock.Anything, "subject").Return(42, provider.Role, nil).Once()
	reader.On("Read", mock.Anything, 42, mock.MatchedBy(func(q provider.ActivityQuery) bool {
		return q.From.Equal(activityNow.Add(-30*24*time.Hour)) && q.To.Equal(activityNow) && q.Granularity == provider.ActivityDay && !q.ComparePrevious
	})).Return(&readmodel.ActivitySnapshot{}, nil).Once()
	result, err := provider.NewActivityService(reader, actors, clock).Query(t.Context(), "subject", provider.ActivityQueryInput{})
	require.NoError(t, err)
	require.Equal(t, activityNow, result.CalculatedAt)
	require.Equal(t, "America/Argentina/Buenos_Aires", result.TimeZone)
	require.Nil(t, result.Results.AverageCents)
	require.Nil(t, result.Comparison)
	require.Len(t, result.Series, 31)
	reader.AssertExpectations(t)
	actors.AssertExpectations(t)
	clock.AssertExpectations(t)
}

func TestActivityServiceClipsWeeklyBucketsAndPreservesSparseCounts(t *testing.T) {
	from := time.Date(2026, 9, 6, 15, 0, 0, 0, time.UTC)
	to := time.Date(2026, 9, 8, 15, 0, 0, 0, time.UTC)
	start := time.Date(2026, 9, 7, 3, 0, 0, 0, time.UTC)
	reader := &activityReaderMock{}
	actors := &providerActorFinderMock{}
	clock := &activityClockMock{}
	clock.On("Now").Return(activityNow).Once()
	actors.On("FindByAuthID", mock.Anything, "subject").Return(42, provider.Role, nil).Once()
	reader.On("Read", mock.Anything, 42, mock.Anything).Return(&readmodel.ActivitySnapshot{
		Current: readmodel.ActivityCounts{Reported: 1},
		Series:  []readmodel.ActivityBucket{{From: start, Reported: 1}},
	}, nil).Once()
	result, err := provider.NewActivityService(reader, actors, clock).Query(t.Context(), "subject", provider.ActivityQueryInput{From: &from, To: &to, Granularity: "week"})
	require.NoError(t, err)
	require.Equal(t, []readmodel.ActivityBucket{
		{From: from, To: start},
		{From: start, To: to, Reported: 1},
	}, result.Series)
}

func TestActivityServiceRoundsMoneyAndComparison(t *testing.T) {
	from := activityNow.Add(-48 * time.Hour)
	to := activityNow
	reader := &activityReaderMock{}
	actors := &providerActorFinderMock{}
	clock := &activityClockMock{}
	clock.On("Now").Return(activityNow).Once()
	actors.On("FindByAuthID", mock.Anything, "subject").Return(42, provider.Role, nil).Once()
	reader.On("Read", mock.Anything, 42, mock.MatchedBy(func(q provider.ActivityQuery) bool {
		return q.PreviousPeriod().From.Equal(from.Add(-48*time.Hour)) && q.PreviousPeriod().To.Equal(from)
	})).Return(&readmodel.ActivitySnapshot{
		Current:  readmodel.ActivityCounts{Confirmed: 2, Reported: 2, Paid: 2, Customers: 2, NewCustomers: 1, ReturningCustomers: 1, ContractValueCents: 30003},
		Previous: &readmodel.ActivityCounts{Confirmed: 1, Reported: 1, Paid: 1, Customers: 1, NewCustomers: 1, ContractValueCents: 10001},
		Series:   []readmodel.ActivityBucket{{From: time.Date(2026, 9, 27, 3, 0, 0, 0, time.UTC), Confirmed: 2, Reported: 2, Paid: 2}},
	}, nil).Once()
	result, err := provider.NewActivityService(reader, actors, clock).Query(t.Context(), "subject", provider.ActivityQueryInput{From: &from, To: &to, ComparePrevious: true})
	require.NoError(t, err)
	require.Equal(t, int64(15002), *result.Results.AverageCents)
	require.Equal(t, int64(10001), *result.Comparison.Results.AverageCents)
	require.Equal(t, int64(5001), *result.Comparison.Change.AverageCents)
	require.Equal(t, float64(100), *result.Comparison.PercentChange.Confirmed)
	require.Equal(t, float64(50.00), *result.Comparison.PercentChange.AverageCents)
	require.Nil(t, result.Comparison.PercentChange.ReturningCustomers)
}

func TestActivityServiceDoesNotInventPreviousAverage(t *testing.T) {
	reader := &activityReaderMock{}
	actors := &providerActorFinderMock{}
	clock := &activityClockMock{}
	clock.On("Now").Return(activityNow).Once()
	actors.On("FindByAuthID", mock.Anything, "subject").Return(42, provider.Role, nil).Once()
	reader.On("Read", mock.Anything, 42, mock.Anything).Return(&readmodel.ActivitySnapshot{
		Current:  readmodel.ActivityCounts{Reported: 1, ContractValueCents: 10001},
		Previous: &readmodel.ActivityCounts{},
		Series:   []readmodel.ActivityBucket{{From: time.Date(2026, 8, 30, 3, 0, 0, 0, time.UTC), Reported: 1}},
	}, nil).Once()
	result, err := provider.NewActivityService(reader, actors, clock).Query(t.Context(), "subject", provider.ActivityQueryInput{ComparePrevious: true})
	require.NoError(t, err)
	require.Nil(t, result.Comparison.Results.AverageCents)
	require.Nil(t, result.Comparison.Change.AverageCents)
	require.Nil(t, result.Comparison.PercentChange.AverageCents)
	require.Nil(t, result.Comparison.PercentChange.Reported)
}

func TestActivityServicePropagatesReadError(t *testing.T) {
	reader := &activityReaderMock{}
	actors := &providerActorFinderMock{}
	clock := &activityClockMock{}
	clock.On("Now").Return(activityNow).Once()
	actors.On("FindByAuthID", mock.Anything, "subject").Return(42, provider.Role, nil).Once()
	readErr := errors.New("read failed")
	reader.On("Read", mock.Anything, 42, mock.Anything).Return(nil, readErr).Once()
	result, err := provider.NewActivityService(reader, actors, clock).Query(context.Background(), "subject", provider.ActivityQueryInput{})
	require.Nil(t, result)
	require.ErrorIs(t, err, readErr)
}
