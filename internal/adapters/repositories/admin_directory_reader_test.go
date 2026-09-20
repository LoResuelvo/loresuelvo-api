package repositories_test

import (
	"context"
	"testing"

	"github.com/LoResuelvo/loresuelvo-api/internal/adapters/repositories"
	"github.com/LoResuelvo/loresuelvo-api/internal/domain/admin"
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
	consumers, err := reader.FindConsumers(t.Context())

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

	consumers, err := reader.FindConsumers(t.Context())

	require.NoError(t, err)
	assert.NotNil(t, consumers)
	assert.Empty(t, consumers)
}

func TestAdminDirectoryReaderHonorsContextCancellation(t *testing.T) {
	_, database := newUserRepositoryTest(t)
	reader := repositories.NewAdminDirectoryReader(database)
	ctx, cancel := context.WithCancel(t.Context())
	cancel()

	consumers, err := reader.FindConsumers(ctx)

	assert.Nil(t, consumers)
	assert.ErrorIs(t, err, context.Canceled)
}
