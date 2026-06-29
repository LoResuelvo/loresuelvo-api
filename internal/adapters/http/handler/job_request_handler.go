package handler

import (
	"errors"
	"fmt"
	"net/http"
	"strconv"
	"strings"

	"github.com/LoResuelvo/loresuelvo-api/internal/adapters/http/middleware"
	"github.com/LoResuelvo/loresuelvo-api/internal/domain/conversation"
	jobrequest "github.com/LoResuelvo/loresuelvo-api/internal/domain/job_request"
	"github.com/gin-gonic/gin"
)

type JobRequestHandler struct {
	jobRequestService *jobrequest.Service
}

type createJobRequestRequest struct {
	ProviderID   int      `json:"provider_id"`
	Title        string   `json:"title"`
	Description  string   `json:"description"`
	ImageFileIDs []string `json:"image_file_ids"`
}

type createJobRequestFromChatbotRequest struct {
	ProviderID int `json:"provider_id"`
}

type jobRequestResponse struct {
	ID             int                    `json:"id"`
	ConversationID int                    `json:"conversation_id"`
	Title          string                 `json:"title"`
	Description    string                 `json:"description"`
	Status         string                 `json:"status"`
	Images         []messageImageResponse `json:"images"`
}

type jobRequestSummaryResponse struct {
	ID             int                         `json:"id"`
	ConversationID int                         `json:"conversation_id"`
	Title          string                      `json:"title"`
	Description    string                      `json:"description"`
	Status         string                      `json:"status"`
	Requester      jobRequestRequesterResponse `json:"requester"`
	Images         []messageImageResponse      `json:"images"`
}

type jobRequestRequesterResponse struct {
	Name    string `json:"name"`
	Surname string `json:"surname"`
}

func NewJobRequestHandler(jobRequestService *jobrequest.Service) *JobRequestHandler {
	return &JobRequestHandler{jobRequestService: jobRequestService}
}

func (h *JobRequestHandler) CreateJobRequest(c *gin.Context) {
	auth0ID, ok := middleware.GetUserID(c)
	if !ok || strings.TrimSpace(auth0ID) == "" {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "missing user id"})
		return
	}

	var req createJobRequestRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	createdJobRequest, err := h.jobRequestService.Create(auth0ID, req.ProviderID, req.Title, req.Description, req.ImageFileIDs)
	if errors.Is(err, jobrequest.ErrTitleRequired) || errors.Is(err, jobrequest.ErrProviderRequired) {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	if errors.Is(err, jobrequest.ErrProviderDoesNotExist) {
		c.JSON(http.StatusNotFound, gin.H{"error": err.Error()})
		return
	}
	if errors.Is(err, jobrequest.ErrOnlyConsumerCanCreateJobRequest) {
		c.JSON(http.StatusForbidden, gin.H{"error": err.Error()})
		return
	}
	if errors.Is(err, jobrequest.ErrAlreadyExists) {
		c.JSON(http.StatusConflict, gin.H{"error": err.Error()})
		return
	}
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.Header("Location", fmt.Sprintf("/job-requests/%d", createdJobRequest.ID))
	c.JSON(http.StatusCreated, jobRequestResponseFromDomain(*createdJobRequest))
}

