package workorder_test

import (
	"testing"
	"time"

	serviceproposal "github.com/LoResuelvo/loresuelvo-api/internal/domain/service_proposal"
	workorder "github.com/LoResuelvo/loresuelvo-api/internal/domain/work_order"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestWorkOrderCanBeMarkedAsFullyPaid(t *testing.T) {
	acceptedOn := time.Date(2026, time.July, 4, 13, 0, 0, 0, time.UTC)
	order := workOrderFixture(84, 10, 20, acceptedOn.Add(48*time.Hour))

	err := order.MarkPaid()

	require.NoError(t, err)
	assert.Equal(t, workorder.StatusPaid, order.Status)
}

func TestWorkOrderRejectsBeingMarkedPaidWhenItIsNotEligible(t *testing.T) {
	order := workOrderFixture(84, 10, 20, time.Now().UTC())
	order.ID = 0

	err := order.MarkPaid()

	assert.ErrorIs(t, err, workorder.ErrWorkOrderNotEligibleForFullPayment)
	assert.Equal(t, workorder.StatusScheduled, order.Status)
}

func TestNewWorkOrderStartsScheduled(t *testing.T) {
	acceptedOn := time.Date(2026, time.July, 4, 13, 0, 0, 0, time.UTC)

	proposal := serviceproposal.ServiceProposal{
		ID: 1,
	}

	order, err := workorder.New(&proposal, acceptedOn)

	assert.NoError(t, err)
	assert.Equal(t, &proposal, order.ServiceProposal)
	assert.Equal(t, workorder.StatusScheduled, order.Status)
	assert.Equal(t, acceptedOn, order.AcceptedOn)
}
