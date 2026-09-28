package readmodel

import (
	"github.com/LoResuelvo/loresuelvo-api/internal/domain/provider"
	"time"
)

const DiagnosticReferenceLimit = 6

type ProviderActivity struct {
	Type       string
	ID         int
	Status     string
	OccurredOn time.Time
}
type ProviderReview struct {
	WorkOrderID int
	Rating      int
	Description string
}
type ProviderOrderSync struct {
	WorkOrderID int
	SyncedOn    *time.Time
}

func (d ProviderDiagnostic) Reputation() provider.RatingSummary { return d.RatingStats.Summary() }
