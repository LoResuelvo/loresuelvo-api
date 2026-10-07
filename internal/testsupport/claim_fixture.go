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
type ClaimLifecycleFixture struct{ DB *sql.DB }

func (f ClaimLifecycleFixture) SetState(ctx context.Context, id int, status claim.Status, resolution *claim.Resolution) error {
	if !status.Valid() {
		return claim.ErrInvalidSubmission
	}
	tx, err := f.DB.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("beginning claim lifecycle fixture: %w", err)
	}
	rollback := func(cause error) error {
		if err := tx.Rollback(); err != nil && !errors.Is(err, sql.ErrTxDone) {
			return errors.Join(cause, err)
		}
		return cause
	}
	var created time.Time
	if err := tx.QueryRowContext(ctx, `SELECT created_on FROM claims WHERE id=$1`, id).Scan(&created); err != nil {
		return rollback(fmt.Errorf("finding fixture claim: %w", err))
	}
	var reviewed, closed any
	if status != claim.StatusOpen {
		reviewed = created.Add(time.Hour)
	}
	if status == claim.StatusResolved || status == claim.StatusDismissed {
		if resolution == nil {
			return rollback(claim.ErrInvalidSubmission)
		}
		closed = resolution.ResolvedOn
		if resolution.ResolvedOn.Before(created) {
			return rollback(claim.ErrInvalidSubmission)
		}
		// A deterministic review instant valid even when a fixture resolution is near creation.
		reviewed = created.Add(resolution.ResolvedOn.Sub(created) / 2)
	} else if resolution != nil {
		return rollback(claim.ErrInvalidSubmission)
	}
	if _, err := tx.ExecContext(ctx, `UPDATE claims SET status=$1,review_started_on=$2,closed_on=$3 WHERE id=$4`, status, reviewed, closed, id); err != nil {
		return rollback(err)
	}
	if resolution != nil {
		var amount, currency, unit any
		if compensation := resolution.SuggestedCompensation; compensation != nil {
			amount = compensation.AmountMinor
			currency = compensation.Currency
			unit = compensation.Unit
		}
		if _, err := tx.ExecContext(ctx, `INSERT INTO claim_resolutions(claim_id,type,reasoning,resolved_on,suggested_amount_minor,suggested_currency,suggested_unit) VALUES($1,$2,$3,$4,$5,$6,$7)`, id, resolution.Type, resolution.Reasoning, resolution.ResolvedOn, amount, currency, unit); err != nil {
			return rollback(err)
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
