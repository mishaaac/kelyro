package storage

import "testing"

func TestRedactRemovesSensitiveValues(t *testing.T) {
	t.Parallel()

	secret := "sk-example-sensitive-value"
	got := Redact("request with "+secret+" failed; token="+secret, secret)
	if got != "request with [REDACTED] failed; token=[REDACTED]" {
		t.Fatalf("Redact() = %q", got)
	}
}

func TestRedactHandlesOverlappingAndEmptyValues(t *testing.T) {
	t.Parallel()

	if got := Redact("token-extended token", "", "token", "token-extended"); got != "[REDACTED] [REDACTED]" {
		t.Fatalf("Redact() = %q", got)
	}
}
