package httpadapter

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
	"time"

	"github.com/LoResuelvo/loresuelvo-api/internal/adapters/auth0"
	"github.com/LoResuelvo/loresuelvo-api/internal/adapters/http/handler/provider_handler"
	"github.com/LoResuelvo/loresuelvo-api/internal/adapters/http/signedcursor"
	"github.com/LoResuelvo/loresuelvo-api/internal/domain/provider"
	readmodel "github.com/LoResuelvo/loresuelvo-api/internal/domain/provider/read_model"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

var reputationTestCursorKey = []byte("test-reputation-cursor-signing-key-2026-private")

const reputationPath = "/providers/me/statistics/reputation"

func reputationRoute(t *testing.T, service *reputationServiceMock) *gin.Engine {
	t.Helper()
	gin.SetMode(gin.TestMode)
	handler, err := provider_handler.NewReputationHandler(service, reputationTestCursorKey)
	require.NoError(t, err)
	router := NewRouter(RouterConfig{Auth0Validator: auth0.NewFakeValidator(), ReputationHandler: handler})
	authentication, err := router.middlewareSetup()
	require.NoError(t, err)
	engine := gin.New()
	router.registerProviderRoutes(engine, authentication)
	return engine
}

func TestProviderReputationRouteRequiresAuthenticationBeforeValidation(t *testing.T) {
	service := &reputationServiceMock{}
	engine := reputationRoute(t, service)
	for _, token := range []string{"", "invalid"} {
		response := collectionRequest(engine, reputationPath+"?provider_id=9", token)
		require.Equal(t, http.StatusUnauthorized, response.Code)
		require.Equal(t, "private, no-store", response.Header().Get("Cache-Control"))
	}
	service.AssertNotCalled(t, "Query", mock.Anything, mock.Anything, mock.Anything)
}

func TestProviderReputationRouteRejectsMalformedQueries(t *testing.T) {
	service := &reputationServiceMock{}
	engine := reputationRoute(t, service)
	token := auth0.NewTokenBuilder().BuildToken("auth0|provider", nil)
	for _, query := range []string{
		"provider_id=2", "from=2026-01-01", "to=2026-02-01", "granularity=day", "compare_previous=true", "purpose=booking_deposit", "unknown=value",
		"limit=", "limit=0", "limit=101", "limit=-1", "limit=%2B1", "limit=+1", "limit=1.0", "limit=999999999999999999999999", "limit=1&limit=1", "limit=%GG", "limit=1;cursor=abc",
		"cursor=", "cursor=forged", "cursor=one&cursor=two",
	} {
		t.Run(query, func(t *testing.T) {
			response := collectionRequest(engine, reputationPath+"?"+query, token)
			require.Equal(t, http.StatusBadRequest, response.Code)
			require.JSONEq(t, `{"error":"invalid reputation query"}`, response.Body.String())
			require.Equal(t, "private, no-store", response.Header().Get("Cache-Control"))
		})
	}
	service.AssertNotCalled(t, "Query", mock.Anything, mock.Anything, mock.Anything)
}

func TestProviderReputationRouteMapsOnlyRequiredPrivateFields(t *testing.T) {
	average, coverage := 3.67, 75.0
	service := &reputationServiceMock{}
	instant := time.Date(2026, 10, 3, 9, 0, 0, 0, time.FixedZone("local", -3*60*60))
	service.On("Query", mock.Anything, "auth0|provider", provider.ReputationQueryInput{Limit: 20}).Return(&readmodel.Reputation{
		CalculatedAt: instant, AverageRating: &average, CoveragePercentage: &coverage, EligiblePaidOrders: 4, ReviewedPaidOrders: 3, RatingDistribution: [5]int64{0, 1, 0, 1, 1}, Reviews: []readmodel.ReputationReview{{WorkOrderID: 12, Rating: 5, Description: ""}},
	}, 7, nil).Once()
	response := collectionRequest(reputationRoute(t, service), reputationPath, auth0.NewTokenBuilder().BuildToken("auth0|provider", nil))
	require.Equal(t, http.StatusOK, response.Code)
	require.Equal(t, "private, no-store", response.Header().Get("Cache-Control"))
	require.JSONEq(t, `{"calculated_at":"2026-10-03T12:00:00Z","average_rating":3.67,"review_count":3,"rating_distribution":[{"rating":1,"count":0},{"rating":2,"count":1},{"rating":3,"count":0},{"rating":4,"count":1},{"rating":5,"count":1}],"eligible_paid_orders":4,"reviewed_paid_orders":3,"coverage_percentage":75,"reviews":[{"work_order_id":12,"rating":5,"description":""}],"next_cursor":null}`, response.Body.String())
	service.AssertExpectations(t)
}

