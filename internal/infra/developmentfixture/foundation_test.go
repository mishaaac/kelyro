package developmentfixture

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/mishaaac/kelyro/internal/infra/curriculumyaml"
	"github.com/mishaaac/kelyro/internal/infra/diagnosticjson"
	"github.com/mishaaac/kelyro/internal/learning"
)

func TestEmbeddedFoundationDemoMatchesVersionedTestdata(t *testing.T) {
	curriculum, diagnostic, err := FoundationDemo()
	if err != nil {
		t.Fatal(err)
	}
	curriculumFile, err := os.Open(filepath.Join("..", "..", "..", "testdata", "curricula", "foundation-demo", "curriculum.yaml"))
	if err != nil {
		t.Fatal(err)
	}
	wantCurriculum, err := curriculumyaml.Load(curriculumFile)
	_ = curriculumFile.Close()
	if err != nil {
		t.Fatal(err)
	}
	diagnosticFile, err := os.Open(filepath.Join("..", "..", "..", "testdata", "curricula", "foundation-demo", "diagnostic.json"))
	if err != nil {
		t.Fatal(err)
	}
	wantDiagnostic, err := diagnosticjson.Load(diagnosticFile)
	_ = diagnosticFile.Close()
	if err != nil {
		t.Fatal(err)
	}
	gotCurriculum, _ := learning.CurriculumFingerprint(curriculum)
	wantCurriculumFingerprint, _ := learning.CurriculumFingerprint(wantCurriculum)
	gotDiagnostic, _ := learning.DiagnosticFingerprint(diagnostic)
	wantDiagnosticFingerprint, _ := learning.DiagnosticFingerprint(wantDiagnostic)
	if gotCurriculum != wantCurriculumFingerprint || gotDiagnostic != wantDiagnosticFingerprint {
		t.Fatalf("embedded fingerprints=(%s,%s), testdata=(%s,%s)", gotCurriculum, gotDiagnostic, wantCurriculumFingerprint, wantDiagnosticFingerprint)
	}
}
