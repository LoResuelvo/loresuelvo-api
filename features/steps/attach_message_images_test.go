package steps_test

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"path/filepath"
	"strings"

	"github.com/cucumber/godog"
)

const (
	conversationMessageImagePurpose = "conversation_message_image"
	validMessageImageSizeBytes      = 1024 * 1024
	messageImageUnavailableError    = "Message image is not available"
)

type messageImageFixture struct {
	FileID       string
	OriginalName string
	MimeType     string
	SizeBytes    int
}

type messageImageResponse struct {
	ID           string `json:"id"`
	URL          string `json:"url"`
	OriginalName string `json:"original_name"`
}

func registerAttachMessageImagesSteps(sc *godog.ScenarioContext, suite *testSuite) {
	sc.Step(`^que cargué y confirmé la imagen "([^"]*)"$`, suite.uploadAndConfirmMessageImage)
	sc.Step(`^que cargué y confirmé las imágenes: "([^"]*)", "([^"]*)"$`, suite.uploadAndConfirmTwoMessageImages)
	sc.Step(`^que cargué pero no confirmé la imagen "([^"]*)"$`, suite.uploadPendingMessageImage)
	sc.Step(`^que la consumidora "([^"]*)" cargó y confirmó la imagen "([^"]*)"$`, suite.consumerUploadedAndConfirmedMessageImage)
	sc.Step(`^que cargué y confirmé la imagen "([^"]*)" como foto de perfil$`, suite.uploadAndConfirmProfilePhotoImage)
	sc.Step(`^que el consumidor "([^"]*)" envió un mensaje con la imagen "([^"]*)" en el chat$`, suite.consumerSentMessageWithImageInChat)

	sc.Step(`^envío un mensaje en el chat con el prestador "([^"]*)" con la imagen cargada "([^"]*)":$`, suite.sendMessageWithImageInChatWithProvider)
	sc.Step(`^envío un mensaje en el chat con la consumidora "([^"]*)" con la imagen cargada "([^"]*)":$`, suite.sendMessageWithImageInChatWithConsumer)
	sc.Step(`^envío un mensaje sin texto en el chat con el prestador "([^"]*)" con la imagen cargada "([^"]*)"$`, suite.sendImageOnlyMessageInChatWithProvider)
	sc.Step(`^envío un mensaje en el chat con el prestador "([^"]*)" con las imágenes cargadas:$`, suite.sendMessageWithUploadedImagesInChatWithProvider)
	sc.Step(`^intento enviar un mensaje en el chat con el prestador "([^"]*)" adjuntando la imagen "([^"]*)"$`, suite.trySendMessageWithImageInChatWithProvider)
	sc.Step(`^intento enviar un mensaje en el chat con la consumidora "([^"]*)" adjuntando la imagen "([^"]*)"$`, suite.trySendMessageWithImageInChatWithConsumer)
	sc.Step(`^consulto el chat activo con el consumidor "([^"]*)"$`, suite.requestActiveChatWithConsumer)
	sc.Step(`^intento acceder a la imagen "([^"]*)" adjunta al mensaje$`, suite.tryAccessMessageImage)

	sc.Step(`^el sistema registra el mensaje con la imagen "([^"]*)"$`, suite.systemRegistersMessageWithImage)
	sc.Step(`^la imagen queda asociada al mensaje enviado$`, suite.imageRemainsAssociatedWithSentMessage)
	sc.Step(`^el sistema registra el mensaje con las dos imágenes$`, suite.systemRegistersMessageWithTwoImages)
	sc.Step(`^el detalle del mensaje incluye la imagen "([^"]*)"$`, suite.messageDetailIncludesImage)
	sc.Step(`^el sistema permite al prestador acceder a la imagen adjunta$`, suite.systemAllowsProviderToAccessAttachedImage)
	sc.Step(`^el prestador "([^"]*)" recibe en tiempo real el mensaje con la imagen "([^"]*)"$`, suite.providerReceivesRealtimeMessageWithImage)
	sc.Step(`^el sistema rechaza el mensaje porque la imagen no está disponible$`, suite.systemRejectsMessageBecauseImageIsUnavailable)
	sc.Step(`^el sistema no asocia la imagen a ningún mensaje$`, suite.systemDoesNotAssociateImageWithAnyMessage)
	sc.Step(`^el sistema me indica que no puedo acceder a esa imagen$`, suite.systemReportsMessageImageAccessDenied)
}

