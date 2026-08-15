package steps_test

import (
	"bytes"
	"context"
	"encoding/binary"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"strings"

	"github.com/LoResuelvo/loresuelvo-api/internal/domain/conversation"
	"github.com/cucumber/godog"
)

const (
	conversationMessageVideoPurpose = "conversation_message_video"
	conversationMessageVideoMIME    = "video/mp4"
)

type messageVideoFixture struct {
	FileID          string
	Key             string
	OriginalName    string
	MimeType        string
	SizeBytes       int
	VideoCodec      string
	AudioCodec      string
	DurationSeconds int
	Width           int
	Height          int
}

type messageVideoResponse struct {
	ID              string `json:"id"`
	URL             string `json:"url"`
	OriginalName    string `json:"original_name"`
	MimeType        string `json:"mime_type"`
	VideoCodec      string `json:"video_codec"`
	AudioCodec      string `json:"audio_codec"`
	DurationSeconds int    `json:"duration_seconds"`
	Width           int    `json:"width"`
	Height          int    `json:"height"`
}

func registerSendVideoSteps(sc *godog.ScenarioContext, suite *testSuite) {
	sc.Step(`^que cargué y confirmé el video sin audio "([^"]*)" de ([0-9]+) segundos$`, suite.uploadAndConfirmMessageVideoWithoutAudio)
	sc.Step(`^que cargué y confirmé el video "([^"]*)" de ([0-9]+) segundos$`, suite.uploadAndConfirmMessageVideo)
	sc.Step(`^envío únicamente el video "([^"]*)" en el chat con el prestador "([^"]*)"$`, suite.sendVideoOnlyMessageInChatWithProvider)
	sc.Step(`^envío el video "([^"]*)" en el chat con la consumidora "([^"]*)" acompañado del texto:$`, suite.sendVideoWithTextInChatWithConsumer)
	sc.Step(`^envío el video "([^"]*)" en la conversación pendiente con el prestador "([^"]*)" acompañado del texto:$`, suite.sendVideoWithTextInPendingConversationWithProvider)
	sc.Step(`^intento enviar únicamente el video "([^"]*)" en la conversación pendiente con la consumidora "([^"]*)"$`, suite.trySendVideoOnlyMessageInPendingConversationWithConsumer)
	sc.Step(`^intento enviar únicamente el video "([^"]*)" en la conversación pendiente con el prestador "([^"]*)"$`, suite.trySendVideoOnlyMessageInPendingConversationWithProvider)
	sc.Step(`^el sistema registra el mensaje con el video "([^"]*)" en el chat$`, suite.systemRegistersMessageWithVideo)
	sc.Step(`^el sistema registra el mensaje con el video "([^"]*)" y el texto enviado$`, suite.systemRegistersMessageWithVideoAndText)
	sc.Step(`^el sistema registra el mensaje con el video "([^"]*)" y el texto enviado en la conversación pendiente$`, suite.systemRegistersMessageWithVideoAndTextInPendingConversation)
	sc.Step(`^el video queda asociado al mensaje enviado$`, suite.videoRemainsAssociatedWithSentMessage)
	sc.Step(`^el sistema no asocia el video a ningún mensaje$`, suite.systemDoesNotAssociateVideoWithAnyMessage)
}

func (suite *testSuite) uploadAndConfirmMessageVideoWithoutAudio(name, durationText string) error {
	return suite.uploadMessageVideo(suite.currentAuth0ID, name, durationText, false)
}

func (suite *testSuite) uploadAndConfirmMessageVideo(name, durationText string) error {
	return suite.uploadMessageVideo(suite.currentAuth0ID, name, durationText, true)
}

