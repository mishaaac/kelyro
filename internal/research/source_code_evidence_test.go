package research

import (
	"strings"
	"testing"
	"time"
)

func TestSourceCodeLocatorCanonicalRoundTrip(t *testing.T) {
	t.Parallel()
	locator := sourceCodeLocatorFixture(t)
	encoded, err := EncodeSourceCodeLocator(locator)
	if err != nil {
		t.Fatal(err)
	}
	parsed, err := ParseSourceCodeLocator(encoded)
	if err != nil {
		t.Fatal(err)
	}
	reencoded, err := EncodeSourceCodeLocator(parsed)
	if err != nil {
		t.Fatal(err)
	}
	if string(encoded) != string(reencoded) || parsed.Path != locator.Path || parsed.License == nil || parsed.License.Identifier != "Apache-2.0" {
		t.Fatalf("source code roundtrip = %+v\n%s\n%s", parsed, encoded, reencoded)
	}
	parsed.License.Identifier = "changed"
	if locator.License.Identifier != "Apache-2.0" {
		t.Fatal("source code locator clone leaked license mutation")
	}
}

func TestSourceCodeLocatorRejectsMutableOrUnboundedLocations(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name   string
		mutate func(*SourceCodeLocator)
		needle string
	}{
		{"branch", func(locator *SourceCodeLocator) { locator.Commit = "main" }, "hexadecimal"},
		{"permalink without commit", func(locator *SourceCodeLocator) {
			locator.Permalink = mustSourceCodeLocator(t, "https://code.example.test/project/blob/main/client.go#L10")
		}, "immutable commit"},
		{"cross host", func(locator *SourceCodeLocator) {
			locator.Permalink = mustSourceCodeLocator(t, "https://mirror.example.test/project/blob/0123456789abcdef/client.go#L10")
		}, "host"},
		{"traversal", func(locator *SourceCodeLocator) { locator.Path = "../client.go" }, "relative path"},
		{"line span", func(locator *SourceCodeLocator) { locator.EndLine = locator.StartLine + MaximumSourceCodeLineSpan }, "exceeds"},
		{"missing version", func(locator *SourceCodeLocator) { locator.VersionScope = "" }, "version scope"},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			locator := sourceCodeLocatorFixture(t)
			test.mutate(&locator)
			if err := locator.Validate(); err == nil || !strings.Contains(err.Error(), test.needle) {
				t.Fatalf("Validate() error = %v, want %q", err, test.needle)
			}
		})
	}
}

func TestValidateSourceCodeEvidenceRelationship(t *testing.T) {
	t.Parallel()
	created, _ := NewTimestamp(time.Date(2026, time.August, 28, 10, 0, 0, 0, time.UTC))
	id, _ := NewSourceID("source.code")
	snapshotID, _ := NewID("snapshot.code")
	evidenceID, _ := NewID("evidence.code")
	locator := sourceCodeLocatorFixture(t)
	source := Source{ID: id, Kind: SourceCode, Locator: locator.Repository, Version: &locator.VersionScope, TemporalScope: SourceTemporalVersionBound, Metadata: SourceMetadata{Title: "Portable client source"}, CreatedAt: created}
	excerpt := "return client.send(request)"
	evidence := Evidence{ID: evidenceID, SourceID: id, SnapshotID: snapshotID, Location: "client.go:10", Excerpt: excerpt, ExcerptHash: CanonicalEvidenceExcerptHashV1(excerpt), ExtractedAt: created, ExtractorVersion: "extract/v1", SourceCode: locator.Clone()}
	if err := ValidateSourceCodeEvidenceRelationship(source, evidence); err != nil {
		t.Fatal(err)
	}
	evidence.SourceCode = nil
	if err := ValidateSourceCodeEvidenceRelationship(source, evidence); err == nil || !strings.Contains(err.Error(), "requires") {
		t.Fatalf("missing source code locator error = %v", err)
	}
	other := source
	other.Kind = SourceOfficialDocumentation
	evidence.SourceCode = locator.Clone()
	if err := ValidateSourceCodeEvidenceRelationship(other, evidence); err == nil || !strings.Contains(err.Error(), "source_code") {
		t.Fatalf("wrong source kind error = %v", err)
	}
}

func sourceCodeLocatorFixture(t *testing.T) SourceCodeLocator {
	t.Helper()
	repository := mustSourceCodeLocator(t, "https://code.example.test/project")
	permalink := mustSourceCodeLocator(t, "https://code.example.test/project/tree/0123456789abcdef/client.go#lines-10-20")
	licenseLocator := mustSourceCodeLocator(t, "https://code.example.test/project/tree/0123456789abcdef/LICENSE")
	version, _ := NewSourceVersion("v2.4.1")
	return SourceCodeLocator{
		Repository: repository, Permalink: permalink, Commit: "0123456789abcdef", Path: "client.go",
		StartLine: 10, EndLine: 20, Symbol: "Client.Send", VersionScope: version,
		License:          &SourceCodeLicense{Identifier: "Apache-2.0", Name: "Apache License 2.0", Locator: &licenseLocator},
		AlgorithmVersion: SourceCodeEvidenceV1,
	}
}

func mustSourceCodeLocator(t *testing.T, value string) SourceLocator {
	t.Helper()
	locator, err := NewSourceLocator(value)
	if err != nil {
		t.Fatal(err)
	}
	return locator
}
