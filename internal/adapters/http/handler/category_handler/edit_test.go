package category_handler

import (
	"encoding/json"
	"errors"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/LoResuelvo/loresuelvo-api/internal/domain/category"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

func TestEditCategoryRejectsMalformedObjectsBeforePersistence(t *testing.T) {
	for _, body := range []string{
		`null`, `[]`, `{"name":null,"expected_version":1}`, `{"enabled":null,"expected_version":1}`, `{"reason":null,"expected_version":1}`, `{"confirm_ongoing_orders":null,"expected_version":1}`, `{"name":4,"expected_version":1}`, `{"enabled":"false","expected_version":1}`, `{"reason":42,"expected_version":1}`, `{"confirm_ongoing_orders":"true","expected_version":1}`, `{"name":"New","expected_version":"1"}`, `{"name":"New","expected_version":1.5}`, `{"name":"New","expected_version":null}`, `{"name":"New","expected_version":1,"id":2}`, `{"name":"New","expected_version":1,"normalized_name":"new"}`, `{"name":"One","name":"Two","expected_version":1}`, `{"name":"New"}`, `{"name":"New","expected_version":0}`, `{"name":"New","expected_version":-1}`, `{"expected_version":1}`, `{"name":"New","expected_version":1} {}`,
	} {
		t.Run(body, func(t *testing.T) {
			operators := new(categoryOperatorIDFinderMock)
			router := handlerTestRouter(nil, nil, operators, nil, nil)
			response := httptest.NewRecorder()
			router.ServeHTTP(response, httptest.NewRequest("PATCH", "/categories/1", strings.NewReader(body)))
			require.Equal(t, 400, response.Code, response.Body.String())
			operators.AssertNotCalled(t, "FindOperatorIDByAuthID", mock.Anything, mock.Anything)
		})
	}
}

func TestEditCategoryRejectsOversizedWhitespaceBeforePersistence(t *testing.T) {
	operators := new(categoryOperatorIDFinderMock)
	router := handlerTestRouter(nil, nil, operators, nil, nil)
	body := strings.Repeat(" ", 64*1024) + `{"name":"New","expected_version":1}`
	response := httptest.NewRecorder()
	router.ServeHTTP(response, httptest.NewRequest("PATCH", "/categories/1", strings.NewReader(body)))
	require.Equal(t, 400, response.Code)
	require.JSONEq(t, `{"error":"Invalid request body"}`, response.Body.String())
	operators.AssertNotCalled(t, "FindOperatorIDByAuthID", mock.Anything, mock.Anything)
}

func TestEditCategoryMapsPersistenceErrorsWithoutLeakingDetails(t *testing.T) {
	for _, test := range []struct {
		name    string
		err     error
		status  int
		message string
	}{
		{"missing", category.ErrDoesNotExist, 404, category.ErrDoesNotExist.Error()},
		{"stale version", category.ErrVersionConflict, 409, category.ErrVersionConflict.Error()},
		{"duplicate name", category.ErrAlreadyExists, 409, category.ErrAlreadyExists.Error()},
		{"confirmation", category.ErrConfirmationRequired, 409, category.ErrConfirmationRequired.Error()},
		{"database", errors.New("SELECT private_database_details"), 500, "Internal Server Error"},
	} {
		t.Run(test.name, func(t *testing.T) {
			operators := new(categoryOperatorIDFinderMock)
			uow := new(categoryUnitOfWorkMock)
			operators.On("FindOperatorIDByAuthID", mock.Anything, "admin").Return(7, nil).Once()
			uow.On("Execute", mock.Anything, mock.Anything).Return(test.err).Once()
			router := handlerTestRouter(nil, uow, operators, nil, nil)
			response := httptest.NewRecorder()
			router.ServeHTTP(response, httptest.NewRequest("PATCH", "/categories/1", strings.NewReader(`{"name":"New","expected_version":1}`)))
			require.Equal(t, test.status, response.Code)
			expected, err := json.Marshal(map[string]string{"error": test.message})
			require.NoError(t, err)
			require.JSONEq(t, string(expected), response.Body.String())
			operators.AssertExpectations(t)
			uow.AssertExpectations(t)
		})
	}
}

func TestEditCategoryReturnsAdministrativeShape(t *testing.T) {
	operators := new(categoryOperatorIDFinderMock)
	uow := new(categoryUnitOfWorkMock)
	store := new(categoryStoreMock)
	clock := new(categoryClockMock)
	operators.On("FindOperatorIDByAuthID", mock.Anything, "admin").Return(7, nil).Once()
	current := &category.Category{ID: 1, Name: "Plomería", NormalizedName: "plomería", Enabled: true, Version: 1}
	store.On("FindCategory", mock.Anything, 1).Return(current, nil).Once()
	updated := &category.Category{ID: 1, Name: "Instalaciones", NormalizedName: "instalaciones", Enabled: true, Version: 2}
	store.On("SaveCategory", mock.Anything, *updated).Return(updated, nil).Once()
	store.On("SaveAuditEvent", mock.Anything, mock.Anything).Return(nil).Once()
	clock.On("Now").Return(time.Date(2026, 7, 4, 12, 0, 0, 0, time.UTC)).Once()
	uow.On("Execute", mock.Anything, mock.Anything).Return(nil).Run(func(args mock.Arguments) {
		require.NoError(t, args.Get(1).(func(category.TransactionalStore) error)(store))
	}).Once()
	router := handlerTestRouter(nil, uow, operators, clock, nil)
	response := httptest.NewRecorder()
	router.ServeHTTP(response, httptest.NewRequest("PATCH", "/categories/1", strings.NewReader(`{"name":"Instalaciones","expected_version":1}`)))
	require.Equal(t, 200, response.Code, response.Body.String())
	require.JSONEq(t, `{"id":1,"name":"Instalaciones","enabled":true,"version":2}`, response.Body.String())
	operators.AssertExpectations(t)
	uow.AssertExpectations(t)
	store.AssertExpectations(t)
	clock.AssertExpectations(t)
}

func TestCategoryCatalogSeparatesOrdinaryAndAdministrativeProjections(t *testing.T) {
	for _, admin := range []bool{false, true} {
		t.Run(map[bool]string{false: "ordinary", true: "administrative"}[admin], func(t *testing.T) {
			repo := new(categoryRepositoryMock)
			repo.On("ListAll").Return([]category.Category{{ID: 1, Name: "Plomería", NormalizedName: "plomería", Enabled: true, Version: 2}, {ID: 2, Name: "Electricidad", Enabled: false, Version: 3}}, nil).Once()
			router := handlerTestRouter(repo, nil, nil, nil, nil)
			path := "/categories"
			if admin {
				path += "?include_disabled=true"
			}
			response := httptest.NewRecorder()
			router.ServeHTTP(response, httptest.NewRequest("GET", path, nil))
			require.Equal(t, 200, response.Code)
			if admin {
				require.JSONEq(t, `[{"id":1,"name":"Plomería","enabled":true,"version":2},{"id":2,"name":"Electricidad","enabled":false,"version":3}]`, response.Body.String())
				require.Equal(t, "private, no-store", response.Header().Get("Cache-Control"))
			} else {
				require.JSONEq(t, `[{"id":1,"name":"Plomería"}]`, response.Body.String())
			}
			repo.AssertExpectations(t)
		})
	}
}

func TestCategoryImpactReturnsOnlyAggregateContext(t *testing.T) {
	reader := new(categoryImpactReaderMock)
	clock := new(categoryClockMock)
	at := time.Date(2026, 7, 4, 12, 0, 0, 0, time.UTC)
	clock.On("Now").Return(at).Once()
	impact := &category.Impact{Category: category.Category{ID: 1, Name: "Plomería", Enabled: true, Version: 2}, ObservedAt: at, Counts: category.ImpactCounts{AssignedProviders: 3, PendingRequests: 4, AcceptedRequestsWithoutProposal: 5, PendingProposals: 6, ScheduledOrders: 7, AwaitingPaymentOrders: 8}}
	reader.On("FindByCategoryID", mock.Anything, 1, at).Return(impact, nil).Once()
	router := handlerTestRouter(nil, nil, nil, clock, reader)
	response := httptest.NewRecorder()
	router.ServeHTTP(response, httptest.NewRequest("GET", "/admin/categories/1/impact", nil))
	require.Equal(t, 200, response.Code)
	require.JSONEq(t, `{"category":{"id":1,"name":"Plomería","enabled":true,"version":2},"observed_at":"2026-07-04T12:00:00Z","counts":{"assigned_providers":3,"pending_requests":4,"accepted_requests_without_proposal":5,"pending_proposals":6,"scheduled_orders":7,"awaiting_payment_orders":8},"has_ongoing_orders":true,"requires_confirmation":true,"existing_operations_can_continue":true}`, response.Body.String())
	reader.AssertExpectations(t)
	clock.AssertExpectations(t)
}

func TestCategoryImpactMapsMissingAndDatabaseErrors(t *testing.T) {
	for _, test := range []struct {
		name    string
		err     error
		status  int
		message string
	}{
		{"missing", category.ErrDoesNotExist, 404, category.ErrDoesNotExist.Error()},
		{"database", errors.New("SELECT private_database_details"), 500, "Internal Server Error"},
	} {
		t.Run(test.name, func(t *testing.T) {
			reader := new(categoryImpactReaderMock)
			clock := new(categoryClockMock)
			at := time.Date(2026, 7, 4, 12, 0, 0, 0, time.UTC)
			clock.On("Now").Return(at).Once()
			reader.On("FindByCategoryID", mock.Anything, 1, at).Return(nil, test.err).Once()
			router := handlerTestRouter(nil, nil, nil, clock, reader)
			response := httptest.NewRecorder()
			router.ServeHTTP(response, httptest.NewRequest("GET", "/admin/categories/1/impact", nil))
			require.Equal(t, test.status, response.Code)
			expected, err := json.Marshal(map[string]string{"error": test.message})
			require.NoError(t, err)
			require.JSONEq(t, string(expected), response.Body.String())
			reader.AssertExpectations(t)
			clock.AssertExpectations(t)
		})
	}
}

func TestCategoryEndpointsRejectInvalidIDsBeforePersistence(t *testing.T) {
	for _, path := range []string{"/categories/0", "/categories/-1", "/categories/not-an-id", "/admin/categories/0/impact", "/admin/categories/-1/impact", "/admin/categories/not-an-id/impact"} {
		t.Run(path, func(t *testing.T) {
			method := "PATCH"
			if strings.HasPrefix(path, "/admin/") {
				method = "GET"
			}
			router := handlerTestRouter(nil, nil, nil, nil, nil)
			response := httptest.NewRecorder()
			router.ServeHTTP(response, httptest.NewRequest(method, path, strings.NewReader(`{"name":"New","expected_version":1}`)))
			require.Equal(t, 400, response.Code)
		})
	}
}

func TestCategoryCatalogRejectsInvalidAdministrativeFlagBeforePersistence(t *testing.T) {
	router := handlerTestRouter(nil, nil, nil, nil, nil)
	response := httptest.NewRecorder()
	router.ServeHTTP(response, httptest.NewRequest("GET", "/categories?include_disabled=invalid", nil))
	require.Equal(t, 400, response.Code)
}
