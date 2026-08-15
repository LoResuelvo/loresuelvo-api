package steps_test

import (
	"bytes"
	"context"
	"encoding/binary"
	"encoding/json"
	"fmt"
	"io"
	"math"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/LoResuelvo/loresuelvo-api/internal/domain/conversation"
	filedomain "github.com/LoResuelvo/loresuelvo-api/internal/domain/file"
	"github.com/cucumber/godog"
	"github.com/google/uuid"
)

const (
	conversationMessageAudioPurpose = "conversation_message_audio"
	conversationMessageAudioMIME    = "audio/webm"
	conversationMessageAudioCodec   = "opus"
)

type messageAudioFixture struct {
	FileID          string
	OriginalName    string
	MimeType        string
	Codec           string
	SizeBytes       int
	DurationSeconds int
}

type messageAudioResponse struct {
	ID              string `json:"id"`
	URL             string `json:"url"`
	OriginalName    string `json:"original_name"`
	MimeType        string `json:"mime_type"`
	Codec           string `json:"codec"`
	DurationSeconds int    `json:"duration_seconds"`
}

type confirmedAudioResponse struct {
	ID              string `json:"id"`
	URL             string `json:"url"`
	OriginalName    string `json:"original_name"`
	MimeType        string `json:"mime_type"`
	Codec           string `json:"codec"`
	DurationSeconds int    `json:"duration_seconds"`
}

