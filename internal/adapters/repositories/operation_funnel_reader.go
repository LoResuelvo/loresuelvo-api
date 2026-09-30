package repositories

import (
	"context"
	"database/sql"
	"fmt"

	"github.com/LoResuelvo/loresuelvo-api/internal/domain/operation"
	readmodel "github.com/LoResuelvo/loresuelvo-api/internal/domain/operation/read_model"
)

type OperationFunnelReader struct{ db *sql.DB }

func NewOperationFunnelReader(db *sql.DB) *OperationFunnelReader {
	return &OperationFunnelReader{db: db}
}

// One statement supplies a coherent PostgreSQL snapshot for both cohorts and all intervals.
const operationFunnelSQL = `
WITH cohorts(cohort) AS (VALUES ('ai'), ('manual')),
origins AS (
 SELECT 'ai'::text AS cohort, a.id AS origin_id
 FROM problem_assessments a
 WHERE a.outcome = 'professional_required'
   AND a.created_on >= $1::timestamp AND a.created_on < $2::timestamp
   AND ($3::integer IS NULL OR a.problem_category_id = $3)
 UNION ALL
 SELECT 'manual'::text, r.id
 FROM job_requests r JOIN providers p ON p.user_id = r.provider_id
 WHERE r.source_assessment_id IS NULL
   AND r.created_on >= $1::timestamp AND r.created_on < $2::timestamp
   AND ($3::integer IS NULL OR p.category_id = $3)
),
requests AS (
 SELECT o.cohort, o.origin_id, r.id AS request_id, r.conversation_id, r.created_on
 FROM origins o JOIN job_requests r ON r.source_assessment_id = o.origin_id
 WHERE o.cohort = 'ai'
 UNION ALL
 SELECT o.cohort, o.origin_id, r.id, r.conversation_id, r.created_on
 FROM origins o JOIN job_requests r ON r.id = o.origin_id
 WHERE o.cohort = 'manual'
),
branches AS (
 SELECT r.cohort, r.origin_id, r.request_id, p.id AS proposal_id,
        p.created_on AS proposed_on, w.id AS order_id, w.accepted_on,
        report.reported_on, w.paid_on, review.work_order_id AS reviewed_order_id
 FROM requests r
 LEFT JOIN service_proposals p ON p.conversation_id = r.conversation_id
 LEFT JOIN work_orders w ON w.service_proposal_id = p.id
 LEFT JOIN work_order_completion_reports report ON report.work_order_id = w.id
 LEFT JOIN work_order_reviews review ON review.work_order_id = w.id
),
origin_flags AS (
 SELECT o.cohort, o.origin_id,
        COALESCE(bool_or(b.request_id IS NOT NULL),false) AS requested,
        COALESCE(bool_or(b.proposal_id IS NOT NULL),false) AS proposed,
        COALESCE(bool_or(b.order_id IS NOT NULL AND b.accepted_on IS NOT NULL),false) AS hired,
        COALESCE(bool_or(b.order_id IS NOT NULL AND b.accepted_on IS NOT NULL AND b.reported_on IS NOT NULL),false) AS completed,
        COALESCE(bool_or(b.order_id IS NOT NULL AND b.accepted_on IS NOT NULL AND b.reported_on IS NOT NULL AND b.paid_on IS NOT NULL),false) AS paid,
        COALESCE(bool_or(b.order_id IS NOT NULL AND b.accepted_on IS NOT NULL AND b.reported_on IS NOT NULL AND b.paid_on IS NOT NULL AND b.reviewed_order_id IS NOT NULL),false) AS reviewed
 FROM origins o LEFT JOIN branches b ON b.cohort=o.cohort AND b.origin_id=o.origin_id
 GROUP BY o.cohort, o.origin_id
),
stage_totals AS (
 SELECT cohort, count(*) AS origins, count(*) FILTER (WHERE requested) AS requested,
        count(*) FILTER (WHERE proposed) AS proposed, count(*) FILTER (WHERE hired) AS hired,
        count(*) FILTER (WHERE completed) AS completed, count(*) FILTER (WHERE paid) AS paid,
        count(*) FILTER (WHERE reviewed) AS reviewed
 FROM origin_flags GROUP BY cohort
),
first_proposals AS (
 -- Select the actual earliest proposal before excluding negative chronology.
 SELECT r.cohort, r.request_id, r.created_on, min(p.created_on) AS proposed_on
 FROM requests r LEFT JOIN service_proposals p ON p.conversation_id=r.conversation_id
 GROUP BY r.cohort,r.request_id,r.created_on
),
request_delays AS (
 SELECT cohort, count(*) AS observations,
        sum(extract(epoch FROM (proposed_on-created_on))*1000000) AS microseconds
 FROM first_proposals WHERE proposed_on >= created_on GROUP BY cohort
),
orders AS (
 SELECT DISTINCT cohort,order_id,proposed_on,accepted_on,reported_on,paid_on
 FROM branches WHERE order_id IS NOT NULL
),
order_delays AS (
 -- Each interval stands alone: anomalous chronology does not discard other samples.
 SELECT cohort,
        count(*) FILTER (WHERE accepted_on>=proposed_on) AS hiring_observations,
        sum(extract(epoch FROM (accepted_on-proposed_on))*1000000) FILTER (WHERE accepted_on>=proposed_on) AS hiring_microseconds,
        count(*) FILTER (WHERE reported_on>=accepted_on) AS completion_observations,
        sum(extract(epoch FROM (reported_on-accepted_on))*1000000) FILTER (WHERE reported_on>=accepted_on) AS completion_microseconds,
        count(*) FILTER (WHERE paid_on>=reported_on) AS payment_observations,
        sum(extract(epoch FROM (paid_on-reported_on))*1000000) FILTER (WHERE paid_on>=reported_on) AS payment_microseconds
 FROM orders GROUP BY cohort
)
SELECT c.cohort,
       COALESCE(s.origins,0),COALESCE(s.requested,0),COALESCE(s.proposed,0),COALESCE(s.hired,0),COALESCE(s.completed,0),COALESCE(s.paid,0),COALESCE(s.reviewed,0),
       COALESCE(r.observations,0),COALESCE(r.microseconds,0)::text,
       COALESCE(d.hiring_observations,0),COALESCE(d.hiring_microseconds,0)::text,
       COALESCE(d.completion_observations,0),COALESCE(d.completion_microseconds,0)::text,
       COALESCE(d.payment_observations,0),COALESCE(d.payment_microseconds,0)::text
FROM cohorts c
LEFT JOIN stage_totals s ON s.cohort=c.cohort
LEFT JOIN request_delays r ON r.cohort=c.cohort
LEFT JOIN order_delays d ON d.cohort=c.cohort`

