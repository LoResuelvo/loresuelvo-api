package serviceproposal_test

import (
	"testing"
	"time"

	"github.com/LoResuelvo/loresuelvo-api/internal/domain/notification"
	serviceproposal "github.com/LoResuelvo/loresuelvo-api/internal/domain/service_proposal"
	workorder "github.com/LoResuelvo/loresuelvo-api/internal/domain/work_order"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestServiceProposalAccept(t *testing.T) {
	now := time.Date(2026, time.July, 4, 13, 0, 0, 0, time.UTC)
	proposal := validSavedServiceProposal()
	proposal.ScheduledOn = now.Add(time.Hour)

	err := proposal.Accept(validConsumerID, now)

	require.NoError(t, err)
	assert.Equal(t, serviceproposal.StatusAccepted, proposal.Status)

	order, err := workorder.New(proposal, now)
	require.NoError(t, err)
	assert.Equal(t, proposal, order.ServiceProposal)
	assert.Equal(t, workorder.StatusScheduled, order.Status)
	assert.Equal(t, now, order.AcceptedOn)
}

func TestAcceptedServiceProposalCreatesProviderNotification(t *testing.T) {
	createdAt := time.Date(2026, time.July, 4, 13, 0, 0, 0, time.UTC)
	proposal := validSavedServiceProposal()
	clock := new(MockClock)
	clock.On("Now").Return(createdAt).Once()

	createdNotification := proposal.CreateAcceptedNotification(clock)

	assert.Equal(t, validProviderID, createdNotification.UserID)
	assert.Equal(t, notification.TypeServiceProposalAccepted, createdNotification.Type)
	assert.Equal(t, notification.ResourceServiceProposal, createdNotification.ResourceType)
	assert.Equal(t, proposal.ID, createdNotification.ResourceID)
	assert.Equal(t, createdAt, createdNotification.CreatedAt)
	clock.AssertExpectations(t)
}

func TestServiceProposalRejectsAcceptanceByAnotherConsumer(t *testing.T) {
	now := time.Date(2026, time.July, 4, 13, 0, 0, 0, time.UTC)
	proposal := validSavedServiceProposal()
	proposal.ScheduledOn = now.Add(time.Hour)

	err := proposal.Accept(validConsumerID+100, now)

	assert.ErrorIs(t, err, serviceproposal.ErrOnlyRecipientCanAccept)
	assert.Equal(t, serviceproposal.StatusPending, proposal.Status)
}

func TestServiceProposalRejectsAcceptanceWhenNotPending(t *testing.T) {
	now := time.Date(2026, time.July, 4, 13, 0, 0, 0, time.UTC)

	for _, status := range []serviceproposal.Status{
		serviceproposal.StatusAccepted,
		serviceproposal.StatusRejected,
	} {
		t.Run(string(status), func(t *testing.T) {
			proposal := validSavedServiceProposal()
			proposal.Status = status
			proposal.ScheduledOn = now.Add(time.Hour)

			err := proposal.Accept(validConsumerID, now)

			assert.ErrorIs(t, err, serviceproposal.ErrOnlyPendingCanBeAccepted)
			assert.Equal(t, status, proposal.Status)
		})
	}
}

func TestServiceProposalRejectsAcceptanceAtScheduledTime(t *testing.T) {
	scheduledOn := time.Date(2026, time.July, 5, 12, 30, 0, 0, time.UTC)
	proposal := validSavedServiceProposal()
	proposal.ScheduledOn = scheduledOn

	err := proposal.Accept(validConsumerID, scheduledOn)

	assert.ErrorIs(t, err, serviceproposal.ErrServiceProposalExpired)
	assert.Equal(t, serviceproposal.StatusPending, proposal.Status)
}
