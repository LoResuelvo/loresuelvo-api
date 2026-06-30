package steps_test

import (
	"context"
	"fmt"
	"net/http"
	"strings"

	"github.com/LoResuelvo/loresuelvo-api/internal/domain/conversation"
	"github.com/cucumber/godog"
)

func registerChatbotImageContextSteps(sc *godog.ScenarioContext, suite *testSuite) {
	sc.Step(`^que envié al chatbot la imagen "([^"]*)"$`, suite.iSentImageToChatbot)
	sc.Step(`^que el chatbot describió la imagen "([^"]*)" como:$`, suite.chatbotDescribedImageAs)
	sc.Step(`^envío un nuevo mensaje sin imágenes en esa conversación:$`, suite.sendNewMessageWithoutImages)
	sc.Step(`^el chatbot recibe como contexto la descripción de la imagen "([^"]*)"$`, suite.chatbotReceivesHistoricalImageDescription)
	sc.Step(`^el chatbot no vuelve a recibir el contenido binario de la imagen "([^"]*)"$`, suite.chatbotDoesNotReceiveHistoricalImageBytes)
	sc.Step(`^que continué la conversación hasta que fue necesario resumir su contexto$`, suite.iContinuedConversationUntilContextSummaryWasNeeded)
	sc.Step(`^envío una consulta posterior sobre la mancha de humedad$`, suite.sendLaterQuestionAboutDampStain)
	sc.Step(`^el resumen acumulado conserva la descripción de la imagen "([^"]*)"$`, suite.accumulatedSummaryKeepsImageDescription)
	sc.Step(`^el chatbot puede relacionar la consulta posterior con esa evidencia visual$`, suite.chatbotCanRelateLaterQuestionToVisualEvidence)
	sc.Step(`^envío un nuevo mensaje con la imagen cargada "([^"]*)":$`, suite.sendNewMessageWithLoadedImage)
	sc.Step(`^el chatbot recibe el contenido binario de la imagen nueva "([^"]*)"$`, suite.chatbotReceivesNewImageBytes)
	sc.Step(`^el chatbot recibe como texto la descripción histórica de la imagen "([^"]*)"$`, suite.chatbotReceivesHistoricalImageDescription)
	sc.Step(`^que el chatbot devolverá una descripción solamente para "([^"]*)"$`, suite.chatbotWillDescribeOnlyImage)
	sc.Step(`^intento enviar ambas imágenes en un nuevo mensaje al chatbot$`, suite.trySendBothImagesInNewChatbotMessage)
	sc.Step(`^que el chatbot devolverá una descripción para una referencia de imagen desconocida$`, suite.chatbotWillDescribeUnknownImageReference)
	sc.Step(`^intento enviar la imagen en un nuevo mensaje al chatbot$`, suite.trySendRememberedImageInNewChatbotMessage)
	sc.Step(`^el sistema rechaza la consulta por una respuesta inválida del chatbot$`, suite.systemRejectsInvalidChatbotResponse)
	sc.Step(`^el sistema no registra el mensaje con las imágenes$`, suite.systemDoesNotPersistAttemptedImageMessage)
	sc.Step(`^el sistema no registra el mensaje con la imagen$`, suite.systemDoesNotPersistAttemptedImageMessage)
	sc.Step(`^el sistema no registra la respuesta del chatbot$`, suite.systemDoesNotPersistChatbotResponse)
}

func (suite *testSuite) iSentImageToChatbot(imageName string) error {
	suite.chatbot.SetImageDescription(imageName, chatbotImageFixtureDescription(imageName))
	if err := suite.uploadAndConfirmMessageImage(imageName); err != nil {
		return err
	}
	if err := suite.requestCreateChatbotConversationWithImages("Necesito orientación con esta imagen.", []string{imageName}); err != nil {
		return err
	}
	return suite.rememberCreatedChatbotConversation()
}

