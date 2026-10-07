package repositories

import (
	"context"
	"database/sql"
	"errors"
	"fmt"

	"github.com/LoResuelvo/loresuelvo-api/internal/domain/claim"
	claimmodel "github.com/LoResuelvo/loresuelvo-api/internal/domain/claim/read_model"
	operationmodel "github.com/LoResuelvo/loresuelvo-api/internal/domain/operation/read_model"
	"github.com/jackc/pgx/v5/pgconn"
)

type ClaimRepository struct{ db *sql.DB }

func NewClaimRepository(db *sql.DB) *ClaimRepository { return &ClaimRepository{db: db} }

func claimConstraintError(err error) error {
	var pgErr *pgconn.PgError
	if errors.As(err, &pgErr) {
		if pgErr.Code == "23505" {
			switch pgErr.ConstraintName {
			case "claims_unfinished_operation_unique":
				return claim.ErrOpenClaimConflict
			case "claim_images_file_unique":
				return claim.ErrInvalidEvidence
			default:
				return claim.ErrPersistenceConflict
			}
		}
		if pgErr.Code == "23503" && pgErr.ConstraintName == "claim_images_file_id_fkey" {
			return claim.ErrInvalidEvidence
		}
	}
	return err
}
func rollbackClaimTx(tx *sql.Tx, err error) error {
	if rollbackErr := tx.Rollback(); rollbackErr != nil && !errors.Is(rollbackErr, sql.ErrTxDone) {
		return errors.Join(err, fmt.Errorf("rolling back claim transaction: %w", rollbackErr))
	}
	return claimConstraintError(err)
}
func claimReferences(found *claim.Claim) ([]any, error) {
	var request, proposal, order, intent any
	switch found.Reference.Kind() {
	case claim.ReferenceKindJobRequest:
		request = found.Reference.ID()
	case claim.ReferenceKindServiceProposal:
		proposal = found.Reference.ID()
	case claim.ReferenceKindWorkOrder:
		order = found.Reference.ID()
	case claim.ReferenceKindPaymentIntent:
		intent = found.Reference.ID()
	default:
		return nil, claim.ErrInvalidSubmission
	}
	return []any{request, proposal, order, intent}, nil
}
func (r *ClaimRepository) Save(ctx context.Context, found *claim.Claim) error {
	refs, err := claimReferences(found)
	if err != nil {
		return err
	}
	var operationRequest, operationProposal any
	switch found.OperationID.Kind {
	case operationmodel.KindJobRequest:
		operationRequest = found.OperationID.ResourceID
	case operationmodel.KindServiceProposal:
		operationProposal = found.OperationID.ResourceID
	default:
		return claim.ErrInvalidSubmission
	}
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("beginning claim transaction: %w", err)
	}
	// Existing claims are immutable submissions. Lifecycle evolution belongs to the shared aggregate, not this ingress writer.
	if found.ID != 0 {
		return rollbackClaimTx(tx, claim.ErrPersistenceConflict)
	}
	var id int
	err = tx.QueryRowContext(ctx, `INSERT INTO claims(claimant_id,claimant_party,operation_job_request_id,operation_service_proposal_id,
 reference_job_request_id,reference_service_proposal_id,reference_work_order_id,reference_payment_intent_id,reason,description,status,
 created_on,review_started_on,closed_on,submission_key,submission_fingerprint) VALUES($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12,$13,$14,$15,$16) RETURNING id`,
		found.ClaimantID, found.ClaimantParty, operationRequest, operationProposal, refs[0], refs[1], refs[2], refs[3], found.Reason, found.Description, found.Status, found.CreatedOn, found.ReviewStartedOn, found.ClosedOn, found.SubmissionKey(), found.SubmissionFingerprint()).Scan(&id)
	if err != nil {
		return rollbackClaimTx(tx, err)
	}
	for _, fileID := range found.ImageFileIDs {
		if _, err := tx.ExecContext(ctx, `INSERT INTO claim_images(claim_id,file_id) VALUES($1,$2)`, id, fileID); err != nil {
			return rollbackClaimTx(tx, err)
		}
	}
	actionIDs := make([]int, len(found.Actions))
	for i, action := range found.Actions {
		if err := tx.QueryRowContext(ctx, `INSERT INTO claim_actions(claim_id,type,actor_id,actor_party,created_on) VALUES($1,$2,$3,$4,$5) RETURNING id`, id, action.Type, action.ActorID, action.ActorParty, action.CreatedOn).Scan(&actionIDs[i]); err != nil {
			return rollbackClaimTx(tx, err)
		}
	}
	if resolution := found.Resolution; resolution != nil {
		var amount, currency, unit any
		if suggested := resolution.SuggestedCompensation; suggested != nil {
			amount = suggested.AmountMinor
			currency = suggested.Currency
			unit = suggested.Unit
		}
		if _, err := tx.ExecContext(ctx, `INSERT INTO claim_resolutions(claim_id,type,reasoning,resolved_on,suggested_amount_minor,suggested_currency,suggested_unit) VALUES($1,$2,$3,$4,$5,$6,$7)`, id, resolution.Type, resolution.Reasoning, resolution.ResolvedOn, amount, currency, unit); err != nil {
			return rollbackClaimTx(tx, err)
		}
	}
	if err := tx.Commit(); err != nil {
		return fmt.Errorf("committing claim transaction: %w", claimConstraintError(err))
	}
	found.ID = id
	for i := range found.Actions {
		found.Actions[i].ID = actionIDs[i]
	}
	return nil
}