func registerSendAudioSteps(sc *godog.ScenarioContext, suite *testSuite) {
	sc.Step(`^que existe una conversación pendiente entre el consumidor "([^"]*)" y el prestador "([^"]*)"$`, suite.thereIsPendingConversationBetweenConsumerAndProviderWithoutInitialMessage)
	sc.Step(`^que el consumidor "([^"]*)" ya alcanzó el límite de mensajes permitido en esa conversación pendiente$`, suite.consumerHasReachedPendingConversationMessageLimit)
	sc.Step(`^intento enviar únicamente el audio "([^"]*)" en la conversación pendiente con la consumidora "([^"]*)"$`, suite.trySendAudioOnlyMessageInPendingConversationWithConsumer)
	sc.Step(`^intento enviar únicamente el audio "([^"]*)" en la conversación pendiente con el prestador "([^"]*)"$`, suite.trySendAudioOnlyMessageInPendingConversationWithProvider)
	sc.Step(`^el sistema rechaza el mensaje porque el prestador debe aceptar la solicitud de trabajo antes de responder$`, suite.systemRejectsProviderAudioInPendingConversation)
	sc.Step(`^el sistema rechaza el mensaje porque se alcanzó el límite de mensajes de la conversación pendiente$`, suite.systemRejectsConsumerAudioInPendingConversation)
	sc.Step(`^el sistema rechaza el mensaje porque el audio no está disponible$`, suite.systemRejectsAudioUnavailable)
	sc.Step(`^el sistema no asocia el audio a ningún mensaje$`, suite.systemDoesNotAssociateAudioWithAnyMessage)
	sc.Step(`^el sistema no asocia el archivo a ningún mensaje$`, suite.systemDoesNotAssociateFileWithAnyMessage)
	sc.Step(`^envío únicamente el audio "([^"]*)" en el chat con la consumidora "([^"]*)"$`, suite.sendAudioOnlyMessageInChatWithConsumer)
	sc.Step(`^intento enviar únicamente el audio "([^"]*)" en el chat con el prestador "([^"]*)"$`, suite.sendAudioOnlyMessageInChatWithProvider)
	sc.Step(`^el mensaje fue enviado por el prestador "([^"]*)"$`, suite.audioMessageWasSentBy)
	sc.Step(`^que cargué y confirmé el audio "([^"]*)" de ([0-9]+) segundos$`, suite.uploadAndConfirmMessageAudio)
	sc.Step(`^intento enviar el audio "([^"]*)" junto con el texto:$`, suite.trySendAudioWithText)
	sc.Step(`^intento enviar el audio "([^"]*)" junto con la imagen "([^"]*)"$`, suite.trySendAudioWithImage)
	sc.Step(`^intento enviar el audio "([^"]*)" junto con la imagen "([^"]*)" y el texto:$`, suite.trySendAudioWithImageAndText)
	sc.Step(`^el sistema rechaza el mensaje porque el audio debe enviarse sin texto ni imágenes$`, suite.systemRejectsAudioCombinedMessage)
	sc.Step(`^el sistema no asocia el audio ni la imagen a ningún mensaje$`, suite.systemDoesNotAssociateAudioOrImageWithAnyMessage)
	sc.Step(`^envío únicamente el audio "([^"]*)" en el chat con el prestador "([^"]*)"$`, suite.sendAudioOnlyMessageInChatWithProvider)
	sc.Step(`^intento enviar únicamente el archivo "([^"]*)" como audio en el chat con el prestador "([^"]*)"$`, suite.sendAudioOnlyMessageInChatWithProvider)
	sc.Step(`^envío únicamente el audio "([^"]*)" en la conversación pendiente con el prestador "([^"]*)"$`, suite.sendAudioOnlyMessageInPendingConversationWithProvider)
	sc.Step(`^el sistema registra el mensaje de audio "([^"]*)" en el chat$`, suite.systemRegistersAudioMessage)
	sc.Step(`^el sistema registra el mensaje de audio "([^"]*)" en la conversación pendiente$`, suite.systemRegistersAudioMessageInPendingConversation)
	sc.Step(`^el mensaje fue enviado por el consumidor "([^"]*)"$`, suite.audioMessageWasSentBy)
	sc.Step(`^el audio queda asociado al mensaje enviado$`, suite.audioRemainsAssociatedWithSentMessage)
	sc.Step(`^que el consumidor "([^"]*)" envió el audio "([^"]*)" en el chat con el prestador "([^"]*)"$`, suite.consumerSentAudioInActiveChat)
	sc.Step(`^que la consumidora "([^"]*)" cargó y confirmó el audio "([^"]*)"$`, suite.consumerUploadedAndConfirmedMessageAudio)
	sc.Step(`^que cargué pero no confirmé el audio "([^"]*)"$`, suite.uploadedButDidNotConfirmMessageAudio)
	sc.Step(`^que cargué y confirmé el archivo "([^"]*)" para otra finalidad$`, suite.uploadedAndConfirmedAudioFileForOtherPurpose)
	sc.Step(`^el detalle incluye el mensaje de audio "([^"]*)"$`, suite.conversationDetailIncludesAudio)
	sc.Step(`^el detalle muestra la duración, el formato WebM y el codec Opus del audio$`, suite.conversationDetailShowsAudioMetadata)
	sc.Step(`^el sistema permite al prestador acceder al audio adjunto$`, suite.systemAllowsProviderToAccessAttachedAudio)
	sc.Step(`^que tengo un chat activo con el prestador "([^"]*)" cuyo último mensaje es el audio "([^"]*)" de ([0-9]+) segundos$`, suite.consumerHasActiveChatWithLastAudio)
}

func (suite *testSuite) consumerSentAudioInActiveChat(consumerEmail, audioName, providerEmail string) error {
	if err := suite.thereIsActiveChatBetweenConsumerAndProvider(consumerEmail, providerEmail); err != nil {
		return err
	}
	suite.currentAuth0ID = auth0IDForConsumerEmail(consumerEmail)
	if err := suite.uploadAndConfirmMessageAudio(audioName, "18"); err != nil {
		return err
	}
	audioFileID, err := suite.messageAudioFileID(audioName)
	if err != nil {
		return err
	}
	suite.lastAttemptedMessageImageNames = nil
	if err := suite.requestSendMessageToPreparedConversation(sendMessageRequest{AudioFileID: audioFileID}); err != nil {
		return err
	}
	return suite.systemRegistersAudioMessage(audioName)
}

