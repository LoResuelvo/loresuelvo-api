package workordercalendar_test

import (
	"context"
	"errors"
	"testing"
	"time"

	calendarconnection "github.com/LoResuelvo/loresuelvo-api/internal/domain/calendar_connection"
	workorder "github.com/LoResuelvo/loresuelvo-api/internal/domain/work_order"
	workordercalendar "github.com/LoResuelvo/loresuelvo-api/internal/domain/work_order_calendar"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

func TestSyncPublishesAnAppointmentForEachConnectedParticipant(t *testing.T) {
	now := time.Date(2026, time.July, 4, 10, 0, 0, 0, time.UTC)
	order, consumerUser, providerUser := workOrderCalendarFixture(t, now)
	consumerConnection := calendarConnectionFixture(t, consumerUser.ID(), now)
	providerConnection := calendarConnectionFixture(t, providerUser.ID(), now)
	consumerEventKey, err := workordercalendar.NewEventKey(order.ID(), consumerUser.ID())
	require.NoError(t, err)
	providerEventKey, err := workordercalendar.NewEventKey(order.ID(), providerUser.ID())
	require.NoError(t, err)

	workOrders := new(workOrderReaderMock)
	connections := new(connectionReaderMock)
	events := new(eventRepositoryMock)
	publisher := new(eventPublisherMock)
	clock := new(clockMock)
	workOrders.On(
		"FindScheduledAfter",
		mock.Anything,
		now,
	).Return([]*workorder.WorkOrder{order}, nil).Once()
	connections.On("FindByUserID", mock.Anything, consumerUser.ID()).Return(consumerConnection, nil).Once()
	connections.On("FindByUserID", mock.Anything, providerUser.ID()).Return(providerConnection, nil).Once()
	events.On("Exists", mock.Anything, consumerEventKey).Return(false, nil).Once()
	events.On("Exists", mock.Anything, providerEventKey).Return(false, nil).Once()
	publisher.On(
		"Create",
		mock.Anything,
		consumerConnection,
		mock.MatchedBy(func(appointment workordercalendar.Appointment) bool {
			return appointment.WorkOrder() == order &&
				appointment.Participant() == consumerUser &&
				appointment.Counterpart() == providerUser
		}),
	).Return(publishedEventFixture(t, "consumer-event"), nil).Once()
	publisher.On(
		"Create",
		mock.Anything,
		providerConnection,
		mock.MatchedBy(func(appointment workordercalendar.Appointment) bool {
			return appointment.WorkOrder() == order &&
				appointment.Participant() == providerUser &&
				appointment.Counterpart() == consumerUser
		}),
	).Return(publishedEventFixture(t, "provider-event"), nil).Once()
	events.On(
		"Save",
		mock.Anything,
		mock.MatchedBy(func(event *workordercalendar.Event) bool {
			return !event.SyncedOn().IsZero() &&
				(event.Key() == consumerEventKey || event.Key() == providerEventKey)
		}),
	).Return(nil).Twice()
	clock.On("Now").Return(now).Once()
	service := workordercalendar.NewService(workOrders, connections, events, publisher, clock)

	err = service.Sync(context.Background())

	require.NoError(t, err)
	workOrders.AssertExpectations(t)
	connections.AssertExpectations(t)
	events.AssertExpectations(t)
	publisher.AssertExpectations(t)
	clock.AssertExpectations(t)
}

func TestSyncSkipsParticipantWithoutConnection(t *testing.T) {
	now := time.Date(2026, time.July, 4, 10, 0, 0, 0, time.UTC)
	order, consumerUser, providerUser := workOrderCalendarFixture(t, now)
	consumerConnection := calendarConnectionFixture(t, consumerUser.ID(), now)
	consumerEventKey, err := workordercalendar.NewEventKey(order.ID(), consumerUser.ID())
	require.NoError(t, err)

	workOrders := new(workOrderReaderMock)
	connections := new(connectionReaderMock)
	events := new(eventRepositoryMock)
	publisher := new(eventPublisherMock)
	clock := new(clockMock)
	workOrders.On(
		"FindScheduledAfter",
		mock.Anything,
		now,
	).Return([]*workorder.WorkOrder{order}, nil).Once()
	connections.On("FindByUserID", mock.Anything, consumerUser.ID()).Return(consumerConnection, nil).Once()
	connections.On("FindByUserID", mock.Anything, providerUser.ID()).Return(nil, calendarconnection.ErrConnectionNotFound).Once()
	events.On("Exists", mock.Anything, consumerEventKey).Return(false, nil).Once()
	publisher.On("Create", mock.Anything, consumerConnection, mock.Anything).
		Return(publishedEventFixture(t, "consumer-event"), nil).Once()
	events.On(
		"Save",
		mock.Anything,
		mock.MatchedBy(func(event *workordercalendar.Event) bool {
			return event.Key() == consumerEventKey && !event.SyncedOn().IsZero()
		}),
	).Return(nil).Once()
	clock.On("Now").Return(now).Once()
	service := workordercalendar.NewService(workOrders, connections, events, publisher, clock)

	err = service.Sync(context.Background())

	require.NoError(t, err)
	workOrders.AssertExpectations(t)
	connections.AssertExpectations(t)
	events.AssertExpectations(t)
	publisher.AssertExpectations(t)
	clock.AssertExpectations(t)
}

func TestSyncReturnsWorkOrderReaderError(t *testing.T) {
	now := time.Date(2026, time.July, 4, 10, 0, 0, 0, time.UTC)
	readErr := errors.New("work order reader unavailable")
	workOrders := new(workOrderReaderMock)
	connections := new(connectionReaderMock)
	events := new(eventRepositoryMock)
	publisher := new(eventPublisherMock)
	clock := new(clockMock)
	workOrders.On("FindScheduledAfter", mock.Anything, mock.Anything).
		Return(nil, readErr).
		Once()
	clock.On("Now").Return(now).Once()
	service := workordercalendar.NewService(workOrders, connections, events, publisher, clock)

	err := service.Sync(context.Background())

	assert.ErrorIs(t, err, readErr)
	connections.AssertNotCalled(t, "FindByUserID", mock.Anything, mock.Anything)
	publisher.AssertNotCalled(t, "Create", mock.Anything, mock.Anything, mock.Anything)
	workOrders.AssertExpectations(t)
	clock.AssertExpectations(t)
}

func TestSyncDoesNotSaveWhenPublishingFails(t *testing.T) {
	now := time.Date(2026, time.July, 4, 10, 0, 0, 0, time.UTC)
	order, consumerUser, _ := workOrderCalendarFixture(t, now)
	consumerConnection := calendarConnectionFixture(t, consumerUser.ID(), now)
	publishErr := errors.New("publisher unavailable")

	workOrders := new(workOrderReaderMock)
	connections := new(connectionReaderMock)
	events := new(eventRepositoryMock)
	publisher := new(eventPublisherMock)
	clock := new(clockMock)
	workOrders.On("FindScheduledAfter", mock.Anything, mock.Anything).
		Return([]*workorder.WorkOrder{order}, nil).
		Once()
	connections.On("FindByUserID", mock.Anything, consumerUser.ID()).Return(consumerConnection, nil).Once()
	events.On("Exists", mock.Anything, mock.Anything).Return(false, nil).Once()
	publisher.On("Create", mock.Anything, consumerConnection, mock.Anything).
		Return(workordercalendar.PublishedEvent{}, publishErr).
		Once()
	clock.On("Now").Return(now).Once()
	service := workordercalendar.NewService(workOrders, connections, events, publisher, clock)

	err := service.Sync(context.Background())

	assert.ErrorIs(t, err, publishErr)
	events.AssertNotCalled(t, "Save", mock.Anything, mock.Anything)
	workOrders.AssertExpectations(t)
	connections.AssertExpectations(t)
	events.AssertExpectations(t)
	publisher.AssertExpectations(t)
	clock.AssertExpectations(t)
}

func TestSyncSkipsAlreadySynchronizedEvent(t *testing.T) {
	now := time.Date(2026, time.July, 4, 10, 0, 0, 0, time.UTC)
	order, consumerUser, providerUser := workOrderCalendarFixture(t, now)
	consumerConnection := calendarConnectionFixture(t, consumerUser.ID(), now)
	consumerEventKey, err := workordercalendar.NewEventKey(order.ID(), consumerUser.ID())
	require.NoError(t, err)

	workOrders := new(workOrderReaderMock)
	connections := new(connectionReaderMock)
	events := new(eventRepositoryMock)
	publisher := new(eventPublisherMock)
	clock := new(clockMock)
	workOrders.On("FindScheduledAfter", mock.Anything, now).Return([]*workorder.WorkOrder{order}, nil).Once()
	connections.On("FindByUserID", mock.Anything, consumerUser.ID()).Return(consumerConnection, nil).Once()
	connections.On("FindByUserID", mock.Anything, providerUser.ID()).Return(nil, calendarconnection.ErrConnectionNotFound).Once()
	events.On("Exists", mock.Anything, consumerEventKey).Return(true, nil).Once()
	clock.On("Now").Return(now).Once()
	service := workordercalendar.NewService(workOrders, connections, events, publisher, clock)

	err = service.Sync(context.Background())

	require.NoError(t, err)
	publisher.AssertNotCalled(t, "Create", mock.Anything, mock.Anything, mock.Anything)
	events.AssertNotCalled(t, "Save", mock.Anything, mock.Anything)
	workOrders.AssertExpectations(t)
	connections.AssertExpectations(t)
	events.AssertExpectations(t)
	clock.AssertExpectations(t)
}
