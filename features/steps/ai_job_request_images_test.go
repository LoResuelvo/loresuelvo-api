package steps_test

import (
	"context"
	"fmt"
	"strings"

	"github.com/LoResuelvo/loresuelvo-api/internal/domain/conversation"
	filedomain "github.com/LoResuelvo/loresuelvo-api/internal/domain/file"
	"github.com/cucumber/godog"
)

func registerAIJobRequestImageSteps(sc *godog.ScenarioContext, suite *testSuite) {
	sc.Step(`^que envié al chatbot las imágenes "([^"]*)" y "([^"]*)"$`, suite.iSentImagesToChatbot)
	sc.Step(`^que la evaluación profesional vigente seleccionó ambas imágenes como evidencia del problema$`, suite.currentProfessionalAssessmentSelectedBothImages)
	sc.Step(`^la solicitud contiene las imágenes "([^"]*)" y "([^"]*)"$`, suite.jobRequestContainsImages)
	sc.Step(`^la solicitud queda vinculada con la evaluación que seleccionó esas imágenes$`, suite.jobRequestIsLinkedToCurrentAssessment)
	sc.Step(`^que envié al chatbot las imágenes "([^"]*)", "([^"]*)" y "([^"]*)"$`, suite.iSentThreeImagesToChatbot)
	sc.Step(`^que la evaluación profesional vigente seleccionó "([^"]*)" y "([^"]*)" como evidencia del problema de plomería$`, suite.currentProfessionalAssessmentSelectedImages)
	sc.Step(`^la solicitud no contiene la imagen "([^"]*)"$`, suite.jobRequestDoesNotContainImage)
	sc.Step(`^que envié la imagen "([^"]*)" mientras la evaluación todavía requería más información$`, suite.iSentImageWhileAssessmentNeededMoreInformation)
	sc.Step(`^que después de aportar nueva información la evaluación vigente requiere un profesional del rubro "([^"]*)"$`, suite.newInformationProducesProfessionalAssessmentForCategory)
	sc.Step(`^que la evaluación vigente seleccionó la imagen histórica "([^"]*)"$`, suite.currentAssessmentSelectedHistoricalImage)
	sc.Step(`^la solicitud contiene la imagen "([^"]*)"$`, suite.jobRequestContainsImage)
	sc.Step(`^la solicitud queda vinculada con la evaluación profesional vigente$`, suite.jobRequestIsLinkedToCurrentAssessment)
	sc.Step(`^que la evaluación profesional vigente seleccionó la imagen "([^"]*)"$`, suite.currentProfessionalAssessmentSelectedImage)
	sc.Step(`^que el título, la descripción, el resultado y el rubro evaluados no cambiaron$`, suite.assessmentContentDidNotChange)
	sc.Step(`^el chatbot reemplaza la selección por la imagen "([^"]*)"$`, suite.chatbotReplacesAssessmentImageSelection)
	sc.Step(`^el sistema registra una nueva revisión de la evaluación$`, suite.systemRegistersNewAssessmentRevision)
	sc.Step(`^la nueva evaluación selecciona solamente la imagen "([^"]*)"$`, suite.newAssessmentSelectsOnlyImage)
	sc.Step(`^la evaluación anterior conserva la imagen "([^"]*)"$`, suite.previousAssessmentKeepsImage)
}

func (suite *testSuite) iSentImagesToChatbot(firstName, secondName string) error {
	return suite.sendImagesToChatbot(firstName, secondName)
}

func (suite *testSuite) iSentThreeImagesToChatbot(firstName, secondName, thirdName string) error {
	return suite.sendImagesToChatbot(firstName, secondName, thirdName)
}

func (suite *testSuite) sendImagesToChatbot(names ...string) error {
	for _, name := range names {
		if err := suite.uploadAndConfirmMessageImage(name); err != nil {
			return err
		}
	}
	if err := suite.requestCreateChatbotConversationWithImages("Adjunto evidencia visual del problema.", names); err != nil {
		return err
	}
	return suite.rememberCreatedChatbotConversation()
}

func (suite *testSuite) currentProfessionalAssessmentSelectedBothImages() error {
	return suite.prepareProfessionalAssessmentWithSelectedImages(suite.lastAttemptedMessageImageNames)
}

func (suite *testSuite) currentProfessionalAssessmentSelectedImages(firstName, secondName string) error {
	return suite.prepareProfessionalAssessmentWithSelectedImages([]string{firstName, secondName})
}

func (suite *testSuite) currentProfessionalAssessmentSelectedImage(imageName string) error {
	if suite.lastConversationID == 0 {
		if err := suite.sendImagesToChatbot(imageName); err != nil {
			return err
		}
	}
	return suite.prepareProfessionalAssessmentWithSelectedImages([]string{imageName})
}

func (suite *testSuite) prepareProfessionalAssessmentWithSelectedImages(imageNames []string) error {
	suite.expectedAssessmentImageNames = append([]string(nil), imageNames...)
	suite.chatbot.SetSelectedImageNames(imageNames...)
	suite.chatbot.SetConcludedDiagnosisResponse("Pérdida de agua", "Hay evidencia de una pérdida que requiere intervención profesional.", "Plomería")
	if err := suite.requestContinueChatbotConversation(
		suite.lastConversationID,
		chatbotConversationRequest{Content: "La evidencia confirma que necesito ayuda profesional."},
	); err != nil {
		return err
	}
	suite.aiSourceChatbotConversationID = suite.lastConversationID
	if err := suite.rememberCurrentAssessmentIDIfAvailable(); err != nil {
		return err
	}
	return suite.assertCurrentAssessmentImages(imageNames)
}

