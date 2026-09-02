package chatbot

import (
	"testing"

	"github.com/LoResuelvo/loresuelvo-api/internal/domain/conversation"
	"github.com/stretchr/testify/require"
)

func TestProviderRecommendationConfigFromEnvDefaultsToThree(t *testing.T) {
	t.Setenv(maxRecommendedProvidersEnv, "")

	config, err := ProviderRecommendationConfigFromEnv()

	require.NoError(t, err)
	require.Equal(t, conversation.DefaultProviderRecommendationConfig(), config)
}

func TestProviderRecommendationConfigFromEnvParsesPositiveMaximum(t *testing.T) {
	t.Setenv(maxRecommendedProvidersEnv, "7")

	config, err := ProviderRecommendationConfigFromEnv()

	require.NoError(t, err)
	require.Equal(t, 7, config.MaxRecommendedProviders)
}

func TestProviderRecommendationConfigFromEnvRejectsInvalidMaximum(t *testing.T) {
	t.Setenv(maxRecommendedProvidersEnv, "0")

	_, err := ProviderRecommendationConfigFromEnv()

	require.Error(t, err)
}
