package learningpack

import (
	"strings"
	"testing"
)

const validManifest = `id: go.backend
name: Go Backend
description: A production-oriented backend curriculum.
version: 1.2.0
schema_version: learning-pack/v1
domain: software-engineering
target: Backend Engineer with Go
authors: [Kelyro Curriculum Team]
maintainers: [Kelyro Curriculum Team]
license: CC-BY-4.0
created_at: 2026-09-11T15:00:00Z
minimum_kelyro_version: 0.2.0
dependencies:
  - id: computing.foundations
    constraint: ">=1.0.0 <2.0.0"
environment_pack: environment/environment.yaml
curriculum_entry: curriculum/curriculum.yaml
source_evidence_entry: sources/evidence-report.json
status: current
curriculum_id: curriculum.go-backend
`

func TestParseManifestV1(t *testing.T) {
	t.Parallel()
	manifest, err := ParseManifest(strings.NewReader(validManifest))
	if err != nil {
		t.Fatalf("ParseManifest() error = %v", err)
	}
	if manifest.ID.String() != "go.backend" || manifest.Version.String() != "1.2.0" ||
		manifest.SchemaVersion != SchemaVersionV1 || manifest.CurriculumID.String() != "curriculum.go-backend" ||
		len(manifest.Dependencies) != 1 || manifest.EnvironmentEntry != "environment/environment.yaml" {
		t.Fatalf("manifest = %+v", manifest)
	}
}

func TestParseManifestRejectsInvalidDocuments(t *testing.T) {
	t.Parallel()
	tests := []struct{ name, old, replacement, want string }{
		{"unknown schema", "schema_version: learning-pack/v1", "schema_version: learning-pack/v2", "unsupported pack schema"},
		{"path traversal", "curriculum_entry: curriculum/curriculum.yaml", "curriculum_entry: ../outside.yaml", "canonical relative path"},
		{"absolute path", "curriculum_entry: curriculum/curriculum.yaml", "curriculum_entry: /tmp/curriculum.yaml", "canonical relative path"},
		{"windows path", "curriculum_entry: curriculum/curriculum.yaml", `curriculum_entry: curriculum\\curriculum.yaml`, "valid portable relative path"},
		{"unstable id", "id: go.backend", "id: Go Backend", "stable portable id"},
		{"invalid dependency", `constraint: ">=1.0.0 <2.0.0"`, `constraint: "latest"`, "invalid version constraint"},
	}
	for _, test := range tests {
		test := test
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			_, err := ParseManifest(strings.NewReader(strings.Replace(validManifest, test.old, test.replacement, 1)))
			if err == nil || !strings.Contains(err.Error(), test.want) {
				t.Fatalf("ParseManifest() error = %v, want containing %q", err, test.want)
			}
		})
	}
	_, err := ParseManifest(strings.NewReader(validManifest + "mystery: true\n"))
	if err == nil || !strings.Contains(err.Error(), "field mystery not found") {
		t.Fatalf("unknown field error = %v", err)
	}
}

func TestParseManifestRejectsMultipleAndOversizedDocuments(t *testing.T) {
	t.Parallel()
	if _, err := ParseManifest(strings.NewReader(validManifest + "---\nid: another\n")); err == nil || !strings.Contains(err.Error(), "multiple documents") {
		t.Fatalf("multiple document error = %v", err)
	}
	oversized := validManifest + "#" + strings.Repeat("x", MaximumManifestBytes)
	if _, err := ParseManifest(strings.NewReader(oversized)); err == nil || !strings.Contains(err.Error(), "exceeds") {
		t.Fatalf("oversized error = %v", err)
	}
}
