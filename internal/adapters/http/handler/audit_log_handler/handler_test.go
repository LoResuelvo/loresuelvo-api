package audit_log_handler

import (
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
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

func testRouter(service *serviceMock) *gin.Engine {
	gin.SetMode(gin.TestMode)
	router := gin.New()
	router.Use(middleware.RequestLogger(slog.New(slog.NewTextHandler(io.Discard, nil))))
	router.GET("/admin/audit-logs", func(c *gin.Context) {
		c.Set(middleware.ContextKeyUserID, "auth0|operator")
	}, NewHandler(service).List)
	return router
}

func TestListReturnsEmptyPageAndPassesOperator(t *testing.T) {
	operatorID := 42
	service := new(serviceMock)
	service.On("Query", mock.Anything, "auth0|operator", "request-1", mock.MatchedBy(func(got audit.LogFilter) bool {
		return got.OperatorID != nil && *got.OperatorID == operatorID
	})).Return([]*audit.Event{}, nil).Once()

	request := httptest.NewRequest(http.MethodGet, "/admin/audit-logs?operator_id=42", nil)
	request.Header.Set("X-Request-ID", "request-1")
	response := httptest.NewRecorder()
	testRouter(service).ServeHTTP(response, request)

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
			testRouter(service).ServeHTTP(response, httptest.NewRequest(http.MethodGet, "/admin/audit-logs?"+query, nil))

			require.Equal(t, http.StatusBadRequest, response.Code)
			require.JSONEq(t, `{"error":"invalid filter"}`, response.Body.String())
			service.AssertNotCalled(t, "Query", mock.Anything, mock.Anything, mock.Anything, mock.Anything)
		})
	}
}

func TestListDoesNotLeakServiceFailure(t *testing.T) {
	service := new(serviceMock)
	service.On("Query", mock.Anything, "auth0|operator", mock.Anything, audit.LogFilter{}).
		Return(nil, errors.New("private reconciliation reason")).Once()
	response := httptest.NewRecorder()
	testRouter(service).ServeHTTP(response, httptest.NewRequest(http.MethodGet, "/admin/audit-logs", nil))

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
	service.On("Query", mock.Anything, "auth0|operator", mock.Anything, mock.MatchedBy(func(got audit.LogFilter) bool {
		return got.OperatorID != nil && *got.OperatorID == 42 &&
			got.Action != nil && *got.Action == audit.ActionExecute &&
			got.ResourceType != nil && *got.ResourceType == "payment" &&
			got.ResourceID != nil && *got.ResourceID == "42" &&
			got.Result != nil && *got.Result == audit.ResultSucceeded &&
			got.OccurredFrom != nil && got.OccurredFrom.Equal(from) &&
			got.OccurredTo != nil && got.OccurredTo.Equal(to)
	})).Return([]*audit.Event{}, nil).Once()

	request := httptest.NewRequest(http.MethodGet,
		"/admin/audit-logs?operator_id=42&action=execute&resource_type=payment&resource_id=42&result=succeeded&occurred_from=2026-09-20T07:00:00-03:00&occurred_to=2026-09-21T00:00:00Z", nil)
	response := httptest.NewRecorder()
	testRouter(service).ServeHTTP(response, request)

	require.Equal(t, http.StatusOK, response.Code)
	service.AssertExpectations(t)
}
