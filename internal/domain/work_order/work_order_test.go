package workorder_test

import (
	"testing"
	"time"

	serviceproposal "github.com/LoResuelvo/loresuelvo-api/internal/domain/service_proposal"
	workorder "github.com/LoResuelvo/loresuelvo-api/internal/domain/work_order"
	"github.com/stretchr/testify/assert"
)

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
