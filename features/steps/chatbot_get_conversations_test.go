package steps_test

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"

	"github.com/cucumber/godog"
)

type chatbotConversationSummaryResponse struct {
	ID             int                                     `json:"id"`
	Status         string                                  `json:"status"`
	Title          string                                  `json:"title"`
	ResponseStatus string                                  `json:"response_status"`
	LastMessage    *conversationLastMessageSummaryResponse `json:"last_message"`
	UpdatedOn      string                                  `json:"updated_on"`
}

func registerChatbotGetConversationsSteps(sc *godog.ScenarioContext, suite *testSuite) {
	sc.Step(`^consulto mis conversaciones con el chatbot asistido por IA$`, suite.requestMyChatbotConversations)
	sc.Step(`^intento consultar mis conversaciones con el chatbot asistido por IA$`, suite.tryRequestMyChatbotConversations)
	sc.Step(`^el sistema muestra un listado de conversaciones con el chatbot asistido por IA vacío$`, suite.systemShowsEmptyChatbotConversationList)
	sc.Step(`^que tengo una conversación con el chatbot asistido por IA titulada "([^"]*)" con el último mensaje:$`, suite.iHaveChatbotConversationTitledWithLastMessage)
	sc.Step(`^que el consumidor "([^"]*)" tiene una conversación con el chatbot asistido por IA titulada "([^"]*)"$`, suite.consumerHasChatbotConversationTitled)
	sc.Step(`^el sistema muestra la conversación con el chatbot asistido por IA titulada "([^"]*)"$`, suite.systemShowsChatbotConversationTitled)
	sc.Step(`^el sistema no muestra la conversación con el chatbot asistido por IA titulada "([^"]*)"$`, suite.systemDoesNotShowChatbotConversationTitled)
	sc.Step(`^el último mensaje de la conversación con el chatbot asistido por IA es:$`, suite.chatbotConversationLastMessageIs)
	sc.Step(`^el sistema no muestra la conversación con el prestador "([^"]*)" en el listado de conversaciones con el chatbot asistido por IA$`, suite.systemDoesNotShowProviderConversationInChatbotList)
	sc.Step(`^el sistema me indica que solo un consumidor puede consultar conversaciones con el chatbot asistido por IA$`, suite.systemReportsOnlyConsumerCanListChatbotConversations)
}

func (suite *testSuite) requestMyChatbotConversations() error {
	return suite.requestChatbotConversations()
}

func (suite *testSuite) tryRequestMyChatbotConversations() error {
	return suite.requestChatbotConversations()
}