func (suite *testSuite) uploadMessageVideo(authID, name, durationText string, withAudio bool) error {
	durationSeconds, err := strconv.Atoi(durationText)
	if err != nil || durationSeconds <= 0 {
		return fmt.Errorf("expected a positive video duration, got %q", durationText)
	}
	if strings.TrimSpace(authID) == "" {
		return fmt.Errorf("expected an authenticated uploader before loading video %q", name)
	}

	videoData := testMP4Video(durationSeconds, withAudio)
	upload, err := suite.requestProfilePhotoPresign(authID, presignFileRequest{
		OriginalName: name,
		MimeType:     conversationMessageVideoMIME,
		SizeBytes:    len(videoData),
		Purpose:      conversationMessageVideoPurpose,
	})
	if err != nil {
		return err
	}
	if err := suite.lastResponseShouldHaveStatusCode(http.StatusOK); err != nil {
		return err
	}
	if upload.FileID == "" || upload.Key == "" || upload.UploadURL == "" {
		return fmt.Errorf("expected presign response for video %q to include file_id, key and upload_url, got body %s", name, string(suite.lastBody))
	}

	if err := suite.putMessageVideoObject(*upload, videoData); err != nil {
		return err
	}
	fixture := messageVideoFixture{
		FileID:          upload.FileID,
		Key:             upload.Key,
		OriginalName:    name,
		MimeType:        conversationMessageVideoMIME,
		SizeBytes:       len(videoData),
		VideoCodec:      "h264",
		DurationSeconds: durationSeconds,
		Width:           1080,
		Height:          1920,
	}
	if withAudio {
		fixture.AudioCodec = "aac"
	}
	suite.messageVideosByName[name] = fixture

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
		return fmt.Errorf("failed to read video confirmation response: %w", err)
	}
	suite.lastStatus = resp.StatusCode
	suite.lastBody = body
	if err := suite.lastResponseShouldHaveStatusCode(http.StatusOK); err != nil {
		return err
	}
	var response confirmedFileResponse
	if err := json.Unmarshal(body, &response); err != nil {
		return fmt.Errorf("failed to parse video confirmation response: %w", err)
	}
	if response.ID != fixture.FileID || response.OriginalName != fixture.OriginalName || response.MimeType != fixture.MimeType || response.Type != "video" || response.Audio != nil || response.Video == nil {
		return fmt.Errorf("video confirmation response does not match fixture %q: %s", fixture.OriginalName, string(body))
	}
	video := response.Video
	if video.VideoCodec != fixture.VideoCodec || video.AudioCodec != fixture.AudioCodec || video.DurationSeconds != fixture.DurationSeconds || video.Width != fixture.Width || video.Height != fixture.Height {
		return fmt.Errorf("video confirmation response does not match fixture %q: %s", fixture.OriginalName, string(body))
	}
	return nil
}

func (suite *testSuite) putMessageVideoObject(upload presignFileResponse, data []byte) error {
	httpReq, err := http.NewRequest(http.MethodPut, upload.UploadURL, bytes.NewReader(data))
	if err != nil {
		return fmt.Errorf("failed to prepare video object upload: %w", err)
	}
	for key, value := range upload.Headers {
		httpReq.Header.Set(key, value)
	}
	httpReq.Header.Set("Content-Type", conversationMessageVideoMIME)
	httpReq.ContentLength = int64(len(data))

	resp, err := http.DefaultClient.Do(httpReq)
	if err != nil {
		return fmt.Errorf("video object upload failed: %w", err)
	}
	defer resp.Body.Close()
	responseBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return fmt.Errorf("failed to read video object upload response: %w", err)
	}
	if resp.StatusCode < http.StatusOK || resp.StatusCode >= http.StatusMultipleChoices {
		return fmt.Errorf("expected video object upload status 2xx, got %d with body %s", resp.StatusCode, string(responseBody))
	}
	return nil
}

func (suite *testSuite) sendVideoOnlyMessageInChatWithProvider(videoName, providerFullName string) error {
	if err := suite.ensureKnownParticipantFullName(providerFullName, participantRoleProvider); err != nil {
		return err
	}
	videoFileID, err := suite.messageVideoFileID(videoName)
	if err != nil {
		return err
	}
	suite.lastAttemptedMessageVideoContent = ""
	return suite.requestSendMessageToPreparedConversation(sendMessageRequest{VideoFileID: videoFileID})
}

func (suite *testSuite) sendVideoWithTextInChatWithConsumer(videoName, consumerFullName string, text *godog.DocString) error {
	if err := suite.ensureKnownParticipantFullName(consumerFullName, participantRoleConsumer); err != nil {
		return err
	}
	fixture, err := suite.messageVideoFixture(videoName)
	if err != nil {
		return err
	}
	content := normalizeDocString(text)
	suite.lastAttemptedMessageVideoName = videoName
	suite.lastAttemptedMessageVideoContent = content
	return suite.requestSendMessageToPreparedConversation(sendMessageRequest{Content: content, VideoFileID: fixture.FileID})
}

func (suite *testSuite) sendVideoWithTextInPendingConversationWithProvider(videoName, providerFullName string, text *godog.DocString) error {
	if err := suite.ensureKnownParticipantFullName(providerFullName, participantRoleProvider); err != nil {
		return err
	}
	videoFileID, err := suite.messageVideoFileID(videoName)
	if err != nil {
		return err
	}
	content := normalizeDocString(text)
	suite.lastAttemptedMessageVideoContent = content
	return suite.requestSendMessageToPreparedConversation(sendMessageRequest{
		Content:     content,
		VideoFileID: videoFileID,
	})
}

