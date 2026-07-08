package repositories

import (
	"context"
	"database/sql"
	"errors"
	"fmt"

	serviceproposal "github.com/LoResuelvo/loresuelvo-api/internal/domain/service_proposal"
	workorder "github.com/LoResuelvo/loresuelvo-api/internal/domain/work_order"
)

type WorkOrderRepository struct {
	db                        *sql.DB
	serviceProposalRepository *ServiceProposalRepository
}

func NewWorkOrderRepository(db *sql.DB, serviceProposalRepository *ServiceProposalRepository) *WorkOrderRepository {
	return &WorkOrderRepository{db: db, serviceProposalRepository: serviceProposalRepository}
}

func (r *WorkOrderRepository) Save(ctx context.Context, order *workorder.WorkOrder) (*workorder.WorkOrder, error) {
	if order == nil {
		return nil, fmt.Errorf("saving work order: work order is required")
	}
	proposal, ok := order.ServiceProposal.(*serviceproposal.ServiceProposal)
	if !ok || proposal == nil {
		return nil, fmt.Errorf("saving work order: service proposal is required")
	}

	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return nil, fmt.Errorf("beginning work order transaction: %w", err)
	}

	if err := r.serviceProposalRepository.updateAcceptedWithTx(ctx, tx, proposal); err != nil {
		return nil, rollbackWorkOrderTx(tx, err)
	}

	savedOrder, err := r.saveWithTx(ctx, tx, order)
	if err != nil {
		return nil, rollbackWorkOrderTx(tx, err)
	}
	if err := tx.Commit(); err != nil {
		return nil, fmt.Errorf("committing work order transaction: %w", err)
	}
	return savedOrder, nil
}

func (r *WorkOrderRepository) FindByServiceProposalID(ctx context.Context, serviceProposalID int) (*workorder.WorkOrder, error) {
	var (
		order    workorder.WorkOrder
		proposal serviceproposal.ServiceProposal
	)
	err := r.db.QueryRowContext(
		ctx,
		`SELECT wo.id, wo.service_proposal_id, wo.status, wo.accepted_on
		FROM work_orders wo
		WHERE wo.service_proposal_id = $1`,
		serviceProposalID,
	).Scan(
		&order.ID,
		&proposal.ID,
		&order.Status,
		&order.AcceptedOn,
	)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, workorder.ErrDoesNotExist
	}
	if err != nil {
		return nil, fmt.Errorf("finding work order by service proposal id: %w", err)
	}
	foundProposal, err := r.serviceProposalRepository.FindByID(ctx, proposal.ID)
	if err != nil {
		return nil, fmt.Errorf("hydrating work order service proposal: %w", err)
	}
	order.ServiceProposal = foundProposal

	return &order, nil
}

func (r *WorkOrderRepository) saveWithTx(ctx context.Context, tx *sql.Tx, order *workorder.WorkOrder) (*workorder.WorkOrder, error) {
	if tx == nil {
		return nil, fmt.Errorf("saving work order: transaction is required")
	}
	if order == nil {
		return nil, fmt.Errorf("saving work order: work order is required")
	}
	if order.ServiceProposal == nil {
		return nil, fmt.Errorf("saving work order: service proposal is required")
	}

	saved := *order
	err := tx.QueryRowContext(
		ctx,
		`INSERT INTO work_orders (
			service_proposal_id,
			status,
			accepted_on,
			updated_on
		)
		VALUES ($1, $2, $3, NOW())
		RETURNING id`,
		order.ServiceProposal.ServiceProposalID(),
		order.Status,
		order.AcceptedOn,
	).Scan(&saved.ID)
	if err != nil {
		return nil, fmt.Errorf("saving work order: %w", err)
	}
	return &saved, nil
}

func rollbackWorkOrderTx(tx *sql.Tx, cause error) error {
	if rollbackErr := tx.Rollback(); rollbackErr != nil {
		return fmt.Errorf("%w: rolling back work order transaction: %v", cause, rollbackErr)
	}
	return cause
}
