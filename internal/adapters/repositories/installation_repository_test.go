package repositories_test

import (
	"errors"
	"testing"

	"github.com/LoResuelvo/loresuelvo-api/internal/adapters/repositories"
	"github.com/LoResuelvo/loresuelvo-api/internal/domain/installation"
	"github.com/stretchr/testify/require"
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

func TestInstallationRepositoryRejectsConcurrentFirstRegistration(t *testing.T) {
	fixture := newNotificationRepositoryTest(t)
	user := consumerWithAddress(t, fixture.database, "auth0|push-create-race", "push-create-race@example.com", "Ana", "Perez")
	_, err := fixture.userRepository.Save(t.Context(), user)
	require.NoError(t, err)
	id, err := fixture.userRepository.FindIDByEmail(user.Email())
	require.NoError(t, err)
	repository := repositories.NewInstallationRepository(fixture.database)
	first, err := installation.New("race-phone", id, "consumer", "race-token", "es", "race-binding")
	require.NoError(t, err)
	second := *first
	start := make(chan struct{})
	results := make(chan error, 2)
	for _, candidate := range []*installation.Installation{first, &second} {
		go func(i *installation.Installation) { <-start; results <- repository.Save(t.Context(), i) }(candidate)
	}
	close(start)
	a, b := <-results, <-results
	require.True(t, (a == nil && errors.Is(b, installation.ErrConflict)) || (b == nil && errors.Is(a, installation.ErrConflict)))
}
func TestInstallationRepositoryRejectsStaleRevision(t *testing.T) {
	fixture := newNotificationRepositoryTest(t)
	user := consumerWithAddress(t, fixture.database, "auth0|push-revision", "push-revision@example.com", "Ana", "Perez")
	_, err := fixture.userRepository.Save(t.Context(), user)
	require.NoError(t, err)
	id, err := fixture.userRepository.FindIDByEmail(user.Email())
	require.NoError(t, err)
	repository := repositories.NewInstallationRepository(fixture.database)
	i, err := installation.New("revision-phone", id, "consumer", "revision-token", "es", "revision-binding")
	require.NoError(t, err)
	require.NoError(t, repository.Save(t.Context(), i))
	current, err := repository.FindByID(t.Context(), i.ID)
	require.NoError(t, err)
	stale := *current
	current.Token = "renewed-revision-token"
	require.NoError(t, repository.Save(t.Context(), current))
	stale.Enabled = false
	require.ErrorIs(t, repository.Save(t.Context(), &stale), installation.ErrConflict)
	found, err := repository.FindByID(t.Context(), i.ID)
	require.NoError(t, err)
	require.True(t, found.Enabled)
	require.Equal(t, "renewed-revision-token", found.Token)
}
func TestInstallationRepositoryMapsDuplicateTokenToConflict(t *testing.T) {
	fixture := newNotificationRepositoryTest(t)
	user := consumerWithAddress(t, fixture.database, "auth0|push-duplicate", "push-duplicate@example.com", "Ana", "Perez")
	_, err := fixture.userRepository.Save(t.Context(), user)
	require.NoError(t, err)
	id, err := fixture.userRepository.FindIDByEmail(user.Email())
	require.NoError(t, err)
	repository := repositories.NewInstallationRepository(fixture.database)
	first, err := installation.New("token-phone-one", id, "consumer", "same-token", "es", "binding-one")
	require.NoError(t, err)
	require.NoError(t, repository.Save(t.Context(), first))
	second, err := installation.New("token-phone-two", id, "consumer", "same-token", "es", "binding-two")
	require.NoError(t, err)
	require.ErrorIs(t, repository.Save(t.Context(), second), installation.ErrConflict)
}
func TestInstallationRepositoryExcludesDisabledDevices(t *testing.T) {
	fixture := newNotificationRepositoryTest(t)
	user := consumerWithAddress(t, fixture.database, "auth0|push-disabled", "push-disabled@example.com", "Ana", "Perez")
	_, err := fixture.userRepository.Save(t.Context(), user)
	require.NoError(t, err)
	id, err := fixture.userRepository.FindIDByEmail(user.Email())
	require.NoError(t, err)
	repository := repositories.NewInstallationRepository(fixture.database)
	i, err := installation.New("disabled-phone", id, "consumer", "disabled-token", "es", "disabled-binding")
	require.NoError(t, err)
	require.NoError(t, repository.Save(t.Context(), i))
	i.Enabled = false
	require.NoError(t, repository.Save(t.Context(), i))
	devices, err := repository.FindByUserID(t.Context(), id)
	require.NoError(t, err)
	require.Empty(t, devices)
}
