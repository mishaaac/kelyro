package learning

import (
	"fmt"
	"strings"
	"unicode/utf8"
)

// OnboardingStatus is the durable lifecycle of the initial learner interview.
type OnboardingStatus string

const (
	OnboardingNotStarted OnboardingStatus = "not_started"
	OnboardingInProgress OnboardingStatus = "in_progress"
	OnboardingCompleted  OnboardingStatus = "completed"
	OnboardingCancelled  OnboardingStatus = "cancelled"
)

func (status OnboardingStatus) Valid() bool {
	switch status {
	case OnboardingNotStarted, OnboardingInProgress, OnboardingCompleted, OnboardingCancelled:
		return true
	default:
		return false
	}
}

// OnboardingSection identifies a stable interview section. Future learning
// packs may add their own sections without changing the common question IDs.
type OnboardingSection string

const (
	OnboardingIdentitySection        OnboardingSection = "identity"
	OnboardingGoalSection            OnboardingSection = "goal"
	OnboardingBackgroundSection      OnboardingSection = "general_background"
	OnboardingPriorExperienceSection OnboardingSection = "prior_experience"
	OnboardingAvailabilitySection    OnboardingSection = "time_availability"
	OnboardingPreferencesSection     OnboardingSection = "study_preferences"
	OnboardingMasterySection         OnboardingSection = "mastery_strictness"
	OnboardingDiagnosticSection      OnboardingSection = "diagnostic_opt_in"
	OnboardingSummarySection         OnboardingSection = "summary"
	OnboardingConfirmationSection    OnboardingSection = "confirm"
)

// OnboardingQuestionKind lets application and presentation adapters share a
// configurable flow without moving validation or navigation into the TUI.
type OnboardingQuestionKind string

const (
	OnboardingTextQuestion    OnboardingQuestionKind = "text"
	OnboardingChoiceQuestion  OnboardingQuestionKind = "choice"
	OnboardingReviewQuestion  OnboardingQuestionKind = "review"
	OnboardingConfirmQuestion OnboardingQuestionKind = "confirm"
)

func (kind OnboardingQuestionKind) Valid() bool {
	switch kind {
	case OnboardingTextQuestion, OnboardingChoiceQuestion, OnboardingReviewQuestion, OnboardingConfirmQuestion:
		return true
	default:
		return false
	}
}

type OnboardingOption struct {
	Value string
	Label string
}

type OnboardingQuestion struct {
	ID       string
	Section  OnboardingSection
	Prompt   string
	Kind     OnboardingQuestionKind
	Required bool
	Options  []OnboardingOption
}

func (question OnboardingQuestion) Validate() error {
	if err := validateOnboardingKey("onboarding question id", question.ID); err != nil {
		return err
	}
	if err := validateOnboardingKey("onboarding section", string(question.Section)); err != nil {
		return err
	}
	if strings.TrimSpace(question.Prompt) == "" || question.Prompt != strings.TrimSpace(question.Prompt) {
		return fmt.Errorf("onboarding question %q has an empty or padded prompt", question.ID)
	}
	if !question.Kind.Valid() {
		return fmt.Errorf("onboarding question %q has invalid kind %q", question.ID, question.Kind)
	}
	if question.Kind == OnboardingChoiceQuestion && len(question.Options) < 2 {
		return fmt.Errorf("choice question %q needs at least two options", question.ID)
	}
	if question.Kind != OnboardingChoiceQuestion && len(question.Options) != 0 {
		return fmt.Errorf("non-choice question %q cannot define options", question.ID)
	}
	seen := make(map[string]struct{}, len(question.Options))
	for _, option := range question.Options {
		if err := validateOnboardingKey("onboarding option value", option.Value); err != nil {
			return fmt.Errorf("question %q: %w", question.ID, err)
		}
		if strings.TrimSpace(option.Label) == "" || option.Label != strings.TrimSpace(option.Label) {
			return fmt.Errorf("question %q has an empty or padded option label", question.ID)
		}
		if _, exists := seen[option.Value]; exists {
			return fmt.Errorf("question %q has duplicate option %q", question.ID, option.Value)
		}
		seen[option.Value] = struct{}{}
	}
	return nil
}

