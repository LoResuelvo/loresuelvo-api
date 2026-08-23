package workordercalendar

import (
	"context"
	"errors"
	"fmt"
	"time"

	calendarconnection "github.com/LoResuelvo/loresuelvo-api/internal/domain/calendar_connection"
	"github.com/LoResuelvo/loresuelvo-api/internal/domain/notification"
	"github.com/LoResuelvo/loresuelvo-api/internal/domain/user"
	workorder "github.com/LoResuelvo/loresuelvo-api/internal/domain/work_order"
)

type Service struct {
	workOrders  WorkOrderReader
	connections ConnectionReader
	events      EventRepository
	publisher   EventPublisher
	clock       Clock
	notificator notification.Notificator
}

func NewService(
	workOrders WorkOrderReader,
	connections ConnectionReader,
	events EventRepository,
	publisher EventPublisher,
	clock Clock,
	notificator notification.Notificator,
) *Service {
	return &Service{
		workOrders:  workOrders,
		connections: connections,
		events:      events,
		publisher:   publisher,
		clock:       clock,
		notificator: notificator,
	}
}

func (service *Service) Sync(ctx context.Context) error {
	now := service.clock.Now().UTC()
	orders, err := service.workOrders.FindScheduledAfter(ctx, now)
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
	connection, err := service.connections.FindByUserID(ctx, participant.ID())
	if errors.Is(err, calendarconnection.ErrConnectionNotFound) {
		return nil
	}
	if err != nil {
		return fmt.Errorf("finding calendar connection for participant: %w", err)
	}
	if connection.Status() == calendarconnection.StatusActionRequired {
		if err := service.notifyReauthorizationRequired(ctx, participant.ID()); err != nil {
			return err
		}
		return nil
	}

	event, err := NewEvent(order, participant)
	if err != nil {
		return fmt.Errorf("creating calendar event: %w", err)
	}
	alreadySynchronized, err := service.events.Exists(ctx, event.Key())
	if err != nil {
		return fmt.Errorf("checking existing calendar appointment: %w", err)
	}
	if alreadySynchronized {
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

func (service *Service) notifyReauthorizationRequired(ctx context.Context, userID int) error {
	reauthorizationNotification := notification.NewNotification(
		userID,
		notification.TypeCalendarReauthorizationRequired,
		notification.ResourceCalendarConnection,
		userID,
		service.clock,
	)
	if err := service.notificator.Notify(ctx, reauthorizationNotification); err != nil {
		return fmt.Errorf("notifying calendar reauthorization required: %w", err)
	}
	return nil
}
