package steps_test

import (
	"context"
	"errors"
	"sync"

	"github.com/LoResuelvo/loresuelvo-api/internal/domain/audit"
	"github.com/LoResuelvo/loresuelvo-api/internal/domain/category"
	"github.com/google/uuid"
)

// categoryAuditEventCapture is a test-only decorator. It records committed
// event IDs and attempted event IDs without changing production behavior or
// reading the database directly.
type categoryAuditEventCapture struct {
	mu                   sync.Mutex
	eventIDs             []uuid.UUID
	attemptedEventIDs    []uuid.UUID
	saveCategoryAttempts int
	failure              categoryAuditFailure
}

type categoryAuditFailure uint8

const (
	categoryAuditFailureNone categoryAuditFailure = iota
	categoryAuditFailureSaveCategory
	categoryAuditFailureSaveAuditEvent
)

var errInjectedCategoryPersistenceFailure = errors.New("injected category persistence failure")

func (capture *categoryAuditEventCapture) decorate(unit category.UnitOfWork) category.UnitOfWork {
	return categoryAuditUnitOfWorkDecorator{delegate: unit, capture: capture}
}

func (capture *categoryAuditEventCapture) reset() {
	capture.mu.Lock()
	defer capture.mu.Unlock()
	capture.eventIDs = nil
	capture.attemptedEventIDs = nil
	capture.saveCategoryAttempts = 0
	capture.failure = categoryAuditFailureNone
}

func (capture *categoryAuditEventCapture) resetAttempt() {
	capture.mu.Lock()
	defer capture.mu.Unlock()
	capture.eventIDs = nil
	capture.attemptedEventIDs = nil
	capture.saveCategoryAttempts = 0
}

func (capture *categoryAuditEventCapture) fail(failure categoryAuditFailure) {
	capture.mu.Lock()
	defer capture.mu.Unlock()
	capture.failure = failure
}

func (capture *categoryAuditEventCapture) shouldFail(failure categoryAuditFailure) bool {
	capture.mu.Lock()
	defer capture.mu.Unlock()
	return capture.failure == failure
}

func (capture *categoryAuditEventCapture) failureMode() categoryAuditFailure {
	capture.mu.Lock()
	defer capture.mu.Unlock()
	return capture.failure
}

func (capture *categoryAuditEventCapture) recordAttempt(eventID uuid.UUID) {
	capture.mu.Lock()
	defer capture.mu.Unlock()
	capture.attemptedEventIDs = append(capture.attemptedEventIDs, eventID)
}

func (capture *categoryAuditEventCapture) recordSaveCategoryAttempt() {
	capture.mu.Lock()
	defer capture.mu.Unlock()
	capture.saveCategoryAttempts++
}

func (capture *categoryAuditEventCapture) saveCategoryAttemptCount() int {
	capture.mu.Lock()
	defer capture.mu.Unlock()
	return capture.saveCategoryAttempts
}

func (capture *categoryAuditEventCapture) snapshot() []uuid.UUID {
	capture.mu.Lock()
	defer capture.mu.Unlock()
	return append([]uuid.UUID(nil), capture.eventIDs...)
}

func (capture *categoryAuditEventCapture) attemptedSnapshot() []uuid.UUID {
	capture.mu.Lock()
	defer capture.mu.Unlock()
	return append([]uuid.UUID(nil), capture.attemptedEventIDs...)
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
			capture:  decorator.capture,
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
	capture  *categoryAuditEventCapture
	onSaved  func(*audit.Event)
}

func (decorator categoryAuditTransactionalStoreDecorator) SaveCategory(ctx context.Context, categoryToSave category.Category) (*category.Category, error) {
	decorator.capture.recordSaveCategoryAttempt()
	if decorator.capture.shouldFail(categoryAuditFailureSaveCategory) {
		return nil, errInjectedCategoryPersistenceFailure
	}
	return decorator.delegate.SaveCategory(ctx, categoryToSave)
}

func (decorator categoryAuditTransactionalStoreDecorator) SaveAuditEvent(ctx context.Context, event *audit.Event) error {
	decorator.capture.recordAttempt(event.ID())
	if decorator.capture.shouldFail(categoryAuditFailureSaveAuditEvent) {
		return errInjectedCategoryPersistenceFailure
	}
	if err := decorator.delegate.SaveAuditEvent(ctx, event); err != nil {
		return err
	}
	decorator.onSaved(event)
	return nil
}
