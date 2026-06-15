package chatbot

import (
	"fmt"
	"os"
	"strings"

	"github.com/LoResuelvo/loresuelvo-api/internal/domain/conversation"
)

func NewChatbotFromEnv() conversation.Chatbot {
	provider := strings.ToLower(envOrDefault("CHATBOT_PROVIDER", "gemini"))
	model := envOrDefault("CHATBOT_MODEL", defaultGeminiModel)
	apiKey := envOrDefault("CHATBOT_API_KEY", "")

	switch provider {
	case "gemini":
		return NewGeminiChatbot(model, apiKey)
	case "fake":
		ensureFakeChatbotAllowed()
		return NewFakeChatbot()
	default:
		panic(fmt.Sprintf("unsupported chatbot provider: %s", provider))
	}
}

func envOrDefault(key, fallback string) string {
	value := strings.TrimSpace(os.Getenv(key))
	if value == "" {
		return fallback
	}
	return value
}

func ensureFakeChatbotAllowed() {
	environment := strings.ToLower(envOrDefault("ENVIRONMENT", "production"))
	switch environment {
	case "dev":
		return
	default:
		panic(fmt.Sprintf("fake chatbot provider is not allowed in %s environment", environment))
	}
}