func (suite *testSuite) consumerHasActiveChatWithLastAudio(providerEmail, audioName, durationText string) error {
	consumer, err := suite.userRepository.FindByAuthID(suite.currentAuth0ID)
	if err != nil {
		return fmt.Errorf("finding authenticated consumer for audio summary fixture: %w", err)
	}
	if consumer.Role() != participantRoleConsumer {
		return fmt.Errorf("expected authenticated participant to be a consumer, got role %q", consumer.Role())
	}
	if err := suite.thereIsActiveChatBetweenConsumerAndProvider(consumer.Email(), providerEmail); err != nil {
		return err
	}
	if err := suite.uploadAndConfirmMessageAudio(audioName, durationText); err != nil {
		return err
	}
	audioFileID, err := suite.messageAudioFileID(audioName)
	if err != nil {
		return err
	}
	suite.lastAttemptedMessageImageNames = nil
	if err := suite.requestSendMessageToPreparedConversation(sendMessageRequest{AudioFileID: audioFileID}); err != nil {
		return err
	}
	return suite.systemRegistersAudioMessage(audioName)
}

func (suite *testSuite) conversationDetailIncludesAudio(audioName string) error {
	if err := suite.lastResponseShouldHaveStatusCode(http.StatusOK); err != nil {
		return err
	}
	detail, err := suite.conversationDetailResponseFromLastBody()
	if err != nil {
		return err
	}
	audio, err := suite.audioInConversationDetail(detail, audioName)
	if err != nil {
		return err
	}
	if audio.ID == "" || audio.OriginalName != audioName {
		return fmt.Errorf("expected conversation detail to include audio %q, got %+v", audioName, audio)
	}
	return nil
}

func (suite *testSuite) conversationDetailShowsAudioMetadata() error {
	detail, err := suite.conversationDetailResponseFromLastBody()
	if err != nil {
		return err
	}
	audio, err := suite.audioInConversationDetail(detail, suite.lastAttemptedMessageAudioName)
	if err != nil {
		return err
	}
	fixture, ok := suite.messageAudiosByName[suite.lastAttemptedMessageAudioName]
	if !ok {
		return fmt.Errorf("expected audio fixture %q to exist", suite.lastAttemptedMessageAudioName)
	}
	if audio.DurationSeconds != fixture.DurationSeconds || audio.MimeType != fixture.MimeType || audio.Codec != fixture.Codec {
		return fmt.Errorf("audio %q metadata does not match fixture", suite.lastAttemptedMessageAudioName)
	}
	return nil
}

func (suite *testSuite) systemAllowsProviderToAccessAttachedAudio() error {
	if err := suite.lastResponseShouldHaveStatusCode(http.StatusOK); err != nil {
		return err
	}
	if suite.currentAuth0ID == "" {
		return fmt.Errorf("expected an authenticated provider before checking audio access")
	}
	detail, err := suite.conversationDetailResponseFromLastBody()
	if err != nil {
		return err
	}
	audio, err := suite.audioInConversationDetail(detail, suite.lastAttemptedMessageAudioName)
	if err != nil {
		return err
	}
	if strings.TrimSpace(audio.URL) == "" {
		return fmt.Errorf("expected provider to receive an access URL for audio %q", audio.OriginalName)
	}
	return nil
}

func (suite *testSuite) audioInConversationDetail(detail conversationDetailResponse, audioName string) (*messageAudioResponse, error) {
	if strings.TrimSpace(audioName) == "" {
		return nil, fmt.Errorf("expected an audio name before checking conversation detail")
	}
	for _, message := range detail.Messages {
		if message.Audio != nil && message.Audio.OriginalName == audioName {
			return message.Audio, nil
		}
	}
	return nil, fmt.Errorf("expected conversation detail to include audio %q", audioName)
}

func (suite *testSuite) uploadAndConfirmMessageAudio(name, durationText string) error {
	return suite.uploadMessageAudio(suite.currentAuth0ID, name, durationText, true)
}

func (suite *testSuite) consumerUploadedAndConfirmedMessageAudio(email, name string) error {
	return suite.uploadMessageAudio(auth0IDForConsumerEmail(email), name, "18", true)
}

