package workordercalendar

import (
	"context"
	"fmt"
	"time"

	"github.com/LoResuelvo/loresuelvo-api/internal/domain/user"
	workorder "github.com/LoResuelvo/loresuelvo-api/internal/domain/work_order"
)

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
	if err != nil {
		return fmt.Errorf("finding calendar connection for participant: %w", err)
	}

	event, err := NewEvent(order, participant)
	if err != nil {
		return fmt.Errorf("creating calendar event: %w", err)
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
