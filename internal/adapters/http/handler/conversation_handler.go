package handler

import (
	"errors"
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/LoResuelvo/loresuelvo-api/internal/adapters/http/middleware"
	"github.com/LoResuelvo/loresuelvo-api/internal/domain/conversation"
	readmodel "github.com/LoResuelvo/loresuelvo-api/internal/domain/conversation/read_model"
	"github.com/gin-gonic/gin"
)

type ConversationHandler struct {
	conversationService *conversation.Service
}

type sendMessageRequest struct {
	Content string `json:"content"`
}

type createChatbotConversationRequest struct {
	Content string `json:"content"`
}

type sentMessageResponse struct {
	ID             int       `json:"id"`
	ConversationID int       `json:"conversation_id"`
	SenderRole     string    `json:"sender_role"`
	Content        string    `json:"content"`
	CreatedOn      time.Time `json:"created_on"`
}

type conversationDetailResponse struct {
	ID          int                             `json:"id"`
	Status      string                          `json:"status"`
	Counterpart conversationCounterpartResponse `json:"counterpart"`
	Messages    []conversationMessageResponse   `json:"messages"`
	UpdatedOn   time.Time                       `json:"updated_on"`
}

type conversationMessageResponse struct {
	ID         int       `json:"id"`
	SenderRole string    `json:"sender_role"`
	Content    string    `json:"content"`
	CreatedOn  time.Time `json:"created_on"`
}

type conversationSummaryResponse struct {
	ID          int                              `json:"id"`
	Status      string                           `json:"status"`
	Counterpart conversationCounterpartResponse  `json:"counterpart"`
	LastMessage *conversationLastMessageResponse `json:"last_message"`
	UpdatedOn   time.Time                        `json:"updated_on"`
}

type conversationCounterpartResponse struct {
	ID              int    `json:"id"`
	Role            string `json:"role"`
	Name            string `json:"name"`
	Surname         string `json:"surname"`
	CategoryName    string `json:"category_name,omitempty"`
	ProfilePhotoURL string `json:"profile_photo_url,omitempty"`
}

type conversationLastMessageResponse struct {
	ID         int       `json:"id"`
	SenderRole string    `json:"sender_role"`
	Content    string    `json:"content"`
	CreatedOn  time.Time `json:"created_on"`
}

func NewConversationHandler(conversationService *conversation.Service) *ConversationHandler {
	return &ConversationHandler{conversationService: conversationService}
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
	if errors.Is(err, conversation.ErrPendingConversationRequiresAcceptance) {
		c.JSON(http.StatusForbidden, gin.H{"error": err.Error()})
		return
	}
	if errors.Is(err, conversation.ErrPendingConversationMessageLimitReached) {
		c.JSON(http.StatusConflict, gin.H{"error": err.Error()})
		return
	}
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, conversationDetailResponseFromDomain(*foundConversation))
}

func (h *ConversationHandler) SendMessage(c *gin.Context) {
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

	var req sendMessageRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	sentMessage, err := h.conversationService.SendMessage(c.Request.Context(), auth0ID, conversationID, req.Content)
	if errors.Is(err, conversation.ErrMessageRequired) {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	if errors.Is(err, conversation.ErrConversationDoesNotExist) {
		c.JSON(http.StatusNotFound, gin.H{"error": err.Error()})
		return
	}
	if errors.Is(err, conversation.ErrConversationAccessDenied) {
		c.JSON(http.StatusForbidden, gin.H{"error": err.Error()})
		return
	}
	if errors.Is(err, conversation.ErrPendingConversationRequiresAcceptance) {
		c.JSON(http.StatusForbidden, gin.H{"error": err.Error()})
		return
	}
	if errors.Is(err, conversation.ErrPendingConversationMessageLimitReached) {
		c.JSON(http.StatusConflict, gin.H{"error": err.Error()})
		return
	}
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.Header("Location", fmt.Sprintf("/conversations/%d/messages/%d", conversationID, sentMessage.ID))
	c.JSON(http.StatusCreated, sentMessageResponseFromDomain(*sentMessage))
}

func (h *ConversationHandler) ListConversations(c *gin.Context) {
	auth0ID, ok := middleware.GetUserID(c)
	if !ok || strings.TrimSpace(auth0ID) == "" {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "missing user id"})
		return
	}

	summaries, err := h.conversationService.List(c.Request.Context(), auth0ID)
	if errors.Is(err, conversation.ErrConversationAccessDenied) {
		c.JSON(http.StatusForbidden, gin.H{"error": err.Error()})
		return
	}
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, conversationSummaryResponsesFromDomain(summaries))
}

func (h *ConversationHandler) CreateChatbotConversation(c *gin.Context) {
	auth0ID, ok := middleware.GetUserID(c)
	if !ok || strings.TrimSpace(auth0ID) == "" {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "missing user id"})
		return
	}

	var req createChatbotConversationRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	createdConversation, err := h.conversationService.CreateChatbotConversation(c.Request.Context(), auth0ID, req.Content)
	if errors.Is(err, conversation.ErrMessageRequired) || errors.Is(err, conversation.ErrChatbotResponseRequired) {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	if errors.Is(err, conversation.ErrOnlyConsumerCanMessageChatbot) {
		c.JSON(http.StatusForbidden, gin.H{"error": err.Error()})
		return
	}
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.Header("Location", fmt.Sprintf("/chatbot/conversations/%d", createdConversation.Conversation.Base().ID))
	c.JSON(http.StatusCreated, chatbotConversationResponseFromDomain(*createdConversation))
}