func (suite *testSuite) uploadedButDidNotConfirmMessageAudio(name string) error {
	return suite.uploadMessageAudio(suite.currentAuth0ID, name, "18", false)
}

func (suite *testSuite) uploadMessageAudio(authID, name, durationText string, confirm bool) error {
	durationSeconds, err := strconv.Atoi(durationText)
	if err != nil || durationSeconds <= 0 {
		return fmt.Errorf("expected a positive audio duration, got %q", durationText)
	}
	if strings.TrimSpace(authID) == "" {
		return fmt.Errorf("expected an authenticated uploader before loading audio %q", name)
	}

	audioData := testWebMOpusAudio(durationSeconds)
	upload, err := suite.requestProfilePhotoPresign(authID, presignFileRequest{
		OriginalName: name,
		MimeType:     conversationMessageAudioMIME,
		SizeBytes:    len(audioData),
		Purpose:      conversationMessageAudioPurpose,
	})
	if err != nil {
		return err
	}
	if err := suite.lastResponseShouldHaveStatusCode(http.StatusOK); err != nil {
		return err
	}
	if upload.FileID == "" || upload.Key == "" || upload.UploadURL == "" {
		return fmt.Errorf("expected presign response for audio %q to include file_id, key and upload_url, got body %s", name, string(suite.lastBody))
	}

	fixture := messageAudioFixture{
		FileID:          upload.FileID,
		OriginalName:    name,
		MimeType:        conversationMessageAudioMIME,
		Codec:           conversationMessageAudioCodec,
		SizeBytes:       len(audioData),
		DurationSeconds: durationSeconds,
	}
	suite.messageAudiosByName[name] = fixture

	if !confirm {
		return nil
	}
	if err := suite.putMessageAudioObject(*upload, audioData); err != nil {
		return err
	}
	return suite.confirmMessageAudio(authID, *upload, fixture)
}

func (suite *testSuite) uploadedAndConfirmedAudioFileForOtherPurpose(name string) error {
	if strings.TrimSpace(suite.currentAuth0ID) == "" {
		return fmt.Errorf("expected an authenticated uploader before loading audio file %q", name)
	}

	metadata, err := filedomain.NewAudioFileMetadata(name, conversationMessageAudioMIME, 1024, 18, conversationMessageAudioCodec)
	if err != nil {
		return fmt.Errorf("creating other-purpose audio fixture: %w", err)
	}
	now := time.Now().UTC()
	fileID := uuid.NewString()
	file, err := filedomain.NewFile(
		fileID,
		"profile-photo/"+fileID+"/"+name,
		"loresuelvo-public-test",
		*metadata,
		filedomain.StatusConfirmed,
		filedomain.VisibilityPublic,
		filedomain.PurposeProfilePhoto,
		suite.currentAuth0ID,
		now,
		now,
	)
	if err != nil {
		return fmt.Errorf("creating confirmed other-purpose file fixture: %w", err)
	}
	if err := suite.fileRepository.Save(context.Background(), *file); err != nil {
		return fmt.Errorf("saving confirmed other-purpose file fixture: %w", err)
	}

	suite.messageAudiosByName[name] = messageAudioFixture{
		FileID:          fileID,
		OriginalName:    name,
		MimeType:        conversationMessageAudioMIME,
		Codec:           conversationMessageAudioCodec,
		SizeBytes:       1024,
		DurationSeconds: 18,
	}
	return nil
}

func (suite *testSuite) thereIsPendingConversationBetweenConsumerAndProviderWithoutInitialMessage(consumerEmail, providerEmail string) error {
	return suite.createPendingConversationBetweenConsumerAndProvider(consumerEmail, providerEmail, "")
}

