package markdown

import (
	"bytes"
	"os"
	"path/filepath"
	"testing"
	"unicode/utf8"
)

func TestGenerateMatchesGoldenDocuments(t *testing.T) {
	t.Parallel()

	documents, err := Generate(Model{Workspace: "Backend Go — Perú"})
	if err != nil {
		t.Fatalf("Generate() error = %v", err)
	}
	if len(documents) != 2 {
		t.Fatalf("Generate() documents = %d, want 2", len(documents))
	}

	for _, document := range documents {
		goldenPath := filepath.Join("testdata", filepath.Base(document.Path)+".golden")
		want, err := os.ReadFile(goldenPath)
		if err != nil {
			t.Fatalf("ReadFile(%q): %v", goldenPath, err)
		}
		if !bytes.Equal(document.Content, want) {
			t.Errorf("%s content:\n%s\nwant golden:\n%s", document.Path, document.Content, want)
		}
		if !utf8.Valid(document.Content) {
			t.Errorf("%s is not valid UTF-8", document.Path)
		}
		if bytes.Contains(document.Content, []byte{'\r'}) || !bytes.HasSuffix(document.Content, []byte{'\n'}) {
			t.Errorf("%s does not use canonical LF-terminated lines", document.Path)
		}
	}
}

func TestGenerateKeepsInternalStateOutOfHumanMarkdown(t *testing.T) {
	t.Parallel()

	documents, err := Generate(Model{Workspace: "  Backend\r\nGo  "})
	if err != nil {
		t.Fatalf("Generate() error = %v", err)
	}
	if !bytes.Contains(documents[0].Content, []byte("Workspace: Backend Go\n")) {
		t.Fatalf("LEARNING.md did not normalize display name:\n%s", documents[0].Content)
	}
	for _, document := range documents {
		if bytes.Contains(document.Content, []byte("{")) {
			t.Errorf("%s unexpectedly contains serialized internal state", document.Path)
		}
	}
}

func TestGenerateRejectsEmptyWorkspaceName(t *testing.T) {
	t.Parallel()

	if _, err := Generate(Model{Workspace: " \r\n "}); err == nil {
		t.Fatal("Generate() error = nil, want empty display name error")
	}
}
