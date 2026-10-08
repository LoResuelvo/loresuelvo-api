package category_test

import (
	"strings"
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

func TestCategoryReturnsErrorWhenNameExceedsMaximumLength(t *testing.T) {
	createdCategory, err := category.New(strings.Repeat("ñ", 101))

	assert.ErrorIs(t, err, category.ErrNameTooLong)
	assert.Nil(t, createdCategory)
}

func TestCategoryAcceptsOneHundredUnicodeCharacters(t *testing.T) {
	name := strings.Repeat("ñ", 100)

	createdCategory, err := category.New(name)

	assert.NoError(t, err)
	assert.Equal(t, name, createdCategory.Name)
}

func TestCategoryNewInitializesAvailability(t *testing.T) {
	created, err := category.New("Plomería")
	assert.NoError(t, err)
	assert.True(t, created.Enabled)
	assert.Equal(t, 1, created.Version)
}

func TestCategoryEditPreservesNoOpVersion(t *testing.T) {
	current, _ := category.New("Plomería")
	changed, err := current.Edit(category.Edit{ExpectedVersion: 1, Name: new("Plomería"), Enabled: new(true)}, false)
	assert.NoError(t, err)
	assert.False(t, changed)
	assert.Equal(t, 1, current.Version)
}

func TestCategoryEditRequiresFreshVersion(t *testing.T) {
	current, _ := category.New("Plomería")
	_, err := current.Edit(category.Edit{ExpectedVersion: 2, Name: new("Electricidad")}, false)
	assert.ErrorIs(t, err, category.ErrVersionConflict)
	assert.Equal(t, "Plomería", current.Name)
}

func TestCategoryEditRequiresConfirmationWithoutPartialRename(t *testing.T) {
	current, _ := category.New("Plomería")
	_, err := current.Edit(category.Edit{ExpectedVersion: 1, Name: new("Electricidad"), Enabled: new(false), Reason: "Retirar oferta"}, true)
	assert.ErrorIs(t, err, category.ErrConfirmationRequired)
	assert.Equal(t, "Plomería", current.Name)
	assert.True(t, current.Enabled)
}

func TestCategoryEditValidatesReasonAndBusinessFields(t *testing.T) {
	for _, testCase := range []struct {
		edit     category.Edit
		expected error
	}{
		{category.Edit{ExpectedVersion: 1}, category.ErrNoChanges},
		{category.Edit{Name: new("Plomería")}, category.ErrVersionRequired},
		{category.Edit{ExpectedVersion: 1, Enabled: new(false), Reason: strings.Repeat("ñ", 251)}, category.ErrInvalidReason},
		{category.Edit{ExpectedVersion: 1, Enabled: new(false), Reason: "line\nbreak"}, category.ErrInvalidReason},
		{category.Edit{ExpectedVersion: 1, Enabled: new(false), Reason: "   "}, category.ErrInvalidReason},
	} {
		current, err := category.New("Plomería")
		assert.NoError(t, err)
		changed, err := current.Edit(testCase.edit, false)
		assert.ErrorIs(t, err, testCase.expected)
		assert.False(t, changed)
		assert.True(t, current.Enabled)
		assert.Equal(t, 1, current.Version)
	}
}
