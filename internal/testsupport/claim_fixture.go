package testsupport

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"

	"github.com/LoResuelvo/loresuelvo-api/internal/domain/claim"
)

// ClaimLifecycleFixture prepares persisted US-68 lifecycle states without exposing an administrative API in US-31.
// This helper is for acceptance fixtures only; production behavior belongs to the claim aggregate.
type ClaimLifecycleFixture struct {
	DB         *sql.DB
	OperatorID int
}

func (f ClaimLifecycleFixture) SetState(ctx context.Context, id int, status claim.Status, resolution *claim.Resolution) error {
	if !status.Valid() {
		return claim.ErrInvalidSubmission
	}
	tx, err := f.DB.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("beginning claim lifecycle fixture: %w", err)
	}
	var created time.Time
	if err := tx.QueryRowContext(ctx, `SELECT created_on FROM claims WHERE id=$1`, id).Scan(&created); err != nil {
		return rollbackClaimFixture(tx, fmt.Errorf("finding fixture claim: %w", err))
	}
	var reviewed, closed any
	if status != claim.StatusOpen {
		reviewed = created.Add(time.Hour)
	}
	if status == claim.StatusResolved || status == claim.StatusDismissed {
		if resolution == nil {
			return rollbackClaimFixture(tx, claim.ErrInvalidSubmission)
		}
		closed = resolution.ResolvedOn
		if resolution.ResolvedOn.Before(created) {
			return rollbackClaimFixture(tx, claim.ErrInvalidSubmission)
		}
		// A deterministic review instant valid even when a fixture resolution is near creation.
		reviewed = created.Add(resolution.ResolvedOn.Sub(created) / 2)
	} else if resolution != nil {
		return rollbackClaimFixture(tx, claim.ErrInvalidSubmission)
	}
	if _, err := tx.ExecContext(ctx, `UPDATE claims SET status=$1,review_started_on=$2,closed_on=$3 WHERE id=$4`, status, reviewed, closed, id); err != nil {
		return rollbackClaimFixture(tx, err)
	}
	// Rebuild only fixture lifecycle evidence, preserving the submitted action.
	if _, err := tx.ExecContext(ctx, `DELETE FROM claim_resolutions WHERE claim_id=$1`, id); err != nil {
		return rollbackClaimFixture(tx, err)
	}
	if _, err := tx.ExecContext(ctx, `DELETE FROM claim_actions WHERE claim_id=$1 AND type<>'submitted'`, id); err != nil {
		return rollbackClaimFixture(tx, err)
	}
	if status != claim.StatusOpen && f.OperatorID > 0 {
		operatorID := f.OperatorID
		if _, err := tx.ExecContext(ctx, `INSERT INTO claim_actions(claim_id,type,actor_id,actor_party,created_on) VALUES($1,'review_started',$2,'operator',$3)`, id, operatorID, reviewed); err != nil {
			return rollbackClaimFixture(tx, err)
		}
		if resolution != nil {
			if _, err := tx.ExecContext(ctx, `INSERT INTO claim_actions(claim_id,type,actor_id,actor_party,created_on) VALUES($1,'resolved',$2,'operator',$3)`, id, operatorID, closed); err != nil {
				return rollbackClaimFixture(tx, err)
			}
		}
	}
	if resolution != nil {
		var amount, currency, unit any
		if compensation := resolution.SuggestedCompensation; compensation != nil {
			amount = compensation.AmountMinor
			currency = compensation.Currency
			unit = compensation.Unit
		}
		if _, err := tx.ExecContext(ctx, `INSERT INTO claim_resolutions(claim_id,type,reasoning,resolved_on,suggested_amount_minor,suggested_currency,suggested_unit) VALUES($1,$2,$3,$4,$5,$6,$7)`, id, resolution.Type, resolution.Reasoning, resolution.ResolvedOn, amount, currency, unit); err != nil {
			return rollbackClaimFixture(tx, err)
		}
	}
	if err := tx.Commit(); err != nil {
		return fmt.Errorf("committing claim lifecycle fixture: %w", err)
	}
	return nil
}

func (f ClaimLifecycleFixture) DeleteAll(ctx context.Context) error {
	if _, err := f.DB.ExecContext(ctx, `DELETE FROM claims`); err != nil {
		return fmt.Errorf("deleting claim fixtures: %w", err)
	}
	return nil
}

func rollbackClaimFixture(tx *sql.Tx, cause error) error {
	if err := tx.Rollback(); err != nil && !errors.Is(err, sql.ErrTxDone) {
		return errors.Join(cause, err)
	}
	return cause
}

// SetEvidence establishes an acceptance fixture's evidence and matching submission fingerprint.
// It is not an editing capability of the participant API.
func (f ClaimLifecycleFixture) SetEvidence(ctx context.Context, found *claim.Claim, fileIDs []string) error {
	input, err := (claim.Submission{Reference: found.Reference, Reason: found.Reason, Description: found.Description, ImageFileIDs: fileIDs}).Normalize()
	if err != nil {
		return err
	}
	tx, err := f.DB.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("beginning claim evidence fixture: %w", err)
	}
	result, err := tx.ExecContext(ctx, `UPDATE claims SET submission_fingerprint=$1 WHERE id=$2`, input.Fingerprint(), found.ID)
	if err != nil {
		return rollbackClaimFixture(tx, err)
	}
	count, err := result.RowsAffected()
	if err != nil {
		return rollbackClaimFixture(tx, err)
	}
	if count != 1 {
		return rollbackClaimFixture(tx, claim.ErrNotFound)
	}
	if _, err := tx.ExecContext(ctx, `DELETE FROM claim_images WHERE claim_id=$1`, found.ID); err != nil {
		return rollbackClaimFixture(tx, err)
	}
	for _, id := range input.ImageFileIDs {
		if _, err := tx.ExecContext(ctx, `INSERT INTO claim_images(claim_id,file_id) VALUES($1,$2)`, found.ID, id); err != nil {
			return rollbackClaimFixture(tx, err)
		}
	}
	if err := tx.Commit(); err != nil {
		return fmt.Errorf("committing claim evidence fixture: %w", err)
	}
	return nil
}
