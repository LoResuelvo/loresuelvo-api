package audit_log_handler

import (
	"errors"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"testing"

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
	service.On("Query", mock.Anything, "auth0|operator", "request-1", mock.MatchedBy(func(got *int) bool {
		return got != nil && *got == operatorID
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
	for _, query := range []string{"operator_id=0", "operator_id=abc", "operator_id=2147483648", "operator_id=1&operator_id=2", "operator_id=", "action=create"} {
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
	service.On("Query", mock.Anything, "auth0|operator", mock.Anything, (*int)(nil)).
		Return(nil, errors.New("private reconciliation reason")).Once()
	response := httptest.NewRecorder()
	testRouter(service).ServeHTTP(response, httptest.NewRequest(http.MethodGet, "/admin/audit-logs", nil))

	require.Equal(t, http.StatusInternalServerError, response.Code)
	require.JSONEq(t, `{"error":"internal server error"}`, response.Body.String())
	require.NotContains(t, response.Body.String(), "private reconciliation reason")
	service.AssertExpectations(t)
}
