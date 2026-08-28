package didit

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/LoResuelvo/loresuelvo-api/internal/domain/identityverification"
	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
)

func TestClientCreatesHostedSession(t *testing.T) {
	workflowID, sessionID := uuid.New(), uuid.New()
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		require.Equal(t, http.MethodPost, request.Method)
		require.Equal(t, "/v3/session/", request.URL.Path)
		require.Equal(t, "api-secret", request.Header.Get("x-api-key"))
		var body createSessionRequest
		require.NoError(t, json.NewDecoder(request.Body).Decode(&body))
		require.Equal(t, workflowID, body.WorkflowID)
		require.Equal(t, "42", body.VendorData)
		require.Equal(t, expectedDetails{FirstName: "Juan", LastName: "Gomez"}, body.ExpectedDetails)
		writer.Header().Set("Content-Type", "application/json")
		require.NoError(t, json.NewEncoder(writer).Encode(createSessionResponse{SessionID: sessionID, SessionToken: "temporary-secret", URL: "https://verify.didit.test/session", Status: "Not Started", WorkflowID: workflowID, WorkflowVersion: 3}))
	}))
	defer server.Close()
	client, err := NewClient(Config{APIKey: "api-secret", WorkflowID: workflowID, BaseURL: server.URL, Timeout: time.Second})
	require.NoError(t, err)

	credentials, err := client.CreateSession(context.Background(), identityverification.SessionRequest{VendorData: "42", FirstName: "Juan", LastName: "Gomez"})

	require.NoError(t, err)
	require.Equal(t, sessionID, credentials.SessionID)
	require.Equal(t, identityverification.StatusNotStarted, credentials.Status)
	require.Equal(t, "temporary-secret", credentials.SessionToken)
}
