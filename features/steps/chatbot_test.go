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

const participantRoleChatbot = "chatbot"

type chatbotConversationRequest struct {
	Content      string   `json:"content"`
	ImageFileIDs []string `json:"image_file_ids,omitempty"`
}

type chatbotConversationResponse struct {
	ID                  int                           `json:"id"`
	Status              string                        `json:"status"`
	Title               string                        `json:"title"`
	ResponseStatus      string                        `json:"response_status"`
	DiagnosisCompleted  bool                          `json:"diagnosis_completed"`
	RecommendedCategory recommendedCategoryDetail     `json:"recommended_category"`
	Messages            []conversationMessageResponse `json:"messages"`
	Response            *conversationMessageResponse  `json:"response"`
}

func registerChatbotSteps(sc *godog.ScenarioContext, suite *testSuite) {
	sc.Step(`^que el chatbot asistido por IA está disponible$`, suite.chatbotIsAvailable)
	sc.Step(`^que no existe una conversación con el chatbot asistido por IA para el consumidor "([^"]*)"$`, suite.thereIsNoChatbotConversationForConsumer)
	sc.Step(`^que el chatbot asistido por IA responderá:$`, suite.chatbotWillRespond)
	sc.Step(`^que el chatbot asistido por IA responderá con un pre diagnóstico:$`, suite.chatbotWillRespondWithPreDiagnosis)
	sc.Step(`^envío un mensaje al chatbot asistido por IA:$`, suite.sendMessageToChatbot)
	sc.Step(`^intento enviar un mensaje al chatbot asistido por IA:$`, suite.trySendMessageToChatbot)
	sc.Step(`^intento enviar una pregunta fuera del ámbito del problema del hogar al chatbot asistido por IA:$`, suite.trySendOutOfScopeQuestionToChatbot)
	sc.Step(`^ya tengo una conversación activa con el chatbot$`, suite.iAlreadyHaveActiveChatbotConversation)
	sc.Step(`^inicio una nueva conversacion enviando un mensaje al chatbot asistido por IA:$`, suite.startNewChatbotConversationWithMessage)
	sc.Step(`^el sistema crea una conversación con el chatbot asistido por IA para el consumidor "([^"]*)"$`, suite.systemCreatesChatbotConversationForConsumer)
	sc.Step(`^el sistema crea una nueva conversación con el chatbot asistido por IA para el nuevo mensaje$`, suite.systemCreatesNewChatbotConversationForNewMessage)
	sc.Step(`^tengo un total de dos conversaciones activas con el chatbot asistido por IA$`, suite.iHaveTwoActiveChatbotConversations)
	sc.Step(`^el sistema crea una conversación con el chatbot asistido por IA con el título "([^"]*)"$`, suite.systemCreatesChatbotConversationWithTitle)
	sc.Step(`^la conversación contiene mi mensaje:$`, suite.chatbotConversationContainsMyMessage)
	sc.Step(`^el sistema muestra la respuesta del chatbot asistido por IA:$`, suite.systemShowsChatbotResponse)
	sc.Step(`^el sistema registra mi mensaje en la conversación con el chatbot asistido por IA:$`, suite.systemRegistersMyMessageInChatbotConversation)
	sc.Step(`^el sistema registra la respuesta del chatbot asistido por IA:$`, suite.systemRegistersChatbotResponse)
	sc.Step(`^el sistema me indica que solo un consumidor puede enviar mensajes al chatbot asistido por IA$`, suite.systemReportsOnlyConsumerCanMessageChatbot)
	sc.Step(`^el sistema no crea una conversación con el chatbot asistido por IA$`, suite.systemDoesNotCreateChatbotConversation)
	sc.Step(`^el sistema muestra un pre diagnóstico del problema del hogar:$`, suite.systemShowsPreDiagnosis)
	sc.Step(`^el sistema me indica que el chatbot solo responde preguntas relacionadas con problemas del hogar$`, suite.systemReportsChatbotScopeRestriction)
}

func (suite *testSuite) chatbotIsAvailable() error {
	suite.chatbot.Reset()
	suite.chatbotConversationIDs = nil
	suite.chatbotConversationStatuses = nil
	return nil
}

func (suite *testSuite) thereIsNoChatbotConversationForConsumer(consumerEmail string) error {
	if _, err := suite.consumerRepository.FindIDByEmail(consumerEmail); err != nil {
		return err
	}

	suite.lastConversationID = 0
	suite.chatbotConversationIDs = nil
	suite.chatbotConversationStatuses = nil
	return nil
}

