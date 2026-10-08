package admin_review_handler

import (
	"errors"
	"fmt"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	workorder "github.com/LoResuelvo/loresuelvo-api/internal/domain/work_order"
	readmodel "github.com/LoResuelvo/loresuelvo-api/internal/domain/work_order/read_model"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

const validHideBody = `{"action":"hide","category":"abusive_language","reason":"Valid reason","expected_version":1}`

func TestModerationRejectsMalformedOrSpoofedInput(t *testing.T) {
	for _, body := range []string{
		`null`, `[]`, `{}`, `{"action":"hide","category":"abusive_language","reason":"Valid","expected_version":1,"operator_id":9}`,
		`{"action":"hide","category":"abusive_language","reason":"Valid","expected_version":1,"created_on":"2026-01-01"}`,
		`{"action":"hide","category":"abusive_language","reason":"Valid","expected_version":1,"decision_id":9}`,
		`{"action":"hide","category":"abusive_language","reason":"Valid","expected_version":1,"visibility":"visible"}`,
		`{"action":"hide","category":"abusive_language","reason":"Valid","reason":"Different","expected_version":1}`,
		validHideBody + ` {}`, validHideBody + ` invalid`,
		"{\"action\":\"hide\",\"category\":\"abusive_language\",\"reason\":\"\xff\",\"expected_version\":1}",
		`{"action":"hide","category":"abusive_language","reason":"Invalid\u0000reason","expected_version":1}`,
		`{"action":null,"reason":"Valid","expected_version":1}`,
		`{"action":"hide","category":null,"reason":"Valid","expected_version":1}`,
		`{"action":"hide","category":"abusive_language","reason":null,"expected_version":1}`,
		`{"action":"hide","category":"abusive_language","reason":"Valid","expected_version":null}`,
		`{"action":"hide","category":"abusive_language","reason":"Valid","expected_version":1,"report_id":null}`,
		`{"action":1,"reason":"Valid","expected_version":1}`,
		`{"action":"hide","category":1,"reason":"Valid","expected_version":1}`,
		`{"action":"hide","category":"abusive_language","reason":1,"expected_version":1}`,
		`{"action":"hide","category":"abusive_language","reason":"Valid","expected_version":"1"}`,
		`{"action":"hide","category":"abusive_language","reason":"Valid","expected_version":1.5}`,
		`{"action":"hide","category":"abusive_language","reason":"Valid","expected_version":0}`,
		`{"action":"dismiss_reports","reason":"Valid","expected_version":1,"report_id":0}`,
		`{"action":"dismiss_reports","reason":"Valid","expected_version":1,"report_id":"1"}`,
		`{"action":"dismiss_reports","reason":"Valid","expected_version":1,"report_id":1.5}`,
		`{"action":"hide","category":"unknown","reason":"Valid","expected_version":1}`,
		`{"action":"unhide","category":"spam_advertising","reason":"Valid","expected_version":1}`,
		`{"action":"hide","category":"abusive_language","reason":"  ","expected_version":1}`,
	} {
		t.Run(fmt.Sprintf("%q", body), func(t *testing.T) {
			service := new(serviceMock)
			response := httptest.NewRecorder()
			newHandlerTestRouter(service).ServeHTTP(response, httptest.NewRequest("POST", "/admin/reviews/1/moderate", strings.NewReader(body)))
			require.Equal(t, 400, response.Code, response.Body.String())
			require.Empty(t, service.Calls)
		})
	}
}

func TestModerationRejectsOversizedValidJSONBeforeService(t *testing.T) {
	service := new(serviceMock)
	body := strings.Repeat(" ", 64*1024) + validHideBody
	response := httptest.NewRecorder()
	newHandlerTestRouter(service).ServeHTTP(response, httptest.NewRequest("POST", "/admin/reviews/1/moderate", strings.NewReader(body)))
	require.Equal(t, 400, response.Code)
	require.Empty(t, service.Calls)
}

func TestModerationReturnsOnlyDecisionConfirmation(t *testing.T) {
	service := new(serviceMock)
	review, err := workorder.NewReview(5, "Restricted original")
	require.NoError(t, err)
	decision, err := review.Moderate(1, workorder.ModerationInput{Action: "hide", Category: "abusive_language", Reason: "Valid reason", ExpectedVersion: 1}, nil, 7, time.Now())
	require.NoError(t, err)
	decision.SetID(9)
	service.On("Moderate", mock.Anything, "admin", 1, workorder.ModerationInput{Action: "hide", Category: "abusive_language", Reason: "Valid reason", ExpectedVersion: 1}, "http-test-correlation").Return(&workorder.ReviewModerationResult{Review: review, Decision: decision}, nil).Once()
	request := httptest.NewRequest("POST", "/admin/reviews/1/moderate", strings.NewReader(strings.Replace(validHideBody, "Valid reason", "  Valid reason  ", 1)))
	request.Header.Set("X-Request-ID", "http-test-correlation")
	response := httptest.NewRecorder()
	newHandlerTestRouter(service).ServeHTTP(response, request)
	require.Equal(t, 200, response.Code, response.Body.String())
	require.JSONEq(t, `{"work_order_id":1,"visibility":"hidden","version":2,"decision_id":9}`, response.Body.String())
	service.AssertExpectations(t)
}

func TestReviewListPassesBoundedPaginationAndOmitsOriginals(t *testing.T) {
	service := new(serviceMock)
	service.On("List", mock.Anything, workorder.ReviewListInput{ReviewPageInput: workorder.ReviewPageInput{Page: 2, Limit: 10}, Status: "hidden"}).Return(&workorder.AdminReviewPage{Page: 2, Limit: 10, Total: 11, Items: []readmodel.AdminReviewSummary{{WorkOrderID: 1, Rating: 5, Version: 2}}}, nil).Once()
	response := httptest.NewRecorder()
	newHandlerTestRouter(service).ServeHTTP(response, httptest.NewRequest("GET", "/admin/reviews?status=hidden&page=2&limit=10", nil))
	require.Equal(t, 200, response.Code)
	require.Contains(t, response.Body.String(), `"visibility":"hidden"`)
	for _, restricted := range []string{"description", "reason", "explanation", "decisions"} {
		require.NotContains(t, response.Body.String(), restricted)
	}
	service.AssertExpectations(t)
}

func TestReviewDetailPassesDecisionPaginationAndServerIdentity(t *testing.T) {
	service := new(serviceMock)
	review, err := workorder.NewReview(5, "Restricted original")
	require.NoError(t, err)
	service.On("Get", mock.Anything, "admin", 1, workorder.ReviewPageInput{Page: 2, Limit: 10}, "http-test-correlation").Return(&workorder.AdminReviewDetail{Review: review, Page: 2, Limit: 10, Total: 21}, nil).Once()
	request := httptest.NewRequest("GET", "/admin/reviews/1?decisions_page=2&decisions_limit=10", nil)
	request.Header.Set("X-Request-ID", "http-test-correlation")
	response := httptest.NewRecorder()
	newHandlerTestRouter(service).ServeHTTP(response, request)
	require.Equal(t, 200, response.Code)
	require.Contains(t, response.Body.String(), `"description":"Restricted original"`)
	require.Contains(t, response.Body.String(), `"has_more":true`)
	service.AssertExpectations(t)
}

func TestReviewQueriesRejectUnknownRepeatedAndUnboundedParameters(t *testing.T) {
	for _, path := range []string{
		"/admin/reviews?page=0", "/admin/reviews?page=-1", "/admin/reviews?page=9223372036854775807&limit=100", "/admin/reviews?limit=101", "/admin/reviews?limit=0", "/admin/reviews?limit=invalid", "/admin/reviews?page=1&page=2", "/admin/reviews?status=hidden&status=visible", "/admin/reviews?unknown=1", "/admin/reviews?decisions_page=1",
		"/admin/reviews/1?page=1", "/admin/reviews/1?limit=10", "/admin/reviews/1?status=hidden", "/admin/reviews/1?decisions_limit=101", "/admin/reviews/1?decisions_page=1&decisions_page=2", "/admin/reviews/invalid", "/admin/reviews/0",
	} {
		t.Run(path, func(t *testing.T) {
			service := new(serviceMock)
			response := httptest.NewRecorder()
			newHandlerTestRouter(service).ServeHTTP(response, httptest.NewRequest("GET", path, nil))
			require.Equal(t, 400, response.Code, response.Body.String())
			require.Empty(t, service.Calls)
		})
	}
}

func TestModerationMapsDomainErrorsWithoutLeakingFailureData(t *testing.T) {
	for _, test := range []struct {
		err    error
		status int
	}{
		{workorder.ErrInvalidReviewModeration, 400}, {workorder.ErrReviewReportForbidden, 403}, {workorder.ErrReviewNotAvailable, 404}, {workorder.ErrReviewModerationConflict, 409}, {errors.New("private-reason private-original private-token"), 500},
	} {
		t.Run(fmt.Sprint(test.status), func(t *testing.T) {
			service := new(serviceMock)
			service.On("Moderate", mock.Anything, "admin", 1, mock.Anything, mock.Anything).Return(nil, test.err).Once()
			response := httptest.NewRecorder()
			newHandlerTestRouter(service).ServeHTTP(response, httptest.NewRequest("POST", "/admin/reviews/1/moderate", strings.NewReader(validHideBody)))
			require.Equal(t, test.status, response.Code)
			require.NotContains(t, response.Body.String(), "private-")
			service.AssertExpectations(t)
		})
	}
}

func TestReviewListDefaultsToBoundedFirstPage(t *testing.T) {
	service := new(serviceMock)
	service.On("List", mock.Anything, workorder.ReviewListInput{ReviewPageInput: workorder.ReviewPageInput{Page: 1, Limit: 20}}).Return(&workorder.AdminReviewPage{Page: 1, Limit: 20}, nil).Once()
	response := httptest.NewRecorder()
	newHandlerTestRouter(service).ServeHTTP(response, httptest.NewRequest("GET", "/admin/reviews", nil))
	require.Equal(t, 200, response.Code)
	require.JSONEq(t, `{"items":[],"page":1,"limit":20,"total":0}`, response.Body.String())
	service.AssertExpectations(t)
}

func TestReviewSensitiveReadsDoNotExposeContentWhenServiceFails(t *testing.T) {
	for _, path := range []string{"/admin/reviews", "/admin/reviews/1"} {
		t.Run(path, func(t *testing.T) {
			service := new(serviceMock)
			privateError := errors.New("private-original private-explanation private-reason")
			if path == "/admin/reviews" {
				service.On("List", mock.Anything, mock.Anything).Return(nil, privateError).Once()
			} else {
				service.On("Get", mock.Anything, "admin", 1, mock.Anything, mock.Anything).Return(nil, privateError).Once()
			}
			response := httptest.NewRecorder()
			newHandlerTestRouter(service).ServeHTTP(response, httptest.NewRequest("GET", path, nil))
			require.Equal(t, 500, response.Code)
			require.JSONEq(t, `{"error":"internal server error"}`, response.Body.String())
			service.AssertExpectations(t)
		})
	}
}

func TestModerationRejectsOversizedReasonWithoutService(t *testing.T) {
	service := new(serviceMock)
	body := strings.Replace(validHideBody, "Valid reason", strings.Repeat("a", 501), 1)
	response := httptest.NewRecorder()
	newHandlerTestRouter(service).ServeHTTP(response, httptest.NewRequest("POST", "/admin/reviews/1/moderate", strings.NewReader(body)))
	require.Equal(t, 400, response.Code)
	require.Empty(t, service.Calls)
}

func TestReviewListAcceptsLargePageWithoutArtificialBusinessCap(t *testing.T) {
	service := new(serviceMock)
	service.On("List", mock.Anything, workorder.ReviewListInput{ReviewPageInput: workorder.ReviewPageInput{Page: 1000001, Limit: 20}}).Return(&workorder.AdminReviewPage{Page: 1000001, Limit: 20}, nil).Once()
	response := httptest.NewRecorder()
	newHandlerTestRouter(service).ServeHTTP(response, httptest.NewRequest("GET", "/admin/reviews?page=1000001", nil))
	require.Equal(t, 200, response.Code)
	require.JSONEq(t, `{"items":[],"page":1000001,"limit":20,"total":0}`, response.Body.String())
	service.AssertExpectations(t)
}
