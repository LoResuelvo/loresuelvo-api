package steps_test

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"reflect"
	"strconv"

	"github.com/LoResuelvo/loresuelvo-api/internal/domain/audit"
	"github.com/LoResuelvo/loresuelvo-api/internal/domain/conversation"
	"github.com/LoResuelvo/loresuelvo-api/internal/testsupport"
	"github.com/cucumber/godog"
)

type operationChatState struct {
	conversations map[string]int
	messages      map[string]conversation.Message
	event         *audit.Event
}

func registerAdminOperationChatSteps(sc *godog.ScenarioContext, suite *testSuite) {
	sc.Step(`^que existe la siguiente solicitud de trabajo con una única conversación de trabajo "([^"]*)" creada junto con ella y activada al ser aceptada por "([^"]*)":$`, suite.thereIsOperationChat)
	sc.Step(`^que "([^"]*)" tiene los siguientes mensajes persistidos:$`, suite.operationChatHasMessages)
	sc.Step(`^consulto el chat administrativo de la operación de la solicitud "([^"]*)" con la cabecera "X-Audit-Reason" igual a "([^"]*)" y la correlación "([^"]*)"$`, suite.queryOperationChat)
	sc.Step(`^la respuesta identifica la operación "jr-" seguida del ID persistido de "([^"]*)", la conversación "([^"]*)" y su vínculo con la solicitud$`, suite.operationChatIdentifiesRequest)
	sc.Step(`^la página contiene "([^"]*)" y "([^"]*)" en ese orden, con ID, rol remitente, contenido e instante de creación persistidos$`, suite.operationChatContainsMessages)
	sc.Step(`^antes de entregar los mensajes queda persistido un evento de acceso con operador "([^"]*)", operación "([^"]*)", conversación "([^"]*)", motivo "([^"]*)", fecha, resultado y correlación "([^"]*)"$`, suite.operationChatAccessPersisted)
	sc.Step(`^el resultado auditado indica la preparación de la entrega, no la recepción o lectura por el cliente$`, suite.operationChatAuditPrepared)
	sc.Step(`^el evento de auditoría no contiene mensajes ni adjuntos del chat$`, suite.operationChatAuditMinimized)
}
func (suite *testSuite) thereIsOperationChat(label, provider string, table *godog.Table) error {
	rows, err := inboxTableRows(table)
	if err != nil {
		return err
	}
	if len(rows) != 1 || rows[0]["prestador"] != provider || rows[0]["estado"] != "accepted" {
		return fmt.Errorf("expected one accepted request for provider %q", provider)
	}
	if err := suite.thereIsAuditedDetailJobRequest(table); err != nil {
		return err
	}
	request := suite.operationInbox.requests[rows[0]["solicitud"]]
	found, err := suite.conversationRepository.FindByID(suite.scenarioContext, request.conversationID)
	if err != nil {
		return err
	}
	if found.ConversationType() != conversation.TypeWork || found.Status() != conversation.StatusActive {
		return fmt.Errorf("request conversation is not active work conversation")
	}
	suite.operationChat.conversations = map[string]int{label: request.conversationID}
	suite.operationChat.messages = map[string]conversation.Message{}
	return nil
}
func (suite *testSuite) operationChatHasMessages(label string, table *godog.Table) error {
	id, ok := suite.operationChat.conversations[label]
	if !ok {
		return fmt.Errorf("unknown conversation %q", label)
	}
	rows, err := inboxTableRows(table)
	if err != nil {
		return err
	}
	for _, row := range rows {
		created, err := parseInboxInstant(row["creado"])
		if err != nil {
			return err
		}
		message, err := (testsupport.OperationChatFixture{DB: suite.database}).AddMessage(suite.scenarioContext, id, row["remitente"], row["contenido"], created)
		if err != nil {
			return err
		}
		suite.operationChat.messages[row["mensaje"]] = message
	}
	return nil
}
func (suite *testSuite) queryOperationChat(label, reason, correlation string) error {
	fixture, ok := suite.operationInbox.requests[label]
	if !ok {
		return fmt.Errorf("unknown request %q", label)
	}
	if err := testsupport.ResetOperationDetailAuditCorrelation(suite.scenarioContext, suite.database, correlation); err != nil {
		return err
	}
	request, err := http.NewRequest(http.MethodGet, suite.server.URL+operationsInboxPath+"/jr-"+strconv.Itoa(fixture.id)+"/conversation", nil)
	if err != nil {
		return err
	}
	request.Header.Set("Authorization", "Bearer "+suite.tokenBuilder.BuildToken(suite.currentAuth0ID, suite.currentPermissions))
	request.Header.Set("X-Audit-Reason", reason)
	request.Header.Set("X-Request-ID", correlation)
	response, err := http.DefaultClient.Do(request)
	if err != nil {
		return err
	}
	defer response.Body.Close()
	body, err := io.ReadAll(response.Body)
	if err != nil {
		return err
	}
	suite.lastStatus = response.StatusCode
	suite.lastBody = body
	suite.adminRequest.headers = response.Header.Clone()
	return nil
}
func (suite *testSuite) operationChatIdentifiesRequest(requestLabel, conversationLabel string) error {
	var response struct {
		OperationID       string `json:"operation_id"`
		ConversationID    int    `json:"conversation_id"`
		JobRequestID      *int   `json:"job_request_id"`
		ServiceProposalID *int   `json:"service_proposal_id"`
	}
	if err := json.Unmarshal(suite.lastBody, &response); err != nil {
		return err
	}
	request := suite.operationInbox.requests[requestLabel]
	if response.OperationID != "jr-"+strconv.Itoa(request.id) || response.ConversationID != suite.operationChat.conversations[conversationLabel] || response.JobRequestID == nil || *response.JobRequestID != request.id || response.ServiceProposalID != nil {
		return fmt.Errorf("incorrect conversation association: %s", suite.lastBody)
	}
	return nil
}
func (suite *testSuite) operationChatContainsMessages(first, second string) error {
	var response struct {
		Messages []struct {
			ID         int    `json:"id"`
			SenderRole string `json:"sender_role"`
			Content    string `json:"content"`
			CreatedOn  string `json:"created_on"`
		} `json:"messages"`
	}
	if err := json.Unmarshal(suite.lastBody, &response); err != nil {
		return err
	}
	if len(response.Messages) != 2 {
		return fmt.Errorf("expected two messages, got %d", len(response.Messages))
	}
	for index, label := range []string{first, second} {
		expected, ok := suite.operationChat.messages[label]
		if !ok {
			return fmt.Errorf("unknown message %q", label)
		}
		found := response.Messages[index]
		created, err := parseInboxInstant(found.CreatedOn)
		if err != nil {
			return err
		}
		if found.ID != expected.ID || found.SenderRole != expected.SenderRole || found.Content != expected.Content || !created.Equal(expected.CreatedOn) {
			return fmt.Errorf("message %q differs from persisted fixture", label)
		}
	}
	return nil
}
func (suite *testSuite) operationChatAccessPersisted(email, requestLabel, conversationLabel, reason, correlation string) error {
	operatorID, err := suite.userRepository.FindOperatorIDByAuthID(suite.scenarioContext, auth0IDForAdminEmail(email))
	if err != nil {
		return err
	}
	reader := suite.dependencies.Persistence.AuditEventRepository
	watermark, err := reader.CaptureWatermark(suite.scenarioContext)
	if err != nil {
		return err
	}
	resourceType := "job_request"
	resourceID := strconv.Itoa(suite.operationInbox.requests[requestLabel].id)
	events, err := reader.FindPage(suite.scenarioContext, audit.LogFilter{OperatorID: &operatorID, ResourceType: &resourceType, ResourceID: &resourceID}, watermark, nil, 100)
	if err != nil {
		return err
	}
	count := 0
	for _, event := range events {
		if event.CorrelationID() != correlation {
			continue
		}
		count++
		persisted, err := reader.FindByID(suite.scenarioContext, event.ID())
		if err != nil {
			return err
		}
		if persisted == nil || persisted.ConversationID() == nil || *persisted.ConversationID() != suite.operationChat.conversations[conversationLabel] || persisted.Reason() == nil || persisted.Reason().Text() != reason || persisted.OccurredOn().IsZero() || persisted.Action() != audit.ActionAccess {
			return fmt.Errorf("incorrect persisted chat access evidence")
		}
		suite.operationChat.event = persisted
	}
	total, err := testsupport.OperationDetailAuditCorrelationCount(suite.scenarioContext, suite.database, correlation)
	if err != nil {
		return err
	}
	if count != 1 || total != 1 || suite.operationDetailAuditCapture.attempts != 1 {
		return fmt.Errorf("expected exactly one synchronous audit save, got matches=%d total=%d attempts=%d", count, total, suite.operationDetailAuditCapture.attempts)
	}
	return nil
}
func (suite *testSuite) operationChatAuditPrepared() error {
	if suite.operationChat.event == nil || suite.operationChat.event.Result() != audit.ResultPrepared {
		return fmt.Errorf("chat access was not audited as prepared")
	}
	return nil
}
func (suite *testSuite) operationChatAuditMinimized() error {
	event := suite.operationChat.event
	if event == nil {
		return fmt.Errorf("chat event was not verified")
	}
	// The audit aggregate is an allowlisted evidence contract, not a chat payload.
	typ := reflect.TypeFor[audit.Event]()
	allowed := map[string]bool{"id": true, "operatorID": true, "action": true, "resourceType": true, "resourceID": true, "occurredOn": true, "result": true, "correlationID": true, "reason": true, "stateChange": true, "conversationID": true}
	for i := 0; i < typ.NumField(); i++ {
		if !allowed[typ.Field(i).Name] {
			return fmt.Errorf("unexpected audit payload field %q", typ.Field(i).Name)
		}
	}
	if event.StateChange() != nil {
		return fmt.Errorf("chat access audit contains changes")
	}
	return nil
}
