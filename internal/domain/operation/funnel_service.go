package operation

import (
	"context"
	"errors"
	"fmt"
	"math"
	"time"

	"github.com/LoResuelvo/loresuelvo-api/internal/domain/clock"
	readmodel "github.com/LoResuelvo/loresuelvo-api/internal/domain/operation/read_model"
)

var ErrInvalidFunnelQuery = errors.New("invalid funnel query")
var ErrInvalidFunnelSnapshot = errors.New("invalid funnel snapshot")

type FunnelQuery struct {
	Period     *TimeWindow
	CategoryID *int
}
type FunnelCriteria struct {
	Period     TimeWindow
	CategoryID *int
}
type FunnelReader interface {
	Read(context.Context, FunnelCriteria) (readmodel.FunnelSnapshot, error)
}
type FunnelService struct {
	reader FunnelReader
	clock  clock.Clock
}

func NewFunnelService(reader FunnelReader, clock clock.Clock) *FunnelService {
	return &FunnelService{reader: reader, clock: clock}
}
func (s *FunnelService) Query(ctx context.Context, query FunnelQuery) (readmodel.FunnelMetrics, error) {
	now := s.clock.Now().UTC()
	period := TimeWindow{From: now.Add(-30 * 24 * time.Hour), To: now}
	if query.Period != nil {
		period = TimeWindow{From: query.Period.From.UTC(), To: query.Period.To.UTC()}
	}
	if period.From.IsZero() || period.To.IsZero() || !period.From.Before(period.To) || period.To.Sub(period.From) > 365*24*time.Hour || period.To.After(now) {
		return readmodel.FunnelMetrics{}, ErrInvalidFunnelQuery
	}
	var categoryID *int
	if query.CategoryID != nil {
		if *query.CategoryID <= 0 || *query.CategoryID > math.MaxInt32 {
			return readmodel.FunnelMetrics{}, ErrInvalidFunnelQuery
		}
		categoryID = new(*query.CategoryID)
	}
	snapshot, err := s.reader.Read(ctx, FunnelCriteria{Period: period, CategoryID: categoryID})
	if err != nil {
		return readmodel.FunnelMetrics{}, fmt.Errorf("reading funnel snapshot: %w", err)
	}
	ai, err := projectFunnelCohort(snapshot.AI, false)
	if err != nil {
		return readmodel.FunnelMetrics{}, err
	}
	manual, err := projectFunnelCohort(snapshot.Manual, true)
	if err != nil {
		return readmodel.FunnelMetrics{}, err
	}
	return readmodel.FunnelMetrics{From: period.From, To: period.To, ObservedAt: now, CategoryID: categoryID, TimeZone: "America/Argentina/Buenos_Aires", Rounding: "half_up", DecimalPlaces: 2, AI: ai, Manual: manual}, nil
}
