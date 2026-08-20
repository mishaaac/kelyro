package application_test

import (
	"context"
	"errors"
	"fmt"
	"testing"
	"time"

	"github.com/mishaaac/kelyro/internal/learning"
	"github.com/mishaaac/kelyro/internal/learning/application"
	"github.com/mishaaac/kelyro/internal/learning/application/memory"
)

func TestLearnerSetupOptOutInitializesEveryConceptWithoutInventingMastery(t *testing.T) {
	t.Parallel()
	fixture := newLearnerSetupFixture(t, false, true)
	view := completeSetupOnboarding(t, fixture.service, "no")
	if view.Setup.Status != learning.SetupCompleted || view.Setup.SetupCompletedAt == nil || view.Instance == nil || view.Diagnostic != nil {
		t.Fatalf("completed opt-out setup = %+v", view)
	}
	states, err := fixture.repositories.InstanceConceptStates.ListByInstance(context.Background(), view.Instance.ID)
	if err != nil || len(states) != 2 {
		t.Fatalf("initial states = (%+v, %v)", states, err)
	}
	for _, state := range states {
		if state.Exposure != learning.ExposureNotSeen || state.Mastery.Value() != 0 || state.FirstSeenAt != nil {
			t.Fatalf("initial state invented learning = %+v", state)
		}
	}
}

func TestLearnerSetupDiagnosticIsPartialResumableAndCompletesAtomically(t *testing.T) {
	t.Parallel()
	fixture := newLearnerSetupFixture(t, false, true)
	view := completeSetupOnboarding(t, fixture.service, "yes")
	if view.Setup.Status != learning.SetupAwaitingDiagnostic || view.Diagnostic == nil || view.Diagnostic.Item == nil {
		t.Fatalf("diagnostic setup = %+v", view)
	}
	view, err := fixture.service.SubmitDiagnostic(context.Background(), []string{"a"})
	if err != nil || view.Setup.Status != learning.SetupAwaitingDiagnostic || view.Diagnostic == nil || len(view.Diagnostic.Attempt.Observations) != 1 {
		t.Fatalf("partial diagnostic = (%+v, %v)", view, err)
	}
	resumed, err := fixture.service.Show(context.Background())
	if err != nil || resumed.Diagnostic == nil || resumed.Diagnostic.Attempt.ID != view.Diagnostic.Attempt.ID || len(resumed.Diagnostic.Attempt.Observations) != 1 {
		t.Fatalf("resumed diagnostic = (%+v, %v)", resumed, err)
	}
	completed, err := fixture.service.SubmitDiagnostic(context.Background(), []string{"b"})
	if err != nil || completed.Setup.Status != learning.SetupCompleted || completed.Setup.SetupCompletedAt == nil || completed.Diagnostic != nil {
		t.Fatalf("completed diagnostic setup = (%+v, %v)", completed, err)
	}
	for _, conceptID := range []learning.ID{testID(t, "concept.a"), testID(t, "concept.b")} {
		evidence, listErr := fixture.repositories.Evidence.ListByConcept(context.Background(), fixture.student.ID, conceptID)
		if listErr != nil || len(evidence) != 1 || evidence[0].Type != learning.EvidenceDiagnostic {
			t.Fatalf("diagnostic evidence for %s = (%+v, %v)", conceptID, evidence, listErr)
		}
	}
}

func TestLearnerSetupDoesNotCompleteWhenInitialStateTransactionFailsAndCanRecover(t *testing.T) {
	t.Parallel()
	fixture := newLearnerSetupFixture(t, true, true)
	_, err := completeSetupOnboardingResult(fixture.service, "no")
	if !errors.Is(err, application.ErrPersistenceFailure) {
		t.Fatalf("Confirm() error = %v, want persistence failure", err)
	}
	studentSetup, err := fixture.repositories.Setup.Get(context.Background(), fixture.student.ID)
	if err != nil || studentSetup.Status != learning.SetupInitializing || studentSetup.SetupCompletedAt != nil {
		t.Fatalf("setup after failed finalize = (%+v, %v)", studentSetup, err)
	}
	states, err := fixture.repositories.InstanceConceptStates.ListByInstance(context.Background(), *studentSetup.CurriculumInstanceID)
	if err != nil || len(states) != 0 {
		t.Fatalf("states after rollback = (%+v, %v)", states, err)
	}

	recoveredService := application.NewLearnerSetupService(fixture.profiles, fixture.onboarding, fixture.instances, fixture.diagnostics,
		fixture.store, fixture.curriculum, fixture.diagnostic, application.WithLearnerSetupClock(fixture.clock))
	recovered, err := recoveredService.Show(context.Background())
	if err != nil || recovered.Setup.Status != learning.SetupCompleted || recovered.Setup.SetupCompletedAt == nil {
		t.Fatalf("recovered setup = (%+v, %v)", recovered, err)
	}
}

func TestLearnerSetupDevelopmentResetIsGatedAndPreservesProfileAndGoalHistory(t *testing.T) {
	t.Parallel()
	blocked := newLearnerSetupFixture(t, false, false)
	if _, err := blocked.service.ResetDevelopment(context.Background()); !errors.Is(err, application.ErrInvalidState) {
		t.Fatalf("production reset error = %v", err)
	}
	fixture := newLearnerSetupFixture(t, false, true)
	completed := completeSetupOnboarding(t, fixture.service, "no")
	reset, err := fixture.service.ResetDevelopment(context.Background())
	if err != nil || reset.Setup.Status != learning.SetupAwaitingOnboarding || reset.Onboarding == nil || reset.Onboarding.Interview.Status != learning.OnboardingInProgress {
		t.Fatalf("ResetDevelopment() = (%+v, %v)", reset, err)
	}
	if _, err := fixture.profiles.Show(context.Background()); err != nil {
		t.Fatalf("profile removed: %v", err)
	}
	goals, err := fixture.goals.Show(context.Background())
	if err != nil || len(goals) != 1 {
		t.Fatalf("goal history = (%+v, %v)", goals, err)
	}
	if _, err := fixture.repositories.CurriculumInstances.Get(context.Background(), completed.Instance.ID); !errors.Is(err, application.ErrNotFound) {
		t.Fatalf("old setup instance error = %v, want not found", err)
	}
}

