package steps_test

import (
	"fmt"
	"net/http"

	"github.com/cucumber/godog"
)

func registerChatbotAttachImagesSteps(sc *godog.ScenarioContext, suite *testSuite) {
	sc.Step(`^envío un mensaje al chatbot asistido por IA con la imagen cargada "([^"]*)":$`, suite.sendMessageToChatbotWithImage)
	sc.Step(`^envío un nuevo mensaje a esa conversación con el chatbot asistido por IA con la imagen cargada "([^"]*)":$`, suite.sendNewMessageToExistingChatbotConversationWithImage)
	sc.Step(`^envío un mensaje sin texto al chatbot asistido por IA con la imagen cargada "([^"]*)"$`, suite.sendImageOnlyMessageToChatbot)
	sc.Step(`^envío un mensaje al chatbot asistido por IA con las imágenes cargadas:$`, suite.sendMessageToChatbotWithUploadedImages)
	sc.Step(`^intento enviar un mensaje al chatbot asistido por IA adjuntando la imagen "([^"]*)":$`, suite.trySendMessageToChatbotWithImage)
	sc.Step(`^intento enviar un mensaje al chatbot asistido por IA adjuntando esas imágenes:$`, suite.trySendMessageToChatbotWithUploadedImages)
	sc.Step(`^que el consumidor "([^"]*)" envió un mensaje al chatbot con la imagen "([^"]*)"$`, suite.consumerSentMessageToChatbotWithImage)
	sc.Step(`^intento acceder a la imagen "([^"]*)" adjunta al mensaje del chatbot$`, suite.tryAccessChatbotMessageImage)

	sc.Step(`^la conversación contiene mi mensaje con la imagen "([^"]*)"$`, suite.chatbotConversationContainsMyMessageWithImage)
	sc.Step(`^el mensaje queda asociado a la imagen "([^"]*)"$`, suite.chatbotTurnMessageRemainsAssociatedWithImage)
	sc.Step(`^la conversación contiene mi mensaje con las dos imágenes$`, suite.chatbotConversationContainsMyMessageWithTwoImages)
	sc.Step(`^el chatbot recibe la imagen "([^"]*)" para elaborar el pre-diagnóstico$`, suite.chatbotReceivesImageForPreDiagnosis)
	sc.Step(`^el chatbot recibe las imágenes "([^"]*)" y "([^"]*)" para elaborar el pre-diagnóstico$`, suite.chatbotReceivesImagesForPreDiagnosis)
	sc.Step(`^el detalle de la conversación con el chatbot incluye mi mensaje con la imagen "([^"]*)"$`, suite.chatbotConversationDetailIncludesMyMessageWithImage)
	sc.Step(`^el sistema permite al consumidor acceder a la imagen adjunta$`, suite.systemAllowsConsumerToAccessChatbotAttachedImage)
}

func (suite *testSuite) sendMessageToChatbotWithImage(imageName string, message *godog.DocString) error {
	return suite.requestCreateChatbotConversationWithImages(normalizeDocString(message), []string{imageName})
}

func (suite *testSuite) sendNewMessageToExistingChatbotConversationWithImage(imageName string, message *godog.DocString) error {
	suite.lastAttemptedChatbotContinuationMessage = normalizeDocString(message)
	return suite.requestContinueChatbotConversationWithImages(suite.lastConversationID, suite.lastAttemptedChatbotContinuationMessage, []string{imageName})
}

func (suite *testSuite) sendImageOnlyMessageToChatbot(imageName string) error {
	return suite.requestCreateChatbotConversationWithImages("", []string{imageName})
}

func (suite *testSuite) sendMessageToChatbotWithUploadedImages(message *godog.DocString) error {
	return suite.requestCreateChatbotConversationWithImages(normalizeDocString(message), suite.allRememberedMessageImageNames())
}

func (suite *testSuite) trySendMessageToChatbotWithImage(imageName string, message *godog.DocString) error {
	return suite.requestCreateChatbotConversationWithImages(normalizeDocString(message), []string{imageName})
}

