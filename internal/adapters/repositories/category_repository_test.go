package repositories_test

import (
	"context"
	"testing"

	"github.com/LoResuelvo/loresuelvo-api/internal/adapters/repositories"
	"github.com/LoResuelvo/loresuelvo-api/internal/domain/category"
	"github.com/LoResuelvo/loresuelvo-api/internal/infrastructure/db"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func newCategoryRepositoryTest(t *testing.T) *repositories.CategoryRepository {
	t.Helper()

	config := db.NewTestPostgresConfigFromEnv()

	database, err := db.ConnectPostgres(context.Background(), config)
	require.NoError(t, err, "could not connect to test database")

	t.Cleanup(func() {
		_, _ = database.Exec("DELETE FROM categories")
		database.Close()
	})

	_, _ = database.Exec("DELETE FROM categories")

	return repositories.NewCategoryRepository(database)
}

func validCategory() category.Category {
	validCategory, _ := category.New("Plomería")
	return *validCategory
}

func TestCategoryRepositoryCanSaveACategory(t *testing.T) {
	repo := newCategoryRepositoryTest(t)
	savedCategory := validCategory()

	err := repo.Save(savedCategory)

	assert.NoError(t, err)
	exists := repo.FindByNormalizedName(savedCategory.NormalizedName)
	assert.True(t, exists, "Category should be saved on database")
}

func TestCategoryRepositoryCanFindByNormalizedName(t *testing.T) {
	repo := newCategoryRepositoryTest(t)
	savedCategory := validCategory()

	err := repo.Save(savedCategory)

	assert.NoError(t, err, "saving category should not return an error")
	assert.True(t, repo.FindByNormalizedName(savedCategory.NormalizedName), "Category should be found by normalized name")
}

func TestCategoryRepositoryCanFindByNormalizedNameFromDifferentDisplayNames(t *testing.T) {
	repo := newCategoryRepositoryTest(t)
	savedCategory := validCategory()
	sameCategory, _ := category.New("  plomería  ")

	err := repo.Save(savedCategory)

	assert.NoError(t, err, "saving category should not return an error")
	assert.True(t, repo.FindByNormalizedName(sameCategory.NormalizedName), "Category should be found by normalized name")
}

func TestCategoryRepositoryCanDeleteAllCategories(t *testing.T) {
	repo := newCategoryRepositoryTest(t)
	savedCategory := validCategory()

	err := repo.Save(savedCategory)

	assert.NoError(t, err, "saving category should not return an error")

	err = repo.DeleteAll()

	assert.NoError(t, err)
	assert.False(t, repo.FindByNormalizedName(savedCategory.NormalizedName), "All categories should be deleted from database")
}

func TestCategoryRepositoryFindByNormalizedNameReturnsFalseIfCategoryDoesNotExist(t *testing.T) {
	repo := newCategoryRepositoryTest(t)

	assert.False(t, repo.FindByNormalizedName("no existe"), "Category should not be found by normalized name if it does not exist")
}
