package application_test

import (
	"context"
	"errors"
	"reflect"
	"testing"
	"time"

	"github.com/mishaaac/kelyro/internal/learning"
	"github.com/mishaaac/kelyro/internal/learning/application"
	"github.com/mishaaac/kelyro/internal/learning/application/memory"
)

func TestMaintenanceRecalculationDryRunApplyAndV1Idempotency(t *testing.T) {
	t.Parallel()
	fixture := newMaintenanceFixture(t)

	dry, err := fixture.service.Recalculate(fixture.ctx, application.RecalculationRequest{DryRun: true})
	if err != nil {
		t.Fatal(err)
	}
	if !dry.DryRun || dry.Applied || dry.BackupID != "" || dry.EvidenceRecords != 1 || dry.ConceptsScanned != 1 ||
		dry.ConceptStatesChanged != 1 || dry.RetentionStatesChanged != 1 || dry.ReviewSchedulesChanged != 1 ||
		dry.ReviewItemsChanged != 1 || dry.DailyPlansChanged != 1 {
		t.Fatalf("dry-run impact = %+v", dry)
	}
	if states, err := fixture.repositories.InstanceConceptStates.ListByInstance(fixture.ctx, fixture.instance.ID); err != nil || len(states) != 0 {
		t.Fatalf("dry-run states = (%+v, %v), want unchanged", states, err)
	}
	if _, err := fixture.repositories.Retention.Get(fixture.ctx, fixture.student.ID, fixture.conceptID); !errors.Is(err, application.ErrNotFound) {
		t.Fatalf("dry-run retention error = %v, want not found", err)
	}

	applied, err := fixture.service.Recalculate(fixture.ctx, application.RecalculationRequest{BackupID: "backup.step29"})
	if err != nil {
		t.Fatal(err)
	}
	if !applied.Applied || applied.DryRun || applied.BackupID != "backup.step29" || applied.ConceptStatesChanged != dry.ConceptStatesChanged {
		t.Fatalf("applied impact = %+v", applied)
	}
	state, err := fixture.repositories.InstanceConceptStates.Get(fixture.ctx, fixture.instance.ID, fixture.conceptID)
	if err != nil || state.MasteryAlgorithmVersion != learning.MasteryAlgorithmVersion ||
		state.ProgressionPolicyVersion != learning.ProgressionPolicyVersion || state.Exposure != learning.ExposureReviewDue {
		t.Fatalf("persisted concept state = (%+v, %v)", state, err)
	}
	retention, err := fixture.repositories.Retention.Get(fixture.ctx, fixture.student.ID, fixture.conceptID)
	if err != nil || retention.AlgorithmVersion != learning.RetentionAlgorithmVersion {
		t.Fatalf("persisted retention = (%+v, %v)", retention, err)
	}

	unchanged, err := fixture.service.Recalculate(fixture.ctx, application.RecalculationRequest{DryRun: true})
	if err != nil {
		t.Fatal(err)
	}
	if unchanged.ConceptStatesChanged != 0 || unchanged.RetentionStatesChanged != 0 || unchanged.ReviewSchedulesChanged != 0 ||
		unchanged.ReviewItemsChanged != 0 || unchanged.DailyPlansChanged != 0 {
		t.Fatalf("same-clock v1 recalculation is not idempotent: %+v", unchanged)
	}
}

