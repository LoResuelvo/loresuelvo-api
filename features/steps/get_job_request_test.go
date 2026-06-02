package steps_test

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"

	"github.com/cucumber/godog"
)

type jobRequestListItemResponse struct {
	ID             int    `json:"id"`
	ConversationID int    `json:"conversation_id"`
	Title          string `json:"title"`
	Description    string `json:"description"`
}

func registerGetJobRequestSteps(sc *godog.ScenarioContext, suite *testSuite) {
	sc.Step(`^que existe una solicitud de trabajo pendiente para el consumidor "([^"]*)"$`, suite.thereIsPendingJobRequestForConsumer)
	sc.Step(`^que existe una solicitud de trabajo pendiente para el prestador "([^"]*)"$`, suite.thereIsPendingJobRequestForProvider)
	sc.Step(`^obtengo mis solicitudes de trabajo pendientes$`, suite.requestMyPendingJobRequests)
	sc.Step(`^el sistema me muestra una lista con (\d+) solicitudes pendientes$`, suite.systemShowsPendingJobRequestListWithCount)
}

func (suite *testSuite) thereIsPendingJobRequestForConsumer(consumerEmail string) error {
	providerEmail := "prestador.solicitud@example.com"
	if err := suite.thereIsRegisteredProviderWithEmailNameSurnameAndCategory(providerEmail, "Prestador", "Solicitud", "Plomería"); err != nil {
		return err
	}

	return suite.createPendingJobRequest(consumerEmail, providerEmail)
}

func (suite *testSuite) thereIsPendingJobRequestForProvider(providerEmail string) error {
	previousAuth0ID := suite.currentAuth0ID
	consumerEmail := "consumidor.solicitud@example.com"

	if err := suite.thereIsRegisteredConsumerWithEmailNameAndSurname(consumerEmail, "Consumidor", "Solicitud"); err != nil {
		return err
	}
	if err := suite.thereIsRegisteredProviderWithEmailNameSurnameAndCategory(providerEmail, "Prestador", "Solicitud", "Plomería"); err != nil {
		return err
	}
	if err := suite.createPendingJobRequest(consumerEmail, providerEmail); err != nil {
		return err
	}

	suite.currentAuth0ID = previousAuth0ID
	return nil
}

func (suite *testSuite) createPendingJobRequest(consumerEmail, providerEmail string) error {
	providerID, err := suite.providerIDByEmail(providerEmail)
	if err != nil {
		return err
	}

	suite.currentAuth0ID = auth0IDForConsumerEmail(consumerEmail)
	if err := suite.requestJobRequest(jobRequestCreationRequest{
		ProviderID:  providerID,
		Title:       "Reparación pendiente",
		Description: "Solicitud pendiente preparada para el escenario",
	}); err != nil {
		return err
	}

	if suite.lastStatus != http.StatusCreated && suite.lastStatus != http.StatusConflict {
		return fmt.Errorf("could not prepare pending job request: status %d, body %s", suite.lastStatus, string(suite.lastBody))
	}
	if suite.lastStatus == http.StatusCreated {
		createdJobRequest, err := suite.jobRequestCreationResponseFromLastBody()
		if err != nil {
			return err
		}
		suite.lastJobRequestID = createdJobRequest.ID
		suite.lastConversationID = createdJobRequest.ConversationID
	}

	return nil
}

func (suite *testSuite) requestMyPendingJobRequests() error {
	httpReq, err := http.NewRequest(http.MethodGet, suite.server.URL+"/job-requests", nil)
	if err != nil {
		return err
	}
	if suite.currentAuth0ID != "" {
		httpReq.Header.Set("Authorization", "Bearer "+suite.tokenBuilder.BuildToken(suite.currentAuth0ID, nil))
	}

	resp, err := http.DefaultClient.Do(httpReq)
	if err != nil {
		return fmt.Errorf("API connection failed: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return fmt.Errorf("failed to read response body: %w", err)
	}

	suite.lastStatus = resp.StatusCode
	suite.lastBody = body

	return nil
}

func (suite *testSuite) systemShowsPendingJobRequestListWithCount(expectedCount int) error {
	jobRequests, err := suite.jobRequestListResponseShouldHaveStatusCode(http.StatusOK)
	if err != nil {
		return err
	}

	if len(jobRequests) != expectedCount {
		return fmt.Errorf("expected %d pending job requests, got %d with body %s", expectedCount, len(jobRequests), string(suite.lastBody))
	}

	for _, jobRequest := range jobRequests {
		if jobRequest.ID == 0 {
			return fmt.Errorf("expected each job request to include id, got body %s", string(suite.lastBody))
		}
		if jobRequest.ConversationID == 0 {
			return fmt.Errorf("expected each job request to include conversation_id, got body %s", string(suite.lastBody))
		}
		if jobRequest.Title == "" {
			return fmt.Errorf("expected each job request to include title, got body %s", string(suite.lastBody))
		}
	}

	return nil
}

func (suite *testSuite) jobRequestListResponseShouldHaveStatusCode(statusCode int) ([]jobRequestListItemResponse, error) {
	if err := suite.lastResponseShouldHaveStatusCode(statusCode); err != nil {
		return nil, err
	}

	var jobRequests []jobRequestListItemResponse
	if err := json.Unmarshal(suite.lastBody, &jobRequests); err != nil {
		return nil, fmt.Errorf("response is not valid JSON job request list: %w", err)
	}

	return jobRequests, nil
}
