package steps_test

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/LoResuelvo/loresuelvo-api/internal/domain/conversation"
	"github.com/cucumber/godog"
)

func registerChatbotContinuationSteps(sc *godog.ScenarioContext, suite *testSuite) {
	sc.Step(`^ya tengo una conversación activa con el chatbot sobre:$`, suite.iHaveActiveChatbotConversationAbout)
	sc.Step(`^ya tengo una conversación activa con el chatbot con muchos mensajes sobre una pérdida de agua en la cocina$`, suite.iHaveActiveChatbotConversationWithManyMessagesAboutKitchenLeak)
	sc.Step(`^que existe un resumen de contexto de esa conversación con el chatbot:$`, suite.thereIsChatbotConversationContextSummary)
	sc.Step(`^que el consumidor "([^"]*)" tiene una conversación activa con el chatbot$`, suite.consumerHasActiveChatbotConversation)
	sc.Step(`^la conversación con el chatbot está procesando una respuesta anterior$`, suite.chatbotConversationIsProcessingPreviousResponse)
	sc.Step(`^envío un nuevo mensaje a esa conversación con el chatbot asistido por IA:$`, suite.sendNewMessageToExistingChatbotConversation)
	sc.Step(`^intento enviar un nuevo mensaje a esa conversación con el chatbot asistido por IA:$`, suite.trySendNewMessageToExistingChatbotConversation)
	sc.Step(`^el sistema agrega mi nuevo mensaje a la misma conversación con el chatbot asistido por IA$`, suite.systemAddsMyNewMessageToSameChatbotConversation)
	sc.Step(`^el sistema registra la nueva respuesta del chatbot asistido por IA:$`, suite.systemRegistersNewChatbotResponse)
	sc.Step(`^el sistema no crea una nueva conversación con el chatbot asistido por IA$`, suite.systemDoesNotCreateNewChatbotConversation)
	sc.Step(`^el sistema envía al chatbot el resumen de contexto de la conversación$`, suite.systemSendsConversationContextSummaryToChatbot)
	sc.Step(`^el sistema envía al chatbot los mensajes recientes relevantes de la conversación$`, suite.systemSendsRecentRelevantMessagesToChatbot)
	sc.Step(`^el sistema me indica que no puedo acceder a esa conversación con el chatbot asistido por IA$`, suite.systemReportsCannotAccessChatbotConversation)
	sc.Step(`^el sistema me indica que la conversación con el chatbot está procesando otro mensaje$`, suite.systemReportsChatbotConversationIsProcessingAnotherMessage)
	sc.Step(`^el sistema no registra mi mensaje en esa conversación con el chatbot asistido por IA$`, suite.systemDoesNotRegisterMyMessageInChatbotConversation)
}

func (suite *testSuite) iHaveActiveChatbotConversationAbout(message *godog.DocString) error {
	if err := suite.requestCreateChatbotConversation(chatbotConversationRequest{Content: normalizeDocString(message)}); err != nil {
		return err
	}

	return suite.rememberCreatedChatbotConversation()
}

func (suite *testSuite) iHaveActiveChatbotConversationWithManyMessagesAboutKitchenLeak() error {
	if err := suite.iHaveActiveChatbotConversationAbout(&godog.DocString{Content: "Tengo una pérdida de agua debajo de la pileta de la cocina."}); err != nil {
		return err
	}

	recentMessage := "Todavía no sé si el agua sale del sifón o de una conexión flexible."
	suite.expectedRecentChatbotContextMessage = recentMessage
	foundConversation, err := suite.conversationRepository.FindByID(context.Background(), suite.lastConversationID)
	if err != nil {
		return err
	}

	for _, message := range []conversation.Message{
		mustConsumerMessage("La pérdida empezó ayer después de lavar los platos."),
		mustChatbotMessage("Revisá si el agua aparece solo cuando usás la bacha."),
		mustConsumerMessage(recentMessage),
	} {
		foundConversation.AddMessage(message)
	}
	_, err = suite.conversationRepository.SaveConversation(context.Background(), foundConversation)
	return err
}

