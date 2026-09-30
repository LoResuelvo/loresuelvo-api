package payment

import (
	"errors"
	"testing"
	"time"

	"github.com/LoResuelvo/loresuelvo-api/internal/domain/audit"
	rm "github.com/LoResuelvo/loresuelvo-api/internal/domain/payment/read_model"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

func TestAdminPaymentServiceFailsClosedAndAuditsCollectionOnce(t *testing.T) {
	for _, failure := range []string{"", "read", "audit", "invalid", "operator", "nil_snapshot"} {
		t.Run(failure, func(t *testing.T) {
			r := &adminPaymentReaderMock{}
			operator := &adminPaymentOperatorIDFinderMock{}
			w := &adminPaymentAuditWriterMock{}
			clock := &adminPaymentClockMock{}
			q := AdminPaymentQuery{Limit: 20}
			if failure == "invalid" {
				q.Limit = 0
			} else {
				var snapshot *rm.AdminPaymentSnapshot = &rm.AdminPaymentSnapshot{}
				var readErr error
				if failure == "read" {
					readErr = errors.New("read failure")
				}
				if failure == "nil_snapshot" {
					snapshot = nil
				}
				readCall := r.On("FindPage", mock.Anything, q).Return(snapshot, readErr).Once()
				if failure != "read" && failure != "nil_snapshot" {
					var operatorErr error
					if failure == "operator" {
						operatorErr = errors.New("operator failure")
					}
					operatorCall := operator.On("FindOperatorIDByAuthID", mock.Anything, "operator").Return(1, operatorErr).Once().NotBefore(readCall)
					if failure != "operator" {
						clock.On("Now").Return(time.Date(2026, 9, 20, 0, 0, 0, 0, time.UTC)).Once()
						var auditErr error
						if failure == "audit" {
							auditErr = errors.New("audit failure")
						}
						w.On("Save", mock.Anything, mock.MatchedBy(func(e *audit.Event) bool {
							return e.ResourceType() == "payment" && e.ResourceID() == "" && e.Reason() == nil && e.Result() == audit.ResultPrepared && e.CorrelationID() == "payments-test"
						})).Return(auditErr).Once().NotBefore(operatorCall)
					}
				}
			}
			s := NewAdminPaymentService(r, operator, w, clock)
			page, err := s.Query(t.Context(), q, "operator", "payments-test")
			if failure != "" {
				require.Error(t, err)
				require.Nil(t, page)
			} else {
				require.NoError(t, err)
				require.NotNil(t, page.Payments)
				require.Empty(t, page.Payments)
			}
			r.AssertExpectations(t)
			operator.AssertExpectations(t)
			clock.AssertExpectations(t)
			w.AssertExpectations(t)
			if failure == "invalid" || failure == "read" || failure == "operator" || failure == "nil_snapshot" {
				w.AssertNotCalled(t, "Save", mock.Anything, mock.Anything)
			}
		})
	}
}

func TestAdminPaymentServiceProjectsWholeProposalBeforeAuditing(t *testing.T) {
	r := &adminPaymentReaderMock{}
	operator := &adminPaymentOperatorIDFinderMock{}
	w := &adminPaymentAuditWriterMock{}
	clock := &adminPaymentClockMock{}
	p := adminPolicyProposal()
	balance := rm.AdminPayment{ID: "balance", ServiceProposalID: 1, Purpose: "service_balance", IntentStatus: "checkout_ready", Currency: "ARS", SellerAmountCents: 2555556, PlatformFeeCents: 432098, TotalAmountCents: 2987654, CreatedOn: time.Now()}
	p.Intents = append(p.Intents, balance)
	q := AdminPaymentQuery{Limit: 1, Purpose: PurposeServiceBalance}
	r.On("FindPage", mock.Anything, q).Return(&rm.AdminPaymentSnapshot{Payments: []rm.AdminPayment{balance}, Proposals: []rm.AdminPaymentProposal{p}}, nil).Once()
	operator.On("FindOperatorIDByAuthID", mock.Anything, "operator").Return(1, nil).Once()
	clock.On("Now").Return(time.Now()).Once()
	w.On("Save", mock.Anything, mock.Anything).Return(nil).Once()
	page, err := NewAdminPaymentService(r, operator, w, clock).Query(t.Context(), q, "operator", "test-full-summary")
	require.NoError(t, err)
	require.Len(t, page.Payments, 1)
	require.Equal(t, int64(901234), page.Payments[0].Summary.ApprovedAmounts[0].AmountCents)
	require.Equal(t, int64(2987654), page.Payments[0].Summary.PendingAmount.AmountCents)
	require.Empty(t, page.Payments[0].Transactions)
	r.AssertExpectations(t)
	operator.AssertExpectations(t)
	w.AssertExpectations(t)
	clock.AssertExpectations(t)
}
func TestAdminPaymentServiceOverflowDoesNotAudit(t *testing.T) {
	r := &adminPaymentReaderMock{}
	operator := &adminPaymentOperatorIDFinderMock{}
	w := &adminPaymentAuditWriterMock{}
	clock := &adminPaymentClockMock{}
	p := adminPolicyProposal()
	p.Breakdown.ServiceTotalCents = 9223372036854775807
	q := AdminPaymentQuery{Limit: 20}
	r.On("FindPage", mock.Anything, q).Return(&rm.AdminPaymentSnapshot{Proposals: []rm.AdminPaymentProposal{p}}, nil).Once()
	page, err := NewAdminPaymentService(r, operator, w, clock).Query(t.Context(), q, "operator", "test-overflow")
	require.ErrorIs(t, err, ErrAdminPaymentAmountOverflow)
	require.Nil(t, page)
	r.AssertExpectations(t)
	operator.AssertNotCalled(t, "FindOperatorIDByAuthID", mock.Anything, mock.Anything)
	w.AssertNotCalled(t, "Save", mock.Anything, mock.Anything)
	clock.AssertExpectations(t)
}
