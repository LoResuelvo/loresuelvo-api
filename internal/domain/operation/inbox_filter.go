package operation

import (
	"fmt"
	"math"
	"time"
	// Alpine images lack zoneinfo.
	_ "time/tzdata"

	readmodel "github.com/LoResuelvo/loresuelvo-api/internal/domain/operation/read_model"
)

var BusinessLocation = mustLoadLocation("America/Argentina/Buenos_Aires")

func mustLoadLocation(name string) *time.Location {
	location, err := time.LoadLocation(name)
	if err != nil {
		panic(fmt.Sprintf("loading %s: %v", name, err))
	}
	return location
}

type CalendarDay struct {
	Year  int
	Month time.Month
	Day   int
}

func (day CalendarDay) Bounds() (time.Time, time.Time) {
	start := time.Date(day.Year, day.Month, day.Day, 0, 0, 0, 0, BusinessLocation)
	return start, start.AddDate(0, 0, 1)
}

func (day CalendarDay) valid() bool {
	start, _ := day.Bounds()
	return start.Year() == day.Year && start.Month() == day.Month && start.Day() == day.Day
}

type InboxFilter struct {
	ConsumerID   *int
	ProviderID   *int
	CategoryID   *int
	StartedFrom  *time.Time
	StartedTo    *time.Time
	Stage        *readmodel.Stage
	Alert        *readmodel.Alert
	ScheduledDay *CalendarDay
}

func (filter InboxFilter) Validate() error {
	for _, reference := range []struct {
		name string
		id   *int
	}{{"consumer", filter.ConsumerID}, {"provider", filter.ProviderID}, {"category", filter.CategoryID}} {
		if reference.id != nil && (*reference.id <= 0 || *reference.id > math.MaxInt32) {
			return fmt.Errorf("%w: %s ID is out of range", ErrInvalidInboxQuery, reference.name)
		}
	}
	if filter.StartedFrom != nil && filter.StartedTo != nil && !filter.StartedFrom.Before(*filter.StartedTo) {
		return fmt.Errorf("%w: start window must begin before it ends", ErrInvalidInboxQuery)
	}
	if filter.Stage != nil && !validStage(*filter.Stage) {
		return fmt.Errorf("%w: stage is invalid", ErrInvalidInboxQuery)
	}
	if filter.Alert != nil && !validAlert(*filter.Alert) {
		return fmt.Errorf("%w: alert is invalid", ErrInvalidInboxQuery)
	}
	if filter.ScheduledDay != nil && !filter.ScheduledDay.valid() {
		return fmt.Errorf("%w: scheduled day is invalid", ErrInvalidInboxQuery)
	}
	return nil
}

func validStage(stage readmodel.Stage) bool {
	switch stage {
	case readmodel.StageRequestPending, readmodel.StageRequestAccepted, readmodel.StageProposalPending,
		readmodel.StageProposalRejected, readmodel.StageWorkOrderScheduled,
		readmodel.StageWorkOrderAwaitingPayment, readmodel.StageWorkOrderPaid:
		return true
	default:
		return false
	}
}

func validAlert(alert readmodel.Alert) bool {
	switch alert {
	case readmodel.AlertRequestPendingOver24h, readmodel.AlertBookingDeadlinePassed, readmodel.AlertDelayed, readmodel.AlertStalled:
		return true
	default:
		return false
	}
}
