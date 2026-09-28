package repositories

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"github.com/LoResuelvo/loresuelvo-api/internal/domain/admin"
	readmodel "github.com/LoResuelvo/loresuelvo-api/internal/domain/admin/read_model"
	operationmodel "github.com/LoResuelvo/loresuelvo-api/internal/domain/operation/read_model"
	"time"
)

type ConsumerHistoryReader struct{ db *sql.DB }

func NewConsumerHistoryReader(db *sql.DB) *ConsumerHistoryReader { return &ConsumerHistoryReader{db} }

const consumerHistoryProfileSQL = `SELECT u.id,u.name,u.surname,u.email,COALESCE(u.profile_photo_file_id::text,''),u.created_on,
 a.street,a.street_number,NULLIF(a.floor,''),NULLIF(a.unit,''),z.id,z.name,z.enabled
 FROM consumers c JOIN users u ON u.id=c.user_id
 LEFT JOIN consumer_addresses a ON a.consumer_id=u.id LEFT JOIN coverage_zones z ON z.id=a.coverage_zone_id
 WHERE u.id=$1 AND u.role='consumer'`
const consumerHistorySummarySQL = `SELECT
 (SELECT count(*) FROM job_requests WHERE consumer_id=$1),
 (SELECT count(*) FROM service_proposals WHERE consumer_id=$1),
 (SELECT count(*) FROM work_orders wo JOIN service_proposals sp ON sp.id=wo.service_proposal_id WHERE sp.consumer_id=$1)`

// The bounded union projects each resource exactly once; payment attempts and private media never join it.
const consumerHistoryPageSQL = `WITH resources AS (
 SELECT 'job_request'::text AS type,jr.id,jr.status,jr.provider_id,jr.created_on AS occurred_on,
 jr.id AS job_request_id,NULL::integer AS proposal_id,jr.created_on,NULL::timestamp AS scheduled_on,
 NULL::integer AS duration,NULL::timestamp AS deadline,NULL::timestamp AS accepted_on,
 NULL::timestamp AS completion_on,NULL::timestamp AS paid_on,'jr'::text AS operation_kind,jr.id AS operation_id
 FROM job_requests jr WHERE jr.consumer_id=$1
 UNION ALL
 SELECT 'service_proposal',sp.id,sp.status,sp.provider_id,sp.created_on,jr.id,sp.id,sp.created_on,
 sp.scheduled_on,sp.estimated_duration_minutes,sp.booking_payment_deadline,NULL,NULL,NULL,
 CASE WHEN jr.id IS NOT NULL AND sp.id=(SELECT min(first.id) FROM service_proposals first WHERE first.conversation_id=sp.conversation_id) THEN 'jr' ELSE 'sp' END,
 CASE WHEN jr.id IS NOT NULL AND sp.id=(SELECT min(first.id) FROM service_proposals first WHERE first.conversation_id=sp.conversation_id) THEN jr.id ELSE sp.id END
 FROM service_proposals sp LEFT JOIN job_requests jr ON jr.conversation_id=sp.conversation_id WHERE sp.consumer_id=$1
 UNION ALL
 SELECT 'work_order',wo.id,wo.status,sp.provider_id,wo.accepted_on,jr.id,sp.id,NULL,NULL,NULL,NULL,
 wo.accepted_on,report.reported_on,wo.paid_on,
 CASE WHEN jr.id IS NOT NULL AND sp.id=(SELECT min(first.id) FROM service_proposals first WHERE first.conversation_id=sp.conversation_id) THEN 'jr' ELSE 'sp' END,
 CASE WHEN jr.id IS NOT NULL AND sp.id=(SELECT min(first.id) FROM service_proposals first WHERE first.conversation_id=sp.conversation_id) THEN jr.id ELSE sp.id END
 FROM work_orders wo JOIN service_proposals sp ON sp.id=wo.service_proposal_id
 LEFT JOIN job_requests jr ON jr.conversation_id=sp.conversation_id
 LEFT JOIN work_order_completion_reports report ON report.work_order_id=wo.id WHERE sp.consumer_id=$1
), page AS (
 SELECT * FROM resources WHERE ($2::text='' OR type=$2) AND ($3::text='' OR status=$3)
 AND ($4::integer=0 OR provider_id=$4) AND ($5::timestamp IS NULL OR occurred_on >=$5)
 AND ($6::timestamp IS NULL OR occurred_on <$6)
 AND ($7::timestamp IS NULL OR (occurred_on,type COLLATE "C",id)<($7,$8::text COLLATE "C",$9::integer))
 ORDER BY occurred_on DESC,type COLLATE "C" DESC,id DESC LIMIT $10
)
 SELECT page.type,page.id,page.status,u.id,u.name,u.surname,page.occurred_on,
 page.job_request_id,page.proposal_id,page.created_on,page.scheduled_on,page.duration,page.deadline,
 page.accepted_on,page.completion_on,page.paid_on,page.operation_kind,page.operation_id
 FROM page JOIN users u ON u.id=page.provider_id
 ORDER BY page.occurred_on DESC,page.type COLLATE "C" DESC,page.id DESC`

