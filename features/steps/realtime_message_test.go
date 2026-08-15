package steps_test

import (
	"bufio"
	"context"
	"crypto/rand"
	"crypto/sha1"
	"encoding/base64"
	"encoding/binary"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/cucumber/godog"
)

const realtimeMessageTimeout = 250 * time.Millisecond

type realtimeMessageEvent struct {
	Type           string               `json:"type"`
	ConversationID int                  `json:"conversation_id"`
	Message        realtimeEventMessage `json:"message"`
}

type realtimeEventMessage struct {
	ID         int                    `json:"id"`
	SenderRole string                 `json:"sender_role"`
	Content    string                 `json:"content"`
	Images     []messageImageResponse `json:"images"`
	Audio      *messageAudioResponse  `json:"audio,omitempty"`
	Video      *messageVideoResponse  `json:"video,omitempty"`
	CreatedOn  time.Time              `json:"created_on"`
}

type realtimeTestConnection struct {
	conn   net.Conn
	reader *bufio.Reader
}

func registerRealtimeMessageSteps(sc *godog.ScenarioContext, suite *testSuite) {
	sc.Step(`^que existe un chat activo entre el consumidor "([^"]*)" y el prestador "([^"]*)" con el mensaje inicial:$`, suite.thereIsActiveChatBetweenConsumerAndProviderWithInitialMessage)
	sc.Step(`^que existe un chat activo entre el consumidor "([^"]*)" y el prestador "([^"]*)"$`, suite.thereIsActiveChatBetweenConsumerAndProvider)
	sc.Step(`^que el consumidor "([^"]*)" está disponible para recibir mensajes en tiempo real$`, suite.consumerIsAvailableForRealtimeMessages)
	sc.Step(`^que el prestador "([^"]*)" está disponible para recibir mensajes en tiempo real$`, suite.providerIsAvailableForRealtimeMessages)
	sc.Step(`^envío un mensaje en el chat con el prestador "([^"]*)":$`, suite.sendMessageInChatWithProvider)
	sc.Step(`^envío un mensaje en el chat con el consumidor "([^"]*)":$`, suite.sendMessageInChatWithConsumer)
	sc.Step(`^el consumidor "([^"]*)" recibe en tiempo real el mensaje:$`, suite.consumerReceivesRealtimeMessage)
	sc.Step(`^el prestador "([^"]*)" recibe en tiempo real el mensaje:$`, suite.providerReceivesRealtimeMessage)
	sc.Step(`^el prestador "([^"]*)" recibe en tiempo real el mensaje de audio "([^"]*)"$`, suite.providerReceivesRealtimeAudioMessage)
	sc.Step(`^el evento recibido incluye la duración y el acceso al audio$`, suite.realtimeAudioEventIncludesDurationAndAccess)
	sc.Step(`^el prestador "([^"]*)" recibe en tiempo real el mensaje con el video "([^"]*)"$`, suite.providerReceivesRealtimeVideoMessage)
	sc.Step(`^el evento recibido incluye los metadatos y el acceso al video$`, suite.realtimeVideoEventIncludesMetadataAndAccess)
	sc.Step(`^el consumidor "([^"]*)" no recibe mensajes en tiempo real$`, suite.consumerDoesNotReceiveRealtimeMessages)
	sc.Step(`^el prestador "([^"]*)" no recibe mensajes en tiempo real$`, suite.providerDoesNotReceiveRealtimeMessages)
}

func (suite *testSuite) thereIsActiveChatBetweenConsumerAndProvider(consumerEmail, providerEmail string) error {
	return suite.thereIsActiveChatBetweenConsumerAndProviderWithInitialMessage(
		consumerEmail,
		providerEmail,
		&godog.DocString{Content: "Conversación preparada para una propuesta de servicio."},
	)
}

