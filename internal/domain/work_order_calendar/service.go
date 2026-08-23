package workordercalendar

import (
	"context"
	"errors"
	"fmt"
	"time"

	calendarconnection "github.com/LoResuelvo/loresuelvo-api/internal/domain/calendar_connection"
	"github.com/LoResuelvo/loresuelvo-api/internal/domain/user"
	workorder "github.com/LoResuelvo/loresuelvo-api/internal/domain/work_order"
)

const futureOrderHorizon = 365 * 24 * time.Hour

type Service struct {
	workOrders  WorkOrderReader
	connections ConnectionReader
	events      EventRepository
	publisher   EventPublisher
	clock       Clock
}

func NewService(
	workOrders WorkOrderReader,
	connections ConnectionReader,
	events EventRepository,
	publisher EventPublisher,
	clock Clock,
) *Service {
	return &Service{
		workOrders:  workOrders,
		connections: connections,
		events:      events,
		publisher:   publisher,
		clock:       clock,
	}
}

func (service *Service) Sync(ctx context.Context) error {
	now := service.clock.Now().UTC()
	orders, err := service.workOrders.FindScheduledBetween(ctx, now, now.Add(futureOrderHorizon))
	if err != nil {
		return fmt.Errorf("finding future work orders for calendar: %w", err)
	}
	for _, order := range orders {
		if err := service.syncOrder(ctx, order, now); err != nil {
			return err
		}
	}
	return nil
}

func (service *Service) syncOrder(ctx context.Context, order *workorder.WorkOrder, now time.Time) error {
	if err := service.syncParticipant(ctx, order, order.Consumer(), order.Provider(), now); err != nil {
		return err
	}
	return service.syncParticipant(ctx, order, order.Provider(), order.Consumer(), now)
}

func (service *Service) syncParticipant(
	ctx context.Context,
	order *workorder.WorkOrder,
	participant user.User,
	counterpart user.User,
	now time.Time,
) error {
	connection, connected, err := service.connectedConnection(ctx, participant)
	if err != nil {
		return err
	}
	if !connected {
		return nil
	}

	event, err := service.eventForParticipant(ctx, order, participant)
	if err != nil {
		return err
	}
	if event.IsSynced() {
		return nil
	}

	appointment := NewAppointment(order, participant, counterpart)
	published, err := service.publisher.Create(ctx, connection, appointment)
	if err != nil {
		return fmt.Errorf("publishing calendar appointment: %w", err)
	}
	if err := event.MarkSynced(published, now); err != nil {
		return fmt.Errorf("marking calendar appointment synchronized: %w", err)
	}
	if err := service.events.Save(ctx, event); err != nil {
		return fmt.Errorf("saving calendar appointment: %w", err)
	}
	return nil
}

func (service *Service) connectedConnection(ctx context.Context, participant user.User) (*calendarconnection.Connection, bool, error) {
	connection, err := service.connections.FindByUserID(ctx, participant.ID())
	if errors.Is(err, calendarconnection.ErrConnectionNotFound) {
		return nil, false, nil
	}
	if err != nil {
		return nil, false, fmt.Errorf("finding calendar connection for participant %d: %w", participant.ID(), err)
	}
	return connection, connection.IsConnected(), nil
}

func (service *Service) eventForParticipant(ctx context.Context, order *workorder.WorkOrder, participant user.User) (*Event, error) {
	key, err := NewEventKey(order.ID(), participant.ID())
	if err != nil {
		return nil, fmt.Errorf("identifying calendar event: %w", err)
	}
	event, err := service.events.FindByKey(ctx, key)
	if errors.Is(err, ErrCalendarEventNotFound) {
		event, err = NewEvent(order, participant)
		if err != nil {
			return nil, fmt.Errorf("creating calendar event: %w", err)
		}
	} else if err != nil {
		return nil, fmt.Errorf("finding calendar event: %w", err)
	}
	return event, nil
}
