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
	Query(ctx context.Context, authSubject, correlationID string, query audit.LogQuery) (audit.LogPage, error)
}

type Handler struct {
	service service
	cursors *cursorCodec
}

func NewHandler(service service, signingKey []byte) (*Handler, error) {
	cursors, err := newCursorCodec(signingKey)
	if err != nil {
		return nil, err
	}
	return &Handler{service: service, cursors: cursors}, nil
}

func (handler *Handler) List(c *gin.Context) {
	query, valid := handler.parseLogQuery(c.Request.URL.Query())
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

	page, err := handler.service.Query(c.Request.Context(), authSubject, correlationID, query)
	if err != nil {
		httphandler.RespondError(c, http.StatusInternalServerError, "internal server error")
		return
	}
	response := pageResponse{Events: eventResponsesFromDomain(page.Events)}
	if page.Next != nil {
		cursor, err := handler.cursors.encode(cursorPayload{
			Watermark: page.Watermark, Before: *page.Next, Filter: query.Filter, Limit: query.Limit,
		})
		if err != nil {
			httphandler.RespondError(c, http.StatusInternalServerError, "internal server error")
			return
		}
		response.NextCursor = &cursor
	}
	c.JSON(http.StatusOK, response)
}

func parseLogFilter(query url.Values) (audit.LogFilter, bool) {
	var filter audit.LogFilter
	for key, values := range query {
		if key == "limit" || key == "cursor" {
			continue
		}
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

func (handler *Handler) parseLogQuery(values url.Values) (audit.LogQuery, bool) {
	explicitFilter, valid := parseLogFilter(values)
	if !valid {
		return audit.LogQuery{}, false
	}

	limit := 20
	if raw, present := values["limit"]; present {
		if len(raw) != 1 {
			return audit.LogQuery{}, false
		}
		parsed, err := strconv.Atoi(raw[0])
		if err != nil || parsed < 1 || parsed > 100 {
			return audit.LogQuery{}, false
		}
		limit = parsed
	}

	query := audit.LogQuery{Filter: explicitFilter, Limit: limit}
	if raw, present := values["cursor"]; present {
		if len(raw) != 1 {
			return audit.LogQuery{}, false
		}
		payload, err := handler.cursors.decode(raw[0])
		if err != nil || !filtersCompatible(explicitFilter, payload.Filter) {
			return audit.LogQuery{}, false
		}
		if _, present := values["limit"]; present && limit != payload.Limit {
			return audit.LogQuery{}, false
		}
		query.Filter = payload.Filter
		query.Limit = payload.Limit
		query.Watermark = &payload.Watermark
		query.Before = &payload.Before
	}
	return query, true
}

func filtersCompatible(explicit, original audit.LogFilter) bool {
	if explicit.OperatorID != nil && (original.OperatorID == nil || *explicit.OperatorID != *original.OperatorID) {
		return false
	}
	if explicit.Action != nil && (original.Action == nil || *explicit.Action != *original.Action) {
		return false
	}
	if explicit.ResourceType != nil && (original.ResourceType == nil || *explicit.ResourceType != *original.ResourceType) {
		return false
	}
	if explicit.ResourceID != nil && (original.ResourceID == nil || *explicit.ResourceID != *original.ResourceID) {
		return false
	}
	if explicit.Result != nil && (original.Result == nil || *explicit.Result != *original.Result) {
		return false
	}
	if explicit.OccurredFrom != nil && (original.OccurredFrom == nil || !explicit.OccurredFrom.Equal(*original.OccurredFrom)) {
		return false
	}
	if explicit.OccurredTo != nil && (original.OccurredTo == nil || !explicit.OccurredTo.Equal(*original.OccurredTo)) {
		return false
	}
	return true
}
