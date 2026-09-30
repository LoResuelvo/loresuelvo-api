package provider_handler

import (
	"context"
	"errors"
	"net/http"
	"net/url"
	"strconv"
	"time"

	httphandler "github.com/LoResuelvo/loresuelvo-api/internal/adapters/http/handler"
	"github.com/LoResuelvo/loresuelvo-api/internal/adapters/http/signedcursor"
	"github.com/LoResuelvo/loresuelvo-api/internal/domain/provider"
	readmodel "github.com/LoResuelvo/loresuelvo-api/internal/domain/provider/read_model"
	"github.com/gin-gonic/gin"
)

const collectionCursorVersion = 1
const collectionCursorPurpose = "provider_collections:v1"

type CollectionService interface {
	Summary(ctx context.Context, authID string, input provider.ActivityQueryInput) (*readmodel.Collections, error)
	Detail(ctx context.Context, authID string, input provider.CollectionDetailInput) (*readmodel.CollectionDetail, int, error)
}

type CollectionHandler struct {
	service CollectionService
	cursors *signedcursor.Codec
}

func NewCollectionHandler(service CollectionService, key []byte) (*CollectionHandler, error) {
	codec, err := signedcursor.New(key, collectionCursorPurpose)
	if err != nil {
		return nil, err
	}
	return &CollectionHandler{service: service, cursors: codec}, nil
}

func (h *CollectionHandler) GetSummary(c *gin.Context) {
	authID, ok := httphandler.GetAuthenticatedUserID(c)
	if !ok {
		return
	}
	values, err := url.ParseQuery(c.Request.URL.RawQuery)
	if err != nil {
		respondCollectionError(c, provider.ErrInvalidCollectionQuery)
		return
	}
	input, err := parseActivityQuery(values)
	if err != nil {
		respondCollectionError(c, provider.ErrInvalidCollectionQuery)
		return
	}
	result, err := h.service.Summary(c.Request.Context(), authID, input)
	if err != nil {
		respondCollectionError(c, err)
		return
	}
	if result == nil {
		respondCollectionError(c, errors.New("collection service returned no summary"))
		return
	}
	c.JSON(http.StatusOK, collectionSummaryResponseFromDomain(result))
}

func (h *CollectionHandler) GetDetail(c *gin.Context) {
	authID, ok := httphandler.GetAuthenticatedUserID(c)
	if !ok {
		return
	}
	input, err := h.parseDetailQuery(c.Request.URL.RawQuery)
	if err != nil {
		respondCollectionError(c, err)
		return
	}
	result, providerID, err := h.service.Detail(c.Request.Context(), authID, input)
	if err != nil {
		respondCollectionError(c, err)
		return
	}
	if result == nil {
		respondCollectionError(c, errors.New("collection service returned no detail"))
		return
	}
	response := collectionDetailResponseFromDomain(result)
	if result.Next != nil {
		token, err := h.cursors.Encode(collectionCursorPayload{
			Version: collectionCursorVersion, ProviderID: providerID,
			From: result.Period.From, To: result.Period.To,
			Purpose: input.Purpose, Limit: input.Limit,
			After: *result.Next,
		})
		if err != nil {
			respondCollectionError(c, err)
			return
		}
		response.NextCursor = &token
	}
	c.JSON(http.StatusOK, response)
}

func respondCollectionError(c *gin.Context, err error) {
	switch {
	case errors.Is(err, provider.ErrInvalidCollectionQuery):
		httphandler.RespondError(c, http.StatusBadRequest, "invalid collection query")
	case errors.Is(err, provider.ErrActivityForbidden):
		httphandler.RespondError(c, http.StatusForbidden, "provider access required")
	case errors.Is(err, provider.ErrActivityProviderNotFound):
		httphandler.RespondError(c, http.StatusNotFound, "provider not found")
	default:
		httphandler.RespondError(c, http.StatusInternalServerError, "internal server error")
	}
}

type collectionCursorPayload struct {
	Version    int                          `json:"v"`
	ProviderID int                          `json:"provider_id"`
	From       time.Time                    `json:"from"`
	To         time.Time                    `json:"to"`
	Purpose    string                       `json:"purpose"`
	Limit      int                          `json:"limit"`
	After      readmodel.CollectionPosition `json:"after"`
}

func (h *CollectionHandler) parseDetailQuery(raw string) (provider.CollectionDetailInput, error) {
	values, err := url.ParseQuery(raw)
	if err != nil {
		return provider.CollectionDetailInput{}, provider.ErrInvalidCollectionQuery
	}
	for key, entries := range values {
		if len(entries) != 1 || entries[0] == "" {
			return provider.CollectionDetailInput{}, provider.ErrInvalidCollectionQuery
		}
		switch key {
		case "from", "to", "purpose", "limit", "cursor":
		default:
			return provider.CollectionDetailInput{}, provider.ErrInvalidCollectionQuery
		}
	}
	input := provider.CollectionDetailInput{}
	if rawLimit := values.Get("limit"); rawLimit != "" {
		for _, digit := range rawLimit {
			if digit < '0' || digit > '9' {
				return input, provider.ErrInvalidCollectionQuery
			}
		}
		input.Limit, err = strconv.Atoi(rawLimit)
		if err != nil || input.Limit < 1 || input.Limit > provider.MaxCollectionPageSize {
			return input, provider.ErrInvalidCollectionQuery
		}
	}
	input.Purpose = values.Get("purpose")
	if input.Purpose != "" && input.Purpose != "booking_deposit" && input.Purpose != "service_balance" {
		return input, provider.ErrInvalidCollectionQuery
	}
	if values.Has("from") != values.Has("to") {
		return input, provider.ErrInvalidCollectionQuery
	}
	if values.Has("from") {
		input.Period.From, err = parseCollectionInstant(values.Get("from"))
		if err != nil {
			return input, err
		}
		input.Period.To, err = parseCollectionInstant(values.Get("to"))
		if err != nil {
			return input, err
		}
	}
	if token := values.Get("cursor"); token != "" {
		var payload collectionCursorPayload
		if err := h.cursors.Decode(token, &payload); err != nil || payload.Version != collectionCursorVersion || payload.ProviderID <= 0 || payload.From.IsZero() || payload.To.IsZero() || !payload.From.Before(payload.To) || payload.Limit < 1 || payload.Limit > provider.MaxCollectionPageSize || payload.After.ID <= 0 || payload.After.VerifiedOn.IsZero() || payload.After.VerifiedOn.Before(payload.From) || !payload.After.VerifiedOn.Before(payload.To) || (payload.Purpose != "" && payload.Purpose != "booking_deposit" && payload.Purpose != "service_balance") {
			return input, provider.ErrInvalidCollectionQuery
		}
		if input.Limit != 0 && input.Limit != payload.Limit || values.Has("purpose") && input.Purpose != payload.Purpose || values.Has("from") && (!input.Period.From.Equal(payload.From) || !input.Period.To.Equal(payload.To)) {
			return input, provider.ErrInvalidCollectionQuery
		}
		input.Period = provider.ActivityQueryInput{From: &payload.From, To: &payload.To}
		input.Purpose = payload.Purpose
		input.Limit = payload.Limit
		input.After = &payload.After
		input.CursorProviderID = payload.ProviderID
	}
	if input.Limit == 0 {
		input.Limit = provider.DefaultCollectionPageSize
	}
	return input, nil
}

func parseCollectionInstant(raw string) (*time.Time, error) {
	instant, err := time.Parse(time.RFC3339Nano, raw)
	if err != nil {
		return nil, provider.ErrInvalidCollectionQuery
	}
	return &instant, nil
}
