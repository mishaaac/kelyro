package artifacts

import (
	"path/filepath"
	"testing"
	"time"
)

func TestClassify(t *testing.T) {
	tests := []struct {
		path string
		want Ownership
	}{
		{path: filepath.Join(".kelyro", "learning.db"), want: MachineOwned},
		{path: filepath.Join(".kelyro", "state", "session"), want: MachineOwned},
		{path: "LEARNING.md", want: SystemGeneratedHumanReadable},
		{path: filepath.Join("00-roadmap", "ROADMAP.md"), want: SystemGeneratedHumanReadable},
		{path: filepath.Join("00-roadmap", "PROGRESS.md"), want: SystemGeneratedHumanReadable},
		{path: filepath.Join("docs", "implementation", "PROGRESS.md"), want: StudentOwned},
		{path: filepath.Join("lesson-01", "LESSON.md"), want: SystemGeneratedHumanReadable},
		{path: "main.go", want: StudentOwned},
		{path: filepath.Join("projects", "demo", "notes.md"), want: StudentOwned},
	}

	for _, test := range tests {
		t.Run(test.path, func(t *testing.T) {
			if got := Classify(test.path); got != test.want {
				t.Fatalf("Classify(%q) = %q, want %q", test.path, got, test.want)
			}
		})
	}
}

func TestArtifactValidate(t *testing.T) {
	now := time.Now().UTC()
	valid := Artifact{
		Path:            "LEARNING.md",
		Ownership:       SystemGeneratedHumanReadable,
		CreatedBy:       "test",
		ContentHash:     Hash([]byte("content")),
		CreatedAt:       now,
		LastGeneratedAt: now,
	}
	if err := valid.Validate(); err != nil {
		t.Fatalf("Validate() error = %v", err)
	}

	invalid := valid
	invalid.ContentHash = "not-a-hash"
	if err := invalid.Validate(); err == nil {
		t.Fatal("Validate() accepted an invalid content hash")
	}
}
