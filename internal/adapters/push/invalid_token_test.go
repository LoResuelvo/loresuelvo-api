package push

import (
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

func TestSenderOnlyClassifiesConfirmedInvalidFCMTokens(t *testing.T) {
	for _, test := range []struct {
		name    string
		status  int
		body    string
		invalid bool
	}{
		{"unregistered", 404, `{"error":{"details":[{"@type":"type.googleapis.com/google.firebase.fcm.v1.FcmError","errorCode":"UNREGISTERED"}]}}`, true},
		{"invalid registration", 400, `{"error":{"details":[{"@type":"type.googleapis.com/google.firebase.fcm.v1.FcmError","errorCode":"INVALID_ARGUMENT"}]}}`, true},
		{"generic invalid argument", 400, `{"error":{"status":"INVALID_ARGUMENT","message":"private registration token"}}`, false},
		{"payload invalid", 400, `{"error":{"details":[{"@type":"type.googleapis.com/google.rpc.BadRequest","fieldViolations":[{"field":"message.data"}]}]}}`, false},
		{"conflicting details", 400, `{"error":{"details":[{"@type":"type.googleapis.com/google.firebase.fcm.v1.FcmError","errorCode":"INVALID_ARGUMENT"},{"@type":"type.googleapis.com/google.rpc.BadRequest"}]}}`, false},
		{"generic not found", 404, `{"error":{"status":"NOT_FOUND"}}`, false},
		{"sender mismatch", 403, `{"error":{"details":[{"@type":"type.googleapis.com/google.firebase.fcm.v1.FcmError","errorCode":"SENDER_ID_MISMATCH"}]}}`, false},
		{"transient", 503, `{"error":{"details":[{"@type":"type.googleapis.com/google.firebase.fcm.v1.FcmError","errorCode":"UNAVAILABLE"}]}}`, false},
		{"wrong HTTP status", 500, `{"error":{"details":[{"@type":"type.googleapis.com/google.firebase.fcm.v1.FcmError","errorCode":"UNREGISTERED"}]}}`, false},
		{"malformed response", 404, `{"error":`, false},
		{"wrong detail type", 404, `{"error":{"details":[{"@type":"unknown","errorCode":"UNREGISTERED"}]}}`, false},
	} {
		t.Run(test.name, func(t *testing.T) {
			transport := new(roundTripperMock)
			transport.On("RoundTrip", mock.Anything).Return(fcmResponse(test.status, test.body), nil).Once()
			err := senderWithTransport(transport).Send(t.Context(), "private-fcm-token", map[string]string{}, time.Hour)
			if test.invalid {
				require.ErrorIs(t, err, ErrInvalidToken)
			} else {
				require.Error(t, err)
				require.NotErrorIs(t, err, ErrInvalidToken)
			}
			require.NotContains(t, err.Error(), "private")
			transport.AssertExpectations(t)
		})
	}
}
func TestSenderDoesNotClassifyOversizedProviderResponse(t *testing.T) {
	transport := new(roundTripperMock)
	body := `{"error":{"details":[{"@type":"type.googleapis.com/google.firebase.fcm.v1.FcmError","errorCode":"UNREGISTERED"}]},"padding":"` + strings.Repeat("x", 65536) + `"}`
	transport.On("RoundTrip", mock.Anything).Return(fcmResponse(http.StatusNotFound, body), nil).Once()
	err := senderWithTransport(transport).Send(t.Context(), "token", nil, time.Hour)
	require.Error(t, err)
	require.NotErrorIs(t, err, ErrInvalidToken)
}
