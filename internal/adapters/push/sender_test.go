package push

import (
	"encoding/json"
	"github.com/stretchr/testify/require"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

func TestSenderUsesDataOnlyHighPriorityWithBoundedTTL(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var body struct {
			Message struct {
				Token        string            `json:"token"`
				Data         map[string]string `json:"data"`
				Android      map[string]string `json:"android"`
				Notification json.RawMessage   `json:"notification"`
			} `json:"message"`
		}
		require.NoError(t, json.NewDecoder(r.Body).Decode(&body))
		require.Equal(t, "registered-token", body.Message.Token)
		require.Equal(t, "event-1", body.Message.Data["event_id"])
		require.Empty(t, body.Message.Notification)
		require.Equal(t, map[string]string{"priority": "high", "ttl": "3600s"}, body.Message.Android)
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()
	sender := &Sender{Client: server.Client(), URL: server.URL, Timeout: time.Second}
	require.NoError(t, sender.Send(t.Context(), "registered-token", map[string]string{"event_id": "event-1"}, time.Hour))
}
func TestSenderDoesNotExposeProviderErrorPayload(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, "private registration token and message", http.StatusBadRequest)
	}))
	defer server.Close()
	sender := &Sender{Client: server.Client(), URL: server.URL, Timeout: time.Second}
	err := sender.Send(t.Context(), "secret-token", map[string]string{}, time.Hour)
	require.EqualError(t, err, "FCM rejected request: status 400")
}
func TestDisabledSenderDoesNotRequireCredentials(t *testing.T) {
	t.Setenv("FCM_ENABLED", "false")
	sender, err := NewSenderFromEnv()
	require.NoError(t, err)
	require.Nil(t, sender)
}
func TestSenderRejectsUnboundedTimeout(t *testing.T) {
	t.Setenv("FCM_ENABLED", "true")
	t.Setenv("FCM_PROJECT_ID", "test-project")
	t.Setenv("FCM_TIMEOUT", "31s")
	_, err := NewSenderFromEnv()
	require.EqualError(t, err, "FCM_TIMEOUT must be between 1s and 30s")
}