func (suite *testSuite) trySendMessageToChatbotWithUploadedImages(message *godog.DocString) error {
	return suite.requestCreateChatbotConversationWithImages(normalizeDocString(message), suite.allRememberedMessageImageNames())
}

func (suite *testSuite) consumerSentMessageToChatbotWithImage(email, imageName string) error {
	previousAuth0ID := suite.currentAuth0ID
	suite.currentAuth0ID = auth0IDForConsumerEmail(email)
	defer func() { suite.currentAuth0ID = previousAuth0ID }()

	if err := suite.uploadAndConfirmMessageImage(imageName); err != nil {
		return err
	}
	if err := suite.requestCreateChatbotConversationWithImages("Necesito orientación con esta imagen del problema.", []string{imageName}); err != nil {
		return err
	}
	return suite.rememberCreatedChatbotConversation()
}

func (suite *testSuite) requestCreateChatbotConversationWithImages(content string, imageNames []string) error {
	fileIDs, err := suite.messageImageFileIDs(imageNames)
	if err != nil {
		return err
	}
	suite.lastAttemptedMessageImageNames = append([]string(nil), imageNames...)
	return suite.requestCreateChatbotConversation(chatbotConversationRequest{Content: content, ImageFileIDs: fileIDs})
}

func (suite *testSuite) requestContinueChatbotConversationWithImages(conversationID int, content string, imageNames []string) error {
	fileIDs, err := suite.messageImageFileIDs(imageNames)
	if err != nil {
		return err
	}
	suite.lastAttemptedMessageImageNames = append([]string(nil), imageNames...)
	return suite.requestContinueChatbotConversation(conversationID, chatbotConversationRequest{Content: content, ImageFileIDs: fileIDs})
}

func (suite *testSuite) chatbotConversationContainsMyMessageWithImage(imageName string) error {
	response, err := suite.chatbotConversationResponseShouldHaveStatusCode(http.StatusCreated)
	if err != nil {
		return err
	}
	return suite.assertChatbotResponseConsumerMessageImages(response, []string{imageName})
}

func (suite *testSuite) chatbotTurnMessageRemainsAssociatedWithImage(imageName string) error {
	response, err := suite.chatbotConversationResponseShouldHaveStatusCode(http.StatusCreated)
	if err != nil {
		return err
	}
	return suite.assertChatbotResponseConsumerMessageImages(response, []string{imageName})
}

func (suite *testSuite) chatbotConversationContainsMyMessageWithTwoImages() error {
	response, err := suite.chatbotConversationResponseShouldHaveStatusCode(http.StatusCreated)
	if err != nil {
		return err
	}
	return suite.assertChatbotResponseConsumerMessageImages(response, suite.lastAttemptedMessageImageNames)
}

func (suite *testSuite) chatbotReceivesImageForPreDiagnosis(imageName string) error {
	if err := suite.assertChatbotWasCalled(); err != nil {
		return err
	}
	if err := suite.assertLastChatbotQuestionImages([]string{imageName}); err != nil {
		return err
	}
	response, err := suite.chatbotConversationResponseShouldHaveStatusCode(http.StatusCreated)
	if err != nil {
		return err
	}
	return suite.assertChatbotResponseConsumerMessageImages(response, []string{imageName})
}

func (suite *testSuite) chatbotReceivesImagesForPreDiagnosis(firstName, secondName string) error {
	if err := suite.assertChatbotWasCalled(); err != nil {
		return err
	}
	if err := suite.assertLastChatbotQuestionImages([]string{firstName, secondName}); err != nil {
		return err
	}
	response, err := suite.chatbotConversationResponseShouldHaveStatusCode(http.StatusCreated)
	if err != nil {
		return err
	}
	return suite.assertChatbotResponseConsumerMessageImages(response, []string{firstName, secondName})
}

