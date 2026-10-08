package repositories

import (
	"context"
	"database/sql"
	"errors"
	"fmt"

	"github.com/LoResuelvo/loresuelvo-api/internal/domain/audit"
	"github.com/LoResuelvo/loresuelvo-api/internal/domain/claim"
	"github.com/jackc/pgx/v5/pgconn"
)

type ClaimAdministrationUnitOfWork struct {
	db     *sql.DB
	claims *ClaimRepository
	events *AuditEventRepository
}

func NewClaimAdministrationUnitOfWork(db *sql.DB, claims *ClaimRepository, events *AuditEventRepository) *ClaimAdministrationUnitOfWork {
	return &ClaimAdministrationUnitOfWork{db: db, claims: claims, events: events}
}

func (u *ClaimAdministrationUnitOfWork) Execute(ctx context.Context, operation func(claim.AdministrationStore) error) error {
	if operation == nil {
		return fmt.Errorf("executing claim administration: operation is required")
	}
	tx, err := u.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("beginning claim administration: %w", err)
	}
	defer func() { _ = tx.Rollback() }()
	store := &claimAdministrationStore{tx: tx, claims: u.claims, events: u.events}
	// Preserve each participant's error; the submission constraint mapper does not own audit failures.
	if err := operation(store); err != nil {
		if rollbackErr := tx.Rollback(); rollbackErr != nil && !errors.Is(rollbackErr, sql.ErrTxDone) {
			return errors.Join(err, fmt.Errorf("rolling back claim administration: %w", rollbackErr))
		}
		return err
	}
	if err := tx.Commit(); err != nil {
		return fmt.Errorf("committing claim administration: %w", err)
	}
	return nil
}

type claimAdministrationStore struct {
	tx     *sql.Tx
	claims *ClaimRepository
	events *AuditEventRepository
}

func (s *claimAdministrationStore) FindClaim(ctx context.Context, id int) (*claim.Claim, error) {
	return s.claims.findLockedByIDWithExecutor(ctx, s.tx, id)
}

// Lock the aggregate root before starting its hydration statement. In Read Committed,
// a joined SELECT that waits for a row lock can see the new claim row but an older
// resolution snapshot; a separate hydration statement observes the winning commit.
func (r *ClaimRepository) findLockedByIDWithExecutor(ctx context.Context, executor claimExecutor, id int) (*claim.Claim, error) {
	var lockedID int
	err := executor.QueryRowContext(ctx, `SELECT id FROM claims WHERE id=$1 FOR UPDATE`, id).Scan(&lockedID)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("locking claim: %w", err)
	}
	return r.findWithExecutor(ctx, executor, claimSelectSQL+` WHERE c.id=$1`, lockedID)
}

func (s *claimAdministrationStore) SaveClaim(ctx context.Context, c *claim.Claim) error {
	return s.claims.saveLifecycleWithExecutor(ctx, s.tx, c)
}

func (s *claimAdministrationStore) SaveAuditEvent(ctx context.Context, event *audit.Event) error {
	return s.events.saveWithExecutor(ctx, s.tx, event)
}

func (s *claimAdministrationStore) FindRecord(ctx context.Context, operator int, key string) (*claim.AdministrationRecord, error) {
	return s.claims.findAdministrationRecordWithExecutor(ctx, s.tx, operator, key)
}

func (s *claimAdministrationStore) SaveRecord(ctx context.Context, record *claim.AdministrationRecord) error {
	return s.claims.saveAdministrationRecordWithExecutor(ctx, s.tx, record)
}

func (r *ClaimRepository) findAdministrationRecordWithExecutor(ctx context.Context, executor claimExecutor, operator int, key string) (*claim.AdministrationRecord, error) {
	record := &claim.AdministrationRecord{OperatorID: operator, Key: key}
	err := executor.QueryRowContext(ctx, `SELECT fingerprint,claim_id,action_id FROM claim_administration_records WHERE operator_id=$1 AND key=$2`, operator, key).Scan(&record.Fingerprint, &record.ClaimID, &record.ActionID)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("finding claim administration record: %w", err)
	}
	return record, nil
}

func (r *ClaimRepository) saveAdministrationRecordWithExecutor(ctx context.Context, executor claimExecutor, record *claim.AdministrationRecord) error {
	_, err := executor.ExecContext(ctx, `INSERT INTO claim_administration_records(operator_id,key,fingerprint,claim_id,action_id) VALUES($1,$2,$3,$4,$5)`, record.OperatorID, record.Key, record.Fingerprint, record.ClaimID, record.ActionID)
	var pgErr *pgconn.PgError
	if errors.As(err, &pgErr) && pgErr.Code == "23505" && pgErr.ConstraintName == "claim_administration_records_pkey" {
		return claim.ErrAdministrationRecordConflict
	}
	if err != nil {
		return fmt.Errorf("saving claim administration record: %w", err)
	}
	return nil
}

func (r *ClaimRepository) saveLifecycleWithExecutor(ctx context.Context, executor claimExecutor, c *claim.Claim) error {
	result, err := executor.ExecContext(ctx, `UPDATE claims SET status=$2,review_started_on=$3,closed_on=$4 WHERE id=$1`, c.ID, c.Status, c.ReviewStartedOn, c.ClosedOn)
	if err != nil {
		return fmt.Errorf("saving claim lifecycle: %w", err)
	}
	count, err := result.RowsAffected()
	if err != nil {
		return err
	}
	if count != 1 {
		return claim.ErrNotFound
	}
	if err := r.saveActionsWithExecutor(ctx, executor, c.ID, c.Actions); err != nil {
		return err
	}
	if c.Resolution != nil {
		return r.saveResolutionWithExecutor(ctx, executor, c.ID, c.Resolution)
	}
	return nil
}

func (r *ClaimRepository) saveActionsWithExecutor(ctx context.Context, executor claimExecutor, id int, actions []claim.Action) error {
	for i := range actions {
		if actions[i].ID != 0 {
			continue
		}
		action := &actions[i]
		if err := executor.QueryRowContext(ctx, `INSERT INTO claim_actions(claim_id,type,actor_id,actor_party,created_on) VALUES($1,$2,$3,$4,$5) RETURNING id`, id, action.Type, action.ActorID, action.ActorParty, action.CreatedOn).Scan(&action.ID); err != nil {
			return fmt.Errorf("saving claim action: %w", err)
		}
	}
	return nil
}

func (r *ClaimRepository) saveResolutionWithExecutor(ctx context.Context, executor claimExecutor, id int, resolution *claim.Resolution) error {
	var amount, currency, unit any
	if suggested := resolution.SuggestedCompensation; suggested != nil {
		amount = suggested.AmountMinor
		currency = suggested.Currency
		unit = suggested.Unit
	}
	_, err := executor.ExecContext(ctx, `INSERT INTO claim_resolutions(claim_id,type,reasoning,resolved_on,suggested_amount_minor,suggested_currency,suggested_unit) VALUES($1,$2,$3,$4,$5,$6,$7)`, id, resolution.Type, resolution.Reasoning, resolution.ResolvedOn, amount, currency, unit)
	if err != nil {
		return fmt.Errorf("saving claim resolution: %w", err)
	}
	return nil
}