func chatbotImageFixtureDescription(imageName string) string {
	switch imageName {
	case "perdida-bajo-mesada.jpg":
		return "Se observa agua acumulada debajo del sifón y humedad alrededor de su conexión."
	case "humedad-pared.webp":
		return "Se observa una mancha de humedad ascendente junto al zócalo."
	case "vista-general-cocina.jpg":
		return "Se observa agua debajo de la pileta y humedad en la base del mueble."
	default:
		return "Se observa evidencia visual relevante del problema."
	}
}

func (suite *testSuite) chatbotDescribedImageAs(imageName string, description *godog.DocString) error {
	expected := normalizeDocString(description)
	suite.expectedChatbotImageDescriptions[imageName] = expected

	foundDescription, err := suite.persistedImageDescription(imageName)
	if err != nil {
		return err
	}
	if foundDescription != expected {
		return fmt.Errorf("expected image %q description %q, got %q", imageName, expected, foundDescription)
	}
	return nil
}

func (suite *testSuite) sendNewMessageWithoutImages(message *godog.DocString) error {
	return suite.requestContinueChatbotConversation(
		suite.lastConversationID,
		chatbotConversationRequest{Content: normalizeDocString(message)},
	)
}

func (suite *testSuite) chatbotReceivesHistoricalImageDescription(imageName string) error {
	expected := suite.expectedChatbotImageDescriptions[imageName]
	if expected == "" {
		return fmt.Errorf("expected stored description fixture for image %q", imageName)
	}
	for _, message := range suite.chatbot.LastQuestion().RecentMessages {
		for _, image := range message.Images {
			if image.OriginalName == imageName {
				if image.Description != expected {
					return fmt.Errorf("expected historical image %q description %q, got %q", imageName, expected, image.Description)
				}
				return nil
			}
		}
	}
	if strings.Contains(suite.chatbot.LastQuestion().ContextSummary, expected) {
		return nil
	}
	return fmt.Errorf("expected chatbot context to include description for image %q", imageName)
}

func (suite *testSuite) chatbotDoesNotReceiveHistoricalImageBytes(imageName string) error {
	for _, image := range suite.chatbot.LastQuestion().Images {
		if image.OriginalName == imageName || image.FileID == suite.messageImagesByName[imageName].FileID {
			return fmt.Errorf("expected historical image %q not to be resent as binary content", imageName)
		}
	}
	return nil
}

func (suite *testSuite) iContinuedConversationUntilContextSummaryWasNeeded() error {
	for index := 0; index < conversation.ChatbotRecentMessageLimit; index++ {
		message := mustConsumerMessage(fmt.Sprintf("Información adicional %d sobre el problema.", index+1))
		if _, err := suite.conversationRepository.AddMessage(context.Background(), suite.lastConversationID, message); err != nil {
			return err
		}
	}
	return nil
}

func (suite *testSuite) sendLaterQuestionAboutDampStain() error {
	return suite.requestContinueChatbotConversation(
		suite.lastConversationID,
		chatbotConversationRequest{Content: "¿La mancha cambió de aspecto según la evidencia anterior?"},
	)
}

func (suite *testSuite) accumulatedSummaryKeepsImageDescription(imageName string) error {
	expected := suite.expectedChatbotImageDescriptions[imageName]
	if expected == "" || !strings.Contains(suite.chatbot.LastQuestion().ContextSummary, expected) {
		return fmt.Errorf("expected accumulated summary to contain description %q for image %q, got %q", expected, imageName, suite.chatbot.LastQuestion().ContextSummary)
	}
	return nil
}

func (suite *testSuite) chatbotCanRelateLaterQuestionToVisualEvidence() error {
	return suite.assertChatbotWasCalled()
}

func (suite *testSuite) sendNewMessageWithLoadedImage(imageName string, message *godog.DocString) error {
	return suite.requestContinueChatbotConversationWithImages(
		suite.lastConversationID,
		normalizeDocString(message),
		[]string{imageName},
	)
}

