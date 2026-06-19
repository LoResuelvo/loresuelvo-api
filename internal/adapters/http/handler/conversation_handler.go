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
	providerreadmodel "github.com/LoResuelvo/loresuelvo-api/internal/domain/provider/read_model"
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
	ID        int                           `json:"id"`
	Type      string                        `json:"type"`
	Status    string                        `json:"status"`
	Work      *workConversationDetail       `json:"work,omitempty"`
	Chatbot   *chatbotConversationDetail    `json:"chatbot,omitempty"`
	Messages  []conversationMessageResponse `json:"messages"`
	UpdatedOn time.Time                     `json:"updated_on"`
}

type workConversationDetail struct {
	Counterpart conversationCounterpartResponse `json:"counterpart"`
}

type chatbotConversationDetail struct {
	Title                string                       `json:"title"`
	ResponseStatus       string                       `json:"response_status"`
	DiagnosisCompleted   bool                         `json:"diagnosis_completed"`
	RecommendedCategory  *recommendedCategoryResponse `json:"recommended_category,omitempty"`
	RecommendedProviders []providerSummaryResponse    `json:"recommended_providers"`
}

type recommendedCategoryResponse struct {
	ID   int    `json:"id"`
	Name string `json:"name"`
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
	LastMessage *conversationLastMessageResponse `json:"last_message"`
	UpdatedOn   time.Time                        `json:"updated_on"`
}

type workConversationSummaryResponse struct {
	conversationSummaryResponse
	Counterpart conversationCounterpartResponse `json:"counterpart"`
}

type chatbotConversationSummaryResponse struct {
	conversationSummaryResponse
	Title string `json:"title"`
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

func (h *ConversationHandler) ListWorkConversations(c *gin.Context) {
	auth0ID, ok := middleware.GetUserID(c)
	if !ok || strings.TrimSpace(auth0ID) == "" {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "missing user id"})
		return
	}

	summaries, err := h.conversationService.ListWorkConversations(c.Request.Context(), auth0ID)
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

func (h *ConversationHandler) ListChatbotConversations(c *gin.Context) {
	auth0ID, ok := middleware.GetUserID(c)
	if !ok || strings.TrimSpace(auth0ID) == "" {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "missing user id"})
		return
	}

	summaries, err := h.conversationService.ListChatbotConversations(c.Request.Context(), auth0ID)
	if errors.Is(err, conversation.ErrOnlyConsumerCanListChatbotConversations) {
		c.JSON(http.StatusForbidden, gin.H{"error": err.Error()})
		return
	}
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, chatbotConversationSummaryResponsesFromDomain(summaries))
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

