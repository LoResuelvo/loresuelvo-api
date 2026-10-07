package push

import (
	"bytes"
	"context"
	"errors"
	"log/slog"
	"net/http"
	"testing"
	"time"

	"github.com/LoResuelvo/loresuelvo-api/internal/domain/installation"
	"github.com/LoResuelvo/loresuelvo-api/internal/domain/notification"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

func TestPublisherNoTargetsDoesNotContactFCM(t *testing.T) {
	now := time.Now()
	store := new(installationStoreMock)
	store.On("FindByUserID", mock.Anything, 10).Return([]installation.Installation{}, nil).Once()
	transport := new(roundTripperMock)
	publisher := NewPublisher(senderWithTransport(transport), store, nil, pushClock(now))
	require.NoError(t, publisher.Notify(t.Context(), &notification.Notification{ID: 1, UserID: 10, Type: notification.TypeServiceProposalReceived, CreatedAt: now}))
	transport.AssertNotCalled(t, "RoundTrip", mock.Anything)
	store.AssertExpectations(t)
}
func TestPublisherSkipsUnsupportedAndNilNotice(t *testing.T) {
	store := new(installationStoreMock)
	publisher := NewPublisher(&Sender{}, store, nil, nil)
	require.NoError(t, publisher.Notify(t.Context(), nil))
	require.NoError(t, publisher.Notify(t.Context(), &notification.Notification{Type: notification.TypeCalendarReauthorizationRequired}))
	store.AssertNotCalled(t, "FindByUserID", mock.Anything, mock.Anything)
}
func TestPublisherSkipsExpiredReminder(t *testing.T) {
	now := time.Now()
	orders := new(orderFinderMock)
	orders.On("FindByID", mock.Anything, 20).Return(scheduledOrder(t, 20, now.Add(-time.Minute)), nil).Once()
	store := new(installationStoreMock)
	publisher := NewPublisher(&Sender{}, store, orders, pushClock(now))
	require.NoError(t, publisher.Notify(t.Context(), &notification.Notification{ID: 1, UserID: 10, Type: notification.TypeWorkOrderCloseToScheduledTime, ResourceID: 20, CreatedAt: now.Add(-time.Hour)}))
	store.AssertNotCalled(t, "FindByUserID", mock.Anything, mock.Anything)
	orders.AssertExpectations(t)
}
func TestPublisherDisablesConfirmedInvalidSnapshotWithoutFailingNotice(t *testing.T) {
	now := time.Now()
	snapshot := installation.Installation{ID: "phone", UserID: 10, Token: "old-token", BindingID: "login", Enabled: true, Revision: 7}
	for _, saveErr := range []error{nil, installation.ErrConflict} {
		t.Run(map[bool]string{true: "saved", false: "concurrent renewal preserved"}[saveErr == nil], func(t *testing.T) {
			store := new(installationStoreMock)
			store.On("FindByUserID", mock.Anything, 10).Return([]installation.Installation{snapshot}, nil).Once()
			store.On("Save", mock.Anything, mock.MatchedBy(func(i *installation.Installation) bool {
				return !i.Enabled && !i.Revoked && i.Token == "old-token" && i.BindingID == "login" && i.Revision == 7
			})).Return(saveErr).Once()
			transport := new(roundTripperMock)
			transport.On("RoundTrip", mock.Anything).Return(fcmResponse(404, `{"error":{"details":[{"@type":"type.googleapis.com/google.firebase.fcm.v1.FcmError","errorCode":"UNREGISTERED"}]}}`), nil).Once()
			publisher := NewPublisher(senderWithTransport(transport), store, nil, pushClock(now))
			require.NoError(t, publisher.Notify(t.Context(), &notification.Notification{ID: 1, UserID: 10, Type: notification.TypeServiceProposalReceived, CreatedAt: now}))
			store.AssertExpectations(t)
		})
	}
}
func TestPublisherKeepsTargetsForUnconfirmedErrors(t *testing.T) {
	now := time.Now()
	for _, status := range []int{400, 401, 403, 404, 429, 503} {
		t.Run(http.StatusText(status), func(t *testing.T) {
			store := new(installationStoreMock)
			store.On("FindByUserID", mock.Anything, 10).Return([]installation.Installation{{ID: "phone", UserID: 10, Token: "token", Enabled: true}}, nil).Once()
			transport := new(roundTripperMock)
			transport.On("RoundTrip", mock.Anything).Return(fcmResponse(status, `{"error":{"message":"private provider detail"}}`), nil).Once()
			publisher := NewPublisher(senderWithTransport(transport), store, nil, pushClock(now))
			require.NoError(t, publisher.Notify(t.Context(), &notification.Notification{ID: 1, UserID: 10, Type: notification.TypeServiceProposalReceived, CreatedAt: now}))
			store.AssertNotCalled(t, "Save", mock.Anything, mock.Anything)
		})
	}
}
func TestPublisherBoundsEntireMultiDeviceBatch(t *testing.T) {
	now := time.Now()
	store := new(installationStoreMock)
	store.On("FindByUserID", mock.Anything, 10).Return([]installation.Installation{{Token: "first"}, {Token: "second"}, {Token: "third"}}, nil).Once()
	transport := new(roundTripperMock)
	transport.On("RoundTrip", mock.Anything).Run(func(a mock.Arguments) {
		request := a.Get(0).(*http.Request)
		deadline, ok := request.Context().Deadline()
		require.True(t, ok)
		require.LessOrEqual(t, time.Until(deadline), 50*time.Millisecond)
		<-request.Context().Done()
	}).Return(nil, context.DeadlineExceeded).Once()
	publisher := NewPublisher(senderWithTransport(transport), store, nil, pushClock(now))
	start := time.Now()
	require.NoError(t, publisher.Notify(t.Context(), &notification.Notification{ID: 1, UserID: 10, Type: notification.TypeServiceProposalReceived, CreatedAt: now}))
	require.Less(t, time.Since(start), 300*time.Millisecond)
	transport.AssertExpectations(t)
	store.AssertNotCalled(t, "Save", mock.Anything, mock.Anything)
}
func TestPublisherLookupFailureDoesNotFailCommittedNotice(t *testing.T) {
	store := new(installationStoreMock)
	store.On("FindByUserID", mock.Anything, 10).Return([]installation.Installation(nil), errors.New("private database error")).Once()
	publisher := NewPublisher(&Sender{}, store, nil, pushClock(time.Now()))
	require.NoError(t, publisher.Notify(t.Context(), &notification.Notification{ID: 1, UserID: 10, Type: notification.TypeServiceProposalReceived, CreatedAt: time.Now()}))
}

func TestPublisherDiagnosticsOmitCredentialsBindingAndPayload(t *testing.T) {
	var logs bytes.Buffer
	previous := slog.Default()
	slog.SetDefault(slog.New(slog.NewJSONHandler(&logs, nil)))
	t.Cleanup(func() { slog.SetDefault(previous) })
	now := time.Now()
	store := new(installationStoreMock)
	store.On("FindByUserID", mock.Anything, 10).Return([]installation.Installation{{ID: "private-installation", Token: "private-fcm-token", BindingID: "private-login-binding", Locale: "es", Enabled: true}}, nil).Once()
	transport := new(roundTripperMock)
	transport.On("RoundTrip", mock.Anything).Return(fcmResponse(503, `{"error":{"message":"private-provider-detail"}}`), nil).Once()
	publisher := NewPublisher(senderWithTransport(transport), store, nil, pushClock(now))
	require.NoError(t, publisher.Notify(t.Context(), &notification.Notification{ID: 1, UserID: 10, Type: notification.TypeServiceProposalReceived, CreatedAt: now}))
	require.Contains(t, logs.String(), "fcm_send")
	for _, private := range []string{"private-installation", "private-fcm-token", "private-login-binding", "private-provider-detail", proposalBodyES, "fcm.test"} {
		require.NotContains(t, logs.String(), private)
	}
}
