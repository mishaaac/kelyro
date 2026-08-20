package application

import (
	"context"
	"errors"
	"fmt"
	"strconv"
	"time"

	"github.com/mishaaac/kelyro/internal/learning"
)

const (
	OnboardingDisplayNameQuestion       = "identity.display_name"
	OnboardingGoalTitleQuestion         = "goal.title"
	OnboardingGoalDomainQuestion        = "goal.domain"
	OnboardingGoalOutcomeQuestion       = "goal.target_outcome"
	OnboardingBackgroundQuestion        = "background.general_experience"
	OnboardingPriorExperienceQuestion   = "experience.subject_experience"
	OnboardingDailyMinutesQuestion      = "availability.daily_minutes"
	OnboardingWeeklyDaysQuestion        = "availability.weekly_days"
	OnboardingStudyPreferenceQuestion   = "preferences.study_mode"
	OnboardingMasteryStrictnessQuestion = "mastery.required_score"
	OnboardingDiagnosticOptInQuestion   = "diagnostic.opt_in"
	OnboardingSummaryQuestion           = "summary.review"
	OnboardingConfirmationQuestion      = "confirm.apply"
)

// DefaultOnboardingFlow is the versioned core interview. Its questions are
// subject-neutral; pack-specific questions can be supplied through an option.
func DefaultOnboardingFlow() learning.OnboardingFlow {
	choice := func(value, label string) learning.OnboardingOption {
		return learning.OnboardingOption{Value: value, Label: label}
	}
	return learning.OnboardingFlow{ID: "core.onboarding", Version: "1", Questions: []learning.OnboardingQuestion{
		{ID: OnboardingDisplayNameQuestion, Section: learning.OnboardingIdentitySection, Prompt: "What should Kelyro call you? (optional)", Kind: learning.OnboardingTextQuestion},
		{ID: OnboardingGoalTitleQuestion, Section: learning.OnboardingGoalSection, Prompt: "What do you want to learn?", Kind: learning.OnboardingTextQuestion, Required: true},
		{ID: OnboardingGoalDomainQuestion, Section: learning.OnboardingGoalSection, Prompt: "What subject or domain does this goal belong to?", Kind: learning.OnboardingTextQuestion, Required: true},
		{ID: OnboardingGoalOutcomeQuestion, Section: learning.OnboardingGoalSection, Prompt: "What outcome would make this goal successful?", Kind: learning.OnboardingTextQuestion, Required: true},
		{ID: OnboardingBackgroundQuestion, Section: learning.OnboardingBackgroundSection, Prompt: "What is your general learning experience?", Kind: learning.OnboardingChoiceQuestion, Required: true, Options: experienceOptions(choice)},
		{ID: OnboardingPriorExperienceQuestion, Section: learning.OnboardingPriorExperienceSection, Prompt: "How much experience do you have with this subject?", Kind: learning.OnboardingChoiceQuestion, Required: true, Options: experienceOptions(choice)},
		{ID: OnboardingDailyMinutesQuestion, Section: learning.OnboardingAvailabilitySection, Prompt: "How much time can you study on a typical study day?", Kind: learning.OnboardingChoiceQuestion, Required: true, Options: []learning.OnboardingOption{choice("15", "15 minutes"), choice("30", "30 minutes"), choice("45", "45 minutes"), choice("60", "60 minutes")}},
		{ID: OnboardingWeeklyDaysQuestion, Section: learning.OnboardingAvailabilitySection, Prompt: "How many days per week can you study?", Kind: learning.OnboardingChoiceQuestion, Required: true, Options: []learning.OnboardingOption{choice("3", "3 days"), choice("5", "5 days"), choice("7", "7 days")}},
		{ID: OnboardingStudyPreferenceQuestion, Section: learning.OnboardingPreferencesSection, Prompt: "How do you prefer to learn?", Kind: learning.OnboardingChoiceQuestion, Required: true, Options: []learning.OnboardingOption{choice(string(learning.PreferenceTheoryFirst), "Understand theory first"), choice(string(learning.PreferencePractice), "Learn through practice"), choice(string(learning.PreferenceProjects), "Build projects"), choice(string(learning.PreferenceReflection), "Reflect and explain")}},
		{ID: OnboardingMasteryStrictnessQuestion, Section: learning.OnboardingMasterySection, Prompt: "How much mastery should be required before moving on?", Kind: learning.OnboardingChoiceQuestion, Required: true, Options: []learning.OnboardingOption{choice("0.70", "Relaxed — 70%"), choice("0.80", "Standard — 80%"), choice("0.85", "Strict — 85%"), choice("0.90", "Mastery — 90%")}},
		{ID: OnboardingDiagnosticOptInQuestion, Section: learning.OnboardingDiagnosticSection, Prompt: "Would you like a diagnostic after setup?", Kind: learning.OnboardingChoiceQuestion, Required: true, Options: []learning.OnboardingOption{choice("yes", "Yes, offer a diagnostic"), choice("no", "No, start from my self-report")}},
		{ID: OnboardingSummaryQuestion, Section: learning.OnboardingSummarySection, Prompt: "Review your setup before applying it.", Kind: learning.OnboardingReviewQuestion},
		{ID: OnboardingConfirmationQuestion, Section: learning.OnboardingConfirmationSection, Prompt: "Apply this learner setup?", Kind: learning.OnboardingConfirmQuestion},
	}}
}

