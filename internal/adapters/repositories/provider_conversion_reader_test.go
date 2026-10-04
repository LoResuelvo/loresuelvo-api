package repositories_test

import (
	"errors"
	"github.com/DATA-DOG/go-sqlmock"
	"github.com/LoResuelvo/loresuelvo-api/internal/adapters/repositories"
	"github.com/LoResuelvo/loresuelvo-api/internal/domain/provider"
	readmodel "github.com/LoResuelvo/loresuelvo-api/internal/domain/provider/read_model"
	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
	"testing"
	"time"
)

func TestProviderConversionReaderUnitRollsBackFailures(t *testing.T) {
	failure := errors.New("unavailable")
	for _, stage := range []string{"begin", "actor", "missing actor", "proposals", "requests", "commit"} {
		t.Run(stage, func(t *testing.T) {
			db, mock, err := sqlmock.New()
			require.NoError(t, err)
			defer db.Close()
			if stage == "begin" {
				mock.ExpectBegin().WillReturnError(failure)
			} else {
				mock.ExpectBegin()
				actor := mock.ExpectQuery("SELECT EXISTS").WithArgs(42)
				if stage == "actor" {
					actor.WillReturnError(failure)
				} else if stage == "missing actor" {
					actor.WillReturnRows(sqlmock.NewRows([]string{"exists"}).AddRow(false))
				} else {
					actor.WillReturnRows(sqlmock.NewRows([]string{"exists"}).AddRow(true))
					proposals := mock.ExpectQuery("WITH conversion_cohort AS").WithArgs(42, sqlmock.AnyArg(), sqlmock.AnyArg())
					if stage == "proposals" {
						proposals.WillReturnError(failure)
					} else {
						proposals.WillReturnRows(sqlmock.NewRows([]string{"issued", "contracted", "reported", "paid"}).AddRow(4, 2, 1, 1))
						requests := mock.ExpectQuery("SELECT COUNT").WithArgs(42, sqlmock.AnyArg(), sqlmock.AnyArg())
						if stage == "requests" {
							requests.WillReturnError(failure)
						} else {
							requests.WillReturnRows(sqlmock.NewRows([]string{"received", "accepted", "pending"}).AddRow(3, 2, 1))
							mock.ExpectCommit().WillReturnError(failure)
						}
					}
				}
				if stage != "commit" {
					mock.ExpectRollback()
				}
			}
			result, err := repositories.NewProviderConversionReader(db).Read(t.Context(), 42, provider.ConversionQuery{From: time.Now().Add(-time.Hour), To: time.Now()})
			require.Nil(t, result)
			require.Error(t, err)
			if stage == "missing actor" {
				require.ErrorIs(t, err, provider.ErrConversionProviderNotFound)
			} else {
				require.ErrorIs(t, err, failure)
			}
			require.NoError(t, mock.ExpectationsWereMet())
		})
	}
}
func TestProviderConversionReaderUnitCommitsAndCeilsBounds(t *testing.T) {
	db, mock, err := sqlmock.New()
	require.NoError(t, err)
	defer db.Close()
	instant := time.Date(2026, 9, 1, 0, 0, 0, 0, time.UTC)
	from, to := instant.Add(time.Nanosecond), instant.Add(time.Microsecond+time.Nanosecond)
	mock.ExpectBegin()
	mock.ExpectQuery("SELECT EXISTS").WithArgs(42).WillReturnRows(sqlmock.NewRows([]string{"exists"}).AddRow(true))
	mock.ExpectQuery("WITH conversion_cohort AS").WithArgs(42, instant.Add(time.Microsecond), instant.Add(2*time.Microsecond)).WillReturnRows(sqlmock.NewRows([]string{"issued", "contracted", "reported", "paid"}).AddRow(4, 2, 1, 1))
	mock.ExpectQuery("SELECT COUNT").WithArgs(42, instant.Add(time.Microsecond), instant.Add(2*time.Microsecond)).WillReturnRows(sqlmock.NewRows([]string{"received", "accepted", "pending"}).AddRow(3, 2, 1))
	mock.ExpectCommit()
	result, err := repositories.NewProviderConversionReader(db).Read(t.Context(), 42, provider.ConversionQuery{From: from, To: to})
	require.NoError(t, err)
	require.Equal(t, &readmodel.ConversionSnapshot{Stages: readmodel.ConversionStages{Issued: 4, Contracted: 2, Reported: 1, Paid: 1}, Requests: readmodel.ConversionRequestCounts{Received: 3, Accepted: 2, Pending: 1}}, result)
	require.NoError(t, mock.ExpectationsWereMet())
}

