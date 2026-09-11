package learningpack

import (
	"archive/zip"
	"bytes"
	"context"
	"crypto/sha256"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"testing"

	curriculumapp "github.com/mishaaac/kelyro/internal/curriculum/application"
)

const validCurriculum = `id: curriculum.go-backend
version: 2026.09.11.1
title: Go Backend
description: A compact source-backed curriculum.
goal:
  id: goal.go-backend
  title: Backend Engineer with Go
  description: Prepare for backend work with Go.
  domain: software-engineering
  outcomes:
    - id: outcome.explain
      statement: Explain Go package structure.
      evidence_refs: &evidence
        - bundle_id: bundle.go-packages
          claim_id: claim.go-packages
competency_matrix:
  version: competency-matrix-v1
  goal_id: goal.go-backend
  competencies:
    - id: competency.packages
      area: go-language
      outcome_id: outcome.explain
      expected_level: understand
      evidence_refs: *evidence
      concept_refs: [concept.go-package]
concepts:
  - id: concept.go-package
    title: Go package
    definition: A package groups Go source files in one namespace.
    version: "1"
    atomicity: atomic
    difficulty: 1
    status: current
    foundational: true
    evidence_refs: *evidence
phases:
  - id: phase.foundation
    title: Foundation
    description: Language foundations.
    order: 0
modules:
  - id: module.packages
    phase_id: phase.foundation
    title: Packages
    description: Package organization.
    order: 0
lessons:
  - id: lesson.packages
    module_id: module.packages
    title: Package basics
    description: Package declarations and organization.
    order: 0
topics:
  - id: topic.packages
    lesson_id: lesson.packages
    title: Package declaration
    description: The package clause.
    order: 0
    concept_ids: [concept.go-package]
source_policy: required
source_bundles:
  - id: bundle.go-packages
    content_hash: sha256:aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa
    algorithm_version: source-bundle-v1
    verified_at: 2026-09-11T14:00:00Z
created_at: 2026-09-11T15:00:00Z
`

const validEvidenceReport = `{
  "schema_version": "curriculum-evidence/v1",
  "bundles": [{
    "id": "bundle.go-packages",
    "content_hash": "sha256:aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa",
    "algorithm_version": "source-bundle-v1",
    "verified_at": "2026-09-11T14:00:00Z"
  }],
  "claims": [{"bundle_id": "bundle.go-packages", "claim_id": "claim.go-packages"}]
}
`

const validEnvironment = `id: environment.go
version: 1.0.0
tools:
  - id: tool.go
    purpose: Compile Go programs.
    level: required
    introduced_at: concept.go-package
    platforms: [linux, macos, windows]
    evidence_refs:
      - bundle_id: bundle.go-packages
        claim_id: claim.go-packages
`

func TestValidatorLoadsDirectoryAndZIP(t *testing.T) {
	t.Parallel()
	for _, archive := range []bool{false, true} {
		archive := archive
		t.Run(fmt.Sprintf("archive=%v", archive), func(t *testing.T) {
			t.Parallel()
			entries := validPackEntries()
			var source string
			if archive {
				source = writeZIP(t, entries, nil)
			} else {
				source = writeDirectory(t, entries)
			}
			result, err := NewValidator().Validate(context.Background(), curriculumapp.PackSource{Path: source})
			if err != nil || len(result.Errors) != 0 || result.Pack == nil {
				t.Fatalf("Validate() result=%+v error=%v", result, err)
			}
			if result.Pack.Manifest.ID.String() != "go.backend" || result.Pack.Environment == nil || result.Pack.Curriculum.ID.String() != "curriculum.go-backend" {
				t.Fatalf("pack = %+v", result.Pack)
			}
		})
	}
}

func TestValidatorRejectsChecksumUTF8AndEvidenceFailures(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name   string
		mutate func(map[string][]byte)
		want   string
	}{
		{"checksum mismatch", func(entries map[string][]byte) { entries["README.md"] = []byte("changed") }, "checksum mismatch"},
		{"invalid UTF-8", func(entries map[string][]byte) { entries["README.md"] = []byte{0xff}; updateChecksums(entries) }, "valid UTF-8"},
		{"missing evidence claim", func(entries map[string][]byte) {
			entries["sources/evidence-report.json"] = []byte(strings.Replace(validEvidenceReport, "claim.go-packages", "claim.other", 1))
			updateChecksums(entries)
		}, "missing from evidence report"},
		{"duplicate JSON key", func(entries map[string][]byte) {
			entries["sources/evidence-report.json"] = []byte(strings.Replace(validEvidenceReport, `"schema_version": "curriculum-evidence/v1",`, `"schema_version": "curriculum-evidence/v1", "schema_version": "curriculum-evidence/v1",`, 1))
			updateChecksums(entries)
		}, "duplicate JSON key"},
		{"unknown curriculum field", func(entries map[string][]byte) {
			entries["curriculum/curriculum.yaml"] = append(entries["curriculum/curriculum.yaml"], []byte("unknown: true\n")...)
			updateChecksums(entries)
		}, "field unknown not found"},
	}
	for _, test := range tests {
		test := test
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			entries := validPackEntries()
			test.mutate(entries)
			result, err := NewValidator().Validate(context.Background(), curriculumapp.PackSource{Path: writeDirectory(t, entries)})
			if err != nil || len(result.Errors) != 1 || !strings.Contains(result.Errors[0].Message, test.want) {
				t.Fatalf("Validate() result=%+v error=%v, want %q", result, err, test.want)
			}
		})
	}
}