func (suite *testSuite) jobRequestContainsImages(firstName, secondName string) error {
	request, err := suite.lastAIJobRequest()
	if err != nil {
		return err
	}
	return assertImageNames(request.Images, []string{firstName, secondName})
}

func (suite *testSuite) jobRequestContainsImage(imageName string) error {
	request, err := suite.lastAIJobRequest()
	if err != nil {
		return err
	}
	return assertImageNames(request.Images, []string{imageName})
}

func (suite *testSuite) jobRequestDoesNotContainImage(imageName string) error {
	request, err := suite.lastAIJobRequest()
	if err != nil {
		return err
	}
	for _, image := range request.Images {
		if image.OriginalName == imageName {
			return fmt.Errorf("expected job request not to contain image %q", imageName)
		}
	}
	return nil
}

func (suite *testSuite) iSentImageWhileAssessmentNeededMoreInformation(imageName string) error {
	if err := suite.uploadAndConfirmMessageImage(imageName); err != nil {
		return err
	}
	if err := suite.requestCreateChatbotConversationWithImages("Adjunto una imagen mientras completo la información.", []string{imageName}); err != nil {
		return err
	}
	if err := suite.rememberCreatedChatbotConversation(); err != nil {
		return err
	}
	suite.aiSourceChatbotConversationID = suite.lastConversationID
	return nil
}

func (suite *testSuite) newInformationProducesProfessionalAssessmentForCategory(categoryName string) error {
	suite.chatbot.SetSelectedImageNames(suite.allRememberedMessageImageNames()...)
	suite.chatbot.SetConcludedDiagnosisResponse("Humedad en pared", "La humedad requiere revisión profesional.", categoryName)
	if err := suite.requestContinueChatbotConversation(
		suite.lastConversationID,
		chatbotConversationRequest{Content: "La mancha continúa creciendo."},
	); err != nil {
		return err
	}
	return suite.rememberCurrentAssessmentIDIfAvailable()
}

func (suite *testSuite) currentAssessmentSelectedHistoricalImage(imageName string) error {
	suite.expectedAssessmentImageNames = []string{imageName}
	return suite.assertCurrentAssessmentImages([]string{imageName})
}

func (suite *testSuite) assessmentContentDidNotChange() error {
	suite.previousAssessmentID = suite.aiExpectedAssessmentID
	images, err := suite.currentAssessmentImages()
	if err != nil {
		return err
	}
	suite.previousAssessmentImages = images
	return nil
}

func (suite *testSuite) chatbotReplacesAssessmentImageSelection(imageName string) error {
	suite.expectedAssessmentImageNames = []string{imageName}
	if _, exists := suite.messageImagesByName[imageName]; !exists {
		if err := suite.uploadAndConfirmMessageImage(imageName); err != nil {
			return err
		}
	}
	suite.chatbot.SetConcludedDiagnosisResponse("Pérdida de agua", "Hay evidencia de una pérdida que requiere intervención profesional.", "Plomería")
	suite.chatbot.SetSelectedImageNames(imageName)
	if err := suite.requestContinueChatbotConversationWithImages(
		suite.lastConversationID,
		"La nueva imagen muestra mejor el mismo problema.",
		[]string{imageName},
	); err != nil {
		return err
	}
	return suite.rememberCurrentAssessmentIDIfAvailable()
}

func (suite *testSuite) systemRegistersNewAssessmentRevision() error {
	if suite.previousAssessmentID == 0 || suite.aiExpectedAssessmentID == 0 {
		return fmt.Errorf("expected previous and current assessment ids")
	}
	if suite.previousAssessmentID == suite.aiExpectedAssessmentID {
		return fmt.Errorf("expected a new assessment revision, current id remained %d", suite.aiExpectedAssessmentID)
	}
	return nil
}

func (suite *testSuite) newAssessmentSelectsOnlyImage(imageName string) error {
	return suite.assertCurrentAssessmentImages([]string{imageName})
}

func (suite *testSuite) previousAssessmentKeepsImage(imageName string) error {
	return assertDomainImageNames(suite.previousAssessmentImages, []string{imageName})
}

func (suite *testSuite) assertCurrentAssessmentImages(expectedNames []string) error {
	images, err := suite.currentAssessmentImages()
	if err != nil {
		return err
	}
	return assertDomainImageNames(images, expectedNames)
}

func (suite *testSuite) currentAssessmentImages() ([]filedomain.MessageImage, error) {
	foundConversation, err := suite.conversationRepository.FindByID(context.Background(), suite.lastConversationID)
	if err != nil {
		return nil, err
	}
	chatbotConversation, ok := foundConversation.(*conversation.ChatBotConversation)
	if !ok || chatbotConversation.CurrentAssessment == nil {
		return nil, fmt.Errorf("expected chatbot conversation with current assessment")
	}
	return append([]filedomain.MessageImage(nil), chatbotConversation.CurrentAssessment.Images...), nil
}

func assertDomainImageNames(images []filedomain.MessageImage, expectedNames []string) error {
	actualNames := make([]string, 0, len(images))
	for _, image := range images {
		actualNames = append(actualNames, strings.TrimSpace(image.OriginalName))
	}
	if strings.Join(actualNames, "\x00") != strings.Join(expectedNames, "\x00") {
		return fmt.Errorf("expected image names %v, got %v", expectedNames, actualNames)
	}
	return nil
}

func assertImageNames(images []filedomain.Image, expectedNames []string) error {
	actualNames := make([]string, 0, len(images))
	for _, image := range images {
		actualNames = append(actualNames, strings.TrimSpace(image.OriginalName))
	}
	if strings.Join(actualNames, "\x00") != strings.Join(expectedNames, "\x00") {
		return fmt.Errorf("expected image names %v, got %v", expectedNames, actualNames)
	}
	return nil
}
