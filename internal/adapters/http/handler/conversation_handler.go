package handler

import (
	"errors"
	"fmt"
	"net/http"
	"strings"

	"github.com/LoResuelvo/loresuelvo-api/internal/adapters/http/middleware"
	"github.com/LoResuelvo/loresuelvo-api/internal/domain/conversation"
	"github.com/gin-gonic/gin"
)

type ConversationHandler struct {
	conversationService *conversation.Service
}

type createConversationRequest struct {
	ProviderID int    `json:"provider_id"`
	Content    string `json:"content"`
}

type conversationResponse struct {
	ID     int    `json:"id"`
	Status string `json:"status"`
}

func NewConversationHandler(conversationService *conversation.Service) *ConversationHandler {
	return &ConversationHandler{conversationService: conversationService}
}

func (h *ConversationHandler) CreateConversation(c *gin.Context) {
	auth0ID, ok := middleware.GetUserID(c)
	if !ok || strings.TrimSpace(auth0ID) == "" {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "missing user id"})
		return
	}

	var req createConversationRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	createdConversation, err := h.conversationService.StartWorkRequest(auth0ID, req.ProviderID, req.Content)
	if errors.Is(err, conversation.ErrMessageRequired) || errors.Is(err, conversation.ErrProviderRequired) {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	if errors.Is(err, conversation.ErrProviderDoesNotExist) {
		c.JSON(http.StatusNotFound, gin.H{"error": err.Error()})
		return
	}
	if errors.Is(err, conversation.ErrOnlyConsumerCanStartWorkRequest) {
		c.JSON(http.StatusForbidden, gin.H{"error": err.Error()})
		return
	}
	if errors.Is(err, conversation.ErrAlreadyExists) {
		c.JSON(http.StatusConflict, gin.H{"error": err.Error()})
		return
	}
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.Header("Location", fmt.Sprintf("/conversations/%d", createdConversation.ID))
	c.JSON(http.StatusCreated, conversationResponse{
		ID:     createdConversation.ID,
		Status: createdConversation.Status,
	})
}
