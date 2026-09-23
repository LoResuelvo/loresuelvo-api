package admin_handler

import (
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/LoResuelvo/loresuelvo-api/internal/domain/admin"
	readmodel "github.com/LoResuelvo/loresuelvo-api/internal/domain/admin/read_model"
	"github.com/LoResuelvo/loresuelvo-api/internal/domain/identityverification"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
)

func TestAdminHandlerReturnsInternalErrorWhenConsumerDirectoryFails(t *testing.T) {
	service := new(serviceMock)
	service.On("ListConsumers", mock.Anything, "").Return(nil, errors.New("database unavailable")).Once()
	handler := NewAdminHandler(service)
	context, recorder := adminHandlerTestContext(http.MethodGet, "/admin/consumers")

	handler.ListConsumers(context)

	assert.Equal(t, http.StatusInternalServerError, recorder.Code)
	assert.JSONEq(t, `{"error":"internal server error"}`, recorder.Body.String())
	service.AssertExpectations(t)
}

func TestAdminHandlerReturnsInternalErrorWhenProviderDirectoryFails(t *testing.T) {
	service := new(serviceMock)
	service.On("ListProviders", mock.Anything, admin.ProviderDirectoryFilter{}).Return(nil, errors.New("database unavailable")).Once()
	handler := NewAdminHandler(service)
	context, recorder := adminHandlerTestContext(http.MethodGet, "/admin/providers")

	handler.ListProviders(context)

	assert.Equal(t, http.StatusInternalServerError, recorder.Code)
	assert.JSONEq(t, `{"error":"internal server error"}`, recorder.Body.String())
	service.AssertExpectations(t)
}

func adminHandlerTestContext(method, path string) (*gin.Context, *httptest.ResponseRecorder) {
	gin.SetMode(gin.TestMode)
	recorder := httptest.NewRecorder()
	context, _ := gin.CreateTestContext(recorder)
	context.Request = httptest.NewRequest(method, path, nil)
	return context, recorder
}

func TestAdminHandlerPassesConsumerSearchQuery(t *testing.T) {
	service := new(serviceMock)
	service.On("ListConsumers", mock.Anything, "PÉREZ").Return([]readmodel.Consumer{}, nil).Once()
	handler := NewAdminHandler(service)
	context, recorder := adminHandlerTestContext(http.MethodGet, "/admin/consumers?q=P%C3%89REZ")

	handler.ListConsumers(context)

	assert.Equal(t, http.StatusOK, recorder.Code)
	assert.JSONEq(t, `[]`, recorder.Body.String())
	service.AssertExpectations(t)
}

func TestAdminHandlerPassesProviderSearchQuery(t *testing.T) {
	service := new(serviceMock)
	service.On("ListProviders", mock.Anything, admin.ProviderDirectoryFilter{Query: "GÓMEZ"}).Return([]readmodel.Provider{}, nil).Once()
	handler := NewAdminHandler(service)
	context, recorder := adminHandlerTestContext(http.MethodGet, "/admin/providers?q=G%C3%93MEZ")

	handler.ListProviders(context)

	assert.Equal(t, http.StatusOK, recorder.Code)
	assert.JSONEq(t, `[]`, recorder.Body.String())
	service.AssertExpectations(t)
}

func TestAdminHandlerPassesCombinedProviderFilter(t *testing.T) {
	categoryID, coverageZoneID := 2, 6
	status := identityverification.StatusApproved
	filter := admin.ProviderDirectoryFilter{
		Query: "JUAN", CategoryID: &categoryID, CoverageZoneID: &coverageZoneID,
		IdentityVerificationStatus: &status,
	}
	service := new(serviceMock)
	service.On("ListProviders", mock.Anything, filter).Return([]readmodel.Provider{}, nil).Once()
	handler := NewAdminHandler(service)
	context, recorder := adminHandlerTestContext(http.MethodGet,
		"/admin/providers?q=JUAN&category_id=2&coverage_zone_id=6&identity_verification_status=approved")

	handler.ListProviders(context)

	assert.Equal(t, http.StatusOK, recorder.Code)
	assert.JSONEq(t, `[]`, recorder.Body.String())
	service.AssertExpectations(t)
}
