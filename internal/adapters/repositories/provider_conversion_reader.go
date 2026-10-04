package repositories

import (
	"context"
	"database/sql"
	"errors"
	"fmt"

	"github.com/LoResuelvo/loresuelvo-api/internal/domain/provider"
	readmodel "github.com/LoResuelvo/loresuelvo-api/internal/domain/provider/read_model"
)

type ProviderConversionReader struct{ db *sql.DB }

func NewProviderConversionReader(db *sql.DB) *ProviderConversionReader {
	return &ProviderConversionReader{db: db}
}

const providerConversionStagesSQL = `
WITH conversion_cohort AS (
 SELECT sp.id FROM service_proposals sp
 WHERE sp.provider_id=$1 AND sp.created_on >= $2 AND sp.created_on < $3
), conversion_evidence AS (
 SELECT cohort.id,
 EXISTS (SELECT 1 FROM work_orders wo WHERE wo.service_proposal_id=cohort.id AND wo.accepted_on IS NOT NULL) AS contracted,
 EXISTS (SELECT 1 FROM work_orders wo WHERE wo.service_proposal_id=cohort.id AND wo.accepted_on IS NOT NULL
 AND EXISTS (SELECT 1 FROM work_order_completion_reports report WHERE report.work_order_id=wo.id)) AS reported,
 EXISTS (SELECT 1 FROM work_orders wo WHERE wo.service_proposal_id=cohort.id AND wo.accepted_on IS NOT NULL AND wo.paid_on IS NOT NULL
 AND EXISTS (SELECT 1 FROM work_order_completion_reports report WHERE report.work_order_id=wo.id)) AS paid
 FROM conversion_cohort cohort
)
SELECT COUNT(*), COUNT(*) FILTER (WHERE contracted), COUNT(*) FILTER (WHERE reported), COUNT(*) FILTER (WHERE paid)
FROM conversion_evidence`

const providerConversionRequestsSQL = `
SELECT COUNT(*), COUNT(*) FILTER (WHERE status='accepted'), COUNT(*) FILTER (WHERE status='pending')
FROM job_requests WHERE provider_id=$1 AND created_on >= $2 AND created_on < $3`

func (r *ProviderConversionReader) Read(ctx context.Context, providerID int, query provider.ConversionQuery) (_ *readmodel.ConversionSnapshot, err error) {
	if providerID <= 0 {
		return nil, errors.New("conversion provider ID must be positive")
	}
	if r.db == nil {
		return nil, errors.New("conversion database is unavailable")
	}
	tx, err := r.db.BeginTx(ctx, &sql.TxOptions{Isolation: sql.LevelRepeatableRead, ReadOnly: true})
	if err != nil {
		return nil, fmt.Errorf("beginning provider conversion snapshot: %w", err)
	}
	committed := false
	defer func() {
		if !committed {
			if rollbackErr := tx.Rollback(); rollbackErr != nil && !errors.Is(rollbackErr, sql.ErrTxDone) {
				err = errors.Join(err, fmt.Errorf("rolling back provider conversion snapshot: %w", rollbackErr))
			}
		}
	}()
	var exists bool
	if err := tx.QueryRowContext(ctx, `SELECT EXISTS (SELECT 1 FROM providers p JOIN users u ON u.id=p.user_id WHERE p.user_id=$1 AND u.role='provider')`, providerID).Scan(&exists); err != nil {
		return nil, fmt.Errorf("checking conversion provider: %w", err)
	}
	if !exists {
		return nil, provider.ErrConversionProviderNotFound
	}
	snapshot := &readmodel.ConversionSnapshot{}
	from, to := activityBound(query.From), activityBound(query.To)
	if err := tx.QueryRowContext(ctx, providerConversionStagesSQL, providerID, from, to).Scan(&snapshot.Stages.Issued, &snapshot.Stages.Contracted, &snapshot.Stages.Reported, &snapshot.Stages.Paid); err != nil {
		return nil, fmt.Errorf("reading conversion proposals: %w", err)
	}
	if err := tx.QueryRowContext(ctx, providerConversionRequestsSQL, providerID, from, to).Scan(&snapshot.Requests.Received, &snapshot.Requests.Accepted, &snapshot.Requests.Pending); err != nil {
		return nil, fmt.Errorf("reading conversion requests: %w", err)
	}
	if err := tx.Commit(); err != nil {
		return nil, fmt.Errorf("committing provider conversion snapshot: %w", err)
	}
	committed = true
	return snapshot, nil
}

var _ provider.ConversionReader = (*ProviderConversionReader)(nil)
