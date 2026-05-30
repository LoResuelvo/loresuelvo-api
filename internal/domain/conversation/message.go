package conversation

import "strings"

const (
	SenderConsumer = "consumer"
	SenderProvider = "provider"
)

type Message struct {
	ID             int
	ConversationID int
	SenderRole     string
	Content        string
}

func NewConsumerMessage(content string) (*Message, error) {
	trimmedContent := strings.TrimSpace(content)
	if trimmedContent == "" {
		return nil, ErrMessageRequired
	}

	return &Message{
		SenderRole: SenderConsumer,
		Content:    trimmedContent,
	}, nil
}