func (h *JobRequestHandler) CreateFromChatbotAssessment(c *gin.Context) {
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
	var request createJobRequestFromChatbotRequest
	if err := c.ShouldBindJSON(&request); err != nil || request.ProviderID <= 0 {
		c.JSON(http.StatusBadRequest, gin.H{"error": jobrequest.ErrProviderRequired.Error()})
		return
	}

	created, err := h.jobRequestService.CreateFromChatbotAssessment(c.Request.Context(), auth0ID, conversationID, request.ProviderID)
	switch {
	case errors.Is(err, conversation.ErrConversationDoesNotExist), errors.Is(err, jobrequest.ErrProviderDoesNotExist):
		c.JSON(http.StatusNotFound, gin.H{"error": err.Error()})
	case errors.Is(err, jobrequest.ErrOnlyConsumerCanCreateJobRequest), errors.Is(err, jobrequest.ErrChatbotConversationAccessDenied):
		c.JSON(http.StatusForbidden, gin.H{"error": err.Error()})
	case errors.Is(err, jobrequest.ErrAssessmentNotContactable),
		errors.Is(err, jobrequest.ErrAssessmentNeedsMoreInformation),
		errors.Is(err, jobrequest.ErrProviderCategoryMismatch),
		errors.Is(err, jobrequest.ErrAlreadyExists):
		c.JSON(http.StatusConflict, gin.H{"error": err.Error()})
	case err != nil:
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
	default:
		c.Header("Location", fmt.Sprintf("/job-requests/%d", created.ID))
		c.JSON(http.StatusCreated, jobRequestResponseFromDomain(*created))
	}
}

func (h *JobRequestHandler) GetJobRequests(c *gin.Context) {
	auth0ID, ok := middleware.GetUserID(c)
	if !ok || strings.TrimSpace(auth0ID) == "" {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "missing user id"})
		return
	}

	jobRequests, err := h.jobRequestService.GetJobRequests(auth0ID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	formatedJobRequests := make([]jobRequestSummaryResponse, len(jobRequests))
	for i, jobRequest := range jobRequests {
		formatedJobRequests[i] = jobRequestSummaryResponse{
			ID:             jobRequest.ID,
			ConversationID: jobRequest.ConversationID,
			Title:          jobRequest.Title,
			Description:    jobRequest.Description,
			Status:         jobRequest.Status,
			Requester: jobRequestRequesterResponse{
				Name:    jobRequest.Requester.Name,
				Surname: jobRequest.Requester.Surname,
			},
			Images: []messageImageResponse{},
		}
	}

	c.JSON(http.StatusOK, formatedJobRequests)
}

func (h *JobRequestHandler) AcceptJobRequest(c *gin.Context) {
	auth0ID, ok := middleware.GetUserID(c)
	if !ok || strings.TrimSpace(auth0ID) == "" {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "missing user id"})
		return
	}

	jobRequestID, err := jobRequestIDFromPath(c.Param("jobRequestID"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	acceptedJobRequest, err := h.jobRequestService.Accept(c.Request.Context(), auth0ID, jobRequestID)
	if errors.Is(err, jobrequest.ErrJobRequestNotFound) || errors.Is(err, conversation.ErrConversationDoesNotExist) {
		c.JSON(http.StatusNotFound, gin.H{"error": err.Error()})
		return
	}
	if errors.Is(err, jobrequest.ErrOnlyAssignedProviderCanAcceptJobRequest) {
		c.JSON(http.StatusForbidden, gin.H{"error": err.Error()})
		return
	}
	if errors.Is(err, jobrequest.ErrOnlyPendingJobRequestCanBeAccepted) {
		c.JSON(http.StatusConflict, gin.H{"error": err.Error()})
		return
	}
	if errors.Is(err, conversation.ErrOnlyPendingConversationCanBeActivated) {
		c.JSON(http.StatusConflict, gin.H{"error": err.Error()})
		return
	}
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, jobRequestResponseFromDomain(*acceptedJobRequest))
}

func jobRequestIDFromPath(value string) (int, error) {
	jobRequestID, err := strconv.Atoi(value)
	if err != nil || jobRequestID <= 0 {
		return 0, fmt.Errorf("job request id must be a positive integer")
	}

	return jobRequestID, nil
}

func jobRequestResponseFromDomain(createdJobRequest jobrequest.JobRequest) jobRequestResponse {
	return jobRequestResponse{
		ID:             createdJobRequest.ID,
		ConversationID: createdJobRequest.ConversationID,
		Title:          createdJobRequest.Title,
		Description:    createdJobRequest.Description,
		Status:         string(createdJobRequest.Status),
		Images:         []messageImageResponse{},
	}
}
