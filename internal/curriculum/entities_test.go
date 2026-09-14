package curriculum

import (
	"strings"
	"testing"
	"time"
)

func TestPackEnvironmentCompilationAndChangeShapesValidate(t *testing.T) {
	t.Parallel()

	definition := validCurriculumDefinition(t)
	packVersion, err := NewPackVersion("0.1.0-alpha.1")
	if err != nil {
		t.Fatal(err)
	}
	createdAt, err := NewTimestamp(time.Date(2026, 9, 9, 19, 0, 0, 0, time.UTC))
	if err != nil {
		t.Fatal(err)
	}
	environment := EnvironmentPack{
		ID: mustID(t, "environment.backend"), Version: packVersion, SchemaVersion: EnvironmentPackSchemaVersionV1,
		SupportedPlatforms: []string{EnvironmentPlatformLinux, EnvironmentPlatformDarwin, EnvironmentPlatformWindows},
		InstallGuidance: []ToolInstallGuidance{
			{ID: mustID(t, "install.compiler.linux"), Platform: EnvironmentPlatformLinux, SourceName: "Compiler", OfficialURL: "https://example.com/compiler/linux", Instructions: "Follow the official Linux instructions.", EvidenceRefs: definition.Concepts[0].EvidenceRefs},
			{ID: mustID(t, "install.compiler.darwin"), Platform: EnvironmentPlatformDarwin, SourceName: "Compiler", OfficialURL: "https://example.com/compiler/darwin", Instructions: "Follow the official macOS instructions.", EvidenceRefs: definition.Concepts[0].EvidenceRefs},
			{ID: mustID(t, "install.compiler.windows"), Platform: EnvironmentPlatformWindows, SourceName: "Compiler", OfficialURL: "https://example.com/compiler/windows", Instructions: "Follow the official Windows instructions.", EvidenceRefs: definition.Concepts[0].EvidenceRefs},
		},
		Tools: []ToolRequirement{{
			ID: mustID(t, "tool.compiler"), DisplayName: "Compiler", Purpose: "Compile examples", MinimumVersion: "1.0.0", Level: ToolRequired,
			IntroducedAt: &definition.Concepts[0].ID, WhenNeeded: &definition.Concepts[0].ID,
			Platforms:           []string{EnvironmentPlatformLinux, EnvironmentPlatformDarwin, EnvironmentPlatformWindows},
			InstallGuidanceRefs: []ID{mustID(t, "install.compiler.linux"), mustID(t, "install.compiler.darwin"), mustID(t, "install.compiler.windows")},
			EvidenceRefs:        definition.Concepts[0].EvidenceRefs,
		}},
	}
	pack := LearningPack{
		Manifest: PackManifest{
			ID: mustID(t, "pack.backend"), Name: "Backend", Description: "Backend learning pack.",
			Version: packVersion, SchemaVersion: "learning-pack/v1", Domain: "software-engineering",
			Target: "Backend engineer", Authors: []string{"Kelyro"}, Maintainers: []string{"Kelyro"},
			License: "CC-BY-4.0", CreatedAt: createdAt, MinimumKelyroVersion: packVersion,
			CurriculumEntry: "curriculum/curriculum.yaml", SourceEvidenceEntry: "sources/evidence-report.json",
			Status: ConceptPreview, CurriculumID: definition.ID,
		},
		Curriculum: definition, Environment: &environment,
	}
	if err := pack.Validate(); err != nil {
		t.Fatalf("valid learning pack rejected: %v", err)
	}

	input := CompilationInput{Goal: definition.Goal, SourceBundles: definition.SourceBundles, RequestedAt: createdAt}
	config := CompilationConfig{CompilerVersion: "curriculum-compiler-v1", SourcePolicy: SourceReferencesRequired}
	if err := input.Validate(config.SourcePolicy); err != nil {
		t.Fatalf("valid compilation input rejected: %v", err)
	}
	if err := config.Validate(); err != nil {
		t.Fatalf("valid compilation config rejected: %v", err)
	}
	pass := CompilationPass{Name: "validate-input", Version: "validate-input-v1", InputHash: "sha256:input", OutputHash: "sha256:output", Duration: time.Millisecond}
	result := CompilationResult{Curriculum: definition, Passes: []CompilationPass{pass}}
	if err := result.Validate(); err != nil {
		t.Fatalf("valid compilation result rejected: %v", err)
	}

	nextVersion, err := NewCurriculumVersion("2026.09.09.2")
	if err != nil {
		t.Fatal(err)
	}
	change := CurriculumChange{
		ID: mustID(t, "change.one"), FromVersion: definition.Version, ToVersion: nextVersion,
		Kind: ChangeConceptAdded, Migration: MigrationRequiresRecompile,
		AffectedConcepts: []ConceptID{definition.Concepts[1].ID}, Rationale: "Adds evidenced application detail.",
	}
	if err := change.Validate(); err != nil {
		t.Fatalf("valid curriculum change rejected: %v", err)
	}
}

func TestClosedDomainVocabulariesRejectUnknownValues(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		err  error
	}{
		{"concept status", ConceptStatus("unknown").Validate()},
		{"atomicity", Atomicity("unknown-new-state").Validate()},
		{"difficulty", Difficulty(6).Validate()},
		{"competency level", CompetencyLevel("expert").Validate()},
		{"prerequisite kind", PrerequisiteKind("implicit").Validate()},
		{"coverage dimension", CoverageDimension("overall-score").Validate()},
		{"gap severity", GapSeverity("fatal").Validate()},
		{"migration class", MigrationClass("automatic").Validate()},
	}
	for _, test := range tests {
		if test.err == nil || !strings.Contains(test.err.Error(), "invalid") && !strings.Contains(test.err.Error(), "outside") {
			t.Errorf("%s accepted unknown value: %v", test.name, test.err)
		}
	}
}