func experienceOptions(choice func(string, string) learning.OnboardingOption) []learning.OnboardingOption {
	return []learning.OnboardingOption{
		choice(string(learning.ExperienceNovice), "None yet"),
		choice(string(learning.ExperienceBeginner), "I have tried introductory material"),
		choice(string(learning.ExperienceIntermediate), "I can apply the fundamentals"),
		choice(string(learning.ExperienceAdvanced), "I use it confidently"),
	}
}

type OnboardingOption func(*onboardingService)

func WithOnboardingClock(now func() time.Time) OnboardingOption {
	return func(service *onboardingService) {
		if now != nil {
			service.now = now
		}
	}
}

func WithOnboardingFlow(flow learning.OnboardingFlow) OnboardingOption {
	return func(service *onboardingService) { service.flow = flow }
}

type onboardingService struct {
	profiles ProfileService
	goals    GoalLifecycleService
	states   OnboardingRepository
	flow     learning.OnboardingFlow
	now      func() time.Time
}

func NewOnboardingService(profiles ProfileService, goals GoalLifecycleService, states OnboardingRepository, options ...OnboardingOption) OnboardingService {
	service := &onboardingService{profiles: profiles, goals: goals, states: states, flow: DefaultOnboardingFlow(), now: time.Now}
	for _, option := range options {
		option(service)
	}
	return service
}

func (service *onboardingService) Show(ctx context.Context) (OnboardingView, error) {
	const operation = "show onboarding"
	student, err := service.student(ctx, operation)
	if err != nil {
		return OnboardingView{}, err
	}
	interview, err := service.states.Get(ctx, student.ID)
	if errors.Is(err, ErrNotFound) {
		timestamp, timestampErr := service.timestamp(operation)
		if timestampErr != nil {
			return OnboardingView{}, timestampErr
		}
		interview, timestampErr = learning.NewOnboardingInterview(student.ID, service.flow, timestamp)
		if timestampErr != nil {
			return OnboardingView{}, invalid(operation, timestampErr)
		}
		return service.view(interview, operation)
	}
	if err != nil {
		return OnboardingView{}, repositoryError(operation, err)
	}
	return service.view(interview, operation)
}

