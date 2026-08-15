package conversation_handler

import (
	"fmt"
	"net/http"

	httphandler "github.com/LoResuelvo/loresuelvo-api/internal/adapters/http/handler"
	"github.com/LoResuelvo/loresuelvo-api/internal/domain/conversation"
	"github.com/gin-gonic/gin"
)

type ConversationHandler struct {
	conversationService *conversation.Service
}

func NewConversationHandler(conversationService *conversation.Service) *ConversationHandler {
	return &ConversationHandler{conversationService: conversationService}
}

func (h *ConversationHandler) GetConversation(c *gin.Context) {
	auth0ID, ok := httphandler.GetAuthenticatedUserID(c)
	if !ok {
		return
	}

	conversationID, err := httphandler.PositiveIDFromString(c.Param("conversationID"), "conversation id")
	if err != nil {
		httphandler.RespondError(c, http.StatusBadRequest, err.Error())
		return
	}

	foundConversation, err := h.conversationService.GetByID(c.Request.Context(), auth0ID, conversationID)
	if err != nil {
		handleGetConversationError(c, err)
		return
	}

	c.JSON(http.StatusOK, conversationDetailResponseFromDomain(*foundConversation))
}

func (h *ConversationHandler) SendMessage(c *gin.Context) {
	auth0ID, ok := httphandler.GetAuthenticatedUserID(c)
	if !ok {
		return
	}

	conversationID, err := httphandler.PositiveIDFromString(c.Param("conversationID"), "conversation id")
	if err != nil {
		httphandler.RespondError(c, http.StatusBadRequest, err.Error())
		return
	}

	var req sendMessageRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		httphandler.RespondError(c, http.StatusBadRequest, err.Error())
		return
	}

	sentMessage, err := h.conversationService.SendMessage(
		c.Request.Context(),
		auth0ID,
		conversationID,
		req.Content,
		req.ImageFileIDs,
		req.AudioFileID,
	)
	if err != nil {
		handleSendMessageError(c, err)
		return
	}

	c.Header("Location", fmt.Sprintf("/conversations/%d/messages/%d", conversationID, sentMessage.ID))
	c.JSON(http.StatusCreated, sentMessageResponseFromDomain(*sentMessage))
}

func (h *ConversationHandler) ListWorkConversations(c *gin.Context) {
	auth0ID, ok := httphandler.GetAuthenticatedUserID(c)
	if !ok {
		return
	}

	summaries, err := h.conversationService.ListWorkConversations(c.Request.Context(), auth0ID)
	if err != nil {
		handleListWorkConversationsError(c, err)
		return
	}

	c.JSON(http.StatusOK, conversationSummaryResponsesFromDomain(summaries))
}

func (h *ConversationHandler) ListChatbotConversations(c *gin.Context) {
	auth0ID, ok := httphandler.GetAuthenticatedUserID(c)
	if !ok {
		return
	}

	summaries, err := h.conversationService.ListChatbotConversations(c.Request.Context(), auth0ID)
	if err != nil {
		handleListChatbotConversationsError(c, err)
		return
	}

	c.JSON(http.StatusOK, chatbotConversationSummaryResponsesFromDomain(summaries))
}

func (h *ConversationHandler) CreateChatbotConversation(c *gin.Context) {
	auth0ID, ok := httphandler.GetAuthenticatedUserID(c)
	if !ok {
		return
	}

	var req createChatbotConversationRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		httphandler.RespondError(c, http.StatusBadRequest, err.Error())
		return
	}

	createdConversation, err := h.conversationService.CreateChatbotConversation(c.Request.Context(), auth0ID, req.Content, req.ImageFileIDs)
	if err != nil {
		handleCreateChatbotConversationError(c, err)
		return
	}

	c.Header("Location", fmt.Sprintf("/chatbot/conversations/%d", createdConversation.Conversation.ID()))
	c.JSON(http.StatusCreated, chatbotConversationResponseFromDomain(*createdConversation))
}

func (h *ConversationHandler) ContinueChatbotConversation(c *gin.Context) {
	auth0ID, ok := httphandler.GetAuthenticatedUserID(c)
	if !ok {
		return
	}

	conversationID, err := httphandler.PositiveIDFromString(c.Param("conversationID"), "conversation id")
	if err != nil {
		httphandler.RespondError(c, http.StatusBadRequest, err.Error())
		return
	}

	var req chatbotMessageRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		httphandler.RespondError(c, http.StatusBadRequest, err.Error())
		return
	}

	turnResult, err := h.conversationService.ContinueChatbotConversation(c.Request.Context(), auth0ID, conversationID, req.Content, req.ImageFileIDs)
	if err != nil {
		handleContinueChatbotConversationError(c, err)
		return
	}

	c.Header("Location", fmt.Sprintf("/chatbot/conversations/%d/messages", conversationID))
	c.JSON(http.StatusCreated, chatbotConversationResponseFromDomain(*turnResult))
}
