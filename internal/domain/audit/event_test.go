package audit_test

import (
	"errors"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/LoResuelvo/loresuelvo-api/internal/domain/audit"
	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
)

func validParams() audit.EventParams {
	return audit.EventParams{
		ID:            uuid.New(),
		OperatorID:    42,
		Action:        audit.ActionCreate,
		ResourceType:  "category",
		ResourceID:    "123",
		OccurredOn:    time.Date(2026, 9, 23, 12, 30, 0, 0, time.FixedZone("ART", -3*60*60)),
		Result:        audit.ResultSucceeded,
		CorrelationID: "req:2026-09-23.1",
	}
}

func TestNewEventNormalizesTimeAndPreservesTypedFields(t *testing.T) {
	params := validParams()
	reason, err := audit.NewReason("  Administrative correction  ")
	require.NoError(t, err)
	change, err := audit.NewStateChange("payment_status", "pending", "approved")
	require.NoError(t, err)
	params.Reason = reason
	params.StateChange = change

	event, err := audit.NewEvent(params)

	require.NoError(t, err)
	require.Equal(t, params.ID, event.ID())
	require.Equal(t, params.OperatorID, event.OperatorID())
	require.Equal(t, params.Action, event.Action())
	require.Equal(t, params.ResourceType, event.ResourceType())
	require.Equal(t, params.ResourceID, event.ResourceID())
	require.Equal(t, params.Result, event.Result())
	require.Equal(t, params.CorrelationID, event.CorrelationID())
	require.Equal(t, params.OccurredOn.UTC(), event.OccurredOn())
	require.Equal(t, time.UTC, event.OccurredOn().Location())
	require.Equal(t, "Administrative correction", event.Reason().Text())
	require.Equal(t, "payment_status", event.StateChange().Field())
	require.Equal(t, "pending", event.StateChange().From())
	require.Equal(t, "approved", event.StateChange().To())
}

func TestNewEventRejectsInvalidRequiredFields(t *testing.T) {
	tests := map[string]func(*audit.EventParams){
		"nil ID":              func(p *audit.EventParams) { p.ID = uuid.Nil },
		"zero operator":       func(p *audit.EventParams) { p.OperatorID = 0 },
		"negative operator":   func(p *audit.EventParams) { p.OperatorID = -1 },
		"missing action":      func(p *audit.EventParams) { p.Action = "" },
		"unknown action":      func(p *audit.EventParams) { p.Action = "publish" },
		"missing resource":    func(p *audit.EventParams) { p.ResourceType = "" },
		"missing time":        func(p *audit.EventParams) { p.OccurredOn = time.Time{} },
		"missing result":      func(p *audit.EventParams) { p.Result = "" },
		"unknown result":      func(p *audit.EventParams) { p.Result = "unknown" },
		"missing correlation": func(p *audit.EventParams) { p.CorrelationID = "" },
	}
	for name, change := range tests {
		t.Run(name, func(t *testing.T) {
			params := validParams()
			change(&params)
			_, err := audit.NewEvent(params)
			require.ErrorIs(t, err, audit.ErrInvalidEvent)
		})
	}
}

func TestNewEventEnforcesActionResultSemantics(t *testing.T) {
	tests := []struct {
		action audit.Action
		result audit.Result
		valid  bool
	}{
		{audit.ActionCreate, audit.ResultSucceeded, true},
		{audit.ActionCreate, audit.ResultFailed, true},
		{audit.ActionCreate, audit.ResultPrepared, false},
		{audit.ActionAccess, audit.ResultPrepared, true},
		{audit.ActionAccess, audit.ResultFailed, true},
		{audit.ActionAccess, audit.ResultSucceeded, false},
		{audit.ActionExecute, audit.ResultSucceeded, true},
		{audit.ActionExecute, audit.ResultFailed, true},
		{audit.ActionExecute, audit.ResultPrepared, false},
	}
	for _, tc := range tests {
		t.Run(string(tc.action)+"/"+string(tc.result), func(t *testing.T) {
			params := validParams()
			params.Action, params.Result = tc.action, tc.result
			_, err := audit.NewEvent(params)
			if tc.valid {
				require.NoError(t, err)
				return
			}
			require.ErrorIs(t, err, audit.ErrInvalidEvent)
		})
	}
}

func TestNewEventRejectsUnsafeOrUnboundedIdentifiers(t *testing.T) {
	tests := map[string]func(*audit.EventParams){
		"resource type spaces":    func(p *audit.EventParams) { p.ResourceType = "payment status" },
		"resource type uppercase": func(p *audit.EventParams) { p.ResourceType = "Payment" },
		"resource type too long":  func(p *audit.EventParams) { p.ResourceType = strings.Repeat("a", 65) },
		"resource ID with URL":    func(p *audit.EventParams) { p.ResourceID = "https://example.test/private" },
		"resource ID too long":    func(p *audit.EventParams) { p.ResourceID = strings.Repeat("a", 129) },
		"correlation control":     func(p *audit.EventParams) { p.CorrelationID = "abc\nsecret" },
		"correlation too long":    func(p *audit.EventParams) { p.CorrelationID = strings.Repeat("a", 129) },
	}
	for name, change := range tests {
		t.Run(name, func(t *testing.T) {
			params := validParams()
			change(&params)
			_, err := audit.NewEvent(params)
			require.ErrorIs(t, err, audit.ErrInvalidEvent)
		})
	}
}

func TestNewEventAllowsCollectionWithoutResourceID(t *testing.T) {
	params := validParams()
	params.ResourceID = ""
	params.Action = audit.ActionAccess
	params.Result = audit.ResultPrepared

	event, err := audit.NewEvent(params)

	require.NoError(t, err)
	require.Empty(t, event.ResourceID())
	require.Nil(t, event.Reason())
	require.Nil(t, event.StateChange())
}

func TestNewReasonRejectsEmptyControlAndOversizeText(t *testing.T) {
	for _, value := range []string{"  ", "line\nwith break", "contains\x00nul", strings.Repeat("a", 501), string([]byte{0xff})} {
		_, err := audit.NewReason(value)
		require.ErrorIs(t, err, audit.ErrInvalidEvent)
	}
}

func TestNewStateChangeRejectsInvalidTransitions(t *testing.T) {
	for _, values := range [][3]string{
		{"", "pending", "approved"},
		{"payment status", "pending", "approved"},
		{"payment_status", "pending", "pending"},
		{"payment_status", "pending", "approved\n"},
		{strings.Repeat("a", 65), "pending", "approved"},
	} {
		_, err := audit.NewStateChange(values[0], values[1], values[2])
		require.ErrorIs(t, err, audit.ErrInvalidEvent)
	}
}

func TestAuditContractExposesOnlyAppendAndRead(t *testing.T) {
	writer := reflect.TypeOf((*audit.Writer)(nil)).Elem()
	reader := reflect.TypeOf((*audit.Reader)(nil)).Elem()
	require.Equal(t, 1, writer.NumMethod())
	require.Equal(t, "Save", writer.Method(0).Name)
	require.Equal(t, 1, reader.NumMethod())
	require.Equal(t, "FindByID", reader.Method(0).Name)

	eventType := reflect.TypeOf(audit.Event{})
	for i := range eventType.NumField() {
		require.False(t, eventType.Field(i).IsExported())
	}
	require.False(t, errors.Is(audit.ErrPersistence, audit.ErrInvalidEvent))
}
