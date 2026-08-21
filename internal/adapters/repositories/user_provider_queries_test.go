package repositories_test

import (
	"context"
	"database/sql"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/LoResuelvo/loresuelvo-api/internal/adapters/repositories"
	"github.com/LoResuelvo/loresuelvo-api/internal/domain/category"
	coveragezone "github.com/LoResuelvo/loresuelvo-api/internal/domain/coverage_zone"
	filedomain "github.com/LoResuelvo/loresuelvo-api/internal/domain/file"
	"github.com/LoResuelvo/loresuelvo-api/internal/domain/provider"
	"github.com/LoResuelvo/loresuelvo-api/internal/infrastructure/db"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func cleanProviderRepositoryTestDatabase(t *testing.T, database *sql.DB) {
	t.Helper()

	_, err := database.Exec("DELETE FROM providers")
	require.NoError(t, err, "could not clean providers")

	_, err = database.Exec("DELETE FROM users")
	require.NoError(t, err, "could not clean users")

	_, err = database.Exec("DELETE FROM coverage_zones")
	require.NoError(t, err, "could not clean coverage zones")

	_, err = database.Exec("DELETE FROM files")
	require.NoError(t, err, "could not clean files")

	_, err = database.Exec("DELETE FROM categories")
	require.NoError(t, err, "could not clean categories")
}

type providerRepositoryTestContext struct {
	providerRepository *repositories.UserRepository
	categoryRepository *repositories.CategoryRepository
	database           *sql.DB
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
		providerRepository: userRepository,
		categoryRepository: repositories.NewCategoryRepository(database),
		database:           database,
	}
}

func validProvider(t *testing.T, testContext providerRepositoryTestContext) *provider.Provider {
	t.Helper()

	return validProviderWithData(t, testContext.categoryRepository, testContext.database, "auth0|josue", "josugod@gmail.com", "Josue", "el pro", "Plomería")
}

func providerUser(value *provider.Provider) *provider.Provider {
	return value
}

func validProviderWithData(t *testing.T, categoryRepository *repositories.CategoryRepository, database *sql.DB, authID, email, name, surname, categoryName string) *provider.Provider {
	t.Helper()

	coverageZone := savedCoverageZoneForProvider(t, database, "Comuna 6")
	return validProviderWithCoverageZones(t, categoryRepository, database, authID, email, name, surname, categoryName, []coveragezone.CoverageZone{*coverageZone})
}

func validProviderWithCoverageZones(t *testing.T, categoryRepository *repositories.CategoryRepository, database *sql.DB, authID, email, name, surname, categoryName string, coverageZones []coveragezone.CoverageZone) *provider.Provider {
	t.Helper()

	savedCategory := savedCategoryForProvider(t, categoryRepository, categoryName)
	profilePhotoFileID := savedProviderProfilePhotoFileID(t, database, authID)
	provider, err := provider.NewProvider(authID, email, name, surname, savedCategory, &filedomain.Image{FileID: profilePhotoFileID}, coverageZones)
	require.NoError(t, err, "could not prepare provider")
	return provider
}

func savedCoverageZoneForProvider(t *testing.T, database *sql.DB, name string) *coveragezone.CoverageZone {
	t.Helper()

	repository := repositories.NewCoverageZoneRepository(database)
	market, err := repository.FindMarketByCode(context.Background(), "CABA")
	if errors.Is(err, coveragezone.ErrDoesNotExist) {
		newMarket, marketErr := coveragezone.NewMarket("CABA", "Ciudad Autónoma de Buenos Aires")
		require.NoError(t, marketErr)
		market, err = repository.SaveMarket(context.Background(), *newMarket)
	}
	require.NoError(t, err, "could not find or create coverage market")

	existingZone, err := repository.FindByMarketCodeAndName(context.Background(), market.Code, name)
	if err == nil {
		return existingZone
	}
	if !errors.Is(err, coveragezone.ErrDoesNotExist) {
		require.NoError(t, err, "could not find provider coverage zone")
	}

	zoneCode := "CABA-" + strings.ToUpper(strings.ReplaceAll(name, " ", "-"))
	zone, err := coveragezone.New(market.ID, zoneCode, name, coveragezone.KindCommune)
	require.NoError(t, err, "could not prepare provider coverage zone")
	savedZone, err := repository.Save(context.Background(), *zone)
	require.NoError(t, err, "could not save provider coverage zone")
	return savedZone
}

