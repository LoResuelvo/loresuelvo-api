package middleware

import (
	"net/http"
	"slices"

	"github.com/LoResuelvo/loresuelvo-api/internal/adapters/auth0"
	"github.com/LoResuelvo/loresuelvo-api/internal/observability"
	jwtmiddleware "github.com/auth0/go-jwt-middleware/v3"
	"github.com/auth0/go-jwt-middleware/v3/validator"
	"github.com/gin-gonic/gin"
)

const (
	ContextKeyUserID      = "userID"
	ContextKeyPermissions = "permissions"
	ContextKeyScope       = "scope"
)

// To get user id of authenticated user.
func GetUserID(c *gin.Context) (string, bool) {
	val, exists := c.Get(ContextKeyUserID)
	if !exists {
		return "", false
	}
	userID, ok := val.(string)
	return userID, ok
}

func newJwtMiddleware(jwtValidator *validator.Validator) (*jwtmiddleware.JWTMiddleware, error) {
	return jwtmiddleware.New(
		jwtmiddleware.WithValidator(jwtValidator),
		jwtmiddleware.WithValidateOnOptions(false),
		jwtmiddleware.WithErrorHandler(func(w http.ResponseWriter, r *http.Request, _ error) {
			observability.LoggerFromContext(r.Context()).WarnContext(r.Context(), "authentication denied")
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusUnauthorized)
			_, _ = w.Write([]byte(`{"error":"invalid_token","message":"Failed to validate JWT."}`))
		}),
	)
}

func BaseAutheticationLayer(jwtValidator *validator.Validator) (gin.HandlerFunc, error) {
	httpMiddleware, err := newJwtMiddleware(jwtValidator)
	if err != nil {
		return nil, err
	}

	return func(c *gin.Context) {
		calledNext := false

		handler := httpMiddleware.CheckJWT(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			claims, err := jwtmiddleware.GetClaims[*validator.ValidatedClaims](r.Context())
			if err != nil {
				c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "missing claims"})
				return
			}

			// Extraction of the subject, the one to save on DB
			c.Set(ContextKeyUserID, claims.RegisteredClaims.Subject)

			if custom, ok := claims.CustomClaims.(*auth0.CustomClaims); ok {
				// Extraction of custom claims, if they exist
				c.Set(ContextKeyPermissions, custom.Permissions)
				c.Set(ContextKeyScope, custom.Scope)
			}

			calledNext = true
			c.Request = r
			c.Next()
		}))

		handler.ServeHTTP(c.Writer, c.Request)
		if !calledNext {
			c.Abort()
		}
	}, nil
}

func RequirePermissionLayer(permission string) gin.HandlerFunc {
	return func(c *gin.Context) {
		val, exists := c.Get(ContextKeyPermissions)
		if !exists {
			c.AbortWithStatusJSON(http.StatusForbidden, gin.H{"error": "no permissions in context"})
			return
		}

		// Safe — we set this ourselves as []string, no outside casting
		permissions, ok := val.([]string)
		if !ok {
			c.AbortWithStatusJSON(http.StatusForbidden, gin.H{"error": "malformed permissions"})
			return
		}

		if slices.Contains(permissions, permission) {
			c.Next()
			return
		}

		observability.LoggerFromContext(c.Request.Context()).WarnContext(
			c.Request.Context(),
			"permission denied",
			"required_permission", permission,
			"http.route", c.FullPath(),
		)
		c.AbortWithStatusJSON(http.StatusForbidden, gin.H{"error": "insufficient permissions"})
	}
}
