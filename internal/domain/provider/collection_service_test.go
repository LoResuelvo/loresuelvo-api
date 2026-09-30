package provider_test

import (
	"errors"
	"math"
	"testing"
	"time"

	"github.com/LoResuelvo/loresuelvo-api/internal/domain/provider"
	readmodel "github.com/LoResuelvo/loresuelvo-api/internal/domain/provider/read_model"
	"github.com/LoResuelvo/loresuelvo-api/internal/domain/user"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

func TestCollectionSummaryBuildsClippedSeriesAndComparison(t *testing.T) {
	now := time.Date(2026, 9, 29, 15, 0, 0, 0, time.UTC)
	from := time.Date(2026, 9, 6, 15, 0, 0, 0, time.UTC)
	to := time.Date(2026, 9, 8, 15, 0, 0, 0, time.UTC)
	previous := &readmodel.CollectionAmounts{BookingDepositCents: 10000, ServiceBalanceCents: 30000}
	reader := &collectionReaderMock{}
	actors := &providerActorFinderMock{}
	clock := &activityClockMock{}
	clock.On("Now").Return(now).Once()
	actors.On("FindByAuthID", mock.Anything, "provider-auth").Return(7, provider.Role, nil).Once()
	reader.On("ReadSummary", mock.Anything, 7, mock.MatchedBy(func(q provider.ActivityQuery) bool {
		return q.From.Equal(from) && q.To.Equal(to) && q.Granularity == provider.ActivityWeek && q.ComparePrevious
	})).Return(&readmodel.CollectionSnapshot{
		Current:  readmodel.CollectionAmounts{BookingDepositCents: 20000, ServiceBalanceCents: 60000},
		Previous: previous,
		Series: []readmodel.CollectionBucket{
			{From: time.Date(2026, 8, 31, 3, 0, 0, 0, time.UTC), CollectionAmounts: readmodel.CollectionAmounts{BookingDepositCents: 20000}},
			{From: time.Date(2026, 9, 7, 3, 0, 0, 0, time.UTC), CollectionAmounts: readmodel.CollectionAmounts{ServiceBalanceCents: 60000}},
		},
		Pending: readmodel.CollectionPending{Scheduled: readmodel.PendingBalance{Orders: 1, AmountCents: 80000}},
	}, nil).Once()
	result, err := provider.NewCollectionService(reader, actors, clock).Summary(t.Context(), "provider-auth", provider.ActivityQueryInput{From: &from, To: &to, Granularity: "week", ComparePrevious: true})
	require.NoError(t, err)
	require.Equal(t, int64(80000), result.Results.TotalCents)
	require.Equal(t, int64(80000), result.Pending.Scheduled.AmountCents)
	require.Len(t, result.Series, 2)
	require.Equal(t, from, result.Series[0].From)
	require.Equal(t, time.Date(2026, 9, 7, 3, 0, 0, 0, time.UTC), result.Series[0].To)
	require.Equal(t, to, result.Series[1].To)
	require.Equal(t, int64(40000), result.Comparison.Results.TotalCents)
	require.Equal(t, int64(40000), result.Comparison.Changes.Absolute.TotalCents)
	require.Equal(t, float64(100), *result.Comparison.Changes.Percentage.TotalCents)
	reader.AssertExpectations(t)
	actors.AssertExpectations(t)
	clock.AssertExpectations(t)
}

func TestCollectionSummaryRejectsInvalidAndIncompleteData(t *testing.T) {
	now := time.Date(2026, 9, 29, 15, 0, 0, 0, time.UTC)
	from, to := now.Add(-24*time.Hour), now
	for name, snapshot := range map[string]*readmodel.CollectionSnapshot{
		"missing comparison": {Current: readmodel.CollectionAmounts{}, Series: []readmodel.CollectionBucket{}},
		"overflow":           {Current: readmodel.CollectionAmounts{BookingDepositCents: math.MaxInt64, ServiceBalanceCents: 1}, Previous: &readmodel.CollectionAmounts{}},
		"series mismatch":    {Current: readmodel.CollectionAmounts{BookingDepositCents: 100}, Previous: &readmodel.CollectionAmounts{}},
	} {
		t.Run(name, func(t *testing.T) {
			reader := &collectionReaderMock{}
			actors := &providerActorFinderMock{}
			clock := &activityClockMock{}
			clock.On("Now").Return(now).Once()
			actors.On("FindByAuthID", mock.Anything, "provider-auth").Return(7, provider.Role, nil).Once()
			reader.On("ReadSummary", mock.Anything, 7, mock.Anything).Return(snapshot, nil).Once()
			_, err := provider.NewCollectionService(reader, actors, clock).Summary(t.Context(), "provider-auth", provider.ActivityQueryInput{From: &from, To: &to, ComparePrevious: true})
			require.Error(t, err)
		})
	}
	clock := &activityClockMock{}
	clock.On("Now").Return(now).Once()
	actors := &providerActorFinderMock{}
	reader := &collectionReaderMock{}
	_, err := provider.NewCollectionService(reader, actors, clock).Summary(t.Context(), "provider-auth", provider.ActivityQueryInput{From: &from})
	require.ErrorIs(t, err, provider.ErrInvalidCollectionQuery)
	actors.AssertNotCalled(t, "FindByAuthID", mock.Anything, mock.Anything)
}

func TestCollectionDetailValidatesActorAndCursorOwner(t *testing.T) {
	now := time.Date(2026, 9, 29, 15, 0, 0, 0, time.UTC)
	from, to := now.Add(-time.Hour), now
	for _, test := range []struct {
		name string
		id   int
		role string
		err  error
		want error
	}{
		{"consumer", 9, "consumer", nil, provider.ErrActivityForbidden},
		{"missing", 0, "", user.ErrNotFound, provider.ErrActivityProviderNotFound},
		{"other provider cursor", 9, provider.Role, nil, provider.ErrInvalidCollectionQuery},
	} {
		t.Run(test.name, func(t *testing.T) {
			clock := &activityClockMock{}
			clock.On("Now").Return(now).Once()
			actors := &providerActorFinderMock{}
			actors.On("FindByAuthID", mock.Anything, "actor").Return(test.id, test.role, test.err).Once()
			reader := &collectionReaderMock{}
			_, _, err := provider.NewCollectionService(reader, actors, clock).Detail(t.Context(), "actor", provider.CollectionDetailInput{
				Period: provider.ActivityQueryInput{From: &from, To: &to}, Limit: 1,
				After: &readmodel.CollectionPosition{VerifiedOn: from.Add(time.Minute), ID: 5}, CursorProviderID: 7,
			})
			require.ErrorIs(t, err, test.want)
			reader.AssertNotCalled(t, "ReadDetail", mock.Anything, mock.Anything, mock.Anything)
		})
	}
}

func TestCollectionDetailPreservesWholeFilterTotals(t *testing.T) {
	now := time.Date(2026, 9, 29, 15, 0, 0, 0, time.UTC)
	from, to := now.Add(-time.Hour), now
	clock := &activityClockMock{}
	clock.On("Now").Return(now).Once()
	actors := &providerActorFinderMock{}
	actors.On("FindByAuthID", mock.Anything, "actor").Return(7, provider.Role, nil).Once()
	reader := &collectionReaderMock{}
	reader.On("ReadDetail", mock.Anything, 7, mock.MatchedBy(func(q provider.CollectionDetailQuery) bool {
		return q.Limit == 1 && q.Purpose == provider.CollectionBookingDeposit && q.Period.From.Equal(from)
	})).Return(&readmodel.CollectionDetailSnapshot{
		TotalCount: 2, TotalAmountCents: 20000,
		Page: []readmodel.CollectionTransaction{{ID: 9, VerifiedOn: from.Add(time.Minute), Purpose: "booking_deposit", SellerAmountCents: 10000, Currency: "ARS", ServiceProposalID: 3}},
		Next: &readmodel.CollectionPosition{VerifiedOn: from.Add(time.Minute), ID: 9},
	}, nil).Once()
	result, providerID, err := provider.NewCollectionService(reader, actors, clock).Detail(t.Context(), "actor", provider.CollectionDetailInput{Period: provider.ActivityQueryInput{From: &from, To: &to}, Purpose: "booking_deposit", Limit: 1})
	require.NoError(t, err)
	require.Equal(t, 7, providerID)
	require.Equal(t, int64(2), result.TotalCount)
	require.Equal(t, int64(20000), result.TotalAmountCents)
	require.Len(t, result.Transactions, 1)
	require.NotNil(t, result.Next)
}

func TestCollectionSummaryPropagatesReadFailure(t *testing.T) {
	now := time.Date(2026, 9, 29, 15, 0, 0, 0, time.UTC)
	clock := &activityClockMock{}
	clock.On("Now").Return(now).Once()
	actors := &providerActorFinderMock{}
	actors.On("FindByAuthID", mock.Anything, "actor").Return(7, provider.Role, nil).Once()
	reader := &collectionReaderMock{}
	failure := errors.New("database unavailable")
	reader.On("ReadSummary", mock.Anything, 7, mock.Anything).Return(nil, failure).Once()
	_, err := provider.NewCollectionService(reader, actors, clock).Summary(t.Context(), "actor", provider.ActivityQueryInput{})
	require.ErrorIs(t, err, failure)
}
