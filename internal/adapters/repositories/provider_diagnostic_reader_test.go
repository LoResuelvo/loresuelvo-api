package repositories_test

import (
	"context"
	"github.com/LoResuelvo/loresuelvo-api/internal/adapters/repositories"
	"github.com/LoResuelvo/loresuelvo-api/internal/domain/identityverification"
	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
	"testing"
	"time"
)

func TestProviderDiagnosticReaderReturnsPersistedSafeEvidence(t *testing.T) {
	users, db := newUserRepositoryTest(t)
	require.NoError(t, users.DeleteAll())
	p := validProviderWithData(t, repositories.NewCategoryRepository(db), db, "auth0|diagnostic-reader", "diagnostic.reader@example.com", "Juan", "Gomez", "Plomeria")
	_, err := users.Save(t.Context(), p)
	require.NoError(t, err)
	now := time.Date(2026, 9, 28, 0, 0, 0, 0, time.UTC)
	_, err = db.ExecContext(t.Context(), `INSERT INTO provider_payment_accounts(provider_id,payment_provider,external_account_id,access_token_ciphertext,token_expires_on) VALUES ($1,'mercado_pago','diagnostic-account',$2,$3)`, p.ID(), []byte("private-not-selected"), now)
	require.NoError(t, err)
	var connected time.Time
	require.NoError(t, db.QueryRowContext(t.Context(), `SELECT connected_on FROM provider_payment_accounts WHERE provider_id=$1`, p.ID()).Scan(&connected))
	_, err = db.ExecContext(t.Context(), `INSERT INTO google_calendar_connections(user_id,refresh_token_ciphertext,calendar_id,status,connected_on,updated_on) VALUES($1,$2,'private-calendar','action_required',$3,$4)`, p.ID(), []byte("private-refresh"), now.Add(-time.Hour), now)
	require.NoError(t, err)
	reader := repositories.NewProviderDiagnosticReader(db)
	result, err := reader.FindByProviderID(t.Context(), p.ID())
	require.NoError(t, err)
	require.Equal(t, p.ID(), result.Provider.ID)
	require.Equal(t, identityverification.StatusUnverified, result.Provider.IdentityVerificationStatus)
	require.Equal(t, connected.UTC(), *result.Payment.ConnectedOn)
	require.Equal(t, now, *result.Payment.TokenExpiresOn)
	require.Equal(t, "action_required", result.Calendar.State())
	require.Equal(t, now, *result.Calendar.UpdatedOn)
	require.NotNil(t, result.Provider.CoverageZones)
	missing, err := reader.FindByProviderID(t.Context(), 987654321)
	require.NoError(t, err)
	require.Nil(t, missing)
	ctx, cancel := context.WithCancel(t.Context())
	cancel()
	result, err = reader.FindByProviderID(ctx, p.ID())
	require.Nil(t, result)
	require.ErrorIs(t, err, context.Canceled)
}

func TestProviderDiagnosticReaderSuppressesStaleVerificationDateFromLatestSession(t *testing.T) {
	users, db := newUserRepositoryTest(t)
	require.NoError(t, users.DeleteAll())
	p := validProviderWithData(t, repositories.NewCategoryRepository(db), db, "auth0|diagnostic-identity", "diagnostic.identity@example.com", "Juan", "Gomez", "Plomeria")
	_, err := users.Save(t.Context(), p)
	require.NoError(t, err)
	approvedOn := time.Date(2026, 9, 18, 15, 30, 0, 0, time.UTC)
	newerOn := approvedOn.Add(time.Hour)
	for _, session := range []struct {
		status  string
		created time.Time
	}{{"approved", approvedOn}, {"declined", newerOn}} {
		_, err = db.ExecContext(t.Context(), `INSERT INTO identity_verification_sessions(external_session_id,provider_id,verifier,workflow_id,status,verified_on,last_result_on,created_on) VALUES($1,$2,'didit',$3,$4,$5,$6,$6)`, uuid.New(), p.ID(), uuid.New(), session.status, approvedOn, session.created)
		require.NoError(t, err)
	}
	result, err := repositories.NewProviderDiagnosticReader(db).FindByProviderID(t.Context(), p.ID())
	require.NoError(t, err)
	require.Equal(t, identityverification.StatusDeclined, result.Provider.IdentityVerificationStatus)
	require.Nil(t, result.Provider.IdentityVerifiedOn)
	require.Equal(t, newerOn, *result.IdentityResultOn)
	require.Equal(t, "identity_declined", result.Checks(newerOn)[0].ReasonCode)
}

