package learning

import "fmt"

type ActivityType string

const (
	ActivityTheory     ActivityType = "theory"
	ActivityPractice   ActivityType = "practice"
	ActivityAssessment ActivityType = "assessment"
	ActivityReview     ActivityType = "review"
	ActivityReflection ActivityType = "reflection"
)

func (activityType ActivityType) Valid() bool {
	switch activityType {
	case ActivityTheory, ActivityPractice, ActivityAssessment, ActivityReview, ActivityReflection:
		return true
	default:
		return false
	}
}

// StudyActivity is one bounded unit of work inside a learning session.
type StudyActivity struct {
	ID         ID
	ConceptIDs []ID
	Type       ActivityType
	StartedAt  Timestamp
	EndedAt    Timestamp
}

func (activity StudyActivity) Validate() error {
	if err := activity.ID.Validate(); err != nil {
		return fmt.Errorf("study activity: %w", err)
	}
	if len(activity.ConceptIDs) == 0 {
		return fmt.Errorf("study activity has no concepts")
	}
	if err := validateIDs("study activity concepts", activity.ConceptIDs); err != nil {
		return err
	}
	if !activity.Type.Valid() {
		return fmt.Errorf("activity type %q is invalid", activity.Type)
	}
	if err := validateTimeRange("study activity", activity.StartedAt, activity.EndedAt); err != nil {
		return err
	}
	return nil
}

// LearningSession is a completed period of study. Open-session lifecycle is an
// application concern; a domain session always has a strict start/end range.
type LearningSession struct {
	ID         ID
	StudentID  ID
	GoalID     ID
	StartedAt  Timestamp
	EndedAt    Timestamp
	Activities []StudyActivity
}

func NewLearningSession(id, studentID, goalID ID, startedAt, endedAt Timestamp, activities []StudyActivity) (LearningSession, error) {
	session := LearningSession{
		ID: id, StudentID: studentID, GoalID: goalID,
		StartedAt: startedAt, EndedAt: endedAt,
		Activities: append([]StudyActivity(nil), activities...),
	}
	return session, session.Validate()
}

func (session LearningSession) Validate() error {
	if err := session.ID.Validate(); err != nil {
		return fmt.Errorf("learning session: %w", err)
	}
	if err := session.StudentID.Validate(); err != nil {
		return fmt.Errorf("learning session student: %w", err)
	}
	if err := session.GoalID.Validate(); err != nil {
		return fmt.Errorf("learning session goal: %w", err)
	}
	if err := validateTimeRange("learning session", session.StartedAt, session.EndedAt); err != nil {
		return err
	}
	seen := make(map[ID]struct{}, len(session.Activities))
	for _, activity := range session.Activities {
		if err := activity.Validate(); err != nil {
			return err
		}
		if _, exists := seen[activity.ID]; exists {
			return fmt.Errorf("learning session contains duplicate activity %q", activity.ID)
		}
		seen[activity.ID] = struct{}{}
		if activity.StartedAt.Before(session.StartedAt) || activity.EndedAt.After(session.EndedAt) {
			return fmt.Errorf("activity %q is outside learning session", activity.ID)
		}
	}
	return nil
}

func validateTimeRange(name string, startedAt, endedAt Timestamp) error {
	if err := startedAt.Validate(); err != nil {
		return fmt.Errorf("%s start: %w", name, err)
	}
	if err := endedAt.Validate(); err != nil {
		return fmt.Errorf("%s end: %w", name, err)
	}
	if !startedAt.Before(endedAt) {
		return fmt.Errorf("%s start must precede end", name)
	}
	return nil
}
