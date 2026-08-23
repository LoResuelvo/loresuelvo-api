package workordercalendar

import (
	"fmt"
	"strings"
	"time"

	"github.com/LoResuelvo/loresuelvo-api/internal/domain/user"
	workorder "github.com/LoResuelvo/loresuelvo-api/internal/domain/work_order"
)

// Appointment is the domain representation of a work order scheduled for one
// participant. The participant and its counterpart are kept as collaborators
// so adapters can render the event without reconstructing users from IDs.
type Appointment struct {
	workOrder   *workorder.WorkOrder
	participant user.User
	counterpart user.User
}

func NewAppointment(order *workorder.WorkOrder, participant, counterpart user.User) Appointment {
	return Appointment{
		workOrder:   order,
		participant: participant,
		counterpart: counterpart,
	}
}

func (appointment Appointment) WorkOrder() *workorder.WorkOrder {
	return appointment.workOrder
}

func (appointment Appointment) Participant() user.User {
	return appointment.participant
}

func (appointment Appointment) Counterpart() user.User {
	return appointment.counterpart
}

func (appointment Appointment) CounterpartName() string {
	return strings.TrimSpace(fmt.Sprintf("%s %s", appointment.counterpart.Name(), appointment.counterpart.Surname()))
}

func (appointment Appointment) Description() string {
	return appointment.workOrder.Description()
}

func (appointment Appointment) ScheduledOn() time.Time {
	return appointment.workOrder.ScheduledOn().UTC()
}

func (appointment Appointment) DurationMinutes() int {
	return appointment.workOrder.EstimatedDurationMinutes()
}

func (appointment Appointment) EndsOn() time.Time {
	return appointment.ScheduledOn().Add(time.Duration(appointment.DurationMinutes()) * time.Minute)
}