func TestMaintenanceRecalculationSupportsSimulatedV2AndPreservesEvidence(t *testing.T) {
	t.Parallel()
	fixture := newMaintenanceFixture(t)
	if _, err := fixture.service.Recalculate(fixture.ctx, application.RecalculationRequest{BackupID: "backup.v1"}); err != nil {
		t.Fatal(err)
	}
	beforeEvidence, err := fixture.repositories.Evidence.ListByStudent(fixture.ctx, fixture.student.ID)
	if err != nil {
		t.Fatal(err)
	}

	suite := application.DefaultLearningAlgorithmSuite()
	suite.Mastery = fakeMasteryV2{}
	suite.Retention = fakeRetentionV2{}
	v2 := application.NewMaintenanceRecalculationService(fixture.profiles, fixture.mastery, fixture.store,
		application.WithMaintenanceRecalculationClock(func() time.Time { return fixture.now }),
		application.WithLearningAlgorithmSuite(suite))
	impact, err := v2.Recalculate(fixture.ctx, application.RecalculationRequest{BackupID: "backup.v2"})
	if err != nil {
		t.Fatal(err)
	}
	if impact.ConceptStatesChanged != 1 || impact.RetentionStatesChanged != 1 ||
		!reflect.DeepEqual(impact.Target.Mastery, []string{"mastery-v2-fake"}) ||
		!reflect.DeepEqual(impact.Target.Retention, []string{"retention-v2-fake"}) {
		t.Fatalf("v2 impact = %+v", impact)
	}
	state, err := fixture.repositories.InstanceConceptStates.Get(fixture.ctx, fixture.instance.ID, fixture.conceptID)
	if err != nil || state.MasteryAlgorithmVersion != "mastery-v2-fake" {
		t.Fatalf("v2 state = (%+v, %v)", state, err)
	}
	retention, err := fixture.repositories.Retention.Get(fixture.ctx, fixture.student.ID, fixture.conceptID)
	if err != nil || retention.AlgorithmVersion != "retention-v2-fake" {
		t.Fatalf("v2 retention = (%+v, %v)", retention, err)
	}
	afterEvidence, err := fixture.repositories.Evidence.ListByStudent(fixture.ctx, fixture.student.ID)
	if err != nil || !reflect.DeepEqual(afterEvidence, beforeEvidence) {
		t.Fatalf("evidence changed by v2 recalculation: before=%+v after=%+v err=%v", beforeEvidence, afterEvidence, err)
	}
}

func TestMaintenanceRecalculationRequiresBackupAndRollsBack(t *testing.T) {
	t.Parallel()
	fixture := newMaintenanceFixture(t)
	if _, err := fixture.service.Recalculate(fixture.ctx, application.RecalculationRequest{}); !errors.Is(err, application.ErrInvalidState) {
		t.Fatalf("apply without backup error = %v, want invalid state", err)
	}
	if _, err := fixture.service.Recalculate(fixture.ctx, application.RecalculationRequest{BackupID: "backup.v1"}); err != nil {
		t.Fatal(err)
	}
	before, err := fixture.repositories.InstanceConceptStates.Get(fixture.ctx, fixture.instance.ID, fixture.conceptID)
	if err != nil {
		t.Fatal(err)
	}

	suite := application.DefaultLearningAlgorithmSuite()
	suite.Mastery = fakeMasteryV2{}
	suite.Retention = fakeRetentionV2{}
	failing := application.NewMaintenanceRecalculationService(fixture.profiles, fixture.mastery,
		failingRetentionUnitOfWork{store: fixture.store}, application.WithMaintenanceRecalculationClock(func() time.Time { return fixture.now }),
		application.WithLearningAlgorithmSuite(suite))
	if _, err := failing.Recalculate(fixture.ctx, application.RecalculationRequest{BackupID: "backup.rollback"}); err == nil {
		t.Fatal("recalculation with failing retention repository succeeded")
	}
	after, err := fixture.repositories.InstanceConceptStates.Get(fixture.ctx, fixture.instance.ID, fixture.conceptID)
	if err != nil || !reflect.DeepEqual(after, before) {
		t.Fatalf("state after rollback = (%+v, %v), want %+v", after, err, before)
	}
}

type maintenanceFixture struct {
	ctx          context.Context
	store        *memory.Store
	repositories application.Repositories
	profiles     application.ProfileService
	mastery      application.MasteryPolicyService
	service      application.MaintenanceRecalculationService
	student      learning.Student
	instance     learning.CurriculumInstance
	conceptID    learning.ID
	now          time.Time
}

