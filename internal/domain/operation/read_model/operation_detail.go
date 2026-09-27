package readmodel

import "time"

// OperationDetail is a persisted administrative read model, not an HTTP representation.
type OperationDetail struct {
	ID                ID
	StartedOn         time.Time
	JobRequest        *DetailJobRequest
	ServiceProposal   *DetailProposal
	RelatedProposals  []RelatedProposal
	WorkOrder         *DetailWorkOrder
	PaymentMilestones []PaymentMilestone
	Timeline          []TimelineEvent
	Consumer          Party
	Provider          Party
	Category          *Category
	Address           *CurrentConsumerAddress
	SourceAssessment  *SourceAssessment
}

type DetailJobRequest struct {
	ID          int
	Status      string
	Title       string
	Description string
	CreatedOn   time.Time
	Images      []PrivateImage
}

// PrivateImage describes persisted media without exposing its storage location.
type PrivateImage struct {
	ID           string
	OriginalName string
	MimeType     string
	Purpose      string
	CreatedOn    time.Time
}

type DetailProposal struct {
	ID                       int
	Status                   string
	Description              string
	AmountCents              int64
	Currency                 string
	CreatedOn                time.Time
	ScheduledOn              time.Time
	EstimatedDurationMinutes int
	DepositCents             int64
	PlatformFeeTotalCents    int64
	PlatformFeeDueNowCents   int64
}

// ServiceBalanceCents and PlatformFeeBalanceCents derive the remaining terms
// from this proposal's persisted contract values, never from a sibling proposal.
func (proposal DetailProposal) ServiceBalanceCents() int64 {
	return proposal.AmountCents - proposal.DepositCents
}

func (proposal DetailProposal) PlatformFeeBalanceCents() int64 {
	return proposal.PlatformFeeTotalCents - proposal.PlatformFeeDueNowCents
}

type RelatedProposal struct {
	OperationID ID
	Proposal    DetailProposal
}

type DetailWorkOrder struct {
	ID                   int
	Status               string
	AcceptedOn           time.Time
	CompletionReportedOn *time.Time
	BalancePaidOn        *time.Time
	CompletionReport     *CompletionReport
	Review               *WorkOrderReview
}

type CompletionReport struct {
	Description string
	ReportedOn  time.Time
	Images      []PrivateImage
}

type WorkOrderReview struct {
	Rating      int
	Description string
}

type PaymentMilestone struct {
	ID        string
	Purpose   string
	Status    string
	CreatedOn time.Time
}

type TimelineEvent struct {
	Type       string
	SourceType string
	SourceID   string
	OccurredOn time.Time
}

type CurrentConsumerAddress struct {
	Street       string
	StreetNumber string
	Floor        *string
	Unit         *string
}

type SourceAssessment struct {
	ID               int
	Version          int
	Outcome          string
	Category         *Category
	Title            string
	Description      string
	BasedOnMessageID int
	CreatedOn        time.Time
}
