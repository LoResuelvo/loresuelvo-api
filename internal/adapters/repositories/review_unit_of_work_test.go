package repositories_test

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"testing"
	"time"

	clockadapter "github.com/LoResuelvo/loresuelvo-api/internal/adapters/clock"
	"github.com/LoResuelvo/loresuelvo-api/internal/adapters/repositories"
	"github.com/LoResuelvo/loresuelvo-api/internal/domain/admin"
	"github.com/LoResuelvo/loresuelvo-api/internal/domain/audit"
	workorder "github.com/LoResuelvo/loresuelvo-api/internal/domain/work_order"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/stretchr/testify/require"
)

type reviewTransactionFixture struct {
	testContext serviceProposalRepositoryTestContext
	provider    providerWorkOrderTestFixture
	order       *workorder.WorkOrder
	operator    *admin.Admin
	unit        *repositories.ReviewUnitOfWork
	service     *workorder.AdminReviewService
	intake      *workorder.ReportReviewService
	reports     *repositories.ReviewReportRepository
}

func newReviewTransactionFixture(t *testing.T, reviewed bool) reviewTransactionFixture {
	t.Helper()
	tc := newServiceProposalRepositoryTest(t)
	provider := newProviderWorkOrderTestFixture(t, tc, "review-transaction")
	scheduled := time.Now().UTC().Truncate(time.Microsecond).Add(48 * time.Hour)
	var order *workorder.WorkOrder
	if reviewed {
		order = savePaidWorkOrderWithReviewForFixture(t, tc, provider, scheduled, uuid.NewString(), 5, "Original preserved")
	} else {
		order = savePaidWorkOrderWithoutReviewForFixture(t, tc, provider, scheduled, uuid.NewString())
	}
	operator, err := admin.NewAdmin("auth0|review-operator", "review.operator@example.com", "Operator", "Local", nil)
	require.NoError(t, err)
	_, err = tc.userRepository.Save(t.Context(), operator)
	require.NoError(t, err)
	reports := repositories.NewReviewReportRepository(tc.database)
	events := repositories.NewAuditEventRepository(tc.database)
	reviews := repositories.NewReviewModerationRepository()
	unit := repositories.NewReviewUnitOfWork(tc.database, reviews, reports, events)
	clock := clockadapter.NewSystemClock()
	service := workorder.NewAdminReviewService(repositories.NewAdminReviewReader(tc.database, reviews, reports), tc.userRepository, unit, events, clock)
	intake := workorder.NewReportReviewService(repositories.NewProviderActivityActorFinder(tc.database), tc.workOrderRepository, unit, clock)
	return reviewTransactionFixture{tc, provider, order, operator, unit, service, intake, reports}
}

func hideReviewInput(version int) workorder.ModerationInput {
	return workorder.ModerationInput{Action: "hide", Category: "abusive_language", Reason: "Verified content", ExpectedVersion: version}
}

// waitForReviewLock verifies real PostgreSQL blocking instead of assuming a scheduler delay.
func waitForReviewLock(t *testing.T, f reviewTransactionFixture, queryFragment string, count int) {
	t.Helper()
	require.Eventually(t, func() bool {
		var waiting int
		err := f.testContext.database.QueryRowContext(t.Context(), `SELECT COUNT(*) FROM pg_stat_activity WHERE datname=current_database() AND state='active' AND wait_event_type='Lock' AND query LIKE $1`, "%"+queryFragment+"%").Scan(&waiting)
		require.NoError(t, err)
		return waiting >= count
	}, 5*time.Second, 10*time.Millisecond, "expected blocked PostgreSQL operation %s", queryFragment)
}

func TestReviewUnitOfWorkConcurrentSameOperatorDecisionsConsumeVersionOnce(t *testing.T) {
	f := newReviewTransactionFixture(t, true)
	ctx, cancel := context.WithTimeout(t.Context(), 10*time.Second)
	defer cancel()
	gate, err := f.testContext.database.BeginTx(ctx, nil)
	require.NoError(t, err)
	defer func() {
		rollbackErr := gate.Rollback()
		if !errors.Is(rollbackErr, sql.ErrTxDone) {
			require.NoError(t, rollbackErr)
		}
	}()
	// Release the lock explicitly below; deferred rollback may already be done.
	_, err = gate.ExecContext(ctx, `SELECT work_order_id FROM work_order_reviews WHERE work_order_id=$1 FOR UPDATE`, f.order.ID())
	require.NoError(t, err)
	results := make(chan error, 2)
	for index := range 2 {
		go func() {
			_, err := f.service.Moderate(ctx, f.operator.AuthID(), f.order.ID(), hideReviewInput(1), fmt.Sprintf("same-operator-tab-%d", index))
			results <- err
		}()
	}
	waitForReviewLock(t, f, "SELECT work_order_id FROM work_order_reviews", 2)
	require.NoError(t, gate.Commit())
	first, second := <-results, <-results
	if first != nil {
		first, second = second, first
	}
	require.NoError(t, first)
	require.ErrorIs(t, second, workorder.ErrReviewModerationConflict)
	var decisions, events int
	require.NoError(t, f.testContext.database.QueryRowContext(ctx, `SELECT COUNT(*) FROM review_decisions WHERE work_order_id=$1`, f.order.ID()).Scan(&decisions))
	require.NoError(t, f.testContext.database.QueryRowContext(ctx, `SELECT COUNT(*) FROM audit_events WHERE operator_id=$1 AND resource_type='review_decision'`, f.operator.ID()).Scan(&events))
	require.Equal(t, 1, decisions)
	require.Equal(t, 1, events)
}

