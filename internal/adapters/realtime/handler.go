package realtime

import (
	"net/http"

	"github.com/LoResuelvo/loresuelvo-api/internal/adapters/http/middleware"
	"github.com/LoResuelvo/loresuelvo-api/internal/domain/conversation"
	"github.com/LoResuelvo/loresuelvo-api/internal/domain/user"
	"github.com/gin-gonic/gin"
	"github.com/gorilla/websocket"
)

type authenticatedUserFinder interface {
	FindByAuthID(authID string) (user.User, error)
}

type Handler struct {
	hub         *Hub
	userFinder  authenticatedUserFinder
	ticketStore *TicketStore
}

func NewHandler(hub *Hub, userFinder authenticatedUserFinder, ticketStore *TicketStore) *Handler {
	return &Handler{
		hub:         hub,
		userFinder:  userFinder,
		ticketStore: ticketStore,
	}
}

func (h *Handler) IssueTicket(c *gin.Context) {
	auth0ID, ok := middleware.GetUserID(c)
	if !ok || auth0ID == "" {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "missing user id"})
		return
	}

	ticket, err := h.ticketStore.Issue(auth0ID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to issue ticket"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"ticket": ticket})
}

// Handle upgrades an HTTP connection to WebSocket.
// It expects a valid one-time ticket in the query parameter.
func (h *Handler) Handle(c *gin.Context) {
	ticket := c.Query("ticket")
	if ticket == "" {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "missing ticket query parameter"})
		return
	}

	auth0ID, valid := h.ticketStore.Consume(ticket)
	if !valid || auth0ID == "" {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "invalid or expired ticket"})
		return
	}

	role := c.Query("role")
	if role != "consumer" && role != "provider" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "missing or invalid role query parameter"})
		return
	}

	conn, err := h.upgrade(c.Writer, c.Request)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to upgrade connection"})
		return
	}

	profileID, err := h.resolveParticipantForRole(auth0ID, role)
	if err != nil {
		_ = conn.Close()
		c.JSON(http.StatusUnauthorized, gin.H{"error": "failed to resolve user role"})
		return
	}

	connection := newConnection(h.hub, conn, auth0ID, role, profileID)
	h.hub.register <- connection

	go connection.writePump()
	go connection.readPump()
}

func (h *Handler) upgrade(w http.ResponseWriter, r *http.Request) (*websocket.Conn, error) {
	return upgrader.Upgrade(w, r, nil)
}

func (h *Handler) resolveParticipantForRole(auth0ID, role string) (profileID int, err error) {
	foundUser, err := h.userFinder.FindByAuthID(auth0ID)
	if err != nil || foundUser.Base().Role != role {
		return 0, conversation.ErrConversationAccessDenied
	}
	return foundUser.Base().ID, nil
}