func (r *OperationFunnelReader) Read(ctx context.Context, criteria operation.FunnelCriteria) (readmodel.FunnelSnapshot, error) {
	rows, err := r.db.QueryContext(ctx, operationFunnelSQL, activityBound(criteria.Period.From), activityBound(criteria.Period.To), criteria.CategoryID)
	if err != nil {
		return readmodel.FunnelSnapshot{}, fmt.Errorf("querying funnel snapshot: %w", err)
	}
	defer rows.Close()
	var snapshot readmodel.FunnelSnapshot
	seen := map[string]bool{}
	for rows.Next() {
		var name string
		var cohort readmodel.FunnelCohortSnapshot
		c, d := &cohort.Counts, &cohort.Delays
		if err := rows.Scan(&name, &c.Origins, &c.Requested, &c.Proposed, &c.Hired, &c.Completed, &c.Paid, &c.Reviewed,
			&d.RequestToFirstProposal.Observations, &d.RequestToFirstProposal.TotalMicroseconds,
			&d.ProposalToConfirmedHiring.Observations, &d.ProposalToConfirmedHiring.TotalMicroseconds,
			&d.ConfirmedHiringToReportedCompletion.Observations, &d.ConfirmedHiringToReportedCompletion.TotalMicroseconds,
			&d.ReportedCompletionToFullPayment.Observations, &d.ReportedCompletionToFullPayment.TotalMicroseconds); err != nil {
			return readmodel.FunnelSnapshot{}, fmt.Errorf("scanning funnel snapshot: %w", err)
		}
		if seen[name] {
			return readmodel.FunnelSnapshot{}, operation.ErrInvalidFunnelSnapshot
		}
		seen[name] = true
		switch name {
		case "ai":
			snapshot.AI = cohort
		case "manual":
			snapshot.Manual = cohort
		default:
			return readmodel.FunnelSnapshot{}, operation.ErrInvalidFunnelSnapshot
		}
	}
	if err := rows.Err(); err != nil {
		return readmodel.FunnelSnapshot{}, fmt.Errorf("reading funnel snapshot rows: %w", err)
	}
	if !seen["ai"] || !seen["manual"] {
		return readmodel.FunnelSnapshot{}, operation.ErrInvalidFunnelSnapshot
	}
	return snapshot, nil
}

var _ operation.FunnelReader = (*OperationFunnelReader)(nil)
