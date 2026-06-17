package conversation

import (
	"time"
)

const (
	StatusPending               = "pending"
	StatusActive                = "active"
	PendingConsumerMessageLimit = 5
	TypeWork                    = "work"
	TypeChatbot                 = "chatbot"
)

type Conversation interface {
	ConversationType() string
	Base() *BaseConversation
	Activate() error
	AddMessage(message Message)
	Messages() []Message
	SetMessages(messages []Message)
}

type BaseConversation struct {
	ID        int
	Type      string
	Status    string
	UpdatedOn time.Time
	messages  []Message
}

func (conversation *BaseConversation) AddMessage(message Message) {
	conversation.messages = append(conversation.messages, message)
}

func (conversation *BaseConversation) Activate() error {
	if conversation.Status != StatusPending {
		return ErrOnlyPendingConversationCanBeActivated
	}

	conversation.Status = StatusActive
	return nil
}

func (conversation *BaseConversation) ConversationType() string {
	return conversation.Type
}

func (conversation *BaseConversation) Base() *BaseConversation {
	return conversation
}

func (conversation *BaseConversation) Messages() []Message {
	return conversation.messages
}

func (conversation *BaseConversation) LastMessage() (Message, bool) {
	if len(conversation.messages) == 0 {
		return Message{}, false
	}

	return conversation.messages[len(conversation.messages)-1], true
}

func (conversation *BaseConversation) SetMessages(messages []Message) {
	conversation.messages = messages
}
