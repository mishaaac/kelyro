package application

import (
	"context"
	"testing"

	"github.com/mishaaac/kelyro/internal/curriculum"
)

func TestCurriculumInspectorV1ReportsRetainedArtifactFacts(t *testing.T) {
	t.Parallel()
	request := completeCompilerFixture(t)
	compilation, err := NewCurriculumCompilerV1().Compile(context.Background(), request)
	if err != nil {
		t.Fatal(err)
	}
	sourceID := request.EvidenceSets[0].SourceAuthority[0].SourceID
	report, err := NewCurriculumEvidenceReporterV1().Generate(context.Background(), CurriculumEvidenceReportRequest{
		Compilation: compilation, EvidenceSets: request.EvidenceSets,
		Citations: []curriculum.EvidenceReportCitation{{SourceID: sourceID, Title: "Official reference", URL: "https://example.com/reference"}},
	})
	if err != nil {
		t.Fatal(err)
	}
	manifest := dependencyManifest(t, "pack.inspection", "1.0.0")
	manifest.CurriculumID = compilation.Curriculum.ID
	manifest.BuildInfoEntry = "build/build-info.json"
	pack := curriculum.LearningPack{Manifest: manifest, Curriculum: compilation.Curriculum, BuildInfo: compilation.BuildInfo, EvidenceReport: &report.Report}

	inspection, err := NewCurriculumInspectorV1().Inspect(context.Background(), pack)
	if err != nil {
		t.Fatalf("Inspect() error = %v", err)
	}
	if inspection.AlgorithmVersion != CurriculumInspectionVersionV1 || len(inspection.Coverage) != len(curriculum.AllCoverageDimensions()) ||
		len(inspection.CompilationPasses) != len(compilation.Passes) || len(inspection.EvidenceLinks) != 1 {
		t.Fatalf("inspection = %+v", inspection)
	}
	for _, audit := range inspection.Audits {
		if !audit.Passed {
			t.Fatalf("audit failed: %+v", audit)
		}
	}
}

func TestCurriculumInspectorV1RejectsInvalidPack(t *testing.T) {
	t.Parallel()
	_, err := NewCurriculumInspectorV1().Inspect(context.Background(), curriculum.LearningPack{})
	if err == nil {
		t.Fatal("Inspect() accepted an invalid pack")
	}
}
