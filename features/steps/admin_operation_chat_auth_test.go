package steps_test

import (
	"fmt"
	"net/url"
	"slices"
	"strconv"
	"time"

	"github.com/LoResuelvo/loresuelvo-api/internal/domain/audit"
	"github.com/LoResuelvo/loresuelvo-api/internal/domain/conversation"
	"github.com/LoResuelvo/loresuelvo-api/internal/testsupport"
	"github.com/cucumber/godog"
)

func registerAdminOperationChatAuthSteps(sc *godog.ScenarioContext, suite *testSuite) {
	sc.Step(`^intento consultar "([^"]*)" por el endpoint de detalle de conversación de participantes$`, suite.queryParticipantConversationAsChatOperator)
	sc.Step(`^no se entregan mensajes ni URLs de adjuntos$`, suite.operationChatErrorHasNoPrivateData)
	sc.Step(`^intento consultar el chat administrativo de "([^"]*)" con motivo "([^"]*)"$`, suite.queryInitialOperationChat)
	sc.Step(`^que no se realizó ninguna consulta previa de "([^"]*)"$`, suite.operationChatHasNoPreviousAccess)
	sc.Step(`^que "([^"]*)" tiene tres mensajes persistidos y una consulta autorizada previa de "([^"]*)" con límite (\d+) y motivo "([^"]*)" devolvió un cursor siguiente válido$`, suite.operationChatHasAuthorizedContinuation)
	sc.Step(`^intento consultar el chat administrativo de la operación de la solicitud "([^"]*)" sin cursor con motivo "([^"]*)"$`, suite.queryOperationChatWithoutCursor)
	sc.Step(`^intento consultar el chat administrativo de la operación de la solicitud "([^"]*)" con el cursor recibido con motivo "([^"]*)"$`, suite.queryOperationChatWithReceivedCursor)
}

func (suite *testSuite) operationChatHasNoPreviousAccess(label string) error {
	request, ok := suite.operationInbox.requests[label]
	if !ok {
		return fmt.Errorf("unknown request %q", label)
	}
	if suite.operationChat.pagination.cursor != "" {
		return fmt.Errorf("unexpected previous chat continuation cursor")
	}
	reader := suite.dependencies.Persistence.AuditEventRepository
	watermark, err := reader.CaptureWatermark(suite.scenarioContext)
	if err != nil {
		return err
	}
	resourceType, resourceID := "job_request", strconv.Itoa(request.id)
	action := audit.ActionAccess
	events, err := reader.FindPage(suite.scenarioContext, audit.LogFilter{ResourceType: &resourceType, ResourceID: &resourceID, Action: &action}, watermark, nil, 1)
	if err != nil {
		return err
	}
	if len(events) != 0 {
		return fmt.Errorf("request already has an audited administrative access")
	}
	return nil
}

func (suite *testSuite) operationChatHasAuthorizedContinuation(conversationLabel, requestLabel string, limit int, reason string) error {
	conversationID, ok := suite.operationChat.conversations[conversationLabel]
	if !ok {
		return fmt.Errorf("unknown conversation %q", conversationLabel)
	}
	request, ok := suite.operationInbox.requests[requestLabel]
	if !ok {
		return fmt.Errorf("unknown request %q", requestLabel)
	}
	if request.conversationID != conversationID {
		return fmt.Errorf("conversation fixture is not linked to request")
	}
	if !slices.Contains(suite.currentPermissions, "read:admin_chat_audit") || suite.adminRequest.omitBearer || suite.adminRequest.invalidBearer {
		return fmt.Errorf("previous chat request must have valid authentication and chat permission")
	}
	fixture := testsupport.OperationChatFixture{DB: suite.database}
	if err := fixture.DeleteMessages(suite.scenarioContext, conversationID); err != nil {
		return err
	}
	for index := 1; index <= 3; index++ {
		label := "auth-M" + strconv.Itoa(index)
		message, err := fixture.AddMessage(suite.scenarioContext, conversationID, conversation.SenderConsumer, "Private permission test message "+strconv.Itoa(index), request.createdOn.Add(time.Duration(index)*time.Minute))
		if err != nil {
			return err
		}
		suite.operationChat.messages[label] = message
	}
	persisted, err := suite.conversationRepository.FindByID(suite.scenarioContext, conversationID)
	if err != nil {
		return err
	}
	if len(persisted.Messages()) != 3 {
		return fmt.Errorf("expected three persisted conversation messages")
	}
	return suite.previousAuthorizedOperationChatPage(requestLabel, limit, reason, "request-chat-permission-prior", "auth-M1", "auth-M2")
}

func (suite *testSuite) queryOperationChatWithoutCursor(label, reason string) error {
	if suite.operationChat.pagination.cursor != "" {
		return fmt.Errorf("initial query must not have a previous cursor")
	}
	return suite.queryInitialOperationChat(label, reason)
}
func (suite *testSuite) queryOperationChatWithReceivedCursor(label, reason string) error {
	previous := suite.operationChat.pagination
	if previous.requestLabel != label || previous.cursor == "" {
		return fmt.Errorf("continuation requires a previously received valid cursor for this request")
	}
	operationID, err := suite.persistedOperationChatRequestID(label)
	if err != nil {
		return err
	}
	return suite.sendOperationChatGetWithQuery(operationID, reason, "", url.Values{"cursor": {previous.cursor}})
}

func (suite *testSuite) queryParticipantConversationAsChatOperator(label string) error {
	conversationID, ok := suite.operationChat.conversations[label]
	if !ok {
		return fmt.Errorf("unknown conversation %q", label)
	}
	persisted, err := suite.conversationRepository.FindByID(suite.scenarioContext, conversationID)
	if err != nil {
		return err
	}
	if persisted.ConversationType() != conversation.TypeWork || persisted.Status() != conversation.StatusActive {
		return fmt.Errorf("participant endpoint fixture must be an existing active work conversation")
	}
	return suite.sendAdminGet("/conversations/"+strconv.Itoa(persisted.ID()), nil, "")
}
