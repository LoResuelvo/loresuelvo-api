package repositories_test

import (
	"context"
	"testing"

	"github.com/LoResuelvo/loresuelvo-api/internal/adapters/repositories"
	"github.com/LoResuelvo/loresuelvo-api/internal/domain/provider"
	"github.com/LoResuelvo/loresuelvo-api/internal/infrastructure/db"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func newProviderRepositoryTest(t *testing.T) *repositories.ProviderRepository {
	t.Helper()

	config := db.NewTestPostgresConfigFromEnv()

	database, err := db.ConnectPostgres(context.Background(), config)
	require.NoError(t, err, "could not connect to test database")

	t.Cleanup(func() {
		_, _ = database.Exec("DELETE FROM users")
		database.Close()
	})

	return repositories.NewProviderRepository(database)
}

func validProvider() provider.Provider {
	return provider.NewProvider("auth0|josue", "josugod@gmail.com", "Josue", "el pro", []string{"Palermo", "Belgrano"})
}

func TestProviderRepositoryCanSaveAProvider(t *testing.T) {
	repo := newProviderRepositoryTest(t)
	provider := validProvider()

	err := repo.Save(provider)

	assert.NoError(t, err)
	exists := repo.FindByEmail(provider.User.Email)
	assert.True(t, exists, "Provider should be saved on database")
}

func TestProviderRepositoryCanDeleteAllProviders(t *testing.T) {
	repo := newProviderRepositoryTest(t)

	_ = repo.Save(validProvider())

	err := repo.DeleteAll()

	assert.NoError(t, err)
	exists := repo.FindByEmail(validProvider().User.Email)
	assert.False(t, exists, "All providers should be deleted from database")
}

func TestProviderRepositoryCanFindByEmail(t *testing.T) {
	repo := newProviderRepositoryTest(t)
	provider := validProvider()

	_ = repo.Save(provider)

	assert.True(t, repo.FindByEmail(provider.User.Email), "Provider should be found by email")
}

func TestProviderRepositoryFindByEmailReturnsFalseIfProviderDoesNotExist(t *testing.T) {
	repo := newProviderRepositoryTest(t)

	assert.False(t, repo.FindByEmail("no-existe@ejemplo.com"), "Provider should not be found by email if it does not exist")
}