func savedProviderProfilePhotoFileID(t *testing.T, database *sql.DB, authID string) string {
	t.Helper()

	fileID := uuid.NewString()
	_, err := database.Exec(
		`INSERT INTO files (id, key, bucket, original_name, mime_type, size_bytes, status, visibility, purpose, uploaded_by_auth_id, created_on, updated_on)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12)`,
		fileID,
		"files/2026/06/profile_photo/"+fileID+".jpg",
		"public-bucket",
		"foto.jpg",
		"image/jpeg",
		1024,
		filedomain.StatusConfirmed,
		filedomain.VisibilityPublic,
		filedomain.PurposeProfilePhoto,
		authID,
		time.Date(2026, 6, 6, 0, 0, 0, 0, time.UTC),
		time.Date(2026, 6, 6, 0, 0, 0, 0, time.UTC),
	)
	require.NoError(t, err, "could not prepare provider profile photo")
	return fileID
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
	require.NotNil(t, &savedCategory, "provider category should exist")
	return savedCategory
}

func TestProviderRepositoryCanSaveAProvider(t *testing.T) {
	testContext := newProviderRepositoryTest(t)
	repo := testContext.providerRepository
	provider := validProvider(t, testContext)

	_, err := repo.Save(context.Background(), providerUser(provider))

	assert.NoError(t, err)
	exists := repo.FindByEmail(provider.Email())
	assert.True(t, exists, "Provider should be saved on database")
}

func TestProviderRepositorySavesCoverageZonesAtomically(t *testing.T) {
	testContext := newProviderRepositoryTest(t)
	coverageZoneRepository := repositories.NewCoverageZoneRepository(testContext.database)
	firstZone := savedCoverageZoneForProvider(t, testContext.database, "Comuna 6")
	secondZone := savedCoverageZoneForProvider(t, testContext.database, "Comuna 14")

	providerToSave := validProviderWithCoverageZones(t, testContext.categoryRepository, testContext.database, "auth0|josue", "josugod@gmail.com", "Josue", "el pro", "Plomería", []coveragezone.CoverageZone{*firstZone, *secondZone})

	savedUser, err := testContext.providerRepository.Save(context.Background(), providerUser(providerToSave))
	require.NoError(t, err)

	zones, err := coverageZoneRepository.FindByProviderID(context.Background(), savedUser.ID())

	require.NoError(t, err)
	require.Equal(t, []coveragezone.CoverageZone{*firstZone, *secondZone}, zones)
}

func TestProviderRepositoryHydratesCoverageZones(t *testing.T) {
	testContext := newProviderRepositoryTest(t)
	firstZone := savedCoverageZoneForProvider(t, testContext.database, "Comuna 6")
	secondZone := savedCoverageZoneForProvider(t, testContext.database, "Comuna 14")
	providerToSave := validProviderWithCoverageZones(t, testContext.categoryRepository, testContext.database, "auth0|josue", "josugod@gmail.com", "Josue", "el pro", "Plomería", []coveragezone.CoverageZone{*firstZone, *secondZone})

	savedUser, err := testContext.providerRepository.Save(context.Background(), providerUser(providerToSave))
	require.NoError(t, err)

	foundProvider, err := testContext.providerRepository.FindProviderByID(context.Background(), savedUser.ID())

	require.NoError(t, err)
	require.Equal(t, []coveragezone.CoverageZone{*firstZone, *secondZone}, foundProvider.CoverageZones)
}

func TestProviderRepositoryRollsBackWhenCoverageZoneCannotBeSaved(t *testing.T) {
	testContext := newProviderRepositoryTest(t)
	missingZone := coveragezone.CoverageZone{ID: 999999999, Name: "Missing zone", Enabled: true}
	providerToSave := validProviderWithCoverageZones(t, testContext.categoryRepository, testContext.database, "auth0|josue", "josugod@gmail.com", "Josue", "el pro", "Plomería", []coveragezone.CoverageZone{missingZone})

	_, err := testContext.providerRepository.Save(context.Background(), providerUser(providerToSave))

	require.Error(t, err)
	require.False(t, testContext.providerRepository.FindByEmail(providerToSave.Email()))

	var providerCount int
	err = testContext.database.QueryRow(
		`SELECT COUNT(*)
		FROM providers
		INNER JOIN users ON users.id = providers.user_id
		WHERE users.email = $1`,
		providerToSave.Email(),
	).Scan(&providerCount)
	require.NoError(t, err)
	require.Zero(t, providerCount)
}

