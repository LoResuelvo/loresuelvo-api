package provider

import "time"

func resolveStatisticsPeriod(now time.Time, from, to *time.Time) (time.Time, time.Time, bool) {
	if (from == nil) != (to == nil) {
		return time.Time{}, time.Time{}, false
	}
	now = now.UTC()
	start, end := now.Add(-activityDefaultDuration), now
	if from != nil {
		start, end = from.UTC(), to.UTC()
	}
	if start.IsZero() || end.IsZero() || !start.Before(end) || end.After(now) || end.Sub(start) > activityMaxDuration {
		return time.Time{}, time.Time{}, false
	}
	return start, end, true
}
