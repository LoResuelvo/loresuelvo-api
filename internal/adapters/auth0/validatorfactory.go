package auth0

import (
	"fmt"
	"os"

	"github.com/auth0/go-jwt-middleware/v3/validator"
)

func NewValidatorFromEnv() (*validator.Validator, error) {
	domain := os.Getenv("AUTH0_DOMAIN")
	audience := os.Getenv("AUTH0_AUDIENCE")

	if domain == "" || audience == "" {
		return nil, fmt.Errorf("AUTH0_DOMAIN and AUTH0_AUDIENCE must be set")
	}

	return NewValidator(domain, audience)
}
