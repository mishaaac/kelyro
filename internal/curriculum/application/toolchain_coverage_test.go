package application

import (
	"context"
	"reflect"
	"testing"

	"github.com/mishaaac/kelyro/internal/curriculum"
)

func TestToolchainCoverageV1ResolvesCompleteCustomToolsWithoutHardcodedInventory(t *testing.T) {
	t.Parallel()
	request, concept, reference := toolchainCoverageFixture(t)
	secondID := curriculumID(t, "tool.custom-inspector")
	request.EnvironmentPacks[0].Tools = append(request.EnvironmentPacks[0].Tools, curriculum.ToolRequirement{
		ID: secondID, DisplayName: "Inspector", Purpose: "Inspect the custom domain artifact.", MinimumVersion: "1.0.0", Level: curriculum.ToolRecommended,
		IntroducedAt: &concept.ID, WhenNeeded: &concept.ID, Platforms: []string{curriculum.EnvironmentPlatformWindows, curriculum.EnvironmentPlatformLinux},
		InstallGuidanceRefs: []curriculum.ID{curriculumID(t, "install.custom.windows"), curriculumID(t, "install.custom.linux")}, EvidenceRefs: []curriculum.EvidenceRef{reference},
	})
	secondRequirement := curriculum.ToolchainCoverageRequirement{
		ID: curriculumID(t, "toolchain.inspector"), ToolID: secondID,
		EnvironmentPack: request.Requirements[0].EnvironmentPack, MinimumLevel: curriculum.ToolOptional,
		Reason: "The goal benefits from inspecting domain artifacts.", EvidenceRefs: []curriculum.EvidenceRef{reference},
	}
	request.Requirements = append(request.Requirements, secondRequirement)

	report, err := NewToolchainCoverageV1().Analyze(context.Background(), request)
	if err != nil {
		t.Fatalf("Analyze() error = %v", err)
	}
	if len(report.Results) != 2 || len(report.CoverageRequirements) != 2 || len(report.CoverageSupports) != 2 || report.AlgorithmVersion != curriculum.ToolchainCoverageVersionV1 {
		t.Fatalf("toolchain report = %+v", report)
	}
	for _, result := range report.Results {
		if result.Status != curriculum.CoverageCovered || result.ResolvedTool == nil || len(result.MissingFields) != 0 {
			t.Fatalf("toolchain result = %+v", result)
		}
	}

	reordered := request
	reordered.Requirements = []curriculum.ToolchainCoverageRequirement{request.Requirements[1], request.Requirements[0]}
	reordered.EnvironmentPacks = []curriculum.EnvironmentPack{request.EnvironmentPacks[0]}
	reordered.EnvironmentPacks[0].Tools = []curriculum.ToolRequirement{request.EnvironmentPacks[0].Tools[1], request.EnvironmentPacks[0].Tools[0]}
	repeated, err := NewToolchainCoverageV1().Analyze(context.Background(), reordered)
	if err != nil || !reflect.DeepEqual(report, repeated) {
		t.Fatalf("reordered toolchain coverage differs: %+v / %+v / %v", report, repeated, err)
	}
}

func TestToolchainCoverageV1ReportsIncompleteMetadataAndWeakLevel(t *testing.T) {
	t.Parallel()
	request, _, _ := toolchainCoverageFixture(t)
	request.EnvironmentPacks[0].Tools[0].DisplayName = ""
	request.EnvironmentPacks[0].Tools[0].MinimumVersion = ""
	request.EnvironmentPacks[0].Tools[0].Level = curriculum.ToolRecommended
	request.EnvironmentPacks[0].Tools[0].IntroducedAt = nil
	request.EnvironmentPacks[0].Tools[0].WhenNeeded = nil
	request.EnvironmentPacks[0].Tools[0].Platforms = nil
	request.EnvironmentPacks[0].Tools[0].InstallGuidanceRefs = nil
	request.EnvironmentPacks[0].Tools[0].EvidenceRefs = nil

	report, err := NewToolchainCoverageV1().Analyze(context.Background(), request)
	if err != nil {
		t.Fatalf("Analyze() error = %v", err)
	}
	expected := []string{"display_name", "evidence", "install_guidance", "minimum_version", "platform_notes", "requirement_level", "when_introduced", "when_needed"}
	if report.Results[0].Status != curriculum.CoveragePartial || !reflect.DeepEqual(report.Results[0].MissingFields, expected) || len(report.CoverageSupports) != 0 {
		t.Fatalf("toolchain report = %+v", report)
	}
}

