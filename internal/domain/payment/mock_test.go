package payment

import (
	"context"
	"github.com/LoResuelvo/loresuelvo-api/internal/domain/consumer"
	"github.com/LoResuelvo/loresuelvo-api/internal/domain/notification"
	"github.com/LoResuelvo/loresuelvo-api/internal/domain/provider"
	serviceproposal "github.com/LoResuelvo/loresuelvo-api/internal/domain/service_proposal"
	"github.com/LoResuelvo/loresuelvo-api/internal/domain/user"
	workorder "github.com/LoResuelvo/loresuelvo-api/internal/domain/work_order"
	"github.com/stretchr/testify/require"
	"testing"
	"time"

	"github.com/LoResuelvo/loresuelvo-api/internal/domain/audit"
	rm "github.com/LoResuelvo/loresuelvo-api/internal/domain/payment/read_model"
	"github.com/stretchr/testify/mock"
)

type adminPaymentReaderMock struct{ mock.Mock }

func (m *adminPaymentReaderMock) FindPage(ctx context.Context, q AdminPaymentQuery) (*rm.AdminPaymentSnapshot, error) {
	args := m.Called(ctx, q)
	var result *rm.AdminPaymentSnapshot
	if args.Get(0) != nil {
		result = args.Get(0).(*rm.AdminPaymentSnapshot)
	}
	return result, args.Error(1)
}

type adminPaymentOperatorIDFinderMock struct{ mock.Mock }

func (m *adminPaymentOperatorIDFinderMock) FindOperatorIDByAuthID(ctx context.Context, id string) (int, error) {
	a := m.Called(ctx, id)
	return a.Int(0), a.Error(1)
}

type adminPaymentAuditWriterMock struct{ mock.Mock }

func (m *adminPaymentAuditWriterMock) Save(ctx context.Context, e *audit.Event) error {
	return m.Called(ctx, e).Error(0)
}

type adminPaymentClockMock struct{ mock.Mock }

func (m *adminPaymentClockMock) Now() time.Time { return m.Called().Get(0).(time.Time) }

type paymentWorkOrderFinderMock struct{ mock.Mock }

func (m *paymentWorkOrderFinderMock) FindByID(ctx context.Context, id int) (*workorder.WorkOrder, error) {
	a := m.Called(ctx, id)
	return a.Get(0).(*workorder.WorkOrder), a.Error(1)
}
func (m *paymentWorkOrderFinderMock) FindByServiceProposalID(ctx context.Context, id int) (*workorder.WorkOrder, error) {
	a := m.Called(ctx, id)
	return a.Get(0).(*workorder.WorkOrder), a.Error(1)
}

type paymentUnitOfWorkMock struct{ mock.Mock }

func (m *paymentUnitOfWorkMock) Execute(ctx context.Context, operation func(TransactionalStore) error) error {
	a := m.Called(ctx, operation)
	if result, ok := a.Get(0).(func() error); ok {
		return result()
	}
	return a.Error(0)
}

type paymentTransactionalStoreMock struct{ mock.Mock }

func (m *paymentTransactionalStoreMock) SaveIntent(ctx context.Context, v *Intent) error {
	return m.Called(ctx, v).Error(0)
}
func (m *paymentTransactionalStoreMock) SaveTransaction(ctx context.Context, v *Transaction) error {
	return m.Called(ctx, v).Error(0)
}
func (m *paymentTransactionalStoreMock) SaveServiceProposal(ctx context.Context, v *serviceproposal.ServiceProposal) error {
	return m.Called(ctx, v).Error(0)
}
func (m *paymentTransactionalStoreMock) SaveWorkOrder(ctx context.Context, v *workorder.WorkOrder) error {
	return m.Called(ctx, v).Error(0)
}
func (m *paymentTransactionalStoreMock) SaveNotification(ctx context.Context, v *notification.Notification) error {
	return m.Called(ctx, v).Error(0)
}

type paymentNotificatorMock struct{ mock.Mock }

func (m *paymentNotificatorMock) Notify(ctx context.Context, n *notification.Notification) error {
	return m.Called(ctx, n).Error(0)
}
func finalPaymentOrder(t *testing.T, now time.Time) *workorder.WorkOrder {
	t.Helper()
	p := &serviceproposal.ServiceProposal{ID: 42, ScheduledOn: now.Add(-time.Hour), Consumer: &consumer.Consumer{BaseUser: user.RehydrateBaseUser(10, "consumer", "ana@example.com", "Ana", "Gomez", "consumer", nil)}, Provider: &provider.Provider{BaseUser: user.RehydrateBaseUser(20, "provider", "juan@example.com", "Juan", "Perez", "provider", nil)}}
	order, err := workorder.New(p, now.Add(-48*time.Hour))
	require.NoError(t, err)
	order.SetID(84)
	report, err := workorder.NewCompletionReport("Completed work", []string{"image-one"}, now)
	require.NoError(t, err)
	require.NoError(t, order.ReportCompletion(20, report))
	return order
}
