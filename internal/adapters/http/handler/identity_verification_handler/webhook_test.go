package identity_verification_handler

import (
	"bytes"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/LoResuelvo/loresuelvo-api/internal/domain/identityverification"
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

func TestProcessWebhookAppliesAuthenticatedResult(t *testing.T) {
	gin.SetMode(gin.TestMode)
	body := []byte(`{"event_id":"event"}`)
	now := time.Date(2026, 9, 1, 12, 0, 0, 0, time.UTC)
	result := identityverification.VerificationResult{
		SessionID: uuid.New(), ProviderID: 42, VendorData: "42", WorkflowID: uuid.New(),
		WorkflowVersion: 2, Status: identityverification.StatusInProgress, OccurredOn: now,
	}
	service := new(identityVerificationServiceMock)
	webhook := new(identityVerificationWebhookMock)
	webhook.On("Authenticate", body, "signature", "1788264000", now).Return(nil).Once()
	webhook.On("Translate", body).Return(result, nil).Once()
	service.On("ApplyResult", mock.Anything, result).Return(nil).Once()
	handler := NewIdentityVerificationHandlerWithWebhook(service, webhook, identityVerificationClockStub{now: now})

	recorder := executeWebhookRequest(handler, body, map[string]string{
		"X-Signature-V2": "signature",
		"X-Timestamp":    "1788264000",
	})

	require.Equal(t, http.StatusNoContent, recorder.Code)
	require.Empty(t, recorder.Body.Bytes())
	service.AssertExpectations(t)
	webhook.AssertExpectations(t)
}

func TestProcessWebhookRejectsInvalidSignatureBeforeTranslation(t *testing.T) {
	gin.SetMode(gin.TestMode)
	body := []byte(`{"event_id":"event"}`)
	now := time.Date(2026, 9, 1, 12, 0, 0, 0, time.UTC)
	service := new(identityVerificationServiceMock)
	webhook := new(identityVerificationWebhookMock)
	webhook.On("Authenticate", body, "invalid", "1788264000", now).Return(identityverification.ErrInvalidVerification).Once()
	handler := NewIdentityVerificationHandlerWithWebhook(service, webhook, identityVerificationClockStub{now: now})

	recorder := executeWebhookRequest(handler, body, map[string]string{
		"X-Signature-V2": "invalid",
		"X-Timestamp":    "1788264000",
	})

	require.Equal(t, http.StatusUnauthorized, recorder.Code)
	var response map[string]string
	require.NoError(t, json.Unmarshal(recorder.Body.Bytes(), &response))
	require.Equal(t, "invalid identity verification webhook signature", response["error"])
	webhook.AssertExpectations(t)
	service.AssertNotCalled(t, "ApplyResult", mock.Anything, mock.Anything)
}

func TestProcessWebhookReturnsServiceUnavailableWhenSessionIsNotLocal(t *testing.T) {
	gin.SetMode(gin.TestMode)
	body := []byte(`{"event_id":"event"}`)
	now := time.Date(2026, 9, 1, 12, 0, 0, 0, time.UTC)
	result := identityverification.VerificationResult{SessionID: uuid.New(), ProviderID: 42, WorkflowID: uuid.New(), Status: identityverification.StatusInProgress, OccurredOn: now}
	service := new(identityVerificationServiceMock)
	webhook := new(identityVerificationWebhookMock)
	webhook.On("Authenticate", body, "signature", "1788264000", now).Return(nil).Once()
	webhook.On("Translate", body).Return(result, nil).Once()
	service.On("ApplyResult", mock.Anything, result).Return(identityverification.ErrSessionNotFound).Once()
	handler := NewIdentityVerificationHandlerWithWebhook(service, webhook, identityVerificationClockStub{now: now})

	recorder := executeWebhookRequest(handler, body, map[string]string{
		"X-Signature-V2": "signature",
		"X-Timestamp":    "1788264000",
	})

	require.Equal(t, http.StatusServiceUnavailable, recorder.Code)
	service.AssertExpectations(t)
	webhook.AssertExpectations(t)
}

func TestProcessWebhookRejectsInvalidTranslatedPayload(t *testing.T) {
	gin.SetMode(gin.TestMode)
	body := []byte(`{"event_id":"event"}`)
	now := time.Date(2026, 9, 1, 12, 0, 0, 0, time.UTC)
	service := new(identityVerificationServiceMock)
	webhook := new(identityVerificationWebhookMock)
	webhook.On("Authenticate", body, "signature", "1788264000", now).Return(nil).Once()
	webhook.On("Translate", body).Return(identityverification.VerificationResult{}, errors.New("invalid payload")).Once()
	handler := NewIdentityVerificationHandlerWithWebhook(service, webhook, identityVerificationClockStub{now: now})

	recorder := executeWebhookRequest(handler, body, map[string]string{
		"X-Signature-V2": "signature",
		"X-Timestamp":    "1788264000",
	})

	require.Equal(t, http.StatusBadRequest, recorder.Code)
	webhook.AssertExpectations(t)
	service.AssertNotCalled(t, "ApplyResult", mock.Anything, mock.Anything)
}

func executeWebhookRequest(handler *IdentityVerificationHandler, body []byte, headers map[string]string) *httptest.ResponseRecorder {
	router := gin.New()
	router.POST("/webhooks/didit", handler.ProcessWebhook)
	request := httptest.NewRequest(http.MethodPost, "/webhooks/didit", bytes.NewReader(body))
	for key, value := range headers {
		request.Header.Set(key, value)
	}
	recorder := httptest.NewRecorder()
	router.ServeHTTP(recorder, request)
	return recorder
}