func (service *onboardingService) Start(ctx context.Context) (OnboardingView, error) {
	const operation = "start onboarding"
	view, err := service.Show(ctx)
	if err != nil {
		return OnboardingView{}, err
	}
	if view.Interview.Status == learning.OnboardingInProgress || view.Interview.Status == learning.OnboardingCompleted {
		return view, nil
	}
	timestamp, err := service.timestamp(operation)
	if err != nil {
		return OnboardingView{}, err
	}
	interview, err := view.Interview.Start(service.flow, timestamp)
	if err != nil {
		return OnboardingView{}, invalid(operation, err)
	}
	if err := service.states.Save(ctx, interview); err != nil {
		return OnboardingView{}, repositoryError(operation, err)
	}
	return service.view(interview, operation)
}

func (service *onboardingService) Submit(ctx context.Context, answer string) (OnboardingView, error) {
	return service.transition(ctx, "submit onboarding answer", func(interview learning.OnboardingInterview, at learning.Timestamp) (learning.OnboardingInterview, error) {
		return interview.Submit(service.flow, answer, at)
	})
}

func (service *onboardingService) Back(ctx context.Context) (OnboardingView, error) {
	return service.transition(ctx, "go back in onboarding", func(interview learning.OnboardingInterview, at learning.Timestamp) (learning.OnboardingInterview, error) {
		return interview.Back(service.flow, at)
	})
}

func (service *onboardingService) Cancel(ctx context.Context) (OnboardingView, error) {
	return service.transition(ctx, "cancel onboarding", func(interview learning.OnboardingInterview, at learning.Timestamp) (learning.OnboardingInterview, error) {
		return interview.Cancel(service.flow, at)
	})
}

func (service *onboardingService) Confirm(ctx context.Context) (OnboardingConfirmation, error) {
	const operation = "confirm onboarding"
	view, err := service.Show(ctx)
	if err != nil {
		return OnboardingConfirmation{}, err
	}
	if view.Interview.Status != learning.OnboardingInProgress || view.Question.Kind != learning.OnboardingConfirmQuestion {
		return OnboardingConfirmation{}, invalid(operation, errors.New("onboarding is not ready for confirmation"))
	}
	changes, goalInput, err := onboardingOutputs(view.Interview.Answers)
	if err != nil {
		return OnboardingConfirmation{}, invalid(operation, err)
	}
	if _, err := service.profiles.Edit(ctx, changes); err != nil {
		return OnboardingConfirmation{}, err
	}
	goals, err := service.goals.Show(ctx)
	if err != nil {
		return OnboardingConfirmation{}, err
	}
	goal, found := matchingActiveGoal(goals, goalInput)
	if !found {
		goal, err = service.goals.Set(ctx, goalInput)
		if err != nil {
			return OnboardingConfirmation{}, err
		}
	}
	timestamp, err := service.timestamp(operation)
	if err != nil {
		return OnboardingConfirmation{}, err
	}
	completed, err := view.Interview.Confirm(service.flow, timestamp)
	if err != nil {
		return OnboardingConfirmation{}, invalid(operation, err)
	}
	if err := service.states.Save(ctx, completed); err != nil {
		return OnboardingConfirmation{}, repositoryError(operation, err)
	}
	return OnboardingConfirmation{View: completed, Goal: goal}, nil
}

func (service *onboardingService) transition(ctx context.Context, operation string, change func(learning.OnboardingInterview, learning.Timestamp) (learning.OnboardingInterview, error)) (OnboardingView, error) {
	view, err := service.Show(ctx)
	if err != nil {
		return OnboardingView{}, err
	}
	if view.Interview.Status == learning.OnboardingNotStarted {
		return OnboardingView{}, invalid(operation, errors.New("onboarding has not started"))
	}
	timestamp, err := service.timestamp(operation)
	if err != nil {
		return OnboardingView{}, err
	}
	interview, err := change(view.Interview, timestamp)
	if err != nil {
		return OnboardingView{}, invalid(operation, err)
	}
	if err := service.states.Save(ctx, interview); err != nil {
		return OnboardingView{}, repositoryError(operation, err)
	}
	return service.view(interview, operation)
}

