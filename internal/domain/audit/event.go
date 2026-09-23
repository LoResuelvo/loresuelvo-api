package audit

import (
	"fmt"
	"regexp"
	"strings"
	"time"
	"unicode"
	"unicode/utf8"

	"github.com/google/uuid"
)

// Action describes the kind of operation, not a particular endpoint.
type Action string

const (
	ActionCreate  Action = "create"
	ActionAccess  Action = "access"
	ActionExecute Action = "execute"
)

// Result distinguishes completed work from a delivery merely prepared for a client.
type Result string

const (
	ResultSucceeded Result = "succeeded"
	ResultPrepared  Result = "prepared"
	ResultFailed    Result = "failed"
)

var (
	snakeCasePattern      = regexp.MustCompile(`^[a-z][a-z0-9_]*$`)
	safeIdentifierPattern = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9._:-]*$`)
)

// Reason is bounded, human-authored text. Callers must select/sanitize it and
// must never copy credentials, request bodies, private documents or chat content.
type Reason struct {
	text string
}

func NewReason(text string) (*Reason, error) {
	text = strings.TrimSpace(text)
	if text == "" || len(text) > 500 || !utf8.ValidString(text) || hasControl(text) {
		return nil, fmt.Errorf("%w: reason must be 1-500 bytes of printable UTF-8", ErrInvalidEvent)
	}
	return &Reason{text: text}, nil
}

func (r *Reason) Text() string {
	if r == nil {
		return ""
	}
	return r.text
}

// StateChange holds only a named state transition; it is not a generic metadata bag.
type StateChange struct {
	field string
	from  string
	to    string
}

func NewStateChange(field, from, to string) (*StateChange, error) {
	if !validSnakeCase(field, 64) || !validSnakeCase(from, 64) || !validSnakeCase(to, 64) || from == to {
		return nil, fmt.Errorf("%w: state change requires distinct bounded snake_case states", ErrInvalidEvent)
	}
	return &StateChange{field: field, from: from, to: to}, nil
}

func (s *StateChange) Field() string {
	if s == nil {
		return ""
	}
	return s.field
}

func (s *StateChange) From() string {
	if s == nil {
		return ""
	}
	return s.from
}

func (s *StateChange) To() string {
	if s == nil {
		return ""
	}
	return s.to
}

// EventParams contains the only metadata admitted by the audit contract.
// OperatorID must be resolved from authenticated identity by the caller.
// ResourceID is optional for collection-level operations.
type EventParams struct {
	ID            uuid.UUID
	OperatorID    int
	Action        Action
	ResourceType  string
	ResourceID    string
	OccurredOn    time.Time
	Result        Result
	CorrelationID string
	Reason        *Reason
	StateChange   *StateChange
}

// Event is immutable to consumers. Corrections require a new event.
type Event struct {
	id            uuid.UUID
	operatorID    int
	action        Action
	resourceType  string
	resourceID    string
	occurredOn    time.Time
	result        Result
	correlationID string
	reason        *Reason
	stateChange   *StateChange
}

func NewEvent(params EventParams) (*Event, error) {
	if params.ID == uuid.Nil {
		return nil, fmt.Errorf("%w: ID is required", ErrInvalidEvent)
	}
	if params.OperatorID <= 0 {
		return nil, fmt.Errorf("%w: operator ID must be positive", ErrInvalidEvent)
	}
	if !validActionResult(params.Action, params.Result) {
		return nil, fmt.Errorf("%w: action and result are invalid or incompatible", ErrInvalidEvent)
	}
	if !validSnakeCase(params.ResourceType, 64) {
		return nil, fmt.Errorf("%w: resource type must be bounded snake_case", ErrInvalidEvent)
	}
	if params.ResourceID != "" && !validSafeIdentifier(params.ResourceID, 128) {
		return nil, fmt.Errorf("%w: resource ID is invalid", ErrInvalidEvent)
	}
	if params.OccurredOn.IsZero() {
		return nil, fmt.Errorf("%w: occurrence time is required", ErrInvalidEvent)
	}
	if !validSafeIdentifier(params.CorrelationID, 128) {
		return nil, fmt.Errorf("%w: correlation ID is invalid", ErrInvalidEvent)
	}
	if params.Reason != nil {
		if _, err := NewReason(params.Reason.text); err != nil {
			return nil, err
		}
	}
	if params.StateChange != nil {
		if _, err := NewStateChange(params.StateChange.field, params.StateChange.from, params.StateChange.to); err != nil {
			return nil, err
		}
	}

	event := &Event{
		id:            params.ID,
		operatorID:    params.OperatorID,
		action:        params.Action,
		resourceType:  params.ResourceType,
		resourceID:    params.ResourceID,
		occurredOn:    params.OccurredOn.UTC(),
		result:        params.Result,
		correlationID: params.CorrelationID,
	}
	if params.Reason != nil {
		reason := *params.Reason
		event.reason = &reason
	}
	if params.StateChange != nil {
		stateChange := *params.StateChange
		event.stateChange = &stateChange
	}
	return event, nil
}

func (e *Event) ID() uuid.UUID         { return e.id }
func (e *Event) OperatorID() int       { return e.operatorID }
func (e *Event) Action() Action        { return e.action }
func (e *Event) ResourceType() string  { return e.resourceType }
func (e *Event) ResourceID() string    { return e.resourceID }
func (e *Event) OccurredOn() time.Time { return e.occurredOn }
func (e *Event) Result() Result        { return e.result }
func (e *Event) CorrelationID() string { return e.correlationID }
func (e *Event) Reason() *Reason {
	if e.reason == nil {
		return nil
	}
	reason := *e.reason
	return &reason
}
func (e *Event) StateChange() *StateChange {
	if e.stateChange == nil {
		return nil
	}
	stateChange := *e.stateChange
	return &stateChange
}

func validActionResult(action Action, result Result) bool {
	switch action {
	case ActionCreate, ActionExecute:
		return result == ResultSucceeded || result == ResultFailed
	case ActionAccess:
		return result == ResultPrepared || result == ResultFailed
	default:
		return false
	}
}

func validSnakeCase(value string, maxBytes int) bool {
	return len(value) > 0 && len(value) <= maxBytes && snakeCasePattern.MatchString(value)
}

func validSafeIdentifier(value string, maxBytes int) bool {
	return len(value) > 0 && len(value) <= maxBytes && safeIdentifierPattern.MatchString(value)
}

func hasControl(value string) bool {
	for _, r := range value {
		if unicode.IsControl(r) {
			return true
		}
	}
	return false
}
