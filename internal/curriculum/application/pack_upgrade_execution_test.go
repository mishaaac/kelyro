package application

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/mishaaac/kelyro/internal/audit"
	"github.com/mishaaac/kelyro/internal/backup"
	"github.com/mishaaac/kelyro/internal/curriculum"
)

func TestPackUpgradeExecutorV1AppliesPatchWithBackupMigrationIntegrityAndAudit(t *testing.T) {
	t.Parallel()
	current := installationPack(t, "pack.upgrade.patch", "1.0.0")
	candidate := installationPack(t, "pack.upgrade.patch", "1.0.1")
	candidate.Curriculum = nextDefinition(t, current.Curriculum, "2026.09.14.2")
	candidate.Curriculum.Description = "Clarified curriculum description."
	fixture := newUpgradeExecutionFixture(t, current, candidate)

	result, err := fixture.executor.Upgrade(context.Background(), PackUpgradeRequest{
		WorkspaceRoot: "/workspace", PackID: current.Manifest.ID, Confirmed: true,
	})
	if err != nil {
		t.Fatal(err)
	}
	if !result.Applied || result.BackupID != "backup.pack-upgrade" || fixture.backups.created != 1 || fixture.backups.restored != 0 ||
		fixture.migration.applied != 1 || fixture.migration.checked != 1 {
		t.Fatalf("upgrade result = %+v; backup=%+v migration=%+v", result, fixture.backups, fixture.migration)
	}
	active, err := fixture.packs.Active(context.Background(), "/workspace")
	if err != nil || active.Pack.Manifest.Version != candidate.Manifest.Version {
		t.Fatalf("active pack = %+v, %v", active, err)
	}
	if len(fixture.audits.events) != 1 || fixture.audits.events[0].Name != "pack.upgrade.completed" || fixture.audits.events[0].Metadata["backup_id"] != result.BackupID {
		t.Fatalf("audit events = %+v", fixture.audits.events)
	}
}

func TestPackUpgradeExecutorV1PlansAddedConceptAndExplicitSplitWithoutMastery(t *testing.T) {
	t.Parallel()
	t.Run("added concept", func(t *testing.T) {
		current := installationPack(t, "pack.upgrade.add", "1.0.0")
		candidate := installationPack(t, "pack.upgrade.add", "1.1.0")
		candidate.Curriculum = addedConceptDefinition(t, current.Curriculum, "2026.09.14.2")
		fixture := newUpgradeExecutionFixture(t, current, candidate)
		result, err := fixture.executor.Upgrade(context.Background(), PackUpgradeRequest{WorkspaceRoot: "/workspace", PackID: current.Manifest.ID, Confirmed: true})
		if err != nil {
			t.Fatal(err)
		}
		if !hasMigrationAction(result.MigrationPlan, curriculum.MigrationInitializeUnknown) || fixture.migration.request.Plan.ID != result.MigrationPlan.ID {
			t.Fatalf("added concept migration = %+v / %+v", result.MigrationPlan, fixture.migration.request)
		}
	})
	t.Run("split", func(t *testing.T) {
		current := installationPack(t, "pack.upgrade.split", "1.0.0")
		candidate := installationPack(t, "pack.upgrade.split", "2.0.0")
		candidate.Curriculum = splitDefinition(t, current.Curriculum, "2026.09.14.2")
		mapping := curriculum.ConceptIdentityMapping{
			OldConceptIDs: []curriculum.ConceptID{current.Curriculum.Concepts[0].ID},
			NewConceptIDs: []curriculum.ConceptID{candidate.Curriculum.Concepts[0].ID, candidate.Curriculum.Concepts[1].ID},
			Rationale:     "Reviewed split into independently assessed concepts.",
		}
		fixture := newUpgradeExecutionFixture(t, current, candidate)
		result, err := fixture.executor.Upgrade(context.Background(), PackUpgradeRequest{
			WorkspaceRoot: "/workspace", PackID: current.Manifest.ID, Confirmed: true,
			IdentityMappings: []curriculum.ConceptIdentityMapping{mapping},
		})
		if err != nil {
			t.Fatal(err)
		}
		for _, action := range result.MigrationPlan.Actions {
			if action.Kind == curriculum.MigrationSplitNoTransfer && (action.PreserveMastery || !action.InitializeUnknown || !action.RequiresStudentReview) {
				t.Fatalf("unsafe split action = %+v", action)
			}
		}
		if !hasMigrationAction(result.MigrationPlan, curriculum.MigrationSplitNoTransfer) {
			t.Fatalf("split action missing: %+v", result.MigrationPlan)
		}
	})
}

func TestPackUpgradeExecutorV1RollsBackFailureAndRequiresConfirmation(t *testing.T) {
	t.Parallel()
	current := installationPack(t, "pack.upgrade.rollback", "1.0.0")
	candidate := installationPack(t, "pack.upgrade.rollback", "1.0.1")
	candidate.Curriculum = nextDefinition(t, current.Curriculum, "2026.09.14.2")
	candidate.Curriculum.Description = "Clarified copy."
	fixture := newUpgradeExecutionFixture(t, current, candidate)

	result, err := fixture.executor.Upgrade(context.Background(), PackUpgradeRequest{WorkspaceRoot: "/workspace", PackID: current.Manifest.ID})
	if err == nil || !strings.Contains(err.Error(), "confirmation") || result.BackupID != "backup.pack-upgrade" || fixture.migration.applied != 0 {
		t.Fatalf("unconfirmed result = %+v, %v", result, err)
	}
	fixture.migration.applyErr = errors.New("injected migration failure")
	_, err = fixture.executor.Upgrade(context.Background(), PackUpgradeRequest{WorkspaceRoot: "/workspace", PackID: current.Manifest.ID, Confirmed: true})
	if err == nil || fixture.backups.restored != 1 {
		t.Fatalf("failed upgrade error = %v, backup=%+v", err, fixture.backups)
	}
	active, activeErr := fixture.packs.Active(context.Background(), "/workspace")
	if activeErr != nil || active.Pack.Manifest.Version != current.Manifest.Version {
		t.Fatalf("active pack after rollback = %+v, %v", active, activeErr)
	}
	if got := fixture.audits.events[len(fixture.audits.events)-1]; got.Name != "pack.upgrade.failed" || got.Metadata["recovered"] != "true" {
		t.Fatalf("failure audit = %+v", got)
	}
}

