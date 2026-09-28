package admin

import (
	"context"
	"fmt"
	readmodel "github.com/LoResuelvo/loresuelvo-api/internal/domain/admin/read_model"
	"github.com/LoResuelvo/loresuelvo-api/internal/domain/audit"
	"github.com/LoResuelvo/loresuelvo-api/internal/domain/clock"
	"github.com/google/uuid"
	"strconv"
)

type ConsumerHistoryReader interface {
	FindByConsumerID(context.Context, int, ConsumerHistoryQuery) (*readmodel.ConsumerHistory, error)
}
type ConsumerHistoryService struct {
	reader    ConsumerHistoryReader
	photos    ProfilePhotoURLResolver
	operators audit.OperatorIDFinder
	writer    audit.Writer
	clock     clock.Clock
}

func NewConsumerHistoryService(reader ConsumerHistoryReader, photos ProfilePhotoURLResolver, operators audit.OperatorIDFinder, writer audit.Writer, clock clock.Clock) *ConsumerHistoryService {
	return &ConsumerHistoryService{reader, photos, operators, writer, clock}
}
func (s *ConsumerHistoryService) Query(ctx context.Context, id int, q ConsumerHistoryQuery, subject, correlation string) (*readmodel.ConsumerHistory, error) {
	if id < 1 || int64(id) > 2147483647 {
		return nil, ErrInvalidConsumerHistoryQuery
	}
	if err := q.Validate(); err != nil {
		return nil, err
	}
	h, err := s.reader.FindByConsumerID(ctx, id, q)
	if err != nil {
		return nil, fmt.Errorf("reading consumer history: %w", err)
	}
	if h == nil {
		return nil, ErrConsumerHistoryNotFound
	}
	if h.Consumer.ProfilePhotoFileID != "" {
		urls, err := s.photos.ResolvePublicURLs(ctx, []string{h.Consumer.ProfilePhotoFileID})
		if err != nil {
			return nil, fmt.Errorf("resolving consumer history photo: %w", err)
		}
		h.Consumer.ProfilePhotoURL = urls[h.Consumer.ProfilePhotoFileID]
	}
	operator, err := s.operators.FindOperatorIDByAuthID(ctx, subject)
	if err != nil {
		return nil, fmt.Errorf("resolving consumer history operator: %w", err)
	}
	event, err := audit.NewEvent(audit.EventParams{ID: uuid.New(), OperatorID: operator, Action: audit.ActionAccess, ResourceType: "consumer", ResourceID: strconv.Itoa(id), OccurredOn: s.clock.Now(), Result: audit.ResultPrepared, CorrelationID: correlation})
	if err != nil {
		return nil, fmt.Errorf("preparing consumer history access: %w", err)
	}
	if err := s.writer.Save(ctx, event); err != nil {
		return nil, fmt.Errorf("saving consumer history access: %w", err)
	}
	return h, nil
}
