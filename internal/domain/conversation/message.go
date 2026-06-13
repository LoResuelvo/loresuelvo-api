package conversation

import (
	"strings"
	"time"
)

const (
	SenderConsumer = "consumer"
	SenderProvider = "provider"
	SenderChatbot  = "chatbot"
)

type Message struct {
	ID             int
	ConversationID int
	SenderRole     string
	Content        string
	CreatedOn      time.Time
}

func NewConsumerMessage(content string) (*Message, error) {
	return newMessage(SenderConsumer, content)
}

func NewProviderMessage(content string) (*Message, error) {
	return newMessage(SenderProvider, content)
}

func NewChatbotMessage(content string) (*Message, error) {
	return newMessage(SenderChatbot, content)
}

func newMessage(senderRole, content string) (*Message, error) {
	trimmedContent := strings.TrimSpace(content)
	if trimmedContent == "" {
		return nil, ErrMessageRequired
	}

	return &Message{
		SenderRole: senderRole,
		Content:    trimmedContent,
	}, nil
}