func (suite *testSuite) chatbotWillRespond(message *godog.DocString) error {
	suite.chatbot.SetResponse("Pérdida de agua en la cocina", normalizeDocString(message))
	return nil
}

func (suite *testSuite) chatbotWillRespondWithPreDiagnosis(message *godog.DocString) error {
	return suite.chatbotWillRespond(message)
}

func (suite *testSuite) sendMessageToChatbot(message *godog.DocString) error {
	return suite.requestCreateChatbotConversation(chatbotConversationRequest{
		Content: normalizeDocString(message),
	})
}

func (suite *testSuite) trySendMessageToChatbot(message *godog.DocString) error {
	return suite.sendMessageToChatbot(message)
}

func (suite *testSuite) trySendOutOfScopeQuestionToChatbot(message *godog.DocString) error {
	suite.chatbot.SetOutOfScopeResponse("Consulta fuera de alcance", "Solo puedo responder consultas relacionadas con problemas del hogar.")
	return suite.requestCreateChatbotConversation(chatbotConversationRequest{
		Content: normalizeDocString(message),
	})
}

func (suite *testSuite) iAlreadyHaveActiveChatbotConversation() error {
	if err := suite.requestCreateChatbotConversation(chatbotConversationRequest{
		Content: "Tengo una pérdida de agua en el baño y necesito orientación.",
	}); err != nil {
		return err
	}

	return suite.rememberCreatedChatbotConversation()
}

func (suite *testSuite) startNewChatbotConversationWithMessage(message *godog.DocString) error {
	return suite.sendMessageToChatbot(message)
}

func (suite *testSuite) systemCreatesChatbotConversationForConsumer(consumerEmail string) error {
	if _, err := suite.consumerRepository.FindIDByEmail(consumerEmail); err != nil {
		return err
	}

	return suite.rememberCreatedChatbotConversation()
}

func (suite *testSuite) systemCreatesNewChatbotConversationForNewMessage() error {
	previousConversationID := suite.lastConversationID

	if err := suite.rememberCreatedChatbotConversation(); err != nil {
		return err
	}

	if previousConversationID != 0 && suite.lastConversationID == previousConversationID {
		return fmt.Errorf("expected a new chatbot conversation id different from %d", previousConversationID)
	}

	return nil
}

func (suite *testSuite) iHaveTwoActiveChatbotConversations() error {
	activeConversationIDs := uniqueActiveChatbotConversationIDs(suite.chatbotConversationIDs, suite.chatbotConversationStatuses)
	if len(activeConversationIDs) != 2 {
		return fmt.Errorf("expected 2 active chatbot conversations, got ids %v with statuses %v", suite.chatbotConversationIDs, suite.chatbotConversationStatuses)
	}

	return nil
}

func (suite *testSuite) systemCreatesChatbotConversationWithTitle(expectedTitle string) error {
	if err := suite.rememberCreatedChatbotConversation(); err != nil {
		return err
	}

	response, err := suite.chatbotConversationResponseFromLastBody()
	if err != nil {
		return err
	}
	if response.Title != expectedTitle {
		return fmt.Errorf("expected chatbot conversation title %q, got %q with body %s", expectedTitle, response.Title, string(suite.lastBody))
	}

	return nil
}

func (suite *testSuite) chatbotConversationContainsMyMessage(message *godog.DocString) error {
	return suite.chatbotConversationContainsMessage(participantRoleConsumer, normalizeDocString(message))
}

func (suite *testSuite) systemShowsChatbotResponse(message *godog.DocString) error {
	if err := suite.lastResponseShouldHaveStatusCode(http.StatusCreated); err != nil {
		return err
	}

	return suite.chatbotConversationContainsMessage(participantRoleChatbot, normalizeDocString(message))
}

func (suite *testSuite) systemRegistersMyMessageInChatbotConversation(message *godog.DocString) error {
	return suite.chatbotConversationContainsMyMessage(message)
}

func (suite *testSuite) systemRegistersChatbotResponse(message *godog.DocString) error {
	return suite.systemShowsChatbotResponse(message)
}

func (suite *testSuite) systemReportsOnlyConsumerCanMessageChatbot() error {
	if err := suite.lastResponseShouldHaveStatusCode(http.StatusForbidden); err != nil {
		return err
	}

	return suite.lastResponseShouldHaveError()
}

