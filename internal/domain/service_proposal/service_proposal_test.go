package serviceproposal_test

import (
	"testing"
	"time"

	"github.com/LoResuelvo/loresuelvo-api/internal/domain/consumer"
	"github.com/LoResuelvo/loresuelvo-api/internal/domain/conversation"
	"github.com/LoResuelvo/loresuelvo-api/internal/domain/notification"
	"github.com/LoResuelvo/loresuelvo-api/internal/domain/provider"
	serviceproposal "github.com/LoResuelvo/loresuelvo-api/internal/domain/service_proposal"
	"github.com/LoResuelvo/loresuelvo-api/internal/domain/user"
	"github.com/stretchr/testify/assert"
)

var (
	invalidServiceScheduledOn = time.Now().Add(-time.Hour)
	validProvider             = &provider.Provider{BaseUser: user.RehydrateBaseUser(validProviderID, "", "", "", "", "", nil)}
	validConsumer             = &consumer.Consumer{BaseUser: user.RehydrateBaseUser(validConsumerID, "", "", "", "", "", nil)}
)

func TestInvalidTime(t *testing.T) {
	clock := new(MockClock)
	clock.
		On("Now").
		Return(time.Now())

	serviceProposal, err := serviceproposal.NewServiceProposal(
		validProvider, validConsumer, validConversation,
		invalidServiceScheduledOn, validServiceDescription, validBookingTerms(), clock)

	assert.Error(t, err)
	assert.Nil(t, serviceProposal)
}

func TestConversationMustBeAccepted(t *testing.T) {
	clock := new(MockClock)
	clock.
		On("Now").
		Return(time.Now())

	pendingConversation := &conversation.WorkConversation{BaseConversation: conversation.NewBaseConversation(conversation.TypeWork, conversation.StatusPending)}

	serviceProposal, err := serviceproposal.NewServiceProposal(
		validProvider, validConsumer, pendingConversation,
		validServiceScheduledOn, validServiceDescription, validBookingTerms(), clock)

	assert.Error(t, err)
	assert.Nil(t, serviceProposal)
}

func TestShouldCreateAsPending(t *testing.T) {
	clock := new(MockClock)
	clock.On("Now").Return(time.Now())

	serviceProposal, err := serviceproposal.NewServiceProposal(
		validProvider, validConsumer, validConversation,
		validServiceScheduledOn, validServiceDescription, validBookingTerms(), clock)

	assert.NoError(t, err)
	assert.NotNil(t, serviceProposal)
	assert.Equal(t, serviceproposal.StatusPending, serviceProposal.Status)
}

func TestServiceProposalReturnsCounterpartForConsumer(t *testing.T) {
	consumerUser := &consumer.Consumer{BaseUser: user.RehydrateBaseUser(0, "auth0|consumer", "", "", "", "", nil)}
	providerUser := &provider.Provider{BaseUser: user.RehydrateBaseUser(0, "auth0|provider", "", "", "", "", nil)}
	proposal := &serviceproposal.ServiceProposal{Consumer: consumerUser, Provider: providerUser}

	counterpart, err := proposal.CounterpartFor("auth0|consumer")

	assert.NoError(t, err)
	assert.Same(t, providerUser, counterpart)
}

func TestServiceProposalReturnsCounterpartForProvider(t *testing.T) {
	consumerUser := &consumer.Consumer{BaseUser: user.RehydrateBaseUser(0, "auth0|consumer", "", "", "", "", nil)}
	providerUser := &provider.Provider{BaseUser: user.RehydrateBaseUser(0, "auth0|provider", "", "", "", "", nil)}
	proposal := &serviceproposal.ServiceProposal{Consumer: consumerUser, Provider: providerUser}

	counterpart, err := proposal.CounterpartFor("auth0|provider")

	assert.NoError(t, err)
	assert.Same(t, consumerUser, counterpart)
}

func TestServiceProposalRejectsNonParticipantCounterpartAccess(t *testing.T) {
	proposal := &serviceproposal.ServiceProposal{
		Consumer: &consumer.Consumer{BaseUser: user.RehydrateBaseUser(0, "auth0|consumer", "", "", "", "", nil)},
		Provider: &provider.Provider{BaseUser: user.RehydrateBaseUser(0, "auth0|provider", "", "", "", "", nil)},
	}

	counterpart, err := proposal.CounterpartFor("auth0|other")

	assert.ErrorIs(t, err, serviceproposal.ErrOnlyParticipantCanView)
	assert.Nil(t, counterpart)
}

func TestShouldCreateANotification(t *testing.T) {
	clock := new(MockClock)
	frezzedTime := time.Now()
	clock.On("Now").Return(frezzedTime)
	expectedNotification := &notification.Notification{
		UserID:       validConsumer.ID(),
		Type:         notification.TypeServiceProposalReceived,
		ResourceType: notification.ResourceServiceProposal,
		ResourceID:   0,
		CreatedAt:    frezzedTime,
		ReadAt:       nil,
	}

	serviceProposal, _ := serviceproposal.NewServiceProposal(
		validProvider, validConsumer, validConversation,
		validServiceScheduledOn, validServiceDescription, validBookingTerms(), clock)

	notification := serviceProposal.CreateReceivedNotification(clock)

	assert.NotNil(t, notification)
	assert.Equal(t, expectedNotification, notification)
}
