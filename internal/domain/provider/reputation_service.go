package provider

import (
	"context"
	"errors"
	"fmt"

	"github.com/LoResuelvo/loresuelvo-api/internal/domain/clock"
	readmodel "github.com/LoResuelvo/loresuelvo-api/internal/domain/provider/read_model"
	"github.com/LoResuelvo/loresuelvo-api/internal/domain/user"
)

var ErrInvalidReputationQuery = errors.New("invalid reputation query")
var ErrReputationForbidden = errors.New("reputation is only available to providers")
var ErrReputationProviderNotFound = errors.New("reputation provider not found")

const DefaultReputationPageSize = 20
const MaxReputationPageSize = 100

type ReputationQueryInput struct {
	Limit            int
	After            *readmodel.ReputationPosition
	CursorProviderID int
}

type ReputationQuery struct {
	Limit int
	After *readmodel.ReputationPosition
}

type ReputationReader interface {
	Read(context.Context, int, ReputationQuery) (*readmodel.ReputationSnapshot, error)
}

type ReputationService struct {
	reader ReputationReader
	actors ProviderActorFinder
	clock  clock.Clock
}

func NewReputationService(reader ReputationReader, actors ProviderActorFinder, clock clock.Clock) *ReputationService {
	return &ReputationService{reader: reader, actors: actors, clock: clock}
}

func (s *ReputationService) Query(ctx context.Context, authID string, input ReputationQueryInput) (*readmodel.Reputation, int, error) {
	limit := input.Limit
	if limit == 0 {
		limit = DefaultReputationPageSize
	}
	if limit < 1 || limit > MaxReputationPageSize || input.CursorProviderID < 0 ||
		(input.After == nil && input.CursorProviderID != 0) ||
		(input.After != nil && (input.After.WorkOrderID <= 0 || input.CursorProviderID <= 0)) {
		return nil, 0, ErrInvalidReputationQuery
	}
	now := s.clock.Now().UTC()
	providerID, role, err := s.actors.FindByAuthID(ctx, authID)
	if errors.Is(err, user.ErrNotFound) {
		return nil, 0, ErrReputationProviderNotFound
	}
	if err != nil {
		return nil, 0, fmt.Errorf("resolving reputation provider: %w", err)
	}
	if role != Role {
		return nil, 0, ErrReputationForbidden
	}
	if providerID <= 0 {
		return nil, 0, ErrReputationProviderNotFound
	}
	if input.CursorProviderID != 0 && input.CursorProviderID != providerID {
		return nil, 0, ErrInvalidReputationQuery
	}
	snapshot, err := s.reader.Read(ctx, providerID, ReputationQuery{Limit: limit, After: input.After})
	if err != nil {
		return nil, 0, fmt.Errorf("reading provider reputation: %w", err)
	}
	if snapshot == nil {
		return nil, 0, errors.New("reputation reader returned incomplete snapshot")
	}
	afterID := 0
	if input.After != nil {
		afterID = input.After.WorkOrderID
	}
	reputation, err := snapshot.Calculate(now, limit, afterID)
	if err != nil {
		return nil, 0, fmt.Errorf("calculating provider reputation: %w", err)
	}
	return reputation, providerID, nil
}
