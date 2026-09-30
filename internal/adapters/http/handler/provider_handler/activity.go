package provider_handler

import (
	"context"
	"errors"
	"net/http"
	"net/url"
	"time"

	httphandler "github.com/LoResuelvo/loresuelvo-api/internal/adapters/http/handler"
	"github.com/LoResuelvo/loresuelvo-api/internal/domain/provider"
	readmodel "github.com/LoResuelvo/loresuelvo-api/internal/domain/provider/read_model"
	"github.com/gin-gonic/gin"
)

type ActivityService interface {
	Query(ctx context.Context, authID string, input provider.ActivityQueryInput) (*readmodel.Activity, error)
}

type ActivityHandler struct{ service ActivityService }

func NewActivityHandler(service ActivityService) *ActivityHandler {
	return &ActivityHandler{service: service}
}

func (h *ActivityHandler) Get(c *gin.Context) {
	authID, ok := httphandler.GetAuthenticatedUserID(c)
	if !ok {
		return
	}
	values, err := url.ParseQuery(c.Request.URL.RawQuery)
	if err != nil {
		httphandler.RespondError(c, http.StatusBadRequest, "invalid activity query")
		return
	}
	input, err := parseActivityQuery(values)
	if err != nil {
		httphandler.RespondError(c, http.StatusBadRequest, "invalid activity query")
		return
	}
	activity, err := h.service.Query(c.Request.Context(), authID, input)
	if err != nil {
		switch {
		case errors.Is(err, provider.ErrInvalidActivityQuery):
			httphandler.RespondError(c, http.StatusBadRequest, "invalid activity query")
		case errors.Is(err, provider.ErrActivityForbidden):
			httphandler.RespondError(c, http.StatusForbidden, "provider access required")
		case errors.Is(err, provider.ErrActivityProviderNotFound):
			httphandler.RespondError(c, http.StatusNotFound, "provider not found")
		default:
			httphandler.RespondError(c, http.StatusInternalServerError, "internal server error")
		}
		return
	}
	if activity == nil {
		httphandler.RespondError(c, http.StatusInternalServerError, "internal server error")
		return
	}
	c.JSON(http.StatusOK, activityResponseFromDomain(activity))
}

func parseActivityQuery(values url.Values) (provider.ActivityQueryInput, error) {
	input := provider.ActivityQueryInput{}
	for key, entries := range values {
		if len(entries) != 1 || entries[0] == "" {
			return input, provider.ErrInvalidActivityQuery
		}
		switch key {
		case "from", "to":
			value, err := time.Parse(time.RFC3339Nano, entries[0])
			if err != nil {
				return input, provider.ErrInvalidActivityQuery
			}
			// Explicit offset or Z is required by RFC3339Nano. Preserve the instant.
			if key == "from" {
				input.From = &value
			} else {
				input.To = &value
			}
		case "granularity":
			input.Granularity = entries[0]
		case "compare_previous":
			if entries[0] != "true" && entries[0] != "false" {
				return input, provider.ErrInvalidActivityQuery
			}
			input.ComparePrevious = entries[0] == "true"
		default:
			return input, provider.ErrInvalidActivityQuery
		}
	}
	return input, nil
}
