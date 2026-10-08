package httpadapter

import (
	"errors"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/LoResuelvo/loresuelvo-api/internal/adapters/auth0"
	"github.com/LoResuelvo/loresuelvo-api/internal/adapters/http/handler/work_order_handler"
	"github.com/LoResuelvo/loresuelvo-api/internal/adapters/http/middleware"
	workorder "github.com/LoResuelvo/loresuelvo-api/internal/domain/work_order"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

func TestReviewReportRouteAuthenticatesAndSetsPrivateCache(t *testing.T) {
	service := new(reviewReporterMock)
	router := NewRouter(RouterConfig{Auth0Validator: auth0.NewFakeValidator(), WorkOrderHandler: work_order_handler.NewWorkOrderHandler(nil), ReviewReportHandler: work_order_handler.NewReviewReportHandler(service)})
	auth, err := middleware.BaseAutheticationLayer(router.auth0Validator)
	require.NoError(t, err)
	engine := gin.New()
	router.registerWorkOrderRoutes(engine, auth)
	for _, token := range []string{"", "invalid-token"} {
		request := httptest.NewRequest("POST", "/work-orders/1/reviews/reports", strings.NewReader(`{"category":"personal_data"}`))
		if token != "" {
			request.Header.Set("Authorization", "Bearer "+token)
		}
		response := httptest.NewRecorder()
		engine.ServeHTTP(response, request)
		require.Equal(t, 401, response.Code)
		require.Equal(t, "private, no-store", response.Header().Get("Cache-Control"))
		require.JSONEq(t, `{"error":"invalid_token","message":"Failed to validate JWT."}`, response.Body.String())
	}
	require.Empty(t, service.Calls)
}
func TestReviewReportRouteHasNoAdminPermissionGateAndKeepsCacheOnResults(t *testing.T) {
	for _, tc := range []struct {
		status int
		err    error
	}{{201, nil}, {400, workorder.ErrInvalidReviewReport}, {403, workorder.ErrReviewReportForbidden}, {404, workorder.ErrReviewNotAvailable}, {409, workorder.ErrReviewReportAlreadyExists}, {500, errors.New("private database details")}} {
		service := new(reviewReporterMock)
		var report *workorder.ReviewReport
		if tc.err == nil {
			var err error
			report, err = workorder.NewReviewReport(1, 2, "personal_data", "", time.Now())
			require.NoError(t, err)
			report.SetID(3)
		}
		service.On("Report", mock.Anything, "subject", 1, "personal_data", "").Return(report, tc.err).Once()
		router := NewRouter(RouterConfig{Auth0Validator: auth0.NewFakeValidator(), WorkOrderHandler: work_order_handler.NewWorkOrderHandler(nil), ReviewReportHandler: work_order_handler.NewReviewReportHandler(service)})
		auth, err := middleware.BaseAutheticationLayer(router.auth0Validator)
		require.NoError(t, err)
		engine := gin.New()
		router.registerWorkOrderRoutes(engine, auth)
		request := httptest.NewRequest("POST", "/work-orders/1/reviews/reports", strings.NewReader(`{"category":"personal_data"}`))
		request.Header.Set("Authorization", "Bearer "+auth0.NewTokenBuilder().BuildToken("subject", nil))
		response := httptest.NewRecorder()
		engine.ServeHTTP(response, request)
		require.Equal(t, tc.status, response.Code)
		require.Equal(t, "private, no-store", response.Header().Get("Cache-Control"))
		require.NotContains(t, response.Body.String(), "private database details")
		service.AssertExpectations(t)
	}
}
