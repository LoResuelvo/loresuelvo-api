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
	invalidServiceAmount      int64 = -100
	invalidServiceScheduledOn       = time.Now().Add(-time.Hour)
	validProvider                   = &provider.Provider{BaseUser: &user.BaseUser{ID: validProviderID}}
	validConsumer                   = &consumer.Consumer{BaseUser: &user.BaseUser{ID: validConsumerID}}
)

func TestInvalidAmount(t *testing.T) {
	clock := new(MockClock)
	serviceProposal, err := serviceproposal.NewServiceProposal(
		validProvider, validConsumer, validConversation, invalidServiceAmount,
		validServiceScheduledOn, validServiceDescription, clock)

	assert.Error(t, err)
	assert.Nil(t, serviceProposal)
}

func TestInvalidTime(t *testing.T) {
	clock := new(MockClock)
	clock.
		On("Now").
		Return(time.Now())

	serviceProposal, err := serviceproposal.NewServiceProposal(
		validProvider, validConsumer, validConversation, validServiceAmount,
		invalidServiceScheduledOn, validServiceDescription, clock)

	assert.Error(t, err)
	assert.Nil(t, serviceProposal)
}

func TestConversationMustBeAccepted(t *testing.T) {
	clock := new(MockClock)
	clock.
		On("Now").
		Return(time.Now())

	pendingConversation := &conversation.WorkConversation{BaseConversation: &conversation.BaseConversation{Status: conversation.StatusPending}}

	serviceProposal, err := serviceproposal.NewServiceProposal(
		validProvider, validConsumer, pendingConversation, validServiceAmount,
		validServiceScheduledOn, validServiceDescription, clock)

	assert.Error(t, err)
	assert.Nil(t, serviceProposal)
}

func TestShouldCreateAsPending(t *testing.T) {
	clock := new(MockClock)
	clock.On("Now").Return(time.Now())

	serviceProposal, err := serviceproposal.NewServiceProposal(
		validProvider, validConsumer, validConversation, validServiceAmount,
		validServiceScheduledOn, validServiceDescription, clock)

	assert.NoError(t, err)
	assert.NotNil(t, serviceProposal)
	assert.Equal(t, serviceproposal.StatusPending, serviceProposal.Status)
}

func TestShouldCreateANotification(t *testing.T) {
	clock := new(MockClock)
	frezzedTime := time.Now()
	clock.On("Now").Return(frezzedTime)
	expectedNotification := &notification.Notification{
		UserID:       validConsumer.ID,
		Type:         notification.TypeServiceProposalReceived,
		ResourceType: notification.ResourceServiceProposal,
		ResourceID:   0,
		CreatedAt:    frezzedTime,
		ReadAt:       nil,
	}

	serviceProposal, _ := serviceproposal.NewServiceProposal(
		validProvider, validConsumer, validConversation, validServiceAmount,
		validServiceScheduledOn, validServiceDescription, clock)

	notification := serviceProposal.CreateReceivedNotification(clock)

	assert.NotNil(t, notification)
	assert.Equal(t, expectedNotification, notification)
}