func TestReviewUnitOfWorkSerializesReportAndHideInBothOrders(t *testing.T) {
	for _, reportFirst := range []bool{true, false} {
		t.Run(fmt.Sprintf("report_first_%t", reportFirst), func(t *testing.T) {
			f := newReviewTransactionFixture(t, true)
			ctx, cancel := context.WithTimeout(t.Context(), 10*time.Second)
			defer cancel()
			gate, err := f.testContext.database.BeginTx(ctx, nil)
			require.NoError(t, err)
			defer func() {
				rollbackErr := gate.Rollback()
				if !errors.Is(rollbackErr, sql.ErrTxDone) {
					require.NoError(t, rollbackErr)
				}
			}()
			table, blockedQuery := "review_reports", "FROM review_reports WHERE"
			if !reportFirst {
				table, blockedQuery = "review_decisions", "INSERT INTO review_decisions"
			}
			_, err = gate.ExecContext(ctx, "LOCK TABLE "+table+" IN ACCESS EXCLUSIVE MODE")
			require.NoError(t, err)
			reported, hidden := make(chan error, 1), make(chan error, 1)
			report := func() {
				_, err := f.intake.Report(ctx, f.provider.providerAuthID, f.order.ID(), "abusive_language", "Provider report")
				reported <- err
			}
			hide := func() {
				_, err := f.service.Moderate(ctx, f.operator.AuthID(), f.order.ID(), hideReviewInput(1), "review-race")
				hidden <- err
			}
			if reportFirst {
				go report()
			} else {
				go hide()
			}
			waitForReviewLock(t, f, blockedQuery, 1)
			if reportFirst {
				go hide()
			} else {
				go report()
			}
			waitForReviewLock(t, f, "SELECT work_order_id FROM work_order_reviews", 1)
			require.NoError(t, gate.Commit())
			hideErr, reportErr := <-hidden, <-reported
			require.NoError(t, hideErr)
			stored, err := f.reports.FindByWorkOrderID(ctx, f.order.ID())
			if reportFirst {
				require.NoError(t, reportErr)
				require.NoError(t, err)
				require.Equal(t, "pending", stored.Status(), "an unselected report must not be handled by direct hide")
			} else {
				require.ErrorIs(t, reportErr, workorder.ErrReviewNotAvailable)
				require.ErrorIs(t, err, workorder.ErrReviewReportNotFound)
			}
		})
	}
}

