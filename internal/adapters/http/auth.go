package httpadapter

import (
	"fmt"
	"net/http"
	"os"

	auth0adapter "github.com/LoResuelvo/loresuelvo-api/internal/adapters/auth0"
	"github.com/LoResuelvo/loresuelvo-api/internal/adapters/http/middleware"
	jwtmiddleware "github.com/auth0/go-jwt-middleware/v3"
	"github.com/auth0/go-jwt-middleware/v3/validator"
	"github.com/gin-gonic/gin"
	"github.com/joho/godotenv"
)

func NewAuth0MiddlewareFromEnv() (gin.HandlerFunc, error) {
	_ = godotenv.Load()

	domain := os.Getenv("AUTH0_DOMAIN")
	if domain == "" {
		return nil, fmt.Errorf("AUTH0_DOMAIN is required")
	}

	audience := os.Getenv("AUTH0_AUDIENCE")
	if audience == "" {
		return nil, fmt.Errorf("AUTH0_AUDIENCE is required")
	}

	jwtValidator, err := auth0adapter.NewValidator(domain, audience)
	if err != nil {
		return nil, err
	}

	jwtMiddleware, err := middleware.NewMiddleware(jwtValidator)
	if err != nil {
		return nil, err
	}

	return middleware.NewGinMiddleware(jwtMiddleware), nil
}

func AuthenticatedUser(c *gin.Context) {
	claims, err := jwtmiddleware.GetClaims[*validator.ValidatedClaims](c.Request.Context())
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"message": "Failed to get token claims."})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"message": "authenticated",
		"user_id": claims.RegisteredClaims.Subject,
	})
}
