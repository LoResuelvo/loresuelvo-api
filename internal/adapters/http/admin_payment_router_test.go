package httpadapter

import (
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/LoResuelvo/loresuelvo-api/internal/adapters/auth0"
	"github.com/LoResuelvo/loresuelvo-api/internal/adapters/http/handler/admin_payment_handler"
	"github.com/LoResuelvo/loresuelvo-api/internal/adapters/http/handler/payment_handler"
	"github.com/LoResuelvo/loresuelvo-api/internal/adapters/http/middleware"
	"github.com/LoResuelvo/loresuelvo-api/internal/domain/consumer"
	coveragezone "github.com/LoResuelvo/loresuelvo-api/internal/domain/coverage_zone"
	"github.com/LoResuelvo/loresuelvo-api/internal/domain/payment"
	readmodel "github.com/LoResuelvo/loresuelvo-api/internal/domain/payment/read_model"
	serviceproposal "github.com/LoResuelvo/loresuelvo-api/internal/domain/service_proposal"
	"github.com/LoResuelvo/loresuelvo-api/internal/domain/user"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

func TestAdminPaymentRouteRequiresItsPermissionAndDisablesCachingBeforeAuth(t *testing.T) {
	gin.SetMode(gin.TestMode)
	service := &adminPaymentQueryServiceMock{}
	service.On("Query", mock.Anything, payment.AdminPaymentQuery{Limit: 20}, "auth0|operator", mock.Anything).Return(&readmodel.AdminPaymentPage{Payments: []readmodel.AdminPayment{}, Limit: 20}, nil).Once()
	handler, err := admin_payment_handler.NewHandler(service, []byte("admin-payment-cursor-signing-key-for-tests"))
	require.NoError(t, err)
	router := NewRouter(RouterConfig{Auth0Validator: auth0.NewFakeValidator(), AdminPaymentHandler: handler})
	authentication, err := router.middlewareSetup()
	require.NoError(t, err)
	engine := gin.New()
	engine.Use(middleware.RequestLogger(slog.New(slog.NewTextHandler(io.Discard, nil))))
	router.registerAdminRoutes(engine, authentication)

	for _, test := range []struct {
		name       string
		token      string
		wantStatus int
	}{
		{name: "missing authentication", wantStatus: http.StatusUnauthorized},
		{name: "consumer has no admin permission", token: auth0.NewTokenBuilder().BuildToken("auth0|consumer", nil), wantStatus: http.StatusForbidden},
	} {
		t.Run(test.name, func(t *testing.T) {
			request := httptest.NewRequest(http.MethodGet, "/admin/payments?external_payment_id=secret", nil)
			if test.token != "" {
				request.Header.Set("Authorization", "Bearer "+test.token)
			}
			response := httptest.NewRecorder()
			engine.ServeHTTP(response, request)
			require.Equal(t, test.wantStatus, response.Code)
			require.Equal(t, "private, no-store", response.Header().Get("Cache-Control"))
		})
	}

	request := httptest.NewRequest(http.MethodGet, "/admin/payments", nil)
	request.Header.Set("Authorization", "Bearer "+auth0.NewTokenBuilder().BuildToken("auth0|operator", []string{"read:admin_payments"}))
	response := httptest.NewRecorder()
	engine.ServeHTTP(response, request)
	require.Equal(t, http.StatusOK, response.Code)
	require.Equal(t, "private, no-store", response.Header().Get("Cache-Control"))
	service.AssertExpectations(t)
}

func TestAdminPaymentPermissionDoesNotAuthorizeParticipantIntentRead(t *testing.T) {
	gin.SetMode(gin.TestMode)
	intentID := "123e4567-e89b-12d3-a456-426614174000"
	owner := consumer.RehydrateConsumer(
		user.RehydrateBaseUser(2, "auth0|intent-owner", "owner@example.com", "Intent", "Owner", consumer.Role, nil),
		consumer.Address{}, consumer.GeoPoint{}, coveragezone.CoverageZone{},
	)
	proposal := &serviceproposal.ServiceProposal{ID: 22, Consumer: owner}
	intent := &payment.Intent{ID: intentID, ServiceProposalID: proposal.ID}

	for _, test := range []struct {
		name, subject, role string
	}{
		{name: "operator with admin payment permission", subject: "auth0|operator", role: "admin"},
		{name: "different consumer", subject: "auth0|other-consumer", role: consumer.Role},
	} {
		t.Run(test.name, func(t *testing.T) {
			userFinder := &paymentUserFinderMock{}
			requester := user.RehydrateBaseUser(3, test.subject, "requester@example.com", "Request", "User", test.role, nil)
			userFinder.On("FindByAuthID", test.subject).Return(requester, nil).Once()
			intentRepository := &paymentIntentRepositoryMock{}
			intentRepository.On("FindByID", mock.Anything, intentID).Return(intent, nil).Once()
			proposalFinder := &serviceProposalFinderMock{}
			proposalFinder.On("FindByID", mock.Anything, proposal.ID).Return(proposal, nil).Once()
			service := payment.NewService(intentRepository, nil, proposalFinder, nil, userFinder, nil, nil, nil, nil, nil, nil, nil, nil, nil)
			router := NewRouter(RouterConfig{Auth0Validator: auth0.NewFakeValidator(), PaymentHandler: payment_handler.NewPaymentHandler(service, nil)})
			authentication, err := router.middlewareSetup()
			require.NoError(t, err)
			engine := gin.New()
			router.registerPaymentRoutes(engine, authentication)
			token := auth0.NewTokenBuilder().BuildToken(test.subject, []string{"read:admin_payments"})
			request := httptest.NewRequest(http.MethodGet, "/payment-intents/"+intentID, nil)
			request.Header.Set("Authorization", "Bearer "+token)
			response := httptest.NewRecorder()
			engine.ServeHTTP(response, request)
			require.Equal(t, http.StatusForbidden, response.Code)
			require.NotContains(t, response.Body.String(), intentID)
			userFinder.AssertExpectations(t)
			intentRepository.AssertExpectations(t)
			proposalFinder.AssertExpectations(t)
		})
	}
}
