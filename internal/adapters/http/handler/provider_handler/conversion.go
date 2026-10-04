package provider_handler

import (
	"context"
	"errors"
	"net/http"
	"net/url"
	"regexp"
	"time"

	httphandler "github.com/LoResuelvo/loresuelvo-api/internal/adapters/http/handler"
	"github.com/LoResuelvo/loresuelvo-api/internal/domain/provider"
	readmodel "github.com/LoResuelvo/loresuelvo-api/internal/domain/provider/read_model"
	"github.com/gin-gonic/gin"
)

// Go's RFC3339 parser also accepts non-RFC syntax such as comma fractions,
// one-digit hours and out-of-range offsets. Guard the wire syntax first.
// Fractions beyond nanoseconds would be silently truncated before range binding.
var conversionTimestampSyntax = regexp.MustCompile(`^[0-9]{4}-[0-9]{2}-[0-9]{2}T([01][0-9]|2[0-3]):[0-5][0-9]:[0-5][0-9](\.[0-9]{1,9})?(Z|[+-]([01][0-9]|2[0-3]):[0-5][0-9])$`)

type ConversionService interface {
	Query(ctx context.Context, authID string, input provider.ConversionQueryInput) (*readmodel.Conversion, error)
}

type ConversionHandler struct{ service ConversionService }

func NewConversionHandler(service ConversionService) *ConversionHandler {
	return &ConversionHandler{service: service}
}

func (h *ConversionHandler) Get(c *gin.Context) {
	authID, ok := httphandler.GetAuthenticatedUserID(c)
	if !ok {
		return
	}
	values, err := url.ParseQuery(c.Request.URL.RawQuery)
	if err != nil {
		httphandler.RespondError(c, http.StatusBadRequest, "invalid conversion query")
		return
	}
	input, err := parseConversionQuery(values)
	if err != nil {
		httphandler.RespondError(c, http.StatusBadRequest, "invalid conversion query")
		return
	}
	if h.service == nil {
		httphandler.RespondError(c, http.StatusInternalServerError, "internal server error")
		return
	}
	conversion, err := h.service.Query(c.Request.Context(), authID, input)
	if err != nil {
		switch {
		case errors.Is(err, provider.ErrInvalidConversionQuery):
			httphandler.RespondError(c, http.StatusBadRequest, "invalid conversion query")
		case errors.Is(err, provider.ErrConversionForbidden):
			httphandler.RespondError(c, http.StatusForbidden, "provider access required")
		case errors.Is(err, provider.ErrConversionProviderNotFound):
			httphandler.RespondError(c, http.StatusNotFound, "provider not found")
		default:
			httphandler.RespondError(c, http.StatusInternalServerError, "internal server error")
		}
		return
	}
	if conversion == nil {
		httphandler.RespondError(c, http.StatusInternalServerError, "internal server error")
		return
	}
	c.JSON(http.StatusOK, conversionResponseFromDomain(conversion))
}

func parseConversionQuery(values url.Values) (provider.ConversionQueryInput, error) {
	input := provider.ConversionQueryInput{}
	for key, entries := range values {
		if len(entries) != 1 || entries[0] == "" {
			return input, provider.ErrInvalidConversionQuery
		}
		switch key {
		case "from", "to":
			if !conversionTimestampSyntax.MatchString(entries[0]) {
				return input, provider.ErrInvalidConversionQuery
			}
			value, err := time.Parse(time.RFC3339Nano, entries[0])
			if err != nil {
				return input, provider.ErrInvalidConversionQuery
			}
			if key == "from" {
				input.From = &value
			} else {
				input.To = &value
			}
		default:
			return input, provider.ErrInvalidConversionQuery
		}
	}
	return input, nil
}
