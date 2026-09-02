package repositories_test

import (
	"context"
	"database/sql"
	"testing"

	"github.com/LoResuelvo/loresuelvo-api/internal/adapters/repositories"
	"github.com/LoResuelvo/loresuelvo-api/internal/domain/category"
	"github.com/LoResuelvo/loresuelvo-api/internal/infrastructure/db"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func cleanCategoryRepositoryTestDatabase(t *testing.T, database *sql.DB) {
	t.Helper()

	_, err := database.Exec("DELETE FROM users")
	require.NoError(t, err, "could not clean users")

	_, err = database.Exec("DELETE FROM categories")
	require.NoError(t, err, "could not clean categories")
}

func newCategoryRepositoryTest(t *testing.T) *repositories.CategoryRepository {
	t.Helper()

	config, err := db.NewTestPostgresConfigFromEnv()
	require.NoError(t, err)

	database, err := db.ConnectPostgres(context.Background(), config)
	require.NoError(t, err, "could not connect to test database")

	t.Cleanup(func() {
		cleanCategoryRepositoryTestDatabase(t, database)
		database.Close()
	})

	cleanCategoryRepositoryTestDatabase(t, database)

	return repositories.NewCategoryRepository(database)
}

func validCategory() category.Category {
	validCategory, _ := category.New("Plomería")
	return *validCategory
}

func TestCategoryRepositoryCanSaveACategory(t *testing.T) {
	repo := newCategoryRepositoryTest(t)
	savedCategory := validCategory()

	createdCategory, err := repo.Save(savedCategory)

	require.NoError(t, err)
	require.NotNil(t, createdCategory)
	assert.NotZero(t, createdCategory.ID, "saved category should return its generated id")
	assert.Equal(t, savedCategory.Name, createdCategory.Name)
	assert.Equal(t, savedCategory.NormalizedName, createdCategory.NormalizedName)
	foundCategory := repo.FindByNormalizedName(savedCategory.NormalizedName)
	assert.NotNil(t, foundCategory, "Category should be saved on database")
}

func TestCategoryRepositoryCanFindByNormalizedName(t *testing.T) {
	repo := newCategoryRepositoryTest(t)
	savedCategory := validCategory()

	_, err := repo.Save(savedCategory)

	assert.NoError(t, err, "saving category should not return an error")
	assert.NotNil(t, repo.FindByNormalizedName(savedCategory.NormalizedName), "Category should be found by normalized name")
}

func TestCategoryRepositoryCanFindByID(t *testing.T) {
	repo := newCategoryRepositoryTest(t)
	categoryToSave := validCategory()

	savedCategory, err := repo.Save(categoryToSave)

	require.NoError(t, err, "saving category should not return an error")
	require.NotNil(t, savedCategory)
	assert.NotNil(t, repo.FindByID(savedCategory.ID), "Category should be found by id")
}

func TestCategoryRepositoryCanListAllCategories(t *testing.T) {
	repo := newCategoryRepositoryTest(t)
	plumbingCategory := validCategory()
	electricityCategory, _ := category.New("Electricidad")

	_, err := repo.Save(plumbingCategory)
	require.NoError(t, err, "saving category should not return an error")
	_, err = repo.Save(*electricityCategory)
	require.NoError(t, err, "saving category should not return an error")

	categories, err := repo.ListAll()

	require.NoError(t, err)
	assert.Len(t, categories, 2)
	assert.Equal(t, "Electricidad", categories[0].Name)
	assert.Equal(t, "Plomería", categories[1].Name)
}

func TestCategoryRepositoryListAllReturnsEmptyListWhenThereAreNoCategories(t *testing.T) {
	repo := newCategoryRepositoryTest(t)

	categories, err := repo.ListAll()

	require.NoError(t, err)
	assert.Empty(t, categories)
}

func TestCategoryRepositoryCanFindByNormalizedNameFromDifferentDisplayNames(t *testing.T) {
	repo := newCategoryRepositoryTest(t)
	savedCategory := validCategory()
	sameCategory, _ := category.New("  plomería  ")

	_, err := repo.Save(savedCategory)

	assert.NoError(t, err, "saving category should not return an error")
	assert.NotNil(t, repo.FindByNormalizedName(sameCategory.NormalizedName), "Category should be found by normalized name")
}

func TestCategoryRepositoryCanDeleteAllCategories(t *testing.T) {
	repo := newCategoryRepositoryTest(t)
	savedCategory := validCategory()

	_, err := repo.Save(savedCategory)

	assert.NoError(t, err, "saving category should not return an error")

	err = repo.DeleteAll()

	assert.NoError(t, err)
	assert.Nil(t, repo.FindByNormalizedName(savedCategory.NormalizedName), "All categories should be deleted from database")
}

func TestCategoryRepositoryFindByNormalizedNameReturnsFalseIfCategoryDoesNotExist(t *testing.T) {
	repo := newCategoryRepositoryTest(t)

	assert.Nil(t, repo.FindByNormalizedName("no existe"), "Category should not be found by normalized name if it does not exist")
}

func TestCategoryRepositoryFindByIDReturnsFalseIfCategoryDoesNotExist(t *testing.T) {
	repo := newCategoryRepositoryTest(t)

	assert.Nil(t, repo.FindByID(999), "Category should not be found by id if it does not exist")
}
