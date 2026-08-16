package repositories

import (
	"context"
	"database/sql"
	"fmt"

	"github.com/LoResuelvo/loresuelvo-api/internal/domain/notification"
	workorder "github.com/LoResuelvo/loresuelvo-api/internal/domain/work_order"
)

type WorkOrderUnitOfWork struct {
	db                     *sql.DB
	workOrderRepository    *WorkOrderRepository
	notificationRepository *NotificationRepository
}

func NewWorkOrderUnitOfWork(
	db *sql.DB,
	workOrderRepository *WorkOrderRepository,
	notificationRepository *NotificationRepository,
) *WorkOrderUnitOfWork {
	return &WorkOrderUnitOfWork{
		db:                     db,
		workOrderRepository:    workOrderRepository,
		notificationRepository: notificationRepository,
	}
}

func (unit *WorkOrderUnitOfWork) Execute(
	ctx context.Context,
	operation func(workorder.TransactionalStore) error,
) error {
	if operation == nil {
		return fmt.Errorf("executing work order unit of work: operation is required")
	}

	tx, err := unit.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("beginning work order unit of work: %w", err)
	}

	store := &workOrderTransactionalStore{unit: unit, tx: tx}
	if err := operation(store); err != nil {
		return rollbackWorkOrderUnitOfWork(tx, err)
	}
	if err := tx.Commit(); err != nil {
		return fmt.Errorf("committing work order unit of work: %w", err)
	}
	return nil
}

type workOrderTransactionalStore struct {
	unit *WorkOrderUnitOfWork
	tx   *sql.Tx
}

func (store *workOrderTransactionalStore) SaveWorkOrder(ctx context.Context, order *workorder.WorkOrder) error {
	saved, err := store.unit.workOrderRepository.saveWithTx(ctx, store.tx, order)
	if err != nil {
		return err
	}
	*order = *saved
	return nil
}

func (store *workOrderTransactionalStore) SaveNotification(
	ctx context.Context,
	event *notification.Notification,
) error {
	saved, err := store.unit.notificationRepository.saveWithTx(ctx, store.tx, event)
	if err != nil {
		return err
	}
	*event = *saved
	return nil
}

func rollbackWorkOrderUnitOfWork(tx *sql.Tx, cause error) error {
	if rollbackErr := tx.Rollback(); rollbackErr != nil {
		return fmt.Errorf("%w: rolling back work order unit of work: %v", cause, rollbackErr)
	}
	return cause
}