func (r *ConsumerHistoryReader) FindByConsumerID(ctx context.Context, id int, q admin.ConsumerHistoryQuery) (result *readmodel.ConsumerHistory, err error) {
	if err = q.Validate(); err != nil {
		return nil, err
	}
	tx, err := r.db.BeginTx(ctx, &sql.TxOptions{Isolation: sql.LevelRepeatableRead, ReadOnly: true})
	if err != nil {
		return nil, fmt.Errorf("beginning consumer history read: %w", err)
	}
	committed := false
	defer func() {
		if !committed {
			if e := tx.Rollback(); e != nil && !errors.Is(e, sql.ErrTxDone) {
				err = errors.Join(err, fmt.Errorf("rolling back consumer history: %w", e))
			}
		}
	}()
	h := &readmodel.ConsumerHistory{Items: make([]readmodel.ConsumerHistoryItem, 0)}
	var street, number, floor, unit, zoneName sql.NullString
	var zoneID sql.NullInt64
	var enabled sql.NullBool
	err = tx.QueryRowContext(ctx, consumerHistoryProfileSQL, id).Scan(&h.Consumer.ID, &h.Consumer.Name, &h.Consumer.Surname, &h.Consumer.Email, &h.Consumer.ProfilePhotoFileID, &h.Consumer.CreatedOn, &street, &number, &floor, &unit, &zoneID, &zoneName, &enabled)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("reading consumer history profile: %w", err)
	}
	h.Consumer.CreatedOn = h.Consumer.CreatedOn.UTC()
	if street.Valid {
		h.Address = &readmodel.ConsumerHistoryAddress{Street: street.String, StreetNumber: number.String, Floor: consumerHistoryString(floor), Unit: consumerHistoryString(unit)}
	}
	if zoneID.Valid {
		h.CoverageZone = &readmodel.ConsumerHistoryZone{ID: int(zoneID.Int64), Name: zoneName.String, Enabled: enabled.Bool}
	}
	if err = tx.QueryRowContext(ctx, consumerHistorySummarySQL, id).Scan(&h.Summary.JobRequests, &h.Summary.ServiceProposals, &h.Summary.WorkOrders); err != nil {
		return nil, fmt.Errorf("reading consumer history summary: %w", err)
	}
	var afterOn any
	var afterType string
	var afterID int
	if q.After != nil {
		afterOn = q.After.OccurredOn.UTC()
		afterType = q.After.Type
		afterID = q.After.ID
	}
	rows, err := tx.QueryContext(ctx, consumerHistoryPageSQL, id, q.Type, q.Status, q.ProviderID, consumerHistoryTimeArg(q.From), consumerHistoryTimeArg(q.To), afterOn, afterType, afterID, q.Limit+1)
	if err != nil {
		return nil, fmt.Errorf("reading consumer history page: %w", err)
	}
	for rows.Next() {
		var i readmodel.ConsumerHistoryItem
		var request, proposal, duration sql.NullInt64
		var created, scheduled, deadline, accepted, completion, paid sql.NullTime
		var kind string
		if e := rows.Scan(&i.Type, &i.ID, &i.Status, &i.Provider.ID, &i.Provider.Name, &i.Provider.Surname, &i.OccurredOn, &request, &proposal, &created, &scheduled, &duration, &deadline, &accepted, &completion, &paid, &kind, &i.Operation.ResourceID); e != nil {
			rows.Close()
			return nil, fmt.Errorf("scanning consumer history item: %w", e)
		}
		i.OccurredOn = i.OccurredOn.UTC()
		if request.Valid {
			v := int(request.Int64)
			i.JobRequestID = &v
		}
		i.ServiceProposalID = int(proposal.Int64)
		i.CreatedOn = created.Time.UTC()
		i.ScheduledOn = scheduled.Time.UTC()
		i.EstimatedDurationMinutes = int(duration.Int64)
		i.BookingPaymentDeadline = deadline.Time.UTC()
		i.AcceptedOn = accepted.Time.UTC()
		i.CompletionReportedOn = diagnosticTime(completion)
		i.BalancePaidOn = diagnosticTime(paid)
		i.Operation.Kind = operationmodel.Kind(kind)
		h.Items = append(h.Items, i)
	}
	if e := rows.Err(); e != nil {
		rows.Close()
		return nil, fmt.Errorf("iterating consumer history page: %w", e)
	}
	if e := rows.Close(); e != nil {
		return nil, fmt.Errorf("closing consumer history page: %w", e)
	}
	if len(h.Items) > q.Limit {
		h.HasMore = true
		h.Items = h.Items[:q.Limit]
	}
	if err = tx.Commit(); err != nil {
		return nil, fmt.Errorf("committing consumer history read: %w", err)
	}
	committed = true
	return h, nil
}
func consumerHistoryString(v sql.NullString) *string {
	if !v.Valid {
		return nil
	}
	return &v.String
}
func consumerHistoryTimeArg(v *time.Time) any {
	if v == nil {
		return nil
	}
	// PostgreSQL TIMESTAMP evidence lies on a microsecond grid. Ceil both
	// inclusive lower and exclusive upper bounds to retain exact nanosecond
	// comparisons without silently truncating a caller's RFC3339 instant.
	utc := v.UTC()
	lower := utc.Truncate(time.Microsecond)
	if utc.Equal(lower) {
		return lower
	}
	return lower.Add(time.Microsecond)
}
