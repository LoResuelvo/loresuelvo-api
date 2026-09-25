package operation

import (
	"context"
	"fmt"
	"time"

	"github.com/LoResuelvo/loresuelvo-api/internal/domain/clock"
	readmodel "github.com/LoResuelvo/loresuelvo-api/internal/domain/operation/read_model"
)

const (
	// RequestResponseWindow is how long a provider may leave a job request
	// unanswered before it raises request_pending_over_24h.
	RequestResponseWindow = 24 * time.Hour
	// StallThreshold is how old the last business advance of an operation
	// awaiting a party may be before it raises stalled.
	StallThreshold = 72 * time.Hour
)

// InboxService serves the administrative operations inbox. Ordinary inbox
// reads are not audited.
type InboxService struct {
	reader InboxReader
	clock  clock.Clock
}

func NewInboxService(reader InboxReader, clock clock.Clock) *InboxService {
	return &InboxService{reader: reader, clock: clock}
}

func (service *InboxService) Query(ctx context.Context, query InboxQuery) (InboxPage, error) {
	if err := query.Validate(); err != nil {
		return InboxPage{}, err
	}
	limit := query.effectiveLimit()
	now := service.clock.Now().UTC()
	operations, err := service.reader.FindPage(ctx, InboxCriteria{
		Now:                  now,
		PendingRequestCutoff: now.Add(-RequestResponseWindow),
		StalledCutoff:        now.Add(-StallThreshold),
		Filter:               query.Filter,
		After:                query.After,
		Limit:                limit + 1,
	})
	if err != nil {
		return InboxPage{}, fmt.Errorf("reading operations inbox: %w", err)
	}
	page := InboxPage{Operations: operations}
	if page.Operations == nil {
		page.Operations = []readmodel.OperationSummary{}
	}
	if len(page.Operations) > limit {
		page.Operations = page.Operations[:limit]
		last := page.Operations[limit-1]
		page.Next = &InboxPosition{StartedOn: last.StartedOn, ID: last.ID}
	}
	for index := range page.Operations {
		page.Operations[index].NextActionOwner = nextActionOwner(page.Operations[index])
		page.Operations[index].Limitations = limitations(page.Operations[index])
	}
	return page, nil
}
