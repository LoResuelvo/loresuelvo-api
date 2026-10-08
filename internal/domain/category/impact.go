package category

import (
	"context"
	"time"
)

// Impact describes operational activity observed at one coherent database snapshot.
type Impact struct {
	Category   Category
	ObservedAt time.Time
	Counts     ImpactCounts
}

type ImpactCounts struct {
	AssignedProviders               int
	PendingRequests                 int
	AcceptedRequestsWithoutProposal int
	PendingProposals                int
	ScheduledOrders                 int
	AwaitingPaymentOrders           int
}

func (impact Impact) HasOngoingOrders() bool {
	return impact.Counts.ScheduledOrders > 0 || impact.Counts.AwaitingPaymentOrders > 0
}

func (impact Impact) RequiresConfirmation() bool {
	return impact.Category.Enabled && impact.HasOngoingOrders()
}

type ImpactReader interface {
	FindByCategoryID(ctx context.Context, id int, observedAt time.Time) (*Impact, error)
}
