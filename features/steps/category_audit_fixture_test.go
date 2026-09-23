package steps_test

import (
	"context"
	"sync"

	"github.com/LoResuelvo/loresuelvo-api/internal/domain/audit"
	"github.com/LoResuelvo/loresuelvo-api/internal/domain/category"
	"github.com/google/uuid"
)

// categoryAuditEventCapture is a test-only decorator. It observes the audit
// event IDs written by a successful category unit of work without changing
// production behavior or reading the database directly.
type categoryAuditEventCapture struct {
	mu       sync.Mutex
	eventIDs []uuid.UUID
}

func (capture *categoryAuditEventCapture) decorate(unit category.UnitOfWork) category.UnitOfWork {
	return categoryAuditUnitOfWorkDecorator{delegate: unit, capture: capture}
}

func (capture *categoryAuditEventCapture) reset() {
	capture.mu.Lock()
	defer capture.mu.Unlock()
	capture.eventIDs = nil
}

func (capture *categoryAuditEventCapture) snapshot() []uuid.UUID {
	capture.mu.Lock()
	defer capture.mu.Unlock()
	return append([]uuid.UUID(nil), capture.eventIDs...)
}

type categoryAuditUnitOfWorkDecorator struct {
	delegate category.UnitOfWork
	capture  *categoryAuditEventCapture
}

func (decorator categoryAuditUnitOfWorkDecorator) Execute(ctx context.Context, operation func(category.TransactionalStore) error) error {
	var eventIDs []uuid.UUID
	err := decorator.delegate.Execute(ctx, func(store category.TransactionalStore) error {
		return operation(categoryAuditTransactionalStoreDecorator{
			delegate: store,
			onSaved: func(event *audit.Event) {
				eventIDs = append(eventIDs, event.ID())
			},
		})
	})
	if err != nil {
		return err
	}

	decorator.capture.mu.Lock()
	decorator.capture.eventIDs = append(decorator.capture.eventIDs, eventIDs...)
	decorator.capture.mu.Unlock()
	return nil
}

type categoryAuditTransactionalStoreDecorator struct {
	delegate category.TransactionalStore
	onSaved  func(*audit.Event)
}

func (decorator categoryAuditTransactionalStoreDecorator) SaveCategory(ctx context.Context, categoryToSave category.Category) (*category.Category, error) {
	return decorator.delegate.SaveCategory(ctx, categoryToSave)
}

func (decorator categoryAuditTransactionalStoreDecorator) SaveAuditEvent(ctx context.Context, event *audit.Event) error {
	if err := decorator.delegate.SaveAuditEvent(ctx, event); err != nil {
		return err
	}
	decorator.onSaved(event)
	return nil
}
