package conversation

import "strings"

type ChatBotConversation struct {
	*BaseConversation
	ConsumerID int
	Title      string
}

func NewChatBotConversation(consumerID int, title string) (Conversation, error) {
	if consumerID <= 0 {
		return nil, ErrOnlyConsumerCanMessageChatbot
	}
	trimmedTitle := strings.TrimSpace(title)
	if trimmedTitle == "" {
		trimmedTitle = "Consulta del hogar"
	}
	return &ChatBotConversation{
		BaseConversation: &BaseConversation{
			Type:   TypeChatbot,
			Status: StatusActive,
		},
		ConsumerID: consumerID,
		Title:      trimmedTitle,
	}, nil
}

func NewChatbotConversation(consumerID int, title string) (Conversation, error) {
	return NewChatBotConversation(consumerID, title)
}