// OnboardingFlow is a deterministic, versioned interview definition. The
// common flow lives in application; future packs can provide another flow.
type OnboardingFlow struct {
	ID        string
	Version   string
	Questions []OnboardingQuestion
}

func (flow OnboardingFlow) Validate() error {
	if err := validateOnboardingKey("onboarding flow id", flow.ID); err != nil {
		return err
	}
	if err := validateOnboardingKey("onboarding flow version", flow.Version); err != nil {
		return err
	}
	if len(flow.Questions) == 0 {
		return fmt.Errorf("onboarding flow has no questions")
	}
	seen := make(map[string]struct{}, len(flow.Questions))
	confirmations := 0
	for index, question := range flow.Questions {
		if err := question.Validate(); err != nil {
			return err
		}
		if _, exists := seen[question.ID]; exists {
			return fmt.Errorf("onboarding flow has duplicate question %q", question.ID)
		}
		seen[question.ID] = struct{}{}
		if question.Kind == OnboardingConfirmQuestion {
			confirmations++
			if index != len(flow.Questions)-1 {
				return fmt.Errorf("onboarding confirmation must be the final question")
			}
		}
	}
	if confirmations != 1 {
		return fmt.Errorf("onboarding flow must have exactly one confirmation question")
	}
	return nil
}

func (flow OnboardingFlow) Question(id string) (OnboardingQuestion, int, bool) {
	for index, question := range flow.Questions {
		if question.ID == id {
			return question, index, true
		}
	}
	return OnboardingQuestion{}, 0, false
}

// OnboardingInterview is the durable learner-owned draft. Answers are keyed
// by stable question IDs so a pack flow can evolve independently of rendering.
type OnboardingInterview struct {
	StudentID         ID
	FlowID            string
	FlowVersion       string
	Status            OnboardingStatus
	CurrentQuestionID string
	Answers           map[string]string
	CreatedAt         Timestamp
	UpdatedAt         Timestamp
	CompletedAt       *Timestamp
	CancelledAt       *Timestamp
}

func NewOnboardingInterview(studentID ID, flow OnboardingFlow, createdAt Timestamp) (OnboardingInterview, error) {
	interview := OnboardingInterview{
		StudentID: studentID, FlowID: flow.ID, FlowVersion: flow.Version,
		Status: OnboardingNotStarted, Answers: map[string]string{}, CreatedAt: createdAt, UpdatedAt: createdAt,
	}
	return interview, interview.Validate(flow)
}

func (interview OnboardingInterview) Start(flow OnboardingFlow, at Timestamp) (OnboardingInterview, error) {
	if interview.Status == OnboardingCompleted {
		return OnboardingInterview{}, fmt.Errorf("completed onboarding cannot be restarted")
	}
	if err := interview.transitionTime(at); err != nil {
		return OnboardingInterview{}, err
	}
	if interview.Status == OnboardingCancelled {
		interview.Answers = map[string]string{}
	}
	interview.Status = OnboardingInProgress
	interview.CurrentQuestionID = flow.Questions[0].ID
	if len(interview.Answers) > 0 {
		interview.CurrentQuestionID = firstUnansweredQuestion(flow, interview.Answers)
	}
	interview.UpdatedAt = at
	interview.CancelledAt = nil
	return interview, interview.Validate(flow)
}

// Submit validates and persists the current answer, then advances. Review
// screens accept an empty submission; confirmation uses Confirm instead.
func (interview OnboardingInterview) Submit(flow OnboardingFlow, value string, at Timestamp) (OnboardingInterview, error) {
	question, index, err := interview.current(flow)
	if err != nil {
		return OnboardingInterview{}, err
	}
	if question.Kind == OnboardingConfirmQuestion {
		return OnboardingInterview{}, fmt.Errorf("confirmation requires Confirm")
	}
	if err := interview.transitionTime(at); err != nil {
		return OnboardingInterview{}, err
	}
	if question.Kind != OnboardingReviewQuestion {
		normalized, validateErr := question.validateAnswer(value)
		if validateErr != nil {
			return OnboardingInterview{}, validateErr
		}
		interview.Answers = cloneOnboardingAnswers(interview.Answers)
		interview.Answers[question.ID] = normalized
	}
	interview.CurrentQuestionID = flow.Questions[index+1].ID
	interview.UpdatedAt = at
	return interview, interview.Validate(flow)
}

