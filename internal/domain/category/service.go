package category

import (
	"context"
	"fmt"
	"strconv"

	"github.com/LoResuelvo/loresuelvo-api/internal/domain/audit"
	"github.com/LoResuelvo/loresuelvo-api/internal/domain/clock"
	"github.com/google/uuid"
)

type Service struct {
	categoryRepository Repository
	unitOfWork         UnitOfWork
	operatorIDs        OperatorIDFinder
	clock              clock.Clock
}

func NewService(categoryRepository Repository, unitOfWork UnitOfWork, operatorIDs OperatorIDFinder, clock clock.Clock) *Service {
	return &Service{categoryRepository: categoryRepository, unitOfWork: unitOfWork, operatorIDs: operatorIDs, clock: clock}
}

func (s *Service) CreateCategory(ctx context.Context, name, authSubject, correlationID string) (*Category, error) {
	category, err := New(name)
	if err != nil {
		return nil, err
	}

	operatorID, err := s.operatorIDs.FindOperatorIDByAuthID(ctx, authSubject)
	if err != nil {
		return nil, fmt.Errorf("finding category creation operator: %w", err)
	}
	var savedCategory *Category
	err = s.unitOfWork.Execute(ctx, func(store TransactionalStore) error {
		var saveErr error
		savedCategory, saveErr = store.SaveCategory(ctx, *category)
		if saveErr != nil {
			return fmt.Errorf("saving category: %w", saveErr)
		}
		event, eventErr := audit.NewEvent(audit.EventParams{
			ID: uuid.New(), OperatorID: operatorID, Action: audit.ActionCreate,
			ResourceType: "category", ResourceID: strconv.Itoa(savedCategory.ID),
			OccurredOn: s.clock.Now(), Result: audit.ResultSucceeded,
			CorrelationID: correlationID,
		})
		if eventErr != nil {
			return fmt.Errorf("creating category audit event: %w", eventErr)
		}
		if saveErr := store.SaveAuditEvent(ctx, event); saveErr != nil {
			return fmt.Errorf("saving category audit event: %w", saveErr)
		}
		return nil
	})
	if err != nil {
		return nil, err
	}
	return savedCategory, nil
}

func (s *Service) ListCategories() ([]Category, error) {
	categories, err := s.categoryRepository.ListAll()
	if err != nil {
		return nil, fmt.Errorf("listing categories: %w", err)
	}

	return categories, nil
}
