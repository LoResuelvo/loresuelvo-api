package steps_test

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"

	"github.com/cucumber/godog"
)

type jobRequestCreationRequest struct {
	ProviderID  int    `json:"provider_id"`
	Title       string `json:"title,omitempty"`
	Description string `json:"description,omitempty"`
}

type jobRequestCreationResponse struct {
	ID             int    `json:"id"`
	ConversationID int    `json:"conversation_id"`
	Title          string `json:"title"`
	Description    string `json:"description"`
}

func registerPostJobRequestSteps(sc *godog.ScenarioContext, suite *testSuite) {
	sc.Step(`^envío una solicitud de trabajo al prestador "([^"]*)" con el título "([^"]*)" y la descripción:$`, suite.sendJobRequestToProviderWithTitleAndDescription)
	sc.Step(`^envío una solicitud de trabajo al prestador "([^"]*)" sin título y con la descripción:$`, suite.sendJobRequestToProviderWithoutTitleAndDescription)
	sc.Step(`^envío una solicitud de trabajo al prestador "([^"]*)" con el título "([^"]*)" y sin descripción$`, suite.sendJobRequestToProviderWithTitleAndNoDescription)
	sc.Step(`^el sistema registra la solicitud de trabajo$`, suite.systemRegistersJobRequest)
	sc.Step(`^el sistema registra la solicitud de trabajo con una descripción vacía$`, suite.systemRegistersJobRequestWithEmptyDescription)
	sc.Step(`^el sistema muestra un mensaje de error indicando que el título es obligatorio$`, suite.systemReportsTitleIsRequired)
	sc.Step(`^el sistema muestra un mensaje de error indicando que el prestador no existe$`, suite.systemReportsProviderDoesNotExist)
	sc.Step(`^envio al chat pendiente con el prestador "([^"]*)" los mensajes:$`, suite.sendMessagesToPendingChatWithProvider)
	sc.Step(`^el sistema muestra un mensaje de error indicando que se ha alcanzado el límite de mensajes permitidos en el chat pendiente$`, suite.systemReportsPendingConversationMessageLimitReached)
	sc.Step(`^el sistema no registra el sexto mensaje en la conversación pendiente$`, suite.systemDoesNotRegisterSixthPendingConversationMessage)
	sc.Step(`^el prestador "([^"]*)" intenta enviar un mensaje en el chat pendiente con el consumidor "([^"]*)" sin aceptar la solicitud de trabajo vinculada$`, suite.providerTriesToSendMessageInPendingChatWithoutAccepting)
	sc.Step(`^el sistema muestra un mensaje de error indicando que no se puede enviar mensajes en el chat pendiente sin aceptar la solicitud de trabajo vinculada$`, suite.systemReportsProviderCannotMessagePendingConversation)
}

func (suite *testSuite) sendJobRequestToProviderWithTitleAndDescription(providerFullName, title string, description *godog.DocString) error {
	return suite.requestJobRequestToProviderFullName(providerFullName, jobRequestPayload{
		title:       title,
		description: normalizeDocString(description),
	})
}

func (suite *testSuite) sendJobRequestToProviderWithoutTitleAndDescription(providerFullName string, description *godog.DocString) error {
	return suite.requestJobRequestToProviderFullName(providerFullName, jobRequestPayload{
		description: normalizeDocString(description),
	})
}

func (suite *testSuite) sendJobRequestToProviderWithTitleAndNoDescription(providerFullName, title string) error {
	return suite.requestJobRequestToProviderFullName(providerFullName, jobRequestPayload{title: title})
}

func (suite *testSuite) systemRegistersJobRequest() error {
	if err := suite.lastResponseShouldHaveStatusCode(http.StatusCreated); err != nil {
		return err
	}

	response, err := suite.jobRequestCreationResponseFromLastBody()
	if err != nil {
		return err
	}
	if response.ID == 0 {
		return fmt.Errorf("expected created job request id, got body %s", string(suite.lastBody))
	}
	if response.ConversationID == 0 {
		return fmt.Errorf("expected linked conversation id, got body %s", string(suite.lastBody))
	}

	suite.lastConversationID = response.ConversationID
	return nil
}

func (suite *testSuite) systemRegistersJobRequestWithEmptyDescription() error {
	if err := suite.systemRegistersJobRequest(); err != nil {
		return err
	}

	foundJobRequest, err := suite.jobRequestRepository.FindByConversationID(suite.lastConversationID)
	if err != nil {
		return err
	}
	if foundJobRequest.Description != "" {
		return fmt.Errorf("expected empty job request description, got %q", foundJobRequest.Description)
	}

	return nil
}

func (suite *testSuite) systemReportsTitleIsRequired() error {
	return suite.conversationRequestShouldFailWithStatus(http.StatusBadRequest)
}

