package audit_log_handler

import (
	"github.com/LoResuelvo/loresuelvo-api/internal/domain/audit"
	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
	"testing"
	"time"
)

func TestAuditResponseIncludesCurrentOperatorNameWithoutReplacingID(t *testing.T) {
	event, err := audit.NewEvent(audit.EventParams{ID: uuid.New(), OperatorID: 7, Action: audit.ActionAccess, ResourceType: "audit_log", OccurredOn: time.Now(), Result: audit.ResultPrepared, CorrelationID: "request-1"})
	require.NoError(t, err)
	responses := eventResponsesFromDomain([]*audit.Event{event}, map[int]string{7: "Sofía López"})
	require.Equal(t, 7, responses[0].OperatorID)
	require.NotNil(t, responses[0].OperatorName)
	require.Equal(t, "Sofía López", *responses[0].OperatorName)
	missing := eventResponsesFromDomain([]*audit.Event{event}, nil)
	require.Nil(t, missing[0].OperatorName)
	require.Equal(t, 7, missing[0].OperatorID)
}
