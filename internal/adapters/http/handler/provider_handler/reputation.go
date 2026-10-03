package provider_handler

import (
	"context"
	"errors"
	"net/http"
	"net/url"
	"strconv"

	httphandler "github.com/LoResuelvo/loresuelvo-api/internal/adapters/http/handler"
	"github.com/LoResuelvo/loresuelvo-api/internal/adapters/http/signedcursor"
	"github.com/LoResuelvo/loresuelvo-api/internal/domain/provider"
	readmodel "github.com/LoResuelvo/loresuelvo-api/internal/domain/provider/read_model"
	"github.com/gin-gonic/gin"
)

const reputationCursorVersion = 1
const reputationCursorPurpose = "provider_reputation:v1"

type ReputationService interface {
	Query(ctx context.Context, authID string, input provider.ReputationQueryInput) (*readmodel.Reputation, int, error)
}

type ReputationHandler struct {
	service ReputationService
	cursors *signedcursor.Codec
}

func NewReputationHandler(service ReputationService, key []byte) (*ReputationHandler, error) {
	codec, err := signedcursor.New(key, reputationCursorPurpose)
	if err != nil {
		return nil, err
	}
	return &ReputationHandler{service: service, cursors: codec}, nil
}

func (h *ReputationHandler) Get(c *gin.Context) {
	authID, ok := httphandler.GetAuthenticatedUserID(c)
	if !ok {
		return
	}
	input, err := h.parseQuery(c.Request.URL.RawQuery)
	if err != nil {
		respondReputationError(c, err)
		return
	}
	result, providerID, err := h.service.Query(c.Request.Context(), authID, input)
	if err != nil {
		respondReputationError(c, err)
		return
	}
	if result == nil {
		respondReputationError(c, errors.New("reputation service returned no result"))
		return
	}
	response := reputationResponseFromDomain(result)
	if result.Next != nil {
		cursor, err := h.cursors.Encode(reputationCursorPayload{Version: reputationCursorVersion, ProviderID: providerID, Limit: input.Limit, After: *result.Next})
		if err != nil {
			respondReputationError(c, err)
			return
		}
		response.NextCursor = &cursor
	}
	c.JSON(http.StatusOK, response)
}

func respondReputationError(c *gin.Context, err error) {
	switch {
	case errors.Is(err, provider.ErrInvalidReputationQuery):
		httphandler.RespondError(c, http.StatusBadRequest, "invalid reputation query")
	case errors.Is(err, provider.ErrReputationForbidden):
		httphandler.RespondError(c, http.StatusForbidden, "provider access required")
	case errors.Is(err, provider.ErrReputationProviderNotFound):
		httphandler.RespondError(c, http.StatusNotFound, "provider not found")
	default:
		httphandler.RespondError(c, http.StatusInternalServerError, "internal server error")
	}
}

type reputationCursorPayload struct {
	Version    int                          `json:"v"`
	ProviderID int                          `json:"provider_id"`
	Limit      int                          `json:"limit"`
	After      readmodel.ReputationPosition `json:"after"`
}

func (h *ReputationHandler) parseQuery(raw string) (provider.ReputationQueryInput, error) {
	input := provider.ReputationQueryInput{}
	values, err := url.ParseQuery(raw)
	if err != nil {
		return input, provider.ErrInvalidReputationQuery
	}
	for key, entries := range values {
		if len(entries) != 1 || entries[0] == "" {
			return input, provider.ErrInvalidReputationQuery
		}
		switch key {
		case "limit", "cursor":
		default:
			return input, provider.ErrInvalidReputationQuery
		}
	}
	if rawLimit := values.Get("limit"); rawLimit != "" {
		for _, digit := range rawLimit {
			if digit < '0' || digit > '9' {
				return input, provider.ErrInvalidReputationQuery
			}
		}
		input.Limit, err = strconv.Atoi(rawLimit)
		if err != nil || input.Limit < 1 || input.Limit > provider.MaxReputationPageSize {
			return input, provider.ErrInvalidReputationQuery
		}
	}
	if token := values.Get("cursor"); token != "" {
		var payload reputationCursorPayload
		if err := h.cursors.Decode(token, &payload); err != nil || payload.Version != reputationCursorVersion || payload.ProviderID <= 0 || payload.Limit < 1 || payload.Limit > provider.MaxReputationPageSize || payload.After.WorkOrderID <= 0 {
			return input, provider.ErrInvalidReputationQuery
		}
		if input.Limit != 0 && input.Limit != payload.Limit {
			return input, provider.ErrInvalidReputationQuery
		}
		input.Limit = payload.Limit
		input.After = &payload.After
		input.CursorProviderID = payload.ProviderID
	}
	if input.Limit == 0 {
		input.Limit = provider.DefaultReputationPageSize
	}
	return input, nil
}
