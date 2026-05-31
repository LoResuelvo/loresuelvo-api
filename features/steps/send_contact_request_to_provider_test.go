package steps_test

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"

	"github.com/LoResuelvo/loresuelvo-api/internal/domain/conversation"
	"github.com/cucumber/godog"
)

const (
	nonExistingProviderID = 999999999
)

type conversationCreationRequest struct {
	ProviderID int    `json:"provider_id"`
	Content    string `json:"content,omitempty"`
}

type conversationCreationResponse struct {
	ID     int    `json:"id"`
	Status string `json:"status"`
}

func registerSendContactRequestToProviderSteps(sc *godog.ScenarioContext, suite *testSuite) {
	sc.Step(`^que existe un consumidor registrado con correo "([^"]*)", nombre "([^"]*)" y apellido "([^"]*)"$`, suite.thereIsRegisteredConsumerWithEmailNameAndSurname)
	sc.Step(`^que no existe una conversación entre el consumidor "([^"]*)" y el prestador "([^"]*)"$`, suite.thereIsNoConversationBetweenConsumerAndProvider)
	sc.Step(`^que estoy autenticado como consumidor "([^"]*)"$`, suite.iAmAuthenticatedAsConsumer)
	sc.Step(`^que estoy autenticado como prestador "([^"]*)"$`, suite.iAmAuthenticatedAsProvider)
	sc.Step(`^envío una solicitud de trabajo al prestador "([^"]*)" con el mensaje:$`, suite.sendWorkRequestToProviderWithMessage)
	sc.Step(`^intento enviar una solicitud de trabajo a un prestador inexistente con el mensaje "([^"]*)"$`, suite.trySendWorkRequestToNonExistingProviderWithMessage)
	sc.Step(`^intento enviar una solicitud de trabajo al prestador "([^"]*)" sin mensaje inicial$`, suite.trySendWorkRequestToProviderWithoutInitialMessage)
	sc.Step(`^intento enviar una solicitud de trabajo al prestador "([^"]*)" con el mensaje "([^"]*)"$`, suite.trySendWorkRequestToProviderWithMessage)
	sc.Step(`^el sistema crea una conversación pendiente entre el consumidor "([^"]*)" y el prestador "([^"]*)"$`, suite.systemCreatesPendingConversationBetweenConsumerAndProvider)
	sc.Step(`^la conversación contiene el mensaje inicial:$`, suite.conversationContainsInitialMessage)
	sc.Step(`^el sistema me indica que el prestador no existe$`, suite.systemReportsProviderDoesNotExist)
	sc.Step(`^el sistema me indica que el mensaje es obligatorio$`, suite.systemReportsMessageIsRequired)
	sc.Step(`^el sistema me indica que solo un consumidor puede iniciar una solicitud de trabajo$`, suite.systemReportsOnlyConsumerCanStartWorkRequest)
	sc.Step(`^que existe una conversación pendiente entre el consumidor "([^"]*)" y el prestador "([^"]*)"$`, suite.thereIsPendingConversationBetweenConsumerAndProvider)
	sc.Step(`^el sistema me indica que ya existe una conversación con ese prestador$`, suite.systemReportsConversationWithProviderAlreadyExists)
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
		suite.rememberParticipantFullName(name, surname, conversation.SenderConsumer)
		return nil
	}

	body, _ := io.ReadAll(resp.Body)
	return fmt.Errorf("could not prepare registered consumer: status %d, body %s", resp.StatusCode, string(body))
}

