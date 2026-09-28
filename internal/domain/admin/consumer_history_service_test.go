package admin_test

import (
	"errors"
	"github.com/LoResuelvo/loresuelvo-api/internal/domain/admin"
	readmodel "github.com/LoResuelvo/loresuelvo-api/internal/domain/admin/read_model"
	"github.com/LoResuelvo/loresuelvo-api/internal/domain/audit"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	"testing"
	"time"
)

func TestConsumerHistoryServiceAuditsPreparedAccessAfterSources(t *testing.T) {
	r := new(consumerHistoryReaderMock)
	p := new(profilePhotoURLResolverMock)
	o := new(diagnosticOperatorMock)
	w := new(diagnosticAuditMock)
	q := admin.ConsumerHistoryQuery{Limit: 20}
	now := time.Now().UTC()
	h := &readmodel.ConsumerHistory{Consumer: readmodel.Consumer{ID: 12, ProfilePhotoFileID: "photo"}}
	read := r.On("FindByConsumerID", t.Context(), 12, q).Return(h, nil).Once()
	photo := p.On("ResolvePublicURLs", t.Context(), []string{"photo"}).Return(map[string]string{"photo": "https://cdn.example/public"}, nil).Once().NotBefore(read)
	op := o.On("FindOperatorIDByAuthID", t.Context(), "support").Return(23, nil).Once().NotBefore(photo)
	w.On("Save", t.Context(), mock.MatchedBy(func(e *audit.Event) bool {
		return e.OperatorID() == 23 && e.ResourceType() == "consumer" && e.ResourceID() == "12" && e.Action() == audit.ActionAccess && e.Result() == audit.ResultPrepared && e.CorrelationID() == "request-history" && e.Reason() == nil && e.StateChange() == nil
	})).Return(nil).Once().NotBefore(op)
	got, err := admin.NewConsumerHistoryService(r, p, o, w, diagnosticClock{now}).Query(t.Context(), 12, q, "support", "request-history")
	require.NoError(t, err)
	require.Equal(t, "https://cdn.example/public", got.Consumer.ProfilePhotoURL)
	r.AssertExpectations(t)
	p.AssertExpectations(t)
	o.AssertExpectations(t)
	w.AssertExpectations(t)
}
func TestConsumerHistoryServiceFailsClosed(t *testing.T) {
	failure := errors.New("private source failure")
	for _, stage := range []string{"reader", "missing", "photo", "operator", "audit"} {
		t.Run(stage, func(t *testing.T) {
			r := new(consumerHistoryReaderMock)
			p := new(profilePhotoURLResolverMock)
			o := new(diagnosticOperatorMock)
			w := new(diagnosticAuditMock)
			q := admin.ConsumerHistoryQuery{Limit: 20}
			h := &readmodel.ConsumerHistory{Consumer: readmodel.Consumer{ID: 12, ProfilePhotoFileID: "photo"}}
			var re error
			if stage == "reader" {
				re = failure
			}
			if stage == "missing" {
				h = nil
			}
			r.On("FindByConsumerID", t.Context(), 12, q).Return(h, re).Once()
			if stage != "reader" && stage != "missing" {
				var pe error
				if stage == "photo" {
					pe = failure
				}
				p.On("ResolvePublicURLs", t.Context(), []string{"photo"}).Return(map[string]string{}, pe).Once()
			}
			if stage == "operator" || stage == "audit" {
				var oe error
				if stage == "operator" {
					oe = failure
				}
				o.On("FindOperatorIDByAuthID", t.Context(), "support").Return(23, oe).Once()
			}
			if stage == "audit" {
				w.On("Save", t.Context(), mock.Anything).Return(failure).Once()
			}
			got, err := admin.NewConsumerHistoryService(r, p, o, w, diagnosticClock{time.Now()}).Query(t.Context(), 12, q, "support", "request-history")
			require.Nil(t, got)
			if stage == "missing" {
				require.ErrorIs(t, err, admin.ErrConsumerHistoryNotFound)
			} else {
				require.ErrorIs(t, err, failure)
			}
			if stage != "audit" {
				w.AssertNotCalled(t, "Save", mock.Anything, mock.Anything)
			}
			r.AssertExpectations(t)
			p.AssertExpectations(t)
			o.AssertExpectations(t)
			w.AssertExpectations(t)
		})
	}
}
func TestConsumerHistoryServiceLegacyMissingPhotoDoesNotInventFailure(t *testing.T) {
	for _, file := range []string{"", "unresolved-photo"} {
		t.Run(file, func(t *testing.T) {
			r := new(consumerHistoryReaderMock)
			p := new(profilePhotoURLResolverMock)
			o := new(diagnosticOperatorMock)
			w := new(diagnosticAuditMock)
			q := admin.ConsumerHistoryQuery{Limit: 20}
			r.On("FindByConsumerID", t.Context(), 12, q).Return(&readmodel.ConsumerHistory{Consumer: readmodel.Consumer{ID: 12, ProfilePhotoFileID: file}}, nil).Once()
			if file != "" {
				p.On("ResolvePublicURLs", t.Context(), []string{file}).Return(map[string]string{}, nil).Once()
			}
			o.On("FindOperatorIDByAuthID", t.Context(), "support").Return(23, nil).Once()
			w.On("Save", t.Context(), mock.Anything).Return(nil).Once()
			got, err := admin.NewConsumerHistoryService(r, p, o, w, diagnosticClock{time.Now()}).Query(t.Context(), 12, q, "support", "request-history")
			require.NoError(t, err)
			require.Empty(t, got.Consumer.ProfilePhotoURL)
			p.AssertExpectations(t)
			w.AssertExpectations(t)
		})
	}
}
func TestConsumerHistoryQueryValidatesStatusesAndBoundsBeforeReading(t *testing.T) {
	for _, q := range []admin.ConsumerHistoryQuery{{Limit: 0}, {Limit: 101}, {Limit: 20, Type: "payment"}, {Limit: 20, Status: "pending"}, {Limit: 20, Type: "work_order", Status: "accepted"}, {Limit: 20, ProviderID: -1}, {Limit: 20, After: &readmodel.ConsumerHistoryPosition{ID: 1, Type: "job_request"}}} {
		r := new(consumerHistoryReaderMock)
		got, err := admin.NewConsumerHistoryService(r, nil, nil, nil, nil).Query(t.Context(), 12, q, "support", "request")
		require.Nil(t, got)
		require.ErrorIs(t, err, admin.ErrInvalidConsumerHistoryQuery)
		r.AssertNotCalled(t, "FindByConsumerID", mock.Anything, mock.Anything, mock.Anything)
	}
	for typ, statuses := range map[string][]string{"job_request": {"pending", "accepted"}, "service_proposal": {"pending", "accepted", "rejected"}, "work_order": {"scheduled", "awaiting_payment", "paid"}} {
		for _, status := range statuses {
			require.NoError(t, (admin.ConsumerHistoryQuery{Limit: 20, Type: typ, Status: status}).Validate())
		}
	}
}
