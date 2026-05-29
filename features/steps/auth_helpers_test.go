package steps_test

import (
	"fmt"
	"strings"
)

func auth0IDForEmail(prefix, email string) string {
	replacer := strings.NewReplacer("@", "-", ".", "-", "+", "-", "_", "-")
	return fmt.Sprintf("auth0|%s-%s", prefix, replacer.Replace(strings.ToLower(email)))
}
