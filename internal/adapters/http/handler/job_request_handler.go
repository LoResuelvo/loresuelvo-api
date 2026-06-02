package handler

import (
	"errors"
	"fmt"
	"net/http"
	"strings"

	"github.com/LoResuelvo/loresuelvo-api/internal/adapters/http/middleware"
	jobrequest "github.com/LoResuelvo/loresuelvo-api/internal/domain/job_request"
	"github.com/gin-gonic/gin"
)

type JobRequestHandler struct {
	jobRequestService *jobrequest.Service
}

type createJobRequestRequest struct {
	ProviderID  int    `json:"provider_id"`
	Title       string `json:"title"`
	Description string `json:"description"`
}

type jobRequestResponse struct {
	ID             int    `json:"id"`
	ConversationID int    `json:"conversation_id"`
	Title          string `json:"title"`
	Description    string `json:"description"`
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

	createdJobRequest, err := h.jobRequestService.Create(auth0ID, req.ProviderID, req.Title, req.Description)
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

	c.JSON(http.StatusOK, jobRequests)
}

func jobRequestResponseFromDomain(createdJobRequest jobrequest.JobRequest) jobRequestResponse {
	return jobRequestResponse{
		ID:             createdJobRequest.ID,
		ConversationID: createdJobRequest.ConversationID,
		Title:          createdJobRequest.Title,
		Description:    createdJobRequest.Description,
	}
}