func (suite *testSuite) consumerHasReachedPendingConversationMessageLimit(consumerEmail string) error {
	if suite.lastConversationID == 0 {
		return fmt.Errorf("expected a prepared pending conversation before reaching the message limit")
	}
	if _, err := suite.userRepository.FindIDByEmail(consumerEmail); err != nil {
		return err
	}

	previousAuth0ID := suite.currentAuth0ID
	suite.currentAuth0ID = auth0IDForConsumerEmail(consumerEmail)
	defer func() { suite.currentAuth0ID = previousAuth0ID }()

	for messageNumber := 1; messageNumber <= conversation.PendingConsumerMessageLimit; messageNumber++ {
		if err := suite.requestSendMessageToPreparedConversation(sendMessageRequest{
			Content: fmt.Sprintf("Mensaje previo %d", messageNumber),
		}); err != nil {
			return err
		}
		if err := suite.lastResponseShouldHaveStatusCode(http.StatusCreated); err != nil {
			return err
		}
	}
	return nil
}

func (suite *testSuite) putMessageAudioObject(upload presignFileResponse, data []byte) error {
	httpReq, err := http.NewRequest(http.MethodPut, upload.UploadURL, bytes.NewReader(data))
	if err != nil {
		return fmt.Errorf("failed to prepare audio object upload: %w", err)
	}
	for key, value := range upload.Headers {
		httpReq.Header.Set(key, value)
	}
	httpReq.Header.Set("Content-Type", conversationMessageAudioMIME)
	httpReq.ContentLength = int64(len(data))

	resp, err := http.DefaultClient.Do(httpReq)
	if err != nil {
		return fmt.Errorf("audio object upload failed: %w", err)
	}
	defer resp.Body.Close()

	responseBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return fmt.Errorf("failed to read audio object upload response: %w", err)
	}
	if resp.StatusCode < http.StatusOK || resp.StatusCode >= http.StatusMultipleChoices {
		return fmt.Errorf("expected audio object upload status 2xx, got %d with body %s", resp.StatusCode, string(responseBody))
	}
	return nil
}

func (suite *testSuite) confirmMessageAudio(authID string, upload presignFileResponse, fixture messageAudioFixture) error {
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
		return fmt.Errorf("failed to read audio confirmation response: %w", err)
	}
	suite.lastStatus = resp.StatusCode
	suite.lastBody = body
	if err := suite.lastResponseShouldHaveStatusCode(http.StatusOK); err != nil {
		return err
	}

	var response confirmedAudioResponse
	if err := json.Unmarshal(body, &response); err != nil {
		return fmt.Errorf("failed to parse audio confirmation response: %w", err)
	}
	if response.ID != fixture.FileID || response.OriginalName != fixture.OriginalName || response.MimeType != fixture.MimeType || response.Codec != fixture.Codec || response.DurationSeconds != fixture.DurationSeconds {
		return fmt.Errorf("audio confirmation response does not match fixture %q: %s", fixture.OriginalName, string(body))
	}
	return nil
}

func (suite *testSuite) sendAudioOnlyMessageInChatWithProvider(audioName, providerFullName string) error {
	return suite.sendAudioOnlyMessageInChatWithParticipant(audioName, providerFullName, participantRoleProvider)
}

func (suite *testSuite) sendAudioOnlyMessageInChatWithConsumer(audioName, consumerFullName string) error {
	return suite.sendAudioOnlyMessageInChatWithParticipant(audioName, consumerFullName, participantRoleConsumer)
}

func (suite *testSuite) sendAudioOnlyMessageInPendingConversationWithProvider(audioName, providerFullName string) error {
	return suite.sendAudioOnlyMessageInChatWithParticipant(audioName, providerFullName, participantRoleProvider)
}

func (suite *testSuite) trySendAudioOnlyMessageInPendingConversationWithConsumer(audioName, consumerFullName string) error {
	return suite.sendAudioOnlyMessageInChatWithParticipant(audioName, consumerFullName, participantRoleConsumer)
}

func (suite *testSuite) trySendAudioOnlyMessageInPendingConversationWithProvider(audioName, providerFullName string) error {
	return suite.sendAudioOnlyMessageInChatWithParticipant(audioName, providerFullName, participantRoleProvider)
}