func (suite *testSuite) thereIsActiveChatBetweenConsumerAndProviderWithInitialMessage(consumerEmail, providerEmail string, message *godog.DocString) error {
	if err := suite.createPendingConversationBetweenConsumerAndProvider(consumerEmail, providerEmail, normalizeDocString(message)); err != nil {
		return err
	}

	preparedConversation, err := suite.conversationRepository.FindByID(context.Background(), suite.lastConversationID)
	if err != nil {
		return fmt.Errorf("finding chat fixture before activation: %w", err)
	}
	if err := preparedConversation.Activate(); err != nil {
		return fmt.Errorf("activating chat fixture: %w", err)
	}
	if _, err := suite.conversationRepository.SaveConversation(context.Background(), preparedConversation); err != nil {
		return fmt.Errorf("saving active chat fixture: %w", err)
	}

	return nil
}

func (suite *testSuite) sendMessageInChatWithProvider(providerFullName string, message *godog.DocString) error {
	return suite.sendMessageInPendingConversationWithProvider(providerFullName, message)
}

func (suite *testSuite) sendMessageInChatWithConsumer(consumerFullName string, message *godog.DocString) error {
	return suite.sendMessageInPendingConversationWithConsumer(consumerFullName, message)
}

func (suite *testSuite) consumerIsAvailableForRealtimeMessages(email string) error {
	return suite.participantIsAvailableForRealtimeMessages(email, auth0IDForConsumerEmail(email), "consumer")
}

func (suite *testSuite) providerIsAvailableForRealtimeMessages(email string) error {
	return suite.participantIsAvailableForRealtimeMessages(email, auth0IDForProviderEmail(email), "provider")
}

func (suite *testSuite) participantIsAvailableForRealtimeMessages(email, auth0ID, role string) error {
	connection, err := suite.openRealtimeConnection(auth0ID, role)
	if err != nil {
		return err
	}

	if suite.realtimeConnections == nil {
		suite.realtimeConnections = map[string]*realtimeTestConnection{}
	}
	suite.realtimeConnections[email] = connection

	return nil
}

func (suite *testSuite) consumerReceivesRealtimeMessage(email string, message *godog.DocString) error {
	return suite.participantReceivesRealtimeMessage(email, message)
}

func (suite *testSuite) providerReceivesRealtimeMessage(email string, message *godog.DocString) error {
	return suite.participantReceivesRealtimeMessage(email, message)
}

func (suite *testSuite) participantReceivesRealtimeMessage(email string, message *godog.DocString) error {
	connection, err := suite.realtimeConnectionForEmail(email)
	if err != nil {
		return err
	}

	event, err := connection.readMessageEvent(realtimeMessageTimeout)
	if err != nil {
		return err
	}

	expectedContent := normalizeDocString(message)
	if event.Type != "conversation.message.created" {
		return fmt.Errorf("expected realtime event type %q, got %q", "conversation.message.created", event.Type)
	}
	if event.ConversationID != suite.lastConversationID {
		return fmt.Errorf("expected realtime conversation id %d, got %d", suite.lastConversationID, event.ConversationID)
	}
	if event.Message.ID == 0 {
		return fmt.Errorf("expected realtime message id to be present")
	}
	if event.Message.Content != expectedContent {
		return fmt.Errorf("expected realtime message content %q, got %q", expectedContent, event.Message.Content)
	}
	if event.Message.CreatedOn.IsZero() {
		return fmt.Errorf("expected realtime message created_on to be present")
	}
	suite.lastRealtimeEvent = &event

	return nil
}

