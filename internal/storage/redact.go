package storage

import (
	"sort"
	"strings"
)

const redacted = "[REDACTED]"

// Redact replaces every non-empty sensitive value in text. Longer values are
// replaced first so overlapping tokens cannot leave a suffix behind.
func Redact(text string, sensitive ...string) string {
	values := make([]string, 0, len(sensitive))
	seen := make(map[string]struct{}, len(sensitive))
	for _, value := range sensitive {
		if value == "" {
			continue
		}
		if _, exists := seen[value]; exists {
			continue
		}
		seen[value] = struct{}{}
		values = append(values, value)
	}
	sort.Slice(values, func(left, right int) bool {
		return len(values[left]) > len(values[right])
	})
	for _, value := range values {
		text = strings.ReplaceAll(text, value, redacted)
	}
	return text
}
