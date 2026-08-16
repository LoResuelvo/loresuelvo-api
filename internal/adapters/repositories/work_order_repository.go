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
	"github.com/jackc/pgx/v5/pgconn"
)

type WorkOrderRepository struct {
	db                        *sql.DB
	serviceProposalRepository *ServiceProposalRepository
}

type workOrderRecord struct {
	ID                           sql.NullInt64
	ServiceProposalID            sql.NullInt64
	Status                       sql.NullString
	AcceptedOn                   sql.NullTime
	PaidOn                       sql.NullTime
	CompletionReportID           sql.NullInt64
	CompletionReportDescription  sql.NullString
	CompletionReportReportedOn   sql.NullTime
	CompletionReportImageFileIDs []string
}

func (record workOrderRecord) Restore(proposal workorder.ServiceProposal) (*workorder.WorkOrder, error) {
	if !record.ID.Valid || record.ID.Int64 <= 0 {
		return nil, fmt.Errorf("work order id is required")
	}
	if !record.ServiceProposalID.Valid || record.ServiceProposalID.Int64 <= 0 {
		return nil, fmt.Errorf("service proposal id is required")
	}
	if !record.Status.Valid || record.Status.String == "" {
		return nil, fmt.Errorf("work order status is required")
	}
	if !record.AcceptedOn.Valid {
		return nil, fmt.Errorf("work order accepted_on is required")
	}
	if proposal == nil || proposal.ServiceProposalID() != int(record.ServiceProposalID.Int64) {
		return nil, fmt.Errorf("work order service proposal does not match record")
	}

	return (workorder.RestoreFactory{}).Restore(workorder.RestoreInput{
		ID:               int(record.ID.Int64),
		ServiceProposal:  proposal,
		Status:           workorder.Status(record.Status.String),
		AcceptedOn:       record.AcceptedOn.Time,
		CompletionReport: record.completionReport(),
		PaidOn:           record.PaidOn.Time,
	})
}

func (record workOrderRecord) completionReport() *workorder.CompletionReportRestoreInput {
	if !record.CompletionReportID.Valid &&
		!record.CompletionReportDescription.Valid &&
		!record.CompletionReportReportedOn.Valid {
		return nil
	}

	return &workorder.CompletionReportRestoreInput{
		ID:           int(record.CompletionReportID.Int64),
		Description:  record.CompletionReportDescription.String,
		ImageFileIDs: append([]string(nil), record.CompletionReportImageFileIDs...),
		ReportedOn:   record.CompletionReportReportedOn.Time,
	}
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
	return r.findOne(ctx, `WHERE wo.service_proposal_id = $1`, serviceProposalID)
}

func (r *WorkOrderRepository) FindByID(ctx context.Context, id int) (*workorder.WorkOrder, error) {
	return r.findOne(ctx, `WHERE wo.id = $1`, id)
}

func (r *WorkOrderRepository) findOne(
	ctx context.Context,
	clause string,
	argument int,
) (*workorder.WorkOrder, error) {
	var record workOrderRecord
	err := r.db.QueryRowContext(
		ctx,
		`SELECT wo.id, wo.service_proposal_id, wo.status, wo.accepted_on, wo.paid_on
		FROM work_orders wo
		`+clause,
		argument,
	).Scan(
		&record.ID,
		&record.ServiceProposalID,
		&record.Status,
		&record.AcceptedOn,
		&record.PaidOn,
	)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, workorder.ErrDoesNotExist
	}
	if err != nil {
		return nil, fmt.Errorf("finding work order: %w", err)
	}
	if !record.ServiceProposalID.Valid || record.ServiceProposalID.Int64 <= 0 {
		return nil, fmt.Errorf("finding work order: service proposal id is required")
	}
	completionReport, err := r.findCompletionReport(ctx, int(record.ID.Int64))
	if err != nil {
		return nil, fmt.Errorf("finding work order completion report: %w", err)
	}
	record.setCompletionReport(completionReport)
	foundProposal, err := r.serviceProposalRepository.FindByID(ctx, int(record.ServiceProposalID.Int64))
	if err != nil {
		return nil, fmt.Errorf("hydrating work order service proposal: %w", err)
	}
	order, err := record.Restore(foundProposal)
	if err != nil {
		return nil, fmt.Errorf("rehydrating work order: %w", err)
	}
	return order, nil
}

