package repositories

import (
	"context"
	"database/sql"
	"errors"
	"fmt"

	workorder "github.com/LoResuelvo/loresuelvo-api/internal/domain/work_order"
)

type WorkOrderRepository struct {
	db *sql.DB
}

func NewWorkOrderRepository(db *sql.DB) *WorkOrderRepository {
	return &WorkOrderRepository{db: db}
}

func (r *WorkOrderRepository) FindByServiceProposalID(ctx context.Context, serviceProposalID int) (*workorder.WorkOrder, error) {
	var order workorder.WorkOrder
	err := r.db.QueryRowContext(
		ctx,
		`SELECT id, service_proposal_id, consumer_id, provider_id, status, accepted_on
		FROM work_orders
		WHERE service_proposal_id = $1`,
		serviceProposalID,
	).Scan(
		&order.ID,
		&order.ServiceProposalID,
		&order.ConsumerID,
		&order.ProviderID,
		&order.Status,
		&order.AcceptedOn,
	)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, workorder.ErrDoesNotExist
	}
	if err != nil {
		return nil, fmt.Errorf("finding work order by service proposal id: %w", err)
	}
	return &order, nil
}

func (r *WorkOrderRepository) saveWithTx(ctx context.Context, tx *sql.Tx, order *workorder.WorkOrder) (*workorder.WorkOrder, error) {
	if tx == nil {
		return nil, fmt.Errorf("saving work order: transaction is required")
	}
	if order == nil {
		return nil, fmt.Errorf("saving work order: work order is required")
	}

	saved := *order
	err := tx.QueryRowContext(
		ctx,
		`INSERT INTO work_orders (
			service_proposal_id,
			consumer_id,
			provider_id,
			status,
			accepted_on,
			updated_on
		)
		VALUES ($1, $2, $3, $4, $5, NOW())
		RETURNING id`,
		order.ServiceProposalID,
		order.ConsumerID,
		order.ProviderID,
		order.Status,
		order.AcceptedOn,
	).Scan(&saved.ID)
	if err != nil {
		return nil, fmt.Errorf("saving work order: %w", err)
	}
	return &saved, nil
}
