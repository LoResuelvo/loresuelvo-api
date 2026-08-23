package repositories

import (
	"context"
	"database/sql"
	"fmt"

	workordercalendar "github.com/LoResuelvo/loresuelvo-api/internal/domain/work_order_calendar"
)

type WorkOrderCalendarEventRepository struct {
	db *sql.DB
}

func NewWorkOrderCalendarEventRepository(db *sql.DB) *WorkOrderCalendarEventRepository {
	return &WorkOrderCalendarEventRepository{db: db}
}

func (repository *WorkOrderCalendarEventRepository) Exists(ctx context.Context, key workordercalendar.EventKey) (bool, error) {
	var exists bool
	err := repository.db.QueryRowContext(
		ctx,
		`SELECT EXISTS(
			SELECT 1
			FROM work_order_calendar_events
			WHERE work_order_id = $1 AND user_id = $2
		)`,
		key.WorkOrderID(),
		key.ParticipantID(),
	).Scan(&exists)
	if err != nil {
		return false, fmt.Errorf("checking work order calendar event existence: %w", err)
	}
	return exists, nil
}

func (repository *WorkOrderCalendarEventRepository) Save(ctx context.Context, event *workordercalendar.Event) error {
	key := event.Key()
	published := event.Published()
	_, err := repository.db.ExecContext(
		ctx,
		`INSERT INTO work_order_calendar_events (
			work_order_id,
			user_id,
			calendar_id,
			google_event_id,
			synced_on,
			updated_on
		)
		VALUES ($1, $2, $3, $4, $5, $5)`,
		key.WorkOrderID(),
		key.ParticipantID(),
		published.CalendarID(),
		published.ExternalID(),
		event.SyncedOn(),
	)
	if err != nil {
		return fmt.Errorf("saving work order calendar event: %w", err)
	}
	return nil
}
