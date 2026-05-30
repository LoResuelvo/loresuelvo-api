package steps_test

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"

	"github.com/LoResuelvo/loresuelvo-api/internal/domain/conversation"
	"github.com/cucumber/godog"
)

const (
	nonExistingConversationID = 999999999
	providerSenderRole        = "provider"
)

type conversationDetailResponse struct {
	ID         int                           `json:"id"`
	ConsumerID int                           `json:"consumer_id"`
	ProviderID int                           `json:"provider_id"`
	Status     string                        `json:"status"`
	Messages   []conversationMessageResponse `json:"messages"`
}

type conversationMessageResponse struct {
	ID             int    `json:"id"`
	ConversationID int    `json:"conversation_id"`
	SenderRole     string `json:"sender_role"`
	SenderEmail    string `json:"sender_email,omitempty"`
	Content        string `json:"content"`
}

func registerGetConversationSteps(sc *godog.ScenarioContext, suite *testSuite) {
	sc.Step(`^que existe una conversación pendiente entre el consumidor "([^"]*)" y el prestador "([^"]*)" con el mensaje inicial:$`, suite.thereIsPendingConversationBetweenConsumerAndProviderWithInitialMessage)
	sc.Step(`^consulto la conversación pendiente con el prestador "([^"]*)"$`, suite.requestPendingConversationWithProvider)
	sc.Step(`^consulto la conversación pendiente con el consumidor "([^"]*)"$`, suite.requestPendingConversationWithConsumer)
	sc.Step(`^intento consultar la conversación pendiente con el prestador "([^"]*)"$`, suite.tryRequestPendingConversationWithProvider)
	sc.Step(`^intento consultar la conversación pendiente con el consumidor "([^"]*)"$`, suite.tryRequestPendingConversationWithConsumer)
	sc.Step(`^intento consultar una conversación inexistente$`, suite.tryRequestNonExistingConversation)
	sc.Step(`^el sistema muestra la conversación entre el consumidor "([^"]*)" y el prestador "([^"]*)"$`, suite.systemShowsConversationBetweenConsumerAndProvider)
	sc.Step(`^el detalle de la conversación incluye el mensaje inicial enviado por "([^"]*)":$`, suite.conversationDetailIncludesInitialMessageSentBy)
	sc.Step(`^el sistema me indica que no puedo acceder a esa conversación$`, suite.systemReportsConversationAccessDenied)
	sc.Step(`^el sistema me indica que la conversación no existe$`, suite.systemReportsConversationDoesNotExist)
}

func (suite *testSuite) thereIsPendingConversationBetweenConsumerAndProviderWithInitialMessage(consumerEmail, providerEmail string, message *godog.DocString) error {
	return suite.createPendingConversationBetweenConsumerAndProvider(consumerEmail, providerEmail, normalizeDocString(message))
}

func (suite *testSuite) createPendingConversationBetweenConsumerAndProvider(consumerEmail, providerEmail, content string) error {
	providerID, err := suite.providerIDByEmail(providerEmail)
	if err != nil {
		return err
	}

	consumerID, err := suite.consumerRepository.FindIDByEmail(consumerEmail)
	if err != nil {
		return err
	}

	pendingConversation, err := conversation.NewPendingConversation(consumerID, providerID)
	if err != nil {
		return err
	}

	message, err := conversation.NewConsumerMessage(content)
	if err != nil {
		return err
	}

	createdConversation, err := suite.conversationRepository.SaveWithMessage(*pendingConversation, *message)
	if err != nil {
		return err
	}

	suite.lastConversationID = createdConversation.ID

	return nil
}

func (suite *testSuite) requestPendingConversationWithProvider(providerEmail string) error {
	return suite.requestPendingConversationWithParticipant(providerEmail)
}

func (suite *testSuite) requestPendingConversationWithConsumer(consumerEmail string) error {
	return suite.requestPendingConversationWithParticipant(consumerEmail)
}

func (suite *testSuite) tryRequestPendingConversationWithProvider(providerEmail string) error {
	return suite.requestPendingConversationWithProvider(providerEmail)
}

func (suite *testSuite) tryRequestPendingConversationWithConsumer(consumerEmail string) error {
	return suite.requestPendingConversationWithConsumer(consumerEmail)
}

