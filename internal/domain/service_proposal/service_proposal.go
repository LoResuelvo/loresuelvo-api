package serviceproposal

import (
	"time"

	"github.com/LoResuelvo/loresuelvo-api/internal/domain/clock"
	"github.com/LoResuelvo/loresuelvo-api/internal/domain/consumer"
	"github.com/LoResuelvo/loresuelvo-api/internal/domain/conversation"
	"github.com/LoResuelvo/loresuelvo-api/internal/domain/notification"
	"github.com/LoResuelvo/loresuelvo-api/internal/domain/provider"
	"github.com/LoResuelvo/loresuelvo-api/internal/domain/user"
)

const minimumBookingLeadTime = 24 * time.Hour

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
	ScheduledOn  time.Time
	Description  string
	Status       Status
	CreatedOn    time.Time
	BookingTerms BookingTerms
}

func NewServiceProposal(provider *provider.Provider, consumer *consumer.Consumer, conversation conversation.Conversation, scheduledOn time.Time, description string, bookingTerms BookingTerms, clock clock.Clock) (*ServiceProposal, error) {
	if err := validateParameters(scheduledOn, clock); err != nil {
		return nil, err
	}

	if conversationStatus := conversation.IsActive(); !conversationStatus {
		return nil, ErrConversationNotActive
	}

	return &ServiceProposal{
		Provider:     provider,
		Consumer:     consumer,
		Conversation: conversation,
		ScheduledOn:  scheduledOn,
		Description:  description,
		Status:       StatusPending,
		BookingTerms: bookingTerms,
	}, nil
}

func (sp *ServiceProposal) CreateReceivedNotification(clock clock.Clock) *notification.Notification {
	return sp.createNotification(
		sp.Consumer.ID(),
		notification.TypeServiceProposalReceived,
		clock,
	)
}

func (sp *ServiceProposal) CreateAcceptedNotification(clock clock.Clock) *notification.Notification {
	return sp.createNotification(
		sp.Provider.ID(),
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
	if sp.Consumer == nil || sp.Consumer.ID() != consumerID {
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
	return sp.BookingTerms.ServiceTotalCents()
}

func (sp *ServiceProposal) ServiceProposalScheduledOn() time.Time {
	return sp.ScheduledOn
}

func (sp *ServiceProposal) ServiceProposalDescription() string {
	return sp.Description
}

func (sp *ServiceProposal) ConsumerID() int {
	return sp.Consumer.ID()
}

func (sp *ServiceProposal) ProviderID() int {
	return sp.Provider.ID()
}

func (sp *ServiceProposal) CounterpartFor(authID string) (user.User, error) {
	if sp.Consumer.AuthID() == authID {
		return sp.Provider, nil
	}
	if sp.Provider.AuthID() == authID {
		return sp.Consumer, nil
	}
	return nil, ErrOnlyParticipantCanView
}

func validateParameters(scheduledOn time.Time, clock clock.Clock) error {
	now := clock.Now()
	if !scheduledOn.After(now) {
		return ErrInvalidScheduledOn
	}
	if !scheduledOn.After(now.Add(minimumBookingLeadTime)) {
		return ErrInsufficientBookingLeadTime
	}

	return nil
}
