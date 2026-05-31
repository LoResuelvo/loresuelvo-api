package readmodel

import "time"

type ConversationDetail struct {
	ID          int
	Status      string
	Counterpart ConversationParticipant
	Messages    []MessageDetail
	UpdatedOn   time.Time
}

type MessageDetail struct {
	ID         int
	SenderRole string
	Content    string
	CreatedOn  time.Time
}
