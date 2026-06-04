package realtime

import (
	"net/http"

	"github.com/LoResuelvo/loresuelvo-api/internal/adapters/http/middleware"
	"github.com/LoResuelvo/loresuelvo-api/internal/domain/conversation"
	"github.com/gin-gonic/gin"
	"github.com/gorilla/websocket"
)

type Handler struct {
	hub                *Hub
	consumerIDFinder   conversation.ConsumerIDFinder
	providerIDFinder   conversation.ProviderIDFinder
}

func NewHandler(hub *Hub, consumerIDFinder conversation.ConsumerIDFinder, providerIDFinder conversation.ProviderIDFinder) *Handler {
	return &Handler{
		hub:                hub,
		consumerIDFinder:   consumerIDFinder,
		providerIDFinder:   providerIDFinder,
	}
}

// Handle upgrades an HTTP connection to WebSocket after auth middleware has validated the JWT.
// It expects auth middleware to have already set userID in context.
func (h *Handler) Handle(c *gin.Context) {
	auth0ID, ok := middleware.GetUserID(c)
	if !ok || auth0ID == "" {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "missing user id"})
		return
	}

	conn, err := h.upgrade(c.Writer, c.Request)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to upgrade connection"})
		return
	}

	role, profileID, err := h.resolveParticipant(auth0ID)
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

func (h *Handler) resolveParticipant(auth0ID string) (role string, profileID int, err error) {
	if profileID, err = h.consumerIDFinder.FindIDByAuthID(auth0ID); err == nil {
		return conversation.SenderConsumer, profileID, nil
	}

	if profileID, err = h.providerIDFinder.FindIDByAuthID(auth0ID); err == nil {
		return conversation.SenderProvider, profileID, nil
	}

	return "", 0, err
}