func (suite *testSuite) chatbotConversationDetailIncludesMyMessageWithImage(imageName string) error {
	response, err := suite.chatbotConversationDetailResponseShouldHaveStatusCode(http.StatusOK)
	if err != nil {
		return err
	}
	return suite.assertConversationDetailConsumerMessageImages(response.Messages, []string{imageName})
}

func (suite *testSuite) systemAllowsConsumerToAccessChatbotAttachedImage() error {
	response, err := suite.chatbotConversationDetailResponseShouldHaveStatusCode(http.StatusOK)
	if err != nil {
		return err
	}
	for _, message := range response.Messages {
		for _, image := range message.Images {
			if image.ID != "" && image.URL != "" {
				return nil
			}
		}
	}
	return fmt.Errorf("expected chatbot conversation detail to expose an accessible image URL")
}

func (suite *testSuite) tryAccessChatbotMessageImage(imageName string) error {
	if _, ok := suite.messageImagesByName[imageName]; !ok {
		return fmt.Errorf("expected image %q to exist before checking access", imageName)
	}
	return suite.requestConversationByID(suite.lastConversationID)
}

func (suite *testSuite) chatbotConversationResponseShouldHaveStatusCode(statusCode int) (chatbotConversationResponse, error) {
	if err := suite.lastResponseShouldHaveStatusCode(statusCode); err != nil {
		return chatbotConversationResponse{}, err
	}
	return suite.chatbotConversationResponseFromLastBody()
}

func (suite *testSuite) assertChatbotResponseConsumerMessageImages(response chatbotConversationResponse, expectedNames []string) error {
	foundConsumerMessage := false
	for _, message := range response.chatbotMessages() {
		if message.SenderRole == participantRoleConsumer {
			foundConsumerMessage = true
			if err := suite.assertMessageImages(message.Images, expectedNames); err == nil {
				return nil
			}
		}
	}
	if foundConsumerMessage {
		return fmt.Errorf("expected chatbot conversation response to include a consumer message with images %v, got body %s", expectedNames, string(suite.lastBody))
	}
	return fmt.Errorf("expected chatbot conversation response to include a consumer message, got body %s", string(suite.lastBody))
}

func (suite *testSuite) assertConversationDetailConsumerMessageImages(messages []conversationMessageResponse, expectedNames []string) error {
	foundConsumerMessage := false
	for _, message := range messages {
		if message.SenderRole == participantRoleConsumer {
			foundConsumerMessage = true
			if err := suite.assertMessageImages(message.Images, expectedNames); err == nil {
				return nil
			}
		}
	}
	if foundConsumerMessage {
		return fmt.Errorf("expected chatbot conversation detail to include a consumer message with images %v, got body %s", expectedNames, string(suite.lastBody))
	}
	return fmt.Errorf("expected chatbot conversation detail to include a consumer message, got body %s", string(suite.lastBody))
}

func (suite *testSuite) assertChatbotWasCalled() error {
	if suite.chatbot.RequestCount() == 0 {
		return fmt.Errorf("expected chatbot to receive a pre-diagnosis request")
	}
	return nil
}

func (suite *testSuite) assertLastChatbotQuestionImages(expectedNames []string) error {
	question := suite.chatbot.LastQuestion()
	if len(question.Images) != len(expectedNames) {
		return fmt.Errorf("expected chatbot question to include %d image(s), got %d", len(expectedNames), len(question.Images))
	}
	for _, expectedName := range expectedNames {
		expectedImage, ok := suite.messageImagesByName[expectedName]
		if !ok {
			return fmt.Errorf("expected image fixture %q to exist", expectedName)
		}
		found := false
		for _, image := range question.Images {
			if image.FileID == expectedImage.FileID && image.OriginalName == expectedName && image.MimeType == expectedImage.MimeType && len(image.Data) == expectedImage.SizeBytes {
				found = true
				break
			}
		}
		if !found {
			return fmt.Errorf("expected chatbot question images to include %q, got %#v", expectedName, question.Images)
		}
	}
	return nil
}
