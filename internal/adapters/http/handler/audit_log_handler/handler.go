package audit_log_handler

import (
	"context"
	"net/http"
	"net/url"
	"strconv"
	"time"

	httphandler "github.com/LoResuelvo/loresuelvo-api/internal/adapters/http/handler"
	"github.com/LoResuelvo/loresuelvo-api/internal/adapters/http/middleware"
	"github.com/LoResuelvo/loresuelvo-api/internal/domain/audit"
	"github.com/gin-gonic/gin"
)

type service interface {
	Query(ctx context.Context, authSubject, correlationID string, filter audit.LogFilter) ([]*audit.Event, error)
}

type Handler struct {
	service service
}

func NewHandler(service service) *Handler {
	return &Handler{service: service}
}

func (handler *Handler) List(c *gin.Context) {
	filter, valid := parseLogFilter(c.Request.URL.Query())
	if !valid {
		httphandler.RespondError(c, http.StatusBadRequest, "invalid filter")
		return
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

	events, err := handler.service.Query(c.Request.Context(), authSubject, correlationID, filter)
	if err != nil {
		httphandler.RespondError(c, http.StatusInternalServerError, "internal server error")
		return
	}

	c.JSON(http.StatusOK, pageResponse{Events: eventResponsesFromDomain(events)})
}

func parseLogFilter(query url.Values) (audit.LogFilter, bool) {
	var filter audit.LogFilter
	for key, values := range query {
		if len(values) != 1 || values[0] == "" {
			return audit.LogFilter{}, false
		}
		value := values[0]
		switch key {
		case "operator_id":
			parsed, err := strconv.ParseInt(value, 10, 32)
			if err != nil || parsed <= 0 {
				return audit.LogFilter{}, false
			}
			operatorID := int(parsed)
			filter.OperatorID = &operatorID
		case "action":
			action := audit.Action(value)
			filter.Action = &action
		case "resource_type":
			filter.ResourceType = &value
		case "resource_id":
			filter.ResourceID = &value
		case "result":
			result := audit.Result(value)
			filter.Result = &result
		case "occurred_from", "occurred_to":
			instant, err := time.Parse(time.RFC3339Nano, value)
			if err != nil {
				return audit.LogFilter{}, false
			}
			instant = instant.UTC()
			if key == "occurred_from" {
				filter.OccurredFrom = &instant
			} else {
				filter.OccurredTo = &instant
			}
		default:
			return audit.LogFilter{}, false
		}
	}
	return filter, filter.Validate() == nil
}
