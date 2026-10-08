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
	impactReader       ImpactReader
}

func NewService(categoryRepository Repository, unitOfWork UnitOfWork, operatorIDs OperatorIDFinder, clock clock.Clock, impactReader ImpactReader) *Service {
	return &Service{categoryRepository: categoryRepository, unitOfWork: unitOfWork, operatorIDs: operatorIDs, clock: clock, impactReader: impactReader}
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

	enabled := make([]Category, 0, len(categories))
	for _, item := range categories {
		if item.Enabled {
			enabled = append(enabled, item)
		}
	}
	return enabled, nil
}

func (s *Service) ListAllCategories() ([]Category, error) {
	categories, err := s.categoryRepository.ListAll()
	if err != nil {
		return nil, fmt.Errorf("listing administrative categories: %w", err)
	}
	return categories, nil
}

func (s *Service) GetImpact(ctx context.Context, id int) (*Impact, error) {
	if id <= 0 {
		return nil, ErrIDRequired
	}
	return s.impactReader.FindByCategoryID(ctx, id, s.clock.Now().UTC())
}

func (s *Service) EditCategory(ctx context.Context, id int, edit Edit, authSubject, correlationID string) (*Category, error) {
	if id <= 0 {
		return nil, ErrIDRequired
	}
	if err := edit.Validate(); err != nil {
		return nil, err
	}
	operatorID, err := s.operatorIDs.FindOperatorIDByAuthID(ctx, authSubject)
	if err != nil {
		return nil, fmt.Errorf("finding category editing operator: %w", err)
	}
	var saved *Category
	err = s.unitOfWork.Execute(ctx, func(store TransactionalStore) error {
		current, err := store.FindCategory(ctx, id)
		if err != nil {
			return err
		}
		previous := *current
		ongoing := false
		if edit.Enabled != nil && !*edit.Enabled && current.Enabled {
			impact, err := store.FindImpact(ctx, id, s.clock.Now().UTC())
			if err != nil {
				return err
			}
			ongoing = impact.HasOngoingOrders()
		}
		changed, err := current.Edit(edit, ongoing)
		if err != nil {
			return err
		}
		if !changed {
			saved = current
			return nil
		}
		saved, err = store.SaveCategory(ctx, *current)
		if err != nil {
			return fmt.Errorf("saving edited category: %w", err)
		}
		var reason *audit.Reason
		if edit.Reason != "" {
			reason, err = audit.NewReason(edit.Reason)
			if err != nil {
				return err
			}
		}
		var stateChange *audit.StateChange
		if previous.Enabled != current.Enabled {
			state := func(enabled bool) string {
				if enabled {
					return "enabled"
				}
				return "disabled"
			}
			stateChange, err = audit.NewStateChange("enabled", state(previous.Enabled), state(current.Enabled))
			if err != nil {
				return err
			}
		}
		event, err := audit.NewEvent(audit.EventParams{ID: uuid.New(), OperatorID: operatorID,
			Action: audit.ActionExecute, ResourceType: "category", ResourceID: strconv.Itoa(id),
			OccurredOn: s.clock.Now(), Result: audit.ResultSucceeded, CorrelationID: correlationID,
			Reason: reason, StateChange: stateChange})
		if err != nil {
			return err
		}
		return store.SaveAuditEvent(ctx, event)
	})
	if err != nil {
		return nil, err
	}
	return saved, nil
}
