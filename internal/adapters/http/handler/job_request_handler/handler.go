package job_request_handler

import (
	"fmt"
	"net/http"

	httphandler "github.com/LoResuelvo/loresuelvo-api/internal/adapters/http/handler"
	jobrequest "github.com/LoResuelvo/loresuelvo-api/internal/domain/job_request"
	"github.com/gin-gonic/gin"
)

type JobRequestHandler struct {
	jobRequestService *jobrequest.Service
}

func NewJobRequestHandler(jobRequestService *jobrequest.Service) *JobRequestHandler {
	return &JobRequestHandler{jobRequestService: jobRequestService}
}

func (h *JobRequestHandler) CreateJobRequest(c *gin.Context) {
	auth0ID, ok := httphandler.GetAuthenticatedUserID(c)
	if !ok {
		return
	}

	var req createJobRequestRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		httphandler.RespondError(c, http.StatusBadRequest, err.Error())
		return
	}

	createdJobRequest, err := h.jobRequestService.Create(c.Request.Context(), auth0ID, req.ProviderID, req.Title, req.Description, req.ImageFileIDs)
	if err != nil {
		handleCreateJobRequestError(c, err)
		return
	}

	c.Header("Location", fmt.Sprintf("/job-requests/%d", createdJobRequest.ID))
	c.JSON(http.StatusCreated, jobRequestResponseFromDomain(*createdJobRequest))
}

func (h *JobRequestHandler) CreateFromChatbotAssessment(c *gin.Context) {
	auth0ID, ok := httphandler.GetAuthenticatedUserID(c)
	if !ok {
		return
	}

	conversationID, err := httphandler.PositiveIDFromString(c.Param("conversationID"), "conversation id")
	if err != nil {
		httphandler.RespondError(c, http.StatusBadRequest, err.Error())
		return
	}

	var request createJobRequestFromChatbotRequest
	if err := c.ShouldBindJSON(&request); err != nil || request.ProviderID <= 0 {
		httphandler.RespondError(c, http.StatusBadRequest, jobrequest.ErrProviderRequired.Error())
		return
	}

	created, err := h.jobRequestService.CreateFromChatbotAssessment(c.Request.Context(), auth0ID, conversationID, request.ProviderID)
	if err != nil {
		handleCreateFromChatbotAssessmentError(c, err)
		return
	}

	c.Header("Location", fmt.Sprintf("/job-requests/%d", created.ID))
	c.JSON(http.StatusCreated, jobRequestResponseFromDomain(*created))
}

func (h *JobRequestHandler) GetJobRequests(c *gin.Context) {
	auth0ID, ok := httphandler.GetAuthenticatedUserID(c)
	if !ok {
		return
	}

	jobRequests, err := h.jobRequestService.GetJobRequests(c.Request.Context(), auth0ID)
	if err != nil {
		httphandler.RespondError(c, http.StatusInternalServerError, err.Error())
		return
	}

	c.JSON(http.StatusOK, jobRequestSummaryResponsesFromReadModel(jobRequests))
}

func (h *JobRequestHandler) AcceptJobRequest(c *gin.Context) {
	auth0ID, ok := httphandler.GetAuthenticatedUserID(c)
	if !ok {
		return
	}

	jobRequestID, err := httphandler.PositiveIDFromString(c.Param("jobRequestID"), "job request id")
	if err != nil {
		httphandler.RespondError(c, http.StatusBadRequest, err.Error())
		return
	}

	acceptedJobRequest, err := h.jobRequestService.Accept(c.Request.Context(), auth0ID, jobRequestID)
	if err != nil {
		handleAcceptJobRequestError(c, err)
		return
	}

	c.JSON(http.StatusOK, jobRequestResponseFromDomain(*acceptedJobRequest))
}
