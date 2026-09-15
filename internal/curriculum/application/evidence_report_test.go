package application

import (
	"context"
	"reflect"
	"strings"
	"testing"

	"github.com/mishaaac/kelyro/internal/curriculum"
)

func TestCurriculumEvidenceReporterV1GeneratesDeterministicSummaryWithoutSourceText(t *testing.T) {
	t.Parallel()
	request := compilerFixture(t)
	compilation, err := NewCurriculumCompilerV1().Compile(context.Background(), request)
	if err != nil {
		t.Fatal(err)
	}
	reporter := NewCurriculumEvidenceReporterV1()
	result, err := reporter.Generate(context.Background(), CurriculumEvidenceReportRequest{Compilation: compilation, EvidenceSets: request.EvidenceSets})
	if err != nil {
		t.Fatalf("Generate() error = %v", err)
	}
	repeated, err := reporter.Generate(context.Background(), CurriculumEvidenceReportRequest{Compilation: compilation, EvidenceSets: request.EvidenceSets})
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(result, repeated) {
		t.Fatalf("same compilation produced different report")
	}
	report := result.Report
	if report.SchemaVersion != curriculum.CurriculumEvidenceReportSchemaVersionV1 || report.Goal.ID != compilation.Curriculum.Goal.ID ||
		report.ConceptCount != len(compilation.Curriculum.Concepts) || report.SourceBundleCount != 1 ||
		report.PrimarySourceCoverage.PrimaryClaims != 1 || report.PrimarySourceCoverage.ReferencedClaims != 1 || report.PrimarySourceCoverage.Ratio != 1 {
		t.Fatalf("report = %+v", report)
	}
	for _, want := range []string{"# Curriculum Evidence Report", "Primary-source coverage: 1/1", "bundle.goal", "curriculum-compiler-v1"} {
		if !strings.Contains(result.Markdown, want) {
			t.Fatalf("markdown %q missing %q", result.Markdown, want)
		}
	}
	if strings.Contains(result.Markdown, request.EvidenceSets[0].Claims[0].Statement) {
		t.Fatalf("markdown retained external claim statement: %q", result.Markdown)
	}
}

func TestCurriculumEvidenceReporterV1RejectsEvidenceOutsideFrozenBuild(t *testing.T) {
	t.Parallel()
	request := compilerFixture(t)
	compilation, err := NewCurriculumCompilerV1().Compile(context.Background(), request)
	if err != nil {
		t.Fatal(err)
	}
	request.EvidenceSets[0].Bundle.ContentHash = "sha256:" + strings.Repeat("f", 64)
	_, err = NewCurriculumEvidenceReporterV1().Generate(context.Background(), CurriculumEvidenceReportRequest{Compilation: compilation, EvidenceSets: request.EvidenceSets})
	if err == nil || !strings.Contains(err.Error(), "does not match frozen input") {
		t.Fatalf("Generate() error = %v", err)
	}
}