func (suite *testSuite) providerReceivesRealtimeAudioMessage(email, audioName string) error {
	connection, err := suite.realtimeConnectionForEmail(email)
	if err != nil {
		return err
	}

	event, err := connection.readMessageEvent(realtimeMessageTimeout)
	if err != nil {
		return err
	}
	fixture, ok := suite.messageAudiosByName[audioName]
	if !ok {
		return fmt.Errorf("expected audio fixture %q to exist", audioName)
	}

	if event.Type != "conversation.message.created" {
		return fmt.Errorf("expected realtime event type %q, got %q", "conversation.message.created", event.Type)
	}
	if event.ConversationID != suite.lastConversationID {
		return fmt.Errorf("expected realtime conversation id %d, got %d", suite.lastConversationID, event.ConversationID)
	}
	if event.Message.ID == 0 {
		return fmt.Errorf("expected realtime audio message id to be present")
	}
	if event.Message.SenderRole != participantRoleConsumer {
		return fmt.Errorf("expected realtime audio sender role %q, got %q", participantRoleConsumer, event.Message.SenderRole)
	}
	if event.Message.Content != "" {
		return fmt.Errorf("expected realtime audio message content to be empty, got %q", event.Message.Content)
	}
	if event.Message.Audio == nil {
		return fmt.Errorf("expected realtime event to include audio %q", audioName)
	}
	if event.Message.Audio.ID != fixture.FileID || event.Message.Audio.OriginalName != fixture.OriginalName {
		return fmt.Errorf("expected realtime audio %q with id %q, got id %q and name %q", audioName, fixture.FileID, event.Message.Audio.ID, event.Message.Audio.OriginalName)
	}
	if event.Message.CreatedOn.IsZero() {
		return fmt.Errorf("expected realtime audio message created_on to be present")
	}

	suite.lastRealtimeEvent = &event
	return nil
}

func (suite *testSuite) realtimeAudioEventIncludesDurationAndAccess() error {
	if suite.lastRealtimeEvent == nil || suite.lastRealtimeEvent.Message.Audio == nil {
		return fmt.Errorf("expected a received realtime audio event before checking its metadata")
	}

	audio := suite.lastRealtimeEvent.Message.Audio
	fixture, ok := suite.messageAudiosByName[audio.OriginalName]
	if !ok {
		return fmt.Errorf("expected audio fixture %q to exist", audio.OriginalName)
	}
	if audio.DurationSeconds != fixture.DurationSeconds {
		return fmt.Errorf("expected realtime audio duration %d seconds, got %d", fixture.DurationSeconds, audio.DurationSeconds)
	}
	if audio.MimeType != fixture.MimeType || audio.Codec != fixture.Codec {
		return fmt.Errorf("expected realtime audio format %q and codec %q, got format %q and codec %q", fixture.MimeType, fixture.Codec, audio.MimeType, audio.Codec)
	}
	if strings.TrimSpace(audio.URL) == "" {
		return fmt.Errorf("expected realtime audio %q to include an access URL", audio.OriginalName)
	}

	return nil
}

func (suite *testSuite) providerReceivesRealtimeVideoMessage(email, videoName string) error {
	connection, err := suite.realtimeConnectionForEmail(email)
	if err != nil {
		return err
	}

	event, err := connection.readMessageEvent(realtimeMessageTimeout)
	if err != nil {
		return err
	}
	fixture, ok := suite.messageVideosByName[videoName]
	if !ok {
		return fmt.Errorf("expected video fixture %q to exist", videoName)
	}

	if event.Type != "conversation.message.created" {
		return fmt.Errorf("expected realtime event type %q, got %q", "conversation.message.created", event.Type)
	}
	if event.ConversationID != suite.lastConversationID {
		return fmt.Errorf("expected realtime conversation id %d, got %d", suite.lastConversationID, event.ConversationID)
	}
	if event.Message.ID == 0 {
		return fmt.Errorf("expected realtime video message id to be present")
	}
	if event.Message.SenderRole != participantRoleConsumer {
		return fmt.Errorf("expected realtime video sender role %q, got %q", participantRoleConsumer, event.Message.SenderRole)
	}
	if event.Message.Content != "" {
		return fmt.Errorf("expected realtime video-only message content to be empty, got %q", event.Message.Content)
	}
	if event.Message.Video == nil {
		return fmt.Errorf("expected realtime event to include video %q", videoName)
	}
	if event.Message.Video.ID != fixture.FileID || event.Message.Video.OriginalName != fixture.OriginalName {
		return fmt.Errorf("expected realtime video %q with id %q, got id %q and name %q", videoName, fixture.FileID, event.Message.Video.ID, event.Message.Video.OriginalName)
	}
	if event.Message.CreatedOn.IsZero() {
		return fmt.Errorf("expected realtime video message created_on to be present")
	}

	suite.lastRealtimeEvent = &event
	return nil
}

