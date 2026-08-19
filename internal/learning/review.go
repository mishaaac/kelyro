package learning

import "fmt"

// RetentionState records current recall health without embedding a scheduling
// formula. The policy that calculates it will be introduced in a later step.
type RetentionState struct {
	StudentID  ID
	ConceptID  ID
	Strength   MasteryScore
	MeasuredAt Timestamp
}

func (state RetentionState) Validate() error {
	if err := state.StudentID.Validate(); err != nil {
		return fmt.Errorf("retention state student: %w", err)
	}
	if err := state.ConceptID.Validate(); err != nil {
		return fmt.Errorf("retention state concept: %w", err)
	}
	if err := state.Strength.Validate(); err != nil {
		return fmt.Errorf("retention state: %w", err)
	}
	if err := state.MeasuredAt.Validate(); err != nil {
		return fmt.Errorf("retention measured at: %w", err)
	}
	return nil
}

// ReviewSchedule states when a concept should next be reviewed. Imported
// schedules may predate Kelyro's own concept introduction record.
type ReviewSchedule struct {
	StudentID    ID
	ConceptID    ID
	IntroducedAt *Timestamp
	DueAt        Timestamp
	Imported     bool
}

func NewReviewSchedule(studentID, conceptID ID, introducedAt *Timestamp, dueAt Timestamp, imported bool) (ReviewSchedule, error) {
	schedule := ReviewSchedule{
		StudentID: studentID, ConceptID: conceptID, IntroducedAt: introducedAt,
		DueAt: dueAt, Imported: imported,
	}
	return schedule, schedule.Validate()
}

func (schedule ReviewSchedule) Validate() error {
	if err := schedule.StudentID.Validate(); err != nil {
		return fmt.Errorf("review schedule student: %w", err)
	}
	if err := schedule.ConceptID.Validate(); err != nil {
		return fmt.Errorf("review schedule concept: %w", err)
	}
	if err := validateOptionalTimestamp("review concept introduced at", schedule.IntroducedAt); err != nil {
		return err
	}
	if err := schedule.DueAt.Validate(); err != nil {
		return fmt.Errorf("review due at: %w", err)
	}
	if schedule.IntroducedAt == nil && !schedule.Imported {
		return fmt.Errorf("review requires concept introduction unless imported")
	}
	if schedule.IntroducedAt != nil && schedule.DueAt.Before(*schedule.IntroducedAt) && !schedule.Imported {
		return fmt.Errorf("review cannot be due before concept introduction unless imported")
	}
	return nil
}

type ReviewStatus string

const (
	ReviewPending   ReviewStatus = "pending"
	ReviewCompleted ReviewStatus = "completed"
	ReviewSkipped   ReviewStatus = "skipped"
)

func (status ReviewStatus) Valid() bool {
	switch status {
	case ReviewPending, ReviewCompleted, ReviewSkipped:
		return true
	default:
		return false
	}
}

type ReviewItem struct {
	ID          ID
	StudentID   ID
	ConceptID   ID
	DueAt       Timestamp
	Status      ReviewStatus
	CompletedAt *Timestamp
}

func (item ReviewItem) Validate() error {
	if err := item.ID.Validate(); err != nil {
		return fmt.Errorf("review item: %w", err)
	}
	if err := item.StudentID.Validate(); err != nil {
		return fmt.Errorf("review item student: %w", err)
	}
	if err := item.ConceptID.Validate(); err != nil {
		return fmt.Errorf("review item concept: %w", err)
	}
	if err := item.DueAt.Validate(); err != nil {
		return fmt.Errorf("review item due at: %w", err)
	}
	if !item.Status.Valid() {
		return fmt.Errorf("review status %q is invalid", item.Status)
	}
	if err := validateOptionalTimestamp("review completed at", item.CompletedAt); err != nil {
		return err
	}
	if item.Status == ReviewCompleted && item.CompletedAt == nil {
		return fmt.Errorf("completed review is missing completion timestamp")
	}
	if item.Status != ReviewCompleted && item.CompletedAt != nil {
		return fmt.Errorf("non-completed review cannot have completion timestamp")
	}
	return nil
}
