package repositories

import (
	"context"
	"database/sql"
	"errors"
	"fmt"

	"github.com/LoResuelvo/loresuelvo-api/internal/domain/claim"
	claimmodel "github.com/LoResuelvo/loresuelvo-api/internal/domain/claim/read_model"
)

// Canonical operation identity is persisted on the claim. Context joins are one-to-one;
// evidence/actions are deliberately absent so a claim never multiplies into several rows.
const claimAdminContextSQL = ` FROM claims c JOIN users u ON u.id=c.claimant_id
 LEFT JOIN job_requests jr ON jr.id=c.operation_job_request_id
 LEFT JOIN service_proposals sp ON sp.id=c.operation_service_proposal_id
 LEFT JOIN providers p ON p.user_id=COALESCE(jr.provider_id,sp.provider_id)
 LEFT JOIN categories cat ON cat.id=p.category_id`
const claimAdminFilterSQL = ` WHERE ($1::text IS NULL OR c.status=$1) AND ($2::text='' OR STRPOS(LOWER(u.email),LOWER($2))>0
 OR (CASE WHEN c.operation_job_request_id IS NOT NULL THEN 'jr-' ELSE 'sp-' END || COALESCE(c.operation_job_request_id,c.operation_service_proposal_id)::text)=$2)`

func (r *ClaimRepository) FindAdministrativePage(ctx context.Context, criteria claim.AdminCriteria) (*claim.AdminPage, error) {
	criteria, err := criteria.Normalize()
	if err != nil {
		return nil, err
	}
	var status any
	if criteria.Status != nil {
		status = string(*criteria.Status)
	}
	tx, err := r.db.BeginTx(ctx, &sql.TxOptions{Isolation: sql.LevelRepeatableRead, ReadOnly: true})
	if err != nil {
		return nil, err
	}
	defer func() { _ = tx.Rollback() }()
	page := &claim.AdminPage{Claims: make([]claimmodel.AdminClaimSummary, 0), Page: criteria.Page, Limit: criteria.Limit}
	if err := tx.QueryRowContext(ctx, `SELECT COUNT(*)`+claimAdminContextSQL+claimAdminFilterSQL, status, criteria.Query).Scan(&page.Total); err != nil {
		return nil, rollbackClaimTx(tx, err)
	}
	rows, err := tx.QueryContext(ctx, `SELECT c.id,c.claimant_id,c.claimant_party,u.email,
 CASE WHEN c.operation_job_request_id IS NOT NULL THEN 'jr' ELSE 'sp' END, COALESCE(c.operation_job_request_id,c.operation_service_proposal_id),
 CASE WHEN c.reference_job_request_id IS NOT NULL THEN 'job_request' WHEN c.reference_service_proposal_id IS NOT NULL THEN 'service_proposal' WHEN c.reference_work_order_id IS NOT NULL THEN 'work_order' ELSE 'payment_intent' END,
 COALESCE(c.reference_job_request_id::text,c.reference_service_proposal_id::text,c.reference_work_order_id::text,c.reference_payment_intent_id::text),c.reason,c.status,c.created_on,c.review_started_on,c.closed_on,cat.id,cat.name`+claimAdminContextSQL+claimAdminFilterSQL+` ORDER BY c.created_on DESC,c.id DESC LIMIT $3 OFFSET $4`, status, criteria.Query, criteria.Limit, int64(criteria.Page-1)*int64(criteria.Limit))
	if err != nil {
		return nil, rollbackClaimTx(tx, err)
	}
	defer rows.Close()
	for rows.Next() {
		var item claimmodel.AdminClaimSummary
		var review, closed sql.NullTime
		var category sql.NullInt64
		var name sql.NullString
		err := rows.Scan(&item.ID, &item.ClaimantID, &item.ClaimantParty, &item.ClaimantEmail, &item.OperationID.Kind, &item.OperationID.ResourceID, &item.ReferenceKind, &item.ReferenceID, &item.Reason, &item.Status, &item.CreatedOn, &review, &closed, &category, &name)
		if err != nil {
			return nil, rollbackClaimTx(tx, err)
		}
		item.CreatedOn = item.CreatedOn.UTC()
		if review.Valid {
			value := review.Time.UTC()
			item.ReviewStartedOn = &value
		}
		if closed.Valid {
			value := closed.Time.UTC()
			item.ClosedOn = &value
		}
		if category.Valid {
			value := int(category.Int64)
			item.CategoryID = &value
		}
		if name.Valid {
			item.CategoryName = &name.String
		}
		page.Claims = append(page.Claims, item)
	}
	if err := errors.Join(rows.Err(), rows.Close()); err != nil {
		return nil, rollbackClaimTx(tx, err)
	}
	if err := tx.Commit(); err != nil {
		return nil, err
	}
	return page, nil
}

func (r *ClaimRepository) FindAdministrativeByID(ctx context.Context, id int) (*claim.AdminDetail, error) {
	tx, err := r.db.BeginTx(ctx, &sql.TxOptions{Isolation: sql.LevelRepeatableRead, ReadOnly: true})
	if err != nil {
		return nil, err
	}
	defer func() { _ = tx.Rollback() }()
	found, err := r.findWithExecutor(ctx, tx, claimSelectSQL+` WHERE c.id=$1`, id)
	if err != nil {
		return nil, rollbackClaimTx(tx, err)
	}
	if found == nil {
		return nil, nil
	}
	result := &claim.AdminDetail{GetResult: claim.GetResult{Claim: found}}
	var category sql.NullInt64
	var name sql.NullString
	err = tx.QueryRowContext(ctx, `SELECT u.email,cat.id,cat.name`+claimAdminContextSQL+` WHERE c.id=$1`, id).Scan(&result.ClaimantEmail, &category, &name)
	if err != nil {
		return nil, rollbackClaimTx(tx, fmt.Errorf("reading claim context: %w", err))
	}
	if category.Valid {
		value := int(category.Int64)
		result.CategoryID = &value
	}
	if name.Valid {
		result.CategoryName = &name.String
	}
	if err := tx.Commit(); err != nil {
		return nil, err
	}
	return result, nil
}
