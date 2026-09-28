package httpadapter

import (
	"github.com/LoResuelvo/loresuelvo-api/internal/adapters/auth0"
	"github.com/LoResuelvo/loresuelvo-api/internal/adapters/http/handler/operation_chat_handler"
	"github.com/LoResuelvo/loresuelvo-api/internal/adapters/http/middleware"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestOperationChatRouteRequiresItsOwnPermissionAndAlwaysPreventsCaching(t *testing.T) {
	gin.SetMode(gin.TestMode)
	router := NewRouter(RouterConfig{Auth0Validator: auth0.NewFakeValidator(), OperationChatHandler: operation_chat_handler.NewHandler(nil)})
	authentication, err := router.middlewareSetup()
	require.NoError(t, err)
	engine := gin.New()
	engine.Use(middleware.RequestLogger(slog.New(slog.NewTextHandler(io.Discard, nil))))
	router.registerAdminRoutes(engine, authentication)
	builder := auth0.NewTokenBuilder()
	for _, test := range []struct {
		name, token string
		status      int
	}{{"no token", "", 401}, {"invalid token", "invalid", 401}, {"operations permission", builder.BuildToken("auth0|support", []string{readAdminOperationsPermission}), 403}, {"chat permission reaches handler", builder.BuildToken("auth0|support", []string{readAdminChatAuditPermission}), 400}} {
		t.Run(test.name, func(t *testing.T) {
			request := httptest.NewRequest(http.MethodGet, "/admin/operations/jr-1/conversation?unsupported=1", nil)
			if test.token != "" {
				request.Header.Set("Authorization", "Bearer "+test.token)
			}
			response := httptest.NewRecorder()
			engine.ServeHTTP(response, request)
			require.Equal(t, test.status, response.Code)
			require.Equal(t, "private, no-store", response.Header().Get("Cache-Control"))
			require.NotContains(t, response.Body.String(), "messages")
		})
	}
}
