package researchnormalize

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/mishaaac/kelyro/internal/research"
	"github.com/mishaaac/kelyro/internal/research/application"
)

func TestNormalizerGoldenFixtures(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name        string
		contentType string
	}{
		{name: "html", contentType: "text/html"},
		{name: "markdown", contentType: "text/markdown"},
		{name: "json", contentType: "application/json"},
		{name: "text", contentType: "text/plain"},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			body, err := os.ReadFile(filepath.Join("testdata", "document."+fixtureExtension(test.name)))
			if err != nil {
				t.Fatal(err)
			}
			result, err := New().Normalize(context.Background(), fetchedDocument(t, test.contentType, body))
			if err != nil {
				t.Fatal(err)
			}
			got := renderNormalized(result)
			want, err := os.ReadFile(filepath.Join("testdata", test.name+".golden"))
			if err != nil {
				t.Fatal(err)
			}
			if got != string(want) {
				t.Fatalf("normalized %s:\n%s\nwant:\n%s", test.name, got, want)
			}
		})
	}
}

func TestHTMLNormalizationRemovesExecutableNoiseAndCanonicalizesLinks(t *testing.T) {
	body, err := os.ReadFile(filepath.Join("testdata", "document.html"))
	if err != nil {
		t.Fatal(err)
	}
	result, err := New().Normalize(context.Background(), fetchedDocument(t, "text/html", body))
	if err != nil {
		t.Fatal(err)
	}
	rendered := renderNormalized(result)
	for _, forbidden := range []string{"prompt", "display: block", "Previous Account Logout", "Cookie settings"} {
		if strings.Contains(rendered, forbidden) {
			t.Fatalf("normalized HTML retained %q", forbidden)
		}
	}
	if result.CanonicalLocator == nil || result.CanonicalLocator.String() != "https://docs.example.dev/guides/api/" {
		t.Fatalf("canonical locator = %+v", result.CanonicalLocator)
	}
	if len(result.Links) != 1 || result.Links[0].Locator.String() != "https://docs.example.dev/reference?view=full#client" {
		t.Fatalf("links = %+v", result.Links)
	}
}

func TestHTMLDiscardedContentRequiresExactClosingTagAndUnsafeLinksAreIgnored(t *testing.T) {
	body := []byte(`<html><body><script>secret </scripture> still secret</script><p>Safe text.</p><a href="javascript:alert(1)">unsafe</a></body></html>`)
	result, err := New().Normalize(context.Background(), fetchedDocument(t, "text/html", body))
	if err != nil {
		t.Fatal(err)
	}
	rendered := renderNormalized(result)
	if strings.Contains(rendered, "secret") || len(result.Links) != 0 {
		t.Fatalf("adversarial HTML leaked discarded content: %s", rendered)
	}
	if len(result.TextSegments) != 2 || result.TextSegments[0] != "Safe text." || result.TextSegments[1] != "unsafe" {
		t.Fatalf("segments = %q", result.TextSegments)
	}
}

func TestMarkdownPreservesNonTagAngleBracketText(t *testing.T) {
	body := []byte("# Comparison\n\n1 < 2 and 3 > 1.")
	result, err := New().Normalize(context.Background(), fetchedDocument(t, "text/markdown", body))
	if err != nil {
		t.Fatal(err)
	}
	if len(result.TextSegments) != 1 || result.TextSegments[0] != "1 < 2 and 3 > 1." {
		t.Fatalf("segments = %q", result.TextSegments)
	}
}

func TestNormalizerRejectsUnsupportedInvalidAndBodylessDocuments(t *testing.T) {
	tests := []struct {
		name        string
		contentType string
		body        []byte
		mutate      func(*application.FetchedSource)
		want        error
	}{
		{name: "unsupported PDF", contentType: "application/pdf", body: []byte("pdf"), want: ErrUnsupportedContentType},
		{name: "invalid JSON", contentType: "application/json", body: []byte(`{"open":`), want: ErrInvalidDocument},
		{name: "invalid UTF-8", contentType: "text/plain", body: []byte{0xff}, want: ErrInvalidDocument},
		{name: "304", contentType: "text/plain", body: []byte("body"), mutate: func(source *application.FetchedSource) {
			source.Metadata.StatusCode = 304
		}, want: ErrInvalidDocument},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			fetched := fetchedDocument(t, test.contentType, test.body)
			if test.mutate != nil {
				test.mutate(&fetched)
			}
			_, err := New().Normalize(context.Background(), fetched)
			if !errors.Is(err, test.want) {
				t.Fatalf("Normalize() error = %v, want %v", err, test.want)
			}
		})
	}

	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	_, err := New().Normalize(ctx, fetchedDocument(t, "text/plain", []byte("fixture")))
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("canceled Normalize() error = %v", err)
	}
}

