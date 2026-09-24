package audit_log_handler

import (
	"context"
	"net/http"
	"strconv"

	httphandler "github.com/LoResuelvo/loresuelvo-api/internal/adapters/http/handler"
	"github.com/LoResuelvo/loresuelvo-api/internal/adapters/http/middleware"
	"github.com/LoResuelvo/loresuelvo-api/internal/domain/audit"
	"github.com/gin-gonic/gin"
)

type service interface {
	Query(ctx context.Context, authSubject, correlationID string, operatorID *int) ([]*audit.Event, error)
}

type Handler struct {
	service service
}

func NewHandler(service service) *Handler {
	return &Handler{service: service}
}

func (handler *Handler) List(c *gin.Context) {
	query := c.Request.URL.Query()
	for key := range query {
		if key != "operator_id" {
			httphandler.RespondError(c, http.StatusBadRequest, "invalid filter")
			return
		}
	}

	var operatorID *int
	if values, present := query["operator_id"]; present {
		if len(values) != 1 {
			httphandler.RespondError(c, http.StatusBadRequest, "invalid filter")
			return
		}
		parsed, err := strconv.ParseInt(values[0], 10, 32)
		value := int(parsed)
		if err != nil || value <= 0 {
			httphandler.RespondError(c, http.StatusBadRequest, "invalid filter")
			return
		}
		operatorID = &value
	}

	authSubject, ok := httphandler.GetAuthenticatedUserID(c)
	if !ok {
		return
	}
	correlationID, ok := middleware.GetRequestID(c)
	if !ok {
		httphandler.RespondError(c, http.StatusInternalServerError, "internal server error")
		return
	}

	events, err := handler.service.Query(c.Request.Context(), authSubject, correlationID, operatorID)
	if err != nil {
		httphandler.RespondError(c, http.StatusInternalServerError, "internal server error")
		return
	}

	c.JSON(http.StatusOK, pageResponse{Events: eventResponsesFromDomain(events)})
}
