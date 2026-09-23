package repositories_test

import (
	"context"
	"testing"
	"time"

	"github.com/LoResuelvo/loresuelvo-api/internal/adapters/repositories"
	"github.com/LoResuelvo/loresuelvo-api/internal/domain/admin"
	coveragezone "github.com/LoResuelvo/loresuelvo-api/internal/domain/coverage_zone"
	"github.com/LoResuelvo/loresuelvo-api/internal/domain/identityverification"
	"github.com/LoResuelvo/loresuelvo-api/internal/domain/provider"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestAdminDirectoryReaderFindsOnlyConsumers(t *testing.T) {
	userRepository, database := newUserRepositoryTest(t)
	require.NoError(t, userRepository.DeleteAll())

	first := consumerWithAddress(t, database, "auth0|admin-directory-ana", "admin.directory.ana@example.com", "Ana", "Perez")
	second := consumerWithAddress(t, database, "auth0|admin-directory-beatriz", "admin.directory.beatriz@example.com", "Beatriz", "Suarez")
	_, err := userRepository.Save(t.Context(), first)
	require.NoError(t, err)
	_, err = userRepository.Save(t.Context(), second)
	require.NoError(t, err)

	profilePhotoFileID := savedProviderProfilePhotoFileID(t, database, first.AuthID())
	_, err = database.ExecContext(
		t.Context(),
		`UPDATE users SET profile_photo_file_id = $1 WHERE id = $2`,
		profilePhotoFileID,
		first.ID(),
	)
	require.NoError(t, err)

	adminUser, err := admin.NewAdmin("auth0|admin-directory-admin", "admin.directory.admin@example.com", "Admin", "User", nil)
	require.NoError(t, err)
	_, err = userRepository.Save(t.Context(), adminUser)
	require.NoError(t, err)
	providerUser := validProviderWithData(
		t,
		repositories.NewCategoryRepository(database),
		database,
		"auth0|admin-directory-provider",
		"admin.directory.provider@example.com",
		"Provider",
		"User",
		"Plomería",
	)
	_, err = userRepository.Save(t.Context(), providerUser)
	require.NoError(t, err)

	reader := repositories.NewAdminDirectoryReader(database)
	consumers, err := reader.FindConsumers(t.Context(), "")

	require.NoError(t, err)
	require.Len(t, consumers, 2)
	assert.Equal(t, first.ID(), consumers[0].ID)
	assert.Equal(t, first.Name(), consumers[0].Name)
	assert.Equal(t, first.Surname(), consumers[0].Surname)
	assert.Equal(t, first.Email(), consumers[0].Email)
	assert.Equal(t, profilePhotoFileID, consumers[0].ProfilePhotoFileID)
	assert.False(t, consumers[0].CreatedOn.IsZero())
	assert.Equal(t, second.ID(), consumers[1].ID)
	assert.Equal(t, second.Email(), consumers[1].Email)
	assert.Empty(t, consumers[1].ProfilePhotoFileID)
}

func TestAdminDirectoryReaderReturnsNonNilEmptyConsumers(t *testing.T) {
	_, database := newUserRepositoryTest(t)
	reader := repositories.NewAdminDirectoryReader(database)

	consumers, err := reader.FindConsumers(t.Context(), "")

	require.NoError(t, err)
	assert.NotNil(t, consumers)
	assert.Empty(t, consumers)
}

func TestAdminDirectoryReaderHonorsContextCancellation(t *testing.T) {
	_, database := newUserRepositoryTest(t)
	reader := repositories.NewAdminDirectoryReader(database)
	ctx, cancel := context.WithCancel(t.Context())
	cancel()

	consumers, err := reader.FindConsumers(ctx, "")

	assert.Nil(t, consumers)
	assert.ErrorIs(t, err, context.Canceled)
}

func TestAdminDirectoryReaderFindsProvidersWithOperationalDataAndLatestVerification(t *testing.T) {
	userRepository, database := newUserRepositoryTest(t)
	require.NoError(t, userRepository.DeleteAll())

	categoryRepository := repositories.NewCategoryRepository(database)
	firstZone := savedCoverageZoneForProvider(t, database, "Comuna 6")
	secondZone := savedCoverageZoneForProvider(t, database, "Comuna 14")
	first := validProviderWithCoverageZones(
		t,
		categoryRepository,
		database,
		"auth0|admin-directory-provider-juan",
		"admin.directory.juan@example.com",
		"Juan",
		"Gómez",
		"Plomería",
		[]coveragezone.CoverageZone{*firstZone, *secondZone},
	)
	second := validProviderWithData(
		t,
		categoryRepository,
		database,
		"auth0|admin-directory-provider-laura",
		"admin.directory.laura@example.com",
		"Laura",
		"Díaz",
		"Plomería",
	)
	third := validProviderWithData(
		t,
		categoryRepository,
		database,
		"auth0|admin-directory-provider-pedro",
		"admin.directory.pedro@example.com",
		"Pedro",
		"Sosa",
		"Plomería",
	)
	_, err := userRepository.Save(t.Context(), first)
	require.NoError(t, err)
	_, err = userRepository.Save(t.Context(), second)
	require.NoError(t, err)
	_, err = userRepository.Save(t.Context(), third)
	require.NoError(t, err)

	consumerUser := consumerWithAddress(t, database, "auth0|admin-directory-provider-consumer", "admin.directory.consumer@example.com", "Ana", "Pérez")
	_, err = userRepository.Save(t.Context(), consumerUser)
	require.NoError(t, err)
	adminUser, err := admin.NewAdmin("auth0|admin-directory-provider-admin", "admin.directory.supervisor@example.com", "Sofía", "López", nil)
	require.NoError(t, err)
	_, err = userRepository.Save(t.Context(), adminUser)
	require.NoError(t, err)

	verificationRepository := repositories.NewIdentityVerificationRepository(database)
	workflowID := uuid.MustParse("10000000-0000-0000-0000-000000000001")
	inReviewOn := time.Date(2026, 9, 18, 14, 0, 0, 0, time.UTC)
	inReview, err := identityverification.Rehydrate(
		uuid.MustParse("20000000-0000-0000-0000-000000000001"),
		first.ID(), "didit", workflowID, 1, identityverification.StatusInReview, inReviewOn, inReviewOn,
	)
	require.NoError(t, err)
	require.NoError(t, verificationRepository.Save(t.Context(), inReview))
	verifiedOn := time.Date(2026, 9, 18, 15, 30, 0, 0, time.UTC)
	approved, err := identityverification.RehydrateWithMetadata(
		uuid.MustParse("20000000-0000-0000-0000-000000000002"),
		first.ID(), "didit", workflowID, 1, identityverification.StatusApproved,
		verifiedOn, verifiedOn, nil, &verifiedOn, &verifiedOn,
	)
	require.NoError(t, err)
	require.NoError(t, verificationRepository.Save(t.Context(), approved))
	declinedOn := verifiedOn.Add(time.Hour)
	declinedWithStaleVerifiedOn, err := identityverification.RehydrateWithMetadata(
		uuid.MustParse("20000000-0000-0000-0000-000000000003"),
		third.ID(), "didit", workflowID, 1, identityverification.StatusDeclined,
		declinedOn, declinedOn, nil, &declinedOn, &verifiedOn,
	)
	require.NoError(t, err)
	require.NoError(t, verificationRepository.Save(t.Context(), declinedWithStaleVerifiedOn))

	reader := repositories.NewAdminDirectoryReader(database)
	providers, err := reader.FindProviders(t.Context(), admin.ProviderDirectoryFilter{})

	require.NoError(t, err)
	require.Len(t, providers, 3)
	assert.Equal(t, first.ID(), providers[0].ID)
	assert.Equal(t, first.Email(), providers[0].Email)
	assert.Equal(t, first.Category.ID, providers[0].Category.ID)
	assert.Equal(t, "Plomería", providers[0].Category.Name)
	require.Len(t, providers[0].CoverageZones, 2)
	assert.Equal(t, firstZone.ID, providers[0].CoverageZones[0].ID)
	assert.Equal(t, firstZone.MarketID, providers[0].CoverageZones[0].MarketID)
	assert.Equal(t, firstZone.Kind, providers[0].CoverageZones[0].Kind)
	assert.Equal(t, secondZone.ID, providers[0].CoverageZones[1].ID)
	assert.Equal(t, identityverification.StatusApproved, providers[0].IdentityVerificationStatus)
	require.NotNil(t, providers[0].IdentityVerifiedOn)
	assert.Equal(t, verifiedOn, *providers[0].IdentityVerifiedOn)
	assert.Equal(t, second.ID(), providers[1].ID)
	assert.Equal(t, identityverification.StatusUnverified, providers[1].IdentityVerificationStatus)
	assert.Nil(t, providers[1].IdentityVerifiedOn)
	assert.Equal(t, third.ID(), providers[2].ID)
	assert.Equal(t, identityverification.StatusDeclined, providers[2].IdentityVerificationStatus)
	assert.Nil(t, providers[2].IdentityVerifiedOn)
}

func TestAdminDirectoryReaderReturnsNonNilEmptyProviders(t *testing.T) {
	userRepository, database := newUserRepositoryTest(t)
	require.NoError(t, userRepository.DeleteAll())
	reader := repositories.NewAdminDirectoryReader(database)

	providers, err := reader.FindProviders(t.Context(), admin.ProviderDirectoryFilter{})

	require.NoError(t, err)
	assert.NotNil(t, providers)
	assert.Empty(t, providers)
}

func TestAdminDirectoryReaderFindProvidersHonorsContextCancellation(t *testing.T) {
	_, database := newUserRepositoryTest(t)
	reader := repositories.NewAdminDirectoryReader(database)
	ctx, cancel := context.WithCancel(t.Context())
	cancel()

	providers, err := reader.FindProviders(ctx, admin.ProviderDirectoryFilter{})

	assert.Nil(t, providers)
	assert.ErrorIs(t, err, context.Canceled)
}

func TestAdminDirectoryReaderSearchesConsumersByNameSurnameAndEmail(t *testing.T) {
	userRepository, database := newUserRepositoryTest(t)
	require.NoError(t, userRepository.DeleteAll())
	ana := consumerWithAddress(t, database, "auth0|admin-search-ana", "ana.perez@example.com", "Ana", "Pérez")
	beatriz := consumerWithAddress(t, database, "auth0|admin-search-beatriz", "beatriz@example.com", "Beatriz", "Suárez")
	_, err := userRepository.Save(t.Context(), ana)
	require.NoError(t, err)
	_, err = userRepository.Save(t.Context(), beatriz)
	require.NoError(t, err)
	reader := repositories.NewAdminDirectoryReader(database)

	for _, query := range []string{"ANA", "PÉREZ", "ANA.PEREZ@EXAMPLE.COM", "ana.perez", "aNa"} {
		t.Run(query, func(t *testing.T) {
			found, err := reader.FindConsumers(t.Context(), query)

			require.NoError(t, err)
			require.Len(t, found, 1)
			assert.Equal(t, ana.ID(), found[0].ID)
		})
	}
	for _, query := range []string{"inexistente", "%", "_"} {
		t.Run(query, func(t *testing.T) {
			found, err := reader.FindConsumers(t.Context(), query)

			require.NoError(t, err)
			assert.NotNil(t, found)
			assert.Empty(t, found)
		})
	}
}

func TestAdminDirectoryReaderSearchesProvidersWithoutTrimmingCoverageZones(t *testing.T) {
	userRepository, database := newUserRepositoryTest(t)
	require.NoError(t, userRepository.DeleteAll())
	categoryRepository := repositories.NewCategoryRepository(database)
	firstZone := savedCoverageZoneForProvider(t, database, "Comuna 6")
	secondZone := savedCoverageZoneForProvider(t, database, "Comuna 14")
	juan := validProviderWithCoverageZones(
		t, categoryRepository, database,
		"auth0|admin-search-juan", "juan.gomez@example.com", "Juan", "Gómez", "Plomería",
		[]coveragezone.CoverageZone{*firstZone, *secondZone},
	)
	laura := validProviderWithData(
		t, categoryRepository, database,
		"auth0|admin-search-laura", "laura@example.com", "Laura", "Díaz", "Plomería",
	)
	_, err := userRepository.Save(t.Context(), juan)
	require.NoError(t, err)
	_, err = userRepository.Save(t.Context(), laura)
	require.NoError(t, err)
	reader := repositories.NewAdminDirectoryReader(database)

	for _, query := range []string{"JUAN", "GÓMEZ", "JUAN.GOMEZ@EXAMPLE.COM", "juan.gomez"} {
		t.Run(query, func(t *testing.T) {
			found, err := reader.FindProviders(t.Context(), admin.ProviderDirectoryFilter{Query: query})

			require.NoError(t, err)
			require.Len(t, found, 1)
			assert.Equal(t, juan.ID(), found[0].ID)
			require.Len(t, found[0].CoverageZones, 2)
			assert.Equal(t, firstZone.ID, found[0].CoverageZones[0].ID)
			assert.Equal(t, secondZone.ID, found[0].CoverageZones[1].ID)
		})
	}
	for _, query := range []string{"inexistente", "%", "_"} {
		t.Run(query, func(t *testing.T) {
			found, err := reader.FindProviders(t.Context(), admin.ProviderDirectoryFilter{Query: query})

			require.NoError(t, err)
			assert.NotNil(t, found)
			assert.Empty(t, found)
		})
	}
}

func TestAdminDirectoryReaderFiltersProvidersWithoutTrimmingCoverageZones(t *testing.T) {
	userRepository, database := newUserRepositoryTest(t)
	require.NoError(t, userRepository.DeleteAll())
	categoryRepository := repositories.NewCategoryRepository(database)
	firstZone := savedCoverageZoneForProvider(t, database, "Comuna 6")
	secondZone := savedCoverageZoneForProvider(t, database, "Comuna 14")
	juan := validProviderWithCoverageZones(t, categoryRepository, database,
		"auth0|admin-filter-juan", "admin.filter.juan@example.com", "Juan", "Gómez", "Plomería",
		[]coveragezone.CoverageZone{*firstZone, *secondZone})
	laura := validProviderWithCoverageZones(t, categoryRepository, database,
		"auth0|admin-filter-laura", "admin.filter.laura@example.com", "Laura", "Díaz", "Electricidad",
		[]coveragezone.CoverageZone{*secondZone})
	pedro := validProviderWithCoverageZones(t, categoryRepository, database,
		"auth0|admin-filter-pedro", "admin.filter.pedro@example.com", "Pedro", "Ruiz", "Plomería",
		[]coveragezone.CoverageZone{*firstZone})
	for _, user := range []*provider.Provider{juan, laura, pedro} {
		_, err := userRepository.Save(t.Context(), user)
		require.NoError(t, err)
	}

	verificationRepository := repositories.NewIdentityVerificationRepository(database)
	earlier := time.Now().UTC().Add(-time.Hour)
	previous, err := identityverification.Rehydrate(
		uuid.New(), juan.ID(), "didit", uuid.New(), 1, identityverification.StatusInReview, earlier, earlier,
	)
	require.NoError(t, err)
	require.NoError(t, verificationRepository.Save(t.Context(), previous))
	for _, record := range []struct {
		providerID int
		status     identityverification.VerificationStatus
	}{
		{juan.ID(), identityverification.StatusApproved},
		{laura.ID(), identityverification.StatusDeclined},
	} {
		now := time.Now().UTC()
		verification, err := identityverification.Rehydrate(
			uuid.New(), record.providerID, "didit", uuid.New(), 1, record.status, now, now,
		)
		require.NoError(t, err)
		require.NoError(t, verificationRepository.Save(t.Context(), verification))
	}

	reader := repositories.NewAdminDirectoryReader(database)
	categoryID := juan.Category.ID
	verificationApproved := identityverification.StatusApproved
	verificationDeclined := identityverification.StatusDeclined
	verificationUnverified := identityverification.StatusUnverified
	tests := []struct {
		name      string
		filter    admin.ProviderDirectoryFilter
		wantID    int
		wantCount int
		wantZones int
	}{
		{"category", admin.ProviderDirectoryFilter{CategoryID: &laura.Category.ID}, laura.ID(), 1, 1},
		{"coverage zone", admin.ProviderDirectoryFilter{CoverageZoneID: &firstZone.ID}, juan.ID(), 2, 2},
		{"approved", admin.ProviderDirectoryFilter{IdentityVerificationStatus: &verificationApproved}, juan.ID(), 1, 2},
		{"declined", admin.ProviderDirectoryFilter{IdentityVerificationStatus: &verificationDeclined}, laura.ID(), 1, 1},
		{"unverified", admin.ProviderDirectoryFilter{IdentityVerificationStatus: &verificationUnverified}, pedro.ID(), 1, 1},
		{"combined", admin.ProviderDirectoryFilter{
			CategoryID: &categoryID, CoverageZoneID: &firstZone.ID,
			IdentityVerificationStatus: &verificationApproved,
		}, juan.ID(), 1, 2},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			found, err := reader.FindProviders(t.Context(), test.filter)

			require.NoError(t, err)
			require.Len(t, found, test.wantCount)
			assert.Equal(t, test.wantID, found[0].ID)
			assert.Len(t, found[0].CoverageZones, test.wantZones)
		})
	}
}