func (suite *testSuite) uploadAndConfirmMessageImage(name string) error {
	return suite.uploadAndRememberImage(suite.currentAuth0ID, name, conversationMessageImagePurpose, true)
}

func (suite *testSuite) uploadAndConfirmTwoMessageImages(firstName, secondName string) error {
	if err := suite.uploadAndConfirmMessageImage(firstName); err != nil {
		return err
	}
	return suite.uploadAndConfirmMessageImage(secondName)
}

func (suite *testSuite) uploadPendingMessageImage(name string) error {
	return suite.uploadAndRememberImage(suite.currentAuth0ID, name, conversationMessageImagePurpose, false)
}

func (suite *testSuite) consumerUploadedAndConfirmedMessageImage(email, name string) error {
	return suite.uploadAndRememberImage(auth0IDForConsumerEmail(email), name, conversationMessageImagePurpose, true)
}

func (suite *testSuite) uploadAndConfirmProfilePhotoImage(name string) error {
	return suite.uploadAndRememberImage(suite.currentAuth0ID, name, providerProfilePhotoPurpose, true)
}

func (suite *testSuite) uploadAndRememberImage(authID, name, purpose string, confirm bool) error {
	if strings.TrimSpace(authID) == "" {
		return fmt.Errorf("expected an authenticated uploader before loading image %q", name)
	}

	mimeType, err := messageImageMIMEType(name)
	if err != nil {
		return err
	}
	upload, err := suite.requestProfilePhotoPresign(authID, presignFileRequest{
		OriginalName: name,
		MimeType:     mimeType,
		SizeBytes:    validMessageImageSizeBytes,
		Purpose:      purpose,
	})
	if err != nil {
		return err
	}
	if err := suite.lastResponseShouldHaveStatusCode(http.StatusOK); err != nil {
		return err
	}
	if upload.FileID == "" || upload.Key == "" || upload.UploadURL == "" {
		return fmt.Errorf("expected presign response for image %q to include file_id, key and upload_url, got body %s", name, string(suite.lastBody))
	}

	fixture := messageImageFixture{
		FileID:       upload.FileID,
		OriginalName: name,
		MimeType:     mimeType,
		SizeBytes:    validMessageImageSizeBytes,
	}
	suite.messageImagesByName[name] = fixture
	if !confirm {
		return nil
	}

	if err := suite.putProfilePhotoObject(*upload, mimeType, validMessageImageSizeBytes); err != nil {
		return err
	}
	return suite.confirmFileUpload(authID, *upload, fixture)
}

func (suite *testSuite) confirmFileUpload(authID string, upload presignFileResponse, fixture messageImageFixture) error {
	resp, err := suite.postJSONWithAuth(authID, "/files/"+upload.FileID+"/confirm", confirmFileRequest{
		Key:       upload.Key,
		MimeType:  fixture.MimeType,
		SizeBytes: fixture.SizeBytes,
	})
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return fmt.Errorf("failed to read file confirmation response: %w", err)
	}
	suite.lastStatus = resp.StatusCode
	suite.lastBody = body
	return suite.lastResponseShouldHaveStatusCode(http.StatusOK)
}

func messageImageMIMEType(name string) (string, error) {
	switch strings.ToLower(filepath.Ext(name)) {
	case ".jpg", ".jpeg":
		return "image/jpeg", nil
	case ".png":
		return "image/png", nil
	case ".webp":
		return "image/webp", nil
	default:
		return "", fmt.Errorf("unsupported test image extension for %q", name)
	}
}

func (suite *testSuite) consumerSentMessageWithImageInChat(email, imageName string) error {
	suite.currentAuth0ID = auth0IDForConsumerEmail(email)
	if err := suite.uploadAndConfirmMessageImage(imageName); err != nil {
		return err
	}
	return suite.requestMessageWithImages("Imagen adjunta para mostrar el problema.", []string{imageName})
}

func (suite *testSuite) sendMessageWithImageInChatWithProvider(providerFullName, imageName string, message *godog.DocString) error {
	if err := suite.ensureKnownParticipantFullName(providerFullName, participantRoleProvider); err != nil {
		return err
	}
	return suite.requestMessageWithImages(normalizeDocString(message), []string{imageName})
}

