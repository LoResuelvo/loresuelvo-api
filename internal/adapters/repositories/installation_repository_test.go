package repositories_test

import (
	"github.com/LoResuelvo/loresuelvo-api/internal/adapters/repositories"
	"github.com/LoResuelvo/loresuelvo-api/internal/domain/installation"
	"github.com/stretchr/testify/require"
	"testing"
)

func TestInstallationRepositoryStoresAndFindsRecipientDevices(t *testing.T) {
	fixture := newNotificationRepositoryTest(t)
	user := consumerWithAddress(t, fixture.database, "auth0|push-installation", "push-installation@example.com", "Ana", "Perez")
	_, err := fixture.userRepository.Save(t.Context(), user)
	require.NoError(t, err)
	id, err := fixture.userRepository.FindIDByEmail(user.Email())
	require.NoError(t, err)
	repository := repositories.NewInstallationRepository(fixture.database)
	device, err := installation.New("test-phone", id, "consumer", "test-token", "en", "test-binding")
	require.NoError(t, err)
	require.NoError(t, repository.Save(t.Context(), device))
	found, err := repository.FindByUserID(t.Context(), id)
	require.NoError(t, err)
	require.Equal(t, []installation.Installation{*device}, found)
	other, err := repository.FindByUserID(t.Context(), id+1)
	require.NoError(t, err)
	require.Empty(t, other)
}
