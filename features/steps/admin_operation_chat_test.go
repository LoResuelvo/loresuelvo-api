package steps_test

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"reflect"
	"slices"
	"strconv"
	"strings"
	"time"

	"github.com/LoResuelvo/loresuelvo-api/internal/domain/audit"
	"github.com/LoResuelvo/loresuelvo-api/internal/domain/conversation"
	"github.com/LoResuelvo/loresuelvo-api/internal/domain/payment"
	serviceproposal "github.com/LoResuelvo/loresuelvo-api/internal/domain/service_proposal"
	"github.com/LoResuelvo/loresuelvo-api/internal/testsupport"
	"github.com/cucumber/godog"
)

type operationChatState struct {
	conversations map[string]int
	messages      map[string]conversation.Message
	event         *audit.Event
	pagination    operationChatPaginationState
}

func registerAdminOperationChatSteps(sc *godog.ScenarioContext, suite *testSuite) {
	registerAdminOperationChatPaginationSteps(sc, suite)
	sc.Step(`^que existe la siguiente solicitud de trabajo con una única conversación de trabajo "([^"]*)" creada junto con ella y activada al ser aceptada por "([^"]*)":$`, suite.thereIsOperationChat)
	sc.Step(`^que "([^"]*)" tiene los siguientes mensajes persistidos:$`, suite.operationChatHasMessages)
	sc.Step(`^que "([^"]*)" no tiene mensajes$`, suite.operationChatHasNoMessages)
	sc.Step(`^que existe otra conversación de trabajo "([^"]*)" con mensajes privados que no está vinculada a "([^"]*)"$`, suite.thereIsUnrelatedOperationChat)
	sc.Step(`^intento consultar el chat administrativo de la operación de la solicitud "([^"]*)" con motivo "([^"]*)" y el parámetro de consulta "conversation_id" igual al ID persistido de "([^"]*)"$`, suite.queryOperationChatSelectingConversation)
	sc.Step(`^la respuesta no contiene mensajes ni URLs de adjuntos de "([^"]*)"$`, suite.operationChatErrorDoesNotLeakUnrelatedConversation)
	sc.Step(`^consulto el chat administrativo de la operación de la solicitud "([^"]*)" con motivo "([^"]*)"$`, suite.queryInitialOperationChat)
	sc.Step(`^que "([^"]*)" contiene los siguientes mensajes persistidos:$`, suite.operationChatHasMessages)
	sc.Step(`^que existen las siguientes propuestas vinculadas a "([^"]*)":$`, suite.operationChatHasProposals)
	sc.Step(`^consulto el chat administrativo de la operación "sp-" seguida del ID persistido de "([^"]*)" con motivo "([^"]*)"$`, suite.queryProposalOperationChat)
	sc.Step(`^la respuesta identifica la operación "sp-" seguida del ID persistido de "([^"]*)" y su vínculo con la conversación "([^"]*)" compartida con "([^"]*)"$`, suite.operationChatIdentifiesSharedProposal)
	sc.Step(`^la página incluye "([^"]*)" y "([^"]*)" con sus instantes persistidos, sin atribuir ambos exclusivamente a "([^"]*)"$`, suite.operationChatMessagesAreConversationScoped)
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
	return suite.sendOperationChatGet("jr-"+strconv.Itoa(fixture.id), reason, correlation)
}

