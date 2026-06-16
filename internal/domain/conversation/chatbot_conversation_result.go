package conversation

import providerreadmodel "github.com/LoResuelvo/loresuelvo-api/internal/domain/provider/read_model"

type ChatbotConversationResult struct {
	Conversation            Conversation
	ResponseStatus          ChatbotResponseStatus
	DiagnosisCompleted      bool
	RecommendedCategoryName string
	RecommendedProviders    []providerreadmodel.ProviderSummary
}
