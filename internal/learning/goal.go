package learning

import (
	"fmt"
	"strings"
)

type GoalStatus string

const (
	GoalDraft     GoalStatus = "draft"
	GoalActive    GoalStatus = "active"
	GoalPaused    GoalStatus = "paused"
	GoalCompleted GoalStatus = "completed"
	GoalArchived  GoalStatus = "archived"
)

func (status GoalStatus) Valid() bool {
	switch status {
	case GoalDraft, GoalActive, GoalPaused, GoalCompleted, GoalArchived:
		return true
	default:
		return false
	}
}

// GoalDetails records learner-provided intent without constraining Domain to
// a closed taxonomy. StartingLevel is explicitly goal-specific and is never
// inferred from the learner's general profile.
type GoalDetails struct {
	Title         string
	Description   string
	Domain        string
	TargetOutcome string
	StartingLevel ExperienceLevel
}

func (details GoalDetails) Validate() error {
	if err := requireText("learning goal title", details.Title); err != nil {
		return err
	}
	if details.Title != strings.TrimSpace(details.Title) {
		return fmt.Errorf("learning goal title is padded")
	}
	if details.Description != strings.TrimSpace(details.Description) {
		return fmt.Errorf("learning goal description is padded")
	}
	if err := requireText("learning goal domain", details.Domain); err != nil {
		return err
	}
	if details.Domain != strings.TrimSpace(details.Domain) {
		return fmt.Errorf("learning goal domain is padded")
	}
	if err := requireText("learning goal target outcome", details.TargetOutcome); err != nil {
		return err
	}
	if details.TargetOutcome != strings.TrimSpace(details.TargetOutcome) {
		return fmt.Errorf("learning goal target outcome is padded")
	}
	if !details.StartingLevel.Valid() {
		return fmt.Errorf("learning goal starting level %q is invalid", details.StartingLevel)
	}
	return nil
}

// LearningGoal records the student's intended outcome and the mastery policy
// for that outcome. It does not contain a generated curriculum.
type LearningGoal struct {
	ID               ID
	StudentID        ID
	Title            string
	Description      string
	Domain           string
	TargetOutcome    string
	StartingLevel    ExperienceLevel
	Status           GoalStatus
	MasteryThreshold MasteryThreshold
	CreatedAt        Timestamp
	UpdatedAt        Timestamp
	ActivatedAt      *Timestamp
	CompletedAt      *Timestamp
}

func NewLearningGoal(id, studentID ID, details GoalDetails, threshold MasteryThreshold, createdAt Timestamp) (LearningGoal, error) {
	goal := LearningGoal{
		ID: id, StudentID: studentID,
		Title: details.Title, Description: details.Description, Domain: details.Domain,
		TargetOutcome: details.TargetOutcome, StartingLevel: details.StartingLevel,
		Status: GoalDraft, MasteryThreshold: threshold, CreatedAt: createdAt, UpdatedAt: createdAt,
	}
	return goal, goal.Validate()
}

func (goal LearningGoal) Details() GoalDetails {
	return GoalDetails{
		Title: goal.Title, Description: goal.Description, Domain: goal.Domain,
		TargetOutcome: goal.TargetOutcome, StartingLevel: goal.StartingLevel,
	}
}

func (goal LearningGoal) Activate(at Timestamp) (LearningGoal, error) {
	if goal.Status != GoalDraft && goal.Status != GoalPaused {
		return LearningGoal{}, fmt.Errorf("cannot activate learning goal from %q", goal.Status)
	}
	if err := goal.validateTransitionTime(at); err != nil {
		return LearningGoal{}, err
	}
	goal.Status = GoalActive
	goal.UpdatedAt = at
	if goal.ActivatedAt == nil {
		activatedAt := at
		goal.ActivatedAt = &activatedAt
	}
	return goal, goal.Validate()
}

func (goal LearningGoal) Pause(at Timestamp) (LearningGoal, error) {
	if goal.Status != GoalActive {
		return LearningGoal{}, fmt.Errorf("cannot pause learning goal from %q", goal.Status)
	}
	if err := goal.validateTransitionTime(at); err != nil {
		return LearningGoal{}, err
	}
	goal.Status = GoalPaused
	goal.UpdatedAt = at
	return goal, goal.Validate()
}

