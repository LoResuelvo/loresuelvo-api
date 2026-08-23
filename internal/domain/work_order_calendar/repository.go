package workordercalendar

import (
	"context"
	"time"

	calendarconnection "github.com/LoResuelvo/loresuelvo-api/internal/domain/calendar_connection"
	"github.com/LoResuelvo/loresuelvo-api/internal/domain/clock"
	workorder "github.com/LoResuelvo/loresuelvo-api/internal/domain/work_order"
)

type EventRepository interface {
	FindByKey(ctx context.Context, key EventKey) (*Event, error)
	Save(ctx context.Context, event *Event) error
}

type ConnectionReader interface {
	FindByUserID(ctx context.Context, userID int) (*calendarconnection.Connection, error)
}

type WorkOrderReader interface {
	FindScheduledBetween(ctx context.Context, from, to time.Time) ([]*workorder.WorkOrder, error)
}

type EventPublisher interface {
	Create(ctx context.Context, connection *calendarconnection.Connection, appointment Appointment) (PublishedEvent, error)
}

type Clock = clock.Clock
