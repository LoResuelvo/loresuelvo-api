package serviceproposal

import (
	"time"

	"github.com/LoResuelvo/loresuelvo-api/internal/domain/clock"
	"github.com/LoResuelvo/loresuelvo-api/internal/domain/consumer"
	"github.com/LoResuelvo/loresuelvo-api/internal/domain/conversation"
	"github.com/LoResuelvo/loresuelvo-api/internal/domain/provider"
)

type Status string

const (
	StatusPending  Status = "pending"
	StatusAccepted Status = "accepted"
	StatusRejected Status = "rejected"
)

type ServiceProposal struct {
	Provider     *provider.Provider
	Consumer     *consumer.Consumer
	Conversation conversation.Conversation
	Amount       int64
	ScheduledOn  time.Time
	Description  string
	Status       Status
}

func NewServiceProposal(provider *provider.Provider, consumer *consumer.Consumer, conversation conversation.Conversation, amount int64, scheduledOn time.Time, description string, clock clock.Clock) (*ServiceProposal, error) {
	if err := validateParameters(amount, scheduledOn, clock); err != nil {
		return nil, err
	}

	if conversationStatus := conversation.IsActive(); !conversationStatus {
		return nil, ErrConversationNotActive
	}

	return &ServiceProposal{
		Provider:     provider,
		Consumer:     consumer,
		Conversation: conversation,
		Amount:       amount,
		ScheduledOn:  scheduledOn,
		Description:  description,
		Status:       StatusPending,
	}, nil
}

func validateParameters(amount int64, scheduledOn time.Time, clock clock.Clock) error {
	if amount <= 0 {
		return ErrInvalidAmount
	}

	if scheduledOn.Before(clock.Now()) {
		return ErrInvalidScheduledOn
	}

	return nil
}