func (interview OnboardingInterview) Back(flow OnboardingFlow, at Timestamp) (OnboardingInterview, error) {
	_, index, err := interview.current(flow)
	if err != nil {
		return OnboardingInterview{}, err
	}
	if index == 0 {
		return OnboardingInterview{}, fmt.Errorf("onboarding is already at the first question")
	}
	if err := interview.transitionTime(at); err != nil {
		return OnboardingInterview{}, err
	}
	interview.CurrentQuestionID = flow.Questions[index-1].ID
	interview.UpdatedAt = at
	return interview, interview.Validate(flow)
}

func (interview OnboardingInterview) Cancel(flow OnboardingFlow, at Timestamp) (OnboardingInterview, error) {
	if interview.Status != OnboardingInProgress {
		return OnboardingInterview{}, fmt.Errorf("cannot cancel onboarding from %q", interview.Status)
	}
	if err := interview.transitionTime(at); err != nil {
		return OnboardingInterview{}, err
	}
	interview.Status = OnboardingCancelled
	interview.CurrentQuestionID = ""
	interview.UpdatedAt = at
	interview.CancelledAt = &at
	return interview, interview.Validate(flow)
}

func (interview OnboardingInterview) Confirm(flow OnboardingFlow, at Timestamp) (OnboardingInterview, error) {
	question, _, err := interview.current(flow)
	if err != nil {
		return OnboardingInterview{}, err
	}
	if question.Kind != OnboardingConfirmQuestion {
		return OnboardingInterview{}, fmt.Errorf("onboarding is not ready for confirmation")
	}
	for _, candidate := range flow.Questions {
		if candidate.Kind == OnboardingReviewQuestion || candidate.Kind == OnboardingConfirmQuestion {
			continue
		}
		answer, exists := interview.Answers[candidate.ID]
		if !exists {
			return OnboardingInterview{}, fmt.Errorf("onboarding question %q is unanswered", candidate.ID)
		}
		if _, validateErr := candidate.validateAnswer(answer); validateErr != nil {
			return OnboardingInterview{}, validateErr
		}
	}
	if err := interview.transitionTime(at); err != nil {
		return OnboardingInterview{}, err
	}
	interview.Status = OnboardingCompleted
	interview.CurrentQuestionID = ""
	interview.UpdatedAt = at
	interview.CompletedAt = &at
	return interview, interview.Validate(flow)
}

func (interview OnboardingInterview) Current(flow OnboardingFlow) (OnboardingQuestion, int, error) {
	return interview.current(flow)
}

