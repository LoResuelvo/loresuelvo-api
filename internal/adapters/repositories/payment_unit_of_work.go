package repositories

import (
	"context"
	"database/sql"
	"fmt"

	"github.com/LoResuelvo/loresuelvo-api/internal/domain/notification"
	"github.com/LoResuelvo/loresuelvo-api/internal/domain/payment"
	serviceproposal "github.com/LoResuelvo/loresuelvo-api/internal/domain/service_proposal"
	workorder "github.com/LoResuelvo/loresuelvo-api/internal/domain/work_order"
)

type PaymentUnitOfWork struct {
	db                     *sql.DB
	intentRepository       *PaymentIntentRepository
	transactionRepository  *PaymentTransactionRepository
	proposalRepository     *ServiceProposalRepository
	workOrderRepository    *WorkOrderRepository
	notificationRepository *NotificationRepository
}

func NewPaymentUnitOfWork(
	db *sql.DB,
	intentRepository *PaymentIntentRepository,
	transactionRepository *PaymentTransactionRepository,
	proposalRepository *ServiceProposalRepository,
	workOrderRepository *WorkOrderRepository,
	notificationRepository *NotificationRepository,
) *PaymentUnitOfWork {
	return &PaymentUnitOfWork{
		db:                     db,
		intentRepository:       intentRepository,
		transactionRepository:  transactionRepository,
		proposalRepository:     proposalRepository,
		workOrderRepository:    workOrderRepository,
		notificationRepository: notificationRepository,
	}
}

func (unit *PaymentUnitOfWork) Execute(
	ctx context.Context,
	operation func(payment.TransactionalStore) error,
) error {
	if operation == nil {
		return fmt.Errorf("executing payment unit of work: operation is required")
	}
	tx, err := unit.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("beginning payment unit of work: %w", err)
	}
	store := &paymentTransactionalStore{unit: unit, tx: tx}
	if err := operation(store); err != nil {
		return rollbackPaymentUnitOfWork(tx, err)
	}
	if err := tx.Commit(); err != nil {
		return fmt.Errorf("committing payment unit of work: %w", err)
	}
	return nil
}

type paymentTransactionalStore struct {
	unit *PaymentUnitOfWork
	tx   *sql.Tx
}

func (store *paymentTransactionalStore) SaveIntent(ctx context.Context, intent *payment.Intent) error {
	return store.unit.intentRepository.saveWithTx(ctx, store.tx, intent)
}

func (store *paymentTransactionalStore) SaveTransaction(ctx context.Context, transaction *payment.Transaction) error {
	return store.unit.transactionRepository.saveWithTx(ctx, store.tx, transaction)
}

func (store *paymentTransactionalStore) SaveServiceProposal(
	ctx context.Context,
	proposal *serviceproposal.ServiceProposal,
) error {
	return store.unit.proposalRepository.saveWithTx(ctx, store.tx, proposal)
}

func (store *paymentTransactionalStore) SaveWorkOrder(ctx context.Context, order *workorder.WorkOrder) error {
	saved, err := store.unit.workOrderRepository.saveWithTx(ctx, store.tx, order)
	if err != nil {
		return err
	}
	*order = *saved
	return nil
}

func (store *paymentTransactionalStore) SaveNotification(
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

func rollbackPaymentUnitOfWork(tx *sql.Tx, cause error) error {
	if rollbackErr := tx.Rollback(); rollbackErr != nil {
		return fmt.Errorf("%w: rolling back payment unit of work: %v", cause, rollbackErr)
	}
	return cause
}