func (h *ConversationHandler) ContinueChatbotConversation(c *gin.Context) {
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

	turnResult, err := h.conversationService.ContinueChatbotConversation(c.Request.Context(), auth0ID, conversationID, req.Content)
	if errors.Is(err, conversation.ErrMessageRequired) || errors.Is(err, conversation.ErrChatbotResponseRequired) {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	if errors.Is(err, conversation.ErrOnlyConsumerCanMessageChatbot) || errors.Is(err, conversation.ErrConversationAccessDenied) {
		c.JSON(http.StatusForbidden, gin.H{"error": err.Error()})
		return
	}
	if errors.Is(err, conversation.ErrConversationDoesNotExist) {
		c.JSON(http.StatusNotFound, gin.H{"error": err.Error()})
		return
	}
	if errors.Is(err, conversation.ErrChatbotConversationAlreadyProcessing) {
		c.JSON(http.StatusConflict, gin.H{"error": err.Error()})
		return
	}
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.Header("Location", fmt.Sprintf("/chatbot/conversations/%d/messages", conversationID))
	c.JSON(http.StatusCreated, chatbotConversationResponseFromDomain(*turnResult))
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
	ID     int    `json:"id"`
	Status string `json:"status"`
	chatbotConversationDetail
	Messages []conversationMessageResponse `json:"messages"`
	Response *conversationMessageResponse  `json:"response,omitempty"`
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
	if chatbotConversation != nil {
		title = chatbotConversation.Title
	}

	var recommendedCategory *recommendedCategoryResponse
	if result.RecommendedCategory != nil {
		recommendedCategory = &recommendedCategoryResponse{
			ID:   result.RecommendedCategory.ID,
			Name: result.RecommendedCategory.Name,
		}
	}

	return chatbotConversationResponse{
		ID:     result.Conversation.Base().ID,
		Status: result.Conversation.Base().Status,
		chatbotConversationDetail: chatbotConversationDetail{
			Title:                title,
			ResponseStatus:       string(result.ResponseStatus),
			DiagnosisCompleted:   result.DiagnosisCompleted,
			RecommendedCategory:  recommendedCategory,
			RecommendedProviders: providerSummaryResponsesFromDomain(result.RecommendedProviders),
		},
		Messages: messages,
		Response: chatbotResponse,
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
		ID:        foundConversation.ID,
		Type:      foundConversation.Type,
		Status:    foundConversation.Status,
		Work:      workConversationDetailResponseFromDomain(foundConversation.Work),
		Chatbot:   chatbotConversationDetailResponseFromDomain(foundConversation.Chatbot),
		Messages:  messages,
		UpdatedOn: foundConversation.UpdatedOn,
	}
}

func workConversationDetailResponseFromDomain(detail *readmodel.WorkConversationDetail) *workConversationDetail {
	if detail == nil {
		return nil
	}

	return &workConversationDetail{
		Counterpart: conversationCounterpartResponseFromDomain(detail.Counterpart),
	}
}

func chatbotConversationDetailResponseFromDomain(detail *readmodel.ChatbotConversationDetail) *chatbotConversationDetail {
	if detail == nil {
		return nil
	}

	var recommendedCategory *recommendedCategoryResponse
	if detail.RecommendedCategory != nil {
		recommendedCategory = &recommendedCategoryResponse{
			ID:   detail.RecommendedCategory.ID,
			Name: detail.RecommendedCategory.Name,
		}
	}

	return &chatbotConversationDetail{
		Title:                detail.Title,
		ResponseStatus:       detail.ResponseStatus,
		DiagnosisCompleted:   detail.DiagnosisCompleted,
		RecommendedCategory:  recommendedCategory,
		RecommendedProviders: providerSummaryResponsesFromDomain(detail.RecommendedProviders),
	}
}

func providerSummaryResponsesFromDomain(providers []providerreadmodel.ProviderSummary) []providerSummaryResponse {
	response := make([]providerSummaryResponse, 0, len(providers))
	for _, foundProvider := range providers {
		response = append(response, providerSummaryResponseFromDomain(foundProvider))
	}

	return response
}

func chatbotConversationSummaryResponsesFromDomain(summaries []readmodel.ConversationSummary) []chatbotConversationSummaryResponse {
	response := make([]chatbotConversationSummaryResponse, 0, len(summaries))
	for _, summary := range summaries {
		response = append(response, chatbotConversationSummaryResponseFromDomain(summary))
	}

	return response
}

func chatbotConversationSummaryResponseFromDomain(summary readmodel.ConversationSummary) chatbotConversationSummaryResponse {
	title := ""
	if summary.Chatbot != nil {
		title = summary.Chatbot.Title
	}

	return chatbotConversationSummaryResponse{
		conversationSummaryResponse: baseConversationSummaryResponseFromDomain(summary),
		Title:                       title,
	}
}

func conversationSummaryResponsesFromDomain(summaries []readmodel.ConversationSummary) []workConversationSummaryResponse {
	response := make([]workConversationSummaryResponse, 0, len(summaries))
	for _, summary := range summaries {
		response = append(response, conversationSummaryResponseFromDomain(summary))
	}

	return response
}

func conversationSummaryResponseFromDomain(summary readmodel.ConversationSummary) workConversationSummaryResponse {
	var counterpart readmodel.ConversationParticipant
	if summary.Work != nil {
		counterpart = summary.Work.Counterpart
	}

	return workConversationSummaryResponse{
		conversationSummaryResponse: baseConversationSummaryResponseFromDomain(summary),
		Counterpart:                 conversationCounterpartResponseFromDomain(counterpart),
	}
}

func baseConversationSummaryResponseFromDomain(summary readmodel.ConversationSummary) conversationSummaryResponse {
	return conversationSummaryResponse{
		ID:          summary.ID,
		Status:      summary.Status,
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