const claimSelectSQL = `SELECT c.id,c.claimant_id,c.claimant_party,
 CASE WHEN c.operation_job_request_id IS NOT NULL THEN 'jr' ELSE 'sp' END,
 COALESCE(c.operation_job_request_id,c.operation_service_proposal_id),
 CASE WHEN c.reference_job_request_id IS NOT NULL THEN 'job_request' WHEN c.reference_service_proposal_id IS NOT NULL THEN 'service_proposal'
 WHEN c.reference_work_order_id IS NOT NULL THEN 'work_order' ELSE 'payment_intent' END,
 COALESCE(c.reference_job_request_id::text,c.reference_service_proposal_id::text,c.reference_work_order_id::text,c.reference_payment_intent_id::text),
 c.reason,c.description,c.status,c.created_on,c.review_started_on,c.closed_on,c.submission_key::text,c.submission_fingerprint,
 cr.type,cr.reasoning,cr.resolved_on,cr.suggested_amount_minor,cr.suggested_currency,cr.suggested_unit
 FROM claims c LEFT JOIN claim_resolutions cr ON cr.claim_id=c.id`

func (r *ClaimRepository) FindBySubmissionKey(ctx context.Context, claimantID int, key string) (*claim.Claim, error) {
	return r.find(ctx, claimSelectSQL+` WHERE c.claimant_id=$1 AND c.submission_key=$2`, claimantID, key)
}
func (r *ClaimRepository) FindOwnedByID(ctx context.Context, claimantID, id int) (*claim.Claim, error) {
	return r.find(ctx, claimSelectSQL+` WHERE c.claimant_id=$1 AND c.id=$2`, claimantID, id)
}
func (r *ClaimRepository) find(ctx context.Context, query string, args ...any) (*claim.Claim, error) {
	var found claim.Claim
	var referenceKind claim.ReferenceKind
	var referenceID, key, fingerprint string
	var review, closed, resolved sql.NullTime
	var resolutionType, reasoning, currency, unit sql.NullString
	var amount sql.NullInt64
	err := r.db.QueryRowContext(ctx, query, args...).Scan(&found.ID, &found.ClaimantID, &found.ClaimantParty, &found.OperationID.Kind, &found.OperationID.ResourceID, &referenceKind, &referenceID, &found.Reason, &found.Description, &found.Status, &found.CreatedOn, &review, &closed, &key, &fingerprint, &resolutionType, &reasoning, &resolved, &amount, &currency, &unit)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("querying owned claim: %w", err)
	}
	found.CreatedOn = found.CreatedOn.UTC()
	review.Time = review.Time.UTC()
	closed.Time = closed.Time.UTC()
	resolved.Time = resolved.Time.UTC()
	found.Reference, err = claim.NewReference(referenceKind, referenceID)
	if err != nil {
		return nil, fmt.Errorf("rehydrating claim reference: %w", err)
	}
	if review.Valid {
		found.ReviewStartedOn = &review.Time
	}
	if closed.Valid {
		found.ClosedOn = &closed.Time
	}
	if resolutionType.Valid {
		found.Resolution = &claim.Resolution{Type: claim.ResolutionType(resolutionType.String), Reasoning: reasoning.String, ResolvedOn: resolved.Time}
		if amount.Valid {
			found.Resolution.SuggestedCompensation = &claim.SuggestedCompensation{AmountMinor: amount.Int64, Currency: currency.String, Unit: unit.String}
		}
	}
	found.ImageFileIDs, err = r.findImages(ctx, found.ID)
	if err != nil {
		return nil, err
	}
	found.Actions, err = r.findActions(ctx, found.ID)
	if err != nil {
		return nil, err
	}
	return claim.Rehydrate(found, key, fingerprint), nil
}
func (r *ClaimRepository) findImages(ctx context.Context, id int) ([]string, error) {
	rows, err := r.db.QueryContext(ctx, `SELECT file_id::text FROM claim_images WHERE claim_id=$1 ORDER BY file_id`, id)
	if err != nil {
		return nil, fmt.Errorf("querying claim images: %w", err)
	}
	defer rows.Close()
	found := make([]string, 0)
	for rows.Next() {
		var fileID string
		if err := rows.Scan(&fileID); err != nil {
			return nil, fmt.Errorf("scanning claim image: %w", err)
		}
		found = append(found, fileID)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterating claim images: %w", err)
	}
	return found, nil
}
func (r *ClaimRepository) findActions(ctx context.Context, id int) ([]claim.Action, error) {
	rows, err := r.db.QueryContext(ctx, `SELECT id,type,actor_id,actor_party,created_on FROM claim_actions WHERE claim_id=$1 ORDER BY id`, id)
	if err != nil {
		return nil, fmt.Errorf("querying claim actions: %w", err)
	}
	defer rows.Close()
	found := make([]claim.Action, 0)
	for rows.Next() {
		var action claim.Action
		if err := rows.Scan(&action.ID, &action.Type, &action.ActorID, &action.ActorParty, &action.CreatedOn); err != nil {
			return nil, fmt.Errorf("scanning claim action: %w", err)
		}
		action.CreatedOn = action.CreatedOn.UTC()
		found = append(found, action)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterating claim actions: %w", err)
	}
	return found, nil
}

