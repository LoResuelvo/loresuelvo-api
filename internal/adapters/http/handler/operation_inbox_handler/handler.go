package operation_inbox_handler

import (
	"context"
	"net/http"

	httphandler "github.com/LoResuelvo/loresuelvo-api/internal/adapters/http/handler"
	"github.com/LoResuelvo/loresuelvo-api/internal/domain/operation"
	"github.com/gin-gonic/gin"
)

type service interface {
	Query(ctx context.Context, query operation.InboxQuery) (operation.InboxPage, error)
}

type Handler struct {
	service service
}

func NewHandler(service service) *Handler {
	return &Handler{service: service}
}

func (handler *Handler) List(c *gin.Context) {
	if len(c.Request.URL.Query()) > 0 {
		httphandler.RespondError(c, http.StatusBadRequest, "invalid filter")
		return
	}

	page, err := handler.service.Query(c.Request.Context(), operation.InboxQuery{})
	if err != nil {
		httphandler.RespondError(c, http.StatusInternalServerError, "internal server error")
		return
	}
	c.JSON(http.StatusOK, pageResponse{Operations: operationResponsesFromDomain(page.Operations)})
}
