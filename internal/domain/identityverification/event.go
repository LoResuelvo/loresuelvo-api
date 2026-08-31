package identityverification

import (
	"time"

	"github.com/google/uuid"
)

// VerificationEvent records only the identity and timing needed to process a
// verifier result idempotently. It deliberately excludes the verifier payload.
type VerificationEvent struct {
	EventID    uuid.UUID
	SessionID  uuid.UUID
	OccurredOn time.Time
	ReceivedOn time.Time
}

func NewVerificationEvent(result VerificationResult, receivedOn time.Time) (*VerificationEvent, error) {
	if result.EventID == uuid.Nil || result.SessionID == uuid.Nil || result.OccurredOn.IsZero() || receivedOn.IsZero() {
		return nil, ErrInvalidVerification
	}
	return &VerificationEvent{
		EventID:    result.EventID,
		SessionID:  result.SessionID,
		OccurredOn: result.OccurredOn.UTC(),
		ReceivedOn: receivedOn.UTC(),
	}, nil
}
