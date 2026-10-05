package realtime

import (
	"context"
	"encoding/json"
	"fmt"
)

// EventEnvelope is the transport representation of a realtime event. The
// payload is kept as the exact JSON document sent to a WebSocket client while
// the target fields let each API instance deliver it to its local connections.
type EventEnvelope struct {
	ID              string          `json:"id"`
	TargetAuthID    string          `json:"target_auth_id"`
	TargetRole      string          `json:"target_role"`
	TargetProfileID int             `json:"target_profile_id"`
	Payload         json.RawMessage `json:"payload"`
}

// EventBus is the transport capability required by Dispatcher.
// Dispatchers call it after the business transaction has committed and listen
// independently on every API instance.
type EventBus interface {
	Publish(context.Context, EventEnvelope) error
	Listen(context.Context, func(EventEnvelope)) error
}

func validateEventEnvelope(event EventEnvelope) error {
	if event.ID == "" {
		return fmt.Errorf("event id is required")
	}
	if event.TargetAuthID == "" {
		return fmt.Errorf("event target auth id is required")
	}
	if event.TargetRole == "" {
		return fmt.Errorf("event target role is required")
	}
	if event.TargetProfileID <= 0 {
		return fmt.Errorf("event target profile id is required")
	}
	if len(event.Payload) == 0 || !json.Valid(event.Payload) {
		return fmt.Errorf("event payload must be valid JSON")
	}
	return nil
}
