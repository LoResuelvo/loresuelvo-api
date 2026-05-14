package auth0

import (
	"context"
	"fmt"
	"slices"
	"strings"
)

// CustomClaims contains custom data we want to parse from the JWT.
type CustomClaims struct {
	Scope       string   `json:"scope"`
	Permissions []string `json:"permissions"`
}

// Validate ensures the custom claims are properly formatted.
func (c *CustomClaims) Validate(ctx context.Context) error {
	if c.Scope == "" {
		return nil
	}

	if strings.TrimSpace(c.Scope) != c.Scope {
		return fmt.Errorf("scope claim has invalid whitespace")
	}

	if strings.Contains(c.Scope, "  ") {
		return fmt.Errorf("scope claim contains double spaces")
	}

	return nil
}

// HasScope checks whether our claims have a specific scope.
func (c *CustomClaims) HasScope(expectedScope string) bool {
	if c.Scope == "" {
		return false
	}

	scopes := strings.SplitSeq(c.Scope, " ")
	for scope := range scopes {
		if scope == expectedScope {
			return true
		}
	}
	return false
}

// HasPermission checks whether our claims include a specific Auth0 RBAC permission.
func (c *CustomClaims) HasPermission(expectedPermission string) bool {
	return slices.Contains(c.Permissions, expectedPermission)
}
