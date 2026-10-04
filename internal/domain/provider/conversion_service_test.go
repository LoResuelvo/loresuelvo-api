package provider_test

import (
	"errors"
	"testing"
	"time"

	"github.com/LoResuelvo/loresuelvo-api/internal/domain/provider"
	"github.com/LoResuelvo/loresuelvo-api/internal/domain/provider/read_model"
	"github.com/LoResuelvo/loresuelvo-api/internal/domain/user"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

var conversionNow = time.Date(2026, 9, 29, 15, 0, 0, 0, time.UTC)

func TestConversionServiceRejectsInvalidPeriodsBeforeReading(t *testing.T) {
	before := conversionNow.Add(-time.Hour)
	after := conversionNow.Add(time.Hour)
	tooOld := conversionNow.Add(-365*24*time.Hour - time.Nanosecond)
	zero := time.Time{}
	cases := []struct {
		name  string
		input provider.ConversionQueryInput
	}{
		{"zero start", provider.ConversionQueryInput{From: &zero, To: &conversionNow}},
		{"missing to", provider.ConversionQueryInput{From: &before}},
		{"missing from", provider.ConversionQueryInput{To: &before}},
		{"equal bounds", provider.ConversionQueryInput{From: &before, To: &before}},
		{"reversed bounds", provider.ConversionQueryInput{From: &after, To: &before}},
		{"future to", provider.ConversionQueryInput{From: &before, To: &after}},
		{"too long", provider.ConversionQueryInput{From: &tooOld, To: &conversionNow}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			reader := &conversionReaderMock{}
			actors := &providerActorFinderMock{}
			clock := &activityClockMock{}
			clock.On("Now").Return(conversionNow).Once()
			result, err := provider.NewConversionService(reader, actors, clock).Query(t.Context(), "subject", tc.input)
			require.Nil(t, result)
			require.ErrorIs(t, err, provider.ErrInvalidConversionQuery)
			reader.AssertNotCalled(t, "Read", mock.Anything, mock.Anything, mock.Anything)
			actors.AssertNotCalled(t, "FindByAuthID", mock.Anything, mock.Anything)
			clock.AssertExpectations(t)
		})
	}
}

func TestConversionServiceSeparatesMissingAndForbiddenActors(t *testing.T) {
	cases := []struct {
		name      string
		id        int
		role      string
		finderErr error
		want      error
	}{
		{"missing user", 0, "", user.ErrNotFound, provider.ErrConversionProviderNotFound},
		{"consumer", 12, "consumer", nil, provider.ErrConversionForbidden},
		{"provider missing subtype", 0, provider.Role, nil, provider.ErrConversionProviderNotFound},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			reader := &conversionReaderMock{}
			actors := &providerActorFinderMock{}
			actors.On("FindByAuthID", mock.Anything, "subject").Return(tc.id, tc.role, tc.finderErr).Once()
			clock := &activityClockMock{}
			clock.On("Now").Return(conversionNow).Once()
			result, err := provider.NewConversionService(reader, actors, clock).Query(t.Context(), "subject", provider.ConversionQueryInput{})
			require.Nil(t, result)
			require.ErrorIs(t, err, tc.want)
			reader.AssertNotCalled(t, "Read", mock.Anything, mock.Anything, mock.Anything)
			actors.AssertExpectations(t)
		})
	}
}

func TestConversionServiceUsesOneClockReadingAndDefaultRange(t *testing.T) {
	reader := &conversionReaderMock{}
	actors := &providerActorFinderMock{}
	clock := &activityClockMock{}
	clock.On("Now").Return(conversionNow).Once()
	actors.On("FindByAuthID", mock.Anything, "subject").Return(42, provider.Role, nil).Once()
	reader.On("Read", mock.Anything, 42, mock.MatchedBy(func(q provider.ConversionQuery) bool {
		return q.From.Equal(conversionNow.Add(-30*24*time.Hour)) && q.To.Equal(conversionNow)
	})).Return(&readmodel.ConversionSnapshot{}, nil).Once()
	result, err := provider.NewConversionService(reader, actors, clock).Query(t.Context(), "subject", provider.ConversionQueryInput{})
	require.NoError(t, err)
	require.Equal(t, conversionNow, result.ObservedAt)
	require.Equal(t, "America/Argentina/Buenos_Aires", result.TimeZone)
	reader.AssertExpectations(t)
	actors.AssertExpectations(t)
	clock.AssertExpectations(t)
}

func TestConversionServicePropagatesFailures(t *testing.T) {
	failure := errors.New("unavailable")
	for _, tc := range []struct {
		name                string
		actorErr, errorRead error
		snapshot            *readmodel.ConversionSnapshot
	}{
		{name: "actor", actorErr: failure}, {name: "reader", errorRead: failure}, {name: "nil snapshot"}, {name: "invalid snapshot", snapshot: &readmodel.ConversionSnapshot{Stages: readmodel.ConversionStages{Issued: 1, Contracted: 2}}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			reader := &conversionReaderMock{}
			actors := &providerActorFinderMock{}
			clock := &activityClockMock{}
			clock.On("Now").Return(conversionNow).Once()
			actors.On("FindByAuthID", mock.Anything, "subject").Return(42, provider.Role, tc.actorErr).Once()
			if tc.actorErr == nil {
				reader.On("Read", mock.Anything, 42, mock.Anything).Return(tc.snapshot, tc.errorRead).Once()
			}
			result, err := provider.NewConversionService(reader, actors, clock).Query(t.Context(), "subject", provider.ConversionQueryInput{})
			require.Nil(t, result)
			require.Error(t, err)
			if tc.actorErr != nil || tc.errorRead != nil {
				require.ErrorIs(t, err, failure)
			}
			actors.AssertExpectations(t)
			reader.AssertExpectations(t)
			clock.AssertExpectations(t)
		})
	}
}
func TestConversionServiceRequiresDependencies(t *testing.T) {
	for _, svc := range []*provider.ConversionService{provider.NewConversionService(nil, &providerActorFinderMock{}, &activityClockMock{}), provider.NewConversionService(&conversionReaderMock{}, nil, &activityClockMock{}), provider.NewConversionService(&conversionReaderMock{}, &providerActorFinderMock{}, nil)} {
		result, err := svc.Query(t.Context(), "subject", provider.ConversionQueryInput{})
		require.Nil(t, result)
		require.Error(t, err)
	}
}
func TestConversionServiceNormalizesMaximumPeriod(t *testing.T) {
	location := time.FixedZone("offset", -3*3600)
	to := conversionNow.In(location)
	from := to.Add(-365 * 24 * time.Hour)
	reader := &conversionReaderMock{}
	actors := &providerActorFinderMock{}
	clock := &activityClockMock{}
	clock.On("Now").Return(to).Once()
	actors.On("FindByAuthID", mock.Anything, "subject").Return(42, provider.Role, nil).Once()
	reader.On("Read", mock.Anything, 42, provider.ConversionQuery{From: from.UTC(), To: to.UTC()}).Return(&readmodel.ConversionSnapshot{}, nil).Once()
	result, err := provider.NewConversionService(reader, actors, clock).Query(t.Context(), "subject", provider.ConversionQueryInput{From: &from, To: &to})
	require.NoError(t, err)
	require.Equal(t, to.UTC(), result.ObservedAt)
	require.Equal(t, from.UTC(), result.Period.From)
	actors.AssertExpectations(t)
	reader.AssertExpectations(t)
	clock.AssertExpectations(t)
}
