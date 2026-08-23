package workordercalendar_test

import (
	"context"
	"testing"
	"time"

	calendarconnection "github.com/LoResuelvo/loresuelvo-api/internal/domain/calendar_connection"
	"github.com/LoResuelvo/loresuelvo-api/internal/domain/consumer"
	"github.com/LoResuelvo/loresuelvo-api/internal/domain/provider"
	serviceproposal "github.com/LoResuelvo/loresuelvo-api/internal/domain/service_proposal"
	"github.com/LoResuelvo/loresuelvo-api/internal/domain/user"
	workorder "github.com/LoResuelvo/loresuelvo-api/internal/domain/work_order"
	workordercalendar "github.com/LoResuelvo/loresuelvo-api/internal/domain/work_order_calendar"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

type workOrderReaderMock struct{ mock.Mock }

func (m *workOrderReaderMock) FindScheduledAfter(ctx context.Context, from time.Time) ([]*workorder.WorkOrder, error) {
	args := m.Called(ctx, from)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]*workorder.WorkOrder), args.Error(1)
}

type connectionReaderMock struct{ mock.Mock }

func (m *connectionReaderMock) FindByUserID(ctx context.Context, userID int) (*calendarconnection.Connection, error) {
	args := m.Called(ctx, userID)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*calendarconnection.Connection), args.Error(1)
}

type eventRepositoryMock struct{ mock.Mock }

func (m *eventRepositoryMock) Save(ctx context.Context, event *workordercalendar.Event) error {
	return m.Called(ctx, event).Error(0)
}

type eventPublisherMock struct{ mock.Mock }

func (m *eventPublisherMock) Create(
	ctx context.Context,
	connection *calendarconnection.Connection,
	appointment workordercalendar.Appointment,
) (workordercalendar.PublishedEvent, error) {
	args := m.Called(ctx, connection, appointment)
	return args.Get(0).(workordercalendar.PublishedEvent), args.Error(1)
}

type clockMock struct{ mock.Mock }

func (m *clockMock) Now() time.Time {
	return m.Called().Get(0).(time.Time)
}

func workOrderCalendarFixture(
	t *testing.T,
	now time.Time,
) (*workorder.WorkOrder, user.User, user.User) {
	t.Helper()

	consumerUser := &consumer.Consumer{BaseUser: user.RehydrateBaseUser(
		10,
		"",
		"ana@example.com",
		"Ana",
		"Pérez",
		consumer.Role,
		nil,
	)}
	providerUser := &provider.Provider{BaseUser: user.RehydrateBaseUser(
		20,
		"",
		"juan@example.com",
		"Juan",
		"Gómez",
		provider.Role,
		nil,
	)}
	proposal := &serviceproposal.ServiceProposal{
		ID:                       30,
		Consumer:                 consumerUser,
		Provider:                 providerUser,
		ScheduledOn:              now.Add(48 * time.Hour),
		Description:              "Repair the kitchen water leak.",
		EstimatedDurationMinutes: 90,
	}
	order, err := workorder.New(proposal, now)
	require.NoError(t, err)
	order.SetID(40)
	return order, consumerUser, providerUser
}

func calendarConnectionFixture(t *testing.T, userID int, now time.Time) *calendarconnection.Connection {
	t.Helper()
	connection, err := calendarconnection.NewConnection(userID, "primary", []byte("encrypted-token"), now)
	require.NoError(t, err)
	return connection
}

func publishedEventFixture(t *testing.T, externalID string) workordercalendar.PublishedEvent {
	t.Helper()
	published, err := workordercalendar.NewPublishedEvent("primary", externalID)
	require.NoError(t, err)
	return published
}