func (goal LearningGoal) Complete(at Timestamp) (LearningGoal, error) {
	if goal.Status != GoalActive && goal.Status != GoalPaused {
		return LearningGoal{}, fmt.Errorf("cannot complete learning goal from %q", goal.Status)
	}
	if err := goal.validateTransitionTime(at); err != nil {
		return LearningGoal{}, err
	}
	completedAt := at
	goal.Status = GoalCompleted
	goal.UpdatedAt = at
	goal.CompletedAt = &completedAt
	return goal, goal.Validate()
}

func (goal LearningGoal) Archive(at Timestamp) (LearningGoal, error) {
	if goal.Status == GoalArchived {
		return LearningGoal{}, fmt.Errorf("cannot archive learning goal from %q", goal.Status)
	}
	if err := goal.validateTransitionTime(at); err != nil {
		return LearningGoal{}, err
	}
	goal.Status = GoalArchived
	goal.UpdatedAt = at
	return goal, goal.Validate()
}

func (goal LearningGoal) validateTransitionTime(at Timestamp) error {
	if err := at.Validate(); err != nil {
		return fmt.Errorf("learning goal transition: %w", err)
	}
	if at.Before(goal.UpdatedAt) {
		return fmt.Errorf("learning goal transition precedes prior update")
	}
	return nil
}

func (goal LearningGoal) Validate() error {
	if err := goal.ID.Validate(); err != nil {
		return fmt.Errorf("learning goal: %w", err)
	}
	if err := goal.StudentID.Validate(); err != nil {
		return fmt.Errorf("learning goal student: %w", err)
	}
	if err := goal.Details().Validate(); err != nil {
		return err
	}
	if !goal.Status.Valid() {
		return fmt.Errorf("learning goal status %q is invalid", goal.Status)
	}
	if err := goal.MasteryThreshold.Validate(); err != nil {
		return err
	}
	if err := goal.CreatedAt.Validate(); err != nil {
		return fmt.Errorf("learning goal created at: %w", err)
	}
	if err := goal.UpdatedAt.Validate(); err != nil {
		return fmt.Errorf("learning goal updated at: %w", err)
	}
	if goal.UpdatedAt.Before(goal.CreatedAt) {
		return fmt.Errorf("learning goal update precedes creation")
	}
	if err := validateOptionalTimestamp("learning goal activated at", goal.ActivatedAt); err != nil {
		return err
	}
	if err := validateOptionalTimestamp("learning goal completed at", goal.CompletedAt); err != nil {
		return err
	}
	if goal.ActivatedAt != nil && goal.ActivatedAt.Before(goal.CreatedAt) {
		return fmt.Errorf("learning goal activation precedes creation")
	}
	if goal.ActivatedAt != nil && goal.ActivatedAt.After(goal.UpdatedAt) {
		return fmt.Errorf("learning goal activation follows latest update")
	}
	if goal.CompletedAt != nil && (goal.ActivatedAt == nil || goal.CompletedAt.Before(*goal.ActivatedAt)) {
		return fmt.Errorf("learning goal completion precedes activation")
	}
	if goal.CompletedAt != nil && goal.CompletedAt.After(goal.UpdatedAt) {
		return fmt.Errorf("learning goal completion follows latest update")
	}
	switch goal.Status {
	case GoalDraft:
		if goal.ActivatedAt != nil || goal.CompletedAt != nil {
			return fmt.Errorf("draft learning goal has lifecycle timestamps")
		}
	case GoalActive, GoalPaused:
		if goal.ActivatedAt == nil || goal.CompletedAt != nil {
			return fmt.Errorf("%s learning goal has inconsistent lifecycle timestamps", goal.Status)
		}
	case GoalCompleted:
		if goal.ActivatedAt == nil || goal.CompletedAt == nil {
			return fmt.Errorf("completed learning goal is missing lifecycle timestamps")
		}
	}
	return nil
}