func TestProviderReputationRouteReturnsEmptyArraysAndNullableValues(t *testing.T) {
	service := &reputationServiceMock{}
	service.On("Query", mock.Anything, "auth0|provider", provider.ReputationQueryInput{Limit: 100}).Return(&readmodel.Reputation{CalculatedAt: time.Date(2026, 10, 3, 0, 0, 0, 0, time.UTC)}, 7, nil).Once()
	response := collectionRequest(reputationRoute(t, service), reputationPath+"?limit=100", auth0.NewTokenBuilder().BuildToken("auth0|provider", nil))
	require.Equal(t, 200, response.Code)
	var body map[string]any
	require.NoError(t, json.Unmarshal(response.Body.Bytes(), &body))
	require.Nil(t, body["average_rating"])
	require.Nil(t, body["coverage_percentage"])
	require.Nil(t, body["next_cursor"])
	require.Empty(t, body["reviews"])
	require.IsType(t, []any{}, body["reviews"])
	require.Len(t, body["rating_distribution"], 5)
	service.AssertExpectations(t)
}

func TestProviderReputationRoutePropagatesRequestContext(t *testing.T) {
	service := &reputationServiceMock{}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	service.On("Query", mock.MatchedBy(func(value context.Context) bool { return value.Err() == context.Canceled }), "auth0|provider", mock.Anything).Return(nil, 0, context.Canceled).Once()
	engine := reputationRoute(t, service)
	request := httptest.NewRequest(http.MethodGet, reputationPath, nil).WithContext(ctx)
	request.Header.Set("Authorization", "Bearer "+auth0.NewTokenBuilder().BuildToken("auth0|provider", nil))
	response := httptest.NewRecorder()
	engine.ServeHTTP(response, request)
	require.Equal(t, 500, response.Code)
	service.AssertExpectations(t)
}

func TestProviderReputationRouteMapsControlledErrors(t *testing.T) {
	for _, test := range []struct {
		name    string
		failure error
		status  int
		message string
	}{
		{"invalid", provider.ErrInvalidReputationQuery, 400, "invalid reputation query"},
		{"forbidden", provider.ErrReputationForbidden, 403, "provider access required"},
		{"missing", provider.ErrReputationProviderNotFound, 404, "provider not found"},
		{"persistence", errors.New("database password=secret"), 500, "internal server error"},
		{"nil result", nil, 500, "internal server error"},
	} {
		t.Run(test.name, func(t *testing.T) {
			service := &reputationServiceMock{}
			service.On("Query", mock.Anything, "auth0|provider", mock.Anything).Return(nil, 0, test.failure).Once()
			response := collectionRequest(reputationRoute(t, service), reputationPath, auth0.NewTokenBuilder().BuildToken("auth0|provider", nil))
			require.Equal(t, test.status, response.Code)
			require.JSONEq(t, `{"error":"`+test.message+`"}`, response.Body.String())
			require.Equal(t, "private, no-store", response.Header().Get("Cache-Control"))
			service.AssertExpectations(t)
		})
	}
}

func TestProviderReputationCursorContinuesWithSignedLimitAndOwner(t *testing.T) {
	service := &reputationServiceMock{}
	position := readmodel.ReputationPosition{WorkOrderID: 9}
	service.On("Query", mock.Anything, "auth0|provider", provider.ReputationQueryInput{Limit: 1}).Return(&readmodel.Reputation{Next: &position, Reviews: []readmodel.ReputationReview{{WorkOrderID: 9, Rating: 4}}}, 7, nil).Once()
	service.On("Query", mock.Anything, "auth0|provider", mock.MatchedBy(func(input provider.ReputationQueryInput) bool {
		return input.Limit == 1 && input.CursorProviderID == 7 && input.After != nil && input.After.WorkOrderID == 9
	})).Return(&readmodel.Reputation{Reviews: []readmodel.ReputationReview{{WorkOrderID: 8, Rating: 5}}}, 7, nil).Twice()
	engine := reputationRoute(t, service)
	token := auth0.NewTokenBuilder().BuildToken("auth0|provider", nil)
	first := collectionRequest(engine, reputationPath+"?limit=1", token)
	require.Equal(t, 200, first.Code)
	var body struct {
		NextCursor *string `json:"next_cursor"`
	}
	require.NoError(t, json.Unmarshal(first.Body.Bytes(), &body))
	require.NotNil(t, body.NextCursor)
	for _, suffix := range []string{"", "&limit=1"} {
		response := collectionRequest(engine, reputationPath+"?cursor="+url.QueryEscape(*body.NextCursor)+suffix, token)
		require.Equal(t, 200, response.Code)
		require.Contains(t, response.Body.String(), `"next_cursor":null`)
	}
	response := collectionRequest(engine, reputationPath+"?cursor="+url.QueryEscape(*body.NextCursor)+"&limit=2", token)
	require.Equal(t, 400, response.Code)
	tampered := "A" + (*body.NextCursor)[1:]
	if strings.HasPrefix(*body.NextCursor, "A") {
		tampered = "B" + (*body.NextCursor)[1:]
	}
	response = collectionRequest(engine, reputationPath+"?cursor="+url.QueryEscape(tampered), token)
	require.Equal(t, 400, response.Code)
	service.AssertExpectations(t)
}

