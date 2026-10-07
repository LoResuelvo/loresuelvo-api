package steps_test

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"
)

const testDiditWebhookSecret = "test-didit-webhook-secret"

type identityVerificationWebhookSignerStub struct {
	secret string
}

func newIdentityVerificationWebhookSignerStub() identityVerificationWebhookSignerStub {
	return identityVerificationWebhookSignerStub{secret: testDiditWebhookSecret}
}

func (stub identityVerificationWebhookSignerStub) Sign(body []byte) string {
	mac := hmac.New(sha256.New, []byte(stub.secret))
	if _, err := mac.Write(body); err != nil {
		panic(err)
	}
	return hex.EncodeToString(mac.Sum(nil))
}

// pushRequestCapture observes the real FCM HTTP boundary, never manufactures notices.
type pushRequestCapture struct {
	mu       sync.Mutex
	requests []pushCapturedRequest
	status   int
}
type pushCapturedRequest struct {
	Message struct {
		Token        string            `json:"token"`
		Data         map[string]string `json:"data"`
		Android      map[string]string `json:"android"`
		Notification json.RawMessage   `json:"notification"`
	} `json:"message"`
}

func newPushRequestCapture(tb testing.TB) (*pushRequestCapture, *httptest.Server) {
	capture := &pushRequestCapture{}
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var request pushCapturedRequest
		if err := json.NewDecoder(r.Body).Decode(&request); err != nil {
			http.Error(w, "invalid request", 400)
			return
		}
		capture.mu.Lock()
		capture.requests = append(capture.requests, request)
		status := capture.status
		capture.mu.Unlock()
		if status == 0 {
			status = http.StatusOK
		}
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(status)
	}))
	tb.Cleanup(server.Close)
	return capture, server
}
func (c *pushRequestCapture) reset() { c.mu.Lock(); defer c.mu.Unlock(); c.requests = nil }
func (c *pushRequestCapture) snapshot() []pushCapturedRequest {
	c.mu.Lock()
	defer c.mu.Unlock()
	return append([]pushCapturedRequest(nil), c.requests...)
}

func (c *pushRequestCapture) setStatus(status int) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.status = status
}
