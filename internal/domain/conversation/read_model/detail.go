package readmodel

import (
	"time"

	filedomain "github.com/LoResuelvo/loresuelvo-api/internal/domain/file"
	"github.com/LoResuelvo/loresuelvo-api/internal/domain/provider"
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
	Assessment           *ProblemAssessmentDetail
	RecommendedProviders []provider.Provider
}

type ProblemAssessmentDetail struct {
	Outcome         string
	ProblemCategory *ProblemCategory
}

type ProblemCategory struct {
	ID   int
	Name string
}

type MessageDetail struct {
	ID         int
	SenderRole string
	Content    string
	Images     []filedomain.MessageImage
	Audio      *filedomain.MessageAudio
	Video      *filedomain.MessageVideo
	CreatedOn  time.Time
}
