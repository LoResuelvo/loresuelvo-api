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

func TestListRendersBoundedSummariesWithExplicitNulls(t *testing.T) {
	art := time.FixedZone("ART", -3*60*60)
	startedOn := time.Date(2026, 9, 25, 12, 0, 0, 0, art)
	reportedOn := time.Date(2026, 9, 28, 13, 0, 0, 0, art)
	consumerOwner := readmodel.OwnerConsumer
	ana := readmodel.Party{ID: 7, Name: "Ana", Surname: "Pérez"}
	juan := readmodel.Party{ID: 8, Name: "Juan", Surname: "Gómez"}
	service := new(serviceMock)
	service.On("Query", mock.Anything, operation.InboxQuery{}).Return(operation.InboxPage{Operations: []readmodel.OperationSummary{
		{
			ID: readmodel.ID{Kind: readmodel.KindServiceProposal, ResourceID: 34}, StartedOn: startedOn,
			Stage:      readmodel.StageWorkOrderAwaitingPayment,
			JobRequest: &readmodel.JobRequest{ID: 12, Status: jobrequest.StatusAccepted, CreatedOn: startedOn.Add(-time.Hour)},
			ServiceProposal: &readmodel.ServiceProposal{
				ID: 34, Status: serviceproposal.StatusAccepted, CreatedOn: startedOn, ScheduledOn: startedOn.Add(72 * time.Hour),
				EstimatedDurationMinutes: 120, BookingPaymentDeadline: startedOn.Add(48 * time.Hour),
			},
			WorkOrder: &readmodel.WorkOrder{
				ID: 56, Status: workorder.StatusAwaitingPayment, AcceptedOn: startedOn.Add(time.Hour), CompletionReportedOn: &reportedOn,
			},
			Consumer: ana, Provider: juan, Category: &readmodel.Category{ID: 3, Name: "Plomería"},
			Alerts: []readmodel.Alert{}, NextActionOwner: &consumerOwner,
		},
		{
			ID: readmodel.ID{Kind: readmodel.KindJobRequest, ResourceID: 13}, StartedOn: startedOn.Add(-time.Hour),
			Stage:      readmodel.StageProposalPending,
			JobRequest: &readmodel.JobRequest{ID: 13, Status: jobrequest.StatusAccepted, CreatedOn: startedOn.Add(-time.Hour)},
			ServiceProposal: &readmodel.ServiceProposal{
				ID: 35, Status: serviceproposal.StatusPending, CreatedOn: startedOn.Add(-time.Hour),
				ScheduledOn: startedOn.Add(12 * time.Hour), EstimatedDurationMinutes: 60, BookingPaymentDeadline: startedOn.Add(-12 * time.Hour),
			},
			Consumer: ana, Provider: juan,
			Alerts: []readmodel.Alert{readmodel.AlertBookingDeadlinePassed},
		},
	}}, nil).Once()

	response := httptest.NewRecorder()
	testRouter(service).ServeHTTP(response, httptest.NewRequest(http.MethodGet, "/admin/operations", nil))

	require.Equal(t, http.StatusOK, response.Code)
	require.JSONEq(t, `{"operations":[
		{"id":"sp-34","stage":"work_order_awaiting_payment","started_on":"2026-09-25T15:00:00Z",
		 "job_request":{"id":12,"status":"accepted","created_on":"2026-09-25T14:00:00Z"},
		 "service_proposal":{"id":34,"status":"accepted","created_on":"2026-09-25T15:00:00Z","scheduled_on":"2026-09-28T15:00:00Z",
		  "estimated_duration_minutes":120,"booking_payment_deadline":"2026-09-27T15:00:00Z"},
		 "work_order":{"id":56,"status":"awaiting_payment","accepted_on":"2026-09-25T16:00:00Z",
		  "completion_reported_on":"2026-09-28T16:00:00Z","balance_paid_on":null},
		 "consumer":{"id":7,"name":"Ana","surname":"Pérez"},"provider":{"id":8,"name":"Juan","surname":"Gómez"},
		 "category":{"id":3,"name":"Plomería"},"alerts":[],"next_action_owner":"consumer"},
		{"id":"jr-13","stage":"proposal_pending","started_on":"2026-09-25T14:00:00Z",
		 "job_request":{"id":13,"status":"accepted","created_on":"2026-09-25T14:00:00Z"},
		 "service_proposal":{"id":35,"status":"pending","created_on":"2026-09-25T14:00:00Z","scheduled_on":"2026-09-26T03:00:00Z",
		  "estimated_duration_minutes":60,"booking_payment_deadline":"2026-09-25T03:00:00Z"},"work_order":null,
		 "consumer":{"id":7,"name":"Ana","surname":"Pérez"},"provider":{"id":8,"name":"Juan","surname":"Gómez"},
		 "category":null,"alerts":["booking_deadline_passed"],"next_action_owner":null}
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