func (suite *testSuite) requestChatbotConversations() error {
	httpReq, err := http.NewRequest(http.MethodGet, suite.server.URL+"/chatbot/conversations", nil)
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

func (suite *testSuite) systemShowsEmptyChatbotConversationList() error {
	summaries, err := suite.chatbotConversationSummaryResponsesShouldHaveStatusCode(http.StatusOK)
	if err != nil {
		return err
	}

	if len(summaries) != 0 {
		return fmt.Errorf("expected empty chatbot conversation list, got body %s", string(suite.lastBody))
	}

	return nil
}

func (suite *testSuite) iHaveChatbotConversationTitledWithLastMessage(title string, message *godog.DocString) error {
	lastMessage := normalizeDocString(message)
	if strings.TrimSpace(lastMessage) == "" {
		return fmt.Errorf("expected chatbot conversation fixture last message to be non-empty")
	}

	suite.chatbot.SetResponse(title, lastMessage)
	if err := suite.requestCreateChatbotConversation(chatbotConversationRequest{Content: "Necesito orientación para un problema del hogar."}); err != nil {
		return err
	}

	return suite.rememberCreatedChatbotConversation()
}

func (suite *testSuite) consumerHasChatbotConversationTitled(consumerEmail, title string) error {
	previousAuth0ID := suite.currentAuth0ID
	suite.currentAuth0ID = auth0IDForConsumerEmail(consumerEmail)
	defer func() { suite.currentAuth0ID = previousAuth0ID }()

	if _, err := suite.userRepository.FindIDByEmail(consumerEmail); err != nil {
		return err
	}

	suite.chatbot.SetResponse(title, "Respuesta para la conversación "+title)
	if err := suite.requestCreateChatbotConversation(chatbotConversationRequest{Content: "Necesito orientación para otro problema del hogar."}); err != nil {
		return err
	}

	return suite.rememberCreatedChatbotConversation()
}

func (suite *testSuite) systemShowsChatbotConversationTitled(title string) error {
	summary, err := suite.findChatbotConversationSummaryByTitle(title)
	if err != nil {
		return err
	}

	return suite.assertValidChatbotConversationSummary(summary)
}

func (suite *testSuite) systemDoesNotShowChatbotConversationTitled(title string) error {
	summaries, err := suite.chatbotConversationSummaryResponsesShouldHaveStatusCode(http.StatusOK)
	if err != nil {
		return err
	}

	for _, summary := range summaries {
		if summary.Title == title {
			return fmt.Errorf("expected chatbot conversation title %q not to be listed, got body %s", title, string(suite.lastBody))
		}
	}

	return nil
}

func (suite *testSuite) chatbotConversationLastMessageIs(message *godog.DocString) error {
	summaries, err := suite.chatbotConversationSummaryResponsesShouldHaveStatusCode(http.StatusOK)
	if err != nil {
		return err
	}

	if len(summaries) != 1 {
		return fmt.Errorf("expected exactly one chatbot conversation before checking its last message, got %d with body %s", len(summaries), string(suite.lastBody))
	}

	if summaries[0].LastMessage == nil {
		return fmt.Errorf("expected chatbot conversation to include last_message, got body %s", string(suite.lastBody))
	}

	expectedContent := normalizeDocString(message)
	if summaries[0].LastMessage.Content != expectedContent {
		return fmt.Errorf("expected chatbot conversation last message %q, got %q", expectedContent, summaries[0].LastMessage.Content)
	}

	return nil
}

func (suite *testSuite) systemDoesNotShowProviderConversationInChatbotList(fullName string) error {
	if _, err := suite.chatbotConversationSummaryResponsesShouldHaveStatusCode(http.StatusOK); err != nil {
		return err
	}

	if strings.Contains(string(suite.lastBody), fullName) {
		return fmt.Errorf("expected chatbot conversation list not to include provider %q, got body %s", fullName, string(suite.lastBody))
	}

	return nil
}

func (suite *testSuite) systemReportsOnlyConsumerCanListChatbotConversations() error {
	if err := suite.lastResponseShouldHaveStatusCode(http.StatusForbidden); err != nil {
		return err
	}

	return suite.lastResponseShouldHaveError()
}

func (suite *testSuite) findChatbotConversationSummaryByTitle(title string) (chatbotConversationSummaryResponse, error) {
	summaries, err := suite.chatbotConversationSummaryResponsesShouldHaveStatusCode(http.StatusOK)
	if err != nil {
		return chatbotConversationSummaryResponse{}, err
	}

	for _, summary := range summaries {
		if summary.Title == title {
			return summary, nil
		}
	}

	return chatbotConversationSummaryResponse{}, fmt.Errorf("expected chatbot conversation titled %q, got body %s", title, string(suite.lastBody))
}

func (suite *testSuite) chatbotConversationSummaryResponsesShouldHaveStatusCode(statusCode int) ([]chatbotConversationSummaryResponse, error) {
	if err := suite.lastResponseShouldHaveStatusCode(statusCode); err != nil {
		return nil, err
	}

	var summaries []chatbotConversationSummaryResponse
	if err := json.Unmarshal(suite.lastBody, &summaries); err != nil {
		return nil, fmt.Errorf("response is not valid JSON chatbot conversation summary list: %w", err)
	}

	return summaries, nil
}

func (suite *testSuite) assertValidChatbotConversationSummary(summary chatbotConversationSummaryResponse) error {
	if summary.ID == 0 {
		return fmt.Errorf("expected chatbot conversation summary to include id, got body %s", string(suite.lastBody))
	}

	if strings.TrimSpace(summary.Status) == "" {
		return fmt.Errorf("expected chatbot conversation summary to include status, got body %s", string(suite.lastBody))
	}

	if strings.TrimSpace(summary.Title) == "" {
		return fmt.Errorf("expected chatbot conversation summary to include title, got body %s", string(suite.lastBody))
	}

	if strings.TrimSpace(summary.UpdatedOn) == "" {
		return fmt.Errorf("expected chatbot conversation summary to include updated_on, got body %s", string(suite.lastBody))
	}

	if summary.LastMessage == nil {
		return fmt.Errorf("expected chatbot conversation summary to include last_message, got body %s", string(suite.lastBody))
	}

	if strings.TrimSpace(summary.LastMessage.Content) == "" {
		return fmt.Errorf("expected chatbot conversation last_message to include content, got body %s", string(suite.lastBody))
	}

	if strings.TrimSpace(summary.LastMessage.SenderRole) == "" {
		return fmt.Errorf("expected chatbot conversation last_message to include sender_role, got body %s", string(suite.lastBody))
	}

	if strings.TrimSpace(summary.LastMessage.CreatedOn) == "" {
		return fmt.Errorf("expected chatbot conversation last_message to include created_on, got body %s", string(suite.lastBody))
	}

	return nil
}