func (interview OnboardingInterview) Validate(flow OnboardingFlow) error {
	if err := interview.StudentID.Validate(); err != nil {
		return fmt.Errorf("onboarding student: %w", err)
	}
	if err := flow.Validate(); err != nil {
		return err
	}
	if interview.FlowID != flow.ID || interview.FlowVersion != flow.Version {
		return fmt.Errorf("onboarding flow %s@%s does not match %s@%s", interview.FlowID, interview.FlowVersion, flow.ID, flow.Version)
	}
	if !interview.Status.Valid() {
		return fmt.Errorf("onboarding status %q is invalid", interview.Status)
	}
	if err := interview.CreatedAt.Validate(); err != nil {
		return fmt.Errorf("onboarding created at: %w", err)
	}
	if err := interview.UpdatedAt.Validate(); err != nil {
		return fmt.Errorf("onboarding updated at: %w", err)
	}
	if interview.UpdatedAt.Before(interview.CreatedAt) {
		return fmt.Errorf("onboarding update precedes creation")
	}
	if err := validateOptionalTimestamp("onboarding completed at", interview.CompletedAt); err != nil {
		return err
	}
	if err := validateOptionalTimestamp("onboarding cancelled at", interview.CancelledAt); err != nil {
		return err
	}
	if interview.CompletedAt != nil && (interview.CompletedAt.Before(interview.CreatedAt) || interview.CompletedAt.After(interview.UpdatedAt)) {
		return fmt.Errorf("onboarding completion is outside its lifecycle")
	}
	if interview.CancelledAt != nil && (interview.CancelledAt.Before(interview.CreatedAt) || interview.CancelledAt.After(interview.UpdatedAt)) {
		return fmt.Errorf("onboarding cancellation is outside its lifecycle")
	}
	for questionID, answer := range interview.Answers {
		question, _, exists := flow.Question(questionID)
		if !exists || question.Kind == OnboardingReviewQuestion || question.Kind == OnboardingConfirmQuestion {
			return fmt.Errorf("onboarding has answer for unknown or non-answer question %q", questionID)
		}
		if _, err := question.validateAnswer(answer); err != nil {
			return err
		}
	}
	switch interview.Status {
	case OnboardingNotStarted:
		if interview.CurrentQuestionID != "" || interview.CompletedAt != nil || interview.CancelledAt != nil || len(interview.Answers) != 0 {
			return fmt.Errorf("not-started onboarding has progress")
		}
	case OnboardingInProgress:
		if _, _, exists := flow.Question(interview.CurrentQuestionID); !exists {
			return fmt.Errorf("onboarding current question %q is invalid", interview.CurrentQuestionID)
		}
		if interview.CompletedAt != nil || interview.CancelledAt != nil {
			return fmt.Errorf("in-progress onboarding has terminal timestamp")
		}
	case OnboardingCompleted:
		if interview.CurrentQuestionID != "" || interview.CompletedAt == nil || interview.CancelledAt != nil {
			return fmt.Errorf("completed onboarding has inconsistent state")
		}
	case OnboardingCancelled:
		if interview.CurrentQuestionID != "" || interview.CompletedAt != nil || interview.CancelledAt == nil {
			return fmt.Errorf("cancelled onboarding has inconsistent state")
		}
	}
	return nil
}

func (interview OnboardingInterview) current(flow OnboardingFlow) (OnboardingQuestion, int, error) {
	if interview.Status != OnboardingInProgress {
		return OnboardingQuestion{}, 0, fmt.Errorf("onboarding is not in progress")
	}
	question, index, exists := flow.Question(interview.CurrentQuestionID)
	if !exists {
		return OnboardingQuestion{}, 0, fmt.Errorf("onboarding current question %q is invalid", interview.CurrentQuestionID)
	}
	return question, index, nil
}

func (interview OnboardingInterview) transitionTime(at Timestamp) error {
	if err := at.Validate(); err != nil {
		return fmt.Errorf("onboarding transition: %w", err)
	}
	if at.Before(interview.UpdatedAt) {
		return fmt.Errorf("onboarding transition precedes prior update")
	}
	return nil
}

func (question OnboardingQuestion) validateAnswer(value string) (string, error) {
	value = strings.TrimSpace(value)
	if question.Required && value == "" {
		return "", fmt.Errorf("answer for %q is required", question.ID)
	}
	if utf8.RuneCountInString(value) > 300 {
		return "", fmt.Errorf("answer for %q exceeds 300 characters", question.ID)
	}
	if question.Kind == OnboardingChoiceQuestion {
		for _, option := range question.Options {
			if option.Value == value {
				return value, nil
			}
		}
		return "", fmt.Errorf("answer is invalid for onboarding question %q", question.ID)
	}
	return value, nil
}

func firstUnansweredQuestion(flow OnboardingFlow, answers map[string]string) string {
	for _, question := range flow.Questions {
		if question.Kind == OnboardingReviewQuestion || question.Kind == OnboardingConfirmQuestion {
			return question.ID
		}
		if _, exists := answers[question.ID]; !exists {
			return question.ID
		}
	}
	return flow.Questions[len(flow.Questions)-1].ID
}

func cloneOnboardingAnswers(source map[string]string) map[string]string {
	clone := make(map[string]string, len(source)+1)
	for key, value := range source {
		clone[key] = value
	}
	return clone
}

func validateOnboardingKey(name, value string) error {
	if value == "" || value != strings.TrimSpace(value) || strings.ContainsAny(value, " \t\r\n") {
		return fmt.Errorf("%s %q is invalid", name, value)
	}
	return nil
}
