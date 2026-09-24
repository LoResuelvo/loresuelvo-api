package audit_log_handler

import (
	"time"

	"github.com/LoResuelvo/loresuelvo-api/internal/domain/audit"
	"github.com/google/uuid"
)

type pageResponse struct {
	Events     []eventResponse `json:"events"`
	NextCursor *string         `json:"next_cursor"`
}

type eventResponse struct {
	ID            uuid.UUID    `json:"id"`
	OperatorID    int          `json:"operator_id"`
	Action        audit.Action `json:"action"`
	ResourceType  string       `json:"resource_type"`
	ResourceID    string       `json:"resource_id,omitempty"`
	OccurredOn    time.Time    `json:"occurred_on"`
	Result        audit.Result `json:"result"`
	CorrelationID string       `json:"correlation_id"`
	Reason        string       `json:"reason,omitempty"`
}

func eventResponsesFromDomain(events []*audit.Event) []eventResponse {
	responses := make([]eventResponse, 0, len(events))
	for _, event := range events {
		response := eventResponse{
			ID: event.ID(), OperatorID: event.OperatorID(), Action: event.Action(),
			ResourceType: event.ResourceType(), ResourceID: event.ResourceID(),
			OccurredOn: event.OccurredOn().UTC(), Result: event.Result(),
			CorrelationID: event.CorrelationID(),
		}
		if reason := event.Reason(); reason != nil {
			response.Reason = reason.Text()
		}
		responses = append(responses, response)
	}
	return responses
}
