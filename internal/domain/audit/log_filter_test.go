package audit_test

import (
	"strings"
	"testing"
	"time"

	"github.com/LoResuelvo/loresuelvo-api/internal/domain/audit"
	"github.com/stretchr/testify/require"
)

func TestLogFilterAcceptsIndependentDimensionsAndHalfOpenWindow(t *testing.T) {
	operatorID := 7
	action := audit.ActionCreate
	resourceType := "payment"
	resourceID := "payment:42"
	result := audit.ResultPrepared // Valid filter value, even if no create event can have this result.
	from := time.Date(2026, 9, 20, 10, 0, 0, 0, time.UTC)
	to := from.Add(time.Hour)
	filter := audit.LogFilter{
		OperatorID: &operatorID, Action: &action, ResourceType: &resourceType,
		ResourceID: &resourceID, Result: &result, OccurredFrom: &from, OccurredTo: &to,
	}
	require.NoError(t, filter.Validate())
	require.NoError(t, (audit.LogFilter{}).Validate())
	require.NoError(t, (audit.LogFilter{OccurredFrom: &from}).Validate())
	require.NoError(t, (audit.LogFilter{OccurredTo: &to}).Validate())
}

func TestLogFilterRejectsInvalidDimensions(t *testing.T) {
	zero := 0
	negative := -1
	tooLarge := int(int64(1 << 31))
	unknownAction := audit.Action("unknown")
	badType := "Payment"
	longType := strings.Repeat("p", 65)
	empty := ""
	badID := "https://private.test"
	longID := strings.Repeat("p", 129)
	unknownResult := audit.Result("unknown")
	start := time.Date(2026, 9, 21, 0, 0, 0, 0, time.UTC)
	end := start.Add(-time.Hour)
	for _, tc := range []struct {
		name   string
		filter audit.LogFilter
	}{
		{"zero operator", audit.LogFilter{OperatorID: &zero}},
		{"negative operator", audit.LogFilter{OperatorID: &negative}},
		{"out-of-range operator", audit.LogFilter{OperatorID: &tooLarge}},
		{"unknown action", audit.LogFilter{Action: &unknownAction}},
		{"invalid resource type", audit.LogFilter{ResourceType: &badType}},
		{"empty resource type", audit.LogFilter{ResourceType: &empty}},
		{"long resource type", audit.LogFilter{ResourceType: &longType}},
		{"unsafe resource ID", audit.LogFilter{ResourceID: &badID}},
		{"empty resource ID", audit.LogFilter{ResourceID: &empty}},
		{"long resource ID", audit.LogFilter{ResourceID: &longID}},
		{"unknown result", audit.LogFilter{Result: &unknownResult}},
		{"zero start", audit.LogFilter{OccurredFrom: new(time.Time)}},
		{"zero end", audit.LogFilter{OccurredTo: new(time.Time)}},
		{"inverted window", audit.LogFilter{OccurredFrom: &start, OccurredTo: &end}},
		{"empty window", audit.LogFilter{OccurredFrom: &start, OccurredTo: &start}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			require.ErrorIs(t, tc.filter.Validate(), audit.ErrInvalidQuery)
		})
	}
}
