package steps_test

import (
	"bufio"
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
	CreatedOn  time.Time              `json:"created_on"`
}

type realtimeTestConnection struct {
	conn   net.Conn
	reader *bufio.Reader
}

func registerRealtimeMessageSteps(sc *godog.ScenarioContext, suite *testSuite) {
	sc.Step(`^que existe un chat activo entre el consumidor "([^"]*)" y el prestador "([^"]*)" con el mensaje inicial:$`, suite.thereIsActiveChatBetweenConsumerAndProviderWithInitialMessage)
	sc.Step(`^que el consumidor "([^"]*)" está disponible para recibir mensajes en tiempo real$`, suite.consumerIsAvailableForRealtimeMessages)
	sc.Step(`^que el prestador "([^"]*)" está disponible para recibir mensajes en tiempo real$`, suite.providerIsAvailableForRealtimeMessages)
	sc.Step(`^envío un mensaje en el chat con el prestador "([^"]*)":$`, suite.sendMessageInChatWithProvider)
	sc.Step(`^envío un mensaje en el chat con el consumidor "([^"]*)":$`, suite.sendMessageInChatWithConsumer)
	sc.Step(`^el consumidor "([^"]*)" recibe en tiempo real el mensaje:$`, suite.consumerReceivesRealtimeMessage)
	sc.Step(`^el prestador "([^"]*)" recibe en tiempo real el mensaje:$`, suite.providerReceivesRealtimeMessage)
	sc.Step(`^el consumidor "([^"]*)" no recibe mensajes en tiempo real$`, suite.consumerDoesNotReceiveRealtimeMessages)
	sc.Step(`^el prestador "([^"]*)" no recibe mensajes en tiempo real$`, suite.providerDoesNotReceiveRealtimeMessages)
}

func (suite *testSuite) thereIsActiveChatBetweenConsumerAndProviderWithInitialMessage(consumerEmail, providerEmail string, message *godog.DocString) error {
	if err := suite.createPendingConversationBetweenConsumerAndProvider(consumerEmail, providerEmail, normalizeDocString(message)); err != nil {
		return err
	}

	_, err := suite.database.Exec(
		`UPDATE conversations SET status = $1 WHERE id = $2`,
		conversationStatusActive,
		suite.lastConversationID,
	)
	if err != nil {
		return fmt.Errorf("activating chat fixture: %w", err)
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
