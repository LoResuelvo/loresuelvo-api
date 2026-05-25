package category_test

import (
	"testing"

	"github.com/LoResuelvo/loresuelvo-api/internal/domain/category"
	"github.com/stretchr/testify/assert"
)

func TestCategoryCanBeCreatedWithValidName(t *testing.T) {
	createdCategory, err := category.New("Plomería")

	assert.NoError(t, err)
	assert.Equal(t, "Plomería", createdCategory.Name)
	assert.Equal(t, "plomería", createdCategory.NormalizedName)
}

func TestCategoryTrimsName(t *testing.T) {
	createdCategory, err := category.New("  Plomería  ")

	assert.NoError(t, err)
	assert.Equal(t, "Plomería", createdCategory.Name)
	assert.Equal(t, "plomería", createdCategory.NormalizedName)
}

func TestCategoryReturnsErrorWhenNameIsEmpty(t *testing.T) {
	createdCategory, err := category.New("   ")

	assert.ErrorIs(t, err, category.ErrNameRequired)
	assert.Nil(t, createdCategory)
}
