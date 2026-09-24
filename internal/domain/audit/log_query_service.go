package audit

import (
	"context"
	"fmt"

	"github.com/LoResuelvo/loresuelvo-api/internal/domain/clock"
	"github.com/google/uuid"
)

const defaultLogQueryLimit = 20

// LogQueryService coordinates a read and its append-only access evidence.
type LogQueryService struct {
	reader      LogReader
	writer      Writer
	operatorIDs OperatorIDFinder
	clock       clock.Clock
}

func NewLogQueryService(reader LogReader, writer Writer, operatorIDs OperatorIDFinder, clock clock.Clock) *LogQueryService {
	return &LogQueryService{reader: reader, writer: writer, operatorIDs: operatorIDs, clock: clock}
}

// Query reads before appending access evidence, so this request never observes
// its own audit event. Nothing is returned unless the evidence is persisted.
func (service *LogQueryService) Query(ctx context.Context, authSubject, correlationID string, operatorID *int) ([]*Event, error) {
	actorID, err := service.operatorIDs.FindOperatorIDByAuthID(ctx, authSubject)
	if err != nil {
		return nil, fmt.Errorf("finding audit log reader: %w", err)
	}

	events, err := service.reader.FindLatest(ctx, operatorID, defaultLogQueryLimit)
	if err != nil {
		return nil, fmt.Errorf("reading audit log: %w", err)
	}

	access, err := NewEvent(EventParams{
		ID: uuid.New(), OperatorID: actorID, Action: ActionAccess,
		ResourceType: "audit_log", OccurredOn: service.clock.Now(),
		Result: ResultPrepared, CorrelationID: correlationID,
	})
	if err != nil {
		return nil, fmt.Errorf("creating audit log access event: %w", err)
	}
	if err := service.writer.Save(ctx, access); err != nil {
		return nil, fmt.Errorf("saving audit log access event: %w", err)
	}

	if events == nil {
		return []*Event{}, nil
	}
	return events, nil
}
