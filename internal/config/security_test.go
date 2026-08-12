package config

import (
	"strings"
	"testing"
)

func TestSchemaContainsNoSecretValueFields(t *testing.T) {
	t.Parallel()

	for key := range Definitions() {
		lower := strings.ToLower(key)
		for _, forbidden := range []string{"secret", "password", "api_key", "token"} {
			if strings.Contains(lower, forbidden) {
				t.Errorf("configuration schema key %q may serialize a secret value", key)
			}
		}
	}
}