func (suite *testSuite) sendAudioOnlyMessageInChatWithParticipant(audioName, participantFullName, participantRole string) error {
	if err := suite.ensureKnownParticipantFullName(participantFullName, participantRole); err != nil {
		return err
	}
	audioFileID, err := suite.messageAudioFileID(audioName)
	if err != nil {
		return err
	}
	suite.lastAttemptedMessageImageNames = nil
	return suite.requestSendMessageToPreparedConversation(sendMessageRequest{AudioFileID: audioFileID})
}

func (suite *testSuite) messageAudioFileID(audioName string) (string, error) {
	fixture, ok := suite.messageAudiosByName[audioName]
	if !ok {
		return "", fmt.Errorf("expected audio fixture %q to exist", audioName)
	}
	suite.lastAttemptedMessageAudioName = audioName
	return fixture.FileID, nil
}

func (suite *testSuite) trySendAudioWithText(audioName string, text *godog.DocString) error {
	audioFileID, err := suite.messageAudioFileID(audioName)
	if err != nil {
		return err
	}
	suite.lastAttemptedMessageImageNames = nil
	return suite.requestSendMessageToPreparedConversation(sendMessageRequest{
		Content:     normalizeDocString(text),
		AudioFileID: audioFileID,
	})
}

func (suite *testSuite) trySendAudioWithImage(audioName, imageName string) error {
	audioFileID, err := suite.messageAudioFileID(audioName)
	if err != nil {
		return err
	}
	imageFileIDs, err := suite.messageImageFileIDs([]string{imageName})
	if err != nil {
		return err
	}
	suite.lastAttemptedMessageImageNames = []string{imageName}
	return suite.requestSendMessageToPreparedConversation(sendMessageRequest{
		ImageFileIDs: imageFileIDs,
		AudioFileID:  audioFileID,
	})
}

func (suite *testSuite) trySendAudioWithImageAndText(audioName, imageName string, text *godog.DocString) error {
	audioFileID, err := suite.messageAudioFileID(audioName)
	if err != nil {
		return err
	}
	imageFileIDs, err := suite.messageImageFileIDs([]string{imageName})
	if err != nil {
		return err
	}
	suite.lastAttemptedMessageImageNames = []string{imageName}
	return suite.requestSendMessageToPreparedConversation(sendMessageRequest{
		Content:      normalizeDocString(text),
		ImageFileIDs: imageFileIDs,
		AudioFileID:  audioFileID,
	})
}

func (suite *testSuite) systemRegistersAudioMessage(audioName string) error {
	if err := suite.lastResponseShouldHaveStatusCode(http.StatusCreated); err != nil {
		return err
	}
	response, err := suite.sentMessageResponseFromLastBody()
	if err != nil {
		return err
	}
	if response.ID == 0 || response.ConversationID != suite.lastConversationID {
		return fmt.Errorf("expected created audio message in conversation %d, got body %s", suite.lastConversationID, string(suite.lastBody))
	}
	if response.Content != "" {
		return fmt.Errorf("expected audio message content to be empty, got %q", response.Content)
	}
	expectedRole, err := suite.currentAuthenticatedParticipantRole()
	if err != nil {
		return err
	}
	if response.SenderRole != expectedRole {
		return fmt.Errorf("expected audio message sender role %q, got %q", expectedRole, response.SenderRole)
	}
	if err := suite.assertMessageAudio(response.Audio, audioName); err != nil {
		return err
	}
	suite.lastSentMessageID = response.ID
	return nil
}

func (suite *testSuite) audioMessageWasSentBy(senderFullName string) error {
	response, err := suite.sentMessageResponseFromLastBody()
	if err != nil {
		return err
	}
	expectedRole, err := suite.participantRoleForFullName(senderFullName)
	if err != nil {
		return err
	}
	if response.SenderRole != expectedRole {
		return fmt.Errorf("expected audio message sender role %q for %q, got %q", expectedRole, senderFullName, response.SenderRole)
	}
	return nil
}

func (suite *testSuite) systemRegistersAudioMessageInPendingConversation(audioName string) error {
	if err := suite.systemRegistersAudioMessage(audioName); err != nil {
		return err
	}
	if err := suite.audioRemainsAssociatedWithSentMessage(); err != nil {
		return err
	}

	detail, err := suite.conversationDetailResponseFromLastBody()
	if err != nil {
		return err
	}
	if detail.Status != "pending" {
		return fmt.Errorf("expected conversation %d to remain pending, got %q", suite.lastConversationID, detail.Status)
	}
	return nil
}

