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
	ID() int
	ConversationType() string
	Status() string
	UpdatedOn() time.Time
	SetPersistenceState(id int, updatedOn time.Time)
	IsActive() bool
	Activate() error
	AddMessage(message Message)
	Messages() []Message
	LastMessage() (Message, bool)
	SetMessages(messages []Message)
}

type BaseConversation struct {
	id               int
	conversationType string
	status           string
	updatedOn        time.Time
	messages         []Message
}

func NewBaseConversation(conversationType, status string) *BaseConversation {
	return &BaseConversation{conversationType: conversationType, status: status}
}

func RehydrateBaseConversation(id int, conversationType, status string, updatedOn time.Time, messages []Message) *BaseConversation {
	conversation := &BaseConversation{id: id, conversationType: conversationType, status: status, updatedOn: updatedOn}
	conversation.SetMessages(messages)
	return conversation
}

func (conversation *BaseConversation) AddMessage(message Message) {
	conversation.messages = append(conversation.messages, message)
}

func (conversation *BaseConversation) Activate() error {
	if conversation.status != StatusPending {
		return ErrOnlyPendingConversationCanBeActivated
	}

	conversation.status = StatusActive
	return nil
}

func (conversation *BaseConversation) ID() int {
	return conversation.id
}

func (conversation *BaseConversation) Status() string {
	return conversation.status
}

func (conversation *BaseConversation) UpdatedOn() time.Time {
	return conversation.updatedOn
}

func (conversation *BaseConversation) SetPersistenceState(id int, updatedOn time.Time) {
	conversation.id = id
	conversation.updatedOn = updatedOn
}

func (conversation *BaseConversation) Messages() []Message {
	return conversation.messages
}

func (conversation *BaseConversation) ConversationType() string {
	return conversation.conversationType
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

func (conversation *BaseConversation) IsActive() bool {
	return conversation.status == StatusActive
}