func (suite *testSuite) trySendVideoOnlyMessageInPendingConversationWithConsumer(videoName, consumerFullName string) error {
	return suite.trySendVideoOnlyMessageInPendingConversationWithParticipant(videoName, consumerFullName, participantRoleConsumer)
}

func (suite *testSuite) trySendVideoOnlyMessageInPendingConversationWithProvider(videoName, providerFullName string) error {
	return suite.trySendVideoOnlyMessageInPendingConversationWithParticipant(videoName, providerFullName, participantRoleProvider)
}

func (suite *testSuite) trySendVideoOnlyMessageInPendingConversationWithParticipant(videoName, participantFullName, participantRole string) error {
	if err := suite.ensureKnownParticipantFullName(participantFullName, participantRole); err != nil {
		return err
	}
	videoFileID, err := suite.messageVideoFileID(videoName)
	if err != nil {
		return err
	}
	suite.lastAttemptedMessageVideoContent = ""
	return suite.requestSendMessageToPreparedConversation(sendMessageRequest{VideoFileID: videoFileID})
}

func (suite *testSuite) messageVideoFixture(name string) (messageVideoFixture, error) {
	fixture, ok := suite.messageVideosByName[name]
	if !ok {
		return messageVideoFixture{}, fmt.Errorf("expected video fixture %q to exist", name)
	}
	return fixture, nil
}

func (suite *testSuite) messageVideoFileID(name string) (string, error) {
	fixture, err := suite.messageVideoFixture(name)
	if err != nil {
		return "", err
	}
	suite.lastAttemptedMessageVideoName = name
	return fixture.FileID, nil
}

func (suite *testSuite) systemRegistersMessageWithVideo(videoName string) error {
	if err := suite.lastResponseShouldHaveStatusCode(http.StatusCreated); err != nil {
		return err
	}
	response, err := suite.sentMessageResponseFromLastBody()
	if err != nil {
		return err
	}
	if response.ID == 0 || response.ConversationID != suite.lastConversationID {
		return fmt.Errorf("expected created video message in conversation %d, got body %s", suite.lastConversationID, string(suite.lastBody))
	}
	if response.Content != "" {
		return fmt.Errorf("expected video-only message content to be empty, got %q", response.Content)
	}
	if response.SenderRole == "" || response.Video == nil {
		return fmt.Errorf("expected sent message to include sender role and video, got body %s", string(suite.lastBody))
	}
	if err := suite.assertMessageVideo(response.Video, videoName); err != nil {
		return err
	}
	suite.lastSentMessageID = response.ID
	return nil
}

func (suite *testSuite) systemRegistersMessageWithVideoAndText(videoName string) error {
	if err := suite.lastResponseShouldHaveStatusCode(http.StatusCreated); err != nil {
		return err
	}
	response, err := suite.sentMessageResponseFromLastBody()
	if err != nil {
		return err
	}
	if response.ID == 0 || response.ConversationID != suite.lastConversationID {
		return fmt.Errorf("expected created video message in conversation %d, got body %s", suite.lastConversationID, string(suite.lastBody))
	}
	if response.Content != suite.lastAttemptedMessageVideoContent {
		return fmt.Errorf("expected video message content %q, got %q", suite.lastAttemptedMessageVideoContent, response.Content)
	}
	if response.Video == nil {
		return fmt.Errorf("expected sent message to include video, got body %s", string(suite.lastBody))
	}
	if err := suite.assertMessageVideo(response.Video, videoName); err != nil {
		return err
	}
	suite.lastSentMessageID = response.ID
	return nil
}

func (suite *testSuite) systemRegistersMessageWithVideoAndTextInPendingConversation(videoName string) error {
	if err := suite.systemRegistersMessageWithVideoAndText(videoName); err != nil {
		return err
	}

	foundConversation, err := suite.conversationRepository.FindByID(context.Background(), suite.lastConversationID)
	if err != nil {
		return fmt.Errorf("finding pending conversation after sending video: %w", err)
	}
	if foundConversation.Status() != conversation.StatusPending {
		return fmt.Errorf("expected conversation %d to remain pending, got %q", suite.lastConversationID, foundConversation.Status())
	}

	lastMessage, ok := foundConversation.LastMessage()
	if !ok || lastMessage.ID != suite.lastSentMessageID {
		return fmt.Errorf("expected sent video message %d to remain the last message in conversation %d", suite.lastSentMessageID, suite.lastConversationID)
	}
	if lastMessage.Content != suite.lastAttemptedMessageVideoContent {
		return fmt.Errorf("expected persisted video message content %q, got %q", suite.lastAttemptedMessageVideoContent, lastMessage.Content)
	}
	fixture, err := suite.messageVideoFixture(videoName)
	if err != nil {
		return err
	}
	if lastMessage.Video == nil || lastMessage.Video.FileID != fixture.FileID {
		return fmt.Errorf("expected persisted video %q to remain associated with message %d", fixture.OriginalName, lastMessage.ID)
	}

	return nil
}

