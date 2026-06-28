package conversation

import (
	"github.com/LoResuelvo/loresuelvo-api/internal/domain/category"
	providerreadmodel "github.com/LoResuelvo/loresuelvo-api/internal/domain/provider/read_model"
)

type ChatbotConversationTurnResult struct {
	Conversation         Conversation
	ResponseStatus       ChatbotResponseStatus
	Assessment           *ProblemAssessment
	ProblemCategory      *category.Category
	RecommendedProviders []providerreadmodel.ProviderSummary
}

type ChatbotConversationResult = ChatbotConversationTurnResult
