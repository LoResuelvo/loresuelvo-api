package steps_test

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/cucumber/godog"
)

type sendMessageRequest struct {
	Content      string   `json:"content,omitempty"`
	ImageFileIDs []string `json:"image_file_ids,omitempty"`
	AudioFileID  string   `json:"audio_file_id,omitempty"`
	VideoFileID  string   `json:"video_file_id,omitempty"`
}

type sentMessageResponse struct {
	ID             int                    `json:"id"`
	ConversationID int                    `json:"conversation_id"`
	SenderRole     string                 `json:"sender_role"`
	Content        string                 `json:"content"`
	Images         []messageImageResponse `json:"images"`
	Audio          *messageAudioResponse  `json:"audio,omitempty"`
	Video          *messageVideoResponse  `json:"video,omitempty"`
	CreatedOn      time.Time              `json:"created_on"`
}

func registerSendMessageSteps(sc *godog.ScenarioContext, suite *testSuite) {
	sc.Step(`^envío un mensaje en la conversación pendiente con el prestador "([^"]*)":$`, suite.sendMessageInPendingConversationWithProvider)
	sc.Step(`^envío un mensaje en la conversación pendiente con el consumidor "([^"]*)":$`, suite.sendMessageInPendingConversationWithConsumer)
	sc.Step(`^intento enviar un mensaje en la conversación pendiente con el prestador "([^"]*)":$`, suite.trySendMessageInPendingConversationWithProvider)
	sc.Step(`^intento enviar un mensaje en la conversación pendiente con el consumidor "([^"]*)":$`, suite.trySendMessageInPendingConversationWithConsumer)
	sc.Step(`^intento enviar un mensaje en una conversación inexistente:$`, suite.trySendMessageInNonExistingConversation)
	sc.Step(`^intento enviar un mensaje en la conversación pendiente con el prestador "([^"]*)" sin contenido$`, suite.trySendMessageInPendingConversationWithProviderWithoutContent)
	sc.Step(`^intento enviar un mensaje en la conversación pendiente con el prestador "([^"]*)" con el mensaje "([^"]*)"$`, suite.trySendMessageInPendingConversationWithProviderWithInlineContent)
	sc.Step(`^el sistema registra el mensaje en la conversación$`, suite.systemRegistersMessageInConversation)
	sc.Step(`^el último mensaje de la conversación fue enviado por "([^"]*)" con el contenido:$`, suite.conversationLastMessageWasSentByWithContent)
}

func (suite *testSuite) sendMessageInPendingConversationWithProvider(providerFullName string, message *godog.DocString) error {
	return suite.sendMessageInPendingConversationWithParticipant(providerFullName, participantRoleProvider, message)
}

func (suite *testSuite) sendMessageInPendingConversationWithConsumer(consumerFullName string, message *godog.DocString) error {
	return suite.sendMessageInPendingConversationWithParticipant(consumerFullName, participantRoleConsumer, message)
}

func (suite *testSuite) trySendMessageInPendingConversationWithProvider(providerFullName string, message *godog.DocString) error {
	return suite.sendMessageInPendingConversationWithProvider(providerFullName, message)
}

func (suite *testSuite) trySendMessageInPendingConversationWithConsumer(consumerFullName string, message *godog.DocString) error {
	return suite.sendMessageInPendingConversationWithConsumer(consumerFullName, message)
}

func (suite *testSuite) trySendMessageInNonExistingConversation(message *godog.DocString) error {
	return suite.requestSendMessage(nonExistingConversationID, sendMessageRequest{
		Content: normalizeDocString(message),
	})
}

func (suite *testSuite) trySendMessageInPendingConversationWithProviderWithoutContent(providerFullName string) error {
	if err := suite.ensureKnownParticipantFullName(providerFullName, participantRoleProvider); err != nil {
		return err
	}

	return suite.requestSendMessageToPreparedConversation(map[string]any{})
}

func (suite *testSuite) trySendMessageInPendingConversationWithProviderWithInlineContent(providerFullName, content string) error {
	if err := suite.ensureKnownParticipantFullName(providerFullName, participantRoleProvider); err != nil {
		return err
	}

	return suite.requestSendMessageToPreparedConversation(sendMessageRequest{
		Content: content,
	})
}

func (suite *testSuite) sendMessageInPendingConversationWithParticipant(fullName, role string, message *godog.DocString) error {
	if err := suite.ensureKnownParticipantFullName(fullName, role); err != nil {
		return err
	}

	return suite.requestSendMessageToPreparedConversation(sendMessageRequest{
		Content: normalizeDocString(message),
	})
}

