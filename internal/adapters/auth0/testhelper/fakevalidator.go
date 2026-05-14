package testhelper

import (
	"context"
	"testing"
	"time"

	"github.com/LoResuelvo/loresuelvo-api/internal/adapters/auth0"
	"github.com/auth0/go-jwt-middleware/v3/validator"
	"github.com/lestrrat-go/jwx/v3/jwa"
	"github.com/lestrrat-go/jwx/v3/jwt"
)

func NewTestValidator(tb testing.TB) *validator.Validator {
	secret := []byte("test-secret-do-not-use-in-production")

	v, err := validator.New(
		validator.WithKeyFunc(func(ctx context.Context) (any, error) {
			return secret, nil
		}),
		validator.WithAlgorithm(validator.HS256),
		validator.WithIssuer("test-issuer"),
		validator.WithAudience("test-audience"),
		validator.WithCustomClaims(func() validator.CustomClaims {
			return &auth0.CustomClaims{}
		}),
	)
	if err != nil {
		tb.Fatal("failed to build test validator:", err)
	}

	return v
}

// TokenBuilder generates signed tokens for use in acceptance test steps
type TokenBuilder struct {
	secret []byte
}

func NewTokenBuilder() *TokenBuilder {
	return &TokenBuilder{secret: []byte("test-secret-do-not-use-in-production")}
}

func (tb *TokenBuilder) BuildToken(userID string, permissions []string) string {
	token := jwt.New()
	_ = token.Set(jwt.SubjectKey, userID)
	_ = token.Set(jwt.IssuerKey, "test-issuer")
	_ = token.Set(jwt.AudienceKey, []string{"test-audience"})
	_ = token.Set("permissions", permissions)
	_ = token.Set(jwt.ExpirationKey, time.Now().Add(time.Hour))

	signed, _ := jwt.Sign(token, jwt.WithKey(jwa.HS256(), tb.secret))
	return string(signed)
}
