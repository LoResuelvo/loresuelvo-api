package push

import (
	"context"
	"io"
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/LoResuelvo/loresuelvo-api/internal/domain/installation"
	serviceproposal "github.com/LoResuelvo/loresuelvo-api/internal/domain/service_proposal"
	workorder "github.com/LoResuelvo/loresuelvo-api/internal/domain/work_order"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

type installationStoreMock struct{ mock.Mock }

func (m *installationStoreMock) FindByUserID(ctx context.Context, id int) ([]installation.Installation, error) {
	a := m.Called(ctx, id)
	return a.Get(0).([]installation.Installation), a.Error(1)
}
func (m *installationStoreMock) Save(ctx context.Context, i *installation.Installation) error {
	return m.Called(ctx, i).Error(0)
}

type orderFinderMock struct{ mock.Mock }

func (m *orderFinderMock) FindByID(ctx context.Context, id int) (*workorder.WorkOrder, error) {
	a := m.Called(ctx, id)
	var order *workorder.WorkOrder
	if a.Get(0) != nil {
		order = a.Get(0).(*workorder.WorkOrder)
	}
	return order, a.Error(1)
}
func (m *orderFinderMock) FindByServiceProposalID(ctx context.Context, id int) (*workorder.WorkOrder, error) {
	a := m.Called(ctx, id)
	var order *workorder.WorkOrder
	if a.Get(0) != nil {
		order = a.Get(0).(*workorder.WorkOrder)
	}
	return order, a.Error(1)
}

type clockMock struct{ mock.Mock }

func (m *clockMock) Now() time.Time { return m.Called().Get(0).(time.Time) }

type roundTripperMock struct{ mock.Mock }

func (m *roundTripperMock) RoundTrip(r *http.Request) (*http.Response, error) {
	a := m.Called(r)
	var response *http.Response
	if a.Get(0) != nil {
		response = a.Get(0).(*http.Response)
	}
	return response, a.Error(1)
}
func fcmResponse(status int, body string) *http.Response {
	return &http.Response{StatusCode: status, Body: io.NopCloser(strings.NewReader(body)), Header: make(http.Header)}
}
func senderWithTransport(transport http.RoundTripper) *Sender {
	return &Sender{Client: &http.Client{Transport: transport}, URL: "https://fcm.test/messages:send", Timeout: 50 * time.Millisecond}
}
func pushClock(now time.Time) *clockMock {
	clock := new(clockMock)
	clock.On("Now").Return(now)
	return clock
}
func scheduledOrder(t *testing.T, id int, scheduled time.Time) *workorder.WorkOrder {
	t.Helper()
	order, err := workorder.New(&serviceproposal.ServiceProposal{ID: 2, ScheduledOn: scheduled}, scheduled.Add(-48*time.Hour))
	require.NoError(t, err)
	order.SetID(id)
	return order
}
