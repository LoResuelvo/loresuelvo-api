package provider

import (
	"errors"
	"time"

	"github.com/LoResuelvo/loresuelvo-api/internal/domain/provider/read_model"
)

var ErrInvalidActivityQuery = errors.New("invalid activity query")

type ActivityGranularity string

const (
	ActivityDay   ActivityGranularity = "day"
	ActivityWeek  ActivityGranularity = "week"
	ActivityMonth ActivityGranularity = "month"
)

const activityMaxDuration = 365 * 24 * time.Hour
const activityDefaultDuration = 30 * 24 * time.Hour
const activityTimeZone = "America/Argentina/Buenos_Aires"

// ActivityQueryInput contains already-parsed transport values. The HTTP adapter
// owns syntax validation; this value owns semantic range validation.
type ActivityQueryInput struct {
	From, To        *time.Time
	Granularity     string
	ComparePrevious bool
}

type ActivityQuery struct {
	From, To        time.Time
	Granularity     ActivityGranularity
	ComparePrevious bool
}

func (q ActivityQuery) PreviousPeriod() readmodel.ActivityPeriod {
	duration := q.To.Sub(q.From)
	return readmodel.ActivityPeriod{From: q.From.Add(-duration), To: q.From}
}

func (input ActivityQueryInput) resolve(now time.Time) (ActivityQuery, error) {
	from, to, valid := resolveStatisticsPeriod(now, input.From, input.To)
	if !valid {
		return ActivityQuery{}, ErrInvalidActivityQuery
	}
	q := ActivityQuery{From: from, To: to, ComparePrevious: input.ComparePrevious}

	switch input.Granularity {
	case "", string(ActivityDay):
		q.Granularity = ActivityDay
	case string(ActivityWeek):
		q.Granularity = ActivityWeek
	case string(ActivityMonth):
		q.Granularity = ActivityMonth
	default:
		return ActivityQuery{}, ErrInvalidActivityQuery
	}
	return q, nil
}

func activityBucketStart(at time.Time, granularity ActivityGranularity, location *time.Location) time.Time {
	local := at.In(location)
	year, month, day := local.Date()
	switch granularity {
	case ActivityWeek:
		day -= (int(local.Weekday()) + 6) % 7
	case ActivityMonth:
		day = 1
	}
	return time.Date(year, month, day, 0, 0, 0, 0, location).UTC()
}

func nextActivityBucket(start time.Time, granularity ActivityGranularity, location *time.Location) time.Time {
	local := start.In(location)
	switch granularity {
	case ActivityWeek:
		return local.AddDate(0, 0, 7).UTC()
	case ActivityMonth:
		return local.AddDate(0, 1, 0).UTC()
	default:
		return local.AddDate(0, 0, 1).UTC()
	}
}
