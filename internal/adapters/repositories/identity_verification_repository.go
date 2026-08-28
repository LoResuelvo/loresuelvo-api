package repositories

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"

	"github.com/LoResuelvo/loresuelvo-api/internal/domain/identityverification"
	"github.com/google/uuid"
)

type IdentityVerificationRepository struct{ db *sql.DB }

func NewIdentityVerificationRepository(db *sql.DB) *IdentityVerificationRepository {
	return &IdentityVerificationRepository{db: db}
}

func (repository *IdentityVerificationRepository) Save(ctx context.Context, verification *identityverification.IdentityVerification) error {
	_, err := repository.db.ExecContext(ctx, `
		INSERT INTO identity_verification_sessions (
			external_session_id, provider_id, verifier, workflow_id, workflow_version, status, created_on, updated_on
		) VALUES ($1, $2, $3, $4, $5, $6, $7, $8)
		ON CONFLICT (external_session_id) DO UPDATE SET
			provider_id = EXCLUDED.provider_id,
			verifier = EXCLUDED.verifier,
			workflow_id = EXCLUDED.workflow_id,
			workflow_version = EXCLUDED.workflow_version,
			status = EXCLUDED.status,
			updated_on = EXCLUDED.updated_on`,
		verification.ExternalSessionID, verification.ProviderID, verification.Verifier,
		verification.WorkflowID, verification.WorkflowVersion, verification.Status,
		verification.CreatedOn, verification.UpdatedOn)
	if err != nil {
		return fmt.Errorf("saving identity verification: %w", err)
	}
	return nil
}

func (repository *IdentityVerificationRepository) FindBySessionID(ctx context.Context, sessionID uuid.UUID) (*identityverification.IdentityVerification, error) {
	var providerID, workflowVersion int
	var verifier, status string
	var workflowID uuid.UUID
	var createdOn, updatedOn time.Time
	err := repository.db.QueryRowContext(ctx, `
		SELECT provider_id, verifier, workflow_id, workflow_version, status, created_on, updated_on
		FROM identity_verification_sessions WHERE external_session_id = $1`, sessionID).
		Scan(&providerID, &verifier, &workflowID, &workflowVersion, &status, &createdOn, &updatedOn)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("finding identity verification: %w", err)
	}
	verification, err := identityverification.Rehydrate(sessionID, providerID, verifier, workflowID, workflowVersion, identityverification.VerificationStatus(status), createdOn, updatedOn)
	if err != nil {
		return nil, fmt.Errorf("rehydrating identity verification: %w", err)
	}
	return verification, nil
}

func (repository *IdentityVerificationRepository) DeleteAll() error {
	if _, err := repository.db.Exec(`DELETE FROM identity_verification_sessions`); err != nil {
		return fmt.Errorf("deleting identity verification sessions: %w", err)
	}
	return nil
}
