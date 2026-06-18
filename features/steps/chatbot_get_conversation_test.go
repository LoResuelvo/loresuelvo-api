package steps_test

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strings"

	"github.com/cucumber/godog"
)

type chatbotConversationDetailResponse struct {
	ID        int                           `json:"id"`
	Type      string                        `json:"type"`
	Status    string                        `json:"status"`
	Chatbot   chatbotConversationDetail     `json:"chatbot"`
	Messages  []conversationMessageResponse `json:"messages"`
	UpdatedOn string                        `json:"updated_on"`
}

type chatbotConversationDetail struct {
	Title                string                    `json:"title"`
	ResponseStatus       string                    `json:"response_status"`
	DiagnosisCompleted   bool                      `json:"diagnosis_completed"`
	RecommendedCategory  recommendedCategoryDetail `json:"recommended_category"`
	RecommendedProviders []providerSummaryResponse `json:"recommended_providers"`
}

type recommendedCategoryDetail struct {
	ID   int    `json:"id"`
	Name string `json:"name"`
}

func registerChatbotGetConversationSteps(sc *godog.ScenarioContext, suite *testSuite) {
	sc.Step(`^que tengo una conversación con el chatbot asistido por IA titulada "([^"]*)" con los mensajes:$`, suite.iHaveChatbotConversationTitledWithMessages)
	sc.Step(`^que tengo una conversación con el chatbot asistido por IA con diagnóstico concluido para el rubro "([^"]*)" y respuesta:$`, suite.iHaveChatbotConversationWithConcludedDiagnosisForCategory)
	sc.Step(`^consulto el detalle de esa conversación$`, suite.requestThatConversationDetail)
	sc.Step(`^intento consultar el detalle de esa conversación con el chatbot asistido por IA$`, suite.tryRequestThatChatbotConversationDetail)
	sc.Step(`^intento consultar una conversación con el chatbot asistido por IA inexistente$`, suite.tryRequestNonExistingChatbotConversation)
	sc.Step(`^el sistema muestra una conversación con el chatbot titulada "([^"]*)"$`, suite.systemShowsChatbotConversationDetailTitled)
	sc.Step(`^el detalle de la conversación con el chatbot incluye mi mensaje:$`, suite.chatbotConversationDetailIncludesMyMessage)
	sc.Step(`^el detalle de la conversación con el chatbot incluye la respuesta del chatbot asistido por IA:$`, suite.chatbotConversationDetailIncludesChatbotResponse)
	sc.Step(`^el sistema muestra que el diagnóstico del chatbot está concluido$`, suite.systemShowsChatbotDiagnosisCompleted)
	sc.Step(`^el sistema muestra el rubro recomendado "([^"]*)" en el detalle de la conversación con el chatbot$`, suite.systemShowsRecommendedCategoryInChatbotConversationDetail)
	sc.Step(`^el sistema muestra al prestador recomendado "([^"]*)" en el detalle de la conversación con el chatbot$`, suite.systemShowsRecommendedProviderInChatbotConversationDetail)
	sc.Step(`^el sistema no muestra al prestador recomendado "([^"]*)" en el detalle de la conversación con el chatbot$`, suite.systemDoesNotShowRecommendedProviderInChatbotConversationDetail)
}

func (suite *testSuite) iHaveChatbotConversationTitledWithMessages(title string, messages *godog.DocString) error {
	consumerMessage, chatbotMessage, err := chatbotConversationFixtureMessages(messages)
	if err != nil {
		return err
	}

	suite.chatbot.SetResponse(title, chatbotMessage)
	if err := suite.requestCreateChatbotConversation(chatbotConversationRequest{Content: consumerMessage}); err != nil {
		return err
	}

	return suite.rememberCreatedChatbotConversation()
}

func (suite *testSuite) iHaveChatbotConversationWithConcludedDiagnosisForCategory(categoryName string, response *godog.DocString) error {
	if err := suite.chatbotWillConcludeDiagnosisAndRecommendCategory(categoryName, response); err != nil {
		return err
	}

	if err := suite.requestCreateChatbotConversation(chatbotConversationRequest{Content: "Hay agua acumulada dentro del mueble bajo mesada cada vez que uso la pileta."}); err != nil {
		return err
	}

	return suite.rememberCreatedChatbotConversation()
}

func (suite *testSuite) requestThatConversationDetail() error {
	if suite.lastConversationID == 0 {
		return fmt.Errorf("expected prepared chatbot conversation id")
	}

	return suite.requestConversationByID(suite.lastConversationID)
}

func (suite *testSuite) tryRequestThatChatbotConversationDetail() error {
	return suite.requestThatConversationDetail()
}

func (suite *testSuite) tryRequestNonExistingChatbotConversation() error {
	return suite.requestConversationByID(nonExistingConversationID)
}

