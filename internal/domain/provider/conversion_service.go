package provider

import (
	"context"
	"errors"
	"fmt"

	"github.com/LoResuelvo/loresuelvo-api/internal/domain/clock"
	readmodel "github.com/LoResuelvo/loresuelvo-api/internal/domain/provider/read_model"
	"github.com/LoResuelvo/loresuelvo-api/internal/domain/user"
)

var ErrConversionForbidden = errors.New("conversion is only available to providers")
var ErrConversionProviderNotFound = errors.New("conversion provider not found")

type ConversionReader interface {
	Read(context.Context, int, ConversionQuery) (*readmodel.ConversionSnapshot, error)
}
type ConversionService struct {
	reader ConversionReader
	actors ProviderActorFinder
	clock  clock.Clock
}

func NewConversionService(reader ConversionReader, actors ProviderActorFinder, clock clock.Clock) *ConversionService {
	return &ConversionService{reader, actors, clock}
}
func (s *ConversionService) Query(ctx context.Context, authID string, input ConversionQueryInput) (*readmodel.Conversion, error) {
	if s.clock == nil || s.actors == nil || s.reader == nil {
		return nil, errors.New("conversion service dependencies are unavailable")
	}
	now := s.clock.Now().UTC()
	query, err := input.resolve(now)
	if err != nil {
		return nil, err
	}
	providerID, role, err := s.actors.FindByAuthID(ctx, authID)
	if errors.Is(err, user.ErrNotFound) {
		return nil, ErrConversionProviderNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("resolving conversion provider: %w", err)
	}
	if role != Role {
		return nil, ErrConversionForbidden
	}
	if providerID <= 0 {
		return nil, ErrConversionProviderNotFound
	}
	snapshot, err := s.reader.Read(ctx, providerID, query)
	if err != nil {
		return nil, fmt.Errorf("reading provider conversion: %w", err)
	}
	if snapshot == nil {
		return nil, errors.New("conversion reader returned incomplete snapshot")
	}
	return snapshot.Calculate(readmodel.ConversionPeriod{From: query.From, To: query.To}, now)
}
