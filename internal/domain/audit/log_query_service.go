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
func (service *LogQueryService) Query(ctx context.Context, authSubject, correlationID string, query LogQuery) (LogPage, error) {
	if err := query.Validate(); err != nil {
		return LogPage{}, err
	}
	actorID, err := service.operatorIDs.FindOperatorIDByAuthID(ctx, authSubject)
	if err != nil {
		return LogPage{}, fmt.Errorf("finding audit log reader: %w", err)
	}

	var watermark int64
	if query.Watermark == nil {
		watermark, err = service.reader.CaptureWatermark(ctx)
		if err != nil {
			return LogPage{}, fmt.Errorf("capturing audit log watermark: %w", err)
		}
	} else {
		watermark = *query.Watermark
	}
	limit := query.effectiveLimit()
	events, err := service.reader.FindPage(ctx, query.Filter, watermark, query.Before, limit+1)
	if err != nil {
		return LogPage{}, fmt.Errorf("reading audit log: %w", err)
	}
	page := LogPage{Events: events, Watermark: watermark}
	if page.Events == nil {
		page.Events = []*Event{}
	}
	if len(page.Events) > limit {
		page.Events = page.Events[:limit]
		last := page.Events[limit-1]
		page.Next = &LogPosition{OccurredOn: last.OccurredOn(), ID: last.ID()}
	}

	access, err := NewEvent(EventParams{
		ID: uuid.New(), OperatorID: actorID, Action: ActionAccess,
		ResourceType: "audit_log", OccurredOn: service.clock.Now(),
		Result: ResultPrepared, CorrelationID: correlationID,
	})
	if err != nil {
		return LogPage{}, fmt.Errorf("creating audit log access event: %w", err)
	}
	if err := service.writer.Save(ctx, access); err != nil {
		return LogPage{}, fmt.Errorf("saving audit log access event: %w", err)
	}
	return page, nil
}