func TestProviderConversionReaderCountsCohortNotMilestonePeriod(t *testing.T) {
	testContext := newServiceProposalRepositoryTest(t)
	fixture := newProviderWorkOrderTestFixture(t, testContext, "conversion-cohort")
	base := time.Now().UTC().Truncate(time.Microsecond)
	first := savePaidWorkOrderWithoutReviewForFixture(t, testContext, fixture, base.Add(26*time.Hour), uuid.NewString())
	second := savePaidWorkOrderWithoutReviewForFixture(t, testContext, fixture, base.Add(27*time.Hour), uuid.NewString())
	old := savePaidWorkOrderWithoutReviewForFixture(t, testContext, fixture, base.Add(28*time.Hour), uuid.NewString())
	addActivityCompletionImages(t, testContext, fixture.providerAuthID, first.ID(), 2)
	for index := 0; index < 2; index++ {
		_, err := testContext.database.ExecContext(t.Context(), `INSERT INTO payment_intents (id,service_proposal_id,purpose,currency,seller_amount_cents,platform_fee_cents,total_amount_cents,status,created_on,updated_on) VALUES($1,$2,'service_balance','ARS',100,10,110,'rejected',$3,$3)`, uuid.NewString(), second.ServiceProposalID(), base)
		require.NoError(t, err)
	}

	for _, order := range []struct {
		proposal, order int
		created         time.Time
	}{{first.ServiceProposalID(), first.ID(), base.Add(-2 * time.Hour)}, {second.ServiceProposalID(), second.ID(), base.Add(-time.Hour)}, {old.ServiceProposalID(), old.ID(), base.Add(-48 * time.Hour)}} {
		_, err := testContext.database.ExecContext(t.Context(), `UPDATE service_proposals SET created_on=$1 WHERE id=$2`, order.created, order.proposal)
		require.NoError(t, err)
		setActivityOrderTimes(t, testContext, order.order, order.proposal, base.Add(time.Hour), base.Add(2*time.Hour), base.Add(3*time.Hour), 10001)
	}
	snapshot, err := repositories.NewProviderConversionReader(testContext.database).Read(t.Context(), fixture.providerID, provider.ConversionQuery{From: base.Add(-24 * time.Hour), To: base})
	require.NoError(t, err)
	require.Equal(t, readmodel.ConversionStages{Issued: 2, Contracted: 2, Reported: 2, Paid: 2}, snapshot.Stages)
}

func TestProviderConversionReaderPartitionsRequestsIndependently(t *testing.T) {
	testContext := newServiceProposalRepositoryTest(t)
	first := newProviderWorkOrderTestFixture(t, testContext, "conversion-requests-first")
	other := newProviderWorkOrderTestFixture(t, testContext, "conversion-requests-other")
	base := time.Now().UTC().Truncate(time.Microsecond)
	for _, fixture := range []providerWorkOrderTestFixture{first, other} {
		_, err := testContext.database.ExecContext(t.Context(), `INSERT INTO job_requests(consumer_id,provider_id,conversation_id,title,status,created_on) VALUES($1,$2,$3,'Request','accepted',$4)`, fixture.consumerID, fixture.providerID, fixture.conversation.ID(), base)
		require.NoError(t, err)
	}
	reader := repositories.NewProviderConversionReader(testContext.database)
	result, err := reader.Read(t.Context(), first.providerID, provider.ConversionQuery{From: base, To: base.Add(time.Hour)})
	require.NoError(t, err)
	require.Equal(t, readmodel.ConversionStages{}, result.Stages)
	require.Equal(t, readmodel.ConversionRequestCounts{Received: 1, Accepted: 1, Pending: 0}, result.Requests)
	// Updating status does not change membership or require an acceptance date.
	_, err = testContext.database.ExecContext(t.Context(), `UPDATE job_requests SET status='pending' WHERE provider_id=$1`, first.providerID)
	require.NoError(t, err)
	result, err = reader.Read(t.Context(), first.providerID, provider.ConversionQuery{From: base, To: base.Add(time.Hour)})
	require.NoError(t, err)
	require.Equal(t, readmodel.ConversionRequestCounts{Received: 1, Accepted: 0, Pending: 1}, result.Requests)
	for _, query := range []provider.ConversionQuery{{From: base.Add(-time.Hour), To: base}, {From: base.Add(time.Nanosecond), To: base.Add(time.Hour)}} {
		result, err = reader.Read(t.Context(), first.providerID, query)
		require.NoError(t, err)
		require.Equal(t, readmodel.ConversionRequestCounts{}, result.Requests)
	}
}