func (suite *testSuite) requestSendMessageToPreparedConversation(payload any) error {
	if suite.lastConversationID == 0 {
		return fmt.Errorf("expected a prepared conversation before sending a message")
	}

	return suite.requestSendMessage(suite.lastConversationID, payload)
}

func (suite *testSuite) requestSendMessage(conversationID int, payload any) error {
	body, err := json.Marshal(payload)
	if err != nil {
		return err
	}

	httpReq, err := http.NewRequest(http.MethodPost, fmt.Sprintf("%s/conversations/%d/messages", suite.server.URL, conversationID), bytes.NewReader(body))
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

func (suite *testSuite) systemRegistersMessageInConversation() error {
	if err := suite.lastResponseShouldHaveStatusCode(http.StatusCreated); err != nil {
		return err
	}

	response, err := suite.sentMessageResponseFromLastBody()
	if err != nil {
		return err
	}

	if response.ID == 0 {
		return fmt.Errorf("expected sent message id, got body %s", string(suite.lastBody))
	}
	if response.ConversationID != suite.lastConversationID {
		return fmt.Errorf("expected message conversation id %d, got %d", suite.lastConversationID, response.ConversationID)
	}
	if strings.TrimSpace(response.Content) == "" {
		return fmt.Errorf("expected sent message content, got body %s", string(suite.lastBody))
	}
	if response.CreatedOn.IsZero() {
		return fmt.Errorf("expected sent message created_on, got body %s", string(suite.lastBody))
	}

	expectedSenderRole, err := suite.currentAuthenticatedParticipantRole()
	if err != nil {
		return err
	}
	if response.SenderRole != expectedSenderRole {
		return fmt.Errorf("expected sent message sender role %q, got %q", expectedSenderRole, response.SenderRole)
	}

	return nil
}

func (suite *testSuite) conversationLastMessageWasSentByWithContent(senderFullName string, message *godog.DocString) error {
	expectedSenderRole, err := suite.participantRoleForFullName(senderFullName)
	if err != nil {
		return err
	}

	if suite.lastConversationID == 0 {
		return fmt.Errorf("expected a prepared conversation before checking its last message")
	}

	if err := suite.requestConversationByID(suite.lastConversationID); err != nil {
		return err
	}
	if err := suite.lastResponseShouldHaveStatusCode(http.StatusOK); err != nil {
		return err
	}

	response, err := suite.conversationDetailResponseFromLastBody()
	if err != nil {
		return err
	}
	if len(response.Messages) == 0 {
		return fmt.Errorf("expected conversation detail to include messages, got body %s", string(suite.lastBody))
	}

	lastMessage := response.Messages[len(response.Messages)-1]
	expectedContent := normalizeDocString(message)
	if lastMessage.Content != expectedContent {
		return fmt.Errorf("expected last message content %q, got %q", expectedContent, lastMessage.Content)
	}
	if lastMessage.SenderRole != expectedSenderRole {
		return fmt.Errorf("expected last message sender role %q for %q, got %q", expectedSenderRole, senderFullName, lastMessage.SenderRole)
	}
	if lastMessage.CreatedOn.IsZero() {
		return fmt.Errorf("expected last message created_on to be present")
	}

	return nil
}

func (suite *testSuite) sentMessageResponseFromLastBody() (sentMessageResponse, error) {
	var response sentMessageResponse
	if err := json.Unmarshal(suite.lastBody, &response); err != nil {
		return sentMessageResponse{}, fmt.Errorf("response is not valid JSON sent message response: %w", err)
	}

	return response, nil
}

func (suite *testSuite) currentAuthenticatedParticipantRole() (string, error) {
	foundUser, err := suite.userRepository.FindByAuthID(suite.currentAuth0ID)
	if err == nil {
		return foundUser.Role(), nil
	}

	return "", fmt.Errorf("authenticated user is not a registered conversation participant")
}

func (suite *testSuite) rememberParticipantFullName(name, surname, role string) {
	fullName := strings.TrimSpace(name + " " + surname)
	if fullName == "" {
		return
	}

	suite.participantRolesByFullName[fullName] = role
}

func (suite *testSuite) ensureKnownParticipantFullName(fullName, expectedRole string) error {
	role, err := suite.participantRoleForFullName(fullName)
	if err != nil {
		return err
	}
	if role != expectedRole {
		return fmt.Errorf("expected %q to be a %s, got role %q", fullName, expectedRole, role)
	}

	return nil
}

func (suite *testSuite) participantRoleForFullName(fullName string) (string, error) {
	role, ok := suite.participantRolesByFullName[strings.TrimSpace(fullName)]
	if !ok {
		return "", fmt.Errorf("unknown registered participant full name %q", fullName)
	}

	return role, nil
}
