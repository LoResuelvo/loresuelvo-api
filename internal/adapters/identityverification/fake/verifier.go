package fake

import (
	"context"
	"sync"

	"github.com/LoResuelvo/loresuelvo-api/internal/domain/identityverification"
	"github.com/google/uuid"
)

type Verifier struct {
	mutex     sync.Mutex
	requests  []identityverification.SessionRequest
	available bool
}

func NewVerifier() *Verifier { return &Verifier{available: true} }

func (verifier *Verifier) CreateSession(_ context.Context, request identityverification.SessionRequest) (identityverification.SessionCredentials, error) {
	verifier.mutex.Lock()
	defer verifier.mutex.Unlock()
	if !verifier.available {
		return identityverification.SessionCredentials{}, identityverification.ErrVerifierUnavailable
	}
	verifier.requests = append(verifier.requests, request)
	sessionID := uuid.New()
	if request.ExistingSessionID != nil {
		sessionID = *request.ExistingSessionID
	}
	return identityverification.SessionCredentials{
		SessionID: sessionID, SessionToken: "temporary-session-token",
		VerificationURL: "https://verify.example/session", Status: identityverification.StatusNotStarted,
		Verifier: "fake", WorkflowID: uuid.New(), WorkflowVersion: 1,
	}, nil
}

func (verifier *Verifier) SetAvailable(available bool) {
	verifier.mutex.Lock()
	defer verifier.mutex.Unlock()
	verifier.available = available
}

func (verifier *Verifier) Reset() {
	verifier.mutex.Lock()
	defer verifier.mutex.Unlock()
	verifier.requests = nil
	verifier.available = true
}