func (suite *testSuite) sendOperationChatGet(operationID, reason, correlation string) error {
	return suite.sendOperationChatGetWithQuery(operationID, reason, correlation, nil)
}
func (suite *testSuite) sendOperationChatGetWithQuery(operationID, reason, correlation string, query url.Values) error {
	path := suite.server.URL + operationsInboxPath + "/" + operationID + "/conversation"
	if len(query) > 0 {
		path += "?" + query.Encode()
	}
	request, err := http.NewRequest(http.MethodGet, path, nil)
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
	return suite.operationChatPageContains(first, second)
}
func (suite *testSuite) operationChatPageContains(labels ...string) error {
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
	if len(response.Messages) != len(labels) {
		return fmt.Errorf("expected %d messages, got %d", len(labels), len(response.Messages))
	}
	for index, label := range labels {
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

func (suite *testSuite) operationChatHasProposals(requestLabel string, table *godog.Table) error {
	rows, err := inboxTableRows(table)
	if err != nil {
		return err
	}
	return suite.withInboxFixtureClock(func() error {
		for _, row := range rows {
			created, err := parseInboxInstant(row["creada"])
			if err != nil {
				return err
			}
			status := serviceproposal.Status(row["estado"])
			if status != serviceproposal.StatusPending && status != serviceproposal.StatusAccepted {
				return fmt.Errorf("unsupported proposal fixture status %q", status)
			}
			label := row["propuesta"]
			if err := suite.createInboxServiceProposal(label, requestLabel, created, created.Add(72*time.Hour), 60, string(status)); err != nil {
				return err
			}
			fixture := suite.operationInbox.proposals[label]
			repository := suite.dependencies.Persistence.ServiceProposalRepository
			proposal, err := repository.FindByID(suite.scenarioContext, fixture.id)
			if err != nil {
				return err
			}
			if status == serviceproposal.StatusAccepted {
				if err := proposal.Accept(proposal.Consumer.ID(), created); err != nil {
					return err
				}
				if err := suite.dependencies.Persistence.PaymentUnitOfWork.Execute(suite.scenarioContext, func(store payment.TransactionalStore) error {
					return store.SaveServiceProposal(suite.scenarioContext, proposal)
				}); err != nil {
					return err
				}
			}
			persisted, err := repository.FindByID(suite.scenarioContext, fixture.id)
			if err != nil {
				return err
			}
			if persisted.Status != status || !persisted.CreatedOn.Equal(created) {
				return fmt.Errorf("proposal %q does not match persisted historical fixture", label)
			}
			// FindByID does not hydrate Conversation; the participant collection does.
			participantProposals, err := repository.FindByUserID(suite.scenarioContext, persisted.Consumer.ID())
			if err != nil {
				return err
			}
			request := suite.operationInbox.requests[requestLabel]
			associationVerified := false
			for _, participantProposal := range participantProposals {
				if participantProposal.ID != persisted.ID {
					continue
				}
				if participantProposal.Conversation == nil || participantProposal.Conversation.ID() != request.conversationID {
					return fmt.Errorf("proposal %q has incorrect persisted conversation association", label)
				}
				associationVerified = true
			}
			if !associationVerified {
				return fmt.Errorf("proposal %q is absent from persisted participant proposals", label)
			}
		}
		return nil
	})
}

func (suite *testSuite) queryProposalOperationChat(label, reason string) error {
	proposal, ok := suite.operationInbox.proposals[label]
	if !ok {
		return fmt.Errorf("unknown proposal %q", label)
	}
	return suite.sendOperationChatGet("sp-"+strconv.Itoa(proposal.id), reason, "")
}

func (suite *testSuite) operationChatIdentifiesSharedProposal(selectedLabel, conversationLabel, relatedLabel string) error {
	selected, ok := suite.operationInbox.proposals[selectedLabel]
	if !ok {
		return fmt.Errorf("unknown selected proposal %q", selectedLabel)
	}
	related, ok := suite.operationInbox.proposals[relatedLabel]
	if !ok {
		return fmt.Errorf("unknown related proposal %q", relatedLabel)
	}
	conversationID, ok := suite.operationChat.conversations[conversationLabel]
	if !ok {
		return fmt.Errorf("unknown conversation %q", conversationLabel)
	}
	var response struct {
		OperationID               string `json:"operation_id"`
		ConversationID            int    `json:"conversation_id"`
		JobRequestID              *int   `json:"job_request_id"`
		ServiceProposalID         *int   `json:"service_proposal_id"`
		RelatedServiceProposalIDs []int  `json:"related_service_proposal_ids"`
		SharedConversation        bool   `json:"shared_conversation"`
	}
	if err := json.Unmarshal(suite.lastBody, &response); err != nil {
		return err
	}
	request := suite.operationInbox.requests[selected.requestLabel]
	expected := []int{selected.id, related.id}
	slices.Sort(expected)
	if response.OperationID != "sp-"+strconv.Itoa(selected.id) || response.ServiceProposalID == nil || *response.ServiceProposalID != selected.id || response.ConversationID != conversationID || response.JobRequestID == nil || *response.JobRequestID != request.id || !response.SharedConversation || !slices.Equal(response.RelatedServiceProposalIDs, expected) {
		return fmt.Errorf("incorrect shared proposal association: %s", suite.lastBody)
	}
	return nil
}

func (suite *testSuite) operationChatMessagesAreConversationScoped(first, second, selectedLabel string) error {
	if _, ok := suite.operationInbox.proposals[selectedLabel]; !ok {
		return fmt.Errorf("unknown proposal %q", selectedLabel)
	}
	if err := suite.operationChatContainsMessages(first, second); err != nil {
		return err
	}
	var response struct {
		Messages []map[string]json.RawMessage `json:"messages"`
	}
	if err := json.Unmarshal(suite.lastBody, &response); err != nil {
		return err
	}
	allowed := map[string]bool{"id": true, "sender_role": true, "content": true, "created_on": true}
	for _, message := range response.Messages {
		for field := range message {
			if !allowed[field] {
				return fmt.Errorf("conversation message contains unexpected attribution or private field %q", field)
			}
		}
	}
	return nil
}

func (suite *testSuite) operationChatHasNoMessages(label string) error {
	conversationID, ok := suite.operationChat.conversations[label]
	if !ok {
		return fmt.Errorf("unknown conversation %q", label)
	}
	if err := (testsupport.OperationChatFixture{DB: suite.database}).DeleteMessages(suite.scenarioContext, conversationID); err != nil {
		return err
	}
	found, err := suite.conversationRepository.FindByID(suite.scenarioContext, conversationID)
	if err != nil {
		return err
	}
	if found.Status() != conversation.StatusActive || len(found.Messages()) != 0 {
		return fmt.Errorf("conversation fixture is not active and empty")
	}
	return nil
}
func (suite *testSuite) queryInitialOperationChat(label, reason string) error {
	fixture, ok := suite.operationInbox.requests[label]
	if !ok {
		return fmt.Errorf("unknown request %q", label)
	}
	return suite.sendOperationChatGet("jr-"+strconv.Itoa(fixture.id), reason, "")
}
func (suite *testSuite) operationChatPageIsEmpty() error {
	var response map[string]json.RawMessage
	if err := json.Unmarshal(suite.lastBody, &response); err != nil {
		return err
	}
	if string(response["messages"]) != "[]" || string(response["next_cursor"]) != "null" {
		return fmt.Errorf("expected empty non-null messages and explicit null next cursor, got %s", suite.lastBody)
	}
	return nil
}

func (suite *testSuite) thereIsUnrelatedOperationChat(label, requestLabel string) error {
	request, ok := suite.operationInbox.requests[requestLabel]
	if !ok {
		return fmt.Errorf("unknown request %q", requestLabel)
	}
	const consumerEmail = "unrelated-chat@example.com"
	if err := suite.thereIsRegisteredConsumerWithEmailNameAndSurname(consumerEmail, "Unrelated", "Consumer"); err != nil {
		return err
	}
	consumer, err := suite.userRepository.FindByAuthID(auth0IDForConsumerEmail(consumerEmail))
	if err != nil {
		return err
	}
	providerID, err := suite.providerIDByEmail(request.providerEmail)
	if err != nil {
		return err
	}
	unrelated, err := conversation.NewPendingConversation(consumer.ID(), providerID)
	if err != nil {
		return err
	}
	if err := unrelated.Activate(); err != nil {
		return err
	}
	persisted, err := suite.conversationRepository.SaveConversation(suite.scenarioContext, unrelated)
	if err != nil {
		return err
	}
	if persisted.ID() == request.conversationID {
		return fmt.Errorf("unrelated conversation unexpectedly matches request conversation")
	}
	content := "Private content belonging only to unrelated conversation " + label
	message, err := (testsupport.OperationChatFixture{DB: suite.database}).AddMessage(suite.scenarioContext, persisted.ID(), conversation.SenderConsumer, content, suite.clock.Now())
	if err != nil {
		return err
	}
	reloaded, err := suite.conversationRepository.FindByID(suite.scenarioContext, persisted.ID())
	if err != nil {
		return err
	}
	if reloaded.ConversationType() != conversation.TypeWork || len(reloaded.Messages()) != 1 || reloaded.Messages()[0].ID != message.ID || reloaded.Messages()[0].Content != content {
		return fmt.Errorf("unrelated conversation private message was not persisted")
	}
	linkedRequest, err := suite.jobRequestRepository.FindByID(request.id)
	if err != nil {
		return err
	}
	if linkedRequest.ConversationID != request.conversationID || linkedRequest.ConversationID == persisted.ID() {
		return fmt.Errorf("unrelated conversation is linked to target request")
	}
	suite.operationChat.conversations[label] = persisted.ID()
	suite.operationChat.messages[label+"-private"] = message
	return nil
}
func (suite *testSuite) queryOperationChatSelectingConversation(requestLabel, reason, conversationLabel string) error {
	request, ok := suite.operationInbox.requests[requestLabel]
	if !ok {
		return fmt.Errorf("unknown request %q", requestLabel)
	}
	conversationID, ok := suite.operationChat.conversations[conversationLabel]
	if !ok {
		return fmt.Errorf("unknown conversation %q", conversationLabel)
	}
	return suite.sendOperationChatGetWithQuery("jr-"+strconv.Itoa(request.id), reason, "", url.Values{"conversation_id": {strconv.Itoa(conversationID)}})
}
func (suite *testSuite) operationChatErrorDoesNotLeakUnrelatedConversation(label string) error {
	fixture, ok := suite.operationChat.messages[label+"-private"]
	if !ok || fixture.ID <= 0 || fixture.Content == "" {
		return fmt.Errorf("unrelated private message fixture is absent")
	}
	if strings.Contains(string(suite.lastBody), fixture.Content) || strings.Contains(string(suite.lastBody), "https://") || strings.Contains(string(suite.lastBody), "http://") {
		return fmt.Errorf("error response exposes private content or attachment URL")
	}
	return suite.adminDetailErrorHasNoData()
}
