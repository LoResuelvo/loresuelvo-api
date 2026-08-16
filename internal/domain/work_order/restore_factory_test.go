package workorder_test

import (
	"testing"
	"time"

	serviceproposal "github.com/LoResuelvo/loresuelvo-api/internal/domain/service_proposal"
	workorder "github.com/LoResuelvo/loresuelvo-api/internal/domain/work_order"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestRestoreFactoryRestoresScheduledWorkOrder(t *testing.T) {
	acceptedOn := time.Date(2026, time.July, 4, 13, 0, 0, 0, time.UTC)
	proposal := &serviceproposal.ServiceProposal{ID: 42}

	order, err := (workorder.RestoreFactory{}).Restore(84, proposal, workorder.StatusScheduled, acceptedOn)

	require.NoError(t, err)
	assert.Equal(t, 84, order.ID())
	assert.Equal(t, proposal, order.ServiceProposal())
	assert.Equal(t, workorder.StatusScheduled, order.Status())
	assert.Equal(t, acceptedOn, order.AcceptedOn())
}

func TestRestoreFactoryRestoresPaidWorkOrder(t *testing.T) {
	order, err := (workorder.RestoreFactory{}).Restore(
		84,
		&serviceproposal.ServiceProposal{ID: 42},
		workorder.StatusPaid,
		time.Now().UTC(),
	)

	require.NoError(t, err)
	assert.Equal(t, workorder.StatusPaid, order.Status())
}

func TestRestoreFactoryRejectsInvalidIdentity(t *testing.T) {
	_, err := (workorder.RestoreFactory{}).Restore(0, &serviceproposal.ServiceProposal{ID: 42}, workorder.StatusScheduled, time.Now().UTC())

	assert.ErrorIs(t, err, workorder.ErrInvalidWorkOrderIdentity)
}

func TestRestoreFactoryRejectsMissingProposal(t *testing.T) {
	_, err := (workorder.RestoreFactory{}).Restore(84, nil, workorder.StatusScheduled, time.Now().UTC())

	assert.ErrorIs(t, err, workorder.ErrInvalidWorkOrderIdentity)
}

func TestRestoreFactoryRejectsUnknownState(t *testing.T) {
	_, err := (workorder.RestoreFactory{}).Restore(84, &serviceproposal.ServiceProposal{ID: 42}, workorder.Status("unknown"), time.Now().UTC())

	assert.ErrorIs(t, err, workorder.ErrInvalidWorkOrderState)
}