func TestProviderConversionReaderNestedEvidenceIgnoresCurrentStatuses(t *testing.T) {
	testContext := newServiceProposalRepositoryTest(t)
	fixture := newProviderWorkOrderTestFixture(t, testContext, "conversion-nested")
	base := time.Now().UTC().Truncate(time.Microsecond)
	for index := 0; index < 5; index++ {
		order := savePaidWorkOrderWithoutReviewForFixture(t, testContext, fixture, base.Add(time.Duration(26+index)*time.Hour), uuid.NewString())
		_, err := testContext.database.ExecContext(t.Context(), `UPDATE service_proposals SET created_on=$1 WHERE id=$2`, base, order.ServiceProposalID())
		require.NoError(t, err)
		switch index {
		case 1:
			_, err = testContext.database.ExecContext(t.Context(), `UPDATE work_orders SET paid_on=NULL WHERE id=$1`, order.ID())
		case 2:
			_, err = testContext.database.ExecContext(t.Context(), `DELETE FROM work_order_completion_reports WHERE work_order_id=$1`, order.ID())
		case 3:
			_, err = testContext.database.ExecContext(t.Context(), `DELETE FROM work_orders WHERE id=$1`, order.ID())
		case 4:
			_, err = testContext.database.ExecContext(t.Context(), `DELETE FROM work_orders WHERE id=$1`, order.ID())
		}
		require.NoError(t, err)
	}
	snapshot, err := repositories.NewProviderConversionReader(testContext.database).Read(t.Context(), fixture.providerID, provider.ConversionQuery{From: base, To: base.Add(time.Hour)})
	require.NoError(t, err)
	require.Equal(t, readmodel.ConversionStages{Issued: 5, Contracted: 3, Reported: 2, Paid: 1}, snapshot.Stages)
}

func TestProviderConversionReaderRoundsCreationBoundsAndIsolatesProviders(t *testing.T) {
	testContext := newServiceProposalRepositoryTest(t)
	fixture := newProviderWorkOrderTestFixture(t, testContext, "conversion-bounds")
	other := newProviderWorkOrderTestFixture(t, testContext, "conversion-bounds-other")
	instant := time.Now().UTC().Truncate(time.Microsecond)
	for index, created := range []time.Time{instant.Add(-time.Microsecond), instant, instant.Add(time.Microsecond), instant.Add(2 * time.Microsecond)} {
		order := saveScheduledWorkOrderAt(t, testContext, fixture.conversation, fixture.consumerID, fixture.providerID, instant.Add(time.Duration(26+index)*time.Hour))
		_, err := testContext.database.ExecContext(t.Context(), `UPDATE service_proposals SET created_on=$1 WHERE id=$2`, created, order.ServiceProposalID())
		require.NoError(t, err)
	}
	order := saveScheduledWorkOrderAt(t, testContext, other.conversation, other.consumerID, other.providerID, instant.Add(30*time.Hour))
	_, err := testContext.database.ExecContext(t.Context(), `UPDATE service_proposals SET created_on=$1 WHERE id=$2`, instant, order.ServiceProposalID())
	require.NoError(t, err)
	reader := repositories.NewProviderConversionReader(testContext.database)
	for _, query := range []provider.ConversionQuery{{From: instant, To: instant.Add(time.Microsecond)}, {From: instant.Add(time.Nanosecond), To: instant.Add(2 * time.Microsecond)}, {From: instant.Add(-time.Microsecond), To: instant.Add(-time.Nanosecond)}} {
		snapshot, err := reader.Read(t.Context(), fixture.providerID, query)
		require.NoError(t, err)
		require.Equal(t, int64(1), snapshot.Stages.Issued)
	}
}

