package realtime

import (
	"log/slog"
	"net/http"
	"sync"
	"time"

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
	closeOnce sync.Once
}

func newConnection(hub *Hub, conn *websocket.Conn, authID, role string, profileID int) *Connection {
	return &Connection{
		hub:       hub,
		conn:      conn,
		send:      make(chan []byte, 256),
		authID:    authID,
		role:      role,
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
		c.hub.removeConnection(c)
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
	// map from authID -> role -> profileID -> connections. A set is used at
	// the final level so multiple tabs/devices can subscribe for the same
	// authenticated participant without replacing one another.
	connections map[string]map[string]map[int]map[*Connection]struct{}
}

func NewHub() *Hub {
	return &Hub{
		connections: make(map[string]map[string]map[int]map[*Connection]struct{}),
	}
}

func (h *Hub) addConnection(conn *Connection) {
	h.mu.Lock()
	defer h.mu.Unlock()

	if _, ok := h.connections[conn.authID]; !ok {
		h.connections[conn.authID] = make(map[string]map[int]map[*Connection]struct{})
	}
	if _, ok := h.connections[conn.authID][conn.role]; !ok {
		h.connections[conn.authID][conn.role] = make(map[int]map[*Connection]struct{})
	}
	if _, ok := h.connections[conn.authID][conn.role][conn.profileID]; !ok {
		h.connections[conn.authID][conn.role][conn.profileID] = make(map[*Connection]struct{})
	}
	h.connections[conn.authID][conn.role][conn.profileID][conn] = struct{}{}
}

func (h *Hub) removeConnection(conn *Connection) {
	h.mu.Lock()
	defer h.mu.Unlock()

	if roleMap, ok := h.connections[conn.authID]; ok {
		if profileMap, ok := roleMap[conn.role]; ok {
			if connections, ok := profileMap[conn.profileID]; ok {
				if _, ok := connections[conn]; !ok {
					return
				}
				delete(connections, conn)
				if len(connections) == 0 {
					delete(profileMap, conn.profileID)
				}
				if len(profileMap) == 0 {
					delete(roleMap, conn.role)
				}
				if len(roleMap) == 0 {
					delete(h.connections, conn.authID)
				}
			}
		}
	}

	conn.closeOnce.Do(func() {
		close(conn.send)
	})
}

// Deliver sends an event to every local WebSocket connection for its target.
// Distributed transport and deduplication are intentionally owned by Dispatcher.
func (h *Hub) Deliver(event EventEnvelope) {
	if h == nil {
		return
	}

	h.mu.RLock()
	defer h.mu.RUnlock()
	h.deliverToAuthIDRoleAndProfile(event.TargetAuthID, event.TargetRole, event.TargetProfileID, event.Payload)
}

func (h *Hub) deliverToAuthIDRoleAndProfile(authID, role string, profileID int, event []byte) {
	if roleMap, ok := h.connections[authID]; ok {
		if profileMap, ok := roleMap[role]; ok {
			if connections, ok := profileMap[profileID]; ok {
				for conn := range connections {
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
}