func (suite *testSuite) systemRejectsProviderAudioInPendingConversation() error {
	if err := suite.lastResponseShouldHaveStatusCode(http.StatusForbidden); err != nil {
		return err
	}
	return suite.lastErrorResponseShouldSay(conversation.ErrPendingConversationRequiresAcceptance.Error())
}

func (suite *testSuite) systemRejectsConsumerAudioInPendingConversation() error {
	if err := suite.lastResponseShouldHaveStatusCode(http.StatusConflict); err != nil {
		return err
	}
	return suite.lastErrorResponseShouldSay(conversation.ErrPendingConversationMessageLimitReached.Error())
}

func (suite *testSuite) systemRejectsAudioUnavailable() error {
	if err := suite.lastResponseShouldHaveStatusCode(http.StatusBadRequest); err != nil {
		return err
	}
	return suite.lastErrorResponseShouldSay(conversation.ErrMessageAudioNotAvailable.Error())
}

func (suite *testSuite) systemRejectsAudioCombinedMessage() error {
	if err := suite.lastResponseShouldHaveStatusCode(http.StatusBadRequest); err != nil {
		return err
	}
	return suite.lastErrorResponseShouldSay(conversation.ErrMessageAudioMustBeExclusive.Error())
}

func (suite *testSuite) systemDoesNotAssociateAudioWithAnyMessage() error {
	fixture, ok := suite.messageAudiosByName[suite.lastAttemptedMessageAudioName]
	if !ok {
		return fmt.Errorf("expected attempted audio fixture %q to exist", suite.lastAttemptedMessageAudioName)
	}
	if err := suite.requestConversationByID(suite.lastConversationID); err != nil {
		return err
	}
	if err := suite.lastResponseShouldHaveStatusCode(http.StatusOK); err != nil {
		return err
	}

	detail, err := suite.conversationDetailResponseFromLastBody()
	if err != nil {
		return err
	}
	for _, message := range detail.Messages {
		if message.Audio != nil && message.Audio.ID == fixture.FileID {
			return fmt.Errorf("expected audio %q not to be associated with a message, found message %d", fixture.OriginalName, message.ID)
		}
	}
	return nil
}

func (suite *testSuite) systemDoesNotAssociateFileWithAnyMessage() error {
	return suite.systemDoesNotAssociateAudioWithAnyMessage()
}

func (suite *testSuite) systemDoesNotAssociateAudioOrImageWithAnyMessage() error {
	audioFixture, ok := suite.messageAudiosByName[suite.lastAttemptedMessageAudioName]
	if !ok {
		return fmt.Errorf("expected attempted audio fixture %q to exist", suite.lastAttemptedMessageAudioName)
	}
	if len(suite.lastAttemptedMessageImageNames) == 0 {
		return fmt.Errorf("expected an attempted image alongside audio %q", audioFixture.OriginalName)
	}
	imageFixture, ok := suite.messageImagesByName[suite.lastAttemptedMessageImageNames[0]]
	if !ok {
		return fmt.Errorf("expected attempted image fixture %q to exist", suite.lastAttemptedMessageImageNames[0])
	}
	if err := suite.requestConversationByID(suite.lastConversationID); err != nil {
		return err
	}
	if err := suite.lastResponseShouldHaveStatusCode(http.StatusOK); err != nil {
		return err
	}

	detail, err := suite.conversationDetailResponseFromLastBody()
	if err != nil {
		return err
	}
	for _, message := range detail.Messages {
		if message.Audio != nil && message.Audio.ID == audioFixture.FileID {
			return fmt.Errorf("expected audio %q not to be associated with a message, found message %d", audioFixture.OriginalName, message.ID)
		}
		for _, image := range message.Images {
			if image.ID == imageFixture.FileID {
				return fmt.Errorf("expected image %q not to be associated with a message, found message %d", imageFixture.OriginalName, message.ID)
			}
		}
	}
	return nil
}

