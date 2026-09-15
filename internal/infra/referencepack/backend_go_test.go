package referencepack

import (
	"bytes"
	"context"
	"os"
	"path/filepath"
	"runtime"
	"testing"

	"github.com/mishaaac/kelyro/internal/curriculum"
	curriculumapp "github.com/mishaaac/kelyro/internal/curriculum/application"
	"github.com/mishaaac/kelyro/internal/infra/learningpack"
)

func TestBackendGoReferenceCompilesBuildsAndMatchesCommittedArchive(t *testing.T) {
	t.Parallel()
	result, err := BuildBackendGo(context.Background())
	if err != nil {
		t.Fatalf("BuildBackendGo() error = %v", err)
	}
	pack := result.Pack
	if pack.Manifest.ID.String() != "backend-go-reference" || pack.Manifest.Status != curriculum.ConceptPreview || len(pack.Curriculum.Concepts) != len(backendGoConcepts) {
		t.Fatalf("pack identity/scope = %+v, concepts=%d", pack.Manifest, len(pack.Curriculum.Concepts))
	}
	if pack.Environment == nil || pack.EvidenceReport == nil || pack.BuildInfo == nil || len(pack.Curriculum.Prerequisites) == 0 {
		t.Fatalf("reference pack omitted required compiler dimensions: %+v", pack)
	}
	statuses := make(map[curriculum.ConceptStatus]int)
	for _, concept := range pack.Curriculum.Concepts {
		statuses[concept.Status]++
	}
	if statuses[curriculum.ConceptCurrent] == 0 || statuses[curriculum.ConceptLegacy] == 0 {
		t.Fatalf("temporal classifications = %v", statuses)
	}
	if pack.EvidenceReport.PrimarySourceCoverage.Ratio != 1 || len(pack.EvidenceReport.Caveats) == 0 {
		t.Fatalf("evidence report = %+v", pack.EvidenceReport)
	}

	archivePath := filepath.Join(repositoryRoot(t), "reference-packs", BackendGoArchiveName)
	committed, err := os.ReadFile(archivePath)
	if err != nil {
		t.Fatalf("read committed archive: %v", err)
	}
	if !bytes.Equal(committed, result.PortableArchive) {
		t.Fatalf("committed archive is stale; regenerate %s", archivePath)
	}
	validated, err := learningpack.NewValidator().Validate(context.Background(), curriculumapp.PackSource{Path: archivePath})
	if err != nil || len(validated.Errors) != 0 || validated.ContentHash != result.ContentHash {
		t.Fatalf("validate committed archive = %+v, %v", validated, err)
	}
}

func TestBackendGoReferenceIsDeterministic(t *testing.T) {
	t.Parallel()
	first, err := BuildBackendGo(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	second, err := BuildBackendGo(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if first.ContentHash != second.ContentHash || !bytes.Equal(first.PortableArchive, second.PortableArchive) {
		t.Fatal("same reference source produced different pack bytes")
	}
}

func TestBackendGoReferenceHonorsCancellation(t *testing.T) {
	t.Parallel()
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, err := BuildBackendGo(ctx); err == nil {
		t.Fatal("BuildBackendGo() accepted a canceled context")
	}
}

func repositoryRoot(t *testing.T) string {
	t.Helper()
	_, file, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("locate reference pack test")
	}
	return filepath.Clean(filepath.Join(filepath.Dir(file), "..", "..", ".."))
}