func TestProviderConversionReaderKeepsSnapshotAcrossProposalsAndRequests(t *testing.T) {
	testContext := newServiceProposalRepositoryTest(t)
	fixture := newProviderWorkOrderTestFixture(t, testContext, "conversion-snapshot")
	base := time.Now().UTC().Truncate(time.Microsecond)
	order := saveScheduledWorkOrderAt(t, testContext, fixture.conversation, fixture.consumerID, fixture.providerID, base.Add(26*time.Hour))
	_, err := testContext.database.ExecContext(t.Context(), `UPDATE service_proposals SET created_on=$1 WHERE id=$2`, base, order.ServiceProposalID())
	require.NoError(t, err)
	_, err = testContext.database.ExecContext(t.Context(), `INSERT INTO job_requests(consumer_id,provider_id,conversation_id,title,status,created_on) VALUES($1,$2,$3,'Request','pending',$4)`, fixture.consumerID, fixture.providerID, fixture.conversation.ID(), base)
	require.NoError(t, err)
	writer, err := testContext.database.BeginTx(t.Context(), nil)
	require.NoError(t, err)
	defer func() { _ = writer.Rollback() }()
	_, err = writer.ExecContext(t.Context(), `LOCK TABLE job_requests IN ACCESS EXCLUSIVE MODE`)
	require.NoError(t, err)
	type readResult struct {
		snapshot *readmodel.ConversionSnapshot
		err      error
	}
	resultCh := make(chan readResult, 1)
	go func() {
		snapshot, readErr := repositories.NewProviderConversionReader(testContext.database).Read(t.Context(), fixture.providerID, provider.ConversionQuery{From: base, To: base.Add(time.Hour)})
		resultCh <- readResult{snapshot, readErr}
	}()
	deadline := time.Now().Add(5 * time.Second)
	blocked := false
	for time.Now().Before(deadline) {
		var waiting bool
		err := testContext.database.QueryRowContext(t.Context(), `SELECT EXISTS(SELECT 1 FROM pg_stat_activity WHERE datname=current_database() AND state='active' AND wait_event_type='Lock' AND query LIKE '%FROM job_requests WHERE provider_id=$1 AND created_on%')`).Scan(&waiting)
		require.NoError(t, err)
		if waiting {
			blocked = true
			break
		}
		time.Sleep(10 * time.Millisecond)
	}
	if !blocked {
		require.NoError(t, writer.Rollback())
		t.Fatal("reader did not reach the request aggregate barrier")
	}
	_, err = writer.ExecContext(t.Context(), `UPDATE job_requests SET status='accepted' WHERE provider_id=$1`, fixture.providerID)
	require.NoError(t, err)
	_, err = writer.ExecContext(t.Context(), `UPDATE service_proposals SET created_on=$1 WHERE id=$2`, base.Add(-time.Hour), order.ServiceProposalID())
	require.NoError(t, err)
	require.NoError(t, writer.Commit())
	select {
	case result := <-resultCh:
		require.NoError(t, result.err)
		require.Equal(t, readmodel.ConversionStages{Issued: 1, Contracted: 1, Reported: 0, Paid: 0}, result.snapshot.Stages)
		require.Equal(t, readmodel.ConversionRequestCounts{Received: 1, Accepted: 0, Pending: 1}, result.snapshot.Requests)
	case <-time.After(5 * time.Second):
		t.Fatal("reader did not finish after concurrent commit")
	}
	final, err := repositories.NewProviderConversionReader(testContext.database).Read(t.Context(), fixture.providerID, provider.ConversionQuery{From: base, To: base.Add(time.Hour)})
	require.NoError(t, err)
	require.Equal(t, readmodel.ConversionStages{}, final.Stages)
	require.Equal(t, readmodel.ConversionRequestCounts{Received: 1, Accepted: 1, Pending: 0}, final.Requests)
}

func TestProviderConversionReaderUnitRejectsUnavailableInputs(t *testing.T) {
	result, err := repositories.NewProviderConversionReader(nil).Read(t.Context(), 42, provider.ConversionQuery{})
	require.Error(t, err)
	require.Nil(t, result)
	db, mock, err := sqlmock.New()
	require.NoError(t, err)
	defer db.Close()
	result, err = repositories.NewProviderConversionReader(db).Read(t.Context(), 0, provider.ConversionQuery{})
	require.Error(t, err)
	require.Nil(t, result)
	require.NoError(t, mock.ExpectationsWereMet())
}
func TestProviderConversionReaderUnitPreservesRollbackFailure(t *testing.T) {
	db, mock, err := sqlmock.New()
	require.NoError(t, err)
	defer db.Close()
	readFailure := errors.New("read failed")
	rollbackFailure := errors.New("rollback failed")
	mock.ExpectBegin()
	mock.ExpectQuery("SELECT EXISTS").WithArgs(42).WillReturnError(readFailure)
	mock.ExpectRollback().WillReturnError(rollbackFailure)
	result, err := repositories.NewProviderConversionReader(db).Read(t.Context(), 42, provider.ConversionQuery{})
	require.Nil(t, result)
	require.ErrorIs(t, err, readFailure)
	require.ErrorIs(t, err, rollbackFailure)
	require.NoError(t, mock.ExpectationsWereMet())
}