func TestProviderReputationCursorRejectsSignedInvalidPayloadsAndCrossPurpose(t *testing.T) {
	service := &reputationServiceMock{}
	engine := reputationRoute(t, service)
	token := auth0.NewTokenBuilder().BuildToken("auth0|provider", nil)
	codec, err := signedcursor.New(reputationTestCursorKey, "provider_reputation:v1")
	require.NoError(t, err)
	for _, payload := range []any{
		map[string]any{"v": 2, "provider_id": 7, "limit": 1, "after": map[string]any{"WorkOrderID": 9}},
		map[string]any{"v": 1, "provider_id": 0, "limit": 1, "after": map[string]any{"WorkOrderID": 9}},
		map[string]any{"v": 1, "provider_id": 7, "limit": 0, "after": map[string]any{"WorkOrderID": 9}},
		map[string]any{"v": 1, "provider_id": 7, "limit": 101, "after": map[string]any{"WorkOrderID": 9}},
		map[string]any{"v": 1, "provider_id": 7, "limit": 1, "after": map[string]any{"WorkOrderID": 0}},
		map[string]any{"v": 1, "provider_id": 7, "limit": 1, "after": map[string]any{"WorkOrderID": 9}, "from": "2026-01-01"},
	} {
		cursor, err := codec.Encode(payload)
		require.NoError(t, err)
		response := collectionRequest(engine, reputationPath+"?cursor="+url.QueryEscape(cursor), token)
		require.Equal(t, 400, response.Code)
	}
	foreignCodec, err := signedcursor.New(reputationTestCursorKey, "provider_collections:v1")
	require.NoError(t, err)
	cursor, err := foreignCodec.Encode(map[string]any{"v": 1, "provider_id": 7, "limit": 1, "after": readmodel.ReputationPosition{WorkOrderID: 9}})
	require.NoError(t, err)
	response := collectionRequest(engine, reputationPath+"?cursor="+url.QueryEscape(cursor), token)
	require.Equal(t, 400, response.Code)
	service.AssertNotCalled(t, "Query", mock.Anything, mock.Anything, mock.Anything)
}

func TestProviderReputationCursorForeignOwnerIsBoundInService(t *testing.T) {
	service := &reputationServiceMock{}
	codec, err := signedcursor.New(reputationTestCursorKey, "provider_reputation:v1")
	require.NoError(t, err)
	cursor, err := codec.Encode(map[string]any{"v": 1, "provider_id": 8, "limit": 1, "after": readmodel.ReputationPosition{WorkOrderID: 9}})
	require.NoError(t, err)
	service.On("Query", mock.Anything, "auth0|provider", mock.MatchedBy(func(input provider.ReputationQueryInput) bool { return input.CursorProviderID == 8 })).Return(nil, 0, provider.ErrInvalidReputationQuery).Once()
	response := collectionRequest(reputationRoute(t, service), reputationPath+"?cursor="+url.QueryEscape(cursor), auth0.NewTokenBuilder().BuildToken("auth0|provider", nil))
	require.Equal(t, 400, response.Code)
	service.AssertExpectations(t)
}

func TestProviderReputationHandlerRejectsShortSigningKey(t *testing.T) {
	handler, err := provider_handler.NewReputationHandler(nil, []byte("short"))
	require.Error(t, err)
	require.Nil(t, handler)
}
