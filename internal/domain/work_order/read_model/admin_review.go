package readmodel

import (
	"time"
)

type AdminReviewSummary struct {
	WorkOrderID    int
	ConsumerID     int
	ProviderID     int
	ConsumerName   string
	ProviderName   string
	OperationID    string
	Rating         int
	Visible        bool
	Version        int
	PendingReports int
	ReportedOn     *time.Time
}
