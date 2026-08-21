package learning

import (
	"fmt"
	"sort"
	"time"
)

const (
	StudyHistoryVersion       = "study-history-v1"
	TimeTrackingPolicyVersion = "time-tracking-v1"
)

type StudyEventType string

const (
	StudyEventOnboardingCompleted StudyEventType = "onboarding.completed"
	StudyEventDiagnosticCompleted StudyEventType = "diagnostic.completed"
	StudyEventConceptIntroduced   StudyEventType = "concept.introduced"
	StudyEventEvidenceRecorded    StudyEventType = "evidence.recorded"
	StudyEventConceptMastered     StudyEventType = "concept.mastered"
	StudyEventReviewCompleted     StudyEventType = "review.completed"
	StudyEventSessionCompleted    StudyEventType = "session.completed"
	StudyEventAchievementUnlocked StudyEventType = "achievement.unlocked"
)

func (eventType StudyEventType) Valid() bool {
	switch eventType {
	case StudyEventOnboardingCompleted, StudyEventDiagnosticCompleted, StudyEventConceptIntroduced,
		StudyEventEvidenceRecorded, StudyEventConceptMastered, StudyEventReviewCompleted,
		StudyEventSessionCompleted, StudyEventAchievementUnlocked:
		return true
	default:
		return false
	}
}

// StudyEvent is an immutable learner-facing fact. SourceID identifies the
// originating aggregate so retries can record the same fact idempotently.
type StudyEvent struct {
	ID                   ID
	StudentID            ID
	Type                 StudyEventType
	SourceID             ID
	OccurredAt           Timestamp
	GoalID               *ID
	CurriculumInstanceID *ID
	ConceptID            *ID
	Version              string
}

func NewStudyEvent(id, studentID ID, eventType StudyEventType, sourceID ID, occurredAt Timestamp, goalID, instanceID, conceptID *ID) (StudyEvent, error) {
	event := StudyEvent{
		ID: id, StudentID: studentID, Type: eventType, SourceID: sourceID, OccurredAt: occurredAt,
		GoalID: cloneIDPointer(goalID), CurriculumInstanceID: cloneIDPointer(instanceID), ConceptID: cloneIDPointer(conceptID),
		Version: StudyHistoryVersion,
	}
	return event, event.Validate()
}

func (event StudyEvent) Validate() error {
	for _, field := range []struct {
		name string
		id   ID
	}{{"study event", event.ID}, {"study event student", event.StudentID}, {"study event source", event.SourceID}} {
		if err := field.id.Validate(); err != nil {
			return fmt.Errorf("%s: %w", field.name, err)
		}
	}
	if !event.Type.Valid() {
		return fmt.Errorf("study event type %q is invalid", event.Type)
	}
	if err := event.OccurredAt.Validate(); err != nil {
		return fmt.Errorf("study event occurred at: %w", err)
	}
	for _, field := range []struct {
		name string
		id   *ID
	}{{"goal", event.GoalID}, {"curriculum instance", event.CurriculumInstanceID}, {"concept", event.ConceptID}} {
		if field.id != nil {
			if err := field.id.Validate(); err != nil {
				return fmt.Errorf("study event %s: %w", field.name, err)
			}
		}
	}
	if event.CurriculumInstanceID != nil && event.GoalID == nil {
		return fmt.Errorf("study event curriculum instance requires a goal")
	}
	switch event.Type {
	case StudyEventDiagnosticCompleted:
		if event.CurriculumInstanceID == nil {
			return fmt.Errorf("study event %q requires a curriculum instance", event.Type)
		}
	case StudyEventConceptIntroduced, StudyEventEvidenceRecorded, StudyEventConceptMastered, StudyEventReviewCompleted:
		if event.ConceptID == nil {
			return fmt.Errorf("study event %q requires a concept", event.Type)
		}
	}
	if event.Version != StudyHistoryVersion {
		return fmt.Errorf("study event version %q is unsupported", event.Version)
	}
	return nil
}

type StudyPeriod string

const (
	StudyPeriodAll   StudyPeriod = "all"
	StudyPeriodToday StudyPeriod = "today"
	StudyPeriodWeek  StudyPeriod = "week"
	StudyPeriodMonth StudyPeriod = "month"
)

func (period StudyPeriod) Valid() bool {
	switch period {
	case StudyPeriodAll, StudyPeriodToday, StudyPeriodWeek, StudyPeriodMonth:
		return true
	default:
		return false
	}
}

// StudyWindow returns an inclusive start and exclusive end in UTC for one
// local calendar period. Weeks start on Monday. Calendar construction in the
// supplied IANA location preserves 23/25-hour DST days.
func StudyWindow(period StudyPeriod, now time.Time, location *time.Location) (Timestamp, Timestamp, error) {
	if period == StudyPeriodAll || !period.Valid() {
		return Timestamp{}, Timestamp{}, fmt.Errorf("study period %q has no bounded window", period)
	}
	if now.IsZero() || location == nil {
		return Timestamp{}, Timestamp{}, fmt.Errorf("study window requires time and location")
	}
	local := now.In(location)
	day := time.Date(local.Year(), local.Month(), local.Day(), 0, 0, 0, 0, location)
	var start, end time.Time
	switch period {
	case StudyPeriodToday:
		start, end = day, day.AddDate(0, 0, 1)
	case StudyPeriodWeek:
		daysSinceMonday := (int(local.Weekday()) + 6) % 7
		start = day.AddDate(0, 0, -daysSinceMonday)
		end = start.AddDate(0, 0, 7)
	case StudyPeriodMonth:
		start = time.Date(local.Year(), local.Month(), 1, 0, 0, 0, 0, location)
		end = start.AddDate(0, 1, 0)
	}
	startUTC, err := NewTimestamp(start)
	if err != nil {
		return Timestamp{}, Timestamp{}, err
	}
	endUTC, err := NewTimestamp(end)
	if err != nil {
		return Timestamp{}, Timestamp{}, err
	}
	return startUTC, endUTC, nil
}

type StudyTimeBreakdown struct {
	ID       ID
	Duration time.Duration
	Sessions int
}

func SortStudyTimeBreakdowns(items []StudyTimeBreakdown) {
	sort.Slice(items, func(i, j int) bool { return items[i].ID.String() < items[j].ID.String() })
}

func cloneIDPointer(value *ID) *ID {
	if value == nil {
		return nil
	}
	copy := *value
	return &copy
}
