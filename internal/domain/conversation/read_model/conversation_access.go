package readmodel

// ConversationAccess identifies the participants without hydrating the conversation.
type ConversationAccess struct {
	ID         int
	Type       string
	ConsumerID int
	ProviderID int
}
