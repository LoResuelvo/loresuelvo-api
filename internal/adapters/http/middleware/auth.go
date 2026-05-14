package middleware

import (
	"log/slog"
	"net/http"

	jwtmiddleware "github.com/auth0/go-jwt-middleware/v3"
	"github.com/auth0/go-jwt-middleware/v3/validator"
	"github.com/gin-gonic/gin"
)

func NewMiddleware(jwtValidator *validator.Validator) (*jwtmiddleware.JWTMiddleware, error) {
	return jwtmiddleware.New(
		jwtmiddleware.WithValidator(jwtValidator),
		jwtmiddleware.WithValidateOnOptions(false),
		jwtmiddleware.WithErrorHandler(func(w http.ResponseWriter, r *http.Request, err error) {
			slog.Error("JWT validation failed", "error", err, "path", r.URL.Path)
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusUnauthorized)
			if _, err := w.Write([]byte(`{"message":"Failed to validate JWT."}`)); err != nil {
				slog.Error("Failed to write error response", "error", err)
			}
		}),
	)
}

func NewGinMiddleware(jwtMiddleware *jwtmiddleware.JWTMiddleware) gin.HandlerFunc {
	return func(c *gin.Context) {
		calledNext := false

		handler := jwtMiddleware.CheckJWT(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			calledNext = true
			c.Request = r
			c.Next()
		}))

		handler.ServeHTTP(c.Writer, c.Request)

		if !calledNext {
			c.Abort()
		}
	}
}