func newMaintenanceFixture(t *testing.T) maintenanceFixture {
	t.Helper()
	ctx := context.Background()
	store := memory.New()
	repositories := store.Repositories()
	createdAt := time.Date(2026, 8, 1, 12, 0, 0, 0, time.UTC)
	profiles := application.NewProfileService(application.NewStudentService(repositories.Students),
		application.WithProfileClock(func() time.Time { return createdAt }))
	student, err := profiles.Show(ctx)
	if err != nil {
		t.Fatal(err)
	}
	goals := application.NewGoalLifecycleService(profiles, store, application.WithGoalClock(func() time.Time { return createdAt }),
		application.WithGoalIDGenerator(func() (learning.ID, error) { return testID(t, "goal.maintenance"), nil }))
	goal, err := goals.Set(ctx, goalInput(t, "Maintenance", "General knowledge"))
	if err != nil {
		t.Fatal(err)
	}
	instances := application.NewCurriculumInstanceService(profiles, store,
		application.WithCurriculumInstanceClock(func() time.Time { return createdAt }),
		application.WithCurriculumInstanceIDGenerator(func() (learning.ID, error) { return testID(t, "instance.maintenance"), nil }))
	instance, err := instances.Create(ctx, goal.ID, instanceTestCurriculum(t, "maintenance-v1"), learning.CurriculumSourceFixture)
	if err != nil {
		t.Fatal(err)
	}
	conceptID := testID(t, "concept.a")
	observedAt := fixtureTimestamp(t, createdAt.Add(time.Hour))
	evidence, err := learning.NewEvidenceWithMetadata(testID(t, "evidence.maintenance"), student.ID, conceptID,
		learning.EvidenceAssessment, "fixture/maintenance", testScore(t, .9), learning.EvidenceMetadata{
			Confidence: 1, Independence: 1, Difficulty: .5, AlgorithmVersion: "fixture/maintenance-v1",
		}, observedAt)
	if err != nil {
		t.Fatal(err)
	}
	if err := repositories.Evidence.Append(ctx, evidence); err != nil {
		t.Fatal(err)
	}
	mastery := application.NewMasteryPolicyService(profiles, repositories.Mastery,
		application.WithMasteryPolicyClock(func() time.Time { return createdAt }))
	now := createdAt.Add(30 * 24 * time.Hour)
	service := application.NewMaintenanceRecalculationService(profiles, mastery, store,
		application.WithMaintenanceRecalculationClock(func() time.Time { return now }))
	return maintenanceFixture{ctx: ctx, store: store, repositories: repositories, profiles: profiles, mastery: mastery,
		service: service, student: student, instance: instance, conceptID: conceptID, now: now}
}

type fakeMasteryV2 struct{}

func (fakeMasteryV2) Version() string { return "mastery-v2-fake" }
func (fakeMasteryV2) Calculate(studentID, conceptID learning.ID, evidence []learning.Evidence) (learning.MasteryCalculation, error) {
	calculation, err := learning.CalculateMasteryV1(studentID, conceptID, evidence)
	calculation.PolicyVersion = "mastery-v2-fake"
	return calculation, err
}

type fakeRetentionV2 struct{}

func (fakeRetentionV2) Version() string { return "retention-v2-fake" }
func (fakeRetentionV2) Calculate(mastery learning.MasteryCalculation, evidence []learning.Evidence, measuredAt learning.Timestamp) (learning.RetentionCalculation, error) {
	calculation, err := learning.CalculateRetentionV1(mastery, evidence, measuredAt)
	calculation.State.AlgorithmVersion = "retention-v2-fake"
	return calculation, err
}

type failingRetentionUnitOfWork struct{ store *memory.Store }

func (unit failingRetentionUnitOfWork) WithinTransaction(ctx context.Context, work func(application.Repositories) error) error {
	return unit.store.WithinTransaction(ctx, func(repositories application.Repositories) error {
		repositories.Retention = failingRetentionRepository{RetentionRepository: repositories.Retention}
		return work(repositories)
	})
}

type failingRetentionRepository struct {
	application.RetentionRepository
}

func (failingRetentionRepository) Save(context.Context, learning.RetentionState) error {
	return errors.New("injected retention write failure")
}