func (suite *testSuite) sendMessageWithImageInChatWithConsumer(consumerFullName, imageName string, message *godog.DocString) error {
	if err := suite.ensureKnownParticipantFullName(consumerFullName, participantRoleConsumer); err != nil {
		return err
	}
	return suite.requestMessageWithImages(normalizeDocString(message), []string{imageName})
}

func (suite *testSuite) sendImageOnlyMessageInChatWithProvider(providerFullName, imageName string) error {
	if err := suite.ensureKnownParticipantFullName(providerFullName, participantRoleProvider); err != nil {
		return err
	}
	return suite.requestMessageWithImages("", []string{imageName})
}

func (suite *testSuite) sendMessageWithUploadedImagesInChatWithProvider(providerFullName string, message *godog.DocString) error {
	if err := suite.ensureKnownParticipantFullName(providerFullName, participantRoleProvider); err != nil {
		return err
	}
	return suite.requestMessageWithImages(normalizeDocString(message), suite.allRememberedMessageImageNames())
}

func (suite *testSuite) trySendMessageWithImageInChatWithProvider(providerFullName, imageName string) error {
	if err := suite.ensureKnownParticipantFullName(providerFullName, participantRoleProvider); err != nil {
		return err
	}
	return suite.requestMessageWithImages("Intento adjuntar una imagen.", []string{imageName})
}

func (suite *testSuite) trySendMessageWithImageInChatWithConsumer(consumerFullName, imageName string) error {
	if err := suite.ensureKnownParticipantFullName(consumerFullName, participantRoleConsumer); err != nil {
		return err
	}
	return suite.requestMessageWithImages("Intento adjuntar una imagen.", []string{imageName})
}

func (suite *testSuite) requestMessageWithImages(content string, imageNames []string) error {
	fileIDs, err := suite.messageImageFileIDs(imageNames)
	if err != nil {
		return err
	}
	suite.lastAttemptedMessageImageNames = append([]string(nil), imageNames...)
	return suite.requestSendMessageToPreparedConversation(sendMessageRequest{Content: content, ImageFileIDs: fileIDs})
}

func (suite *testSuite) messageImageFileIDs(names []string) ([]string, error) {
	fileIDs := make([]string, 0, len(names))
	for _, name := range names {
		fixture, ok := suite.messageImagesByName[name]
		if !ok {
			return nil, fmt.Errorf("expected image %q to be loaded before using it", name)
		}
		fileIDs = append(fileIDs, fixture.FileID)
	}
	return fileIDs, nil
}

func (suite *testSuite) allRememberedMessageImageNames() []string {
	names := make([]string, 0, len(suite.messageImagesByName))
	for name := range suite.messageImagesByName {
		names = append(names, name)
	}
	return names
}

func (suite *testSuite) systemRegistersMessageWithImage(imageName string) error {
	if err := suite.lastResponseShouldHaveStatusCode(http.StatusCreated); err != nil {
		return err
	}
	response, err := suite.sentMessageResponseFromLastBody()
	if err != nil {
		return err
	}
	suite.lastSentMessageID = response.ID
	return suite.assertMessageImages(response.Images, []string{imageName})
}

func (suite *testSuite) systemRegistersMessageWithTwoImages() error {
	if err := suite.lastResponseShouldHaveStatusCode(http.StatusCreated); err != nil {
		return err
	}
	response, err := suite.sentMessageResponseFromLastBody()
	if err != nil {
		return err
	}
	suite.lastSentMessageID = response.ID
	return suite.assertMessageImages(response.Images, suite.lastAttemptedMessageImageNames)
}

func (suite *testSuite) assertMessageImages(images []messageImageResponse, expectedNames []string) error {
	if len(images) != len(expectedNames) {
		return fmt.Errorf("expected %d message images, got %d", len(expectedNames), len(images))
	}
	for _, expectedName := range expectedNames {
		expectedImage, ok := suite.messageImagesByName[expectedName]
		if !ok {
			return fmt.Errorf("expected image fixture %q to exist", expectedName)
		}
		found := false
		for _, image := range images {
			if image.OriginalName != expectedName {
				continue
			}
			if image.ID != expectedImage.FileID {
				return fmt.Errorf("expected image %q id %q, got %q", expectedName, expectedImage.FileID, image.ID)
			}
			if image.URL == "" {
				return fmt.Errorf("expected image %q to include url", expectedName)
			}
			found = true
			break
		}
		if !found {
			return fmt.Errorf("expected message images to include %q", expectedName)
		}
	}
	return nil
}

