package diagnosticjson

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/mishaaac/kelyro/internal/learning"
)

func TestLoadFoundationFixture(t *testing.T) {
	path := filepath.Join("..", "..", "..", "testdata", "curricula", "foundation-demo", "diagnostic.json")
	firstFile, err := os.Open(path)
	if err != nil {
		t.Fatal(err)
	}
	first, err := Load(firstFile)
	_ = firstFile.Close()
	if err != nil {
		t.Fatal(err)
	}
	secondFile, err := os.Open(path)
	if err != nil {
		t.Fatal(err)
	}
	second, err := Load(secondFile)
	_ = secondFile.Close()
	if err != nil {
		t.Fatal(err)
	}
	firstFingerprint, _ := learning.DiagnosticFingerprint(first)
	secondFingerprint, _ := learning.DiagnosticFingerprint(second)
	if first.Curriculum.ID.String() != "foundation-demo" || first.Curriculum.Version != "1.0.0" || len(first.Items()) != 4 || firstFingerprint != secondFingerprint {
		t.Fatalf("fixture = %+v fingerprints=(%s,%s)", first, firstFingerprint, secondFingerprint)
	}
	wantKinds := []learning.DiagnosticItemKind{learning.DiagnosticSingleChoice, learning.DiagnosticShortAnswer, learning.DiagnosticMultipleChoice, learning.DiagnosticSelfReport}
	for index, item := range first.Items() {
		if item.Kind != wantKinds[index] {
			t.Fatalf("item %d kind = %s, want %s", index, item.Kind, wantKinds[index])
		}
	}
}

func TestLoadIsStrict(t *testing.T) {
	if _, err := Load(strings.NewReader(`{"unknown":true}`)); err == nil {
		t.Fatal("Load() accepted unknown field")
	}
	if _, err := Load(strings.NewReader("")); err == nil {
		t.Fatal("Load() accepted empty document")
	}
}