func (suite *testSuite) realtimeVideoEventIncludesMetadataAndAccess() error {
	if suite.lastRealtimeEvent == nil || suite.lastRealtimeEvent.Message.Video == nil {
		return fmt.Errorf("expected a received realtime video event before checking its metadata")
	}

	video := suite.lastRealtimeEvent.Message.Video
	fixture, ok := suite.messageVideosByName[video.OriginalName]
	if !ok {
		return fmt.Errorf("expected video fixture %q to exist", video.OriginalName)
	}
	if video.DurationSeconds != fixture.DurationSeconds || video.Width != fixture.Width || video.Height != fixture.Height {
		return fmt.Errorf("expected realtime video duration %d seconds and dimensions %dx%d, got duration %d and dimensions %dx%d", fixture.DurationSeconds, fixture.Width, fixture.Height, video.DurationSeconds, video.Width, video.Height)
	}
	if video.MimeType != fixture.MimeType || video.VideoCodec != fixture.VideoCodec || video.AudioCodec != fixture.AudioCodec {
		return fmt.Errorf("expected realtime video format %q, video codec %q and audio codec %q, got format %q, video codec %q and audio codec %q", fixture.MimeType, fixture.VideoCodec, fixture.AudioCodec, video.MimeType, video.VideoCodec, video.AudioCodec)
	}
	if strings.TrimSpace(video.URL) == "" {
		return fmt.Errorf("expected realtime video %q to include an access URL", video.OriginalName)
	}

	return nil
}

func (suite *testSuite) consumerDoesNotReceiveRealtimeMessages(email string) error {
	return suite.participantDoesNotReceiveRealtimeMessages(email)
}

func (suite *testSuite) providerDoesNotReceiveRealtimeMessages(email string) error {
	return suite.participantDoesNotReceiveRealtimeMessages(email)
}

func (suite *testSuite) participantDoesNotReceiveRealtimeMessages(email string) error {
	connection, err := suite.realtimeConnectionForEmail(email)
	if err != nil {
		return err
	}

	event, err := connection.readMessageEvent(realtimeMessageTimeout)
	if err == nil {
		return fmt.Errorf("expected no realtime messages for %q, got %+v", email, event)
	}
	if isRealtimeReadTimeout(err) {
		return nil
	}

	return err
}

func (suite *testSuite) realtimeConnectionForEmail(email string) (*realtimeTestConnection, error) {
	connection, ok := suite.realtimeConnections[email]
	if !ok || connection == nil {
		return nil, fmt.Errorf("expected %q to be available for realtime messages", email)
	}

	return connection, nil
}

