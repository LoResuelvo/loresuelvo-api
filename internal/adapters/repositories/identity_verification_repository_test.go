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
	testContext := identityVerificationRepositoryTestContext{database: database, repository: repositories.NewIdentityVerificationRepository(database)}
	_, err = database.Exec(`DELETE FROM identity_verification_sessions`)
	require.NoError(t, err)
	testContext.providerID = saveIdentityProvider(t, database)
	t.Cleanup(func() { _ = testContext.repository.DeleteAll(); database.Close() })
	return testContext
}

func saveIdentityProvider(t *testing.T, database *sql.DB) int {
	t.Helper()
	var categoryID, providerID int
	require.NoError(t, database.QueryRow(`INSERT INTO categories (name, normalized_name, created_on, updated_on) VALUES ($1, $2, NOW(), NOW()) RETURNING id`, "Identity Test "+uuid.NewString(), uuid.NewString()).Scan(&categoryID))
	require.NoError(t, database.QueryRow(`INSERT INTO users (auth_id, email, name, surname, role, created_on, updated_on) VALUES ($1, $2, 'Juan', 'Gomez', 'provider', NOW(), NOW()) RETURNING id`, "auth0|"+uuid.NewString(), uuid.NewString()+"@example.com").Scan(&providerID))
	_, err := database.Exec(`INSERT INTO providers (user_id, category_id) VALUES ($1, $2)`, providerID, categoryID)
	require.NoError(t, err)
	return providerID
}

func TestIdentityVerificationRepositoryStoresAndHydratesSession(t *testing.T) {
	testContext := newIdentityVerificationRepositoryTest(t)
	verification, err := identityverification.NewVerification(testContext.providerID, uuid.New(), uuid.New(), "didit", 3, time.Date(2026, 9, 1, 12, 0, 0, 0, time.UTC))
	require.NoError(t, err)

	require.NoError(t, testContext.repository.Save(context.Background(), verification))
	found, err := testContext.repository.FindBySessionID(context.Background(), verification.ExternalSessionID)

	require.NoError(t, err)
	require.Equal(t, verification, found)
}

func TestIdentityVerificationRepositoryFindsInProgressSession(t *testing.T) {
	testContext := newIdentityVerificationRepositoryTest(t)
	verification, err := identityverification.NewVerification(testContext.providerID, uuid.New(), uuid.New(), "fake", 1, time.Now().UTC())
	require.NoError(t, err)
	verification.Status = identityverification.StatusInProgress
	require.NoError(t, testContext.repository.Save(context.Background(), verification))

	found, err := testContext.repository.FindLatestByProviderID(context.Background(), testContext.providerID)

	require.NoError(t, err)
	require.Equal(t, verification.ExternalSessionID, found.ExternalSessionID)
	require.Equal(t, identityverification.StatusInProgress, found.Status)
}

func TestIdentityVerificationRepositoryFindsAwaitingUserSession(t *testing.T) {
	testContext := newIdentityVerificationRepositoryTest(t)
	verification, err := identityverification.NewVerification(testContext.providerID, uuid.New(), uuid.New(), "fake", 1, time.Now().UTC())
	require.NoError(t, err)
	verification.Status = identityverification.StatusAwaitingUser
	require.NoError(t, testContext.repository.Save(context.Background(), verification))

	found, err := testContext.repository.FindLatestByProviderID(context.Background(), testContext.providerID)

	require.NoError(t, err)
	require.Equal(t, verification.ExternalSessionID, found.ExternalSessionID)
	require.Equal(t, identityverification.StatusAwaitingUser, found.Status)
}
