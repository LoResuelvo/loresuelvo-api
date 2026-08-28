package repositories_test

import (
	"context"
	"database/sql"
	"testing"
	"time"

	"github.com/LoResuelvo/loresuelvo-api/internal/adapters/repositories"
	identityverification "github.com/LoResuelvo/loresuelvo-api/internal/domain/identityverification"
	"github.com/LoResuelvo/loresuelvo-api/internal/infrastructure/db"
	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
)

type identityVerificationRepositoryTestContext struct {
	database   *sql.DB
	repository *repositories.IdentityVerificationRepository
	providerID int
}

func newIdentityVerificationRepositoryTest(t *testing.T) identityVerificationRepositoryTestContext {
	t.Helper()
	database, err := db.ConnectPostgres(context.Background(), db.NewTestPostgresConfigFromEnv())
	require.NoError(t, err)
	context := identityVerificationRepositoryTestContext{database: database, repository: repositories.NewIdentityVerificationRepository(database)}
	context.clean(t)
	context.providerID = saveIdentityProvider(t, database)
	t.Cleanup(func() {
		context.clean(t)
		database.Close()
	})
	return context
}

func (context identityVerificationRepositoryTestContext) clean(t *testing.T) {
	t.Helper()
	_, err := context.database.Exec(`DELETE FROM identity_verification_events`)
	require.NoError(t, err)
	_, err = context.database.Exec(`DELETE FROM identity_verification_sessions`)
	require.NoError(t, err)
	_, err = context.database.Exec(`DELETE FROM providers`)
	require.NoError(t, err)
	_, err = context.database.Exec(`DELETE FROM users`)
	require.NoError(t, err)
	_, err = context.database.Exec(`DELETE FROM categories`)
	require.NoError(t, err)
}

func saveIdentityProvider(t *testing.T, database *sql.DB) int {
	t.Helper()
	var categoryID, providerID int
	require.NoError(t, database.QueryRow(`
		INSERT INTO categories (name, normalized_name, created_on, updated_on)
		VALUES ('Identity Test', 'identity-test', NOW(), NOW()) RETURNING id`).Scan(&categoryID))
	require.NoError(t, database.QueryRow(`
		INSERT INTO users (auth_id, email, name, surname, role, created_on, updated_on)
		VALUES ('auth0|identity-repository', 'identity.repository@example.com', 'Juan', 'Gomez', 'provider', NOW(), NOW()) RETURNING id`).Scan(&providerID))
	_, err := database.Exec(`INSERT INTO providers (user_id, category_id) VALUES ($1, $2)`, providerID, categoryID)
	require.NoError(t, err)
	return providerID
}

func identityVerificationFixture(t *testing.T, providerID int, createdOn time.Time) *identityverification.IdentityVerification {
	t.Helper()
	verification, err := identityverification.NewVerification(providerID, uuid.New(), uuid.New(), "didit", 3, createdOn)
	require.NoError(t, err)
	return verification
}

func TestIdentityVerificationRepositoryStoresAndHydratesSession(t *testing.T) {
	testContext := newIdentityVerificationRepositoryTest(t)
	verification := identityVerificationFixture(t, testContext.providerID, time.Date(2026, 9, 1, 12, 0, 0, 0, time.UTC))
	verification.RiskCodes = []string{"DOCUMENT_EXPIRED"}

	require.NoError(t, testContext.repository.Save(context.Background(), verification))
	found, err := testContext.repository.FindBySessionID(context.Background(), verification.ExternalSessionID)

	require.NoError(t, err)
	require.Equal(t, verification.ExternalSessionID, found.ExternalSessionID)
	require.Equal(t, verification.ProviderID, found.ProviderID)
	require.Equal(t, verification.WorkflowID, found.WorkflowID)
	require.Equal(t, verification.Status, found.Status)
	require.Equal(t, []string{"DOCUMENT_EXPIRED"}, found.RiskCodes)
}

func TestIdentityVerificationRepositoryReturnsMostRecentSession(t *testing.T) {
	testContext := newIdentityVerificationRepositoryTest(t)
	first := identityVerificationFixture(t, testContext.providerID, time.Date(2026, 9, 1, 12, 0, 0, 0, time.UTC))
	second := identityVerificationFixture(t, testContext.providerID, first.CreatedOn.Add(time.Minute))
	require.NoError(t, testContext.repository.Save(context.Background(), first))
	require.NoError(t, testContext.repository.Save(context.Background(), second))

	found, err := testContext.repository.FindLatestByProviderID(context.Background(), testContext.providerID)

	require.NoError(t, err)
	require.Equal(t, second.ExternalSessionID, found.ExternalSessionID)
}

func TestIdentityVerificationRepositoryDeduplicatesEventsAtomically(t *testing.T) {
	testContext := newIdentityVerificationRepositoryTest(t)
	verification := identityVerificationFixture(t, testContext.providerID, time.Date(2026, 9, 1, 12, 0, 0, 0, time.UTC))
	require.NoError(t, testContext.repository.Save(context.Background(), verification))
	event := identityverification.VerificationEvent{ExternalEventID: uuid.New(), ExternalSessionID: verification.ExternalSessionID, EventType: "status.updated", OccurredOn: verification.CreatedOn.Add(time.Minute), ReceivedOn: verification.CreatedOn.Add(2 * time.Minute)}
	verification.Status = identityverification.StatusApproved
	verification.LastResultOn = event.OccurredOn
	verifiedOn := event.OccurredOn
	verification.VerifiedOn = &verifiedOn
	require.NoError(t, testContext.repository.SaveResult(context.Background(), event, verification))

	verification.Status = identityverification.StatusDeclined
	verification.VerifiedOn = nil
	require.NoError(t, testContext.repository.SaveResult(context.Background(), event, verification))
	found, err := testContext.repository.FindBySessionID(context.Background(), verification.ExternalSessionID)
	require.NoError(t, err)
	require.Equal(t, identityverification.StatusApproved, found.Status)

	var eventCount int
	require.NoError(t, testContext.database.QueryRow(`SELECT COUNT(*) FROM identity_verification_events WHERE external_session_id = $1`, verification.ExternalSessionID).Scan(&eventCount))
	require.Equal(t, 1, eventCount)
}

func TestIdentityVerificationRepositoryRollsBackUnknownSessionEvent(t *testing.T) {
	testContext := newIdentityVerificationRepositoryTest(t)
	verification := identityVerificationFixture(t, testContext.providerID, time.Date(2026, 9, 1, 12, 0, 0, 0, time.UTC))
	event := identityverification.VerificationEvent{ExternalEventID: uuid.New(), ExternalSessionID: verification.ExternalSessionID, EventType: "status.updated", OccurredOn: verification.CreatedOn.Add(time.Minute), ReceivedOn: verification.CreatedOn.Add(2 * time.Minute)}

	err := testContext.repository.SaveResult(context.Background(), event, verification)

	require.Error(t, err)
	var eventCount int
	require.NoError(t, testContext.database.QueryRow(`SELECT COUNT(*) FROM identity_verification_events`).Scan(&eventCount))
	require.Equal(t, 0, eventCount)
}