func (record *workOrderRecord) setCompletionReport(report *workorder.CompletionReportRestoreInput) {
	if report == nil {
		return
	}
	record.CompletionReportID = sql.NullInt64{Int64: int64(report.ID), Valid: true}
	record.CompletionReportDescription = sql.NullString{String: report.Description, Valid: true}
	record.CompletionReportReportedOn = sql.NullTime{Time: report.ReportedOn, Valid: true}
	record.CompletionReportImageFileIDs = append([]string(nil), report.ImageFileIDs...)
}

func (r *WorkOrderRepository) findCompletionReport(ctx context.Context, workOrderID int) (*workorder.CompletionReportRestoreInput, error) {
	var report workorder.CompletionReportRestoreInput
	var reportID sql.NullInt64
	var description sql.NullString
	var reportedOn sql.NullTime
	err := r.db.QueryRowContext(
		ctx,
		`SELECT id, description, reported_on
		FROM work_order_completion_reports
		WHERE work_order_id = $1`,
		workOrderID,
	).Scan(&reportID, &description, &reportedOn)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("finding completion report: %w", err)
	}

	images, err := r.db.QueryContext(
		ctx,
		`SELECT file_id::text
		FROM work_order_completion_images
		WHERE completion_report_id = $1
		ORDER BY position ASC`,
		reportID.Int64,
	)
	if err != nil {
		return nil, fmt.Errorf("finding completion report images: %w", err)
	}
	defer func() { _ = images.Close() }()

	imageFileIDs := make([]string, 0, 3)
	for images.Next() {
		var fileID sql.NullString
		if err := images.Scan(&fileID); err != nil {
			return nil, fmt.Errorf("scanning completion report image: %w", err)
		}
		imageFileIDs = append(imageFileIDs, fileID.String)
	}
	if err := images.Err(); err != nil {
		return nil, fmt.Errorf("iterating completion report images: %w", err)
	}

	report.ID = int(reportID.Int64)
	report.Description = description.String
	report.ImageFileIDs = imageFileIDs
	report.ReportedOn = reportedOn.Time
	return &report, nil
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
			wo.paid_on,
			sp.consumer_id,
			sp.provider_id
		FROM work_orders wo
		INNER JOIN service_proposals sp ON sp.id = wo.service_proposal_id
		WHERE sp.scheduled_on >= $1 AND sp.scheduled_on < $2
			AND wo.status = $3
		ORDER BY sp.scheduled_on ASC, wo.id ASC`,
		from,
		to,
		workorder.StatusScheduled,
	)
	if err != nil {
		return nil, fmt.Errorf("finding work orders scheduled between: %w", err)
	}
	defer func() { _ = rows.Close() }()

	workOrders := make([]*workorder.WorkOrder, 0)
	for rows.Next() {
		var record workOrderRecord
		var proposal serviceproposal.ServiceProposal
		var serviceProposalID sql.NullInt64
		var consumerID, providerID int
		var serviceTotalCents, depositCents, platformFeeTotalCents, platformFeeDueNowCents int64
		var currency string
		var bookingPaymentDeadline time.Time
		if err := rows.Scan(
			&record.ID,
			&serviceProposalID,
			&serviceTotalCents,
			&currency,
			&depositCents,
			&platformFeeTotalCents,
			&platformFeeDueNowCents,
			&bookingPaymentDeadline,
			&proposal.ScheduledOn,
			&proposal.Description,
			&record.Status,
			&record.AcceptedOn,
			&record.PaidOn,
			&consumerID,
			&providerID,
		); err != nil {
			return nil, fmt.Errorf("scanning work orders scheduled between: %w", err)
		}
		record.ServiceProposalID = serviceProposalID
		completionReport, err := r.findCompletionReport(ctx, int(record.ID.Int64))
		if err != nil {
			return nil, fmt.Errorf("finding scheduled work order completion report: %w", err)
		}
		record.setCompletionReport(completionReport)
		if !serviceProposalID.Valid || serviceProposalID.Int64 <= 0 {
			return nil, fmt.Errorf("rehydrating scheduled work order: service proposal id is required")
		}
		proposal.ID = int(serviceProposalID.Int64)
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
		order, err := record.Restore(&proposal)
		if err != nil {
			return nil, fmt.Errorf("rehydrating scheduled work order %d: %w", record.ID.Int64, err)
		}
		workOrders = append(workOrders, order)
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
	if order.ServiceProposalID() <= 0 {
		return nil, fmt.Errorf("saving work order: service proposal is required")
	}
	var paidOn any
	if !order.PaidOn().IsZero() {
		paidOn = order.PaidOn().UTC()
	}

	if order.ID() <= 0 {
		var savedID int
		err := tx.QueryRowContext(
			ctx,
			`INSERT INTO work_orders (
			service_proposal_id,
			status,
			accepted_on,
			paid_on,
			updated_on
		)
		VALUES ($1, $2, $3, $4, NOW())
		RETURNING id`,
			order.ServiceProposalID(),
			order.Status(),
			order.AcceptedOn(),
			paidOn,
		).Scan(&savedID)
		if err != nil {
			return nil, fmt.Errorf("saving work order: %w", err)
		}
		saved := *order
		saved.SetID(savedID)
		if err := r.saveCompletionReportWithTx(ctx, tx, &saved); err != nil {
			return nil, err
		}
		return &saved, nil
	}

	var savedID int
	err := tx.QueryRowContext(
		ctx,
		`UPDATE work_orders
		SET status = $1,
			paid_on = $2,
			updated_on = NOW()
		WHERE id = $3 AND service_proposal_id = $4
		RETURNING id`,
		order.Status(),
		paidOn,
		order.ID(),
		order.ServiceProposalID(),
	).Scan(&savedID)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, workorder.ErrDoesNotExist
	}
	if err != nil {
		return nil, fmt.Errorf("updating work order: %w", err)
	}
	saved := *order
	saved.SetID(savedID)
	if err := r.saveCompletionReportWithTx(ctx, tx, &saved); err != nil {
		return nil, err
	}
	return &saved, nil
}

func (r *WorkOrderRepository) saveCompletionReportWithTx(
	ctx context.Context,
	tx *sql.Tx,
	order *workorder.WorkOrder,
) error {
	if order == nil || order.ID() <= 0 {
		return fmt.Errorf("saving work order completion report: work order identity is required")
	}

	report := order.CompletionReport()
	if report == nil {
		if _, err := tx.ExecContext(
			ctx,
			`DELETE FROM work_order_completion_reports WHERE work_order_id = $1`,
			order.ID(),
		); err != nil {
			return fmt.Errorf("removing work order completion report: %w", err)
		}
		return nil
	}

	var reportID int
	err := tx.QueryRowContext(
		ctx,
		`INSERT INTO work_order_completion_reports (work_order_id, description, reported_on)
		VALUES ($1, $2, $3)
		ON CONFLICT (work_order_id) DO UPDATE SET
			description = EXCLUDED.description,
			reported_on = EXCLUDED.reported_on
		RETURNING id`,
		order.ID(),
		report.Description(),
		report.ReportedOn().UTC(),
	).Scan(&reportID)
	if err != nil {
		return fmt.Errorf("saving work order completion report: %w", err)
	}
	report.SetID(reportID)

	if _, err := tx.ExecContext(
		ctx,
		`DELETE FROM work_order_completion_images WHERE completion_report_id = $1`,
		reportID,
	); err != nil {
		return fmt.Errorf("replacing work order completion images: %w", err)
	}

	for position, fileID := range report.ImageFileIDs() {
		if _, err := tx.ExecContext(
			ctx,
			`INSERT INTO work_order_completion_images (completion_report_id, file_id, position)
			VALUES ($1, $2, $3)`,
			reportID,
			fileID,
			position,
		); err != nil {
			var pgErr *pgconn.PgError
			if errors.As(err, &pgErr) && (pgErr.Code == uniqueViolationCode || pgErr.Code == foreignKeyViolationCode) {
				return workorder.ErrCompletionReportImageNotAvailable
			}
			return fmt.Errorf("saving work order completion image: %w", err)
		}
	}
	return nil
}

func rollbackWorkOrderTx(tx *sql.Tx, cause error) error {
	if rollbackErr := tx.Rollback(); rollbackErr != nil {
		return fmt.Errorf("%w: rolling back work order transaction: %v", cause, rollbackErr)
	}
	return cause
}
