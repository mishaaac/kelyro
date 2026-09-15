package learningpack

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"strings"
	"testing"

	"github.com/mishaaac/kelyro/internal/curriculum"
	curriculumapp "github.com/mishaaac/kelyro/internal/curriculum/application"
)

func TestBuilderCreatesDeterministicCopyrightAwarePack(t *testing.T) {
	t.Parallel()
	request := packBuildFixture(t)
	builder := NewBuilder()
	first, err := builder.Build(context.Background(), request)
	if err != nil {
		t.Fatalf("Build() error = %v", err)
	}
	second, err := builder.Build(context.Background(), request)
	if err != nil {
		t.Fatal(err)
	}
	if first.ContentHash != second.ContentHash || !bytes.Equal(first.PortableArchive, second.PortableArchive) {
		t.Fatalf("same build request produced different archives")
	}
	for _, name := range []string{EvidenceMarkdownName, AssetLicensesName, "assets/example.go", "build/build-info.json", "sources/evidence-report.json"} {
		if !containsString(first.Entries, name) {
			t.Fatalf("built entries %v missing %q", first.Entries, name)
		}
	}
	if first.Pack.EvidenceReport == nil || len(first.Pack.EvidenceReport.Citations) != 1 || first.Pack.BuildInfo == nil {
		t.Fatalf("built pack metadata = %+v", first.Pack)
	}
	if bytes.Contains(first.PortableArchive, []byte(request.EvidenceSets[0].Claims[0].Statement)) {
		t.Fatalf("built archive retained the external Claim statement")
	}
	archive := writeZIPBytes(t, first.PortableArchive)
	validated, err := NewValidator().Validate(context.Background(), curriculumapp.PackSource{Path: archive})
	if err != nil || len(validated.Errors) != 0 || validated.ContentHash != first.ContentHash {
		t.Fatalf("validate built archive = %+v, %v", validated, err)
	}
}

func TestBuilderRejectsMissingCitationAndUnlicensedOrExternalAssets(t *testing.T) {
	t.Parallel()
	request := packBuildFixture(t)
	request.Citations = nil
	if _, err := NewBuilder().Build(context.Background(), request); err == nil || !strings.Contains(err.Error(), "has no citation URL") {
		t.Fatalf("missing citation error = %v", err)
	}
	request = packBuildFixture(t)
	request.Assets[0].KelyroAuthored = false
	if _, err := NewBuilder().Build(context.Background(), request); err == nil || !strings.Contains(err.Error(), "not declared Kelyro-authored") {
		t.Fatalf("external asset error = %v", err)
	}
	request = packBuildFixture(t)
	request.Assets[0].License = ""
	if _, err := NewBuilder().Build(context.Background(), request); err == nil || !strings.Contains(err.Error(), "requires license") {
		t.Fatalf("unlicensed asset error = %v", err)
	}
	request = packBuildFixture(t)
	request.Citations[0].Excerpt = strings.Repeat("x", curriculum.MaximumCitationExcerptBytes+1)
	if _, err := NewBuilder().Build(context.Background(), request); err == nil || !strings.Contains(err.Error(), "within 512 bytes") {
		t.Fatalf("oversized excerpt error = %v", err)
	}
}

func TestValidatorRejectsDetectableForbiddenRetentionAndUnlicensedAssets(t *testing.T) {
	t.Parallel()
	for _, test := range []struct {
		name    string
		path    string
		content []byte
		want    string
	}{
		{"raw cache path", "sources/cache/page.txt", []byte("cached page"), "forbidden retained-source path"},
		{"transcript path", "sources/full-transcript.txt", []byte("transcript"), "forbidden retained-source path"},
		{"body field", "sources/metadata.json", []byte("{\"raw_body\":\"page\"}"), "forbidden source-retention field"},
		{"unlicensed asset", "assets/example.go", []byte("package example\n"), "has no license record"},
		{"noncanonical evidence markdown", EvidenceMarkdownName, []byte("# copied article\n"), "does not match the canonical evidence report"},
	} {
		test := test
		t.Run(test.name, func(t *testing.T) {
			entries := validPackEntries()
			entries[test.path] = test.content
			updateChecksums(entries)
			result, err := NewValidator().Validate(context.Background(), curriculumapp.PackSource{Path: writeDirectory(t, entries)})
			if err != nil || len(result.Errors) != 1 || !strings.Contains(result.Errors[0].Message, test.want) {
				t.Fatalf("Validate() = %+v, %v; want %q", result, err, test.want)
			}
		})
	}
}

func packBuildFixture(t *testing.T) curriculumapp.PackBuildRequest {
	t.Helper()
	pack, _, err := validateEntries(validPackEntries())
	if err != nil {
		t.Fatal(err)
	}
	buildInfo := *pack.BuildInfo
	passes := []curriculum.CompilationPass{
		{Name: buildInfo.Passes[0].Name, Version: buildInfo.Passes[0].Version, InputHash: buildInfo.InputHash, OutputHash: "sha256:" + strings.Repeat("d", 64)},
		{Name: buildInfo.Passes[1].Name, Version: buildInfo.Passes[1].Version, InputHash: "sha256:" + strings.Repeat("e", 64), OutputHash: buildInfo.OutputHash},
	}
	compilation := curriculum.CompilationResult{Curriculum: pack.Curriculum, Passes: passes, BuildInfo: &buildInfo}
	sourceID, _ := curriculum.NewID("source.go-packages")
	claimID, _ := curriculum.NewID("claim.go-packages")
	evidence := curriculum.CurriculumEvidenceSet{
		Bundle: pack.Curriculum.SourceBundles[0], Eligibility: curriculum.EvidenceReadyForCompile,
		Claims:           []curriculum.CurriculumEvidenceClaim{{ID: claimID, Statement: "Go packages group files.", Kind: curriculum.EvidenceClaimDefinition, Scope: "packages", Status: curriculum.ConceptCurrent, Confidence: 1, SourceIDs: []curriculum.ID{sourceID}}},
		SourceAuthority:  []curriculum.EvidenceSourceAuthority{{SourceID: sourceID, Role: "primary", TemporalScope: "current"}},
		Freshness:        curriculum.EvidenceFreshness{State: "fresh", Score: 1, LastVerifiedAt: &pack.Curriculum.SourceBundles[0].VerifiedAt, Algorithm: "source-bundle-freshness-v1"},
		AlgorithmVersion: curriculum.EvidenceIngestionAlgorithmV1,
	}
	excerpt := "Package clauses group related Go files."
	digest := sha256.Sum256([]byte(excerpt))
	return curriculumapp.PackBuildRequest{
		Manifest: pack.Manifest, Compilation: compilation, EvidenceSets: []curriculum.CurriculumEvidenceSet{evidence}, Environment: pack.Environment,
		Citations: []curriculum.EvidenceReportCitation{{SourceID: sourceID, Title: "Go package documentation", URL: "https://go.dev/ref/spec#Package_clause", License: "BSD-3-Clause", Excerpt: excerpt, ExcerptHash: "sha256:" + hex.EncodeToString(digest[:])}},
		Assets:    []curriculumapp.PackBuildAsset{{Path: "assets/example.go", Content: []byte("package example\n"), License: "MIT", CopyrightHolder: "Kelyro contributors", KelyroAuthored: true}},
	}
}

func containsString(values []string, target string) bool {
	for _, value := range values {
		if value == target {
			return true
		}
	}
	return false
}