func TestProviderDiagnosticReaderBoundsActivityReviewsAndScopesSynchronization(t *testing.T) {
	testContext := newServiceProposalRepositoryTest(t)
	fixture := newProviderWorkOrderTestFixture(t, testContext, "diagnostic-activity")
	other := newProviderWorkOrderTestFixture(t, testContext, "diagnostic-other")
	base := time.Now().UTC().Truncate(time.Microsecond).Add(48 * time.Hour)
	orderIDs := make([]int, 0, 8)
	for i := 0; i < 8; i++ {
		rating := 5
		if i < 2 {
			rating = 1
		}
		order := savePaidWorkOrderWithReviewForFixture(t, testContext, fixture, base.Add(time.Duration(i)*24*time.Hour), uuid.NewString(), rating, "Persisted review")
		orderIDs = append(orderIDs, order.ID())
		// Force an equal accepted time to exercise the stable ID tie-breaker.
		_, err := testContext.database.ExecContext(t.Context(), `UPDATE work_orders SET accepted_on=$1 WHERE id=$2`, base, order.ID())
		require.NoError(t, err)
		_, err = testContext.database.ExecContext(t.Context(), `UPDATE service_proposals SET created_on=$1 WHERE id=$2`, base.Add(-time.Hour), order.ServiceProposalID())
		require.NoError(t, err)
	}
	savePaidWorkOrderWithReviewForFixture(t, testContext, other, base, uuid.NewString(), 1, "Not this provider")
	last := orderIDs[len(orderIDs)-1]
	_, err := testContext.database.ExecContext(t.Context(), `INSERT INTO work_order_calendar_events(work_order_id,user_id,calendar_id,synced_on) VALUES($1,$2,'private-consumer-calendar',$3)`, last, fixture.consumerID, base)
	require.NoError(t, err)
	providerSynced := base.Add(time.Minute)
	_, err = testContext.database.ExecContext(t.Context(), `INSERT INTO work_order_calendar_events(work_order_id,user_id,calendar_id,synced_on) VALUES($1,$2,'private-provider-calendar',$3)`, orderIDs[6], fixture.providerID, providerSynced)
	require.NoError(t, err)
	reader := repositories.NewProviderDiagnosticReader(testContext.database)
	result, err := reader.FindByProviderID(t.Context(), fixture.providerID)
	require.NoError(t, err)
	require.Len(t, result.Activity, 6)
	require.Len(t, result.Reviews, 6)
	require.Len(t, result.OrderSync, 6)
	require.Equal(t, 8, result.Reputation().Count)
	require.Equal(t, float64(4), result.Reputation().Average)
	for i := 0; i < 6; i++ {
		require.Equal(t, "work_order", result.Activity[i].Type)
		require.Equal(t, orderIDs[7-i], result.Activity[i].ID)
		require.Equal(t, "paid", result.Activity[i].Status)
		require.Equal(t, base, result.Activity[i].OccurredOn)
		require.Equal(t, orderIDs[7-i], result.Reviews[i].WorkOrderID)
		require.Equal(t, 5, result.Reviews[i].Rating)
	}
	require.Equal(t, last, result.OrderSync[0].WorkOrderID)
	require.Nil(t, result.OrderSync[0].SyncedOn)
	require.Equal(t, orderIDs[6], result.OrderSync[1].WorkOrderID)
	require.Equal(t, providerSynced, *result.OrderSync[1].SyncedOn)
	again, err := reader.FindByProviderID(t.Context(), fixture.providerID)
	require.NoError(t, err)
	require.Equal(t, result, again)
	// Reads do not create retries or overwrite either participant's evidence.
	var events, attempts int
	require.NoError(t, testContext.database.QueryRowContext(t.Context(), `SELECT count(*),sum(attempt_count) FROM work_order_calendar_events WHERE work_order_id IN ($1,$2)`, last, orderIDs[6]).Scan(&events, &attempts))
	require.Equal(t, 2, events)
	require.Zero(t, attempts)
}

func TestProviderDiagnosticReaderDoesNotReturnConsumerAsProvider(t *testing.T) {
	testContext := newServiceProposalRepositoryTest(t)
	fixture := newProviderWorkOrderTestFixture(t, testContext, "diagnostic-wrong-role")
	result, err := repositories.NewProviderDiagnosticReader(testContext.database).FindByProviderID(t.Context(), fixture.consumerID)
	require.NoError(t, err)
	require.Nil(t, result)
}

func TestProviderDiagnosticReaderTreatsZeroPersistedExpiryAsUnknown(t *testing.T) {
	users, db := newUserRepositoryTest(t)
	require.NoError(t, users.DeleteAll())
	p := validProviderWithData(t, repositories.NewCategoryRepository(db), db, "auth0|diagnostic-zero-expiry", "diagnostic.zero.expiry@example.com", "Juan", "Gomez", "Plomeria")
	_, err := users.Save(t.Context(), p)
	require.NoError(t, err)
	_, err = db.ExecContext(t.Context(), `INSERT INTO provider_payment_accounts(provider_id,payment_provider,external_account_id,access_token_ciphertext,token_expires_on) VALUES ($1,'mercado_pago','zero-expiry-account',$2,$3)`, p.ID(), []byte("private-not-selected"), time.Time{})
	require.NoError(t, err)
	result, err := repositories.NewProviderDiagnosticReader(db).FindByProviderID(t.Context(), p.ID())
	require.NoError(t, err)
	require.Nil(t, result.Payment.TokenExpiresOn)
	require.Equal(t, "unknown", result.Checks(time.Now())[2].Result)
	require.Nil(t, result.Checks(time.Now())[2].EvidenceOn)
}
