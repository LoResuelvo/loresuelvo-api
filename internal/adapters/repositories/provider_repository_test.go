package repositories_test

import (
	"context"
	"database/sql"
	"testing"

	"github.com/LoResuelvo/loresuelvo-api/internal/adapters/repositories"
	"github.com/LoResuelvo/loresuelvo-api/internal/domain/category"
	"github.com/LoResuelvo/loresuelvo-api/internal/domain/provider"
	"github.com/LoResuelvo/loresuelvo-api/internal/infrastructure/db"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func cleanProviderRepositoryTestDatabase(t *testing.T, database *sql.DB) {
	t.Helper()

	_, err := database.Exec("DELETE FROM users")
	require.NoError(t, err, "could not clean users")

	_, err = database.Exec("DELETE FROM categories")
	require.NoError(t, err, "could not clean categories")
}

type providerRepositoryTestContext struct {
	providerRepository *repositories.ProviderRepository
	categoryRepository *repositories.CategoryRepository
}

func newProviderRepositoryTest(t *testing.T) providerRepositoryTestContext {
	t.Helper()

	config := db.NewTestPostgresConfigFromEnv()

	database, err := db.ConnectPostgres(context.Background(), config)
	require.NoError(t, err, "could not connect to test database")

	t.Cleanup(func() {
		cleanProviderRepositoryTestDatabase(t, database)
		database.Close()
	})

	cleanProviderRepositoryTestDatabase(t, database)

	userRepository := repositories.NewUserRepository(database)
	return providerRepositoryTestContext{
		providerRepository: repositories.NewProviderRepository(database, userRepository),
		categoryRepository: repositories.NewCategoryRepository(database),
	}
}

func validProvider(t *testing.T, categoryRepository *repositories.CategoryRepository) *provider.Provider {
	t.Helper()

	return validProviderWithData(t, categoryRepository, "auth0|josue", "josugod@gmail.com", "Josue", "el pro", "Plomería")
}

func validProviderWithData(t *testing.T, categoryRepository *repositories.CategoryRepository, authID, email, name, surname, categoryName string) *provider.Provider {
	t.Helper()

	savedCategory := savedCategoryForProvider(t, categoryRepository, categoryName)
	provider, err := provider.NewProvider(authID, email, name, surname, savedCategory)
	require.NoError(t, err, "could not prepare provider")
	return provider
}

func savedCategoryForProvider(t *testing.T, categoryRepository *repositories.CategoryRepository, categoryName string) *category.Category {
	t.Helper()

	categoryToSave, err := category.New(categoryName)
	require.NoError(t, err, "could not prepare provider category")

	existingCategory := categoryRepository.FindByNormalizedName(categoryToSave.NormalizedName)
	if existingCategory != nil {
		return existingCategory
	}

	savedCategory, err := categoryRepository.Save(*categoryToSave)
	require.NoError(t, err, "could not prepare provider category")
	require.NotNil(t, savedCategory, "provider category should exist")
	return savedCategory
}

func TestProviderRepositoryCanSaveAProvider(t *testing.T) {
	testContext := newProviderRepositoryTest(t)
	repo := testContext.providerRepository
	provider := validProvider(t, testContext.categoryRepository)

	err := repo.Save(*provider)

	assert.NoError(t, err)
	exists := repo.FindByEmail(provider.User.Email)
	assert.True(t, exists, "Provider should be saved on database")
}

func TestProviderRepositoryCanDeleteAllProviders(t *testing.T) {
	testContext := newProviderRepositoryTest(t)
	repo := testContext.providerRepository
	provider := validProvider(t, testContext.categoryRepository)

	_ = repo.Save(*provider)

	err := repo.DeleteAll()

	assert.NoError(t, err)
	exists := repo.FindByEmail(provider.User.Email)
	assert.False(t, exists, "All providers should be deleted from database")
}

func TestProviderRepositoryCanFindByEmail(t *testing.T) {
	testContext := newProviderRepositoryTest(t)
	repo := testContext.providerRepository
	provider := validProvider(t, testContext.categoryRepository)

	_ = repo.Save(*provider)

	assert.True(t, repo.FindByEmail(provider.User.Email), "Provider should be found by email")
}

func TestProviderRepositoryFindByEmailReturnsFalseIfProviderDoesNotExist(t *testing.T) {
	testContext := newProviderRepositoryTest(t)
	repo := testContext.providerRepository

	assert.False(t, repo.FindByEmail("no-existe@ejemplo.com"), "Provider should not be found by email if it does not exist")
}

func TestProviderRepositoryCanFindIDByEmail(t *testing.T) {
	testContext := newProviderRepositoryTest(t)
	repo := testContext.providerRepository
	provider := validProvider(t, testContext.categoryRepository)

	err := repo.Save(*provider)
	require.NoError(t, err, "saving provider should not return an error")

	providerID, err := repo.FindIDByEmail(provider.User.Email)

	require.NoError(t, err)
	assert.NotZero(t, providerID)
}

func TestProviderRepositoryCanCheckExistenceByID(t *testing.T) {
	testContext := newProviderRepositoryTest(t)
	repo := testContext.providerRepository
	provider := validProvider(t, testContext.categoryRepository)

	err := repo.Save(*provider)
	require.NoError(t, err, "saving provider should not return an error")

	providerID, err := repo.FindIDByEmail(provider.User.Email)
	require.NoError(t, err)

	exists, err := repo.ExistsByID(providerID)

	require.NoError(t, err)
	assert.True(t, exists)
}

func TestProviderRepositoryExistsByIDReturnsFalseIfProviderDoesNotExist(t *testing.T) {
	testContext := newProviderRepositoryTest(t)
	repo := testContext.providerRepository

	exists, err := repo.ExistsByID(999999999)

	require.NoError(t, err)
	assert.False(t, exists)
}

func TestProviderRepositoryCanFindProvidersByCategoryID(t *testing.T) {
	testContext := newProviderRepositoryTest(t)
	repo := testContext.providerRepository
	plumbingProvider := validProviderWithData(t, testContext.categoryRepository, "auth0|juan", "juan.plomero@example.com", "Juan", "Pérez", "Plomería")
	anotherPlumbingProvider := validProviderWithData(t, testContext.categoryRepository, "auth0|pedro", "pedro.plomero@example.com", "Pedro", "Dib", "Plomería")
	electricProvider := validProviderWithData(t, testContext.categoryRepository, "auth0|laura", "laura.electricista@example.com", "Laura", "Gómez", "Electricidad")

	require.NoError(t, repo.Save(*plumbingProvider))
	require.NoError(t, repo.Save(*anotherPlumbingProvider))
	require.NoError(t, repo.Save(*electricProvider))

	providers, err := repo.FindByCategoryID(plumbingProvider.Category.ID)

	require.NoError(t, err)
	assert.Len(t, providers, 2)
	assert.NotZero(t, providers[0].ID)
	assert.Equal(t, "Juan", providers[0].User.Name)
	assert.Equal(t, "Pérez", providers[0].User.Surname)
	assert.NotZero(t, providers[1].ID)
	assert.Equal(t, "Pedro", providers[1].User.Name)
	assert.Equal(t, "Dib", providers[1].User.Surname)
}

func TestProviderRepositoryFindByCategoryIDReturnsEmptyListIfNoProvidersExistForCategory(t *testing.T) {
	testContext := newProviderRepositoryTest(t)
	repo := testContext.providerRepository
	emptyCategory := savedCategoryForProvider(t, testContext.categoryRepository, "Gasista")

	providers, err := repo.FindByCategoryID(emptyCategory.ID)

	require.NoError(t, err)
	assert.Empty(t, providers)
}

func TestProviderRepositoryCanFindIDByAuthID(t *testing.T) {
	testContext := newProviderRepositoryTest(t)
	repo := testContext.providerRepository
	provider := validProvider(t, testContext.categoryRepository)

	err := repo.Save(*provider)
	require.NoError(t, err, "saving provider should not return an error")

	providerID, err := repo.FindIDByAuthID(provider.User.AuthID)

	require.NoError(t, err)
	assert.NotZero(t, providerID)
}
