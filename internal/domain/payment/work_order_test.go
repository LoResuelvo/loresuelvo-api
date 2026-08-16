package payment_test

import (
	"testing"
	"time"

	serviceproposal "github.com/LoResuelvo/loresuelvo-api/internal/domain/service_proposal"
	workorder "github.com/LoResuelvo/loresuelvo-api/internal/domain/work_order"
	"github.com/stretchr/testify/require"
)

func newWorkOrderFixture(
	t *testing.T,
	id int,
	proposal *serviceproposal.ServiceProposal,
	acceptedOn time.Time,
) *workorder.WorkOrder {
	t.Helper()
	order, err := workorder.New(proposal, acceptedOn)
	require.NoError(t, err)
	order.SetID(id)
	return order
}
