package httpadapter

import (
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/LoResuelvo/loresuelvo-api/internal/adapters/auth0"
	"github.com/LoResuelvo/loresuelvo-api/internal/adapters/http/handler/admin_review_handler"
	"github.com/LoResuelvo/loresuelvo-api/internal/adapters/http/middleware"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

func TestAdminReviewRoutesRequireSpecificPermissionsAndPrivateCaching(t *testing.T) {
	for _, route := range []struct{ method, path, permission string }{
		{"GET", "/admin/reviews?limit=0", "read:admin_reviews"},
		{"GET", "/admin/reviews/invalid", "read:admin_reviews"},
		{"POST", "/admin/reviews/1/moderate", "write:admin_reviews"},
	} {
		for _, mode := range []string{"no token", "invalid token", "unrelated permission", "opposite permission", "exact permission"} {
			t.Run(route.method+route.path+mode, func(t *testing.T) {
				router := NewRouter(RouterConfig{Auth0Validator: auth0.NewFakeValidator(), AdminReviewHandler: admin_review_handler.New(nil)})
				auth, err := middleware.BaseAutheticationLayer(router.auth0Validator)
				require.NoError(t, err)
				gin.SetMode(gin.TestMode)
				engine := gin.New()
				router.registerAdminReviewRoutes(engine, auth)
				request := httptest.NewRequest(route.method, route.path, strings.NewReader(`{}`))
				status := 403
				permissions := []string{"read:admin_operations", "read:providers", "read:admin_audit"}
				switch mode {
				case "no token":
					status = 401
				case "invalid token":
					status = 401
					request.Header.Set("Authorization", "Bearer invalid")
				case "opposite permission":
					permissions = []string{"write:admin_reviews"}
					if route.permission == "write:admin_reviews" {
						permissions = []string{"read:admin_reviews"}
					}
				case "exact permission":
					permissions = []string{route.permission}
					status = 400
				}
				if mode != "no token" && mode != "invalid token" {
					request.Header.Set("Authorization", "Bearer "+auth0.NewTokenBuilder().BuildToken("subject", permissions))
				}
				response := httptest.NewRecorder()
				engine.ServeHTTP(response, request)
				require.Equal(t, status, response.Code, response.Body.String())
				require.Equal(t, "private, no-store", response.Header().Get("Cache-Control"))
			})
		}
	}
}
