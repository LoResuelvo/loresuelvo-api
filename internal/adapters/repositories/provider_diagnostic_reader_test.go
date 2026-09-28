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
