package steps_test

import (
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/LoResuelvo/loresuelvo-api/internal/testsupport"
	"github.com/cucumber/godog"
)

// Registered by registerAllSteps in suite_test.go. This file owns only the
// legacy absent-evidence scenario; other detail scenarios use their own steps.
func registerAdminOperationDetailAbsentSteps(sc *godog.ScenarioContext, suite *testSuite) {
	sc.Step(`^que no existen dirección, propuestas, órdenes, reportes, reviews ni referencias de pago persistidas para "([^"]*)"$`, suite.prepareDetailWithoutRelatedEvidence)
	sc.Step(`^consulto el detalle administrativo de la operación de la solicitud "([^"]*)"$`, suite.queryAdminJobRequestDetail)
	sc.Step(`^el detalle informa el ID estable "jr-" seguido del ID persistido de "([^"]*)" y conserva los datos persistidos de solicitud y partes$`, suite.absentDetailPreservesRequestAndParties)
	sc.Step(`^la evaluación de origen se informa no disponible porque no existe una referencia persistida desde la solicitud, sin inferir procedencia manual o histórica$`, suite.absentDetailHasNoSourceAssessment)
	sc.Step(`^la dirección, propuestas, órdenes, reportes, reviews y hitos de pago se informan como ausentes$`, suite.absentDetailHasNoRelatedEvidence)
	sc.Step(`^la cronología contiene únicamente eventos respaldados por datos persistidos y sus instantes se comparan como instantes exactos$`, suite.absentDetailHasOnlyPersistedTimeline)
	sc.Step(`^no se inventa evento de contacto, aceptación, finalización, pago ni referencia faltante$`, suite.absentDetailHasNoInventedEvents)
	sc.Step(`^la respuesta no contiene mensajes ni extractos de chat$`, suite.absentDetailHasNoChat)
}

func (suite *testSuite) prepareDetailWithoutRelatedEvidence(label string) error {
	fixture, ok := suite.operationInbox.requests[label]
	if !ok {
		return fmt.Errorf("unknown job request label %q", label)
	}
	request, err := suite.jobRequestRepository.FindByID(fixture.id)
	if err != nil {
		return fmt.Errorf("finding job request %q: %w", label, err)
	}
	if request.SourceAssessmentID != nil {
		return fmt.Errorf("request %q has a persisted source assessment", label)
	}
	proposals, err := suite.dependencies.Persistence.ServiceProposalRepository.FindByUserID(suite.scenarioContext, request.ConsumerID)
	if err != nil {
		return fmt.Errorf("finding consumer proposals: %w", err)
	}
	for _, proposal := range proposals {
		if proposal.Conversation.ID() == request.ConversationID {
			return fmt.Errorf("request %q has proposal %d", label, proposal.ID)
		}
	}
	// Payment references, orders, reports and reviews in this model are all
	// descendants of a proposal. There are none for this conversation.
	return testsupport.RemoveCurrentConsumerAddressForLegacyOperation(suite.scenarioContext, suite.database, request.ConsumerID)
}

func (suite *testSuite) absentDetailResponse() (map[string]json.RawMessage, error) {
	var detail map[string]json.RawMessage
	if err := json.Unmarshal(suite.lastBody, &detail); err != nil {
		return nil, fmt.Errorf("decoding operation detail: %w", err)
	}
	return detail, nil
}

func (suite *testSuite) absentDetailPreservesRequestAndParties(label string) error {
	fixture, ok := suite.operationInbox.requests[label]
	if !ok {
		return fmt.Errorf("unknown job request label %q", label)
	}
	persisted, err := suite.jobRequestRepository.FindByID(fixture.id)
	if err != nil {
		return err
	}
	detail, err := suite.absentDetailResponse()
	if err != nil {
		return err
	}
	var id string
	if err := json.Unmarshal(detail["id"], &id); err != nil || id != fmt.Sprintf("jr-%d", persisted.ID) {
		return fmt.Errorf("expected stable ID jr-%d, got %s", persisted.ID, detail["id"])
	}
	var request struct {
		ID          int       `json:"id"`
		Status      string    `json:"status"`
		Title       string    `json:"title"`
		Description string    `json:"description"`
		CreatedOn   time.Time `json:"created_on"`
	}
	if err := json.Unmarshal(detail["job_request"], &request); err != nil {
		return fmt.Errorf("decoding job request: %w", err)
	}
	if request.ID != persisted.ID || request.Status != string(persisted.Status) || request.Title != persisted.Title || request.Description != persisted.Description || !request.CreatedOn.Equal(persisted.CreatedOn) {
		return fmt.Errorf("detail job request differs from persisted request: %+v", request)
	}
	parties := []struct {
		field, name, surname string
		id                   int
	}{
		{"consumer", "Ana", "Pérez", persisted.ConsumerID},
		{"provider", "Juan", "Gómez", persisted.ProviderID},
	}
	for _, party := range parties {
		var found struct {
			ID      int    `json:"id"`
			Name    string `json:"name"`
			Surname string `json:"surname"`
		}
		if err := json.Unmarshal(detail[party.field], &found); err != nil {
			return fmt.Errorf("decoding %s: %w", party.field, err)
		}
		if found.ID != party.id || found.Name != party.name || found.Surname != party.surname {
			return fmt.Errorf("%s differs from persisted fixture: %+v", party.field, found)
		}
	}
	return nil
}