func (service *onboardingService) student(ctx context.Context, operation string) (learning.Student, error) {
	if service == nil || service.profiles == nil || service.goals == nil || service.states == nil {
		return learning.Student{}, Classify(ErrorUnavailable, operation, errors.New("onboarding dependencies are not configured"))
	}
	if err := service.flow.Validate(); err != nil {
		return learning.Student{}, Classify(ErrorUnavailable, operation, fmt.Errorf("invalid onboarding flow: %w", err))
	}
	return service.profiles.Show(ctx)
}

func (service *onboardingService) timestamp(operation string) (learning.Timestamp, error) {
	if service == nil || service.now == nil {
		return learning.Timestamp{}, Classify(ErrorUnavailable, operation, errors.New("onboarding clock is not configured"))
	}
	timestamp, err := learning.NewTimestamp(service.now())
	if err != nil {
		return learning.Timestamp{}, invalid(operation, err)
	}
	return timestamp, nil
}

func (service *onboardingService) view(interview learning.OnboardingInterview, operation string) (OnboardingView, error) {
	if err := interview.Validate(service.flow); err != nil {
		return OnboardingView{}, invalid(operation, err)
	}
	view := OnboardingView{Interview: interview, Total: len(service.flow.Questions)}
	if interview.Status == learning.OnboardingInProgress {
		question, index, err := interview.Current(service.flow)
		if err != nil {
			return OnboardingView{}, invalid(operation, err)
		}
		view.Question, view.Position = question, index+1
	}
	return view, nil
}

func onboardingOutputs(answers map[string]string) (ProfileChanges, SetGoalInput, error) {
	dailyMinutes, err := strconv.Atoi(answers[OnboardingDailyMinutesQuestion])
	if err != nil {
		return ProfileChanges{}, SetGoalInput{}, fmt.Errorf("invalid daily minutes: %w", err)
	}
	weeklyDays, err := strconv.Atoi(answers[OnboardingWeeklyDaysQuestion])
	if err != nil {
		return ProfileChanges{}, SetGoalInput{}, fmt.Errorf("invalid weekly days: %w", err)
	}
	threshold, err := strconv.ParseFloat(answers[OnboardingMasteryStrictnessQuestion], 64)
	if err != nil {
		return ProfileChanges{}, SetGoalInput{}, fmt.Errorf("invalid mastery strictness: %w", err)
	}
	masteryThreshold, err := learning.NewMasteryThreshold(threshold)
	if err != nil {
		return ProfileChanges{}, SetGoalInput{}, err
	}
	displayName := answers[OnboardingDisplayNameQuestion]
	background := learning.ExperienceLevel(answers[OnboardingBackgroundQuestion])
	preference := learning.StudyPreference(answers[OnboardingStudyPreferenceQuestion])
	preferences := []learning.StudyPreference{preference}
	changes := ProfileChanges{DisplayName: &displayName, Experience: &background, DailyMinutes: &dailyMinutes, WeeklyDaysTarget: &weeklyDays, Preferences: &preferences}
	goal := SetGoalInput{
		Title: answers[OnboardingGoalTitleQuestion], Domain: answers[OnboardingGoalDomainQuestion],
		TargetOutcome: answers[OnboardingGoalOutcomeQuestion],
		StartingLevel: learning.ExperienceLevel(answers[OnboardingPriorExperienceQuestion]), MasteryThreshold: masteryThreshold,
	}
	return changes, goal, nil
}

func matchingActiveGoal(goals []learning.LearningGoal, input SetGoalInput) (learning.LearningGoal, bool) {
	for _, goal := range goals {
		if goal.Status == learning.GoalActive && goal.Title == input.Title && goal.Description == input.Description &&
			goal.Domain == input.Domain && goal.TargetOutcome == input.TargetOutcome && goal.StartingLevel == input.StartingLevel &&
			goal.MasteryThreshold.Value() == input.MasteryThreshold.Value() {
			return goal, true
		}
	}
	return learning.LearningGoal{}, false
}