func (suite *testSuite) thereIsChatbotConversationContextSummary(summary *godog.DocString) error {
	if suite.lastConversationID == 0 {
		return fmt.Errorf("expected existing chatbot conversation id")
	}

	suite.expectedChatbotContextSummary = normalizeDocString(summary)
	foundConversation, err := suite.conversationRepository.FindByID(context.Background(), suite.lastConversationID)
	if err != nil {
		return err
	}
	chatbotConversation, ok := foundConversation.(*conversation.ChatBotConversation)
	if !ok {
		return fmt.Errorf("expected chatbot conversation fixture")
	}
	if err := chatbotConversation.UpdateContext(conversation.ChatbotConversationContext{
		Summary:                 suite.expectedChatbotContextSummary,
		LastSummarizedMessageID: chatbotConversation.Context.LastSummarizedMessageID,
	}); err != nil {
		return err
	}
	_, err = suite.conversationRepository.SaveConversation(context.Background(), chatbotConversation)
	return err
}

func (suite *testSuite) consumerHasActiveChatbotConversation(consumerEmail string) error {
	previousAuth0ID := suite.currentAuth0ID
	suite.currentAuth0ID = auth0IDForConsumerEmail(consumerEmail)
	defer func() { suite.currentAuth0ID = previousAuth0ID }()

	return suite.iHaveActiveChatbotConversationAbout(&godog.DocString{Content: "Tengo una pérdida de agua debajo de la pileta de la cocina."})
}

func (suite *testSuite) chatbotConversationIsProcessingPreviousResponse() error {
	if suite.lastConversationID == 0 {
		return fmt.Errorf("expected existing chatbot conversation id")
	}

	foundConversation, err := suite.conversationRepository.FindByID(context.Background(), suite.lastConversationID)
	if err != nil {
		return err
	}
	chatbotConversation, ok := foundConversation.(*conversation.ChatBotConversation)
	if !ok {
		return fmt.Errorf("expected chatbot conversation fixture")
	}
	if err := chatbotConversation.StartProcessing(time.Now().UTC()); err != nil {
		return err
	}
	_, err = suite.conversationRepository.SaveConversation(context.Background(), chatbotConversation)
	return err
}

func (suite *testSuite) sendNewMessageToExistingChatbotConversation(message *godog.DocString) error {
	suite.lastAttemptedChatbotContinuationMessage = normalizeDocString(message)
	return suite.requestContinueChatbotConversation(suite.lastConversationID, chatbotConversationRequest{Content: suite.lastAttemptedChatbotContinuationMessage})
}

func (suite *testSuite) trySendNewMessageToExistingChatbotConversation(message *godog.DocString) error {
	return suite.sendNewMessageToExistingChatbotConversation(message)
}

func (suite *testSuite) systemAddsMyNewMessageToSameChatbotConversation() error {
	if err := suite.lastResponseShouldHaveStatusCode(http.StatusCreated); err != nil {
		return err
	}

	response, err := suite.chatbotConversationResponseFromLastBody()
	if err != nil {
		return err
	}

	if response.conversationID() != suite.lastConversationID {
		return fmt.Errorf("expected chatbot response conversation id %d, got %d with body %s", suite.lastConversationID, response.conversationID(), string(suite.lastBody))
	}

	return suite.chatbotConversationContainsMessage(participantRoleConsumer, suite.lastAttemptedChatbotContinuationMessage)
}

func (suite *testSuite) systemRegistersNewChatbotResponse(message *godog.DocString) error {
	return suite.chatbotConversationContainsMessage(participantRoleChatbot, normalizeDocString(message))
}

func (suite *testSuite) systemDoesNotCreateNewChatbotConversation() error {
	response, err := suite.chatbotConversationResponseFromLastBody()
	if err != nil {
		return err
	}
	if response.conversationID() != suite.lastConversationID {
		return fmt.Errorf("expected existing chatbot conversation id %d, got %d with body %s", suite.lastConversationID, response.conversationID(), string(suite.lastBody))
	}

	return nil
}

