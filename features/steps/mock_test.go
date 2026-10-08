package steps_test

import (
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"

	"github.com/LoResuelvo/loresuelvo-api/internal/domain/audit"
	"github.com/LoResuelvo/loresuelvo-api/internal/domain/claim"
	"github.com/google/uuid"
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

// adminClaimCapture observes real persistence at the evidence resolver boundary.
// The writer remains real except in the explicitly injected failure scenario.
type adminClaimCapture struct {
	mu       sync.Mutex
	fail     bool
	reader   audit.Reader
	pending  []*audit.Event
	ids      []string
	verified []uuid.UUID
}

func (c *adminClaimCapture) reset() {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.fail = false
	c.pending = nil
	c.ids = nil
	c.verified = nil
}

type adminClaimAuditWriter struct {
	inner   audit.Writer
	capture *adminClaimCapture
}

func (w adminClaimAuditWriter) Save(ctx context.Context, event *audit.Event) error {
	w.capture.mu.Lock()
	fail := w.capture.fail
	w.capture.mu.Unlock()
	if fail {
		return errors.New("injected administrative claim access audit failure")
	}
	if err := w.inner.Save(ctx, event); err != nil {
		return err
	}
	w.capture.mu.Lock()
	defer w.capture.mu.Unlock()
	w.capture.pending = append(w.capture.pending, event)
	return nil
}

type adminClaimEvidenceImages struct {
	inner   claim.AdministrativeEvidenceImages
	capture *adminClaimCapture
}

func (r adminClaimEvidenceImages) ResolveAdministrativeClaimEvidenceImage(ctx context.Context, fileID string) (string, error) {
	c := r.capture
	c.mu.Lock()
	defer c.mu.Unlock()
	c.ids = append(c.ids, fileID)
	if len(c.pending) != 1 || c.reader == nil {
		return "", fmt.Errorf("expected one persisted access audit before evidence resolution")
	}
	expected := c.pending[0]
	event, err := c.reader.FindByID(ctx, expected.ID())
	if err != nil {
		return "", err
	}
	if event == nil || event.Action() != audit.ActionAccess || event.Result() != audit.ResultPrepared || event.ResourceType() != "claim" || event.ResourceID() != expected.ResourceID() || event.OperatorID() != expected.OperatorID() || event.CorrelationID() != expected.CorrelationID() {
		return "", fmt.Errorf("access audit not persisted before evidence signing")
	}
	c.verified = append(c.verified, event.ID())
	return r.inner.ResolveAdministrativeClaimEvidenceImage(ctx, fileID)
}
