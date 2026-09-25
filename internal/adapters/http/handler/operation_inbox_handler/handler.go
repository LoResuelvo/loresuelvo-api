package operation_inbox_handler

import (
	"context"
	"errors"
	"net/http"
	"net/url"
	"strconv"
	"time"

	httphandler "github.com/LoResuelvo/loresuelvo-api/internal/adapters/http/handler"
	"github.com/LoResuelvo/loresuelvo-api/internal/domain/operation"
	readmodel "github.com/LoResuelvo/loresuelvo-api/internal/domain/operation/read_model"
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
	query, valid := parseInboxQuery(c.Request.URL.Query())
	if !valid {
		httphandler.RespondError(c, http.StatusBadRequest, "invalid filter")
		return
	}

	page, err := handler.service.Query(c.Request.Context(), query)
	if errors.Is(err, operation.ErrInvalidInboxQuery) {
		httphandler.RespondError(c, http.StatusBadRequest, "invalid filter")
		return
	}
	if err != nil {
		httphandler.RespondError(c, http.StatusInternalServerError, "internal server error")
		return
	}
	c.JSON(http.StatusOK, pageResponse{Operations: operationResponsesFromDomain(page.Operations)})
}

func parseInboxQuery(values url.Values) (operation.InboxQuery, bool) {
	var query operation.InboxQuery
	filter := &query.Filter
	for key, raw := range values {
		if len(raw) != 1 || raw[0] == "" {
			return operation.InboxQuery{}, false
		}
		value, valid := raw[0], true
		switch key {
		case "consumer_id":
			filter.ConsumerID, valid = parseID(value)
		case "provider_id":
			filter.ProviderID, valid = parseID(value)
		case "category_id":
			filter.CategoryID, valid = parseID(value)
		case "started_from", "started_to":
			instant, err := time.Parse(time.RFC3339Nano, value)
			if err != nil {
				return operation.InboxQuery{}, false
			}
			instant = instant.UTC()
			if key == "started_from" {
				filter.StartedFrom = &instant
			} else {
				filter.StartedTo = &instant
			}
		case "stage":
			stage := readmodel.Stage(value)
			filter.Stage = &stage
		case "alert":
			alert := readmodel.Alert(value)
			filter.Alert = &alert
		case "scheduled_date":
			date, err := time.Parse(time.DateOnly, value)
			if err != nil {
				return operation.InboxQuery{}, false
			}
			filter.ScheduledDay = &operation.CalendarDay{Year: date.Year(), Month: date.Month(), Day: date.Day()}
		default:
			return operation.InboxQuery{}, false
		}
		if !valid {
			return operation.InboxQuery{}, false
		}
	}
	return query, true
}

func parseID(value string) (*int, bool) {
	parsed, err := strconv.ParseInt(value, 10, 32)
	if err != nil {
		return nil, false
	}
	id := int(parsed)
	return &id, true
}
