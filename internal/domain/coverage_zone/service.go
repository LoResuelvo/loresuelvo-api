package coveragezone

import (
	"context"
	"fmt"
)

const DefaultMarketCode = "CABA"

type Service struct {
	repository Repository
}

func NewService(repository Repository) *Service {
	return &Service{repository: repository}
}

func (s *Service) List(ctx context.Context) ([]CatalogEntry, error) {
	entries, err := s.repository.ListAvailableByMarketCode(ctx, DefaultMarketCode)
	if err != nil {
		return nil, fmt.Errorf("listing available coverage zones: %w", err)
	}
	if entries == nil {
		return []CatalogEntry{}, nil
	}

	return entries, nil
}