func (suite *testSuite) audioRemainsAssociatedWithSentMessage() error {
	if suite.lastSentMessageID == 0 {
		return fmt.Errorf("expected a sent audio message before checking its association")
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
	for _, message := range response.Messages {
		if message.ID != suite.lastSentMessageID {
			continue
		}
		return suite.assertMessageAudio(message.Audio, suite.lastAttemptedMessageAudioName)
	}
	return fmt.Errorf("expected conversation detail to include sent audio message %d", suite.lastSentMessageID)
}

func (suite *testSuite) assertMessageAudio(audio *messageAudioResponse, expectedName string) error {
	if audio == nil {
		return fmt.Errorf("expected message to include audio %q", expectedName)
	}
	fixture, ok := suite.messageAudiosByName[expectedName]
	if !ok {
		return fmt.Errorf("expected audio fixture %q to exist", expectedName)
	}
	if audio.ID != fixture.FileID || audio.OriginalName != fixture.OriginalName {
		return fmt.Errorf("expected audio %q with id %q, got id %q and name %q", expectedName, fixture.FileID, audio.ID, audio.OriginalName)
	}
	if audio.MimeType != fixture.MimeType || audio.Codec != fixture.Codec || audio.DurationSeconds != fixture.DurationSeconds {
		return fmt.Errorf("audio %q metadata does not match fixture", expectedName)
	}
	if strings.TrimSpace(audio.URL) == "" {
		return fmt.Errorf("expected audio %q to include a private access URL", expectedName)
	}
	return nil
}

func testWebMOpusAudio(durationSeconds int) []byte {
	timecodeScale := testEBMLUnsigned(1_000_000)
	duration := make([]byte, 8)
	binary.BigEndian.PutUint64(duration, math.Float64bits(float64(durationSeconds)*1000))

	ebmlHeader := testEBMLElement(0x1A45DFA3,
		testEBMLElement(0x4286, testEBMLUnsigned(1)),
		testEBMLElement(0x4282, []byte("webm")),
	)
	info := testEBMLElement(0x1549A966,
		testEBMLElement(0x2AD7B1, timecodeScale),
		testEBMLElement(0x4489, duration),
	)
	trackEntry := testEBMLElement(0xAE, testEBMLElement(0x86, []byte("A_OPUS")))
	tracks := testEBMLElement(0x1654AE6B, trackEntry)
	segment := testEBMLElement(0x18538067, append(info, tracks...))
	return append(ebmlHeader, segment...)
}

func testEBMLElement(id uint64, payload ...[]byte) []byte {
	var body []byte
	for _, part := range payload {
		body = append(body, part...)
	}
	idBytes := testEBMLID(id)
	result := make([]byte, 0, len(idBytes)+8+len(body))
	result = append(result, idBytes...)
	result = append(result, testEBMLSize(len(body))...)
	result = append(result, body...)
	return result
}

func testEBMLID(id uint64) []byte {
	length := 1
	for value := id; value > 0xff; value >>= 8 {
		length++
	}
	result := make([]byte, length)
	for index := length - 1; index >= 0; index-- {
		result[index] = byte(id)
		id >>= 8
	}
	return result
}

func testEBMLSize(size int) []byte {
	for length := 1; length <= 8; length++ {
		max := (uint64(1) << uint(7*length)) - 2
		if uint64(size) > max {
			continue
		}
		result := make([]byte, length)
		value := uint64(size)
		for index := length - 1; index >= 0; index-- {
			result[index] = byte(value)
			value >>= 8
		}
		result[0] |= byte(1 << uint(8-length))
		return result
	}
	panic("test WebM element payload is too large")
}

func testEBMLUnsigned(value uint64) []byte {
	length := 1
	for candidate := value; candidate > 0xff; candidate >>= 8 {
		length++
	}
	result := make([]byte, length)
	for index := length - 1; index >= 0; index-- {
		result[index] = byte(value)
		value >>= 8
	}
	return result
}
