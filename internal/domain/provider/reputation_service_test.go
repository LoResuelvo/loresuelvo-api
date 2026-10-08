package provider_test

import (
	"context"
	"errors"
	"math"
	"strconv"
	"testing"
	"time"

	"github.com/LoResuelvo/loresuelvo-api/internal/domain/provider"
	readmodel "github.com/LoResuelvo/loresuelvo-api/internal/domain/provider/read_model"
	"github.com/LoResuelvo/loresuelvo-api/internal/domain/user"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

func TestReputationComputesGlobalWeightedRatingAndCoverage(t *testing.T) {
	now := time.Date(2026, 10, 3, 15, 0, 0, 0, time.FixedZone("offset", -3*3600))
	reader, actors, clock := &reputationReaderMock{}, &providerActorFinderMock{}, &activityClockMock{}
	clock.On("Now").Return(now).Once()
	actors.On("FindByAuthID", t.Context(), "actor").Return(7, provider.Role, nil).Once()
	reader.On("Read", t.Context(), 7, provider.ReputationQuery{Limit: 1}).Return(&readmodel.ReputationSnapshot{
		EligiblePaidOrders: 6, ReviewedPaidOrders: 4, VisibleReviews: 4, RatingDistribution: [5]int64{1, 0, 0, 1, 2},
		Reviews: []readmodel.ReputationReview{{WorkOrderID: 9, Rating: 5, Description: ""}}, Next: &readmodel.ReputationPosition{WorkOrderID: 9},
	}, nil).Once()
	result, id, err := provider.NewReputationService(reader, actors, clock).Query(t.Context(), "actor", provider.ReputationQueryInput{Limit: 1})
	require.NoError(t, err)
	require.Equal(t, 7, id)
	require.Equal(t, now.UTC(), result.CalculatedAt)
	require.Equal(t, 3.75, *result.AverageRating)
	require.Equal(t, 66.67, *result.CoveragePercentage)
	require.Equal(t, int64(4), result.ReviewedPaidOrders)
	require.Equal(t, "", result.Reviews[0].Description)
	reader.AssertExpectations(t)
	actors.AssertExpectations(t)
	clock.AssertExpectations(t)
}

func TestReputationRoundsHalfUpWithoutWeightedOverflow(t *testing.T) {
	for _, test := range []struct {
		name               string
		eligible, reviewed int64
		distribution       [5]int64
		average, coverage  float64
	}{
		{"exact average half", 200, 200, [5]int64{199, 1}, 1.01, 100},
		{"exact percentage half", 32, 1, [5]int64{1}, 1, 3.13},
		{"weighted sum exceeds int64", math.MaxInt64, math.MaxInt64, [5]int64{0, 0, 0, 0, math.MaxInt64}, 5, 100},
	} {
		t.Run(test.name, func(t *testing.T) {
			reader, actors, clock := &reputationReaderMock{}, &providerActorFinderMock{}, &activityClockMock{}
			clock.On("Now").Return(time.Now()).Once()
			actors.On("FindByAuthID", mock.Anything, "actor").Return(7, provider.Role, nil).Once()
			rating := 1
			if test.distribution[4] > 0 {
				rating = 5
			}
			snapshot := &readmodel.ReputationSnapshot{EligiblePaidOrders: test.eligible, ReviewedPaidOrders: test.reviewed, VisibleReviews: test.reviewed, RatingDistribution: test.distribution, Reviews: []readmodel.ReputationReview{{WorkOrderID: 9, Rating: rating}}}
			if test.reviewed > 1 {
				snapshot.Next = &readmodel.ReputationPosition{WorkOrderID: 9}
			}
			reader.On("Read", mock.Anything, 7, mock.Anything).Return(snapshot, nil).Once()
			result, _, err := provider.NewReputationService(reader, actors, clock).Query(t.Context(), "actor", provider.ReputationQueryInput{Limit: 1})
			require.NoError(t, err)
			require.Equal(t, test.average, *result.AverageRating)
			require.Equal(t, test.coverage, *result.CoveragePercentage)
			reader.AssertExpectations(t)
			actors.AssertExpectations(t)
			clock.AssertExpectations(t)
		})
	}
}

func TestReputationNullAverageAndUndefinedCoverage(t *testing.T) {
	for _, eligible := range []int64{0, 3} {
		t.Run(strconv.FormatInt(eligible, 10), func(t *testing.T) {
			reader, actors, clock := &reputationReaderMock{}, &providerActorFinderMock{}, &activityClockMock{}
			clock.On("Now").Return(time.Now()).Once()
			actors.On("FindByAuthID", mock.Anything, "actor").Return(7, provider.Role, nil).Once()
			reader.On("Read", mock.Anything, 7, provider.ReputationQuery{Limit: 20}).Return(&readmodel.ReputationSnapshot{EligiblePaidOrders: eligible}, nil).Once()
			result, _, err := provider.NewReputationService(reader, actors, clock).Query(t.Context(), "actor", provider.ReputationQueryInput{})
			require.NoError(t, err)
			require.Nil(t, result.AverageRating)
			if eligible == 0 {
				require.Nil(t, result.CoveragePercentage)
			} else {
				require.Equal(t, float64(0), *result.CoveragePercentage)
			}
			require.NotNil(t, result.Reviews)
			reader.AssertExpectations(t)
		})
	}
}

func TestReputationRejectsInvalidInputBeforePersistence(t *testing.T) {
	for name, input := range map[string]provider.ReputationQueryInput{
		"negative limit": {Limit: -1}, "oversized limit": {Limit: 101},
		"negative owner": {CursorProviderID: -1}, "owner without position": {CursorProviderID: 7},
		"position without owner": {After: &readmodel.ReputationPosition{WorkOrderID: 1}},
		"zero position":          {After: &readmodel.ReputationPosition{}, CursorProviderID: 7},
	} {
		t.Run(name, func(t *testing.T) {
			reader, actors, clock := &reputationReaderMock{}, &providerActorFinderMock{}, &activityClockMock{}
			clock.On("Now").Return(time.Now()).Maybe()
			_, _, err := provider.NewReputationService(reader, actors, clock).Query(t.Context(), "actor", input)
			require.ErrorIs(t, err, provider.ErrInvalidReputationQuery)
			actors.AssertNotCalled(t, "FindByAuthID", mock.Anything, mock.Anything)
			reader.AssertNotCalled(t, "Read", mock.Anything, mock.Anything, mock.Anything)
		})
	}
}

func TestReputationResolvesOnlyProviderAndOwnCursor(t *testing.T) {
	failure := errors.New("actor database unavailable")
	for _, test := range []struct {
		name      string
		id        int
		role      string
		err, want error
	}{
		{"consumer", 9, "consumer", nil, provider.ErrReputationForbidden},
		{"admin", 9, "admin", nil, provider.ErrReputationForbidden},
		{"missing user", 0, "", user.ErrNotFound, provider.ErrReputationProviderNotFound},
		{"missing provider", 0, provider.Role, nil, provider.ErrReputationProviderNotFound},
		{"foreign cursor", 9, provider.Role, nil, provider.ErrInvalidReputationQuery},
		{"actor failure", 0, "", failure, failure},
	} {
		t.Run(test.name, func(t *testing.T) {
			reader, actors, clock := &reputationReaderMock{}, &providerActorFinderMock{}, &activityClockMock{}
			clock.On("Now").Return(time.Now()).Maybe()
			actors.On("FindByAuthID", t.Context(), "actor").Return(test.id, test.role, test.err).Once()
			_, _, err := provider.NewReputationService(reader, actors, clock).Query(t.Context(), "actor", provider.ReputationQueryInput{After: &readmodel.ReputationPosition{WorkOrderID: 5}, CursorProviderID: 7})
			require.ErrorIs(t, err, test.want)
			reader.AssertNotCalled(t, "Read", mock.Anything, mock.Anything, mock.Anything)
			actors.AssertExpectations(t)
		})
	}
}

func TestReputationRejectsInconsistentSnapshot(t *testing.T) {
	valid := func() *readmodel.ReputationSnapshot {
		return &readmodel.ReputationSnapshot{EligiblePaidOrders: 3, ReviewedPaidOrders: 2, VisibleReviews: 2, RatingDistribution: [5]int64{0, 0, 0, 1, 1}, Reviews: []readmodel.ReputationReview{{WorkOrderID: 9, Rating: 5}, {WorkOrderID: 8, Rating: 4}}}
	}
	for name, mutate := range map[string]func(*readmodel.ReputationSnapshot){
		"negative eligible":                func(s *readmodel.ReputationSnapshot) { s.EligiblePaidOrders = -1 },
		"negative reviewed":                func(s *readmodel.ReputationSnapshot) { s.ReviewedPaidOrders = -1 },
		"reviews exceed paid":              func(s *readmodel.ReputationSnapshot) { s.EligiblePaidOrders = 1 },
		"negative bucket":                  func(s *readmodel.ReputationSnapshot) { s.RatingDistribution[0] = -1 },
		"bucket sum mismatch":              func(s *readmodel.ReputationSnapshot) { s.RatingDistribution[0] = 1 },
		"bucket sum overflow":              func(s *readmodel.ReputationSnapshot) { s.RatingDistribution = [5]int64{math.MaxInt64, math.MaxInt64} },
		"empty first page":                 func(s *readmodel.ReputationSnapshot) { s.Reviews = nil },
		"short first page":                 func(s *readmodel.ReputationSnapshot) { s.Reviews = s.Reviews[:1] },
		"invalid rating":                   func(s *readmodel.ReputationSnapshot) { s.Reviews[0].Rating = 0 },
		"invalid ID":                       func(s *readmodel.ReputationSnapshot) { s.Reviews[0].WorkOrderID = 0 },
		"duplicate order":                  func(s *readmodel.ReputationSnapshot) { s.Reviews[1].WorkOrderID = 9 },
		"ascending order":                  func(s *readmodel.ReputationSnapshot) { s.Reviews[1].WorkOrderID = 10 },
		"page distribution exceeds global": func(s *readmodel.ReputationSnapshot) { s.Reviews[1].Rating = 5 },
		"next on exhausted page":           func(s *readmodel.ReputationSnapshot) { s.Next = &readmodel.ReputationPosition{WorkOrderID: 8} },
	} {
		t.Run(name, func(t *testing.T) {
			snapshot := valid()
			mutate(snapshot)
			reader, actors, clock := &reputationReaderMock{}, &providerActorFinderMock{}, &activityClockMock{}
			clock.On("Now").Return(time.Now()).Maybe()
			actors.On("FindByAuthID", mock.Anything, "actor").Return(7, provider.Role, nil).Once()
			reader.On("Read", mock.Anything, 7, mock.Anything).Return(snapshot, nil).Once()
			result, _, err := provider.NewReputationService(reader, actors, clock).Query(t.Context(), "actor", provider.ReputationQueryInput{})
			require.Error(t, err)
			require.Nil(t, result)
			reader.AssertExpectations(t)
		})
	}
}

func TestReputationPropagatesMissingSnapshotAndReadErrors(t *testing.T) {
	for name, failure := range map[string]error{"nil snapshot": nil, "persistence": errors.New("database unavailable"), "cancelled": context.Canceled, "provider removed": provider.ErrReputationProviderNotFound} {
		t.Run(name, func(t *testing.T) {
			reader, actors, clock := &reputationReaderMock{}, &providerActorFinderMock{}, &activityClockMock{}
			clock.On("Now").Return(time.Now()).Maybe()
			actors.On("FindByAuthID", t.Context(), "actor").Return(7, provider.Role, nil).Once()
			reader.On("Read", t.Context(), 7, mock.Anything).Return(nil, failure).Once()
			result, _, err := provider.NewReputationService(reader, actors, clock).Query(t.Context(), "actor", provider.ReputationQueryInput{})
			require.Error(t, err)
			require.Nil(t, result)
			if failure != nil {
				require.ErrorIs(t, err, failure)
			}
			reader.AssertExpectations(t)
		})
	}
}

func TestReputationPassesOwnedCursorAndContext(t *testing.T) {
	reader, actors, clock := &reputationReaderMock{}, &providerActorFinderMock{}, &activityClockMock{}
	clock.On("Now").Return(time.Now()).Once()
	actors.On("FindByAuthID", t.Context(), "actor").Return(7, provider.Role, nil).Once()
	after := &readmodel.ReputationPosition{WorkOrderID: 10}
	reader.On("Read", t.Context(), 7, provider.ReputationQuery{Limit: 100, After: after}).Return(&readmodel.ReputationSnapshot{EligiblePaidOrders: 1, ReviewedPaidOrders: 1, VisibleReviews: 1, RatingDistribution: [5]int64{1}, Reviews: []readmodel.ReputationReview{{WorkOrderID: 9, Rating: 1}}}, nil).Once()
	result, id, err := provider.NewReputationService(reader, actors, clock).Query(t.Context(), "actor", provider.ReputationQueryInput{Limit: 100, After: after, CursorProviderID: 7})
	require.NoError(t, err)
	require.Equal(t, 7, id)
	require.Len(t, result.Reviews, 1)
	reader.AssertExpectations(t)
	actors.AssertExpectations(t)
	clock.AssertExpectations(t)
}

func TestReputationRejectsInvalidContinuationFacts(t *testing.T) {
	for name, test := range map[string]struct {
		input    provider.ReputationQueryInput
		snapshot *readmodel.ReputationSnapshot
	}{
		"page exceeds limit": {
			input:    provider.ReputationQueryInput{Limit: 1},
			snapshot: &readmodel.ReputationSnapshot{EligiblePaidOrders: 2, ReviewedPaidOrders: 2, VisibleReviews: 2, RatingDistribution: [5]int64{2}, Reviews: []readmodel.ReputationReview{{WorkOrderID: 9, Rating: 1}, {WorkOrderID: 8, Rating: 1}}},
		},
		"missing first continuation": {
			input:    provider.ReputationQueryInput{Limit: 1},
			snapshot: &readmodel.ReputationSnapshot{EligiblePaidOrders: 2, ReviewedPaidOrders: 2, VisibleReviews: 2, RatingDistribution: [5]int64{2}, Reviews: []readmodel.ReputationReview{{WorkOrderID: 9, Rating: 1}}},
		},
		"next differs from last delivered": {
			input:    provider.ReputationQueryInput{Limit: 1},
			snapshot: &readmodel.ReputationSnapshot{EligiblePaidOrders: 2, ReviewedPaidOrders: 2, VisibleReviews: 2, RatingDistribution: [5]int64{2}, Reviews: []readmodel.ReputationReview{{WorkOrderID: 9, Rating: 1}}, Next: &readmodel.ReputationPosition{WorkOrderID: 8}},
		},
		"next on empty continuation": {
			input:    provider.ReputationQueryInput{Limit: 1, After: &readmodel.ReputationPosition{WorkOrderID: 9}, CursorProviderID: 7},
			snapshot: &readmodel.ReputationSnapshot{EligiblePaidOrders: 2, ReviewedPaidOrders: 2, VisibleReviews: 2, RatingDistribution: [5]int64{2}, Next: &readmodel.ReputationPosition{WorkOrderID: 8}},
		},
		"page at cursor boundary": {
			input:    provider.ReputationQueryInput{Limit: 1, After: &readmodel.ReputationPosition{WorkOrderID: 9}, CursorProviderID: 7},
			snapshot: &readmodel.ReputationSnapshot{EligiblePaidOrders: 2, ReviewedPaidOrders: 2, VisibleReviews: 2, RatingDistribution: [5]int64{2}, Reviews: []readmodel.ReputationReview{{WorkOrderID: 9, Rating: 1}}},
		},
		"short continuation with next": {
			input:    provider.ReputationQueryInput{Limit: 2, After: &readmodel.ReputationPosition{WorkOrderID: 9}, CursorProviderID: 7},
			snapshot: &readmodel.ReputationSnapshot{EligiblePaidOrders: 3, ReviewedPaidOrders: 3, VisibleReviews: 3, RatingDistribution: [5]int64{3}, Reviews: []readmodel.ReputationReview{{WorkOrderID: 8, Rating: 1}}, Next: &readmodel.ReputationPosition{WorkOrderID: 8}},
		},
	} {
		t.Run(name, func(t *testing.T) {
			reader, actors, clock := &reputationReaderMock{}, &providerActorFinderMock{}, &activityClockMock{}
			clock.On("Now").Return(time.Now()).Once()
			actors.On("FindByAuthID", t.Context(), "actor").Return(7, provider.Role, nil).Once()
			reader.On("Read", t.Context(), 7, mock.Anything).Return(test.snapshot, nil).Once()
			result, _, err := provider.NewReputationService(reader, actors, clock).Query(t.Context(), "actor", test.input)
			require.Error(t, err)
			require.Nil(t, result)
			reader.AssertExpectations(t)
			actors.AssertExpectations(t)
			clock.AssertExpectations(t)
		})
	}
}

func TestReputationAllowsExhaustedContinuationWithoutChangingGlobalResults(t *testing.T) {
	reader, actors, clock := &reputationReaderMock{}, &providerActorFinderMock{}, &activityClockMock{}
	clock.On("Now").Return(time.Now()).Once()
	actors.On("FindByAuthID", t.Context(), "actor").Return(7, provider.Role, nil).Once()
	after := &readmodel.ReputationPosition{WorkOrderID: 1}
	reader.On("Read", t.Context(), 7, provider.ReputationQuery{Limit: 20, After: after}).Return(&readmodel.ReputationSnapshot{EligiblePaidOrders: 2, ReviewedPaidOrders: 1, VisibleReviews: 1, RatingDistribution: [5]int64{0, 0, 0, 0, 1}}, nil).Once()
	result, _, err := provider.NewReputationService(reader, actors, clock).Query(t.Context(), "actor", provider.ReputationQueryInput{After: after, CursorProviderID: 7})
	require.NoError(t, err)
	require.Empty(t, result.Reviews)
	require.NotNil(t, result.Reviews)
	require.Nil(t, result.Next)
	require.Equal(t, float64(5), *result.AverageRating)
	require.Equal(t, float64(50), *result.CoveragePercentage)
	reader.AssertExpectations(t)
	actors.AssertExpectations(t)
	clock.AssertExpectations(t)
}