func (suite *testSuite) chatbotReceivesNewImageBytes(imageName string) error {
	return suite.assertLastChatbotQuestionImages([]string{imageName})
}

func (suite *testSuite) chatbotWillDescribeOnlyImage(imageName string) error {
	suite.lastAttemptedMessageImageNames = []string{imageName}
	suite.chatbot.SetImageDescriptionMode("omit_after_first")
	return nil
}

func (suite *testSuite) trySendBothImagesInNewChatbotMessage() error {
	if err := suite.rememberChatbotMessageCountsBeforeAttempt(); err != nil {
		return err
	}
	return suite.requestContinueChatbotConversationWithImages(
		suite.lastConversationID,
		"Te envío ambas imágenes para completar la evaluación.",
		suite.allRememberedMessageImageNames(),
	)
}

func (suite *testSuite) chatbotWillDescribeUnknownImageReference() error {
	suite.chatbot.SetImageDescriptionMode("unknown_ref")
	return nil
}

func (suite *testSuite) trySendRememberedImageInNewChatbotMessage() error {
	if err := suite.rememberChatbotMessageCountsBeforeAttempt(); err != nil {
		return err
	}
	return suite.requestContinueChatbotConversationWithImages(
		suite.lastConversationID,
		"Te envío una imagen para completar la evaluación.",
		suite.allRememberedMessageImageNames(),
	)
}

func (suite *testSuite) systemRejectsInvalidChatbotResponse() error {
	if suite.lastStatus != http.StatusBadRequest {
		return fmt.Errorf("expected invalid chatbot response to return status %d, got %d with body %s", http.StatusBadRequest, suite.lastStatus, string(suite.lastBody))
	}
	return suite.lastResponseShouldHaveError()
}

func (suite *testSuite) systemDoesNotPersistAttemptedImageMessage() error {
	currentCount, err := suite.conversationRepository.CountMessagesBySenderRole(
		context.Background(),
		suite.lastConversationID,
		conversation.SenderConsumer,
	)
	if err != nil {
		return err
	}
	if currentCount != suite.consumerMessageCountBeforeAttempt {
		return fmt.Errorf("expected consumer message count to remain %d, got %d", suite.consumerMessageCountBeforeAttempt, currentCount)
	}
	return nil
}

func (suite *testSuite) systemDoesNotPersistChatbotResponse() error {
	currentCount, err := suite.conversationRepository.CountMessagesBySenderRole(
		context.Background(),
		suite.lastConversationID,
		conversation.SenderChatbot,
	)
	if err != nil {
		return err
	}
	if currentCount != suite.chatbotMessageCountBeforeAttempt {
		return fmt.Errorf("expected chatbot message count to remain %d, got %d", suite.chatbotMessageCountBeforeAttempt, currentCount)
	}
	return nil
}

func (suite *testSuite) rememberChatbotMessageCountsBeforeAttempt() error {
	consumerCount, err := suite.conversationRepository.CountMessagesBySenderRole(
		context.Background(),
		suite.lastConversationID,
		conversation.SenderConsumer,
	)
	if err != nil {
		return err
	}
	chatbotCount, err := suite.conversationRepository.CountMessagesBySenderRole(
		context.Background(),
		suite.lastConversationID,
		conversation.SenderChatbot,
	)
	if err != nil {
		return err
	}
	suite.consumerMessageCountBeforeAttempt = consumerCount
	suite.chatbotMessageCountBeforeAttempt = chatbotCount
	return nil
}

func (suite *testSuite) persistedImageDescription(imageName string) (string, error) {
	foundConversation, err := suite.conversationRepository.FindByID(context.Background(), suite.lastConversationID)
	if err != nil {
		return "", err
	}
	for _, message := range foundConversation.Messages() {
		for _, image := range message.Images {
			if image.OriginalName == imageName {
				return strings.TrimSpace(image.Description), nil
			}
		}
	}
	return "", fmt.Errorf("expected persisted image %q in chatbot conversation", imageName)
}
