package readmodel

import "time"

type ConversationSummary struct {
	ID          int
	Status      string
	Counterpart ConversationParticipant
	LastMessage *MessageSummary
	UpdatedOn   time.Time
}

type ConversationParticipant struct {
	ID           int
	Role         string
	Name         string
	Surname      string
	CategoryName string
}

type MessageSummary struct {
	ID         int
	SenderRole string
	Content    string
	CreatedOn  time.Time
}