func (suite *testSuite) thereIsNoConversationBetweenConsumerAndProvider(consumerEmail, providerEmail string) error {
	consumerID, err := suite.consumerRepository.FindIDByEmail(consumerEmail)
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

func (suite *testSuite) sendWorkRequestToProviderWithMessage(providerEmail string, message *godog.DocString) error {
	return suite.requestWorkRequestToProvider(providerEmail, stringPointer(normalizeDocString(message)))
}

func (suite *testSuite) trySendWorkRequestToNonExistingProviderWithMessage(message string) error {
	return suite.requestWorkRequest(conversationCreationRequest{
		ProviderID: nonExistingProviderID,
		Content:    message,
	})
}

func (suite *testSuite) trySendWorkRequestToProviderWithoutInitialMessage(providerEmail string) error {
	providerID, err := suite.providerIDByEmail(providerEmail)
	if err != nil {
		return err
	}

	return suite.requestWorkRequest(map[string]any{
		"provider_id": providerID,
	})
}

func (suite *testSuite) trySendWorkRequestToProviderWithMessage(providerEmail, message string) error {
	return suite.requestWorkRequestToProvider(providerEmail, &message)
}

func (suite *testSuite) systemCreatesPendingConversationBetweenConsumerAndProvider(consumerEmail, providerEmail string) error {
	if err := suite.lastResponseShouldHaveStatusCode(http.StatusCreated); err != nil {
		return err
	}

	response, err := suite.conversationCreationResponseFromLastBody()
	if err != nil {
		return err
	}

	if response.ID == 0 {
		return fmt.Errorf("expected created conversation id, got body %s", string(suite.lastBody))
	}

	if strings.TrimSpace(response.Status) != conversation.StatusPending {
		return fmt.Errorf("expected conversation status %q, got %q", conversation.StatusPending, response.Status)
	}

	suite.lastConversationID = response.ID

	consumerID, err := suite.consumerRepository.FindIDByEmail(consumerEmail)
	if err != nil {
		return err
	}

	providerID, err := suite.providerIDByEmail(providerEmail)
	if err != nil {
		return err
	}

	createdConversation, err := suite.conversationRepository.FindBetween(consumerID, providerID)
	if err != nil {
		return err
	}

	if createdConversation.ID != response.ID {
		return fmt.Errorf("expected persisted conversation id %d, got %d", response.ID, createdConversation.ID)
	}

	if createdConversation.Status != conversation.StatusPending {
		return fmt.Errorf("expected persisted conversation status %q, got %q", conversation.StatusPending, createdConversation.Status)
	}

	return nil
}

func (suite *testSuite) conversationContainsInitialMessage(message *godog.DocString) error {
	content := normalizeDocString(message)
	if strings.TrimSpace(content) == "" {
		return fmt.Errorf("expected initial message content to be non-empty")
	}
	if suite.lastConversationID == 0 {
		return fmt.Errorf("expected a previously created conversation before checking its initial message")
	}

	exists, err := suite.messageRepository.ExistsInConversation(suite.lastConversationID, content)
	if err != nil {
		return err
	}

	if !exists {
		return fmt.Errorf("expected initial message %q in conversation %d", content, suite.lastConversationID)
	}

	return nil
}

func (suite *testSuite) systemReportsProviderDoesNotExist() error {
	return suite.conversationRequestShouldFailWithStatus(http.StatusNotFound)
}

func (suite *testSuite) systemReportsMessageIsRequired() error {
	return suite.conversationRequestShouldFailWithStatus(http.StatusBadRequest)
}

func (suite *testSuite) systemReportsOnlyConsumerCanStartWorkRequest() error {
	return suite.conversationRequestShouldFailWithStatus(http.StatusForbidden)
}

func (suite *testSuite) thereIsPendingConversationBetweenConsumerAndProvider(consumerEmail, providerEmail string) error {
	return suite.createPendingConversationBetweenConsumerAndProvider(consumerEmail, providerEmail, "Existing work request")
}

func (suite *testSuite) systemReportsConversationWithProviderAlreadyExists() error {
	if err := suite.conversationRequestShouldFailWithStatus(http.StatusConflict); err != nil {
		return err
	}

	consumerID, err := suite.consumerRepository.FindIDByAuthID(suite.currentAuth0ID)
	if err != nil {
		return err
	}

	if suite.lastWorkRequestProviderID == 0 {
		return fmt.Errorf("expected attempted provider id to be recorded before duplicate rejection assertion")
	}

	exists, err := suite.conversationRepository.ExistsBetween(consumerID, suite.lastWorkRequestProviderID)
	if err != nil {
		return err
	}
	if !exists {
		return fmt.Errorf("expected existing conversation after duplicate rejection")
	}

	return nil
}

func (suite *testSuite) requestWorkRequestToProvider(providerEmail string, content *string) error {
	providerID, err := suite.providerIDByEmail(providerEmail)
	if err != nil {
		return err
	}

	suite.lastWorkRequestProviderID = providerID
	payload := conversationCreationRequest{ProviderID: providerID}
	if content != nil {
		payload.Content = *content
	}

	return suite.requestWorkRequest(payload)
}

func (suite *testSuite) requestWorkRequest(payload any) error {
	body, err := json.Marshal(payload)
	if err != nil {
		return err
	}

	httpReq, err := http.NewRequest(http.MethodPost, suite.server.URL+"/conversations", bytes.NewReader(body))
	if err != nil {
		return err
	}
	httpReq.Header.Set("Content-Type", "application/json")
	if suite.currentAuth0ID != "" {
		httpReq.Header.Set("Authorization", "Bearer "+suite.tokenBuilder.BuildToken(suite.currentAuth0ID, nil))
	}

	resp, err := http.DefaultClient.Do(httpReq)
	if err != nil {
		return fmt.Errorf("API connection failed: %w", err)
	}
	defer resp.Body.Close()

	responseBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return fmt.Errorf("failed to read response body: %w", err)
	}

	suite.lastStatus = resp.StatusCode
	suite.lastBody = responseBody

	return nil
}

func (suite *testSuite) conversationCreationResponseFromLastBody() (conversationCreationResponse, error) {
	var response conversationCreationResponse
	if err := json.Unmarshal(suite.lastBody, &response); err != nil {
		return conversationCreationResponse{}, fmt.Errorf("response is not valid JSON conversation creation response: %w", err)
	}

	return response, nil
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
	return suite.providerRepository.FindIDByEmail(email)
}

func normalizeDocString(docString *godog.DocString) string {
	if docString == nil {
		return ""
	}

	return strings.TrimSpace(docString.Content)
}

func stringPointer(value string) *string {
	return &value
}

func auth0IDForConsumerEmail(email string) string {
	return auth0IDForEmail("consumer-contact", email)
}
