package repositories

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"

	"github.com/LoResuelvo/loresuelvo-api/internal/domain/consumer"
	"github.com/LoResuelvo/loresuelvo-api/internal/domain/provider"
	serviceproposal "github.com/LoResuelvo/loresuelvo-api/internal/domain/service_proposal"
	"github.com/LoResuelvo/loresuelvo-api/internal/domain/user"
	workorder "github.com/LoResuelvo/loresuelvo-api/internal/domain/work_order"
	readmodel "github.com/LoResuelvo/loresuelvo-api/internal/domain/work_order/read_model"
)

type WorkOrderRepository struct {
	db                        *sql.DB
	serviceProposalRepository *ServiceProposalRepository
}

func NewWorkOrderRepository(
	db *sql.DB,
	serviceProposalRepository *ServiceProposalRepository,
) *WorkOrderRepository {
	return &WorkOrderRepository{
		db:                        db,
		serviceProposalRepository: serviceProposalRepository,
	}
}

func (r *WorkOrderRepository) Save(ctx context.Context, order *workorder.WorkOrder) (*workorder.WorkOrder, error) {
	if order == nil {
		return nil, fmt.Errorf("saving work order: work order is required")
	}
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return nil, fmt.Errorf("beginning work order transaction: %w", err)
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

func (r *WorkOrderRepository) FindByUserID(ctx context.Context, userID int, viewerRole string) ([]readmodel.WorkOrderSummary, error) {
	if viewerRole != consumer.Role && viewerRole != provider.Role {
		return nil, fmt.Errorf("finding work orders by user id: unsupported viewer role %q", viewerRole)
	}

	rows, err := r.db.QueryContext(
		ctx,
		`SELECT
			wo.id,
			wo.service_proposal_id,
			sp.amount_cents,
			sp.scheduled_on,
			sp.description,
			wo.status,
			wo.accepted_on,
			CASE WHEN $2 = $3 THEN provider_user.id ELSE consumer_user.id END AS counterpart_id,
			CASE WHEN $2 = $3 THEN $4 ELSE $3 END AS counterpart_role,
			CASE WHEN $2 = $3 THEN provider_user.name ELSE consumer_user.name END AS counterpart_name,
			CASE WHEN $2 = $3 THEN provider_user.surname ELSE consumer_user.surname END AS counterpart_surname,
			CASE WHEN $2 = $3 THEN cat.name ELSE '' END AS counterpart_category_name,
			CASE WHEN $2 = $3 THEN COALESCE(provider_user.profile_photo_file_id::text, '') ELSE COALESCE(consumer_user.profile_photo_file_id::text, '') END AS counterpart_profile_photo_file_id
		FROM work_orders wo
		INNER JOIN service_proposals sp ON sp.id = wo.service_proposal_id
		INNER JOIN users consumer_user ON consumer_user.id = sp.consumer_id
		INNER JOIN providers p ON p.user_id = sp.provider_id
		INNER JOIN users provider_user ON provider_user.id = p.user_id
		INNER JOIN categories cat ON cat.id = p.category_id
		WHERE sp.consumer_id = $1 OR sp.provider_id = $1
		ORDER BY sp.scheduled_on ASC, wo.id ASC`,
		userID,
		viewerRole,
		consumer.Role,
		provider.Role,
	)
	if err != nil {
		return nil, fmt.Errorf("finding work orders by user id: %w", err)
	}
	defer func() { _ = rows.Close() }()

	orders := make([]readmodel.WorkOrderSummary, 0)
	for rows.Next() {
		var order readmodel.WorkOrderSummary
		if err := rows.Scan(
			&order.ID,
			&order.ServiceProposalID,
			&order.Amount,
			&order.ScheduledOn,
			&order.Description,
			&order.Status,
			&order.AcceptedOn,
			&order.Counterpart.ID,
			&order.Counterpart.Role,
			&order.Counterpart.Name,
			&order.Counterpart.Surname,
			&order.Counterpart.CategoryName,
			&order.Counterpart.ProfilePhotoFileID,
		); err != nil {
			return nil, fmt.Errorf("scanning work orders by user id: %w", err)
		}
		orders = append(orders, order)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterating work orders by user id: %w", err)
	}
	return orders, nil
}

func (r *WorkOrderRepository) FindScheduledBetween(ctx context.Context, from time.Time, to time.Time) ([]*workorder.WorkOrder, error) {
	rows, err := r.db.QueryContext(
		ctx,
		`SELECT
			wo.id,
			wo.service_proposal_id,
			sp.amount_cents,
			sp.currency,
			sp.deposit_cents,
			sp.platform_fee_total_cents,
			sp.platform_fee_due_now_cents,
			sp.booking_payment_deadline,
			sp.scheduled_on,
			sp.description,
			wo.status,
			wo.accepted_on,
			sp.consumer_id,
			sp.provider_id
		FROM work_orders wo
		INNER JOIN service_proposals sp ON sp.id = wo.service_proposal_id
		WHERE sp.scheduled_on >= $1 AND sp.scheduled_on < $2
		ORDER BY sp.scheduled_on ASC, wo.id ASC`,
		from,
		to,
	)
	if err != nil {
		return nil, fmt.Errorf("finding work orders scheduled between: %w", err)
	}
	defer func() { _ = rows.Close() }()

	workOrders := make([]*workorder.WorkOrder, 0)
	for rows.Next() {
		var order workorder.WorkOrder
		var proposal serviceproposal.ServiceProposal
		var consumerID, providerID int
		var serviceTotalCents, depositCents, platformFeeTotalCents, platformFeeDueNowCents int64
		var currency string
		var bookingPaymentDeadline time.Time
		if err := rows.Scan(
			&order.ID,
			&proposal.ID,
			&serviceTotalCents,
			&currency,
			&depositCents,
			&platformFeeTotalCents,
			&platformFeeDueNowCents,
			&bookingPaymentDeadline,
			&proposal.ScheduledOn,
			&proposal.Description,
			&order.Status,
			&order.AcceptedOn,
			&consumerID,
			&providerID,
		); err != nil {
			return nil, fmt.Errorf("scanning work orders scheduled between: %w", err)
		}
		proposal.BookingTerms, err = serviceproposal.NewBookingTerms(
			currency,
			serviceTotalCents,
			depositCents,
			platformFeeTotalCents,
			platformFeeDueNowCents,
			bookingPaymentDeadline,
		)
		if err != nil {
			return nil, fmt.Errorf("rehydrating booking terms for service proposal %d: %w", proposal.ID, err)
		}
		proposal.Consumer = &consumer.Consumer{BaseUser: user.RehydrateBaseUser(consumerID, "", "", "", "", consumer.Role, nil)}
		proposal.Provider = &provider.Provider{BaseUser: user.RehydrateBaseUser(providerID, "", "", "", "", provider.Role, nil)}
		order.ServiceProposal = &proposal
		workOrders = append(workOrders, &order)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterating work orders scheduled between: %w", err)
	}
	return workOrders, nil
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
