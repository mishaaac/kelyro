package application_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/mishaaac/kelyro/internal/learning"
	"github.com/mishaaac/kelyro/internal/learning/application"
	"github.com/mishaaac/kelyro/internal/learning/application/memory"
)

func TestOnboardingServicePersistsResumeBackCancelAndConfirmation(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	store := memory.New()
	clock := onboardingClock()
	profiles := application.NewProfileService(application.NewStudentService(store.Repositories().Students), application.WithProfileClock(clock))
	goalID, _ := learning.NewID("goal.onboarding")
	goals := application.NewGoalLifecycleService(profiles, store, application.WithGoalClock(clock), application.WithGoalIDGenerator(func() (learning.ID, error) { return goalID, nil }))
	service := application.NewOnboardingService(profiles, goals, store.Repositories().Onboarding, application.WithOnboardingClock(clock))

	started, err := service.Start(ctx)
	if err != nil || started.Interview.Status != learning.OnboardingInProgress || started.Position != 1 {
		t.Fatalf("Start() = (%+v, %v)", started, err)
	}
	answered, err := service.Submit(ctx, "Ada")
	if err != nil || answered.Position != 2 {
		t.Fatalf("Submit() = (%+v, %v)", answered, err)
	}
	resumed, err := service.Show(ctx)
	if err != nil || resumed.Question.ID != application.OnboardingGoalTitleQuestion || resumed.Interview.Answers[application.OnboardingDisplayNameQuestion] != "Ada" {
		t.Fatalf("Show() resume = (%+v, %v)", resumed, err)
	}
	back, err := service.Back(ctx)
	if err != nil || back.Question.ID != application.OnboardingDisplayNameQuestion {
		t.Fatalf("Back() = (%+v, %v)", back, err)
	}
	cancelled, err := service.Cancel(ctx)
	if err != nil || cancelled.Interview.Status != learning.OnboardingCancelled {
		t.Fatalf("Cancel() = (%+v, %v)", cancelled, err)
	}
	student, _ := profiles.Show(ctx)
	listed, _ := goals.Show(ctx)
	if student.Profile.DisplayName != "" || len(listed) != 0 {
		t.Fatalf("cancel changed profile or goals: profile=%+v goals=%+v", student.Profile, listed)
	}

	restarted, err := service.Start(ctx)
	if err != nil || len(restarted.Interview.Answers) != 0 {
		t.Fatalf("restart = (%+v, %v)", restarted, err)
	}
	ready := completeOnboardingAnswers(t, ctx, service, restarted)
	if ready.Question.Kind != learning.OnboardingConfirmQuestion {
		t.Fatalf("ready question = %+v", ready.Question)
	}
	confirmation, err := service.Confirm(ctx)
	if err != nil || confirmation.View.Status != learning.OnboardingCompleted || confirmation.Goal.ID != goalID {
		t.Fatalf("Confirm() = (%+v, %v)", confirmation, err)
	}
	student, _ = profiles.Show(ctx)
	if student.Profile.DisplayName != "Ada" || student.Profile.Experience != learning.ExperienceIntermediate ||
		student.Profile.Availability.DailyMinutes != 30 || student.Profile.Availability.WeeklyDaysTarget != 5 ||
		len(student.Profile.Preferences) != 1 || student.Profile.Preferences[0] != learning.PreferencePractice {
		t.Fatalf("confirmed profile = %+v", student.Profile)
	}
	if confirmation.Goal.Title != "Learn calculus" || confirmation.Goal.StartingLevel != learning.ExperienceBeginner || confirmation.Goal.MasteryThreshold.Value() != .85 {
		t.Fatalf("confirmed goal = %+v", confirmation.Goal)
	}
	if confirmation.View.Answers[application.OnboardingDiagnosticOptInQuestion] != "yes" {
		t.Fatal("diagnostic preference was not retained")
	}
}