func (suite *testSuite) sendMessagesToPendingChatWithProvider(providerFullName string, messages *godog.DocString) error {
	if err := suite.ensureKnownParticipantFullName(providerFullName, participantRoleProvider); err != nil {
		return err
	}
	for _, content := range splitPendingChatMessages(messages.Content) {
		if err := suite.requestSendMessageToPreparedConversation(sendMessageRequest{Content: content}); err != nil {
			return err
		}
	}

	return nil
}

func (suite *testSuite) systemReportsPendingConversationMessageLimitReached() error {
	return suite.conversationRequestShouldFailWithStatus(http.StatusConflict)
}

func (suite *testSuite) systemDoesNotRegisterSixthPendingConversationMessage() error {
	count, err := suite.messageRepository.CountByConversationIDAndSenderRole(suite.lastConversationID, participantRoleConsumer)
	if err != nil {
		return err
	}
	if count != pendingConsumerMessageLimit {
		return fmt.Errorf("expected %d consumer messages in pending conversation, got %d", pendingConsumerMessageLimit, count)
	}

	return nil
}

func (suite *testSuite) providerTriesToSendMessageInPendingChatWithoutAccepting(providerFullName, consumerName string) error {
	if err := suite.ensureKnownParticipantFullName(providerFullName, participantRoleProvider); err != nil {
		return err
	}

	providerAuthID, err := suite.authIDForProviderFullName(providerFullName)
	if err != nil {
		return err
	}
	suite.currentAuth0ID = providerAuthID

	return suite.requestSendMessageToPreparedConversation(sendMessageRequest{
		Content: fmt.Sprintf("Hola %s, todavía no acepté la solicitud.", strings.TrimSpace(consumerName)),
	})
}

func (suite *testSuite) systemReportsProviderCannotMessagePendingConversation() error {
	return suite.conversationRequestShouldFailWithStatus(http.StatusForbidden)
}

type jobRequestPayload struct {
	title       string
	description string
}

func (suite *testSuite) requestJobRequestToProviderFullName(providerFullName string, payload jobRequestPayload) error {
	providerID, err := suite.providerIDByFullName(providerFullName)
	if err != nil {
		providerID = nonExistingProviderID
	}

	suite.lastWorkRequestProviderID = providerID
	request := jobRequestCreationRequest{
		ProviderID:  providerID,
		Title:       payload.title,
		Description: payload.description,
	}

	return suite.requestJobRequest(request)
}

func (suite *testSuite) providerIDByFullName(fullName string) (int, error) {
	parts := strings.Fields(fullName)
	if len(parts) < 2 {
		return 0, fmt.Errorf("provider full name must include name and surname")
	}

	var providerID int
	err := suite.database.QueryRow(
		`SELECT providers.id
		FROM providers
		INNER JOIN users ON users.id = providers.user_id
		WHERE users.name = $1 AND users.surname = $2`,
		parts[0],
		strings.Join(parts[1:], " "),
	).Scan(&providerID)
	if err != nil {
		return 0, fmt.Errorf("finding provider id by full name %q: %w", fullName, err)
	}

	return providerID, nil
}

func (suite *testSuite) authIDForProviderFullName(fullName string) (string, error) {
	parts := strings.Fields(fullName)
	if len(parts) < 2 {
		return "", fmt.Errorf("provider full name must include name and surname")
	}

	var authID string
	err := suite.database.QueryRow(
		`SELECT users.auth_id
		FROM providers
		INNER JOIN users ON users.id = providers.user_id
		WHERE users.name = $1 AND users.surname = $2`,
		parts[0],
		strings.Join(parts[1:], " "),
	).Scan(&authID)
	if err != nil {
		return "", fmt.Errorf("finding provider auth id by full name %q: %w", fullName, err)
	}

	return authID, nil
}

func splitPendingChatMessages(content string) []string {
	parts := strings.Split(content, "\n")
	messages := make([]string, 0, len(parts))
	for _, part := range parts {
		trimmed := strings.TrimSpace(part)
		if trimmed == "" {
			continue
		}
		messages = append(messages, trimmed)
	}

	return messages
}

func (suite *testSuite) requestJobRequest(payload any) error {
	body, err := json.Marshal(payload)
	if err != nil {
		return err
	}

	httpReq, err := http.NewRequest(http.MethodPost, suite.server.URL+"/job-requests", bytes.NewReader(body))
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

func (suite *testSuite) jobRequestCreationResponseFromLastBody() (jobRequestCreationResponse, error) {
	var response jobRequestCreationResponse
	if err := json.Unmarshal(suite.lastBody, &response); err != nil {
		return jobRequestCreationResponse{}, fmt.Errorf("response is not valid JSON job request creation response: %w", err)
	}

	return response, nil
}