func conversationIDFromPath(value string) (int, error) {
	conversationID, err := strconv.Atoi(value)
	if err != nil || conversationID <= 0 {
		return 0, fmt.Errorf("conversation id must be a positive integer")
	}

	return conversationID, nil
}

func sentMessageResponseFromDomain(message conversation.Message) sentMessageResponse {
	return sentMessageResponse{
		ID:             message.ID,
		ConversationID: message.ConversationID,
		SenderRole:     message.SenderRole,
		Content:        message.Content,
		CreatedOn:      message.CreatedOn,
	}
}

type chatbotConversationResponse struct {
	ID                   int                           `json:"id"`
	Status               string                        `json:"status"`
	Title                string                        `json:"title"`
	ResponseStatus       string                        `json:"response_status"`
	Messages             []conversationMessageResponse `json:"messages"`
	Response             *conversationMessageResponse  `json:"response,omitempty"`
	RecommendedProviders []providerSummaryResponse     `json:"recommended_providers"`
}

func chatbotConversationResponseFromDomain(result conversation.ChatbotConversationResult) chatbotConversationResponse {
	chatbotConversation, _ := result.Conversation.(*conversation.ChatBotConversation)
	messages := make([]conversationMessageResponse, 0, len(result.Conversation.Messages()))
	var chatbotResponse *conversationMessageResponse
	for _, message := range result.Conversation.Messages() {
		messageResponse := conversationMessageResponse{
			ID:         message.ID,
			SenderRole: message.SenderRole,
			Content:    message.Content,
			CreatedOn:  message.CreatedOn,
		}
		messages = append(messages, messageResponse)
		if message.SenderRole == conversation.SenderChatbot {
			chatbotResponse = &messageResponse
		}
	}

	title := ""
	responseStatus := ""
	if chatbotConversation != nil {
		title = chatbotConversation.Title
		responseStatus = string(result.ResponseStatus)
	}

	recommendedProviders := make([]providerSummaryResponse, 0, len(result.RecommendedProviders))
	for _, recommendedProvider := range result.RecommendedProviders {
		recommendedProviders = append(recommendedProviders, providerSummaryResponseFromDomain(recommendedProvider))
	}

	return chatbotConversationResponse{
		ID:                   result.Conversation.Base().ID,
		Status:               result.Conversation.Base().Status,
		Title:                title,
		ResponseStatus:       responseStatus,
		Messages:             messages,
		Response:             chatbotResponse,
		RecommendedProviders: recommendedProviders,
	}
}

func conversationDetailResponseFromDomain(foundConversation readmodel.ConversationDetail) conversationDetailResponse {
	messages := make([]conversationMessageResponse, 0, len(foundConversation.Messages))
	for _, message := range foundConversation.Messages {
		messages = append(messages, conversationMessageResponse{
			ID:         message.ID,
			SenderRole: message.SenderRole,
			Content:    message.Content,
			CreatedOn:  message.CreatedOn,
		})
	}

	return conversationDetailResponse{
		ID:          foundConversation.ID,
		Status:      foundConversation.Status,
		Counterpart: conversationCounterpartResponseFromDomain(foundConversation.Counterpart),
		Messages:    messages,
		UpdatedOn:   foundConversation.UpdatedOn,
	}
}

func conversationSummaryResponsesFromDomain(summaries []readmodel.ConversationSummary) []conversationSummaryResponse {
	response := make([]conversationSummaryResponse, 0, len(summaries))
	for _, summary := range summaries {
		response = append(response, conversationSummaryResponseFromDomain(summary))
	}

	return response
}

func conversationSummaryResponseFromDomain(summary readmodel.ConversationSummary) conversationSummaryResponse {
	return conversationSummaryResponse{
		ID:          summary.ID,
		Status:      summary.Status,
		Counterpart: conversationCounterpartResponseFromDomain(summary.Counterpart),
		LastMessage: conversationLastMessageResponseFromDomain(summary.LastMessage),
		UpdatedOn:   summary.UpdatedOn,
	}
}

func conversationCounterpartResponseFromDomain(counterpart readmodel.ConversationParticipant) conversationCounterpartResponse {
	return conversationCounterpartResponse{
		ID:              counterpart.ID,
		Role:            counterpart.Role,
		Name:            counterpart.Name,
		Surname:         counterpart.Surname,
		CategoryName:    counterpart.CategoryName,
		ProfilePhotoURL: counterpart.ProfilePhotoURL,
	}
}

func conversationLastMessageResponseFromDomain(message *readmodel.MessageSummary) *conversationLastMessageResponse {
	if message == nil {
		return nil
	}

	return &conversationLastMessageResponse{
		ID:         message.ID,
		SenderRole: message.SenderRole,
		Content:    message.Content,
		CreatedOn:  message.CreatedOn,
	}
}
