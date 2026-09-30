package repositories

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"

	"github.com/LoResuelvo/loresuelvo-api/internal/domain/payment"
	rm "github.com/LoResuelvo/loresuelvo-api/internal/domain/payment/read_model"
)

type AdminPaymentReader struct{ db *sql.DB }

func NewAdminPaymentReader(db *sql.DB) *AdminPaymentReader { return &AdminPaymentReader{db} }

var _ payment.AdminPaymentReader = (*AdminPaymentReader)(nil)

const adminPaymentIntentColumns = `pi.id::text,pi.service_proposal_id,pi.purpose,pi.status,pi.currency,
 pi.seller_amount_cents,pi.platform_fee_cents,pi.total_amount_cents,pi.created_on,pi.updated_on,
 sp.consumer_id,sp.provider_id,wo.id`
const adminPaymentPageSQL = `SELECT ` + adminPaymentIntentColumns + `
 FROM payment_intents pi JOIN service_proposals sp ON sp.id=pi.service_proposal_id
 JOIN users consumer ON consumer.id=sp.consumer_id JOIN users provider ON provider.id=sp.provider_id
 LEFT JOIN work_orders wo ON wo.service_proposal_id=sp.id
 WHERE ($1::uuid IS NULL OR pi.id=$1) AND ($2::uuid IS NULL OR pi.id=$2)
 AND ($3::text='' OR EXISTS(SELECT 1 FROM payment_transactions pt WHERE pt.payment_intent_id=pi.id AND pt.external_payment_id=$3))
 AND ($4::integer=0 OR sp.id=$4)
 AND ($5::text='' OR strpos(lower(consumer.email),lower($5))>0)
 AND ($6::text='' OR strpos(lower(provider.email),lower($6))>0)
 AND ($7::text='' OR pi.purpose=$7) AND ($8::text='' OR pi.status=$8)
 AND ($9::timestamptz IS NULL OR pi.created_on >= $9) AND ($10::timestamptz IS NULL OR pi.created_on < $10)
 AND ($11::timestamptz IS NULL OR (pi.created_on,pi.id)<($11,$12::uuid))
 ORDER BY pi.created_on DESC,pi.id DESC LIMIT $13`
const adminPaymentProposalsSQL = `SELECT ` + adminPaymentIntentColumns + `,sp.status,COALESCE(wo.status,''),sp.currency,sp.amount_cents,sp.deposit_cents,
 sp.platform_fee_total_cents,sp.platform_fee_due_now_cents,sp.booking_payment_deadline
 FROM payment_intents pi JOIN service_proposals sp ON sp.id=pi.service_proposal_id
 LEFT JOIN work_orders wo ON wo.service_proposal_id=sp.id
 WHERE sp.id=ANY($1::integer[]) ORDER BY sp.id,pi.created_on DESC,pi.id DESC`
const adminPaymentTransactionsSQL = `SELECT pt.id,pt.payment_intent_id::text,pt.processor,pt.external_payment_id,pt.status,pt.currency,
 pt.amount_cents,pt.verified_on,pt.created_on,pt.updated_on
 FROM payment_transactions pt JOIN payment_intents pi ON pi.id=pt.payment_intent_id
 WHERE pi.service_proposal_id=ANY($1::integer[]) ORDER BY pt.verified_on,pt.id`

