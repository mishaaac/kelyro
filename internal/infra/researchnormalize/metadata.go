package researchnormalize

import (
	"strings"
	"time"

	"github.com/mishaaac/kelyro/internal/research"
)

var timestampLayouts = []string{
	time.RFC3339Nano,
	time.RFC3339,
	time.RFC1123,
	time.RFC1123Z,
	time.RFC822,
	time.RFC822Z,
	"2006-01-02",
}

func parseDocumentTimestamp(value string) *research.Timestamp {
	value = strings.TrimSpace(value)
	for _, layout := range timestampLayouts {
		parsed, err := time.Parse(layout, value)
		if err != nil {
			continue
		}
		timestamp, err := research.NewTimestamp(parsed)
		if err == nil {
			return &timestamp
		}
	}
	return nil
}

func (builder *documentBuilder) setPublished(value string) {
	if builder.result.PublishedAt == nil {
		builder.result.PublishedAt = parseDocumentTimestamp(value)
	}
}

func (builder *documentBuilder) setUpdated(value string) {
	if builder.result.UpdatedAt == nil {
		builder.result.UpdatedAt = parseDocumentTimestamp(value)
	}
}
