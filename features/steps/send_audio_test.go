package steps_test

import (
	"bytes"
	"encoding/binary"
	"encoding/json"
	"fmt"
	"io"
	"math"
	"net/http"
	"strconv"
	"strings"

	"github.com/cucumber/godog"
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
	sc.Step(`^que cargué y confirmé el audio "([^"]*)" de ([0-9]+) segundos$`, suite.uploadAndConfirmMessageAudio)
	sc.Step(`^envío únicamente el audio "([^"]*)" en el chat con el prestador "([^"]*)"$`, suite.sendAudioOnlyMessageInChatWithProvider)
	sc.Step(`^el sistema registra el mensaje de audio "([^"]*)" en el chat$`, suite.systemRegistersAudioMessage)
	sc.Step(`^el mensaje fue enviado por el consumidor "([^"]*)"$`, suite.audioMessageWasSentBy)
	sc.Step(`^el audio queda asociado al mensaje enviado$`, suite.audioRemainsAssociatedWithSentMessage)
}

func (suite *testSuite) uploadAndConfirmMessageAudio(name, durationText string) error {
	durationSeconds, err := strconv.Atoi(durationText)
	if err != nil || durationSeconds <= 0 {
		return fmt.Errorf("expected a positive audio duration, got %q", durationText)
	}
	if strings.TrimSpace(suite.currentAuth0ID) == "" {
		return fmt.Errorf("expected an authenticated uploader before loading audio %q", name)
	}

	audioData := testWebMOpusAudio(durationSeconds)
	upload, err := suite.requestProfilePhotoPresign(suite.currentAuth0ID, presignFileRequest{
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

	if err := suite.putMessageAudioObject(*upload, audioData); err != nil {
		return err
	}
	return suite.confirmMessageAudio(suite.currentAuth0ID, *upload, fixture)
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
	if err := suite.ensureKnownParticipantFullName(providerFullName, participantRoleProvider); err != nil {
		return err
	}
	fixture, ok := suite.messageAudiosByName[audioName]
	if !ok {
		return fmt.Errorf("expected audio fixture %q to exist", audioName)
	}
	suite.lastAttemptedMessageAudioName = audioName
	suite.lastAttemptedMessageImageNames = nil
	return suite.requestSendMessageToPreparedConversation(sendMessageRequest{AudioFileID: fixture.FileID})
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
