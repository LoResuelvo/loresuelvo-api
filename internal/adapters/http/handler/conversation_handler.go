package handler

import (
	"errors"
	"fmt"
	"net/http"
	"strconv"
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

type conversationDetailResponse struct {
	ID         int                           `json:"id"`
	ConsumerID int                           `json:"consumer_id"`
	ProviderID int                           `json:"provider_id"`
	Status     string                        `json:"status"`
	Messages   []conversationMessageResponse `json:"messages"`
}

type conversationMessageResponse struct {
	ID             int    `json:"id"`
	ConversationID int    `json:"conversation_id"`
	SenderRole     string `json:"sender_role"`
	Content        string `json:"content"`
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

func (h *ConversationHandler) GetConversation(c *gin.Context) {
	auth0ID, ok := middleware.GetUserID(c)
	if !ok || strings.TrimSpace(auth0ID) == "" {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "missing user id"})
		return
	}

	conversationID, err := conversationIDFromPath(c.Param("conversationID"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	foundConversation, err := h.conversationService.GetByID(c.Request.Context(), auth0ID, conversationID)
	if errors.Is(err, conversation.ErrConversationDoesNotExist) {
		c.JSON(http.StatusNotFound, gin.H{"error": err.Error()})
		return
	}
	if errors.Is(err, conversation.ErrConversationAccessDenied) {
		c.JSON(http.StatusForbidden, gin.H{"error": err.Error()})
		return
	}
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, conversationDetailResponseFromDomain(*foundConversation))
}

func conversationIDFromPath(value string) (int, error) {
	conversationID, err := strconv.Atoi(value)
	if err != nil || conversationID <= 0 {
		return 0, fmt.Errorf("conversation id must be a positive integer")
	}

	return conversationID, nil
}

func conversationDetailResponseFromDomain(foundConversation conversation.Conversation) conversationDetailResponse {
	messages := make([]conversationMessageResponse, 0, len(foundConversation.Messages))
	for _, message := range foundConversation.Messages {
		messages = append(messages, conversationMessageResponse{
			ID:             message.ID,
			ConversationID: message.ConversationID,
			SenderRole:     message.SenderRole,
			Content:        message.Content,
		})
	}

	return conversationDetailResponse{
		ID:         foundConversation.ID,
		ConsumerID: foundConversation.ConsumerID,
		ProviderID: foundConversation.ProviderID,
		Status:     foundConversation.Status,
		Messages:   messages,
	}
}
