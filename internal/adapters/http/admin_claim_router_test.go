package httpadapter

import (
	"io"
	"log/slog"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/LoResuelvo/loresuelvo-api/internal/adapters/auth0"
	"github.com/LoResuelvo/loresuelvo-api/internal/adapters/http/handler/claim_handler"
	"github.com/LoResuelvo/loresuelvo-api/internal/adapters/http/middleware"
	"github.com/LoResuelvo/loresuelvo-api/internal/domain/claim"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

func TestAdministrativeClaimRoutesRequireExactPermissionAndPrivateCaching(t *testing.T) {
	for _, route := range []struct{ method, path, permission string }{{"GET", "/admin/claims", readAdminClaimsPermission}, {"GET", "/admin/claims/1", readAdminClaimsPermission}, {"POST", "/admin/claims/1/review", writeAdminClaimsPermission}, {"POST", "/admin/claims/1/resolution", writeAdminClaimsPermission}} {
		for _, mode := range []string{"no token", "malformed token", "unrelated permission", "opposite permission", "authorized"} {
			t.Run(route.method+route.path+mode, func(t *testing.T) {
				svc := new(adminClaimServiceMock)
				config := RouterConfig{Auth0Validator: auth0.NewFakeValidator(), ClaimHandler: claim_handler.NewHandler(new(claimServiceMock)), AdminClaimHandler: claim_handler.NewAdminHandler(svc)}
				router := NewRouter(config)
				auth, err := middleware.BaseAutheticationLayer(router.auth0Validator)
				require.NoError(t, err)
				gin.SetMode(gin.TestMode)
				engine := gin.New()
				engine.Use(middleware.RequestLogger(slog.New(slog.NewTextHandler(io.Discard, nil))))
				router.registerClaimRoutes(engine, auth)
				req := httptest.NewRequest(route.method, route.path, strings.NewReader(`{"type":"agreement","reasoning":"valid"}`))
				req.Header.Set("Idempotency-Key", "550e8400-e29b-41d4-a716-446655440000")
				permissions := []string{"read:admin_audit", "read:admin_chat_audit", "read:admin_payments", "read:admin_operations"}
				expected := 403
				switch mode {
				case "no token":
					expected = 401
				case "malformed token":
					req.Header.Set("Authorization", "Bearer invalid")
					expected = 401
				case "opposite permission":
					if route.permission == readAdminClaimsPermission {
						permissions = []string{writeAdminClaimsPermission}
					} else {
						permissions = []string{readAdminClaimsPermission}
					}
				case "authorized":
					permissions = []string{route.permission}
					expected = 200
					switch route.path {
					case "/admin/claims":
						svc.On("List", mock.Anything, mock.Anything).Return(&claim.AdminPage{}, nil).Once()
					case "/admin/claims/1":
						svc.On("Get", mock.Anything, "subject", 1, mock.Anything).Return(&claim.AdminDetail{GetResult: claim.GetResult{Claim: &claim.Claim{ID: 1}}}, nil).Once()
					case "/admin/claims/1/review":
						req = httptest.NewRequest(route.method, route.path, nil)
						req.Header.Set("Idempotency-Key", "550e8400-e29b-41d4-a716-446655440000")
						svc.On("StartReview", mock.Anything, "subject", 1, mock.Anything, mock.Anything).Return(&claim.AdministrationResult{Claim: &claim.Claim{ID: 1}}, nil).Once()
					case "/admin/claims/1/resolution":
						svc.On("Resolve", mock.Anything, "subject", 1, mock.Anything, mock.Anything, mock.Anything).Return(&claim.AdministrationResult{Claim: &claim.Claim{ID: 1}}, nil).Once()
					}
				}
				if mode != "no token" && mode != "malformed token" {
					req.Header.Set("Authorization", "Bearer "+auth0.NewTokenBuilder().BuildToken("subject", permissions))
				}
				response := httptest.NewRecorder()
				engine.ServeHTTP(response, req)
				require.Equal(t, expected, response.Code, response.Body.String())
				require.Equal(t, "private, no-store", response.Header().Get("Cache-Control"))
				if expected != 200 {
					require.Empty(t, svc.Calls)
				}
				svc.AssertExpectations(t)
			})
		}
	}
}