func (suite *testSuite) systemShowsChatbotConversationDetailTitled(title string) error {
	response, err := suite.chatbotConversationDetailResponseShouldHaveStatusCode(http.StatusOK)
	if err != nil {
		return err
	}

	if response.ID != suite.lastConversationID {
		return fmt.Errorf("expected chatbot conversation id %d, got %d", suite.lastConversationID, response.ID)
	}
	if response.Type != "" && response.Type != conversationTypeChatbot {
		return fmt.Errorf("expected chatbot conversation type %q, got %q", conversationTypeChatbot, response.Type)
	}
	if response.Chatbot.Title != title {
		return fmt.Errorf("expected chatbot conversation title %q, got %q with body %s", title, response.Chatbot.Title, string(suite.lastBody))
	}
	if strings.TrimSpace(response.Status) == "" {
		return fmt.Errorf("expected chatbot conversation detail to include status, got body %s", string(suite.lastBody))
	}

	return nil
}

func (suite *testSuite) chatbotConversationDetailIncludesMyMessage(message *godog.DocString) error {
	return suite.chatbotConversationDetailIncludesMessage(participantRoleConsumer, normalizeDocString(message))
}

func (suite *testSuite) chatbotConversationDetailIncludesChatbotResponse(message *godog.DocString) error {
	return suite.chatbotConversationDetailIncludesMessage(participantRoleChatbot, normalizeDocString(message))
}

func (suite *testSuite) systemShowsChatbotDiagnosisCompleted() error {
	response, err := suite.chatbotConversationDetailResponseShouldHaveStatusCode(http.StatusOK)
	if err != nil {
		return err
	}
	if !response.Chatbot.DiagnosisCompleted {
		return fmt.Errorf("expected chatbot diagnosis to be completed, got body %s", string(suite.lastBody))
	}

	return nil
}

func (suite *testSuite) systemShowsRecommendedCategoryInChatbotConversationDetail(categoryName string) error {
	response, err := suite.chatbotConversationDetailResponseShouldHaveStatusCode(http.StatusOK)
	if err != nil {
		return err
	}
	if !sameNormalizedName(response.Chatbot.RecommendedCategory.Name, categoryName) {
		return fmt.Errorf("expected recommended category %q, got %q with body %s", categoryName, response.Chatbot.RecommendedCategory.Name, string(suite.lastBody))
	}

	return nil
}

func (suite *testSuite) systemShowsRecommendedProviderInChatbotConversationDetail(fullName string) error {
	providers, err := suite.recommendedProvidersFromLastChatbotConversationDetail()
	if err != nil {
		return err
	}
	if providerListIncludesFullName(providers, fullName) {
		return nil
	}

	return fmt.Errorf("expected chatbot conversation detail recommended providers to include %q, got body %s", fullName, string(suite.lastBody))
}

func (suite *testSuite) systemDoesNotShowRecommendedProviderInChatbotConversationDetail(fullName string) error {
	providers, err := suite.recommendedProvidersFromLastChatbotConversationDetail()
	if err != nil {
		return err
	}
	if providerListIncludesFullName(providers, fullName) {
		return fmt.Errorf("expected chatbot conversation detail recommended providers not to include %q, got body %s", fullName, string(suite.lastBody))
	}

	return nil
}

func (suite *testSuite) chatbotConversationDetailIncludesMessage(senderRole, content string) error {
	response, err := suite.chatbotConversationDetailResponseShouldHaveStatusCode(http.StatusOK)
	if err != nil {
		return err
	}

	for _, message := range response.Messages {
		if message.SenderRole == senderRole && message.Content == content {
			if message.CreatedOn.IsZero() {
				return fmt.Errorf("expected chatbot conversation detail message created_on to be present")
			}
			return nil
		}
	}

	return fmt.Errorf("expected chatbot conversation detail to include %s message %q, got body %s", senderRole, content, string(suite.lastBody))
}

func (suite *testSuite) recommendedProvidersFromLastChatbotConversationDetail() ([]providerSummaryResponse, error) {
	response, err := suite.chatbotConversationDetailResponseShouldHaveStatusCode(http.StatusOK)
	if err != nil {
		return nil, err
	}

	return response.Chatbot.RecommendedProviders, nil
}

func (suite *testSuite) chatbotConversationDetailResponseShouldHaveStatusCode(statusCode int) (chatbotConversationDetailResponse, error) {
	if err := suite.lastResponseShouldHaveStatusCode(statusCode); err != nil {
		return chatbotConversationDetailResponse{}, err
	}

	var response chatbotConversationDetailResponse
	if err := json.Unmarshal(suite.lastBody, &response); err != nil {
		return chatbotConversationDetailResponse{}, fmt.Errorf("response is not valid JSON chatbot conversation detail: %w", err)
	}

	return response, nil
}

func chatbotConversationFixtureMessages(messages *godog.DocString) (string, string, error) {
	var consumerMessage string
	var chatbotMessage string
	for _, line := range strings.Split(normalizeDocString(messages), "\n") {
		line = strings.TrimSpace(line)
		switch {
		case strings.HasPrefix(line, participantRoleConsumer+":"):
			consumerMessage = strings.TrimSpace(strings.TrimPrefix(line, participantRoleConsumer+":"))
		case strings.HasPrefix(line, participantRoleChatbot+":"):
			chatbotMessage = strings.TrimSpace(strings.TrimPrefix(line, participantRoleChatbot+":"))
		}
	}

	if consumerMessage == "" || chatbotMessage == "" {
		return "", "", fmt.Errorf("expected chatbot fixture messages to include consumer and chatbot lines, got %q", normalizeDocString(messages))
	}

	return consumerMessage, chatbotMessage, nil
}
