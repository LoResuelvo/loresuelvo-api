package readmodel

import (
	operationmodel "github.com/LoResuelvo/loresuelvo-api/internal/domain/operation/read_model"
	"time"
)

type ConsumerHistory struct {
	Consumer     Consumer
	Address      *ConsumerHistoryAddress
	CoverageZone *ConsumerHistoryZone
	Summary      ConsumerHistorySummary
	Items        []ConsumerHistoryItem
	HasMore      bool
}
type ConsumerHistoryAddress struct {
	Street, StreetNumber string
	Floor, Unit          *string
}
type ConsumerHistoryZone struct {
	ID      int
	Name    string
	Enabled bool
}
type ConsumerHistorySummary struct{ JobRequests, ServiceProposals, WorkOrders int }
type ConsumerHistoryPosition struct {
	OccurredOn time.Time
	Type       string
	ID         int
}
type ConsumerHistoryItem struct {
	Type                                string
	ID                                  int
	Status                              string
	Provider                            operationmodel.Party
	OccurredOn                          time.Time
	Operation                           operationmodel.ID
	JobRequestID                        *int
	ServiceProposalID                   int
	CreatedOn                           time.Time
	ScheduledOn                         time.Time
	EstimatedDurationMinutes            int
	BookingPaymentDeadline              time.Time
	AcceptedOn                          time.Time
	CompletionReportedOn, BalancePaidOn *time.Time
}

func (i ConsumerHistoryItem) Position() ConsumerHistoryPosition {
	return ConsumerHistoryPosition{OccurredOn: i.OccurredOn, Type: i.Type, ID: i.ID}
}