type upgradeExecutionFixture struct {
	executor  *PackUpgradeExecutorV1
	packs     *PackInstallerV1
	backups   *upgradeBackupFake
	migration *studentMigrationFake
	audits    *upgradeAuditFactoryFake
}

func newUpgradeExecutionFixture(t *testing.T, current, candidate curriculum.LearningPack) upgradeExecutionFixture {
	t.Helper()
	repository := newInstallationRepositoryFake()
	repository.values[installationKey(current)] = installedFixture(t, current, "a")
	repository.values[installationKey(candidate)] = installedFixture(t, candidate, "b")
	repository.active["/workspace"] = PackActivation{PackID: current.Manifest.ID, Version: current.Manifest.Version, ActivatedAt: fixedCurriculumClock(t, 30).Now()}
	packs := NewPackInstallerV1(validationFake{}, NewPackDependencyResolverV1(), repository, fixedCurriculumClock(t, 40))
	preview := NewPackUpgradePlannerV1(packs, NewCurriculumChangeClassifierV1(), NewCurriculumMigrationPlannerV1(), NewPackVersioningPolicyV1())
	backups := &upgradeBackupFake{}
	migration := &studentMigrationFake{result: StudentCurriculumMigrationResult{SourceInstancesArchived: 1, TargetInstancesCreated: 1, ConceptStatesPreserved: 1, ProjectionVersion: "test-projection-v1"}}
	audits := &upgradeAuditFactoryFake{}
	executor := NewPackUpgradeExecutorV1(preview, packs, backups, migration, audits, func(context.Context, string) (int, error) { return 5, nil }, "test")
	return upgradeExecutionFixture{executor: executor, packs: packs, backups: backups, migration: migration, audits: audits}
}

type upgradeBackupFake struct{ created, restored int }

func (fake *upgradeBackupFake) Create(context.Context, string, backup.CreateOptions) (backup.Info, error) {
	fake.created++
	return backup.Info{ID: "backup.pack-upgrade"}, nil
}
func (*upgradeBackupFake) List(context.Context, string) ([]backup.Info, error) { return nil, nil }
func (fake *upgradeBackupFake) Restore(context.Context, string, string) (backup.Info, error) {
	fake.restored++
	return backup.Info{ID: "backup.pack-upgrade"}, nil
}

type studentMigrationFake struct {
	request  StudentCurriculumMigrationRequest
	result   StudentCurriculumMigrationResult
	applyErr error
	checkErr error
	applied  int
	checked  int
}

func (fake *studentMigrationFake) Apply(_ context.Context, request StudentCurriculumMigrationRequest) (StudentCurriculumMigrationResult, error) {
	fake.applied++
	fake.request = request
	return fake.result, fake.applyErr
}
func (fake *studentMigrationFake) CheckIntegrity(context.Context, string) error {
	fake.checked++
	return fake.checkErr
}

type upgradeAuditFactoryFake struct{ events []audit.Event }

func (fake *upgradeAuditFactoryFake) Open(context.Context, string) (audit.Store, error) {
	return &upgradeAuditStoreFake{factory: fake}, nil
}

type upgradeAuditStoreFake struct{ factory *upgradeAuditFactoryFake }

func (fake *upgradeAuditStoreFake) Record(_ context.Context, event audit.Event) error {
	fake.factory.events = append(fake.factory.events, event)
	return nil
}
func (*upgradeAuditStoreFake) List(context.Context) ([]audit.Entry, error) { return nil, nil }
func (*upgradeAuditStoreFake) Close() error                                { return nil }

func hasMigrationAction(plan curriculum.CurriculumMigrationPlan, kind curriculum.CurriculumMigrationActionKind) bool {
	for _, action := range plan.Actions {
		if action.Kind == kind {
			return true
		}
	}
	return false
}

func addedConceptDefinition(t *testing.T, old curriculum.CurriculumDefinition, version string) curriculum.CurriculumDefinition {
	t.Helper()
	result := nextDefinition(t, old, version)
	addedID, _ := curriculum.NewConceptID("concept.http.responses")
	added := old.Concepts[0]
	added.ID, added.Title, added.Definition = addedID, "HTTP responses", "An HTTP response carries a server result."
	result.Concepts = append(append([]curriculum.Concept(nil), old.Concepts...), added)
	result.Competencies.Competencies = append([]curriculum.Competency(nil), old.Competencies.Competencies...)
	result.Competencies.Competencies[0].ConceptRefs = append(result.Competencies.Competencies[0].ConceptRefs, addedID)
	result.Topics = append([]curriculum.TopicSpec(nil), old.Topics...)
	result.Topics[0].ConceptIDs = append(result.Topics[0].ConceptIDs, addedID)
	return result
}
