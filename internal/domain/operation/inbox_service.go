package operation

import (
	"context"
	"fmt"

	"github.com/LoResuelvo/loresuelvo-api/internal/domain/clock"
	readmodel "github.com/LoResuelvo/loresuelvo-api/internal/domain/operation/read_model"
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
	operations, err := service.reader.FindPage(ctx, InboxCriteria{
		Now: service.clock.Now().UTC(), After: query.After, Limit: limit + 1,
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
	}
	return page, nil
}
