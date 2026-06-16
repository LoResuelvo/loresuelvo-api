package auth0

import (
	"fmt"
	"os"
	"strings"

	"github.com/auth0/go-jwt-middleware/v3/validator"
)

const devSwaggerAuthID = "auth0|swagger-dev"

func NewValidatorFromEnv() (*validator.Validator, error) {
	domain := strings.TrimSpace(os.Getenv("AUTH0_DOMAIN"))
	audience := strings.TrimSpace(os.Getenv("AUTH0_AUDIENCE"))
	environment := strings.ToLower(strings.TrimSpace(os.Getenv("ENVIRONMENT")))

	switch environment {
	case "dev":
		if domain == "" || audience == "" {
			printFakeValidatorSwaggerToken(devSwaggerAuthID)
			return NewFakeValidator(), nil
		}
		return NewValidator(domain, audience)
	case "staging", "production":
		if domain == "" || audience == "" {
			return nil, fmt.Errorf("missing AUTH0_DOMAIN or AUTH0_AUDIENCE environment variable")
		}
		return NewValidator(domain, audience)
	default:
		return nil, fmt.Errorf("unsupported environment: %s", environment)
	}
}

func printFakeValidatorSwaggerToken(authID string) {
	token := NewTokenBuilder().BuildToken(authID, nil)
	fmt.Printf(`
[auth0] using fake JWT validator because ENVIRONMENT=dev and Auth0 credentials are incomplete.
[auth0] Swagger dev auth_id: %s
[auth0] Swagger dev Bearer token (copy this token into Swagger Authorize, without the "Bearer " prefix):
%s

`, authID, token)
}
