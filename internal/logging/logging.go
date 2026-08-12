// Package logging defines structured, presentation-neutral diagnostic logging.
package logging

import (
	"context"
	"sort"
	"strings"
)

// Level controls the severity and default visibility of a diagnostic entry.
type Level string

const (
	Debug Level = "debug"
	Info  Level = "info"
	Warn  Level = "warn"
	Error Level = "error"
)

const (
	Redacted = "[REDACTED]"
	Omitted  = "[OMITTED]"
)

// Entry is a structured diagnostic record. Sensitive contains values that an
// adapter must remove before serialization and is never itself persisted.
type Entry struct {
	Level         Level
	Message       string
	Operation     string
	Workspace     string
	Component     string
	ErrorCategory string
	Fields        map[string]string
	Sensitive     []string
}

// Logger writes diagnostic entries without coupling callers to a file format.
type Logger interface {
	Log(ctx context.Context, entry Entry) error
	Close() error
}

// WorkspaceFactory opens a logger rooted in one initialized workspace.
type WorkspaceFactory interface {
	Open(workspaceRoot string, verbose bool) (Logger, error)
	Path(workspaceRoot string) (string, error)
}

// Valid reports whether a level is supported by Foundation logging.
func (level Level) Valid() bool {
	switch level {
	case Debug, Info, Warn, Error:
		return true
	default:
		return false
	}
}

// Sanitize removes explicitly supplied sensitive values, secret-like fields,
// and fields that could contain complete student-authored content.
func Sanitize(entry Entry) Entry {
	entry.Message = redactValues(entry.Message, entry.Sensitive)
	entry.Operation = redactValues(entry.Operation, entry.Sensitive)
	entry.Workspace = redactValues(entry.Workspace, entry.Sensitive)
	entry.Component = redactValues(entry.Component, entry.Sensitive)
	entry.ErrorCategory = redactValues(entry.ErrorCategory, entry.Sensitive)
	entry.Fields = sanitizeFields(entry.Fields, entry.Sensitive)
	entry.Sensitive = nil
	return entry
}

func sanitizeFields(fields map[string]string, sensitive []string) map[string]string {
	if len(fields) == 0 {
		return nil
	}
	result := make(map[string]string, len(fields))
	for key, value := range fields {
		normalized := strings.ToLower(strings.ReplaceAll(strings.ReplaceAll(key, "-", "_"), ".", "_"))
		switch {
		case containsAny(normalized, "secret", "token", "password", "credential", "api_key", "apikey"):
			result[key] = Redacted
		case containsAny(normalized, "student_content", "document_content", "submission", "answer", "prompt") || normalized == "content":
			result[key] = Omitted
		default:
			result[key] = redactValues(value, sensitive)
		}
	}
	return result
}

func containsAny(value string, candidates ...string) bool {
	for _, candidate := range candidates {
		if strings.Contains(value, candidate) {
			return true
		}
	}
	return false
}

func redactValues(text string, sensitive []string) string {
	values := append([]string(nil), sensitive...)
	sort.Slice(values, func(left, right int) bool { return len(values[left]) > len(values[right]) })
	for _, value := range values {
		if value != "" {
			text = strings.ReplaceAll(text, value, Redacted)
		}
	}
	return text
}
