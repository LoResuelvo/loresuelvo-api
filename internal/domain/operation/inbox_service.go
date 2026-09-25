package operation

import (
	"context"
	"fmt"

	readmodel "github.com/LoResuelvo/loresuelvo-api/internal/domain/operation/read_model"
)

// InboxService serves the administrative operations inbox. Ordinary inbox
// reads are not audited.
type InboxService struct {
	reader InboxReader
}

func NewInboxService(reader InboxReader) *InboxService {
	return &InboxService{reader: reader}
}

func (service *InboxService) Query(ctx context.Context, query InboxQuery) (InboxPage, error) {
	if err := query.Validate(); err != nil {
		return InboxPage{}, err
	}
	limit := query.effectiveLimit()
	operations, err := service.reader.FindPage(ctx, query.After, limit+1)
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
	return page, nil
}
