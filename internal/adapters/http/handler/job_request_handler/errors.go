package job_request_handler

import (
	"errors"
	"net/http"

	httphandler "github.com/LoResuelvo/loresuelvo-api/internal/adapters/http/handler"
	"github.com/LoResuelvo/loresuelvo-api/internal/domain/conversation"
	jobrequest "github.com/LoResuelvo/loresuelvo-api/internal/domain/job_request"
	"github.com/gin-gonic/gin"
)

func handleCreateJobRequestError(c *gin.Context, err error) {
	if errors.Is(err, jobrequest.ErrTitleRequired) || errors.Is(err, jobrequest.ErrProviderRequired) || errors.Is(err, jobrequest.ErrJobRequestImageNotAvailable) {
		httphandler.RespondError(c, http.StatusBadRequest, err.Error())
		return
	}
	if errors.Is(err, jobrequest.ErrProviderDoesNotExist) {
		httphandler.RespondError(c, http.StatusNotFound, err.Error())
		return
	}
	if errors.Is(err, jobrequest.ErrOnlyConsumerCanCreateJobRequest) {
		httphandler.RespondError(c, http.StatusForbidden, err.Error())
		return
	}
	if errors.Is(err, jobrequest.ErrAlreadyExists) {
		httphandler.RespondError(c, http.StatusConflict, err.Error())
		return
	}

	httphandler.RespondError(c, http.StatusInternalServerError, err.Error())
}

func handleCreateFromChatbotAssessmentError(c *gin.Context, err error) {
	switch {
	case errors.Is(err, conversation.ErrConversationDoesNotExist), errors.Is(err, jobrequest.ErrProviderDoesNotExist):
		httphandler.RespondError(c, http.StatusNotFound, err.Error())
	case errors.Is(err, jobrequest.ErrOnlyConsumerCanCreateJobRequest), errors.Is(err, jobrequest.ErrChatbotConversationAccessDenied):
		httphandler.RespondError(c, http.StatusForbidden, err.Error())
	case errors.Is(err, jobrequest.ErrAssessmentNotContactable),
		errors.Is(err, jobrequest.ErrAssessmentNeedsMoreInformation),
		errors.Is(err, jobrequest.ErrProviderCategoryMismatch),
		errors.Is(err, jobrequest.ErrAlreadyExists):
		httphandler.RespondError(c, http.StatusConflict, err.Error())
	default:
		httphandler.RespondError(c, http.StatusInternalServerError, err.Error())
	}
}

func handleAcceptJobRequestError(c *gin.Context, err error) {
	if errors.Is(err, jobrequest.ErrJobRequestNotFound) || errors.Is(err, conversation.ErrConversationDoesNotExist) {
		httphandler.RespondError(c, http.StatusNotFound, err.Error())
		return
	}
	if errors.Is(err, jobrequest.ErrOnlyAssignedProviderCanAcceptJobRequest) {
		httphandler.RespondError(c, http.StatusForbidden, err.Error())
		return
	}
	if errors.Is(err, jobrequest.ErrOnlyPendingJobRequestCanBeAccepted) {
		httphandler.RespondError(c, http.StatusConflict, err.Error())
		return
	}
	if errors.Is(err, conversation.ErrOnlyPendingConversationCanBeActivated) {
		httphandler.RespondError(c, http.StatusConflict, err.Error())
		return
	}

	httphandler.RespondError(c, http.StatusInternalServerError, err.Error())
}