func (suite *testSuite) systemDoesNotCreateChatbotConversation() error {
	if suite.lastStatus >= http.StatusOK && suite.lastStatus < http.StatusMultipleChoices {
		return fmt.Errorf("expected chatbot conversation creation to fail, got status %d with body %s", suite.lastStatus, string(suite.lastBody))
	}

	response, err := suite.chatbotConversationResponseFromLastBody()
	if err == nil && response.ID != 0 {
		return fmt.Errorf("expected no chatbot conversation id, got %d", response.ID)
	}

	return nil
}

func (suite *testSuite) systemShowsPreDiagnosis(message *godog.DocString) error {
	return suite.systemShowsChatbotResponse(message)
}

func (suite *testSuite) systemReportsChatbotScopeRestriction() error {
	if err := suite.lastResponseShouldHaveStatusCode(http.StatusCreated); err != nil {
		return err
	}

	response, err := suite.chatbotConversationResponseFromLastBody()
	if err != nil {
		return err
	}
	if response.ResponseStatus != chatbotResponseOutOfScope {
		return fmt.Errorf("expected chatbot response status %q, got %q", chatbotResponseOutOfScope, response.ResponseStatus)
	}

	return suite.chatbotConversationContainsMessage(participantRoleChatbot, "Solo puedo responder consultas relacionadas con problemas del hogar.")
}

func (suite *testSuite) requestCreateChatbotConversation(payload chatbotConversationRequest) error {
	body, err := json.Marshal(payload)
	if err != nil {
		return err
	}

	httpReq, err := http.NewRequest(http.MethodPost, suite.server.URL+"/chatbot/conversations", bytes.NewReader(body))
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

func (suite *testSuite) chatbotConversationContainsMessage(senderRole, content string) error {
	if strings.TrimSpace(content) == "" {
		return fmt.Errorf("expected chatbot message content to be non-empty")
	}

	response, err := suite.chatbotConversationResponseFromLastBody()
	if err != nil {
		return err
	}

	for _, message := range response.chatbotMessages() {
		if message.SenderRole == senderRole && message.Content == content {
			if message.CreatedOn.IsZero() {
				return fmt.Errorf("expected chatbot message created_on to be present")
			}

			return nil
		}
	}

	return fmt.Errorf("expected chatbot conversation response to include %s message %q, got body %s", senderRole, content, string(suite.lastBody))
}

func (suite *testSuite) chatbotConversationResponseFromLastBody() (chatbotConversationResponse, error) {
	var response chatbotConversationResponse
	if err := json.Unmarshal(suite.lastBody, &response); err != nil {
		return chatbotConversationResponse{}, fmt.Errorf("response is not valid JSON chatbot conversation response: %w", err)
	}

	return response, nil
}

func (suite *testSuite) rememberCreatedChatbotConversation() error {
	if err := suite.lastResponseShouldHaveStatusCode(http.StatusCreated); err != nil {
		return err
	}

	response, err := suite.chatbotConversationResponseFromLastBody()
	if err != nil {
		return err
	}
	if response.ID == 0 {
		return fmt.Errorf("expected created chatbot conversation id, got body %s", string(suite.lastBody))
	}

	suite.lastConversationID = response.ID
	suite.chatbotConversationIDs = append(suite.chatbotConversationIDs, response.ID)
	suite.chatbotConversationStatuses = append(suite.chatbotConversationStatuses, response.Status)

	return nil
}

func uniqueActiveChatbotConversationIDs(ids []int, statuses []string) []int {
	seen := make(map[int]struct{}, len(ids))
	unique := make([]int, 0, len(ids))
	for index, id := range ids {
		if id <= 0 {
			continue
		}
		if index >= len(statuses) || statuses[index] != conversationStatusActive {
			continue
		}
		if _, ok := seen[id]; ok {
			continue
		}

		seen[id] = struct{}{}
		unique = append(unique, id)
	}

	return unique
}

func (response chatbotConversationResponse) chatbotMessages() []conversationMessageResponse {
	messages := make([]conversationMessageResponse, 0, len(response.Messages)+1)
	messages = append(messages, response.Messages...)
	if response.Response != nil {
		messages = append(messages, *response.Response)
	}

	return messages
}

func (response chatbotConversationResponse) conversationID() int {
	return response.ID
}
