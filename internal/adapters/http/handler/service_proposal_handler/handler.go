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
	serviceproposal, err := h.serviceProposalService.CreateServiceProposal(auth0ID, req.ConsumerID, amountInCents, scheduledOnUTC, req.Description)
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

	serviceProposals, _ := h.serviceProposalService.GetServiceProposals(auth0ID)

	c.JSON(http.StatusOK, serviceProposals)
}