type learnerSetupFixture struct {
	store        *memory.Store
	repositories application.Repositories
	profiles     application.ProfileService
	goals        application.GoalLifecycleService
	onboarding   application.OnboardingService
	instances    application.CurriculumInstanceService
	diagnostics  application.DiagnosticService
	service      application.LearnerSetupService
	student      learning.Student
	curriculum   learning.Curriculum
	diagnostic   learning.Diagnostic
	clock        func() time.Time
}

func newLearnerSetupFixture(t *testing.T, failStateSave, allowReset bool) learnerSetupFixture {
	t.Helper()
	ctx := context.Background()
	store := memory.New()
	repositories := store.Repositories()
	current := time.Date(2026, time.August, 19, 9, 0, 0, 0, time.UTC)
	clock := func() time.Time { value := current; current = current.Add(time.Minute); return value }
	profiles := application.NewProfileService(application.NewStudentService(repositories.Students), application.WithProfileClock(clock))
	student, err := profiles.Show(ctx)
	if err != nil {
		t.Fatal(err)
	}
	goalIndex := 0
	goals := application.NewGoalLifecycleService(profiles, store, application.WithGoalClock(clock), application.WithGoalIDGenerator(func() (learning.ID, error) {
		goalIndex++
		return testID(t, fmt.Sprintf("goal.setup.%d", goalIndex)), nil
	}))
	mastery := application.NewMasteryPolicyService(profiles, repositories.Mastery, application.WithMasteryPolicyClock(clock))
	onboarding := application.NewOnboardingService(profiles, goals, repositories.Onboarding, application.WithOnboardingClock(clock), application.WithOnboardingMasteryPolicy(mastery))
	curriculum := instanceTestCurriculum(t, "1.0.0")
	instanceIndex := 0
	instances := application.NewCurriculumInstanceService(profiles, store, application.WithCurriculumInstanceClock(clock), application.WithCurriculumInstanceIDGenerator(func() (learning.ID, error) {
		instanceIndex++
		return testID(t, fmt.Sprintf("instance.setup.%d", instanceIndex)), nil
	}))
	diagnostic := applicationDiagnosticFixture(t, curriculum.Reference)
	attemptIndex, evidenceIndex := 0, 0
	diagnostics := application.NewDiagnosticService(profiles, store, application.WithDiagnosticClock(clock),
		application.WithDiagnosticAttemptIDGenerator(func() (learning.ID, error) {
			attemptIndex++
			return testID(t, fmt.Sprintf("attempt.setup.%d", attemptIndex)), nil
		}),
		application.WithDiagnosticEvidenceIDGenerator(func() (learning.ID, error) {
			evidenceIndex++
			return testID(t, fmt.Sprintf("evidence.setup.%d", evidenceIndex)), nil
		}))
	var setupUnit application.UnitOfWork = store
	if failStateSave {
		setupUnit = failingStateUnitOfWork{base: store}
	}
	service := application.NewLearnerSetupService(profiles, onboarding, instances, diagnostics, setupUnit, curriculum, diagnostic,
		application.WithLearnerSetupClock(clock), application.WithDevelopmentSetupReset(allowReset))
	return learnerSetupFixture{store: store, repositories: repositories, profiles: profiles, goals: goals, onboarding: onboarding,
		instances: instances, diagnostics: diagnostics, service: service, student: student, curriculum: curriculum, diagnostic: diagnostic, clock: clock}
}

func completeSetupOnboarding(t *testing.T, service application.LearnerSetupService, diagnosticAnswer string) application.LearnerSetupView {
	t.Helper()
	view, err := completeSetupOnboardingResult(service, diagnosticAnswer)
	if err != nil {
		t.Fatal(err)
	}
	return view
}

func completeSetupOnboardingResult(service application.LearnerSetupService, diagnosticAnswer string) (application.LearnerSetupView, error) {
	ctx := context.Background()
	if _, err := service.Start(ctx); err != nil {
		return application.LearnerSetupView{}, err
	}
	answers := []string{"Ada", "Ratios", "Mathematics", "Reason proportionally", "beginner", "novice", "30", "5", "theory_first", "0.80", diagnosticAnswer, ""}
	for _, answer := range answers {
		if _, err := service.SubmitOnboarding(ctx, answer); err != nil {
			return application.LearnerSetupView{}, err
		}
	}
	return service.Confirm(ctx)
}

type failingStateUnitOfWork struct{ base application.UnitOfWork }

func (unit failingStateUnitOfWork) WithinTransaction(ctx context.Context, work func(application.Repositories) error) error {
	return unit.base.WithinTransaction(ctx, func(repositories application.Repositories) error {
		repositories.InstanceConceptStates = failingInstanceStateRepository{InstanceConceptStateRepository: repositories.InstanceConceptStates}
		return work(repositories)
	})
}

type failingInstanceStateRepository struct {
	application.InstanceConceptStateRepository
}

func (repository failingInstanceStateRepository) Save(context.Context, learning.InstanceConceptState) error {
	return application.Classify(application.ErrorPersistenceFailure, "save injected setup state", errors.New("injected failure"))
}
