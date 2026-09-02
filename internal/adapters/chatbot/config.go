package chatbot

import (
	"fmt"
	"os"
	"strconv"
	"strings"

	"github.com/LoResuelvo/loresuelvo-api/internal/domain/conversation"
)

const maxRecommendedProvidersEnv = "CHATBOT_MAX_RECOMMENDED_PROVIDERS"

func ProviderRecommendationConfigFromEnv() (conversation.ProviderRecommendationConfig, error) {
	value := strings.TrimSpace(os.Getenv(maxRecommendedProvidersEnv))
	if value == "" {
		return conversation.DefaultProviderRecommendationConfig(), nil
	}

	maxProviders, err := strconv.Atoi(value)
	if err != nil {
		return conversation.ProviderRecommendationConfig{}, fmt.Errorf("parsing %s: %w", maxRecommendedProvidersEnv, err)
	}
	config := conversation.ProviderRecommendationConfig{MaxRecommendedProviders: maxProviders}
	if err := config.Validate(); err != nil {
		return conversation.ProviderRecommendationConfig{}, fmt.Errorf("validating %s: %w", maxRecommendedProvidersEnv, err)
	}
	return config, nil
}
