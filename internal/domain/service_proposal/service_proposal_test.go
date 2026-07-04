package serviceproposal_test

import (
	"testing"
	"time"

	"github.com/LoResuelvo/loresuelvo-api/internal/domain/consumer"
	"github.com/LoResuelvo/loresuelvo-api/internal/domain/conversation"
	"github.com/LoResuelvo/loresuelvo-api/internal/domain/provider"
	serviceproposal "github.com/LoResuelvo/loresuelvo-api/internal/domain/service_proposal"
	"github.com/stretchr/testify/assert"
)

var (
	invalidServiceAmount      int64 = -100
	invalidServiceScheduledOn       = time.Now().Add(-time.Hour)
	validProvider                   = &provider.Provider{}
	validConsumer                   = &consumer.Consumer{}
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
