package httpadapter

import (
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"net/url"
	"testing"
	"time"

	"github.com/LoResuelvo/loresuelvo-api/internal/adapters/auth0"
	"github.com/LoResuelvo/loresuelvo-api/internal/adapters/http/handler/provider_handler"
	"github.com/LoResuelvo/loresuelvo-api/internal/domain/provider"
	readmodel "github.com/LoResuelvo/loresuelvo-api/internal/domain/provider/read_model"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

var collectionTestCursorKey = []byte("test-collection-cursor-signing-key-2026-private")

func collectionRoute(t *testing.T, service *collectionServiceMock) *gin.Engine {
	t.Helper()
	gin.SetMode(gin.TestMode)
	handler, err := provider_handler.NewCollectionHandler(service, collectionTestCursorKey)
	require.NoError(t, err)
	router := NewRouter(RouterConfig{Auth0Validator: auth0.NewFakeValidator(), CollectionHandler: handler})
	authentication, err := router.middlewareSetup()
	require.NoError(t, err)
	engine := gin.New()
	router.registerProviderRoutes(engine, authentication)
	return engine
}

func collectionRequest(engine *gin.Engine, path, token string) *httptest.ResponseRecorder {
	request := httptest.NewRequest(http.MethodGet, path, nil)
	if token != "" {
		request.Header.Set("Authorization", "Bearer "+token)
	}
	response := httptest.NewRecorder()
	engine.ServeHTTP(response, request)
	return response
}

func TestProviderCollectionRoutesRequireAuthenticationAndPrivateCaching(t *testing.T) {
	engine := collectionRoute(t, &collectionServiceMock{})
	for _, path := range []string{"/providers/me/statistics/collections", "/providers/me/statistics/collections/transactions"} {
		for _, token := range []string{"", "invalid"} {
			response := collectionRequest(engine, path, token)
			require.Equal(t, http.StatusUnauthorized, response.Code)
			require.Equal(t, "private, no-store", response.Header().Get("Cache-Control"))
		}
	}
}

func TestProviderCollectionRoutesRejectMalformedQueries(t *testing.T) {
	service := &collectionServiceMock{}
	engine := collectionRoute(t, service)
	token := auth0.NewTokenBuilder().BuildToken("auth0|provider", nil)
	for _, path := range []string{
		"/providers/me/statistics/collections?provider_id=3",
		"/providers/me/statistics/collections?from=%GG",
		"/providers/me/statistics/collections?granularity=",
		"/providers/me/statistics/collections?compare_previous=true&compare_previous=false",
		"/providers/me/statistics/collections/transactions?provider_id=3",
		"/providers/me/statistics/collections/transactions?purpose=other",
		"/providers/me/statistics/collections/transactions?limit=101",
		"/providers/me/statistics/collections/transactions?limit=+2",
		"/providers/me/statistics/collections/transactions?limit=1&limit=1",
		"/providers/me/statistics/collections/transactions?from=2026-09-01T00%3A00%3A00Z",
		"/providers/me/statistics/collections/transactions?cursor=forged",
		"/providers/me/statistics/collections/transactions?granularity=day",
	} {
		response := collectionRequest(engine, path, token)
		require.Equal(t, http.StatusBadRequest, response.Code, path)
		require.Equal(t, "private, no-store", response.Header().Get("Cache-Control"), path)
		require.JSONEq(t, `{"error":"invalid collection query"}`, response.Body.String(), path)
	}
	service.AssertNotCalled(t, "Summary", mock.Anything, mock.Anything, mock.Anything)
	service.AssertNotCalled(t, "Detail", mock.Anything, mock.Anything, mock.Anything)
}

func TestProviderCollectionSummaryRouteMapsPrivateResponse(t *testing.T) {
	service := &collectionServiceMock{}
	from := time.Date(2026, 9, 1, 3, 0, 0, 0, time.UTC)
	to := from.Add(24 * time.Hour)
	service.On("Summary", mock.Anything, "auth0|provider", mock.Anything).Return(&readmodel.Collections{
		Period: readmodel.ActivityPeriod{From: from, To: to}, Granularity: "day", TimeZone: "America/Argentina/Buenos_Aires", CalculatedAt: to,
		Results: readmodel.CollectionAmounts{BookingDepositCents: 20000, ServiceBalanceCents: 80000, TotalCents: 100000},
		Series:  []readmodel.CollectionBucket{{From: from, To: to, CollectionAmounts: readmodel.CollectionAmounts{BookingDepositCents: 20000, ServiceBalanceCents: 80000, TotalCents: 100000}}},
	}, nil).Once()
	engine := collectionRoute(t, service)
	token := auth0.NewTokenBuilder().BuildToken("auth0|provider", nil)
	response := collectionRequest(engine, "/providers/me/statistics/collections?from=2026-09-01T03%3A00%3A00Z&to=2026-09-02T03%3A00%3A00Z", token)
	require.Equal(t, http.StatusOK, response.Code)
	require.Equal(t, "private, no-store", response.Header().Get("Cache-Control"))
	var body map[string]any
	require.NoError(t, json.Unmarshal(response.Body.Bytes(), &body))
	require.Equal(t, "ARS", body["currency"])
	require.Equal(t, float64(100000), body["results"].(map[string]any)["total_cents"])
	require.Equal(t, float64(100000), body["evolution"].([]any)[0].(map[string]any)["total_cents"])
	require.NotContains(t, response.Body.String(), "seller_account_id")
	require.NotContains(t, response.Body.String(), "external_payment_id")
	service.AssertExpectations(t)
}

func TestProviderCollectionDetailCursorCarriesFilterPeriodLimitAndOwner(t *testing.T) {
	service := &collectionServiceMock{}
	from := time.Date(2026, 9, 1, 3, 0, 0, 0, time.UTC)
	to := time.Date(2026, 9, 29, 3, 0, 0, 0, time.UTC)
	firstPosition := readmodel.CollectionPosition{VerifiedOn: from.Add(24 * time.Hour), ID: 9}
	service.On("Detail", mock.Anything, "auth0|provider", mock.MatchedBy(func(input provider.CollectionDetailInput) bool {
		return input.Limit == 1 && input.Purpose == "booking_deposit" && input.Period.From.Equal(from) && input.Period.To.Equal(to) && input.After == nil
	})).Return(&readmodel.CollectionDetail{
		Period: readmodel.ActivityPeriod{From: from, To: to}, TimeZone: "America/Argentina/Buenos_Aires", CalculatedAt: to,
		TotalCount: 2, TotalAmountCents: 20000,
		Transactions: []readmodel.CollectionTransaction{{ID: 9, VerifiedOn: firstPosition.VerifiedOn, Purpose: "booking_deposit", SellerAmountCents: 10000, Currency: "ARS", ServiceProposalID: 4}},
		Next:         &firstPosition,
	}, 7, nil).Once()
	service.On("Detail", mock.Anything, "auth0|provider", mock.MatchedBy(func(input provider.CollectionDetailInput) bool {
		return input.Limit == 1 && input.Purpose == "booking_deposit" && input.Period.From.Equal(from) && input.Period.To.Equal(to) && input.CursorProviderID == 7 && input.After != nil && input.After.ID == 9
	})).Return(&readmodel.CollectionDetail{
		Period: readmodel.ActivityPeriod{From: from, To: to}, TimeZone: "America/Argentina/Buenos_Aires", CalculatedAt: to.Add(time.Hour),
		TotalCount: 2, TotalAmountCents: 20000,
		Transactions: []readmodel.CollectionTransaction{{ID: 8, VerifiedOn: from.Add(time.Hour), Purpose: "booking_deposit", SellerAmountCents: 10000, Currency: "ARS", ServiceProposalID: 5}},
	}, 7, nil).Once()
	engine := collectionRoute(t, service)
	token := auth0.NewTokenBuilder().BuildToken("auth0|provider", nil)
	first := collectionRequest(engine, "/providers/me/statistics/collections/transactions?from=2026-09-01T03%3A00%3A00Z&to=2026-09-29T03%3A00%3A00Z&purpose=booking_deposit&limit=1", token)
	require.Equal(t, http.StatusOK, first.Code)
	var body struct {
		NextCursor *string `json:"next_cursor"`
	}
	require.NoError(t, json.Unmarshal(first.Body.Bytes(), &body))
	require.NotNil(t, body.NextCursor)
	second := collectionRequest(engine, "/providers/me/statistics/collections/transactions?cursor="+url.QueryEscape(*body.NextCursor), token)
	require.Equal(t, http.StatusOK, second.Code)
	require.Contains(t, second.Body.String(), `"total_count":2`)
	require.Contains(t, second.Body.String(), `"total_amount_cents":20000`)
	require.Contains(t, second.Body.String(), `"next_cursor":null`)
	for _, incompatible := range []string{
		"&limit=2", "&purpose=service_balance", "&from=2026-09-01T03%3A00%3A00Z", "&from=2026-09-02T03%3A00%3A00Z&to=2026-09-29T03%3A00%3A00Z",
	} {
		response := collectionRequest(engine, "/providers/me/statistics/collections/transactions?cursor="+url.QueryEscape(*body.NextCursor)+incompatible, token)
		require.Equal(t, http.StatusBadRequest, response.Code, incompatible)
	}
	service.AssertExpectations(t)
}

func TestProviderCollectionRoutesMapAuthorizationAndReadErrors(t *testing.T) {
	for _, test := range []struct {
		name    string
		path    string
		failure error
		status  int
	}{
		{"summary forbidden", "/providers/me/statistics/collections", provider.ErrActivityForbidden, 403},
		{"detail forbidden", "/providers/me/statistics/collections/transactions", provider.ErrActivityForbidden, 403},
		{"summary missing", "/providers/me/statistics/collections", provider.ErrActivityProviderNotFound, 404},
		{"detail missing", "/providers/me/statistics/collections/transactions", provider.ErrActivityProviderNotFound, 404},
		{"summary read failure", "/providers/me/statistics/collections", errors.New("database failure"), 500},
		{"detail inconsistent", "/providers/me/statistics/collections/transactions", errors.New("inconsistent amount"), 500},
	} {
		t.Run(test.name, func(t *testing.T) {
			service := &collectionServiceMock{}
			if test.path == "/providers/me/statistics/collections" {
				service.On("Summary", mock.Anything, "auth0|provider", mock.Anything).Return(nil, test.failure).Once()
			} else {
				service.On("Detail", mock.Anything, "auth0|provider", mock.Anything).Return(nil, 0, test.failure).Once()
			}
			engine := collectionRoute(t, service)
			token := auth0.NewTokenBuilder().BuildToken("auth0|provider", nil)
			response := collectionRequest(engine, test.path, token)
			require.Equal(t, test.status, response.Code)
			require.Equal(t, "private, no-store", response.Header().Get("Cache-Control"))
			require.NotContains(t, response.Body.String(), "database failure")
			require.NotContains(t, response.Body.String(), "inconsistent amount")
			service.AssertExpectations(t)
		})
	}
}
