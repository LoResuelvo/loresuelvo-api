package conversation

import (
	"strings"
	"time"
)

const ChatbotRecentMessageLimit = 10

type ChatbotConversationContext struct {
	Summary                 string
	LastSummarizedMessageID int
}

func NewChatbotConversationContext(summary string, lastSummarizedMessageID int) (ChatbotConversationContext, error) {
	if lastSummarizedMessageID < 0 {
		return ChatbotConversationContext{}, ErrChatbotContextInvalid
	}

	return ChatbotConversationContext{
		Summary:                 strings.TrimSpace(summary),
		LastSummarizedMessageID: lastSummarizedMessageID,
	}, nil
}

const ChatbotProcessingStaleAfter = 5 * time.Minute

type ChatBotConversation struct {
	*BaseConversation
	ConsumerID            int
	Title                 string
	ResponseStatus        ChatbotResponseStatus
	DiagnosisCompleted    bool
	RecommendedCategoryID *int
	Context               ChatbotConversationContext
	ProcessingStartedOn   *time.Time
}

func NewChatbotConversation(consumerID int, title string) (Conversation, error) {
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
		ConsumerID:     consumerID,
		Title:          trimmedTitle,
		ResponseStatus: ChatbotResponseAnswered,
	}, nil
}

func (conversation *ChatBotConversation) ApplyResponse(response ChatbotResponse, recommendedCategoryID *int) {
	conversation.ResponseStatus = response.Status
	conversation.DiagnosisCompleted = response.DiagnosisCompleted
	conversation.RecommendedCategoryID = copyOptionalInt(recommendedCategoryID)
}

func (conversation *ChatBotConversation) UpdateContext(context ChatbotConversationContext) error {
	validatedContext, err := NewChatbotConversationContext(context.Summary, context.LastSummarizedMessageID)
	if err != nil {
		return err
	}

	conversation.Context = validatedContext
	return nil
}

func (conversation *ChatBotConversation) StartProcessing(now time.Time) error {
	if !conversation.canStartProcessing(now) {
		return ErrChatbotConversationAlreadyProcessing
	}

	startedOn := now.UTC()
	conversation.ProcessingStartedOn = &startedOn
	return nil
}

func (conversation *ChatBotConversation) FinishProcessing() {
	conversation.ProcessingStartedOn = nil
}

func (conversation *ChatBotConversation) ProcessingStartedAt() *time.Time {
	if conversation.ProcessingStartedOn == nil {
		return nil
	}

	startedOn := *conversation.ProcessingStartedOn
	return &startedOn
}

func (conversation *ChatBotConversation) AddTurn(consumerMessage Message, chatbotMessage Message) error {
	if consumerMessage.SenderRole != SenderConsumer || chatbotMessage.SenderRole != SenderChatbot {
		return ErrInvalidChatbotTurn
	}

	conversation.AddMessage(consumerMessage)
	conversation.AddMessage(chatbotMessage)
	return nil
}

func (conversation *ChatBotConversation) RecentMessages(limit int) []Message {
	messages := conversation.Messages()
	if limit > 0 && len(messages) > limit {
		messages = messages[len(messages)-limit:]
	}

	recentMessages := make([]Message, len(messages))
	copy(recentMessages, messages)
	return recentMessages
}

func (conversation *ChatBotConversation) MessagesPendingSummary() []Message {
	messages := conversation.Messages()
	pendingMessages := make([]Message, 0, len(messages))
	for _, message := range messages {
		if message.ID > conversation.Context.LastSummarizedMessageID {
			pendingMessages = append(pendingMessages, message)
		}
	}

	return pendingMessages
}

func (conversation *ChatBotConversation) ShouldSummarizeContext(limit int) bool {
	return limit > 0 && len(conversation.MessagesPendingSummary()) >= limit
}

func (conversation *ChatBotConversation) canStartProcessing(now time.Time) bool {
	if conversation.ProcessingStartedOn == nil {
		return true
	}

	return !conversation.ProcessingStartedOn.Add(ChatbotProcessingStaleAfter).After(now)
}

func copyOptionalInt(value *int) *int {
	if value == nil {
		return nil
	}

	copied := *value
	return &copied
}
