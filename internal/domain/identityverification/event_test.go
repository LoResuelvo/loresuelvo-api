package identityverification

import (
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
)

func TestNewVerificationEventKeepsOnlyIdentityAndTiming(t *testing.T) {
	eventID, sessionID := uuid.New(), uuid.New()
	occurredOn := time.Date(2026, 9, 1, 11, 59, 0, 0, time.FixedZone("ART", -3*60*60))
	receivedOn := occurredOn.Add(time.Minute)

	event, err := NewVerificationEvent(VerificationResult{
		EventID: eventID, SessionID: sessionID, OccurredOn: occurredOn,
	}, receivedOn)

	require.NoError(t, err)
	require.Equal(t, eventID, event.EventID)
	require.Equal(t, sessionID, event.SessionID)
	require.Equal(t, occurredOn.UTC(), event.OccurredOn)
	require.Equal(t, receivedOn.UTC(), event.ReceivedOn)
}

func TestNewVerificationEventRejectsMissingEventID(t *testing.T) {
	_, err := NewVerificationEvent(VerificationResult{
		SessionID: uuid.New(), OccurredOn: time.Now().UTC(),
	}, time.Now().UTC())

	require.ErrorIs(t, err, ErrInvalidVerification)
}
