package application

import (
	"context"
	"strings"
	"testing"

	"github.com/mishaaac/kelyro/internal/curriculum"
)

func TestPackUpgradePlannerV1DiscoversAndPlansNewestCompatibleInstalledVersion(t *testing.T) {
	t.Parallel()
	current := installationPack(t, "pack.backend.upgrade", "1.0.0")
	candidate := installationPack(t, "pack.backend.upgrade", "1.0.1")
	candidate.Curriculum = nextDefinition(t, current.Curriculum, "2026.09.14.2")
	candidate.Curriculum.Description = "Clarified display copy."
	olderCandidate := candidate
	olderCandidate.Manifest.Version, _ = curriculum.NewPackVersion("1.0.0-alpha.1")
	repository := newInstallationRepositoryFake()
	repository.values[installationKey(current)] = installedFixture(t, current, "a")
	repository.values[installationKey(candidate)] = installedFixture(t, candidate, "b")
	repository.values[installationKey(olderCandidate)] = installedFixture(t, olderCandidate, "c")
	repository.active["/workspace"] = PackActivation{PackID: current.Manifest.ID, Version: current.Manifest.Version, ActivatedAt: fixedCurriculumClock(t, 30).Now()}
	packs := NewPackInstallerV1(validationFake{}, NewPackDependencyResolverV1(), repository, fixedCurriculumClock(t, 30))
	service := NewPackUpgradePlannerV1(packs, NewCurriculumChangeClassifierV1(), NewCurriculumMigrationPlannerV1(), NewPackVersioningPolicyV1())

	result, err := service.Upgrade(context.Background(), PackUpgradeRequest{WorkspaceRoot: "/workspace", PackID: current.Manifest.ID, DryRun: true})
	if err != nil {
		t.Fatal(err)
	}
	if result.Applied || result.Candidate.Manifest.Version.String() != "1.0.1" || result.MigrationPlan.ID.String() == "" || !result.VersioningDecision.Allowed {
		t.Fatalf("upgrade dry-run = %+v", result)
	}
	if len(result.MigrationPlan.Actions) != len(current.Curriculum.Concepts) || result.MigrationPlan.Actions[0].Kind != curriculum.MigrationPreserveState {
		t.Fatalf("migration plan = %+v", result.MigrationPlan)
	}
}

func TestPackUpgradePlannerV1RejectsApplyAndInvalidSemVerTransition(t *testing.T) {
	t.Parallel()
	current := installationPack(t, "pack.backend.invalid-upgrade", "1.0.0")
	candidate := installationPack(t, "pack.backend.invalid-upgrade", "1.1.0")
	candidate.Curriculum = nextDefinition(t, current.Curriculum, "2026.09.14.2")
	candidate.Curriculum.Description = "Patch-only copy edit."
	repository := newInstallationRepositoryFake()
	repository.values[installationKey(current)] = installedFixture(t, current, "d")
	repository.values[installationKey(candidate)] = installedFixture(t, candidate, "e")
	repository.active["/workspace"] = PackActivation{PackID: current.Manifest.ID, Version: current.Manifest.Version, ActivatedAt: fixedCurriculumClock(t, 30).Now()}
	packs := NewPackInstallerV1(validationFake{}, NewPackDependencyResolverV1(), repository, fixedCurriculumClock(t, 30))
	service := NewPackUpgradePlannerV1(packs, NewCurriculumChangeClassifierV1(), NewCurriculumMigrationPlannerV1(), NewPackVersioningPolicyV1())

	if _, err := service.Upgrade(context.Background(), PackUpgradeRequest{WorkspaceRoot: "/workspace", PackID: current.Manifest.ID}); err == nil || !strings.Contains(err.Error(), "dry-run") {
		t.Fatalf("apply error = %v", err)
	}
	if _, err := service.Upgrade(context.Background(), PackUpgradeRequest{WorkspaceRoot: "/workspace", PackID: current.Manifest.ID, DryRun: true}); err == nil || !strings.Contains(err.Error(), "incompatible") {
		t.Fatalf("versioning error = %v", err)
	}
}
