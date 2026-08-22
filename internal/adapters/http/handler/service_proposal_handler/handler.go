package service_proposal_handler

import (
	"net/http"

	httphandler "github.com/LoResuelvo/loresuelvo-api/internal/adapters/http/handler"
	serviceproposal "github.com/LoResuelvo/loresuelvo-api/internal/domain/service_proposal"
	"github.com/gin-gonic/gin"
)

type ServiceProposalHandler struct {
	serviceProposalService *serviceproposal.Service
}

func NewServiceProposalHandler(service *serviceproposal.Service) *ServiceProposalHandler {
	return &ServiceProposalHandler{
		serviceProposalService: service,
	}
}

func (h *ServiceProposalHandler) CreateServiceProposal(c *gin.Context) {
	auth0ID, ok := httphandler.GetAuthenticatedUserID(c)
	if !ok {
		return
	}

	var req createServiceProposalRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	scheduledOnUTC := req.ScheduledOn.UTC()
	amountInCents, err := httphandler.ParseAmountToCents(req.Amount)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	if req.EstimatedDurationMinutes == nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": serviceproposal.ErrEstimatedDurationRequired.Error()})
		return
	}
	serviceproposal, err := h.serviceProposalService.CreateServiceProposal(c.Request.Context(), auth0ID, req.ConsumerID, amountInCents, scheduledOnUTC, req.Description, *req.EstimatedDurationMinutes)
	if err != nil {
		handleCreateServiceProposalError(c, err)
		return
	}
	c.JSON(http.StatusCreated, serviceProposalCreationResponseFromDomain(serviceproposal))
}

func (h *ServiceProposalHandler) GetServiceProposals(c *gin.Context) {
	auth0ID, ok := httphandler.GetAuthenticatedUserID(c)
	if !ok {
		return
	}

	serviceProposals, err := h.serviceProposalService.GetServiceProposals(c.Request.Context(), auth0ID)
	if err != nil {
		httphandler.RespondError(c, http.StatusInternalServerError, "Could not get service proposals")
		return
	}

	response, err := serviceProposalSummaryResponsesFromDomain(serviceProposals, auth0ID)
	if err != nil {
		httphandler.RespondError(c, http.StatusInternalServerError, "Could not map service proposals")
		return
	}

	c.JSON(http.StatusOK, response)
}
