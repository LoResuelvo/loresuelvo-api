package operation_test

import (
	"context"
	"errors"
	"math"
	"testing"
	"time"

	"github.com/LoResuelvo/loresuelvo-api/internal/domain/operation"
	readmodel "github.com/LoResuelvo/loresuelvo-api/internal/domain/operation/read_model"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

func funnelEmptySnapshot() readmodel.FunnelSnapshot {
	z := readmodel.FunnelDelayAggregate{TotalMicroseconds: "0"}
	c := readmodel.FunnelCohortSnapshot{Delays: readmodel.FunnelDelayTotals{RequestToFirstProposal: z, ProposalToConfirmedHiring: z, ConfirmedHiringToReportedCompletion: z, ReportedCompletionToFullPayment: z}}
	return readmodel.FunnelSnapshot{AI: c, Manual: c}
}
func TestFunnelDefaultPeriodAndEmptyCohorts(t *testing.T) {
	now := time.Date(2026, 9, 30, 15, 0, 0, 123, time.UTC)
	r := &funnelReaderMock{}
	r.On("Read", mock.Anything, operation.FunnelCriteria{Period: operation.TimeWindow{From: now.Add(-30 * 24 * time.Hour), To: now}}).Return(funnelEmptySnapshot(), nil).Once()
	got, err := operation.NewFunnelService(r, inboxFixedClock{now}).Query(context.Background(), operation.FunnelQuery{})
	require.NoError(t, err)
	require.Equal(t, now, got.ObservedAt)
	require.Equal(t, now.Add(-30*24*time.Hour), got.From)
	require.Equal(t, "America/Argentina/Buenos_Aires", got.TimeZone)
	require.Equal(t, "half_up", got.Rounding)
	require.Equal(t, 2, got.DecimalPlaces)
	require.Len(t, got.AI.Stages, 7)
	require.Len(t, got.Manual.Stages, 6)
	require.Nil(t, got.AI.GlobalCompletionConversionHundredths)
	require.Nil(t, got.Manual.Delays.RequestToFirstProposal.MeanSecondsHundredths)
	r.AssertExpectations(t)
}
func TestFunnelRejectsInvalidQueryBeforeRead(t *testing.T) {
	now := time.Date(2026, 9, 30, 15, 0, 0, 0, time.UTC)
	for name, q := range map[string]operation.FunnelQuery{
		"missing from": {Period: &operation.TimeWindow{To: now}}, "missing to": {Period: &operation.TimeWindow{From: now.Add(-time.Hour)}},
		"equal": {Period: &operation.TimeWindow{From: now, To: now}}, "reversed": {Period: &operation.TimeWindow{From: now, To: now.Add(-time.Hour)}},
		"too long": {Period: &operation.TimeWindow{From: now.Add(-365*24*time.Hour - time.Nanosecond), To: now}}, "future": {Period: &operation.TimeWindow{From: now.Add(-time.Hour), To: now.Add(time.Nanosecond)}},
		"zero category": {CategoryID: new(int)}, "negative category": {CategoryID: new(-1)}, "overflow category": {CategoryID: new(int(math.MaxInt32) + 1)},
	} {
		t.Run(name, func(t *testing.T) {
			r := &funnelReaderMock{}
			_, err := operation.NewFunnelService(r, inboxFixedClock{now}).Query(context.Background(), q)
			require.ErrorIs(t, err, operation.ErrInvalidFunnelQuery)
			r.AssertNotCalled(t, "Read", mock.Anything, mock.Anything)
		})
	}
}
func TestFunnelExactConversionsAndDelayRounding(t *testing.T) {
	snap := funnelEmptySnapshot()
	snap.AI.Counts = readmodel.FunnelCounts{Origins: 3, Requested: 2, Proposed: 2, Hired: 2, Completed: 2, Paid: 1, Reviewed: 1}
	snap.AI.Delays.RequestToFirstProposal = readmodel.FunnelDelayAggregate{Observations: 2, TotalMicroseconds: "66670000.000000"}
	r := &funnelReaderMock{}
	r.On("Read", mock.Anything, mock.Anything).Return(snap, nil).Once()
	got, err := operation.NewFunnelService(r, inboxFixedClock{time.Now()}).Query(context.Background(), operation.FunnelQuery{})
	require.NoError(t, err)
	require.Nil(t, got.AI.Stages[0].ConversionHundredths)
	require.EqualValues(t, 6667, *got.AI.Stages[1].ConversionHundredths)
	require.EqualValues(t, 5000, *got.AI.Stages[5].ConversionHundredths)
	require.EqualValues(t, 6667, *got.AI.GlobalCompletionConversionHundredths)
	require.EqualValues(t, 3334, *got.AI.Delays.RequestToFirstProposal.MeanSecondsHundredths)
	r.AssertExpectations(t)
}
func TestFunnelLargeExactAggregate(t *testing.T) {
	snap := funnelEmptySnapshot()
	snap.Manual.Counts = readmodel.FunnelCounts{Origins: math.MaxInt64, Requested: math.MaxInt64, Proposed: math.MaxInt64 - 1}
	snap.Manual.Delays.RequestToFirstProposal = readmodel.FunnelDelayAggregate{Observations: 2, TotalMicroseconds: "18446744073709551614"}
	r := &funnelReaderMock{}
	r.On("Read", mock.Anything, mock.Anything).Return(snap, nil).Once()
	got, err := operation.NewFunnelService(r, inboxFixedClock{time.Now()}).Query(context.Background(), operation.FunnelQuery{})
	require.NoError(t, err)
	require.EqualValues(t, 10000, *got.Manual.Stages[1].ConversionHundredths)
	require.EqualValues(t, 922337203685478, *got.Manual.Delays.RequestToFirstProposal.MeanSecondsHundredths)
}
func TestFunnelFailsClosedForInvalidSnapshots(t *testing.T) {
	for name, change := range map[string]func(*readmodel.FunnelSnapshot){
		"negative count":         func(s *readmodel.FunnelSnapshot) { s.AI.Counts.Origins = -1 },
		"non nested":             func(s *readmodel.FunnelSnapshot) { s.AI.Counts.Proposed = 1 },
		"manual origin":          func(s *readmodel.FunnelSnapshot) { s.Manual.Counts.Origins = 1 },
		"missing sum":            func(s *readmodel.FunnelSnapshot) { s.AI.Delays.RequestToFirstProposal.TotalMicroseconds = "" },
		"negative sum":           func(s *readmodel.FunnelSnapshot) { s.AI.Delays.RequestToFirstProposal.TotalMicroseconds = "-1" },
		"fractional microsecond": func(s *readmodel.FunnelSnapshot) { s.AI.Delays.RequestToFirstProposal.TotalMicroseconds = "0.5" },
		"negative sample":        func(s *readmodel.FunnelSnapshot) { s.AI.Delays.RequestToFirstProposal.Observations = -1 },
		"no sample nonzero":      func(s *readmodel.FunnelSnapshot) { s.AI.Delays.RequestToFirstProposal.TotalMicroseconds = "1" },
		"overflow mean": func(s *readmodel.FunnelSnapshot) {
			s.AI.Delays.RequestToFirstProposal = readmodel.FunnelDelayAggregate{Observations: 1, TotalMicroseconds: "999999999999999999999999999999"}
		},
	} {
		t.Run(name, func(t *testing.T) {
			snap := funnelEmptySnapshot()
			change(&snap)
			r := &funnelReaderMock{}
			r.On("Read", mock.Anything, mock.Anything).Return(snap, nil).Once()
			got, err := operation.NewFunnelService(r, inboxFixedClock{time.Now()}).Query(context.Background(), operation.FunnelQuery{})
			require.ErrorIs(t, err, operation.ErrInvalidFunnelSnapshot)
			require.Equal(t, readmodel.FunnelMetrics{}, got)
		})
	}
}
func TestFunnelPropagatesReadFailure(t *testing.T) {
	r := &funnelReaderMock{}
	cause := errors.New("read failed")
	r.On("Read", mock.Anything, mock.Anything).Return(readmodel.FunnelSnapshot{}, cause).Once()
	got, err := operation.NewFunnelService(r, inboxFixedClock{time.Now()}).Query(context.Background(), operation.FunnelQuery{})
	require.ErrorIs(t, err, cause)
	require.Equal(t, readmodel.FunnelMetrics{}, got)
}

func TestFunnelCapturesClockOnceAndPreservesContext(t *testing.T) {
	now := time.Date(2026, 9, 30, 12, 0, 0, 0, time.FixedZone("Argentina", -3*60*60))
	clock := &funnelClockMock{}
	clock.On("Now").Return(now).Once()
	r := &funnelReaderMock{}
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	category := 2147483000
	period := operation.TimeWindow{From: now.Add(-365 * 24 * time.Hour), To: now}
	r.On("Read", ctx, operation.FunnelCriteria{Period: operation.TimeWindow{From: period.From.UTC(), To: period.To.UTC()}, CategoryID: &category}).Return(funnelEmptySnapshot(), nil).Once()
	got, err := operation.NewFunnelService(r, clock).Query(ctx, operation.FunnelQuery{Period: &period, CategoryID: &category})
	require.NoError(t, err)
	require.Equal(t, now.UTC(), got.ObservedAt)
	require.Equal(t, time.UTC, got.From.Location())
	require.Equal(t, category, *got.CategoryID)
	clock.AssertExpectations(t)
	r.AssertExpectations(t)
}
func TestFunnelDistinguishesNoDenominatorFromZeroAdvance(t *testing.T) {
	snap := funnelEmptySnapshot()
	snap.AI.Counts.Origins = 3
	r := &funnelReaderMock{}
	r.On("Read", mock.Anything, mock.Anything).Return(snap, nil).Once()
	got, err := operation.NewFunnelService(r, inboxFixedClock{time.Now()}).Query(context.Background(), operation.FunnelQuery{})
	require.NoError(t, err)
	require.EqualValues(t, 0, *got.AI.Stages[1].ConversionHundredths)
	require.Nil(t, got.AI.Stages[2].ConversionHundredths)
	require.EqualValues(t, 0, *got.AI.GlobalCompletionConversionHundredths)
	require.Nil(t, got.Manual.GlobalCompletionConversionHundredths)
}
