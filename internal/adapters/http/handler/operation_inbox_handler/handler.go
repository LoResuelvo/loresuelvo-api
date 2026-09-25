package operation_inbox_handler

import (
	"context"
	"errors"
	"net/http"
	"net/url"

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
	for key, raw := range values {
		if len(raw) != 1 || raw[0] == "" {
			return operation.InboxQuery{}, false
		}
		switch key {
		case "alert":
			alert := readmodel.Alert(raw[0])
			query.Filter.Alert = &alert
		default:
			return operation.InboxQuery{}, false
		}
	}
	return query, true
}
