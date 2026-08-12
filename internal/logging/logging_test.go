package logging

import (
	"strings"
	"testing"
)

func TestSanitizeRedactsSecretsAndStudentContent(t *testing.T) {
	t.Parallel()
	secret := "sk-sensitive-value"
	entry := Sanitize(Entry{
		Message:   "provider rejected " + secret,
		Workspace: "/work/" + secret,
		Fields: map[string]string{
			"api_token":       secret,
			"student_content": "complete essay",
			"detail":          "request " + secret,
		},
		Sensitive: []string{secret},
	})
	encoded := entry.Message + entry.Workspace + entry.Fields["api_token"] + entry.Fields["student_content"] + entry.Fields["detail"]
	if strings.Contains(encoded, secret) || strings.Contains(encoded, "complete essay") {
		t.Fatalf("sanitized entry exposed protected data: %#v", entry)
	}
	if entry.Fields["api_token"] != Redacted || entry.Fields["student_content"] != Omitted || entry.Sensitive != nil {
		t.Fatalf("sanitized entry = %#v", entry)
	}
}

func TestFoundationLogLevelsAreExplicit(t *testing.T) {
	t.Parallel()
	for _, level := range []Level{Debug, Info, Warn, Error} {
		if !level.Valid() {
			t.Errorf("level %q is invalid", level)
		}
	}
	if Level("trace").Valid() {
		t.Fatal("unexpected trace level accepted")
	}
}
