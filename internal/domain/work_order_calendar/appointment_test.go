package workordercalendar_test

import (
	"testing"
	"time"

	workordercalendar "github.com/LoResuelvo/loresuelvo-api/internal/domain/work_order_calendar"
	"github.com/stretchr/testify/assert"
)

func TestAppointmentKeepsTheWorkOrderAndParticipants(t *testing.T) {
	now := time.Date(2026, time.July, 4, 10, 0, 0, 0, time.UTC)
	order, consumerUser, providerUser := workOrderCalendarFixture(t, now)

	appointment := workordercalendar.NewAppointment(order, consumerUser, providerUser)

	assert.Same(t, order, appointment.WorkOrder())
	assert.Equal(t, consumerUser, appointment.Participant())
	assert.Equal(t, providerUser, appointment.Counterpart())
}

func TestAppointmentDescribesTheCounterpartAndWork(t *testing.T) {
	now := time.Date(2026, time.July, 4, 10, 0, 0, 0, time.UTC)
	order, consumerUser, providerUser := workOrderCalendarFixture(t, now)

	appointment := workordercalendar.NewAppointment(order, consumerUser, providerUser)

	assert.Equal(t, "Juan Gómez", appointment.CounterpartName())
	assert.Equal(t, order.Description(), appointment.Description())
}

func TestAppointmentDerivesItsScheduleFromTheWorkOrder(t *testing.T) {
	now := time.Date(2026, time.July, 4, 10, 0, 0, 0, time.UTC)
	order, consumerUser, providerUser := workOrderCalendarFixture(t, now)
	appointment := workordercalendar.NewAppointment(order, consumerUser, providerUser)

	assert.Equal(t, order.ScheduledOn(), appointment.ScheduledOn())
	assert.Equal(t, order.EstimatedDurationMinutes(), appointment.DurationMinutes())
	assert.Equal(t, order.ScheduledOn().Add(90*time.Minute), appointment.EndsOn())
}