func (suite *testSuite) openRealtimeConnection(auth0ID, role string) (*realtimeTestConnection, error) {
	serverURL, err := url.Parse(suite.server.URL)
	if err != nil {
		return nil, err
	}

	req, err := http.NewRequest("POST", suite.server.URL+"/ws-tickets", nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Authorization", "Bearer "+suite.tokenBuilder.BuildToken(auth0ID, nil))
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("requesting ticket: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("expected 200 for ticket, got %d", resp.StatusCode)
	}
	var ticketPayload struct {
		Ticket string `json:"ticket"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&ticketPayload); err != nil {
		return nil, fmt.Errorf("decoding ticket: %w", err)
	}

	conn, err := net.Dial("tcp", serverURL.Host)
	if err != nil {
		return nil, fmt.Errorf("opening realtime connection: %w", err)
	}

	key, err := newWebSocketKey()
	if err != nil {
		_ = conn.Close()
		return nil, err
	}

	requestPath := fmt.Sprintf("/ws?ticket=%s&role=%s", ticketPayload.Ticket, role)

	request := strings.Join([]string{
		"GET " + requestPath + " HTTP/1.1",
		"Host: " + serverURL.Host,
		"Upgrade: websocket",
		"Connection: Upgrade",
		"Sec-WebSocket-Key: " + key,
		"Sec-WebSocket-Version: 13",
		"",
		"",
	}, "\r\n")

	if _, err := io.WriteString(conn, request); err != nil {
		_ = conn.Close()
		return nil, fmt.Errorf("requesting realtime connection: %w", err)
	}

	reader := bufio.NewReader(conn)
	response, err := http.ReadResponse(reader, nil)
	if err != nil {
		_ = conn.Close()
		return nil, fmt.Errorf("reading realtime handshake response: %w", err)
	}
	defer response.Body.Close()

	if response.StatusCode != http.StatusSwitchingProtocols {
		_ = conn.Close()
		return nil, fmt.Errorf("expected realtime connection to be accepted, got status %d", response.StatusCode)
	}

	if got := response.Header.Get("Sec-WebSocket-Accept"); got != webSocketAcceptKey(key) {
		_ = conn.Close()
		return nil, fmt.Errorf("realtime connection returned an invalid accept key")
	}

	return &realtimeTestConnection{conn: conn, reader: reader}, nil
}

func (suite *testSuite) closeRealtimeConnections() {
	for _, connection := range suite.realtimeConnections {
		if connection != nil {
			_ = connection.conn.Close()
		}
	}
	suite.realtimeConnections = map[string]*realtimeTestConnection{}
}

func (connection *realtimeTestConnection) readMessageEvent(timeout time.Duration) (realtimeMessageEvent, error) {
	if err := connection.conn.SetReadDeadline(time.Now().Add(timeout)); err != nil {
		return realtimeMessageEvent{}, err
	}

	payload, err := connection.readTextFrame()
	if err != nil {
		return realtimeMessageEvent{}, err
	}

	var event realtimeMessageEvent
	if err := json.Unmarshal(payload, &event); err != nil {
		return realtimeMessageEvent{}, fmt.Errorf("realtime message is not valid JSON: %w", err)
	}

	_ = connection.conn.SetReadDeadline(time.Time{})
	return event, nil
}

func (connection *realtimeTestConnection) readTextFrame() ([]byte, error) {
	header := make([]byte, 2)
	if _, err := io.ReadFull(connection.reader, header); err != nil {
		return nil, err
	}

	opcode := header[0] & 0x0f
	if opcode == 0x8 {
		return nil, fmt.Errorf("realtime connection was closed")
	}
	if opcode != 0x1 {
		return nil, fmt.Errorf("expected realtime text frame, got opcode %d", opcode)
	}

	masked := header[1]&0x80 != 0
	payloadLength := uint64(header[1] & 0x7f)
	switch payloadLength {
	case 126:
		extended := make([]byte, 2)
		if _, err := io.ReadFull(connection.reader, extended); err != nil {
			return nil, err
		}
		payloadLength = uint64(binary.BigEndian.Uint16(extended))
	case 127:
		extended := make([]byte, 8)
		if _, err := io.ReadFull(connection.reader, extended); err != nil {
			return nil, err
		}
		payloadLength = binary.BigEndian.Uint64(extended)
	}

	var maskKey []byte
	if masked {
		maskKey = make([]byte, 4)
		if _, err := io.ReadFull(connection.reader, maskKey); err != nil {
			return nil, err
		}
	}

	payload := make([]byte, payloadLength)
	if _, err := io.ReadFull(connection.reader, payload); err != nil {
		return nil, err
	}

	if masked {
		for index := range payload {
			payload[index] ^= maskKey[index%4]
		}
	}

	return payload, nil
}

func newWebSocketKey() (string, error) {
	key := make([]byte, 16)
	if _, err := rand.Read(key); err != nil {
		return "", fmt.Errorf("creating websocket key: %w", err)
	}

	return base64.StdEncoding.EncodeToString(key), nil
}

func webSocketAcceptKey(key string) string {
	hash := sha1.Sum([]byte(key + "258EAFA5-E914-47DA-95CA-C5AB0DC85B11"))
	return base64.StdEncoding.EncodeToString(hash[:])
}

func isRealtimeReadTimeout(err error) bool {
	var netErr net.Error
	return err != nil && (strings.Contains(err.Error(), "i/o timeout") || errors.As(err, &netErr) && netErr.Timeout())
}