func TestProviderRepositoryCanDeleteAllProviders(t *testing.T) {
	testContext := newProviderRepositoryTest(t)
	repo := testContext.providerRepository
	provider := validProvider(t, testContext)

	_, _ = repo.Save(context.Background(), providerUser(provider))

	err := repo.DeleteAll()

	assert.NoError(t, err)
	exists := repo.FindByEmail(provider.Email())
	assert.False(t, exists, "All providers should be deleted from database")
}

func TestProviderRepositoryCanFindByEmail(t *testing.T) {
	testContext := newProviderRepositoryTest(t)
	repo := testContext.providerRepository
	provider := validProvider(t, testContext)

	_, _ = repo.Save(context.Background(), providerUser(provider))

	assert.True(t, repo.FindByEmail(provider.Email()), "Provider should be found by email")
}

func TestProviderRepositoryFindByEmailReturnsFalseIfProviderDoesNotExist(t *testing.T) {
	testContext := newProviderRepositoryTest(t)
	repo := testContext.providerRepository

	assert.False(t, repo.FindByEmail("no-existe@ejemplo.com"), "Provider should not be found by email if it does not exist")
}

func TestProviderRepositoryCanFindIDByEmail(t *testing.T) {
	testContext := newProviderRepositoryTest(t)
	repo := testContext.providerRepository
	provider := validProvider(t, testContext)

	_, err := repo.Save(context.Background(), providerUser(provider))
	require.NoError(t, err, "saving provider should not return an error")

	providerID, err := repo.FindIDByEmail(provider.Email())

	require.NoError(t, err)
	assert.NotZero(t, providerID)
}

func TestProviderRepositoryCanCheckExistenceByID(t *testing.T) {
	testContext := newProviderRepositoryTest(t)
	repo := testContext.providerRepository
	provider := validProvider(t, testContext)

	_, err := repo.Save(context.Background(), providerUser(provider))
	require.NoError(t, err, "saving provider should not return an error")

	providerID, err := repo.FindIDByEmail(provider.Email())
	require.NoError(t, err)

	exists, err := repo.ExistsProviderByID(providerID)

	require.NoError(t, err)
	assert.True(t, exists)
}

func TestProviderRepositoryExistsByIDReturnsFalseIfProviderDoesNotExist(t *testing.T) {
	testContext := newProviderRepositoryTest(t)
	repo := testContext.providerRepository

	exists, err := repo.ExistsProviderByID(999999999)

	require.NoError(t, err)
	assert.False(t, exists)
}

func TestProviderRepositoryCanFindProvidersByCategoryID(t *testing.T) {
	testContext := newProviderRepositoryTest(t)
	repo := testContext.providerRepository
	plumbingProvider := validProviderWithData(t, testContext.categoryRepository, testContext.database, "auth0|juan", "juan.plomero@example.com", "Juan", "Pérez", "Plomería")
	anotherPlumbingProvider := validProviderWithData(t, testContext.categoryRepository, testContext.database, "auth0|pedro", "pedro.plomero@example.com", "Pedro", "Dib", "Plomería")
	electricProvider := validProviderWithData(t, testContext.categoryRepository, testContext.database, "auth0|laura", "laura.electricista@example.com", "Laura", "Gómez", "Electricidad")

	_, err := repo.Save(context.Background(), providerUser(plumbingProvider))
	require.NoError(t, err)
	_, err = repo.Save(context.Background(), providerUser(anotherPlumbingProvider))
	require.NoError(t, err)
	_, err = repo.Save(context.Background(), providerUser(electricProvider))
	require.NoError(t, err)

	providers, err := repo.FindProvidersByCategoryID(plumbingProvider.Category.ID)

	require.NoError(t, err)
	assert.Len(t, providers, 2)
	assert.NotZero(t, providers[0].ID())
	assert.Equal(t, "Juan", providers[0].Name())
	assert.Equal(t, "Pérez", providers[0].Surname())
	require.NotNil(t, providers[0].Category)
	assert.Equal(t, plumbingProvider.Category.ID, providers[0].Category.ID)
	assert.Equal(t, "Plomería", providers[0].Category.Name)
	assert.Equal(t, plumbingProvider.ProfilePhoto().FileID, providers[0].ProfilePhoto().FileID)
	assert.Equal(t, "foto.jpg", providers[0].ProfilePhoto().OriginalName)
	assert.Equal(t, plumbingProvider.CoverageZones, providers[0].CoverageZones)
	assert.NotZero(t, providers[1].ID)
	assert.Equal(t, "Pedro", providers[1].Name())
	assert.Equal(t, "Dib", providers[1].Surname())
	require.NotNil(t, providers[1].Category)
	assert.Equal(t, anotherPlumbingProvider.Category.ID, providers[1].Category.ID)
	assert.Equal(t, "Plomería", providers[1].Category.Name)
	assert.Equal(t, anotherPlumbingProvider.ProfilePhoto().FileID, providers[1].ProfilePhoto().FileID)
	assert.Equal(t, anotherPlumbingProvider.CoverageZones, providers[1].CoverageZones)
}

