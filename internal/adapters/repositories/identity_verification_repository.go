package repositories

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/LoResuelvo/loresuelvo-api/internal/domain/identityverification"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgtype"
)

type IdentityVerificationRepository struct {
	db *sql.DB
}

func NewIdentityVerificationRepository(db *sql.DB) *IdentityVerificationRepository {
	return &IdentityVerificationRepository{db: db}
}

func (repository *IdentityVerificationRepository) Save(ctx context.Context, verification *identityverification.IdentityVerification) error {
	_, err := repository.db.ExecContext(ctx, `
		INSERT INTO identity_verification_sessions (
			external_session_id, provider_id, verifier, workflow_id, workflow_version,
			status, risk_codes, last_result_on, verified_on, created_on, updated_on
		) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11)
		ON CONFLICT (external_session_id) DO UPDATE SET
			provider_id = EXCLUDED.provider_id,
			verifier = EXCLUDED.verifier,
			workflow_id = EXCLUDED.workflow_id,
			workflow_version = EXCLUDED.workflow_version,
			status = EXCLUDED.status,
			risk_codes = EXCLUDED.risk_codes,
			last_result_on = EXCLUDED.last_result_on,
			verified_on = EXCLUDED.verified_on,
			updated_on = EXCLUDED.updated_on`,
		verification.ExternalSessionID,
		verification.ProviderID,
		verification.Verifier,
		verification.WorkflowID,
		verification.WorkflowVersion,
		verification.Status,
		riskCodesValue(verification.RiskCodes),
		nullableTime(verification.LastResultOn),
		nullableTimePtr(verification.VerifiedOn),
		verification.CreatedOn,
		verification.UpdatedOn,
	)
	if err != nil {
		return fmt.Errorf("saving identity verification: %w", err)
	}
	return nil
}

func (repository *IdentityVerificationRepository) FindBySessionID(ctx context.Context, sessionID uuid.UUID) (*identityverification.IdentityVerification, error) {
	return repository.find(ctx, `WHERE external_session_id = $1`, sessionID)
}

func (repository *IdentityVerificationRepository) FindLatestByProviderID(ctx context.Context, providerID int) (*identityverification.IdentityVerification, error) {
	return repository.find(ctx, `WHERE provider_id = $1 ORDER BY created_on DESC, external_session_id DESC LIMIT 1`, providerID)
}

func (repository *IdentityVerificationRepository) FindStatusByProviderID(ctx context.Context, providerID int) (identityverification.StatusSnapshot, error) {
	verification, err := repository.FindLatestByProviderID(ctx, providerID)
	if err != nil {
		return identityverification.StatusSnapshot{}, err
	}
	if verification == nil {
		return identityverification.StatusSnapshot{Status: identityverification.StatusUnverified}, nil
	}
	return identityverification.StatusSnapshot{
		Status: verification.Status, Verified: verification.IsApproved(), VerifiedOn: cloneTime(verification.VerifiedOn),
		RiskCodes: append([]string(nil), verification.RiskCodes...),
	}, nil
}

func (repository *IdentityVerificationRepository) EventExists(ctx context.Context, eventID uuid.UUID) (bool, error) {
	var exists bool
	if err := repository.db.QueryRowContext(ctx, `SELECT EXISTS(SELECT 1 FROM identity_verification_events WHERE external_event_id = $1)`, eventID).Scan(&exists); err != nil {
		return false, fmt.Errorf("checking identity verification event: %w", err)
	}
	return exists, nil
}

func (repository *IdentityVerificationRepository) SaveEvent(ctx context.Context, event identityverification.VerificationEvent) error {
	_, err := repository.db.ExecContext(ctx, `
		INSERT INTO identity_verification_events (external_event_id, external_session_id, event_type, occurred_on, received_on)
		VALUES ($1, $2, $3, $4, $5)
		ON CONFLICT (external_event_id) DO NOTHING`,
		event.ExternalEventID, event.ExternalSessionID, event.EventType, event.OccurredOn, event.ReceivedOn)
	if err != nil {
		return fmt.Errorf("saving identity verification event: %w", err)
	}
	return nil
}