const claimSummarySQL = `SELECT c.id,CASE WHEN operation_job_request_id IS NOT NULL THEN 'jr' ELSE 'sp' END,
 COALESCE(operation_job_request_id,operation_service_proposal_id),
 CASE WHEN reference_job_request_id IS NOT NULL THEN 'job_request' WHEN reference_service_proposal_id IS NOT NULL THEN 'service_proposal'
 WHEN reference_work_order_id IS NOT NULL THEN 'work_order' ELSE 'payment_intent' END,
 COALESCE(reference_job_request_id::text,reference_service_proposal_id::text,reference_work_order_id::text,reference_payment_intent_id::text),reason,status,created_on,review_started_on,closed_on
 FROM claims c WHERE claimant_id=$1 AND ($2::text IS NULL OR status=$2)`

func (r *ClaimRepository) FindOwnedPage(ctx context.Context, claimantID int, criteria claim.ListCriteria) (*claim.Page, error) {
	criteria, err := criteria.Normalize()
	if err != nil {
		return nil, err
	}
	var status any
	if criteria.Status != nil {
		status = string(*criteria.Status)
	}
	// One read-only snapshot keeps the total and page consistent under concurrent submissions.
	tx, err := r.db.BeginTx(ctx, &sql.TxOptions{Isolation: sql.LevelRepeatableRead, ReadOnly: true})
	if err != nil {
		return nil, fmt.Errorf("beginning claim page snapshot: %w", err)
	}
	page := &claim.Page{Claims: make([]claimmodel.ClaimSummary, 0), Page: criteria.Page, Limit: criteria.Limit}
	if err := tx.QueryRowContext(ctx, `SELECT COUNT(*) FROM claims WHERE claimant_id=$1 AND ($2::text IS NULL OR status=$2)`, claimantID, status).Scan(&page.Total); err != nil {
		return nil, rollbackClaimTx(tx, err)
	}
	rows, err := tx.QueryContext(ctx, claimSummarySQL+` ORDER BY created_on DESC,id DESC LIMIT $3 OFFSET $4`, claimantID, status, criteria.Limit, int64(criteria.Page-1)*int64(criteria.Limit))
	if err != nil {
		return nil, rollbackClaimTx(tx, err)
	}
	for rows.Next() {
		var found claimmodel.ClaimSummary
		var review, closed sql.NullTime
		if err := rows.Scan(&found.ID, &found.OperationID.Kind, &found.OperationID.ResourceID, &found.ReferenceKind, &found.ReferenceID, &found.Reason, &found.Status, &found.CreatedOn, &review, &closed); err != nil {
			closeErr := rows.Close()
			return nil, rollbackClaimTx(tx, errors.Join(err, closeErr))
		}
		found.CreatedOn = found.CreatedOn.UTC()
		review.Time = review.Time.UTC()
		closed.Time = closed.Time.UTC()
		if review.Valid {
			found.ReviewStartedOn = &review.Time
		}
		if closed.Valid {
			found.ClosedOn = &closed.Time
		}
		page.Claims = append(page.Claims, found)
	}
	if err := errors.Join(rows.Err(), rows.Close()); err != nil {
		return nil, rollbackClaimTx(tx, err)
	}
	if err := tx.Commit(); err != nil {
		return nil, fmt.Errorf("committing claim page snapshot: %w", err)
	}
	return page, nil
}

// Compile-time contracts document the adapters' narrow dependencies.
var _ claim.Repository = (*ClaimRepository)(nil)
var _ claim.OperationReferenceResolver = (*ClaimOperationReferenceResolver)(nil)
var _ claim.UserFinder = (*ClaimUserFinder)(nil)
