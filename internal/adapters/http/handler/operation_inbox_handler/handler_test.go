package operation_inbox_handler

import (
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	jobrequest "github.com/LoResuelvo/loresuelvo-api/internal/domain/job_request"
	"github.com/LoResuelvo/loresuelvo-api/internal/domain/operation"
	readmodel "github.com/LoResuelvo/loresuelvo-api/internal/domain/operation/read_model"
	serviceproposal "github.com/LoResuelvo/loresuelvo-api/internal/domain/service_proposal"
	workorder "github.com/LoResuelvo/loresuelvo-api/internal/domain/work_order"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

func testRouter(service *serviceMock) *gin.Engine {
	gin.SetMode(gin.TestMode)
	router := gin.New()
	router.GET("/admin/operations", NewHandler(service).List)
	return router
}

func TestListRendersOperationsWithExplicitNullReferences(t *testing.T) {
	startedOn := time.Date(2026, 9, 25, 12, 0, 0, 0, time.FixedZone("ART", -3*60*60))
	service := new(serviceMock)
	service.On("Query", mock.Anything, operation.InboxQuery{}).Return(operation.InboxPage{Operations: []readmodel.OperationSummary{
		{
			ID: readmodel.ID{Kind: readmodel.KindServiceProposal, ResourceID: 34}, StartedOn: startedOn,
			Stage:           readmodel.StageWorkOrderScheduled,
			JobRequest:      &readmodel.JobRequest{ID: 12, Status: jobrequest.StatusAccepted},
			ServiceProposal: &readmodel.ServiceProposal{ID: 34, Status: serviceproposal.StatusAccepted},
			WorkOrder:       &readmodel.WorkOrder{ID: 56, Status: workorder.StatusScheduled},
		},
		{
			ID: readmodel.ID{Kind: readmodel.KindJobRequest, ResourceID: 13}, StartedOn: startedOn.Add(-time.Hour),
			Stage:      readmodel.StageRequestPending,
			JobRequest: &readmodel.JobRequest{ID: 13, Status: jobrequest.StatusPending},
		},
	}}, nil).Once()

	response := httptest.NewRecorder()
	testRouter(service).ServeHTTP(response, httptest.NewRequest(http.MethodGet, "/admin/operations", nil))

	require.Equal(t, http.StatusOK, response.Code)
	require.JSONEq(t, `{"operations":[
		{"id":"sp-34","stage":"work_order_scheduled","started_on":"2026-09-25T15:00:00Z",
		 "job_request":{"id":12,"status":"accepted"},"service_proposal":{"id":34,"status":"accepted"},
		 "work_order":{"id":56,"status":"scheduled"}},
		{"id":"jr-13","stage":"request_pending","started_on":"2026-09-25T14:00:00Z",
		 "job_request":{"id":13,"status":"pending"},"service_proposal":null,"work_order":null}
	],"next_cursor":null}`, response.Body.String())
	service.AssertExpectations(t)
}

func TestListReturnsEmptyCollection(t *testing.T) {
	service := new(serviceMock)
	service.On("Query", mock.Anything, operation.InboxQuery{}).Return(operation.InboxPage{Operations: []readmodel.OperationSummary{}}, nil).Once()

	response := httptest.NewRecorder()
	testRouter(service).ServeHTTP(response, httptest.NewRequest(http.MethodGet, "/admin/operations", nil))

	require.Equal(t, http.StatusOK, response.Code)
	require.JSONEq(t, `{"operations":[],"next_cursor":null}`, response.Body.String())
}

func TestListRejectsUnsupportedParametersWithoutCallingService(t *testing.T) {
	service := new(serviceMock)

	response := httptest.NewRecorder()
	testRouter(service).ServeHTTP(response, httptest.NewRequest(http.MethodGet, "/admin/operations?unknown=1", nil))

	require.Equal(t, http.StatusBadRequest, response.Code)
	service.AssertNotCalled(t, "Query", mock.Anything, mock.Anything)
}

func TestListHidesServiceFailures(t *testing.T) {
	service := new(serviceMock)
	service.On("Query", mock.Anything, operation.InboxQuery{}).Return(operation.InboxPage{}, errors.New("database unavailable")).Once()

	response := httptest.NewRecorder()
	testRouter(service).ServeHTTP(response, httptest.NewRequest(http.MethodGet, "/admin/operations", nil))

	require.Equal(t, http.StatusInternalServerError, response.Code)
	require.JSONEq(t, `{"error":"internal server error"}`, response.Body.String())
}
