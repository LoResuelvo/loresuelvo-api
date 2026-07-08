package serviceproposal

import (
	"time"

	"github.com/LoResuelvo/loresuelvo-api/internal/domain/clock"
	"github.com/LoResuelvo/loresuelvo-api/internal/domain/consumer"
	"github.com/LoResuelvo/loresuelvo-api/internal/domain/conversation"
	"github.com/LoResuelvo/loresuelvo-api/internal/domain/notification"
	"github.com/LoResuelvo/loresuelvo-api/internal/domain/provider"
)

type Status string

const (
	StatusPending  Status = "pending"
	StatusAccepted Status = "accepted"
	StatusRejected Status = "rejected"
)

type ServiceProposal struct {
	ID           int
	Provider     *provider.Provider
	Consumer     *consumer.Consumer
	Conversation conversation.Conversation
	Amount       int64
	ScheduledOn  time.Time
	Description  string
	Status       Status
	CreatedOn    time.Time
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

func (sp *ServiceProposal) CreateReceivedNotification(clock clock.Clock) *notification.Notification {
	return sp.createNotification(
		sp.Consumer.ID,
		notification.TypeServiceProposalReceived,
		clock,
	)
}

func (sp *ServiceProposal) CreateAcceptedNotification(clock clock.Clock) *notification.Notification {
	return sp.createNotification(
		sp.Provider.ID,
		notification.TypeServiceProposalAccepted,
		clock,
	)
}

func (sp *ServiceProposal) createNotification(recipientID int, notificationType notification.Type, clock clock.Clock) *notification.Notification {
	return notification.NewNotification(
		recipientID,
		notificationType,
		notification.ResourceServiceProposal,
		sp.ID,
		clock,
	)
}

func (sp *ServiceProposal) Accept(consumerID int, acceptedOn time.Time) error {
	if sp.Consumer == nil || sp.Consumer.ID != consumerID {
		return ErrOnlyRecipientCanAccept
	}
	if sp.Status != StatusPending {
		return ErrOnlyPendingCanBeAccepted
	}
	if !sp.ScheduledOn.After(acceptedOn) {
		return ErrServiceProposalExpired
	}

	sp.Status = StatusAccepted
	return nil
}

func (sp *ServiceProposal) ServiceProposalID() int {
	return sp.ID
}

func (sp *ServiceProposal) ServiceProposalAmount() int64 {
	return sp.Amount
}

func (sp *ServiceProposal) ServiceProposalScheduledOn() time.Time {
	return sp.ScheduledOn
}

func (sp *ServiceProposal) ServiceProposalDescription() string {
	return sp.Description
}

func (sp *ServiceProposal) ConsumerID() int {
	return sp.Consumer.ID
}

func (sp *ServiceProposal) ProviderID() int {
	return sp.Provider.ID
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
