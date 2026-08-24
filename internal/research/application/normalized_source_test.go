package application_test

import (
	"strings"
	"testing"

	"github.com/mishaaac/kelyro/internal/research/application"
)

func TestNormalizedSourceValidatesStructureBoundsAndChronology(t *testing.T) {
	source := testSource(t, "normalized-contract")
	published := testTimestamp(t, 10)
	updated := testTimestamp(t, 11)
	valid := application.NormalizedSource{
		SourceID: source.ID, Locator: source.Locator, ContentType: "text/html", Title: "Reference",
		Headings:     []application.NormalizedHeading{{Level: 1, Text: "Guide", Path: []string{"Guide"}}},
		TextSegments: []string{"Bounded content."},
		CodeBlocks:   []application.NormalizedCodeBlock{{Language: "go", Content: "func main() {}"}},
		Links:        []application.NormalizedLink{{Label: "Reference", Locator: source.Locator}},
		PublishedAt:  &published, UpdatedAt: &updated, VersionHints: []string{"1.2.3"},
		NormalizationVersion: "source-normalization-v1",
	}
	if err := valid.Validate(); err != nil {
		t.Fatalf("NormalizedSource.Validate() error = %v", err)
	}

	tests := []struct {
		name   string
		mutate func(*application.NormalizedSource)
	}{
		{name: "heading level", mutate: func(source *application.NormalizedSource) { source.Headings[0].Level = 7 }},
		{name: "heading path", mutate: func(source *application.NormalizedSource) { source.Headings[0].Path = []string{"Other"} }},
		{name: "empty content", mutate: func(source *application.NormalizedSource) {
			source.Headings, source.TextSegments, source.CodeBlocks = nil, nil, nil
		}},
		{name: "oversize code", mutate: func(source *application.NormalizedSource) {
			source.CodeBlocks[0].Content = strings.Repeat("x", application.MaximumNormalizedCodeBytes+1)
		}},
		{name: "chronology", mutate: func(source *application.NormalizedSource) {
			source.PublishedAt, source.UpdatedAt = &updated, &published
		}},
		{name: "missing version", mutate: func(source *application.NormalizedSource) { source.NormalizationVersion = "" }},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			candidate := valid
			candidate.Headings = append([]application.NormalizedHeading(nil), valid.Headings...)
			candidate.Headings[0].Path = append([]string(nil), valid.Headings[0].Path...)
			candidate.CodeBlocks = append([]application.NormalizedCodeBlock(nil), valid.CodeBlocks...)
			test.mutate(&candidate)
			if err := candidate.Validate(); err == nil {
				t.Fatal("NormalizedSource.Validate() accepted invalid value")
			}
		})
	}
}
