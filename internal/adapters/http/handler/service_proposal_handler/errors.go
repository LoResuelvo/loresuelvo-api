package service_proposal_handler

import (
	"errors"
	"net/http"

	httphandler "github.com/LoResuelvo/loresuelvo-api/internal/adapters/http/handler"
	serviceproposal "github.com/LoResuelvo/loresuelvo-api/internal/domain/service_proposal"
	"github.com/gin-gonic/gin"
)

func handleCreateServiceProposalError(c *gin.Context, err error) {
	if errors.Is(err, serviceproposal.ErrInvalidAmount) || errors.Is(err, serviceproposal.ErrInvalidScheduledOn) {
		httphandler.RespondError(c, http.StatusBadRequest, err.Error())
		return
	}
	if errors.Is(err, serviceproposal.ErrProviderRequired) {
		httphandler.RespondError(c, http.StatusForbidden, err.Error())
		return
	}
	if errors.Is(err, serviceproposal.ErrConsumerRequired) {
		httphandler.RespondError(c, http.StatusNotFound, err.Error())
		return
	}
	if errors.Is(err, serviceproposal.ErrConversationRequired) || errors.Is(err, serviceproposal.ErrConversationNotActive) {
		httphandler.RespondError(c, http.StatusConflict, err.Error())
		return
	}

	httphandler.RespondError(c, http.StatusInternalServerError, err.Error())
}