func (suite *testSuite) systemSendsConversationContextSummaryToChatbot() error {
	if err := suite.lastResponseShouldHaveStatusCode(http.StatusCreated); err != nil {
		return err
	}
	if strings.TrimSpace(suite.expectedChatbotContextSummary) == "" {
		return fmt.Errorf("expected chatbot context summary fixture to be present")
	}
	if suite.chatbot.LastQuestion().ContextSummary != suite.expectedChatbotContextSummary {
		return fmt.Errorf("expected chatbot question to include context summary %q, got %#v", suite.expectedChatbotContextSummary, suite.chatbot.LastQuestion())
	}

	return nil
}

func (suite *testSuite) systemSendsRecentRelevantMessagesToChatbot() error {
	if strings.TrimSpace(suite.expectedRecentChatbotContextMessage) == "" {
		return fmt.Errorf("expected recent chatbot context message fixture to be present")
	}
	question := suite.chatbot.LastQuestion()
	for _, message := range question.RecentMessages {
		if message.Content == suite.expectedRecentChatbotContextMessage {
			return nil
		}
	}

	return fmt.Errorf("expected chatbot question to include recent message %q, got %#v", suite.expectedRecentChatbotContextMessage, question)
}

func (suite *testSuite) systemReportsCannotAccessChatbotConversation() error {
	if err := suite.lastResponseShouldHaveStatusCode(http.StatusForbidden); err != nil {
		return err
	}

	return suite.lastResponseShouldHaveError()
}

func (suite *testSuite) systemReportsChatbotConversationIsProcessingAnotherMessage() error {
	if err := suite.lastResponseShouldHaveStatusCode(http.StatusConflict); err != nil {
		return err
	}

	return suite.lastResponseShouldHaveError()
}

func (suite *testSuite) systemDoesNotRegisterMyMessageInChatbotConversation() error {
	if suite.lastStatus >= http.StatusOK && suite.lastStatus < http.StatusMultipleChoices {
		return fmt.Errorf("expected chatbot continuation to fail, got status %d with body %s", suite.lastStatus, string(suite.lastBody))
	}
	if strings.TrimSpace(suite.lastAttemptedChatbotContinuationMessage) == "" {
		return nil
	}
	if strings.Contains(string(suite.lastBody), suite.lastAttemptedChatbotContinuationMessage) {
		return fmt.Errorf("expected failed response not to include attempted message %q, got body %s", suite.lastAttemptedChatbotContinuationMessage, string(suite.lastBody))
	}

	foundConversation, err := suite.conversationRepository.FindByID(context.Background(), suite.lastConversationID)
	if err != nil {
		return err
	}
	for _, message := range foundConversation.Messages() {
		if message.SenderRole == conversation.SenderConsumer && message.Content == suite.lastAttemptedChatbotContinuationMessage {
			return fmt.Errorf("expected attempted chatbot continuation message %q not to be persisted", suite.lastAttemptedChatbotContinuationMessage)
		}
	}

	return nil
}

func (suite *testSuite) requestContinueChatbotConversation(conversationID int, payload chatbotConversationRequest) error {
	if conversationID == 0 {
		return fmt.Errorf("expected existing chatbot conversation id")
	}

	body, err := json.Marshal(payload)
	if err != nil {
		return err
	}

	httpReq, err := http.NewRequest(http.MethodPost, fmt.Sprintf("%s/chatbot/conversations/%d/messages", suite.server.URL, conversationID), bytes.NewReader(body))
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

func mustConsumerMessage(content string) conversation.Message {
	message, err := conversation.NewConsumerMessage(content)
	if err != nil {
		panic(err)
	}
	return *message
}

func mustChatbotMessage(content string) conversation.Message {
	message, err := conversation.NewChatbotMessage(content)
	if err != nil {
		panic(err)
	}
	return *message
}