func (suite *testSuite) absentDetailHasNoSourceAssessment() error {
	detail, err := suite.absentDetailResponse()
	if err != nil {
		return err
	}
	if string(detail["source_assessment"]) != "null" {
		return fmt.Errorf("source_assessment must be null, got %s", detail["source_assessment"])
	}
	return nil
}

func (suite *testSuite) absentDetailHasNoRelatedEvidence() error {
	detail, err := suite.absentDetailResponse()
	if err != nil {
		return err
	}
	for _, field := range []string{"address", "service_proposal", "work_order"} {
		if string(detail[field]) != "null" {
			return fmt.Errorf("%s must be null, got %s", field, detail[field])
		}
	}
	for _, field := range []string{"related_proposals", "payment_milestones"} {
		if string(detail[field]) != "[]" {
			return fmt.Errorf("%s must be an empty non-null array, got %s", field, detail[field])
		}
	}
	// Completion report and review belong to work_order, which is null. They
	// must not be represented as fabricated top-level resources either.
	for _, field := range []string{"completion_report", "report", "review"} {
		if _, exists := detail[field]; exists {
			return fmt.Errorf("unexpected top-level %s without a work order", field)
		}
	}
	return nil
}

func (suite *testSuite) absentDetailHasOnlyPersistedTimeline() error {
	detail, err := suite.absentDetailResponse()
	if err != nil {
		return err
	}
	var events []struct {
		Type       string    `json:"type"`
		OccurredOn time.Time `json:"occurred_on"`
		SourceType string    `json:"source_type"`
		SourceID   string    `json:"source_id"`
	}
	if err := json.Unmarshal(detail["timeline"], &events); err != nil {
		return fmt.Errorf("decoding timeline: %w", err)
	}
	if len(events) != 1 {
		return fmt.Errorf("expected exactly one persisted event, got %s", detail["timeline"])
	}
	if len(suite.operationInbox.requests) != 1 {
		return fmt.Errorf("expected one request fixture, got %d", len(suite.operationInbox.requests))
	}
	for _, fixture := range suite.operationInbox.requests {
		event := events[0]
		if event.Type != "job_request_created" || event.SourceType != "job_request" || event.SourceID != fmt.Sprint(fixture.id) || !event.OccurredOn.Equal(fixture.createdOn) {
			return fmt.Errorf("timeline event is not exactly the persisted request creation: %+v", event)
		}
	}
	return nil
}

func (suite *testSuite) absentDetailHasNoInventedEvents() error {
	if err := suite.absentDetailHasOnlyPersistedTimeline(); err != nil {
		return err
	}
	return suite.absentDetailHasNoRelatedEvidence()
}

func (suite *testSuite) absentDetailHasNoChat() error {
	var detail any
	if err := json.Unmarshal(suite.lastBody, &detail); err != nil {
		return fmt.Errorf("decoding operation detail: %w", err)
	}
	var inspect func(any) error
	inspect = func(value any) error {
		switch typed := value.(type) {
		case map[string]any:
			for key, child := range typed {
				if key == "messages" || key == "message" || key == "chat" || key == "excerpt" || key == "conversation_content" {
					return fmt.Errorf("operation detail exposes chat field %q", key)
				}
				if err := inspect(child); err != nil {
					return err
				}
			}
		case []any:
			for _, child := range typed {
				if err := inspect(child); err != nil {
					return err
				}
			}
		}
		return nil
	}
	if err := inspect(detail); err != nil {
		return err
	}
	if strings.Contains(strings.ToLower(string(suite.lastBody)), "mensaje privado") {
		return fmt.Errorf("operation detail exposes private chat content")
	}
	return nil
}
