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

	order, err := proposal.Accept(validConsumerID, now)

	require.NoError(t, err)
	assert.Equal(t, serviceproposal.StatusAccepted, proposal.Status)
	assert.Equal(t, proposal.ID, order.ServiceProposalID)
	assert.Equal(t, validConsumerID, order.ConsumerID)
	assert.Equal(t, validProviderID, order.ProviderID)
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