func (r *AdminPaymentReader) FindPage(ctx context.Context, q payment.AdminPaymentQuery) (result *rm.AdminPaymentSnapshot, err error) {
	if err = q.Validate(); err != nil {
		return nil, err
	}
	tx, err := r.db.BeginTx(ctx, &sql.TxOptions{Isolation: sql.LevelRepeatableRead, ReadOnly: true})
	if err != nil {
		return nil, fmt.Errorf("beginning administrative payment snapshot: %w", err)
	}
	committed := false
	defer func() {
		if !committed {
			if e := tx.Rollback(); e != nil && !errors.Is(e, sql.ErrTxDone) {
				err = errors.Join(err, fmt.Errorf("rolling back administrative payments: %w", e))
			}
		}
	}()
	snapshot := &rm.AdminPaymentSnapshot{Payments: make([]rm.AdminPayment, 0), Proposals: make([]rm.AdminPaymentProposal, 0)}
	var afterTime, afterID any
	if q.After != nil {
		afterTime = q.After.CreatedOn.UTC()
		afterID = q.After.ID
	}
	rows, err := tx.QueryContext(ctx, adminPaymentPageSQL, adminPaymentIDArg(q.PaymentIntentID), adminPaymentIDArg(q.ExternalReference), q.ExternalPaymentID, q.ServiceProposalID, q.ConsumerEmail, q.ProviderEmail, string(q.Purpose), q.IntentStatus, adminPaymentTimeArg(q.CreatedFrom), adminPaymentTimeArg(q.CreatedTo), afterTime, afterID, q.Limit+1)
	if err != nil {
		return nil, fmt.Errorf("reading administrative payment page: %w", err)
	}
	for rows.Next() {
		var p rm.AdminPayment
		var order sql.NullInt64
		if e := rows.Scan(adminPaymentScanArgs(&p, &order)...); e != nil {
			return nil, adminPaymentRowsError(rows, "scanning administrative payment", e)
		}
		adminPaymentNormalize(&p, order)
		snapshot.Payments = append(snapshot.Payments, p)
	}
	if err = adminPaymentRowsFinish(rows); err != nil {
		return nil, err
	}
	snapshot.HasMore = len(snapshot.Payments) > q.Limit
	if snapshot.HasMore {
		snapshot.Payments = snapshot.Payments[:q.Limit]
	}
	if len(snapshot.Payments) > 0 {
		if err = adminPaymentReadProposals(ctx, tx, snapshot); err != nil {
			return nil, err
		}
	}
	if err = tx.Commit(); err != nil {
		return nil, fmt.Errorf("committing administrative payment snapshot: %w", err)
	}
	committed = true
	return snapshot, nil
}
func adminPaymentReadProposals(ctx context.Context, tx *sql.Tx, s *rm.AdminPaymentSnapshot) error {
	ids := make([]int, 0, len(s.Payments))
	seen := map[int]bool{}
	for _, p := range s.Payments {
		if !seen[p.ServiceProposalID] {
			ids = append(ids, p.ServiceProposalID)
			seen[p.ServiceProposalID] = true
		}
	}
	rows, err := tx.QueryContext(ctx, adminPaymentProposalsSQL, ids)
	if err != nil {
		return fmt.Errorf("reading administrative payment contracts: %w", err)
	}
	proposalPositions := map[int]int{}
	for rows.Next() {
		var i rm.AdminPayment
		var p rm.AdminPaymentProposal
		var order sql.NullInt64
		args := adminPaymentScanArgs(&i, &order)
		args = append(args, &p.Status, &p.WorkOrderStatus, &p.Breakdown.Currency, &p.Breakdown.ServiceTotalCents, &p.Breakdown.DepositCents, &p.Breakdown.PlatformFeeTotalCents, &p.Breakdown.PlatformFeeDueNowCents, &p.Breakdown.BookingPaymentDeadline)
		if e := rows.Scan(args...); e != nil {
			return adminPaymentRowsError(rows, "scanning administrative payment contract", e)
		}
		adminPaymentNormalize(&i, order)
		p.ID = i.ServiceProposalID
		p.WorkOrderID = i.WorkOrderID
		p.Breakdown.BookingPaymentDeadline = p.Breakdown.BookingPaymentDeadline.UTC()
		position, ok := proposalPositions[p.ID]
		if !ok {
			position = len(s.Proposals)
			proposalPositions[p.ID] = position
			s.Proposals = append(s.Proposals, p)
		}
		s.Proposals[position].Intents = append(s.Proposals[position].Intents, i)
	}
	if err = adminPaymentRowsFinish(rows); err != nil {
		return err
	}
	transactions := map[string][]rm.AdminPaymentTransaction{}
	rows, err = tx.QueryContext(ctx, adminPaymentTransactionsSQL, ids)
	if err != nil {
		return fmt.Errorf("reading administrative payment transactions: %w", err)
	}
	for rows.Next() {
		var t rm.AdminPaymentTransaction
		var intentID string
		if e := rows.Scan(&t.ID, &intentID, &t.Processor, &t.ExternalPaymentID, &t.Status, &t.Currency, &t.AmountCents, &t.VerifiedOn, &t.CreatedOn, &t.UpdatedOn); e != nil {
			return adminPaymentRowsError(rows, "scanning administrative payment transaction", e)
		}
		t.VerifiedOn = t.VerifiedOn.UTC()
		t.CreatedOn = t.CreatedOn.UTC()
		t.UpdatedOn = t.UpdatedOn.UTC()
		transactions[intentID] = append(transactions[intentID], t)
	}
	if err = adminPaymentRowsFinish(rows); err != nil {
		return err
	}
	for n := range s.Proposals {
		for j := range s.Proposals[n].Intents {
			i := &s.Proposals[n].Intents[j]
			i.Transactions = transactions[i.ID]
		}
	}
	for n := range s.Payments {
		s.Payments[n].Transactions = transactions[s.Payments[n].ID]
	}
	return nil
}
func adminPaymentScanArgs(p *rm.AdminPayment, order *sql.NullInt64) []any {
	return []any{&p.ID, &p.ServiceProposalID, &p.Purpose, &p.IntentStatus, &p.Currency, &p.SellerAmountCents, &p.PlatformFeeCents, &p.TotalAmountCents, &p.CreatedOn, &p.UpdatedOn, &p.ConsumerID, &p.ProviderID, order}
}
func adminPaymentNormalize(p *rm.AdminPayment, order sql.NullInt64) {
	p.CreatedOn = p.CreatedOn.UTC()
	p.UpdatedOn = p.UpdatedOn.UTC()
	if order.Valid {
		id := int(order.Int64)
		p.WorkOrderID = &id
	}
}
func adminPaymentIDArg(id string) any {
	if id == "" {
		return nil
	}
	return id
}
func adminPaymentRowsError(rows *sql.Rows, operation string, err error) error {
	return errors.Join(fmt.Errorf("%s: %w", operation, err), rows.Close())
}
func adminPaymentRowsFinish(rows *sql.Rows) error { return errors.Join(rows.Err(), rows.Close()) }

func adminPaymentTimeArg(t *time.Time) any {
	if t == nil {
		return nil
	}
	return activityBound(*t)
}
