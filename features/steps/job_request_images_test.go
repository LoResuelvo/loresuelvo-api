package steps_test

import (
	"fmt"
	"net/http"

	"github.com/cucumber/godog"
)

const (
	jobRequestImagePurpose          = "job_request_image"
	jobRequestImageUnavailableError = "Job request image is not available"
)

func registerJobRequestImagesSteps(sc *godog.ScenarioContext, suite *testSuite) {
	sc.Step(`^que cargué y confirmé la imagen de solicitud de trabajo "([^"]*)"$`, suite.uploadAndConfirmJobRequestImage)
	sc.Step(`^que cargué y confirmé las imágenes de solicitud de trabajo: "([^"]*)", "([^"]*)", "([^"]*)"$`, suite.uploadAndConfirmThreeJobRequestImages)
	sc.Step(`^que cargué y confirmé las imágenes de solicitud de trabajo: "([^"]*)", "([^"]*)", "([^"]*)", "([^"]*)"$`, suite.uploadAndConfirmFourJobRequestImages)
	sc.Step(`^que existe una solicitud de trabajo pendiente para el prestador "([^"]*)" con la imagen "([^"]*)"$`, suite.thereIsPendingJobRequestForProviderWithImage)

	sc.Step(`^envío una solicitud de trabajo al prestador "([^"]*)" con el título "([^"]*)" y la imagen cargada "([^"]*)":$`, suite.sendJobRequestToProviderWithImage)
	sc.Step(`^envío una solicitud de trabajo al prestador "([^"]*)" con el título "([^"]*)" y las imágenes cargadas:$`, suite.sendJobRequestToProviderWithUploadedImages)
	sc.Step(`^intento enviar una solicitud de trabajo al prestador "([^"]*)" con el título "([^"]*)" y las imágenes cargadas:$`, suite.trySendJobRequestToProviderWithUploadedImages)

	sc.Step(`^el sistema registra la solicitud de trabajo con la imagen "([^"]*)"$`, suite.systemRegistersJobRequestWithImage)
	sc.Step(`^el sistema registra la solicitud de trabajo con las imágenes adjuntas$`, suite.systemRegistersJobRequestWithUploadedImages)
	sc.Step(`^el sistema rechaza la solicitud de trabajo porque supera el límite de imágenes$`, suite.systemReportsTooManyJobRequestImages)
	sc.Step(`^el sistema no asocia las imágenes a ninguna solicitud de trabajo$`, suite.systemDoesNotAssociateImagesWithAnyJobRequest)
	sc.Step(`^el sistema me muestra la solicitud de trabajo con la imagen "([^"]*)"$`, suite.systemShowsPendingJobRequestWithImage)
}

func (suite *testSuite) uploadAndConfirmJobRequestImage(name string) error {
	return suite.uploadAndConfirmJobRequestImages(name)
}

func (suite *testSuite) uploadAndConfirmThreeJobRequestImages(firstName, secondName, thirdName string) error {
	return suite.uploadAndConfirmJobRequestImages(firstName, secondName, thirdName)
}

func (suite *testSuite) uploadAndConfirmFourJobRequestImages(firstName, secondName, thirdName, fourthName string) error {
	return suite.uploadAndConfirmJobRequestImages(firstName, secondName, thirdName, fourthName)
}

func (suite *testSuite) uploadAndConfirmJobRequestImages(names ...string) error {
	for _, name := range names {
		if err := suite.uploadAndRememberImage(suite.currentAuth0ID, name, jobRequestImagePurpose, true); err != nil {
			return err
		}
	}
	suite.lastAttemptedMessageImageNames = append([]string(nil), names...)
	return nil
}

func (suite *testSuite) thereIsPendingJobRequestForProviderWithImage(providerEmail, imageName string) error {
	previousAuth0ID := suite.currentAuth0ID
	consumerEmail := "consumidor.imagen.solicitud@example.com"

	if err := suite.thereIsRegisteredConsumerWithEmailNameAndSurname(consumerEmail, "Consumidor", "Imagen"); err != nil {
		return err
	}
	suite.currentAuth0ID = auth0IDForConsumerEmail(consumerEmail)
	if err := suite.uploadAndConfirmJobRequestImage(imageName); err != nil {
		return err
	}
	fileIDs, err := suite.messageImageFileIDs([]string{imageName})
	if err != nil {
		return err
	}
	suite.lastAttemptedMessageImageNames = []string{imageName}
	if err := suite.requestJobRequestToProviderEmail(providerEmail, jobRequestPayload{
		title:        "Reparación con imagen",
		description:  "Solicitud pendiente preparada con imagen",
		imageFileIDs: fileIDs,
	}); err != nil {
		return err
	}
	if suite.lastStatus != http.StatusCreated {
		return fmt.Errorf("could not prepare pending job request with image: status %d, body %s", suite.lastStatus, string(suite.lastBody))
	}

	suite.currentAuth0ID = previousAuth0ID
	return nil
}

