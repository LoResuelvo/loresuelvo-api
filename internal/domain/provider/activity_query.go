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
	if (input.From == nil) != (input.To == nil) {
		return ActivityQuery{}, ErrInvalidActivityQuery
	}
	q := ActivityQuery{ComparePrevious: input.ComparePrevious}
	if input.From == nil {
		q.From, q.To = now.Add(-activityDefaultDuration), now
	} else {
		q.From, q.To = input.From.UTC(), input.To.UTC()
	}
	if q.From.IsZero() || q.To.IsZero() || !q.From.Before(q.To) || q.To.After(now) || q.To.Sub(q.From) > activityMaxDuration {
		return ActivityQuery{}, ErrInvalidActivityQuery
	}
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