func TestNormalizerEnforcesBoundedOutput(t *testing.T) {
	var document strings.Builder
	for index := 0; index <= application.MaximumNormalizedHeadings; index++ {
		fmt.Fprintf(&document, "<h2>Heading %d</h2>", index)
	}
	_, err := New().Normalize(context.Background(), fetchedDocument(t, "text/html", []byte(document.String())))
	if !errors.Is(err, ErrOutputLimit) {
		t.Fatalf("Normalize() error = %v, want ErrOutputLimit", err)
	}
}

func fetchedDocument(t *testing.T, contentType string, body []byte) application.FetchedSource {
	t.Helper()
	sourceID, err := research.NewSourceID("source.normalization-fixture")
	if err != nil {
		t.Fatal(err)
	}
	locator, err := research.NewSourceLocator("https://docs.example.dev/reference/page.html")
	if err != nil {
		t.Fatal(err)
	}
	fetchedAt, err := research.NewTimestamp(time.Date(2026, 8, 24, 20, 0, 0, 0, time.UTC))
	if err != nil {
		t.Fatal(err)
	}
	return application.FetchedSource{
		SourceID: sourceID, Locator: locator, FetchedAt: fetchedAt, Origin: application.FetchOriginLive,
		Metadata: research.FetchMetadata{
			StatusCode: 200, ContentType: contentType, ContentHash: research.CanonicalContentHashV1(body),
			ContentLength: int64(len(body)), FetchVersion: "source-fetch-v1",
		}, Body: append([]byte(nil), body...),
	}
}

func fixtureExtension(name string) string {
	if name == "markdown" {
		return "md"
	}
	if name == "text" {
		return "txt"
	}
	return name
}

func renderNormalized(source application.NormalizedSource) string {
	var output strings.Builder
	fmt.Fprintf(&output, "normalization_version: %s\n", source.NormalizationVersion)
	fmt.Fprintf(&output, "source_id: %s\n", source.SourceID.String())
	fmt.Fprintf(&output, "locator: %s\n", source.Locator.String())
	fmt.Fprintf(&output, "content_type: %s\n", source.ContentType)
	fmt.Fprintf(&output, "title: %s\n", strconv.Quote(source.Title))
	canonical := ""
	if source.CanonicalLocator != nil {
		canonical = source.CanonicalLocator.String()
	}
	fmt.Fprintf(&output, "canonical: %s\n", canonical)
	fmt.Fprintf(&output, "language: %s\n", renderedOptional(source.Language))
	fmt.Fprintf(&output, "published_at: %s\n", renderedOptional(renderedTimestamp(source.PublishedAt)))
	fmt.Fprintf(&output, "updated_at: %s\n", renderedOptional(renderedTimestamp(source.UpdatedAt)))
	fmt.Fprintf(&output, "version_hints: %s\n", strings.Join(source.VersionHints, ", "))
	output.WriteString("headings:\n")
	for _, heading := range source.Headings {
		fmt.Fprintf(&output, "- h%d %s path=%s\n", heading.Level, strconv.Quote(heading.Text), strconv.Quote(strings.Join(heading.Path, " > ")))
	}
	output.WriteString("segments:\n")
	for _, segment := range source.TextSegments {
		fmt.Fprintf(&output, "- %s\n", strconv.Quote(segment))
	}
	output.WriteString("code_blocks:\n")
	for _, block := range source.CodeBlocks {
		fmt.Fprintf(&output, "- language=%s content=%s\n", block.Language, strconv.Quote(block.Content))
	}
	output.WriteString("links:\n")
	for _, link := range source.Links {
		fmt.Fprintf(&output, "- %s -> %s\n", strconv.Quote(link.Label), link.Locator.String())
	}
	return output.String()
}

func renderedTimestamp(timestamp *research.Timestamp) string {
	if timestamp == nil {
		return ""
	}
	return timestamp.Time().Format(time.RFC3339Nano)
}

func renderedOptional(value string) string {
	if value == "" {
		return `""`
	}
	return value
}
