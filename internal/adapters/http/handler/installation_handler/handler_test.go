package installation_handler

import (
	"errors"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/LoResuelvo/loresuelvo-api/internal/domain/installation"
	"github.com/google/uuid"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

func TestInstallationEndpointsRequireJWT(t *testing.T) {
	engine, _ := handlerEngine(t, nil)
	for _, method := range []string{"PUT", "DELETE"} {
		for _, token := range []string{"", "invalid-token"} {
			request := httptest.NewRequest(method, "/installations/"+uuid.NewString(), strings.NewReader(`{}`))
			if token != "" {
				request.Header.Set("Authorization", "Bearer "+token)
			}
			response := httptest.NewRecorder()
			engine.ServeHTTP(response, request)
			require.Equal(t, 401, response.Code)
		}
	}
}
func TestRegistrationUsesJWTActorAndOmitsCredentialsFromResponse(t *testing.T) {
	for _, created := range []bool{true, false} {
		t.Run(map[bool]string{true: "created", false: "renewed"}[created], func(t *testing.T) {
			id, binding := uuid.NewString(), uuid.NewString()
			service := new(serviceMock)
			service.On("Register", mock.Anything, "authenticated-actor", mock.MatchedBy(func(r installation.Registration) bool {
				return r.ID == id && r.App == "consumer" && r.Token == "secret-fcm-token" && r.Secret == "installation-proof" && r.BindingID == binding
			})).Return(&installation.Installation{ID: id, App: "consumer", Locale: "es", BindingID: binding, Enabled: true, Token: "secret-fcm-token", SecretHash: []byte("private-hash")}, created, nil).Once()
			engine, token := handlerEngine(t, service)
			request := httptest.NewRequest("PUT", "/installations/"+id, strings.NewReader(`{"user_id":999,"installation_secret":"installation-proof","app":"consumer","fcm_token":"secret-fcm-token","binding_id":"`+binding+`"}`))
			request.Header.Set("Content-Type", "application/json")
			request.Header.Set("Authorization", "Bearer "+token)
			response := httptest.NewRecorder()
			engine.ServeHTTP(response, request)
			status := 200
			if created {
				status = 201
				require.Equal(t, "/installations/"+id, response.Header().Get("Location"))
			}
			require.Equal(t, status, response.Code)
			for _, secret := range []string{"secret-fcm-token", "installation-proof", "private-hash", "user_id", "secret_hash", "fcm_token", "installation_secret"} {
				require.NotContains(t, response.Body.String(), secret)
			}
			require.JSONEq(t, `{"installation_id":"`+id+`","binding_id":"`+binding+`","app":"consumer","locale":"es","enabled":true}`, response.Body.String())
			service.AssertExpectations(t)
		})
	}
}
func TestRegistrationMapsDomainErrorsWithoutPrivateDetails(t *testing.T) {
	for _, test := range []struct {
		err    error
		status int
	}{{installation.ErrInvalidInstallation, 400}, {installation.ErrForbidden, 403}, {installation.ErrConflict, 409}, {errors.New("private-token-database-detail"), 500}} {
		t.Run(test.err.Error(), func(t *testing.T) {
			service := new(serviceMock)
			service.On("Register", mock.Anything, mock.Anything, mock.Anything).Return(nil, false, test.err).Once()
			engine, token := handlerEngine(t, service)
			request := httptest.NewRequest("PUT", "/installations/"+uuid.NewString(), strings.NewReader(`{}`))
			request.Header.Set("Content-Type", "application/json")
			request.Header.Set("Authorization", "Bearer "+token)
			response := httptest.NewRecorder()
			engine.ServeHTTP(response, request)
			require.Equal(t, test.status, response.Code)
			require.NotContains(t, response.Body.String(), "private-token-database-detail")
			service.AssertExpectations(t)
		})
	}
}
func TestInstallationRejectsMalformedJSONBeforeCallingService(t *testing.T) {
	service := new(serviceMock)
	engine, token := handlerEngine(t, service)
	for _, method := range []string{"PUT", "DELETE"} {
		request := httptest.NewRequest(method, "/installations/"+uuid.NewString(), strings.NewReader(`{"fcm_token":`))
		request.Header.Set("Content-Type", "application/json")
		request.Header.Set("Authorization", "Bearer "+token)
		response := httptest.NewRecorder()
		engine.ServeHTTP(response, request)
		require.Equal(t, 400, response.Code)
	}
	service.AssertNotCalled(t, "Register", mock.Anything, mock.Anything, mock.Anything)
	service.AssertNotCalled(t, "Unregister", mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything)
}
func TestUnregisterPassesActorProofAndBinding(t *testing.T) {
	id, secret, binding := uuid.NewString(), uuid.NewString(), uuid.NewString()
	service := new(serviceMock)
	service.On("Unregister", mock.Anything, "authenticated-actor", id, secret, binding).Return(nil).Once()
	engine, token := handlerEngine(t, service)
	request := httptest.NewRequest("DELETE", "/installations/"+id, strings.NewReader(`{"installation_secret":"`+secret+`","binding_id":"`+binding+`"}`))
	request.Header.Set("Content-Type", "application/json")
	request.Header.Set("Authorization", "Bearer "+token)
	response := httptest.NewRecorder()
	engine.ServeHTTP(response, request)
	require.Equal(t, 204, response.Code)
	require.Empty(t, response.Body.String())
	service.AssertExpectations(t)
}

func TestInstallationRejectsOversizedBodyBeforeCallingService(t *testing.T) {
	service := new(serviceMock)
	engine, token := handlerEngine(t, service)
	for _, method := range []string{"PUT", "DELETE"} {
		request := httptest.NewRequest(method, "/installations/"+uuid.NewString(), strings.NewReader(`{"installation_secret":"`+strings.Repeat("s", 8192)+`"}`))
		request.Header.Set("Content-Type", "application/json")
		request.Header.Set("Authorization", "Bearer "+token)
		response := httptest.NewRecorder()
		engine.ServeHTTP(response, request)
		require.Equal(t, 400, response.Code)
		require.JSONEq(t, `{"error":"invalid installation request"}`, response.Body.String())
	}
	service.AssertNotCalled(t, "Register", mock.Anything, mock.Anything, mock.Anything)
	service.AssertNotCalled(t, "Unregister", mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything)
}
