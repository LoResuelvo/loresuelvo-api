package steps_test

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"

	"github.com/cucumber/godog"
)

const (
	nonExistingProviderID = 999999999
)

func registerSendContactRequestToProviderSteps(sc *godog.ScenarioContext, suite *testSuite) {
	sc.Step(`^que existe un consumidor registrado con correo "([^"]*)", nombre "([^"]*)" y apellido "([^"]*)"$`, suite.thereIsRegisteredConsumerWithEmailNameAndSurname)
	sc.Step(`^que no existe una conversación entre el consumidor "([^"]*)" y el prestador "([^"]*)"$`, suite.thereIsNoConversationBetweenConsumerAndProvider)
	sc.Step(`^que estoy autenticado como consumidor "([^"]*)"$`, suite.iAmAuthenticatedAsConsumer)
	sc.Step(`^que estoy autenticado como prestador "([^"]*)"$`, suite.iAmAuthenticatedAsProvider)
	sc.Step(`^el sistema me indica que el mensaje es obligatorio$`, suite.systemReportsMessageIsRequired)
}

func (suite *testSuite) thereIsRegisteredConsumerWithEmailNameAndSurname(email, name, surname string) error {
	resp, err := suite.postConsumerRegistrationWithAuth0ID(auth0IDForConsumerEmail(email), consumerRegistrationRequest{
		Email:   email,
		Name:    name,
		Surname: surname,
	})
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusCreated || resp.StatusCode == http.StatusConflict {
		suite.rememberParticipantFullName(name, surname, participantRoleConsumer)
		return nil
	}

	body, _ := io.ReadAll(resp.Body)
	return fmt.Errorf("could not prepare registered consumer: status %d, body %s", resp.StatusCode, string(body))
}

func (suite *testSuite) thereIsNoConversationBetweenConsumerAndProvider(consumerEmail, providerEmail string) error {
	consumerID, err := suite.userRepository.FindIDByEmail(consumerEmail)
	if err != nil {
		return err
	}

	providerID, err := suite.providerIDByEmail(providerEmail)
	if err != nil {
		return err
	}

	return suite.conversationRepository.DeleteBetween(consumerID, providerID)
}

func (suite *testSuite) iAmAuthenticatedAsConsumer(email string) error {
	suite.currentAuth0ID = auth0IDForConsumerEmail(email)
	return nil
}

func (suite *testSuite) iAmAuthenticatedAsProvider(email string) error {
	suite.currentAuth0ID = auth0IDForProviderEmail(email)
	return nil
}

func (suite *testSuite) systemReportsProviderDoesNotExist() error {
	return suite.conversationRequestShouldFailWithStatus(http.StatusNotFound)
}

func (suite *testSuite) systemReportsMessageIsRequired() error {
	return suite.conversationRequestShouldFailWithStatus(http.StatusBadRequest)
}

func (suite *testSuite) conversationRequestShouldFailWithStatus(statusCode int) error {
	if err := suite.lastResponseShouldHaveStatusCode(statusCode); err != nil {
		return err
	}

	return suite.lastResponseShouldHaveError()
}

func (suite *testSuite) lastResponseShouldHaveStatusCode(statusCode int) error {
	if suite.lastStatus != statusCode {
		return fmt.Errorf("expected status code %d, got %d with body %s", statusCode, suite.lastStatus, string(suite.lastBody))
	}

	return nil
}

func (suite *testSuite) lastResponseShouldHaveError() error {
	var response registrationResponse
	if err := json.Unmarshal(suite.lastBody, &response); err != nil {
		return fmt.Errorf("response is not valid JSON: %w", err)
	}

	if strings.TrimSpace(response.Error) == "" {
		return fmt.Errorf("expected an error response, got body %s", string(suite.lastBody))
	}

	return nil
}

func (suite *testSuite) providerIDByEmail(email string) (int, error) {
	return suite.userRepository.FindIDByEmail(email)
}

func normalizeDocString(docString *godog.DocString) string {
	if docString == nil {
		return ""
	}

	return strings.TrimSpace(docString.Content)
}

func auth0IDForConsumerEmail(email string) string {
	return auth0IDForEmail("consumer-contact", email)
}
