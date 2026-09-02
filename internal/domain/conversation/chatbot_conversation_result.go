package conversation

import (
	"github.com/LoResuelvo/loresuelvo-api/internal/domain/category"
	"github.com/LoResuelvo/loresuelvo-api/internal/domain/provider"
)

type ChatbotConversationTurnResult struct {
	Conversation          Conversation
	ResponseStatus        ChatbotResponseStatus
	Assessment            *ProblemAssessment
	ProblemCategory       *category.Category
	RecommendedProviders  []provider.Provider
	RecommendationReasons map[int]string
}

type ChatbotConversationResult = ChatbotConversationTurnResult
