package learning

import "fmt"

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

// LearningGoal records the student's intended outcome and the mastery policy
// for that outcome. It does not contain a generated curriculum.
type LearningGoal struct {
	ID               ID
	StudentID        ID
	Title            string
	Status           GoalStatus
	MasteryThreshold MasteryThreshold
	CreatedAt        Timestamp
	UpdatedAt        Timestamp
}

func NewLearningGoal(id, studentID ID, title string, threshold MasteryThreshold, createdAt Timestamp) (LearningGoal, error) {
	goal := LearningGoal{
		ID: id, StudentID: studentID, Title: title, Status: GoalDraft,
		MasteryThreshold: threshold, CreatedAt: createdAt, UpdatedAt: createdAt,
	}
	return goal, goal.Validate()
}

func (goal LearningGoal) Validate() error {
	if err := goal.ID.Validate(); err != nil {
		return fmt.Errorf("learning goal: %w", err)
	}
	if err := goal.StudentID.Validate(); err != nil {
		return fmt.Errorf("learning goal student: %w", err)
	}
	if err := requireText("learning goal title", goal.Title); err != nil {
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
	return nil
}
