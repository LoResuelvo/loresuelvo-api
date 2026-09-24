package audit_log_handler

import (
	"encoding/json"
	"errors"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/LoResuelvo/loresuelvo-api/internal/adapters/http/middleware"
	"github.com/LoResuelvo/loresuelvo-api/internal/domain/audit"
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

func testRouter(t *testing.T, service *serviceMock) *gin.Engine {
	t.Helper()
	gin.SetMode(gin.TestMode)
	router := gin.New()
	router.Use(middleware.RequestLogger(slog.New(slog.NewTextHandler(io.Discard, nil))))
	handler, err := NewHandler(service, []byte("test-audit-cursor-signing-key-2026-keep-private"))
	require.NoError(t, err)
	router.GET("/admin/audit-logs", func(c *gin.Context) {
		c.Set(middleware.ContextKeyUserID, "auth0|operator")
	}, handler.List)
	return router
}

func TestListReturnsEmptyPageAndPassesOperator(t *testing.T) {
	operatorID := 42
	service := new(serviceMock)
	service.On("Query", mock.Anything, "auth0|operator", "request-1", mock.MatchedBy(func(got audit.LogQuery) bool {
		return got.Filter.OperatorID != nil && *got.Filter.OperatorID == operatorID && got.Limit == 20
	})).Return(audit.LogPage{Events: []*audit.Event{}}, nil).Once()

	request := httptest.NewRequest(http.MethodGet, "/admin/audit-logs?operator_id=42", nil)
	request.Header.Set("X-Request-ID", "request-1")
	response := httptest.NewRecorder()
	testRouter(t, service).ServeHTTP(response, request)

	require.Equal(t, http.StatusOK, response.Code)
	require.JSONEq(t, `{"events":[],"next_cursor":null}`, response.Body.String())
	service.AssertExpectations(t)
}

func TestListRejectsInvalidOperatorWithoutCallingService(t *testing.T) {
	for _, query := range []string{
		"operator_id=0", "operator_id=abc", "operator_id=2147483648", "operator_id=1&operator_id=2", "operator_id=",
		"action=unknown", "resource_type=Payment", "resource_id=https%3A%2F%2Fprivate.test", "result=unknown",
		"occurred_from=not-a-date", "occurred_to=not-a-date", "occurred_from=2026-09-21T00%3A00%3A00Z&occurred_to=2026-09-20T00%3A00%3A00Z",
		"action=create&action=execute", "unknown=1",
	} {
		t.Run(query, func(t *testing.T) {
			service := new(serviceMock)
			response := httptest.NewRecorder()
			testRouter(t, service).ServeHTTP(response, httptest.NewRequest(http.MethodGet, "/admin/audit-logs?"+query, nil))

			require.Equal(t, http.StatusBadRequest, response.Code)
			require.JSONEq(t, `{"error":"invalid filter"}`, response.Body.String())
			service.AssertNotCalled(t, "Query", mock.Anything, mock.Anything, mock.Anything, mock.Anything)
		})
	}
}

func TestListSignsNextCursorAndContinuesWithOriginalFilters(t *testing.T) {
	service := new(serviceMock)
	position := audit.LogPosition{OccurredOn: time.Date(2026, 9, 20, 10, 0, 0, 0, time.UTC), ID: uuid.New()}
	service.On("Query", mock.Anything, "auth0|operator", mock.Anything, mock.MatchedBy(func(query audit.LogQuery) bool {
		return query.Limit == 2 && query.Filter.OperatorID != nil && *query.Filter.OperatorID == 42 && query.Watermark == nil && query.Before == nil
	})).Return(audit.LogPage{Events: []*audit.Event{}, Watermark: 81, Next: &position}, nil).Once()
	service.On("Query", mock.Anything, "auth0|operator", mock.Anything, mock.MatchedBy(func(query audit.LogQuery) bool {
		return query.Limit == 2 && query.Filter.OperatorID != nil && *query.Filter.OperatorID == 42 &&
			query.Watermark != nil && *query.Watermark == 81 && query.Before != nil && *query.Before == position
	})).Return(audit.LogPage{Events: []*audit.Event{}, Watermark: 81}, nil).Once()
	router := testRouter(t, service)
	first := httptest.NewRecorder()
	router.ServeHTTP(first, httptest.NewRequest(http.MethodGet, "/admin/audit-logs?operator_id=42&limit=2", nil))
	require.Equal(t, http.StatusOK, first.Code)
	var page struct {
		NextCursor *string `json:"next_cursor"`
	}
	require.NoError(t, json.Unmarshal(first.Body.Bytes(), &page))
	require.NotNil(t, page.NextCursor)

	second := httptest.NewRecorder()
	router.ServeHTTP(second, httptest.NewRequest(http.MethodGet, "/admin/audit-logs?cursor="+*page.NextCursor, nil))
	require.Equal(t, http.StatusOK, second.Code)
	require.JSONEq(t, `{"events":[],"next_cursor":null}`, second.Body.String())
	service.AssertExpectations(t)
}

func TestListRejectsInvalidLimitAndCursorBeforeService(t *testing.T) {
	position := audit.LogPosition{OccurredOn: time.Date(2026, 9, 20, 10, 0, 0, 0, time.UTC), ID: uuid.New()}
	codec, err := newCursorCodec([]byte("test-audit-cursor-signing-key-2026-keep-private"))
	require.NoError(t, err)
	operatorID := 42
	action := audit.ActionCreate
	valid, err := codec.encode(cursorPayload{Watermark: 81, Before: position, Filter: audit.LogFilter{OperatorID: &operatorID, Action: &action}, Limit: 2})
	require.NoError(t, err)
	for _, query := range []string{
		"limit=0", "limit=101", "limit=abc", "limit=2&limit=3", "cursor=not-a-cursor",
		"cursor=" + valid + "&operator_id=43", "cursor=" + valid + "&action=execute", "cursor=" + valid + "&limit=3",
	} {
		t.Run(query, func(t *testing.T) {
			service := new(serviceMock)
			response := httptest.NewRecorder()
			testRouter(t, service).ServeHTTP(response, httptest.NewRequest(http.MethodGet, "/admin/audit-logs?"+query, nil))
			require.Equal(t, http.StatusBadRequest, response.Code)
			require.JSONEq(t, `{"error":"invalid filter"}`, response.Body.String())
			service.AssertNotCalled(t, "Query", mock.Anything, mock.Anything, mock.Anything, mock.Anything)
		})
	}
}

func TestListDoesNotLeakServiceFailure(t *testing.T) {
	service := new(serviceMock)
	service.On("Query", mock.Anything, "auth0|operator", mock.Anything, audit.LogQuery{Limit: 20}).
		Return(audit.LogPage{}, errors.New("private reconciliation reason")).Once()
	response := httptest.NewRecorder()
	testRouter(t, service).ServeHTTP(response, httptest.NewRequest(http.MethodGet, "/admin/audit-logs", nil))

	require.Equal(t, http.StatusInternalServerError, response.Code)
	require.JSONEq(t, `{"error":"internal server error"}`, response.Body.String())
	require.NotContains(t, response.Body.String(), "private reconciliation reason")
	service.AssertExpectations(t)
}

func TestListPassesCombinedFiltersWithNormalizedInstants(t *testing.T) {
	from, err := time.Parse(time.RFC3339, "2026-09-20T10:00:00Z")
	require.NoError(t, err)
	to, err := time.Parse(time.RFC3339, "2026-09-21T00:00:00Z")
	require.NoError(t, err)
	service := new(serviceMock)
	service.On("Query", mock.Anything, "auth0|operator", mock.Anything, mock.MatchedBy(func(got audit.LogQuery) bool {
		return got.Limit == 20 &&
			got.Filter.OperatorID != nil && *got.Filter.OperatorID == 42 &&
			got.Filter.Action != nil && *got.Filter.Action == audit.ActionExecute &&
			got.Filter.ResourceType != nil && *got.Filter.ResourceType == "payment" &&
			got.Filter.ResourceID != nil && *got.Filter.ResourceID == "42" &&
			got.Filter.Result != nil && *got.Filter.Result == audit.ResultSucceeded &&
			got.Filter.OccurredFrom != nil && got.Filter.OccurredFrom.Equal(from) &&
			got.Filter.OccurredTo != nil && got.Filter.OccurredTo.Equal(to)
	})).Return(audit.LogPage{Events: []*audit.Event{}}, nil).Once()

	request := httptest.NewRequest(http.MethodGet,
		"/admin/audit-logs?operator_id=42&action=execute&resource_type=payment&resource_id=42&result=succeeded&occurred_from=2026-09-20T07:00:00-03:00&occurred_to=2026-09-21T00:00:00Z", nil)
	response := httptest.NewRecorder()
	testRouter(t, service).ServeHTTP(response, request)

	require.Equal(t, http.StatusOK, response.Code)
	service.AssertExpectations(t)
}
