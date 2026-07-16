package service_proposal_handler

import (
	"fmt"
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
	serviceproposal, err := h.serviceProposalService.CreateServiceProposal(c.Request.Context(), auth0ID, req.ConsumerID, amountInCents, scheduledOnUTC, req.Description)
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

func (h *ServiceProposalHandler) AcceptServiceProposal(c *gin.Context) {
	auth0ID, ok := httphandler.GetAuthenticatedUserID(c)
	if !ok {
		return
	}

	proposalID, err := httphandler.PositiveIDFromString(c.Param("serviceProposalID"), "service proposal id")
	if err != nil {
		httphandler.RespondError(c, http.StatusBadRequest, err.Error())
		return
	}

	order, err := h.serviceProposalService.Accept(c.Request.Context(), auth0ID, proposalID)
	if err != nil {
		handleAcceptServiceProposalError(c, err)
		return
	}

	c.Header("Location", fmt.Sprintf("/work-orders/%d", order.ID))
	c.JSON(http.StatusCreated, workOrderResponseFromDomain(order))
}
