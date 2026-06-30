package steps_test

import (
	"context"
	"fmt"
	"reflect"
	"strings"

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
	return suite.prepareProfessionalAssessmentWithSelectedImages(suite.allRememberedMessageImageNames())
}

func (suite *testSuite) currentProfessionalAssessmentSelectedImages(firstName, secondName string) error {
	return suite.prepareProfessionalAssessmentWithSelectedImages([]string{firstName, secondName})
}

func (suite *testSuite) currentProfessionalAssessmentSelectedImage(imageName string) error {
	return suite.prepareProfessionalAssessmentWithSelectedImages([]string{imageName})
}

func (suite *testSuite) prepareProfessionalAssessmentWithSelectedImages(imageNames []string) error {
	suite.expectedAssessmentImageNames = append([]string(nil), imageNames...)
	if err := suite.prepareProfessionalAssessment("Plomería", "Pérdida de agua", "Hay evidencia de una pérdida que requiere intervención profesional."); err != nil {
		return err
	}
	return suite.assertCurrentAssessmentImages(imageNames)
}

func (suite *testSuite) jobRequestContainsImages(firstName, secondName string) error {
	request, err := suite.lastAIJobRequest()
	if err != nil {
		return err
	}
	return suite.assertMessageImagesFromDomain(request.Images, []string{firstName, secondName})
}

func (suite *testSuite) jobRequestContainsImage(imageName string) error {
	request, err := suite.lastAIJobRequest()
	if err != nil {
		return err
	}
	return suite.assertMessageImagesFromDomain(request.Images, []string{imageName})
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
	return suite.rememberCreatedChatbotConversation()
}

func (suite *testSuite) newInformationProducesProfessionalAssessmentForCategory(categoryName string) error {
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
	suite.chatbot.SetConcludedDiagnosisResponse("Pérdida de agua", "Hay evidencia de una pérdida que requiere intervención profesional.", "Plomería")
	if err := suite.requestContinueChatbotConversation(
		suite.lastConversationID,
		chatbotConversationRequest{Content: "La nueva imagen muestra mejor el mismo problema."},
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

func (suite *testSuite) currentAssessmentImages() ([]any, error) {
	foundConversation, err := suite.conversationRepository.FindByID(context.Background(), suite.lastConversationID)
	if err != nil {
		return nil, err
	}
	images, err := nestedExportedSliceField(foundConversation, "CurrentAssessment", "Images")
	if err != nil {
		return nil, err
	}
	return images, nil
}

func (suite *testSuite) assertMessageImagesFromDomain(images any, expectedNames []string) error {
	reflected := reflect.ValueOf(images)
	values := make([]any, reflected.Len())
	for index := 0; index < reflected.Len(); index++ {
		values[index] = reflected.Index(index).Interface()
	}
	return assertDomainImageNames(values, expectedNames)
}

func nestedExportedSliceField(value any, parentFieldName, childFieldName string) ([]any, error) {
	reflected := reflect.ValueOf(value)
	if reflected.Kind() == reflect.Pointer {
		if reflected.IsNil() {
			return nil, fmt.Errorf("expected non-nil value with field %s", parentFieldName)
		}
		reflected = reflected.Elem()
	}
	parent := reflected.FieldByName(parentFieldName)
	if !parent.IsValid() {
		return nil, fmt.Errorf("expected field %s", parentFieldName)
	}
	if parent.Kind() == reflect.Pointer {
		if parent.IsNil() {
			return nil, fmt.Errorf("expected non-nil field %s", parentFieldName)
		}
		parent = parent.Elem()
	}
	child := parent.FieldByName(childFieldName)
	if !child.IsValid() || child.Kind() != reflect.Slice {
		return nil, fmt.Errorf("expected slice field %s.%s", parentFieldName, childFieldName)
	}
	result := make([]any, child.Len())
	for index := 0; index < child.Len(); index++ {
		result[index] = child.Index(index).Interface()
	}
	return result, nil
}

func assertDomainImageNames(images []any, expectedNames []string) error {
	actualNames := make([]string, 0, len(images))
	for _, image := range images {
		name, err := exportedStringField(image, "OriginalName")
		if err != nil {
			return err
		}
		actualNames = append(actualNames, name)
	}
	if strings.Join(actualNames, "\x00") != strings.Join(expectedNames, "\x00") {
		return fmt.Errorf("expected image names %v, got %v", expectedNames, actualNames)
	}
	return nil
}