func TestValidatorRejectsUnsafeDirectoryAndArchiveEntries(t *testing.T) {
	t.Parallel()
	directory := writeDirectory(t, validPackEntries())
	if err := os.Symlink(filepath.Join(directory, "pack.yaml"), filepath.Join(directory, "linked.yaml")); err != nil {
		t.Fatal(err)
	}
	result, err := NewValidator().Validate(context.Background(), curriculumapp.PackSource{Path: directory})
	if err != nil || len(result.Errors) != 1 || !strings.Contains(result.Errors[0].Message, "symlink") {
		t.Fatalf("symlink result=%+v err=%v", result, err)
	}

	for _, test := range []struct {
		name   string
		header *zip.FileHeader
		want   string
	}{
		{"traversal", &zip.FileHeader{Name: "../escape"}, "canonical relative path"},
		{"duplicate", &zip.FileHeader{Name: ManifestName}, "duplicate archive entry"},
		{"symlink", symlinkHeader("link"), "symlink archive entry"},
		{"executable", executableHeader("bin/tool"), "executable archive entry"},
	} {
		test := test
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			source := writeZIP(t, validPackEntries(), test.header)
			result, err := NewValidator().Validate(context.Background(), curriculumapp.PackSource{Path: source})
			if err != nil || len(result.Errors) != 1 || !strings.Contains(result.Errors[0].Message, test.want) {
				t.Fatalf("Validate() result=%+v error=%v, want %q", result, err, test.want)
			}
		})
	}
}

func validPackEntries() map[string][]byte {
	manifest := strings.Replace(validManifest, "version: 1.2.0", "version: 1.0.0", 1)
	manifest = strings.Replace(manifest, "constraint: \">=1.0.0 <2.0.0\"", "constraint: \"1.0.0\"", 1)
	entries := map[string][]byte{ManifestName: []byte(manifest), "curriculum/curriculum.yaml": []byte(validCurriculum), "sources/evidence-report.json": []byte(validEvidenceReport), "environment/environment.yaml": []byte(validEnvironment), "README.md": []byte("# Go Backend\n")}
	updateChecksums(entries)
	return entries
}

func updateChecksums(entries map[string][]byte) {
	delete(entries, ChecksumsName)
	names := make([]string, 0, len(entries))
	for name := range entries {
		names = append(names, name)
	}
	sort.Strings(names)
	var output strings.Builder
	for _, name := range names {
		sum := sha256.Sum256(entries[name])
		fmt.Fprintf(&output, "%x  %s\n", sum, name)
	}
	entries[ChecksumsName] = []byte(output.String())
}

func writeDirectory(t *testing.T, entries map[string][]byte) string {
	t.Helper()
	root := t.TempDir()
	for name, encoded := range entries {
		target := filepath.Join(root, filepath.FromSlash(name))
		if err := os.MkdirAll(filepath.Dir(target), 0755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(target, encoded, 0644); err != nil {
			t.Fatal(err)
		}
	}
	return root
}

func writeZIP(t *testing.T, entries map[string][]byte, extra *zip.FileHeader) string {
	t.Helper()
	target := filepath.Join(t.TempDir(), "pack.zip")
	file, err := os.Create(target)
	if err != nil {
		t.Fatal(err)
	}
	archive := zip.NewWriter(file)
	names := make([]string, 0, len(entries))
	for name := range entries {
		names = append(names, name)
	}
	sort.Strings(names)
	for _, name := range names {
		header := &zip.FileHeader{Name: name, Method: zip.Store}
		header.SetMode(0644)
		writer, err := archive.CreateHeader(header)
		if err != nil {
			t.Fatal(err)
		}
		if _, err := writer.Write(entries[name]); err != nil {
			t.Fatal(err)
		}
	}
	if extra != nil {
		writer, err := archive.CreateHeader(extra)
		if err != nil {
			t.Fatal(err)
		}
		if _, err := writer.Write([]byte("extra")); err != nil {
			t.Fatal(err)
		}
	}
	if err := archive.Close(); err != nil {
		t.Fatal(err)
	}
	if err := file.Close(); err != nil {
		t.Fatal(err)
	}
	return target
}

func symlinkHeader(name string) *zip.FileHeader {
	header := &zip.FileHeader{Name: name, Method: zip.Store}
	header.SetMode(os.ModeSymlink | 0777)
	return header
}
func executableHeader(name string) *zip.FileHeader {
	header := &zip.FileHeader{Name: name, Method: zip.Store}
	header.SetMode(0755)
	return header
}

func TestValidatorHonorsCancellation(t *testing.T) {
	t.Parallel()
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	_, err := NewValidator().Validate(ctx, curriculumapp.PackSource{Path: "ignored"})
	if err == nil {
		t.Fatal("Validate() accepted canceled context")
	}
}

func TestChecksumFixtureIsDeterministic(t *testing.T) {
	t.Parallel()
	first, second := validPackEntries(), validPackEntries()
	if !bytes.Equal(first[ChecksumsName], second[ChecksumsName]) {
		t.Fatal("checksums are not deterministic")
	}
}
