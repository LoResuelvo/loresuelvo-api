package readmodel

import (
	operationmodel "github.com/LoResuelvo/loresuelvo-api/internal/domain/operation/read_model"
	"time"
)

// ClaimSummary is an owned-list projection; sensitive testimony and evidence are deliberately absent.
type ClaimSummary struct {
	ID              int
	OperationID     operationmodel.ID
	ReferenceKind   string
	ReferenceID     string
	Reason          string
	Status          string
	CreatedOn       time.Time
	ReviewStartedOn *time.Time
	ClosedOn        *time.Time
}