func (suite *testSuite) requestJobRequestToProviderEmail(providerEmail string, payload jobRequestPayload) error {
	providerID, err := suite.providerIDByEmail(providerEmail)
	if err != nil {
		return err
	}
	suite.lastWorkRequestProviderID = providerID
	return suite.requestJobRequest(jobRequestCreationRequest{
		ProviderID:   providerID,
		Title:        payload.title,
		Description:  payload.description,
		ImageFileIDs: payload.imageFileIDs,
	})
}

func (suite *testSuite) sendJobRequestToProviderWithImage(providerFullName, title, imageName string, description *godog.DocString) error {
	return suite.requestJobRequestToProviderWithImages(providerFullName, title, normalizeDocString(description), []string{imageName})
}

func (suite *testSuite) sendJobRequestToProviderWithUploadedImages(providerFullName, title string, description *godog.DocString) error {
	return suite.requestJobRequestToProviderWithImages(providerFullName, title, normalizeDocString(description), suite.lastAttemptedMessageImageNames)
}

func (suite *testSuite) trySendJobRequestToProviderWithUploadedImages(providerFullName, title string, description *godog.DocString) error {
	return suite.requestJobRequestToProviderWithImages(providerFullName, title, normalizeDocString(description), suite.lastAttemptedMessageImageNames)
}

func (suite *testSuite) requestJobRequestToProviderWithImages(providerFullName, title, description string, imageNames []string) error {
	fileIDs, err := suite.messageImageFileIDs(imageNames)
	if err != nil {
		return err
	}
	suite.lastAttemptedMessageImageNames = append([]string(nil), imageNames...)
	return suite.requestJobRequestToProviderFullName(providerFullName, jobRequestPayload{
		title:        title,
		description:  description,
		imageFileIDs: fileIDs,
	})
}

func (suite *testSuite) systemRegistersJobRequestWithImage(imageName string) error {
	return suite.systemRegistersJobRequestWithImages([]string{imageName})
}

func (suite *testSuite) systemRegistersJobRequestWithUploadedImages() error {
	return suite.systemRegistersJobRequestWithImages(suite.lastAttemptedMessageImageNames)
}

func (suite *testSuite) systemRegistersJobRequestWithImages(expectedNames []string) error {
	if err := suite.lastResponseShouldHaveStatusCode(http.StatusCreated); err != nil {
		return err
	}
	response, err := suite.jobRequestCreationResponseFromLastBody()
	if err != nil {
		return err
	}
	return suite.assertMessageImages(response.Images, expectedNames)
}

func (suite *testSuite) systemReportsTooManyJobRequestImages() error {
	if err := suite.lastResponseShouldHaveStatusCode(http.StatusBadRequest); err != nil {
		return err
	}
	return suite.lastErrorResponseShouldSay(jobRequestImageUnavailableError)
}

func (suite *testSuite) systemDoesNotAssociateImagesWithAnyJobRequest() error {
	if err := suite.requestMyPendingJobRequests(); err != nil {
		return err
	}
	jobRequests, err := suite.jobRequestListResponseShouldHaveStatusCode(http.StatusOK)
	if err != nil {
		return err
	}
	if len(jobRequests) != 0 {
		return fmt.Errorf("expected no pending job requests after rejected image attachment, got %d", len(jobRequests))
	}
	return nil
}

func (suite *testSuite) systemShowsPendingJobRequestWithImage(imageName string) error {
	jobRequests, err := suite.jobRequestListResponseShouldHaveStatusCode(http.StatusOK)
	if err != nil {
		return err
	}
	for _, jobRequest := range jobRequests {
		if err := suite.assertMessageImages(jobRequest.Images, []string{imageName}); err == nil {
			return nil
		}
	}
	return fmt.Errorf("expected pending job request list to include image %q, got body %s", imageName, string(suite.lastBody))
}
