package work_order_handler

import (
	"errors"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/LoResuelvo/loresuelvo-api/internal/adapters/http/middleware"
	workorder "github.com/LoResuelvo/loresuelvo-api/internal/domain/work_order"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

func TestReviewReportHandlerRejectsInvalidDocuments(t *testing.T) {
	for _, body := range []string{`[]`, `null`, `{`, `{} {}`, `{"category":"personal_data","category":"spam_advertising"}`, `{"category":null}`, `{"category":3}`, `{"category":"personal_data","explanation":null}`, `{"category":"personal_data","explanation":{}}`, `{"category":"personal_data","id":22}`, `{"category":"personal_data","reporter_id":22}`, `{"category":"personal_data","status":"dismissed"}`, `{"category":"personal_data","created_on":"2026-01-01"}`, "{\"category\":\"personal_data\",\"explanation\":\"\xff\"}", strings.Repeat(" ", 65537) + `{"category":"personal_data"}`} {
		t.Run(body[:min(len(body), 60)], func(t *testing.T) {
			service := new(reviewReporterMock)
			router := gin.New()
			router.POST("/work-orders/:workOrderID/reviews/reports", func(c *gin.Context) { c.Set(middleware.ContextKeyUserID, "auth") }, NewReviewReportHandler(service).Report)
			response := httptest.NewRecorder()
			router.ServeHTTP(response, httptest.NewRequest("POST", "/work-orders/1/reviews/reports", strings.NewReader(body)))
			require.Equal(t, 400, response.Code)
			service.AssertNotCalled(t, "Report", mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything)
		})
	}
}
func TestReviewReportHandlerMapsErrorsWithoutPrivateText(t *testing.T) {
	for _, tc := range []struct {
		err    error
		status int
	}{{workorder.ErrInvalidReviewReport, 400}, {workorder.ErrReviewReportForbidden, 403}, {workorder.ErrReviewNotAvailable, 404}, {workorder.ErrReviewReportAlreadyExists, 409}, {errors.New("private explanation database detail"), 500}} {
		service := new(reviewReporterMock)
		service.On("Report", mock.Anything, "auth", 1, "personal_data", "private explanation").Return(nil, tc.err).Once()
		router := gin.New()
		router.POST("/work-orders/:workOrderID/reviews/reports", func(c *gin.Context) { c.Set(middleware.ContextKeyUserID, "auth") }, NewReviewReportHandler(service).Report)
		response := httptest.NewRecorder()
		router.ServeHTTP(response, httptest.NewRequest("POST", "/work-orders/1/reviews/reports", strings.NewReader(`{"category":"personal_data","explanation":"private explanation"}`)))
		require.Equal(t, tc.status, response.Code)
		require.NotContains(t, response.Body.String(), "private explanation")
		service.AssertExpectations(t)
	}
}
func TestReviewReportHandlerOnlyReturnsReceipt(t *testing.T) {
	report, err := workorder.NewReviewReport(1, 2, "personal_data", "secret explanation", time.Now())
	require.NoError(t, err)
	report.SetID(3)
	service := new(reviewReporterMock)
	service.On("Report", mock.Anything, "auth", 1, "personal_data", "").Return(report, nil).Once()
	router := gin.New()
	router.POST("/work-orders/:workOrderID/reviews/reports", func(c *gin.Context) { c.Set(middleware.ContextKeyUserID, "auth") }, NewReviewReportHandler(service).Report)
	response := httptest.NewRecorder()
	router.ServeHTTP(response, httptest.NewRequest("POST", "/work-orders/1/reviews/reports", strings.NewReader(`{"category":"personal_data"}`)))
	require.Equal(t, 201, response.Code)
	require.JSONEq(t, `{"id":3,"status":"pending"}`, response.Body.String())
	service.AssertExpectations(t)
}
func TestReviewReportHandlerRejectsMissingIdentityAndInvalidTarget(t *testing.T) {
	for _, tc := range []struct {
		auth, id string
		status   int
	}{{"", "1", 401}, {"auth", "0", 400}, {"auth", "abc", 400}} {
		service := new(reviewReporterMock)
		router := gin.New()
		router.POST("/work-orders/:workOrderID/reviews/reports", func(c *gin.Context) {
			if tc.auth != "" {
				c.Set(middleware.ContextKeyUserID, tc.auth)
			}
		}, NewReviewReportHandler(service).Report)
		response := httptest.NewRecorder()
		router.ServeHTTP(response, httptest.NewRequest("POST", "/work-orders/"+tc.id+"/reviews/reports", strings.NewReader(`{"category":"personal_data"}`)))
		require.Equal(t, tc.status, response.Code)
		service.AssertNotCalled(t, "Report", mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything)
	}
}
