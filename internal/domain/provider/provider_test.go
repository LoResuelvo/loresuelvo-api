package provider_test

import (
	"testing"

	"github.com/LoResuelvo/loresuelvo-api/internal/domain/category"
	coveragezone "github.com/LoResuelvo/loresuelvo-api/internal/domain/coverage_zone"
	"github.com/LoResuelvo/loresuelvo-api/internal/domain/provider"
	"github.com/stretchr/testify/assert"
)

func TestProviderHasCategory(t *testing.T) {
	foundProvider := provider.Provider{Category: &category.Category{ID: 3}}

	assert.True(t, foundProvider.HasCategory(3))
	assert.False(t, foundProvider.HasCategory(4))
	assert.False(t, foundProvider.HasCategory(0))
}

func TestNewProviderIncludesCoverageZones(t *testing.T) {
	providerCategory := &category.Category{ID: 1, Name: "Plomería"}
	selectedZones := []coveragezone.CoverageZone{{ID: 6, Name: "Comuna 6", Enabled: true}}
	foundProvider, err := provider.NewProvider(
		"auth0|ana",
		"ana@example.com",
		"Ana",
		"Perez",
		providerCategory,
		nil,
		selectedZones,
	)

	assert.NoError(t, err)

	selectedZones[0].Name = "Changed after selection"

	assert.Equal(t, []coveragezone.CoverageZone{{ID: 6, Name: "Comuna 6", Enabled: true}}, foundProvider.CoverageZones)
}

func TestNewProviderRequiresCoverageZone(t *testing.T) {
	providerCategory := &category.Category{ID: 1, Name: "Plomería"}

	foundProvider, err := provider.NewProvider(
		"auth0|ana",
		"ana@example.com",
		"Ana",
		"Perez",
		providerCategory,
		nil,
		nil,
	)

	assert.Nil(t, foundProvider)
	assert.ErrorIs(t, err, coveragezone.ErrAtLeastOneRequired)
}

func TestNewProviderRejectsUnavailableCoverageZone(t *testing.T) {
	providerCategory := &category.Category{ID: 1, Name: "Plomería"}
	selectedZones := []coveragezone.CoverageZone{{ID: 15, Name: "Comuna 15", Enabled: false}}

	foundProvider, err := provider.NewProvider(
		"auth0|ana",
		"ana@example.com",
		"Ana",
		"Perez",
		providerCategory,
		nil,
		selectedZones,
	)

	assert.Nil(t, foundProvider)
	assert.ErrorIs(t, err, coveragezone.ErrNotAvailable)
}

func TestNewProviderIncludesMultipleCoverageZones(t *testing.T) {
	providerCategory := &category.Category{ID: 1, Name: "Plomería"}
	selectedZones := []coveragezone.CoverageZone{
		{ID: 6, Name: "Comuna 6", Enabled: true},
		{ID: 14, Name: "Comuna 14", Enabled: true},
	}

	foundProvider, err := provider.NewProvider(
		"auth0|ana",
		"ana@example.com",
		"Ana",
		"Perez",
		providerCategory,
		nil,
		selectedZones,
	)

	assert.NoError(t, err)
	assert.Equal(t, selectedZones, foundProvider.CoverageZones)
}

func TestNewProviderRejectsDuplicateCoverageZones(t *testing.T) {
	providerCategory := &category.Category{ID: 1, Name: "Plomería"}
	selectedZones := []coveragezone.CoverageZone{
		{ID: 6, Name: "Comuna 6", Enabled: true},
		{ID: 6, Name: "Comuna 6", Enabled: true},
	}

	foundProvider, err := provider.NewProvider(
		"auth0|ana",
		"ana@example.com",
		"Ana",
		"Perez",
		providerCategory,
		nil,
		selectedZones,
	)

	assert.Nil(t, foundProvider)
	assert.ErrorIs(t, err, coveragezone.ErrDuplicateCoverageZone)
}
