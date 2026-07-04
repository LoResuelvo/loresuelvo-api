package service_proposal_handler

import "github.com/gin-gonic/gin"

type ServiceProposalHandler struct {
}

func NewServiceProposalHandler() *ServiceProposalHandler {
	return &ServiceProposalHandler{}
}

func (h *ServiceProposalHandler) CreateServiceProposal(c *gin.Context) {

}