func (suite *testSuite) requestPendingConversationWithParticipant(_ string) error {
	if suite.lastConversationID == 0 {
		return fmt.Errorf("expected a prepared conversation before requesting it")
	}

	return suite.requestConversationByID(suite.lastConversationID)
}

func (suite *testSuite) tryRequestNonExistingConversation() error {
	return suite.requestConversationByID(nonExistingConversationID)
}

func (suite *testSuite) requestConversationByID(conversationID int) error {
	httpReq, err := http.NewRequest(http.MethodGet, fmt.Sprintf("%s/conversations/%d", suite.server.URL, conversationID), nil)
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

func (suite *testSuite) systemShowsConversationBetweenConsumerAndProvider(consumerEmail, providerEmail string) error {
	if err := suite.lastResponseShouldHaveStatusCode(http.StatusOK); err != nil {
		return err
	}

	response, err := suite.conversationDetailResponseFromLastBody()
	if err != nil {
		return err
	}

	consumerID, err := suite.consumerRepository.FindIDByEmail(consumerEmail)
	if err != nil {
		return err
	}

	providerID, err := suite.providerIDByEmail(providerEmail)
	if err != nil {
		return err
	}

	if response.ID != suite.lastConversationID {
		return fmt.Errorf("expected conversation id %d, got %d", suite.lastConversationID, response.ID)
	}
	if response.ConsumerID != consumerID {
		return fmt.Errorf("expected consumer id %d, got %d", consumerID, response.ConsumerID)
	}
	if response.ProviderID != providerID {
		return fmt.Errorf("expected provider id %d, got %d", providerID, response.ProviderID)
	}
	if response.Status != conversation.StatusPending {
		return fmt.Errorf("expected conversation status %q, got %q", conversation.StatusPending, response.Status)
	}

	return nil
}

func (suite *testSuite) conversationDetailIncludesInitialMessageSentBy(senderEmail string, message *godog.DocString) error {
	response, err := suite.conversationDetailResponseFromLastBody()
	if err != nil {
		return err
	}

	expectedContent := normalizeDocString(message)
	expectedSenderRole, err := suite.senderRoleForEmail(senderEmail)
	if err != nil {
		return err
	}

	for _, responseMessage := range response.Messages {
		if responseMessage.Content != expectedContent {
			continue
		}

		if responseMessage.SenderRole != expectedSenderRole {
			return fmt.Errorf("expected message sender role %q, got %q", expectedSenderRole, responseMessage.SenderRole)
		}
		if responseMessage.ConversationID != 0 && responseMessage.ConversationID != response.ID {
			return fmt.Errorf("expected message conversation id %d, got %d", response.ID, responseMessage.ConversationID)
		}
		if strings.TrimSpace(responseMessage.SenderEmail) != "" && responseMessage.SenderEmail != senderEmail {
			return fmt.Errorf("expected message sender email %q, got %q", senderEmail, responseMessage.SenderEmail)
		}

		return nil
	}

	return fmt.Errorf("expected initial message %q in conversation detail, got body %s", expectedContent, string(suite.lastBody))
}

func (suite *testSuite) systemReportsConversationAccessDenied() error {
	return suite.conversationRequestShouldFailWithStatus(http.StatusForbidden)
}

func (suite *testSuite) systemReportsConversationDoesNotExist() error {
	return suite.conversationRequestShouldFailWithStatus(http.StatusNotFound)
}

func (suite *testSuite) conversationDetailResponseFromLastBody() (conversationDetailResponse, error) {
	var response conversationDetailResponse
	if err := json.Unmarshal(suite.lastBody, &response); err != nil {
		return conversationDetailResponse{}, fmt.Errorf("response is not valid JSON conversation detail response: %w", err)
	}

	return response, nil
}

func (suite *testSuite) senderRoleForEmail(email string) (string, error) {
	if _, err := suite.consumerRepository.FindIDByEmail(email); err == nil {
		return conversation.SenderConsumer, nil
	}

	if _, err := suite.providerIDByEmail(email); err == nil {
		return providerSenderRole, nil
	}

	return "", fmt.Errorf("could not resolve sender role for email %q", email)
}
