package work_order_handler

import (
	"errors"
	"net/http"

	httphandler "github.com/LoResuelvo/loresuelvo-api/internal/adapters/http/handler"
	workorder "github.com/LoResuelvo/loresuelvo-api/internal/domain/work_order"
	"github.com/gin-gonic/gin"
)

func handleGetWorkOrderError(c *gin.Context, err error) {
	switch {
	case errors.Is(err, workorder.ErrDoesNotExist):
		httphandler.RespondError(c, http.StatusNotFound, err.Error())
	case errors.Is(err, workorder.ErrOnlyWorkOrderParticipantCanView):
		httphandler.RespondError(c, http.StatusForbidden, err.Error())
	default:
		httphandler.RespondError(c, http.StatusInternalServerError, "internal server error")
	}
}

func handleReportCompletionError(c *gin.Context, err error) {
	switch {
	case errors.Is(err, workorder.ErrDoesNotExist):
		httphandler.RespondError(c, http.StatusNotFound, err.Error())
	case errors.Is(err, workorder.ErrOnlyAssignedProviderCanReport):
		httphandler.RespondError(c, http.StatusForbidden, err.Error())
	case errors.Is(err, workorder.ErrCompletionReportDescriptionRequired),
		errors.Is(err, workorder.ErrCompletionReportImageCount),
		errors.Is(err, workorder.ErrCompletionReportImageRequired),
		errors.Is(err, workorder.ErrCompletionReportDuplicateImage),
		errors.Is(err, workorder.ErrWorkOrderCompletionImageNotAvailable):
		httphandler.RespondError(c, http.StatusBadRequest, err.Error())
	case errors.Is(err, workorder.ErrCompletionReportAlreadyExists),
		errors.Is(err, workorder.ErrWorkOrderNotReadyForCompletion):
		httphandler.RespondError(c, http.StatusConflict, err.Error())
	default:
		httphandler.RespondError(c, http.StatusInternalServerError, "internal server error")
	}
}