func TestProviderRepositoryFindByCategoryIDReturnsEmptyListIfNoProvidersExistForCategory(t *testing.T) {
	testContext := newProviderRepositoryTest(t)
	repo := testContext.providerRepository
	emptyCategory := savedCategoryForProvider(t, testContext.categoryRepository, "Gasista")

	providers, err := repo.FindProvidersByCategoryID(emptyCategory.ID)

	require.NoError(t, err)
	assert.Empty(t, providers)
}

func TestProviderRepositoryCanFindIDByAuthID(t *testing.T) {
	testContext := newProviderRepositoryTest(t)
	repo := testContext.providerRepository
	provider := validProvider(t, testContext)

	_, err := repo.Save(context.Background(), providerUser(provider))
	require.NoError(t, err, "saving provider should not return an error")

	providerID, err := repo.FindIDByAuthID(provider.AuthID())

	require.NoError(t, err)
	assert.NotZero(t, providerID)
}

func TestProviderRepositoryCanFindByAuthID(t *testing.T) {
	testContext := newProviderRepositoryTest(t)
	repo := testContext.providerRepository
	providerToSave := validProvider(t, testContext)

	savedUser, err := repo.Save(context.Background(), providerUser(providerToSave))
	providerID := savedUser.ID()
	require.NoError(t, err, "saving provider should not return an error")

	foundProvider, err := repo.FindProviderByAuthID(providerToSave.AuthID())

	require.NoError(t, err)
	assert.Equal(t, providerID, foundProvider.ID())
	assert.Equal(t, providerToSave.AuthID(), foundProvider.AuthID())
	assert.Equal(t, providerToSave.Email(), foundProvider.Email())
	assert.Equal(t, providerToSave.Name(), foundProvider.Name())
	assert.Equal(t, providerToSave.Surname(), foundProvider.Surname())
	require.NotNil(t, foundProvider.Category)
	assert.Equal(t, providerToSave.Category.ID, foundProvider.Category.ID)
	assert.Equal(t, providerToSave.Category.Name, foundProvider.Category.Name)
	assert.Equal(t, providerToSave.ProfilePhoto().FileID, foundProvider.ProfilePhoto().FileID)
	assert.Equal(t, "foto.jpg", foundProvider.ProfilePhoto().OriginalName)
	assert.Equal(t, providerToSave.CoverageZones, foundProvider.CoverageZones)
}

func TestProviderRepositoryCanFindByID(t *testing.T) {
	testContext := newProviderRepositoryTest(t)
	providerToSave := validProvider(t, testContext)
	savedUser, err := testContext.providerRepository.Save(context.Background(), providerUser(providerToSave))
	providerID := savedUser.ID()
	require.NoError(t, err)

	foundProvider, err := testContext.providerRepository.FindProviderByID(context.Background(), providerID)

	require.NoError(t, err)
	assert.Equal(t, providerID, foundProvider.ID())
	assert.Equal(t, providerToSave.Name(), foundProvider.Name())
	require.NotNil(t, foundProvider.Category)
	assert.Equal(t, providerToSave.Category.ID, foundProvider.Category.ID)
	assert.Equal(t, providerToSave.CoverageZones, foundProvider.CoverageZones)
}

func TestProviderRepositoryFindByIDReturnsNotFound(t *testing.T) {
	testContext := newProviderRepositoryTest(t)

	foundProvider, err := testContext.providerRepository.FindProviderByID(context.Background(), 999999)

	assert.ErrorIs(t, err, provider.ErrDoesNotExist)
	assert.Nil(t, foundProvider)
}
