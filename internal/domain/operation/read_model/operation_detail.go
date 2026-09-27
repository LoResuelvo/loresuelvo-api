package readmodel

import "time"

// OperationDetail is a persisted administrative read model, not an HTTP representation.
type OperationDetail struct {
	ID               ID
	StartedOn        time.Time
	JobRequest       *DetailJobRequest
	ServiceProposal  *DetailProposal
	Consumer         Party
	Provider         Party
	Category         *Category
	Address          *CurrentConsumerAddress
	SourceAssessment *SourceAssessment
}

type DetailJobRequest struct {
	ID          int
	Status      string
	Title       string
	Description string
	CreatedOn   time.Time
}

type DetailProposal struct {
	ID        int
	Status    string
	CreatedOn time.Time
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