func TestReviewUnitOfWorkAuditFailureRollsBackReviewReportDecisionAndIngest(t *testing.T) {
	f := newReviewTransactionFixture(t, true)
	ctx := t.Context()
	report, err := f.intake.Report(ctx, f.provider.providerAuthID, f.order.ID(), "abusive_language", "Original explanation")
	require.NoError(t, err)
	events := repositories.NewAuditEventRepository(f.testContext.database)
	event, err := audit.NewEvent(audit.EventParams{ID: uuid.New(), OperatorID: f.operator.ID(), Action: audit.ActionExecute, ResourceType: "review_decision", ResourceID: "existing", OccurredOn: time.Now(), Result: audit.ResultSucceeded, CorrelationID: "rollback-review-audit-" + uuid.NewString()})
	require.NoError(t, err)
	require.NoError(t, events.Save(ctx, event))
	var beforeCounter int64
	require.NoError(t, f.testContext.database.QueryRowContext(ctx, `SELECT last_value FROM audit_ingest_counter WHERE id=1`).Scan(&beforeCounter))
	err = f.unit.Execute(ctx, func(store workorder.ReviewStore) error {
		review, err := store.FindReview(ctx, f.order.ID())
		if err != nil {
			return err
		}
		persistedReport, err := store.FindReport(ctx, f.order.ID())
		if err != nil {
			return err
		}
		input := hideReviewInput(1)
		input.ReportID = report.ID()
		decision, err := review.Moderate(f.order.ID(), input, persistedReport, f.operator.ID(), time.Now())
		if err != nil {
			return err
		}
		if err := store.SaveDecision(ctx, decision); err != nil {
			return err
		}
		review.AssociateDecision(decision)
		if err := store.SaveReview(ctx, f.order.ID(), review); err != nil {
			return err
		}
		if err := store.SaveReport(ctx, persistedReport); err != nil {
			return err
		}
		return store.SaveAuditEvent(ctx, event)
	})
	var pgError *pgconn.PgError
	require.ErrorAs(t, err, &pgError)
	require.Equal(t, "23505", pgError.Code)
	require.Equal(t, "audit_events_pkey", pgError.ConstraintName)
	var state string
	require.NoError(t, f.testContext.database.QueryRowContext(ctx, `SELECT jsonb_build_object('visible',r.visible,'version',r.moderation_version,'hide',r.hiding_decision_id,'rating',r.rating,'description',r.description,'report_status',p.status,'explanation',p.explanation,'decisions',(SELECT COUNT(*) FROM review_decisions WHERE work_order_id=r.work_order_id),'events',(SELECT COUNT(*) FROM audit_events WHERE correlation_id=$2))::text FROM work_order_reviews r JOIN review_reports p ON p.work_order_id=r.work_order_id WHERE r.work_order_id=$1`, f.order.ID(), event.CorrelationID()).Scan(&state))
	require.JSONEq(t, `{"visible":true,"version":1,"hide":null,"rating":5,"description":"Original preserved","report_status":"pending","explanation":"Original explanation","decisions":0,"events":1}`, state)
	var afterCounter int64
	require.NoError(t, f.testContext.database.QueryRowContext(ctx, `SELECT last_value FROM audit_ingest_counter WHERE id=1`).Scan(&afterCounter))
	require.Equal(t, beforeCounter, afterCounter)
}

func TestReviewUnitOfWorkRejectsForeignAndHandledReportWithoutWritingDecision(t *testing.T) {
	f := newReviewTransactionFixture(t, true)
	ctx := t.Context()
	report, err := f.intake.Report(ctx, f.provider.providerAuthID, f.order.ID(), "abusive_language", "")
	require.NoError(t, err)
	other := savePaidWorkOrderWithReviewForFixture(t, f.testContext, f.provider, time.Now().UTC().Add(72*time.Hour), uuid.NewString(), 3, "Other")
	input := workorder.ModerationInput{Action: "dismiss_reports", Reason: "Verified content", ExpectedVersion: 1, ReportID: report.ID()}
	_, err = f.service.Moderate(ctx, f.operator.AuthID(), other.ID(), input, "foreign-report")
	require.ErrorIs(t, err, workorder.ErrInvalidReviewModeration)
	_, err = f.service.Moderate(ctx, f.operator.AuthID(), f.order.ID(), input, "first-dismiss")
	require.NoError(t, err)
	input.ExpectedVersion = 2
	_, err = f.service.Moderate(ctx, f.operator.AuthID(), f.order.ID(), input, "handled-report")
	require.ErrorIs(t, err, workorder.ErrInvalidReviewModeration)
	var count int
	require.NoError(t, f.testContext.database.QueryRowContext(ctx, `SELECT COUNT(*) FROM review_decisions WHERE work_order_id IN ($1,$2)`, f.order.ID(), other.ID()).Scan(&count))
	require.Equal(t, 1, count)
}

func TestReviewUnitOfWorkPreservesLateReportUntilExplicitHandling(t *testing.T) {
	f := newReviewTransactionFixture(t, true)
	ctx := t.Context()
	detail, err := f.service.Get(ctx, f.operator.AuthID(), f.order.ID(), workorder.ReviewPageInput{}, "loaded-before-report")
	require.NoError(t, err)
	require.Nil(t, detail.Report)
	report, err := f.intake.Report(ctx, f.provider.providerAuthID, f.order.ID(), "abusive_language", "")
	require.NoError(t, err)
	_, err = f.service.Moderate(ctx, f.operator.AuthID(), f.order.ID(), hideReviewInput(detail.Review.Version()), "hide-without-selection")
	require.NoError(t, err)
	pending, err := f.reports.FindByWorkOrderID(ctx, f.order.ID())
	require.NoError(t, err)
	require.Equal(t, "pending", pending.Status())
	input := hideReviewInput(2)
	input.ReportID = report.ID()
	_, err = f.service.Moderate(ctx, f.operator.AuthID(), f.order.ID(), input, "handle-selected-report")
	require.NoError(t, err)
	handled, err := f.reports.FindByWorkOrderID(ctx, f.order.ID())
	require.NoError(t, err)
	require.Equal(t, "upheld", handled.Status())
}
