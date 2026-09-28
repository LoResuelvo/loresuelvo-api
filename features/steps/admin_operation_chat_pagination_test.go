package steps_test

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"strconv"

	"github.com/LoResuelvo/loresuelvo-api/internal/domain/audit"
	"github.com/LoResuelvo/loresuelvo-api/internal/testsupport"
	"github.com/cucumber/godog"
)

type operationChatPaginationState struct {
	requestLabel     string
	reason           string
	cursor           string
	firstCorrelation string
}

func registerAdminOperationChatPaginationSteps(sc *godog.ScenarioContext, suite *testSuite) {
	sc.Step(`^que "([^"]*)" contiene los siguientes mensajes persistidos en el orden indicado, con IDs crecientes:$`, suite.operationChatHasOrderedMessages)
	sc.Step(`^consulto el chat administrativo de la operación de la solicitud "([^"]*)" con límite (\d+), motivo "([^"]*)" y correlación "([^"]*)"$`, suite.queryOperationChatFirstPage)
	sc.Step(`^la página contiene "([^"]*)" y "([^"]*)" en ese orden y entrega un cursor siguiente, sin incluir "([^"]*)"$`, suite.operationChatFirstPageExcludesLaterMessage)
	sc.Step(`^que una consulta autorizada previa de "([^"]*)" con límite (\d+), motivo "([^"]*)" y correlación "([^"]*)" devolvió "([^"]*)" y "([^"]*)" y un cursor siguiente$`, suite.previousAuthorizedOperationChatPage)
	sc.Step(`^consulto el chat administrativo de la operación de la solicitud "([^"]*)" con el cursor recibido, reutilizando el motivo "([^"]*)" en "X-Audit-Reason" y con correlación "([^"]*)"$`, suite.queryOperationChatContinuation)
	sc.Step(`^la página contiene sólo "([^"]*)" y no tiene cursor siguiente$`, suite.operationChatLastPageContains)
	sc.Step(`^quedan persistidos dos eventos de acceso distintos, ambos con el motivo "([^"]*)" reutilizado y con las correlaciones "([^"]*)" y "([^"]*)" respectivamente$`, suite.operationChatPagesHaveDistinctAuditEvents)
}

func (suite *testSuite) operationChatHasOrderedMessages(label string, table *godog.Table) error {
	if err := suite.operationChatHasMessages(label, table); err != nil {
		return err
	}
	rows, err := inboxTableRows(table)
	if err != nil {
		return err
	}
	previousID := 0
	for _, row := range rows {
		message := suite.operationChat.messages[row["mensaje"]]
		if message.ID <= previousID {
			return fmt.Errorf("historical message IDs do not increase in table order")
		}
		previousID = message.ID
	}
	return nil
}

func (suite *testSuite) queryOperationChatFirstPage(label string, limit int, reason, correlation string) error {
	request, ok := suite.operationInbox.requests[label]
	if !ok {
		return fmt.Errorf("unknown request %q", label)
	}
	if err := testsupport.ResetOperationDetailAuditCorrelation(suite.scenarioContext, suite.database, correlation); err != nil {
		return err
	}
	suite.operationChat.pagination = operationChatPaginationState{requestLabel: label, reason: reason, firstCorrelation: correlation}
	return suite.sendOperationChatGetWithQuery("jr-"+strconv.Itoa(request.id), reason, correlation, url.Values{"limit": {strconv.Itoa(limit)}})
}

func (suite *testSuite) captureOperationChatNextCursor() error {
	var response struct {
		NextCursor *string `json:"next_cursor"`
	}
	if err := json.Unmarshal(suite.lastBody, &response); err != nil {
		return err
	}
	if response.NextCursor == nil || *response.NextCursor == "" {
		return fmt.Errorf("first chat page has no nonempty continuation cursor")
	}
	suite.operationChat.pagination.cursor = *response.NextCursor
	return nil
}

func (suite *testSuite) operationChatFirstPageExcludesLaterMessage(first, second, excluded string) error {
	if _, ok := suite.operationChat.messages[excluded]; !ok {
		return fmt.Errorf("unknown later message %q", excluded)
	}
	if excluded == first || excluded == second {
		return fmt.Errorf("excluded message must differ from first-page messages")
	}
	if err := suite.operationChatPageContains(first, second); err != nil {
		return err
	}
	return suite.captureOperationChatNextCursor()
}