func (suite *testSuite) assertMessageVideo(video *messageVideoResponse, expectedName string) error {
	fixture, err := suite.messageVideoFixture(expectedName)
	if err != nil {
		return err
	}
	if video == nil || video.ID != fixture.FileID || video.OriginalName != fixture.OriginalName {
		return fmt.Errorf("expected video %q with id %q, got %+v", expectedName, fixture.FileID, video)
	}
	if strings.TrimSpace(video.URL) == "" || video.MimeType != fixture.MimeType || video.VideoCodec != fixture.VideoCodec || video.AudioCodec != fixture.AudioCodec || video.DurationSeconds != fixture.DurationSeconds || video.Width != fixture.Width || video.Height != fixture.Height {
		return fmt.Errorf("video %q metadata does not match fixture: got %+v", expectedName, video)
	}
	return nil
}

func (suite *testSuite) videoRemainsAssociatedWithSentMessage() error {
	fixture, err := suite.messageVideoFixture(suite.lastAttemptedMessageVideoName)
	if err != nil {
		return err
	}
	foundConversation, err := suite.conversationRepository.FindByID(context.Background(), suite.lastConversationID)
	if err != nil {
		return fmt.Errorf("finding conversation after sending video: %w", err)
	}
	lastMessage, ok := foundConversation.LastMessage()
	if !ok || lastMessage.ID != suite.lastSentMessageID || lastMessage.Video == nil || lastMessage.Video.FileID != fixture.FileID {
		return fmt.Errorf("expected video %q to remain associated with sent message %d", fixture.OriginalName, suite.lastSentMessageID)
	}
	return nil
}

func (suite *testSuite) systemDoesNotAssociateVideoWithAnyMessage() error {
	fixture, err := suite.messageVideoFixture(suite.lastAttemptedMessageVideoName)
	if err != nil {
		return err
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
		if message.Video != nil && message.Video.ID == fixture.FileID {
			return fmt.Errorf("expected video %q not to be associated with a message, found message %d", fixture.OriginalName, message.ID)
		}
	}
	return nil
}

func testMP4Video(durationSeconds int, withAudio bool) []byte {
	mvhd := make([]byte, 20)
	binary.BigEndian.PutUint32(mvhd[12:16], 1000)
	binary.BigEndian.PutUint32(mvhd[16:20], uint32(durationSeconds*1000))
	videoTkhd := make([]byte, 84)
	binary.BigEndian.PutUint32(videoTkhd[76:80], uint32(1080)<<16)
	binary.BigEndian.PutUint32(videoTkhd[80:84], uint32(1920)<<16)
	videoStsd := append(make([]byte, 8), testMP4Box("avc1", videoSampleEntry())...)
	videoTrack := testMP4Box("trak",
		testMP4Box("tkhd", videoTkhd),
		testMP4Box("mdia",
			testMP4Box("hdlr", append(make([]byte, 8), []byte("vide")...)),
			testMP4Box("minf", testMP4Box("stbl", testMP4Box("stsd", videoStsd))),
		),
	)
	moovPayload := append(testMP4Box("mvhd", mvhd), videoTrack...)
	if withAudio {
		audioStsd := append(make([]byte, 8), testMP4Box("mp4a")...)
		moovPayload = append(moovPayload, testMP4Box("trak", testMP4Box("mdia", testMP4Box("hdlr", append(make([]byte, 8), []byte("soun")...)), testMP4Box("minf", testMP4Box("stbl", testMP4Box("stsd", audioStsd)))))...)
	}
	ftyp := append([]byte("isom"), make([]byte, 4)...)
	ftyp = append(ftyp, []byte("isommp42")...)
	return append(testMP4Box("ftyp", ftyp), append(testMP4Box("moov", moovPayload), testMP4Box("mdat", []byte{0})...)...)
}

func videoSampleEntry() []byte {
	entry := make([]byte, 28)
	binary.BigEndian.PutUint16(entry[24:26], 1080)
	binary.BigEndian.PutUint16(entry[26:28], 1920)
	return entry
}

func testMP4Box(typ string, payload ...[]byte) []byte {
	body := make([]byte, 0)
	for _, part := range payload {
		body = append(body, part...)
	}
	result := make([]byte, 8+len(body))
	binary.BigEndian.PutUint32(result[:4], uint32(len(result)))
	copy(result[4:8], typ)
	copy(result[8:], body)
	return result
}
