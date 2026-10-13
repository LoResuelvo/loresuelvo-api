package evals

import (
	"fmt"
	"sort"
	"strings"
)

// ParseEvaluationAPIKeys reads the eval-only comma-separated credential list.
// Error messages never echo credentials, even for malformed entries.
func ParseEvaluationAPIKeys(value string) ([]string, error) {
	parts := strings.Split(value, ",")
	keys := make([]string, 0, len(parts))
	seen := map[string]bool{}
	for _, part := range parts {
		key := strings.TrimSpace(part)
		if key == "" {
			return nil, fmt.Errorf("CHATBOT_EVALS_API_KEYS must contain nonempty comma-separated keys")
		}
		if seen[key] {
			return nil, fmt.Errorf("CHATBOT_EVALS_API_KEYS contains duplicate keys")
		}
		seen[key] = true
		keys = append(keys, key)
	}
	return keys, nil
}

func CredentialsRedactor(keys []string) func(string) string {
	ordered := append([]string(nil), keys...)
	sort.Slice(ordered, func(i, j int) bool { return len(ordered[i]) > len(ordered[j]) })
	redactors := make([]func(string) string, len(ordered))
	for i, key := range ordered {
		redactors[i] = CredentialRedactor(key)
	}
	return func(text string) string {
		for _, redact := range redactors {
			text = redact(text)
		}
		return text
	}
}