func TestOnboardingConfirmationIsIdempotentAfterFinalStateWriteFailure(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	store := memory.New()
	clock := onboardingClock()
	profiles := application.NewProfileService(application.NewStudentService(store.Repositories().Students), application.WithProfileClock(clock))
	next := 0
	goals := application.NewGoalLifecycleService(profiles, store, application.WithGoalClock(clock), application.WithGoalIDGenerator(func() (learning.ID, error) {
		next++
		return learning.NewID("goal.retry." + string(rune('0'+next)))
	}))
	repository := &failCompletedOnboardingRepository{OnboardingRepository: store.Repositories().Onboarding, remaining: 1}
	service := application.NewOnboardingService(profiles, goals, repository, application.WithOnboardingClock(clock))
	ready, err := service.Start(ctx)
	if err != nil {
		t.Fatal(err)
	}
	completeOnboardingAnswers(t, ctx, service, ready)
	if _, err := service.Confirm(ctx); !errors.Is(err, application.ErrPersistenceFailure) {
		t.Fatalf("first Confirm() error = %v, want persistence failure", err)
	}
	listed, _ := goals.Show(ctx)
	if len(listed) != 1 {
		t.Fatalf("goals after interrupted confirmation = %+v", listed)
	}
	confirmed, err := service.Confirm(ctx)
	if err != nil || confirmed.View.Status != learning.OnboardingCompleted {
		t.Fatalf("retry Confirm() = (%+v, %v)", confirmed, err)
	}
	listed, _ = goals.Show(ctx)
	if len(listed) != 1 || next != 1 {
		t.Fatalf("retry duplicated goal: goals=%+v generated=%d", listed, next)
	}
}

func TestOnboardingServiceRejectsInvalidAnswerWithoutAdvancing(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	store := memory.New()
	clock := onboardingClock()
	profiles := application.NewProfileService(application.NewStudentService(store.Repositories().Students), application.WithProfileClock(clock))
	goals := application.NewGoalLifecycleService(profiles, store, application.WithGoalClock(clock))
	service := application.NewOnboardingService(profiles, goals, store.Repositories().Onboarding, application.WithOnboardingClock(clock))
	view, _ := service.Start(ctx)
	view, _ = service.Submit(ctx, "Ada")
	if _, err := service.Submit(ctx, "   "); !errors.Is(err, application.ErrInvalidState) {
		t.Fatalf("invalid Submit() error = %v", err)
	}
	resumed, _ := service.Show(ctx)
	if resumed.Question.ID != view.Question.ID {
		t.Fatalf("invalid answer advanced from %q to %q", view.Question.ID, resumed.Question.ID)
	}
}

func TestDefaultOnboardingFlowContainsEveryRequiredCoreSection(t *testing.T) {
	t.Parallel()
	flow := application.DefaultOnboardingFlow()
	if err := flow.Validate(); err != nil {
		t.Fatal(err)
	}
	sections := make(map[learning.OnboardingSection]bool)
	for _, question := range flow.Questions {
		sections[question.Section] = true
	}
	for _, required := range []learning.OnboardingSection{
		learning.OnboardingIdentitySection, learning.OnboardingGoalSection, learning.OnboardingBackgroundSection,
		learning.OnboardingPriorExperienceSection, learning.OnboardingAvailabilitySection, learning.OnboardingPreferencesSection,
		learning.OnboardingMasterySection, learning.OnboardingDiagnosticSection, learning.OnboardingSummarySection,
		learning.OnboardingConfirmationSection,
	} {
		if !sections[required] {
			t.Errorf("default flow is missing section %q", required)
		}
	}
}

type failCompletedOnboardingRepository struct {
	application.OnboardingRepository
	remaining int
}

func (repository *failCompletedOnboardingRepository) Save(ctx context.Context, interview learning.OnboardingInterview) error {
	if interview.Status == learning.OnboardingCompleted && repository.remaining > 0 {
		repository.remaining--
		return errors.New("simulated crash before final checkpoint")
	}
	return repository.OnboardingRepository.Save(ctx, interview)
}

func completeOnboardingAnswers(t *testing.T, ctx context.Context, service application.OnboardingService, view application.OnboardingView) application.OnboardingView {
	t.Helper()
	answers := map[string]string{
		application.OnboardingDisplayNameQuestion:       "Ada",
		application.OnboardingGoalTitleQuestion:         "Learn calculus",
		application.OnboardingGoalDomainQuestion:        "Mathematics",
		application.OnboardingGoalOutcomeQuestion:       "Solve optimization problems",
		application.OnboardingBackgroundQuestion:        "intermediate",
		application.OnboardingPriorExperienceQuestion:   "beginner",
		application.OnboardingDailyMinutesQuestion:      "30",
		application.OnboardingWeeklyDaysQuestion:        "5",
		application.OnboardingStudyPreferenceQuestion:   "practice",
		application.OnboardingMasteryStrictnessQuestion: "0.85",
		application.OnboardingDiagnosticOptInQuestion:   "yes",
	}
	var err error
	for view.Question.Kind != learning.OnboardingConfirmQuestion {
		view, err = service.Submit(ctx, answers[view.Question.ID])
		if err != nil {
			t.Fatalf("submit %q: %v", view.Question.ID, err)
		}
	}
	return view
}

func onboardingClock() func() time.Time {
	current := time.Date(2026, time.August, 19, 12, 0, 0, 0, time.UTC)
	return func() time.Time {
		value := current
		current = current.Add(time.Minute)
		return value
	}
}
