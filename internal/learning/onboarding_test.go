package learning_test

import (
	"strings"
	"testing"
	"time"

	"github.com/mishaaac/kelyro/internal/learning"
	"github.com/mishaaac/kelyro/internal/learning/application"
)

func TestOnboardingInterviewTransitionsBackCancelAndResume(t *testing.T) {
	t.Parallel()
	flow := application.DefaultOnboardingFlow()
	studentID, _ := learning.NewID("student.onboarding")
	interview, err := learning.NewOnboardingInterview(studentID, flow, onboardingTime(t, 0))
	if err != nil {
		t.Fatal(err)
	}
	if interview.Status != learning.OnboardingNotStarted {
		t.Fatalf("initial status = %q", interview.Status)
	}
	interview, err = interview.Start(flow, onboardingTime(t, 1))
	if err != nil || interview.CurrentQuestionID != application.OnboardingDisplayNameQuestion {
		t.Fatalf("Start() = (%+v, %v)", interview, err)
	}
	interview, err = interview.Submit(flow, "  Ada  ", onboardingTime(t, 2))
	if err != nil || interview.Answers[application.OnboardingDisplayNameQuestion] != "Ada" {
		t.Fatalf("Submit() = (%+v, %v)", interview, err)
	}
	interview, err = interview.Back(flow, onboardingTime(t, 3))
	if err != nil || interview.CurrentQuestionID != application.OnboardingDisplayNameQuestion {
		t.Fatalf("Back() = (%+v, %v)", interview, err)
	}
	interview, err = interview.Cancel(flow, onboardingTime(t, 4))
	if err != nil || interview.Status != learning.OnboardingCancelled || interview.CancelledAt == nil {
		t.Fatalf("Cancel() = (%+v, %v)", interview, err)
	}
	interview, err = interview.Start(flow, onboardingTime(t, 5))
	if err != nil || interview.Status != learning.OnboardingInProgress || len(interview.Answers) != 0 {
		t.Fatalf("restart = (%+v, %v)", interview, err)
	}
}

func TestOnboardingInterviewRejectsInvalidInputAndRequiresFinalConfirmation(t *testing.T) {
	t.Parallel()
	flow := application.DefaultOnboardingFlow()
	studentID, _ := learning.NewID("student.invalid")
	interview, _ := learning.NewOnboardingInterview(studentID, flow, onboardingTime(t, 0))
	interview, _ = interview.Start(flow, onboardingTime(t, 1))
	interview, _ = interview.Submit(flow, "", onboardingTime(t, 2))
	if _, err := interview.Submit(flow, "   ", onboardingTime(t, 3)); err == nil {
		t.Fatal("required goal title accepted whitespace")
	}
	if _, err := interview.Confirm(flow, onboardingTime(t, 3)); err == nil {
		t.Fatal("confirmation succeeded before final question")
	}

	answers := onboardingAnswers()
	for interview.CurrentQuestionID != application.OnboardingConfirmationQuestion {
		question, _, err := interview.Current(flow)
		if err != nil {
			t.Fatal(err)
		}
		answer := answers[question.ID]
		interview, err = interview.Submit(flow, answer, onboardingTime(t, 4+len(interview.Answers)))
		if err != nil {
			t.Fatalf("submit %s: %v", question.ID, err)
		}
	}
	completed, err := interview.Confirm(flow, onboardingTime(t, 30))
	if err != nil || completed.Status != learning.OnboardingCompleted || completed.CompletedAt == nil {
		t.Fatalf("Confirm() = (%+v, %v)", completed, err)
	}
	if _, err := completed.Start(flow, onboardingTime(t, 31)); err == nil {
		t.Fatal("completed interview restarted")
	}
}

func TestOnboardingErrorsDoNotEchoRejectedAnswers(t *testing.T) {
	t.Parallel()
	const privateAnswer = "private-selection-4e2c"
	flow := application.DefaultOnboardingFlow()
	studentID, _ := learning.NewID("student.private-answer")
	interview, _ := learning.NewOnboardingInterview(studentID, flow, onboardingTime(t, 0))
	interview, _ = interview.Start(flow, onboardingTime(t, 1))
	interview, _ = interview.Submit(flow, "", onboardingTime(t, 2))
	interview, _ = interview.Submit(flow, "Goal", onboardingTime(t, 3))
	interview, _ = interview.Submit(flow, "Domain", onboardingTime(t, 4))
	interview, _ = interview.Submit(flow, "Outcome", onboardingTime(t, 5))
	_, err := interview.Submit(flow, privateAnswer, onboardingTime(t, 6))
	if err == nil {
		t.Fatal("invalid onboarding selection was accepted")
	}
	if strings.Contains(err.Error(), privateAnswer) {
		t.Fatalf("onboarding error leaked rejected answer in %q", err)
	}
}

func onboardingAnswers() map[string]string {
	return map[string]string{
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
}

func onboardingTime(t *testing.T, minute int) learning.Timestamp {
	t.Helper()
	timestamp, err := learning.NewTimestamp(time.Date(2026, time.August, 19, 12, minute, 0, 0, time.UTC))
	if err != nil {
		t.Fatal(err)
	}
	return timestamp
}
