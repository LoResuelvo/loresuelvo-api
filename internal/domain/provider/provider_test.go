package provider_test

import (
	"testing"

	"github.com/LoResuelvo/loresuelvo-api/internal/domain/category"
	"github.com/LoResuelvo/loresuelvo-api/internal/domain/provider"
	"github.com/stretchr/testify/assert"
)

func TestProviderHasCategory(t *testing.T) {
	foundProvider := provider.Provider{Category: &category.Category{ID: 3}}

	assert.True(t, foundProvider.HasCategory(3))
	assert.False(t, foundProvider.HasCategory(4))
	assert.False(t, foundProvider.HasCategory(0))
}

func TestProviderWithoutCategoryDoesNotMatchCategory(t *testing.T) {
	assert.False(t, (provider.Provider{}).HasCategory(3))
}
