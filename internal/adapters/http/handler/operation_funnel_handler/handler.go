package operation_funnel_handler

import (
	"context"
	"errors"
	"net/http"

	httphandler "github.com/LoResuelvo/loresuelvo-api/internal/adapters/http/handler"
	"github.com/LoResuelvo/loresuelvo-api/internal/domain/operation"
	readmodel "github.com/LoResuelvo/loresuelvo-api/internal/domain/operation/read_model"
	"github.com/gin-gonic/gin"
)

type queryService interface {
	Query(context.Context, operation.FunnelQuery) (readmodel.FunnelMetrics, error)
}

type Handler struct {
	service queryService
}

func NewHandler(service queryService) *Handler {
	return &Handler{service: service}
}

func (handler *Handler) Get(c *gin.Context) {
	query, err := parseFunnelQuery(c.Request.URL.RawQuery)
	if err != nil {
		httphandler.RespondError(c, http.StatusBadRequest, "invalid funnel query")
		return
	}
	metrics, err := handler.service.Query(c.Request.Context(), query)
	if errors.Is(err, operation.ErrInvalidFunnelQuery) {
		httphandler.RespondError(c, http.StatusBadRequest, "invalid funnel query")
		return
	}
	if err != nil {
		httphandler.RespondError(c, http.StatusInternalServerError, "internal server error")
		return
	}
	c.JSON(http.StatusOK, responseFromMetrics(metrics))
}
