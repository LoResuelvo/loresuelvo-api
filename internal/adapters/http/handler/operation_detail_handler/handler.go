package operation_detail_handler

import (
	"context"
	"errors"
	"net/http"

	httphandler "github.com/LoResuelvo/loresuelvo-api/internal/adapters/http/handler"
	"github.com/LoResuelvo/loresuelvo-api/internal/adapters/http/middleware"
	"github.com/LoResuelvo/loresuelvo-api/internal/domain/operation"
	readmodel "github.com/LoResuelvo/loresuelvo-api/internal/domain/operation/read_model"
	"github.com/gin-gonic/gin"
)

type service interface {
	Query(ctx context.Context, id, authSubject, correlationID string) (*readmodel.OperationDetail, error)
}
type Handler struct{ service service }

func NewHandler(service service) *Handler { return &Handler{service: service} }

func (handler *Handler) Get(c *gin.Context) {
	rawID := c.Param("operation_id")
	authSubject, ok := httphandler.GetAuthenticatedUserID(c)
	if !ok {
		return
	}
	correlationID, ok := middleware.GetRequestID(c)
	if !ok {
		httphandler.RespondError(c, http.StatusInternalServerError, "internal server error")
		return
	}
	found, err := handler.service.Query(c.Request.Context(), rawID, authSubject, correlationID)
	if errors.Is(err, operation.ErrOperationNotFound) {
		httphandler.RespondError(c, http.StatusNotFound, "operation not found")
		return
	}
	if errors.Is(err, operation.ErrInvalidOperationID) {
		httphandler.RespondError(c, http.StatusBadRequest, "invalid operation ID")
		return
	}
	if err != nil {
		httphandler.RespondError(c, http.StatusInternalServerError, "internal server error")
		return
	}
	c.JSON(http.StatusOK, responseFromDomain(found))
}
