package repositories_test

import (
	"testing"

	"github.com/LoResuelvo/loresuelvo-api/internal/adapters/repositories"
	"github.com/LoResuelvo/loresuelvo-api/internal/domain/user"
	"github.com/stretchr/testify/require"
)

func TestProviderActivityActorFinderDistinguishesProviderConsumerAndMissingIdentity(t *testing.T) {
	testContext := newServiceProposalRepositoryTest(t)
	providerUser := savedProviderIDWithData(t, jobRequestRepositoryTestContext{
		database: testContext.database, userRepository: testContext.userRepository,
		categoryRepository: testContext.categoryRepository,
	}, "auth0|activity-actor-provider", "activity.actor.provider@example.com", "Juan", "Gomez", "Plomeria")
	consumerID := savedConsumerIDWithData(t, jobRequestRepositoryTestContext{
		database: testContext.database, userRepository: testContext.userRepository,
	}, "auth0|activity-actor-consumer", "activity.actor.consumer@example.com", "Ana", "Perez")
	finder := repositories.NewProviderActivityActorFinder(testContext.database)

	gotProviderID, providerRole, err := finder.FindByAuthID(t.Context(), "auth0|activity-actor-provider")
	require.NoError(t, err)
	require.Equal(t, providerUser, gotProviderID)
	require.Equal(t, "provider", providerRole)

	gotConsumerProviderID, consumerRole, err := finder.FindByAuthID(t.Context(), "auth0|activity-actor-consumer")
	require.NoError(t, err)
	require.Zero(t, gotConsumerProviderID)
	require.Equal(t, "consumer", consumerRole)
	require.Positive(t, consumerID)

	_, _, err = finder.FindByAuthID(t.Context(), "auth0|activity-actor-missing")
	require.ErrorIs(t, err, user.ErrNotFound)
}
