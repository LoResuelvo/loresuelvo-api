package workordercalendar

import (
	"context"
	"time"

	calendarconnection "github.com/LoResuelvo/loresuelvo-api/internal/domain/calendar_connection"
	"github.com/LoResuelvo/loresuelvo-api/internal/domain/clock"
	workorder "github.com/LoResuelvo/loresuelvo-api/internal/domain/work_order"
)

type EventRepository interface {
	Exists(ctx context.Context, key EventKey) (bool, error)
	Save(ctx context.Context, event *Event) error
}

type ConnectionReader interface {
	FindByUserID(ctx context.Context, userID int) (*calendarconnection.Connection, error)
}

type WorkOrderReader interface {
	FindScheduledAfter(ctx context.Context, from time.Time) ([]*workorder.WorkOrder, error)
}

type EventPublisher interface {
	Create(ctx context.Context, connection *calendarconnection.Connection, appointment Appointment) (PublishedEvent, error)
}

type Clock = clock.Clock
