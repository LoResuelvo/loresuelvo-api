package steps_test

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"strconv"

	jobrequest "github.com/LoResuelvo/loresuelvo-api/internal/domain/job_request"
	serviceproposal "github.com/LoResuelvo/loresuelvo-api/internal/domain/service_proposal"
	"github.com/cucumber/godog"
)

func registerAdminOperationDetailSteps(sc *godog.ScenarioContext, suite *testSuite) {
	sc.Step(`^que existe la siguiente solicitud de trabajo sin referencia persistida a una evaluación de origen:$`, suite.thereIsDetailJobRequestWithoutAssessment)
	sc.Step(`^que no existe ninguna solicitud con ID persistido (\d+) ni propuesta con ID persistido (\d+)$`, suite.thereIsNoDetailResourceWithIDs)
	sc.Step(`^consulto el detalle administrativo de la operación "([^"]*)"$`, suite.queryAdminOperationDetail)
	sc.Step(`^intento consultar el detalle administrativo de la operación de la solicitud "([^"]*)"$`, suite.queryAdminJobRequestDetail)
	sc.Step(`^la respuesta no contiene datos del detalle$`, suite.adminDetailErrorHasNoData)
}

func (suite *testSuite) thereIsDetailJobRequestWithoutAssessment(table *godog.Table) error {
	rows, err := inboxTableRows(table)
	if err != nil {
		return err
	}
	if len(rows) != 1 {
		return fmt.Errorf("expected exactly one job request, got %d", len(rows))
	}
	row := rows[0]
	createdOn, err := parseInboxInstant(row["creada"])
	if err != nil {
		return err
	}
	suite.operationInbox.ensureMaps()
	return suite.withInboxFixtureClock(func() error {
		if row["estado"] != "pending" {
			return fmt.Errorf("unsupported job request status %q", row["estado"])
		}
		providerID, err := suite.providerIDByEmail(row["prestador"])
		if err != nil {
			return err
		}
		if err := suite.setInboxFixtureClock(createdOn); err != nil {
			return err
		}
		suite.currentAuth0ID = auth0IDForConsumerEmail(row["consumidor"])
		if err := suite.requestJobRequest(jobRequestCreationRequest{
			ProviderID: providerID, Title: row["título"], Description: row["descripción"],
		}); err != nil {
			return err
		}
		if suite.lastStatus != http.StatusCreated {
			return fmt.Errorf("creating job request %q returned %d: %s", row["solicitud"], suite.lastStatus, suite.lastBody)
		}
		created, err := suite.jobRequestCreationResponseFromLastBody()
		if err != nil {
			return err
		}
		suite.operationInbox.requests[row["solicitud"]] = inboxJobRequestFixture{
			createdOn: createdOn, id: created.ID, conversationID: created.ConversationID,
			consumerEmail: row["consumidor"], providerEmail: row["prestador"],
		}
		return nil
	})
}

func (suite *testSuite) thereIsNoDetailResourceWithIDs(requestIDText, proposalIDText string) error {
	requestID, err := strconv.Atoi(requestIDText)
	if err != nil {
		return err
	}
	proposalID, err := strconv.Atoi(proposalIDText)
	if err != nil {
		return err
	}
	_, err = suite.jobRequestRepository.FindByID(requestID)
	if !errors.Is(err, jobrequest.ErrJobRequestNotFound) {
		return fmt.Errorf("expected job request %d to be absent, got %v", requestID, err)
	}
	_, err = suite.dependencies.Persistence.ServiceProposalRepository.FindByID(suite.scenarioContext, proposalID)
	if !errors.Is(err, serviceproposal.ErrDoesNotExist) {
		return fmt.Errorf("expected service proposal %d to be absent, got %v", proposalID, err)
	}
	return nil
}

func (suite *testSuite) queryAdminJobRequestDetail(label string) error {
	request, exists := suite.operationInbox.requests[label]
	if !exists {
		return fmt.Errorf("unknown job request label %q", label)
	}
	return suite.queryAdminOperationDetail(fmt.Sprintf("jr-%d", request.id))
}

func (suite *testSuite) queryAdminOperationDetail(id string) error {
	return suite.sendAdminGet(operationsInboxPath+"/"+url.PathEscape(id), nil, "")
}

func (suite *testSuite) adminDetailErrorHasNoData() error {
	var raw map[string]json.RawMessage
	if err := json.Unmarshal(suite.lastBody, &raw); err != nil {
		return fmt.Errorf("invalid detail error response JSON: %w", err)
	}
	if len(raw) == 0 || raw["error"] == nil {
		return fmt.Errorf("detail error response has no error: %s", suite.lastBody)
	}
	for field := range raw {
		if field != "error" && field != "message" {
			return fmt.Errorf("detail error response exposes field %q", field)
		}
	}
	return nil
}