func (repository *IdentityVerificationRepository) SaveResult(ctx context.Context, event identityverification.VerificationEvent, verification *identityverification.IdentityVerification) error {
	tx, err := repository.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("beginning identity verification transaction: %w", err)
	}

	result, err := tx.ExecContext(ctx, `
		INSERT INTO identity_verification_events (external_event_id, external_session_id, event_type, occurred_on, received_on)
		VALUES ($1, $2, $3, $4, $5)
		ON CONFLICT (external_event_id) DO NOTHING`,
		event.ExternalEventID, event.ExternalSessionID, event.EventType, event.OccurredOn, event.ReceivedOn)
	if err != nil {
		return rollbackIdentityVerificationTx(tx, fmt.Errorf("saving identity verification event: %w", err))
	}
	rowsAffected, err := result.RowsAffected()
	if err != nil {
		return rollbackIdentityVerificationTx(tx, fmt.Errorf("checking identity verification event: %w", err))
	}
	if rowsAffected == 0 {
		if err := tx.Commit(); err != nil {
			return fmt.Errorf("committing duplicate identity verification event: %w", err)
		}
		return nil
	}

	result, err = tx.ExecContext(ctx, `
		UPDATE identity_verification_sessions SET
			status = $2, risk_codes = $3, last_result_on = $4, verified_on = $5, updated_on = $6
		WHERE external_session_id = $1`,
		verification.ExternalSessionID, verification.Status, riskCodesValue(verification.RiskCodes),
		nullableTime(verification.LastResultOn), nullableTimePtr(verification.VerifiedOn), verification.UpdatedOn)
	if err != nil {
		return rollbackIdentityVerificationTx(tx, fmt.Errorf("updating identity verification: %w", err))
	}
	rowsAffected, err = result.RowsAffected()
	if err != nil {
		return rollbackIdentityVerificationTx(tx, fmt.Errorf("checking identity verification update: %w", err))
	}
	if rowsAffected != 1 {
		return rollbackIdentityVerificationTx(tx, identityverification.ErrSessionNotFound)
	}
	if err := tx.Commit(); err != nil {
		return fmt.Errorf("committing identity verification result: %w", err)
	}
	return nil
}

func (repository *IdentityVerificationRepository) DeleteAll() error {
	_, err := repository.db.Exec(`DELETE FROM identity_verification_events`)
	if err != nil {
		return fmt.Errorf("deleting identity verification events: %w", err)
	}
	_, err = repository.db.Exec(`DELETE FROM identity_verification_sessions`)
	if err != nil {
		return fmt.Errorf("deleting identity verification sessions: %w", err)
	}
	return nil
}

func (repository *IdentityVerificationRepository) find(ctx context.Context, suffix string, args ...any) (*identityverification.IdentityVerification, error) {
	var sessionID, workflowID uuid.UUID
	var providerID, workflowVersion int
	var verifier, status string
	var rawRiskCodes string
	var lastResultOn, verifiedOn sql.NullTime
	var createdOn, updatedOn time.Time
	err := repository.db.QueryRowContext(ctx, `
		SELECT external_session_id, provider_id, verifier, workflow_id, workflow_version,
			status, risk_codes, last_result_on, verified_on, created_on, updated_on
		FROM identity_verification_sessions `+suffix,
		args...).Scan(&sessionID, &providerID, &verifier, &workflowID, &workflowVersion,
		&status, &rawRiskCodes, &lastResultOn, &verifiedOn, &createdOn, &updatedOn)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("finding identity verification: %w", err)
	}
	var verifiedOnPtr *time.Time
	if verifiedOn.Valid {
		value := verifiedOn.Time
		verifiedOnPtr = &value
	}
	lastResult := time.Time{}
	if lastResultOn.Valid {
		lastResult = lastResultOn.Time
	}
	verification, err := identityverification.Rehydrate(sessionID, providerID, verifier, workflowID, workflowVersion,
		identityverification.VerificationStatus(status), parseRiskCodes(rawRiskCodes), lastResult, verifiedOnPtr, createdOn, updatedOn)
	if err != nil {
		return nil, fmt.Errorf("rehydrating identity verification: %w", err)
	}
	return verification, nil
}

func riskCodesValue(codes []string) pgtype.FlatArray[string] {
	if codes == nil {
		return pgtype.FlatArray[string]{}
	}
	return pgtype.FlatArray[string](codes)
}

func parseRiskCodes(value string) []string {
	value = strings.TrimSpace(value)
	if len(value) < 2 || value == "{}" {
		return []string{}
	}
	value = strings.TrimPrefix(strings.TrimSuffix(value, "}"), "{")
	parts := strings.Split(value, ",")
	result := make([]string, 0, len(parts))
	for _, part := range parts {
		part = strings.Trim(part, `" `)
		if part != "" {
			result = append(result, part)
		}
	}
	return result
}

func nullableTime(value time.Time) any {
	if value.IsZero() {
		return nil
	}
	return value
}

func nullableTimePtr(value *time.Time) any {
	if value == nil {
		return nil
	}
	return *value
}

func cloneTime(value *time.Time) *time.Time {
	if value == nil {
		return nil
	}
	cloned := value.UTC()
	return &cloned
}

func rollbackIdentityVerificationTx(tx *sql.Tx, originalErr error) error {
	if rollbackErr := tx.Rollback(); rollbackErr != nil {
		return fmt.Errorf("%w; additionally could not rollback identity verification transaction: %v", originalErr, rollbackErr)
	}
	return originalErr
}