func TestToolchainCoverageV1ReportsMissingEnvironmentPack(t *testing.T) {
	t.Parallel()
	request, _, _ := toolchainCoverageFixture(t)
	request.EnvironmentPacks = nil
	report, err := NewToolchainCoverageV1().Analyze(context.Background(), request)
	if err != nil {
		t.Fatalf("Analyze() error = %v", err)
	}
	if report.Results[0].Status != curriculum.CoverageMissing || !reflect.DeepEqual(report.Results[0].MissingFields, []string{"environment_pack"}) || report.Results[0].ResolvedTool != nil {
		t.Fatalf("toolchain report = %+v", report)
	}
}

func TestToolchainCoverageV1ReportsMissingToolInAvailablePack(t *testing.T) {
	t.Parallel()
	request, _, _ := toolchainCoverageFixture(t)
	request.EnvironmentPacks[0].Tools[0].ID = curriculumID(t, "tool.another")
	report, err := NewToolchainCoverageV1().Analyze(context.Background(), request)
	if err != nil {
		t.Fatalf("Analyze() error = %v", err)
	}
	if report.Results[0].Status != curriculum.CoverageMissing || !reflect.DeepEqual(report.Results[0].MissingFields, []string{"tool"}) {
		t.Fatalf("toolchain report = %+v", report)
	}
}

func toolchainCoverageFixture(t *testing.T) (ToolchainCoverageRequest, curriculum.Concept, curriculum.EvidenceRef) {
	t.Helper()
	evidence, reference := decompositionEvidence(t)
	goal := decompositionGoal(t, reference, false)
	concept := graphConcept(t, "concept.custom-tool", false)
	concept.EvidenceRefs = []curriculum.EvidenceRef{reference}
	packID := curriculumID(t, "environment.custom")
	version, err := curriculum.NewPackVersion("1.2.0")
	if err != nil {
		t.Fatal(err)
	}
	toolID := curriculumID(t, "tool.custom-builder")
	pack := curriculum.EnvironmentPack{
		ID: packID, Version: version, SchemaVersion: curriculum.EnvironmentPackSchemaVersionV1,
		SupportedPlatforms: []string{curriculum.EnvironmentPlatformLinux, curriculum.EnvironmentPlatformWindows},
		InstallGuidance: []curriculum.ToolInstallGuidance{
			{ID: curriculumID(t, "install.custom.linux"), Platform: curriculum.EnvironmentPlatformLinux, SourceName: "Custom project", OfficialURL: "https://example.com/custom/linux", Instructions: "Follow official Linux guidance.", EvidenceRefs: []curriculum.EvidenceRef{reference}},
			{ID: curriculumID(t, "install.custom.windows"), Platform: curriculum.EnvironmentPlatformWindows, SourceName: "Custom project", OfficialURL: "https://example.com/custom/windows", Instructions: "Follow official Windows guidance.", EvidenceRefs: []curriculum.EvidenceRef{reference}},
		},
		Tools: []curriculum.ToolRequirement{{
			ID: toolID, DisplayName: "Custom builder", Purpose: "Build artifacts for the custom domain.", MinimumVersion: "1.0.0", Level: curriculum.ToolRequired,
			IntroducedAt: &concept.ID, WhenNeeded: &concept.ID, Platforms: []string{curriculum.EnvironmentPlatformWindows, curriculum.EnvironmentPlatformLinux},
			InstallGuidanceRefs: []curriculum.ID{curriculumID(t, "install.custom.windows"), curriculumID(t, "install.custom.linux")}, EvidenceRefs: []curriculum.EvidenceRef{reference},
		}},
	}
	requirement := curriculum.ToolchainCoverageRequirement{
		ID: curriculumID(t, "toolchain.builder"), ToolID: toolID,
		EnvironmentPack: curriculum.EnvironmentPackReference{ID: packID, Version: version}, MinimumLevel: curriculum.ToolRequired,
		Reason: "The declared goal requires its domain-specific builder.", EvidenceRefs: []curriculum.EvidenceRef{reference},
	}
	return ToolchainCoverageRequest{
		GoalID: goal.ID, Concepts: []curriculum.Concept{concept}, Requirements: []curriculum.ToolchainCoverageRequirement{requirement},
		EnvironmentPacks: []curriculum.EnvironmentPack{pack}, EvidenceSets: []curriculum.CurriculumEvidenceSet{evidence},
	}, concept, reference
}
