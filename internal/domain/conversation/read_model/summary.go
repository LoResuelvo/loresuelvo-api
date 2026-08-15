package readmodel

import (
	"time"

	filedomain "github.com/LoResuelvo/loresuelvo-api/internal/domain/file"
)

type ConversationSummary struct {
	ID          int
	Type        string
	Status      string
	LastMessage *MessageSummary
	UpdatedOn   time.Time
	Work        *WorkConversationSummary
	Chatbot     *ChatbotConversationSummary
}

type WorkConversationSummary struct {
	Counterpart ConversationParticipant
}

type ChatbotConversationSummary struct {
	Title string
}

type ConversationParticipant struct {
	ID                 int
	Role               string
	Name               string
	Surname            string
	CategoryName       string
	ProfilePhotoFileID string
	ProfilePhotoURL    string
}

type MessageSummary struct {
	ID         int
	SenderRole string
	Content    string
	Audio      *filedomain.MessageAudio
	CreatedOn  time.Time
}
