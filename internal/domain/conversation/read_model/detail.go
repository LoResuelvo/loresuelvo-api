package readmodel

import (
	"time"

	filedomain "github.com/LoResuelvo/loresuelvo-api/internal/domain/file"
	providerreadmodel "github.com/LoResuelvo/loresuelvo-api/internal/domain/provider/read_model"
)

type ConversationDetail struct {
	ID        int
	Type      string
	Status    string
	Messages  []MessageDetail
	UpdatedOn time.Time
	Work      *WorkConversationDetail
	Chatbot   *ChatbotConversationDetail
}

type WorkConversationDetail struct {
	Counterpart ConversationParticipant
}

type ChatbotConversationDetail struct {
	Title                string
	ResponseStatus       string
	DiagnosisCompleted   bool
	RecommendedCategory  *RecommendedCategory
	RecommendedProviders []providerreadmodel.ProviderSummary
}

type RecommendedCategory struct {
	ID   int
	Name string
}

type MessageDetail struct {
	ID         int
	SenderRole string
	Content    string
	Images     []filedomain.MessageImage
	CreatedOn  time.Time
}
