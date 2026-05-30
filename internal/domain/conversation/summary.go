package conversation

import (
	"time"

	"github.com/LoResuelvo/loresuelvo-api/internal/domain/consumer"
	"github.com/LoResuelvo/loresuelvo-api/internal/domain/provider"
)

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

func BuildConsumerSummaries(
	conversations []Conversation,
	providers map[int]provider.Provider,
	messages map[int]Message,
) []ConversationSummary {
	summaries := make([]ConversationSummary, 0, len(conversations))
	for _, conv := range conversations {
		prov := providers[conv.ProviderID]
		categoryName := ""
		if prov.Category != nil {
			categoryName = prov.Category.Name
		}
		summaries = append(summaries, ConversationSummary{
			ID:     conv.ID,
			Status: conv.Status,
			Counterpart: ConversationParticipant{
				ID:           conv.ProviderID,
				Role:         SenderProvider,
				Name:         prov.User.Name,
				Surname:      prov.User.Surname,
				CategoryName: categoryName,
			},
			LastMessage: lastMessageToSummary(messages[conv.ID]),
			UpdatedOn:   conv.UpdatedOn,
		})
	}
	return summaries
}

func BuildProviderSummaries(
	conversations []Conversation,
	consumers map[int]consumer.Consumer,
	messages map[int]Message,
) []ConversationSummary {
	summaries := make([]ConversationSummary, 0, len(conversations))
	for _, conv := range conversations {
		cons := consumers[conv.ConsumerID]
		summaries = append(summaries, ConversationSummary{
			ID:     conv.ID,
			Status: conv.Status,
			Counterpart: ConversationParticipant{
				ID:      conv.ConsumerID,
				Role:    SenderConsumer,
				Name:    cons.User.Name,
				Surname: cons.User.Surname,
			},
			LastMessage: lastMessageToSummary(messages[conv.ID]),
			UpdatedOn:   conv.UpdatedOn,
		})
	}
	return summaries
}

func lastMessageToSummary(msg Message) *MessageSummary {
	if msg.ID == 0 {
		return nil
	}
	return &MessageSummary{
		ID:         msg.ID,
		SenderRole: msg.SenderRole,
		Content:    msg.Content,
		CreatedOn:  msg.CreatedOn,
	}
}
