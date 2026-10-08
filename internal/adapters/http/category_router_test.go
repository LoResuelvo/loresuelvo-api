package httpadapter

import (
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/LoResuelvo/loresuelvo-api/internal/adapters/auth0"
	"github.com/LoResuelvo/loresuelvo-api/internal/adapters/http/handler/category_handler"
	"github.com/LoResuelvo/loresuelvo-api/internal/adapters/http/middleware"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

func TestCategoryAdministrationRequiresSpecificPermissionsAndPrivateCaching(t *testing.T) {
	for _, route := range []struct{ method, path, permission string }{
		{"PATCH", "/categories/invalid", "write:categories"},
		{"GET", "/admin/categories/invalid/impact", "read:categories"},
		{"GET", "/categories?include_disabled=true", "read:categories"},
	} {
		modes := []string{"no token", "invalid token", "unrelated permission", "opposite permission"}
		if route.method == "PATCH" || strings.HasPrefix(route.path, "/admin/") {
			modes = append(modes, "exact permission")
		}
		for _, mode := range modes {
			t.Run(route.method+route.path+mode, func(t *testing.T) {
				router := NewRouter(RouterConfig{Auth0Validator: auth0.NewFakeValidator(), CategoryHandler: category_handler.NewCategoryHandler(nil)})
				auth, err := middleware.BaseAutheticationLayer(router.auth0Validator)
				require.NoError(t, err)
				gin.SetMode(gin.TestMode)
				engine := gin.New()
				router.registerCategoryRoutes(engine, auth)
				path := route.path
				request := httptest.NewRequest(route.method, path, strings.NewReader(`{}`))
				status := 403
				permissions := []string{"read:admin_operations", "create:categories"}
				switch mode {
				case "no token":
					status = 401
				case "invalid token":
					status = 401
					request.Header.Set("Authorization", "Bearer invalid")
				case "opposite permission":
					permissions = []string{"write:categories"}
					if route.permission == "write:categories" {
						permissions = []string{"read:categories"}
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

func TestOrdinaryCategoryCatalogDoesNotRequireAdministrativePermission(t *testing.T) {
	router := NewRouter(RouterConfig{Auth0Validator: auth0.NewFakeValidator(), CategoryHandler: category_handler.NewCategoryHandler(nil)})
	auth, err := middleware.BaseAutheticationLayer(router.auth0Validator)
	require.NoError(t, err)
	gin.SetMode(gin.TestMode)
	engine := gin.New()
	router.registerCategoryRoutes(engine, auth)
	// The invalid flag reaches input validation, not an administrative permission gate.
	request := httptest.NewRequest("GET", "/categories?include_disabled=invalid", nil)
	request.Header.Set("Authorization", "Bearer "+auth0.NewTokenBuilder().BuildToken("subject", nil))
	response := httptest.NewRecorder()
	engine.ServeHTTP(response, request)
	require.Equal(t, 400, response.Code, response.Body.String())
	require.Empty(t, response.Header().Get("Cache-Control"))
}