func (suite *testSuite) imageRemainsAssociatedWithSentMessage() error {
	if suite.lastSentMessageID == 0 {
		return fmt.Errorf("expected a sent message before checking its image association")
	}
	if err := suite.requestConversationByID(suite.lastConversationID); err != nil {
		return err
	}
	response, err := suite.conversationDetailResponseFromLastBody()
	if err != nil {
		return err
	}
	for _, message := range response.Messages {
		if message.ID == suite.lastSentMessageID {
			return suite.assertMessageImages(message.Images, suite.lastAttemptedMessageImageNames)
		}
	}
	return fmt.Errorf("expected conversation detail to include sent message %d", suite.lastSentMessageID)
}

func (suite *testSuite) requestActiveChatWithConsumer(consumerEmail string) error {
	if _, err := suite.consumerRepository.FindIDByEmail(consumerEmail); err != nil {
		return err
	}
	return suite.requestConversationByID(suite.lastConversationID)
}

func (suite *testSuite) messageDetailIncludesImage(imageName string) error {
	response, err := suite.conversationDetailResponseFromLastBody()
	if err != nil {
		return err
	}
	for _, message := range response.Messages {
		for _, image := range message.Images {
			if image.OriginalName == imageName {
				return nil
			}
		}
	}
	return fmt.Errorf("expected conversation detail to include image %q", imageName)
}

func (suite *testSuite) systemAllowsProviderToAccessAttachedImage() error {
	if err := suite.lastResponseShouldHaveStatusCode(http.StatusOK); err != nil {
		return err
	}
	response, err := suite.conversationDetailResponseFromLastBody()
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
	return fmt.Errorf("expected provider conversation detail to expose an accessible image URL")
}

func (suite *testSuite) providerReceivesRealtimeMessageWithImage(email, imageName string) error {
	connection, err := suite.realtimeConnectionForEmail(email)
	if err != nil {
		return err
	}
	event, err := connection.readMessageEvent(realtimeMessageTimeout)
	if err != nil {
		return err
	}
	if event.Type != "conversation.message.created" || event.ConversationID != suite.lastConversationID {
		return fmt.Errorf("unexpected realtime message event: %+v", event)
	}
	return suite.assertMessageImages(event.Message.Images, []string{imageName})
}

func (suite *testSuite) systemRejectsMessageBecauseImageIsUnavailable() error {
	if err := suite.lastResponseShouldHaveStatusCode(http.StatusBadRequest); err != nil {
		return err
	}
	return suite.lastErrorResponseShouldSay(messageImageUnavailableError)
}

func (suite *testSuite) systemDoesNotAssociateImageWithAnyMessage() error {
	if err := suite.requestConversationByID(suite.lastConversationID); err != nil {
		return err
	}
	response, err := suite.conversationDetailResponseFromLastBody()
	if err != nil {
		return err
	}
	for _, name := range suite.lastAttemptedMessageImageNames {
		fixture := suite.messageImagesByName[name]
		for _, message := range response.Messages {
			for _, image := range message.Images {
				if image.ID == fixture.FileID {
					return fmt.Errorf("expected image %q not to be associated with any message", name)
				}
			}
		}
	}
	return nil
}

func (suite *testSuite) tryAccessMessageImage(imageName string) error {
	if _, ok := suite.messageImagesByName[imageName]; !ok {
		return fmt.Errorf("expected image %q to exist before checking access", imageName)
	}
	return suite.requestConversationByID(suite.lastConversationID)
}

func (suite *testSuite) systemReportsMessageImageAccessDenied() error {
	return suite.lastResponseShouldHaveStatusCode(http.StatusForbidden)
}

func (suite *testSuite) lastErrorResponseShouldSay(expected string) error {
	var payload struct {
		Error string `json:"error"`
	}
	if err := json.Unmarshal(suite.lastBody, &payload); err != nil {
		return fmt.Errorf("response is not a valid error response: %w", err)
	}
	if payload.Error != expected {
		return fmt.Errorf("expected error %q, got %q", expected, payload.Error)
	}
	return nil
}