func (suite *testSuite) previousAuthorizedOperationChatPage(label string, limit int, reason, correlation, first, second string) error {
	if err := suite.queryOperationChatFirstPage(label, limit, reason, correlation); err != nil {
		return err
	}
	if suite.lastStatus != http.StatusOK {
		return fmt.Errorf("previous authorized chat page returned %d: %s", suite.lastStatus, suite.lastBody)
	}
	if err := suite.operationChatPageContains(first, second); err != nil {
		return err
	}
	return suite.captureOperationChatNextCursor()
}

func (suite *testSuite) queryOperationChatContinuation(label, reason, correlation string) error {
	previous := suite.operationChat.pagination
	if previous.requestLabel != label || previous.reason != reason || previous.cursor == "" {
		return fmt.Errorf("continuation does not reuse the authorized page's request, reason and cursor")
	}
	if correlation == previous.firstCorrelation {
		return fmt.Errorf("continuation correlation must differ from initial correlation")
	}
	request := suite.operationInbox.requests[label]
	if err := testsupport.ResetOperationDetailAuditCorrelation(suite.scenarioContext, suite.database, correlation); err != nil {
		return err
	}
	return suite.sendOperationChatGetWithQuery("jr-"+strconv.Itoa(request.id), reason, correlation, url.Values{"cursor": {previous.cursor}})
}

func (suite *testSuite) operationChatLastPageContains(label string) error {
	if err := suite.operationChatPageContains(label); err != nil {
		return err
	}
	var response map[string]json.RawMessage
	if err := json.Unmarshal(suite.lastBody, &response); err != nil {
		return err
	}
	if string(response["next_cursor"]) != "null" {
		return fmt.Errorf("last chat page must have an explicit null continuation cursor")
	}
	return nil
}

func (suite *testSuite) operationChatPagesHaveDistinctAuditEvents(reason, firstCorrelation, secondCorrelation string) error {
	previous := suite.operationChat.pagination
	if previous.reason != reason || previous.firstCorrelation != firstCorrelation || firstCorrelation == secondCorrelation {
		return fmt.Errorf("audit assertion does not match the two page requests")
	}
	request := suite.operationInbox.requests[previous.requestLabel]
	reader := suite.dependencies.Persistence.AuditEventRepository
	watermark, err := reader.CaptureWatermark(suite.scenarioContext)
	if err != nil {
		return err
	}
	resourceType, resourceID := "job_request", strconv.Itoa(request.id)
	action := audit.ActionAccess
	events, err := reader.FindPage(suite.scenarioContext, audit.LogFilter{ResourceType: &resourceType, ResourceID: &resourceID, Action: &action}, watermark, nil, 100)
	if err != nil {
		return err
	}
	if len(events) != 2 {
		return fmt.Errorf("expected exactly two persisted chat access events, got %d", len(events))
	}
	if events[0].ID() == events[1].ID() {
		return fmt.Errorf("chat pages reused one audit event")
	}
	expected := map[string]bool{firstCorrelation: false, secondCorrelation: false}
	for _, event := range events {
		if _, ok := expected[event.CorrelationID()]; !ok || expected[event.CorrelationID()] {
			return fmt.Errorf("unexpected or repeated chat page audit correlation %q", event.CorrelationID())
		}
		persisted, err := reader.FindByID(suite.scenarioContext, event.ID())
		if err != nil {
			return err
		}
		if persisted == nil || persisted.Reason() == nil || persisted.Reason().Text() != reason || persisted.ConversationID() == nil || *persisted.ConversationID() != request.conversationID || persisted.Result() != audit.ResultPrepared {
			return fmt.Errorf("chat page audit evidence does not match persisted reason and conversation")
		}
		total, err := testsupport.OperationDetailAuditCorrelationCount(suite.scenarioContext, suite.database, event.CorrelationID())
		if err != nil {
			return err
		}
		if total != 1 {
			return fmt.Errorf("chat page correlation has %d persisted events", total)
		}
		expected[event.CorrelationID()] = true
	}
	return nil
}
