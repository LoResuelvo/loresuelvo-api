package auth0

import (
	"fmt"
	"net/url"
	"time"

	"github.com/auth0/go-jwt-middleware/v3/jwks"
	"github.com/auth0/go-jwt-middleware/v3/validator"
)

func NewValidator(domain, audience string) (*validator.Validator, error) {
	// Construct issuer URL (must include trailing slash)
	issuerURL, err := url.Parse("https://" + domain + "/")
	if err != nil {
		return nil, fmt.Errorf("failed to parse issuer URL: %w", err)
	}

	// Initialize JWKS provider using v3 options pattern
	provider, err := jwks.NewCachingProvider(
		jwks.WithIssuerURL(issuerURL),
		jwks.WithCacheTTL(5*time.Minute),
	)
	if err != nil {
		return nil, fmt.Errorf("failed to create JWKS provider: %w", err)
	}

	// Create validator using v3 options pattern
	jwtValidator, err := validator.New(
		validator.WithKeyFunc(provider.KeyFunc),
		validator.WithAlgorithm(validator.RS256),
		validator.WithIssuer(issuerURL.String()),
		validator.WithAudience(audience),
		validator.WithCustomClaims(func() validator.CustomClaims {
			return &CustomClaims{}
		}),
		validator.WithAllowedClockSkew(30*time.Second),
	)
	if err != nil {
		return nil, fmt.Errorf("failed to create validator: %w", err)
	}

	return jwtValidator, nil
}
