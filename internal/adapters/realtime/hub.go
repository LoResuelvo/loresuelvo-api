package realtime

import (
	"context"
	"encoding/json"
	"log/slog"
	"net/http"
	"sync"
	"time"

	"github.com/LoResuelvo/loresuelvo-api/internal/domain/conversation"
	"github.com/gorilla/websocket"
)

const writeWait = 10 * time.Second

var upgrader = websocket.Upgrader{
	ReadBufferSize:  1024,
	WriteBufferSize: 1024,
	CheckOrigin: func(r *http.Request) bool {
		return true
	},
}

type Connection struct {
	hub       *Hub
	conn      *websocket.Conn
	send      chan []byte
	authID    string
	role      string
	profileID int
}

func newConnection(hub *Hub, conn *websocket.Conn, authID, role string, profileID int) *Connection {
	return &Connection{
		hub:       hub,
		conn:      conn,
		send:      make(chan []byte, 256),
		authID:   authID,
		role:     role,
		profileID: profileID,
	}
}

func (c *Connection) writePump() {
	defer func() {
		_ = c.conn.Close()
	}()

	ticker := time.NewTicker(30 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case message, ok := <-c.send:
			if err := c.conn.SetWriteDeadline(time.Now().Add(writeWait)); err != nil {
				return
			}
			if !ok {
				_ = c.conn.WriteMessage(websocket.CloseMessage, []byte{})
				return
			}

			w, err := c.conn.NextWriter(websocket.TextMessage)
			if err != nil {
				return
			}
			if _, err := w.Write(message); err != nil {
				return
			}
			if err := w.Close(); err != nil {
				return
			}

		case <-ticker.C:
			if err := c.conn.SetWriteDeadline(time.Now().Add(writeWait)); err != nil {
				return
			}
			if err := c.conn.WriteMessage(websocket.PingMessage, nil); err != nil {
				return
			}
		}
	}
}

func (c *Connection) readPump() {
	defer func() {
		c.hub.unregister <- c
		_ = c.conn.Close()
	}()

	for {
		if _, _, err := c.conn.ReadMessage(); err != nil {
			break
		}
	}
}

type Hub struct {
	mu sync.RWMutex
	// map from authID -> role -> profileID -> *Connection
	connections map[string]map[string]map[int]*Connection

	register   chan *Connection
	unregister chan *Connection
}

func newHub() *Hub {
	return &Hub{
		connections: make(map[string]map[string]map[int]*Connection),
		register:    make(chan *Connection),
		unregister: make(chan *Connection),
	}
}

// NewHub creates a new Hub for managing WebSocket connections.
func NewHub() *Hub {
	return newHub()
}

func (h *Hub) Run(ctx context.Context) {
	for {
		select {
		case <-ctx.Done():
			return
		case conn := <-h.register:
			h.addConnection(conn)
		case conn := <-h.unregister:
			h.removeConnection(conn)
		}
	}
}

func (h *Hub) addConnection(conn *Connection) {
	h.mu.Lock()
	defer h.mu.Unlock()

	if _, ok := h.connections[conn.authID]; !ok {
		h.connections[conn.authID] = make(map[string]map[int]*Connection)
	}
	if _, ok := h.connections[conn.authID][conn.role]; !ok {
		h.connections[conn.authID][conn.role] = make(map[int]*Connection)
	}
	h.connections[conn.authID][conn.role][conn.profileID] = conn
}

func (h *Hub) removeConnection(conn *Connection) {
	h.mu.Lock()
	defer h.mu.Unlock()

	if roleMap, ok := h.connections[conn.authID]; ok {
		if profileMap, ok := roleMap[conn.role]; ok {
			if _, ok := profileMap[conn.profileID]; ok {
				delete(profileMap, conn.profileID)
				if len(profileMap) == 0 {
					delete(roleMap, conn.role)
				}
				if len(roleMap) == 0 {
					delete(h.connections, conn.authID)
				}
			}
		}
	}

	close(conn.send)
}

// BroadcastMessage broadcasts a message event to all participants of a conversation except the sender.
// consumerAuthID, consumerProfileID: auth ID and profile ID of the consumer participant
// providerAuthID, providerProfileID: auth ID and profile ID of the provider participant
// senderAuthID, senderRole: the sender to exclude from delivery
// event: the JSON encoded event bytes
func (h *Hub) BroadcastMessage(ctx context.Context, consumerAuthID string, consumerProfileID int, providerAuthID string, providerProfileID int, senderAuthID string, senderRole string, event []byte) {
	h.mu.RLock()
	defer h.mu.RUnlock()

	if consumerAuthID != senderAuthID {
		h.deliverToAuthIDRoleAndProfile(consumerAuthID, conversation.SenderConsumer, consumerProfileID, event)
	}
	if providerAuthID != senderAuthID {
		h.deliverToAuthIDRoleAndProfile(providerAuthID, conversation.SenderProvider, providerProfileID, event)
	}
}

func (h *Hub) deliverToAuthIDRoleAndProfile(authID, role string, profileID int, event []byte) {
	if roleMap, ok := h.connections[authID]; ok {
		if profileMap, ok := roleMap[role]; ok {
			if conn, ok := profileMap[profileID]; ok {
				select {
				case conn.send <- event:
				default:
					slog.Warn("failed to queue realtime message, connection buffer full",
						"authID", authID, "role", role, "profileID", profileID)
				}
			}
		}
	}
}

// BuildMessageEvent creates the JSON payload for a conversation.message.created event
func BuildMessageEvent(conversationID int, messageID int, senderRole string, content string, createdOn time.Time) ([]byte, error) {
	event := realtimeMessageEvent{
		Type:           "conversation.message.created",
		ConversationID: conversationID,
		Message: realtimeEventMessage{
			ID:         messageID,
			SenderRole: senderRole,
			Content:    content,
			CreatedOn:  createdOn,
		},
	}
	return json.Marshal(event)
}

type realtimeMessageEvent struct {
	Type           string               `json:"type"`
	ConversationID int                  `json:"conversation_id"`
	Message        realtimeEventMessage `json:"message"`
}

type realtimeEventMessage struct {
	ID         int       `json:"id"`
	SenderRole string    `json:"sender_role"`
	Content    string    `json:"content"`
	CreatedOn  time.Time `json:"created_on"`
}