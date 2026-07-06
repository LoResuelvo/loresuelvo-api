package workorder_test

import (
	"testing"
	"time"

	workorder "github.com/LoResuelvo/loresuelvo-api/internal/domain/work_order"
	"github.com/stretchr/testify/assert"
)

func TestNewWorkOrderStartsScheduled(t *testing.T) {
	acceptedOn := time.Date(2026, time.July, 4, 13, 0, 0, 0, time.UTC)

	order := workorder.New(10, 20, 30, acceptedOn)

	assert.Equal(t, 10, order.ServiceProposalID)
	assert.Equal(t, 20, order.ConsumerID)
	assert.Equal(t, 30, order.ProviderID)
	assert.Equal(t, workorder.StatusScheduled, order.Status)
	assert.Equal(t, acceptedOn, order.AcceptedOn)
}
