package learning

import "fmt"

// Streak is a materialized learning consistency record. Streak calculation
// policy and versioning are intentionally deferred to the dedicated I-02 step.
type Streak struct {
	StudentID   ID
	CurrentDays int
	LongestDays int
	LastStudyAt *Timestamp
}

func (streak Streak) Validate() error {
	if err := streak.StudentID.Validate(); err != nil {
		return fmt.Errorf("streak student: %w", err)
	}
	if streak.CurrentDays < 0 || streak.LongestDays < 0 {
		return fmt.Errorf("streak days cannot be negative")
	}
	if streak.CurrentDays > streak.LongestDays {
		return fmt.Errorf("current streak cannot exceed longest streak")
	}
	if err := validateOptionalTimestamp("last study at", streak.LastStudyAt); err != nil {
		return err
	}
	if streak.CurrentDays > 0 && streak.LastStudyAt == nil {
		return fmt.Errorf("active streak is missing last study timestamp")
	}
	return nil
}

type AchievementStatus string

const (
	AchievementLocked   AchievementStatus = "locked"
	AchievementUnlocked AchievementStatus = "unlocked"
)

func (status AchievementStatus) Valid() bool {
	return status == AchievementLocked || status == AchievementUnlocked
}

// Achievement represents recognition defined by a stable key. Name is display
// text and may change without losing the student's achievement history.
type Achievement struct {
	ID         ID
	StudentID  ID
	Key        ID
	Name       string
	Status     AchievementStatus
	UnlockedAt *Timestamp
}

func (achievement Achievement) Validate() error {
	if err := achievement.ID.Validate(); err != nil {
		return fmt.Errorf("achievement: %w", err)
	}
	if err := achievement.StudentID.Validate(); err != nil {
		return fmt.Errorf("achievement student: %w", err)
	}
	if err := achievement.Key.Validate(); err != nil {
		return fmt.Errorf("achievement key: %w", err)
	}
	if err := requireText("achievement name", achievement.Name); err != nil {
		return err
	}
	if !achievement.Status.Valid() {
		return fmt.Errorf("achievement status %q is invalid", achievement.Status)
	}
	if err := validateOptionalTimestamp("achievement unlocked at", achievement.UnlockedAt); err != nil {
		return err
	}
	if achievement.Status == AchievementUnlocked && achievement.UnlockedAt == nil {
		return fmt.Errorf("unlocked achievement is missing timestamp")
	}
	if achievement.Status == AchievementLocked && achievement.UnlockedAt != nil {
		return fmt.Errorf("locked achievement cannot have unlock timestamp")
	}
	return nil
}

// Milestone records meaningful progress toward a goal independently from
// gamified achievements.
type Milestone struct {
	ID        ID
	StudentID ID
	GoalID    ID
	Name      string
	ReachedAt Timestamp
}

func (milestone Milestone) Validate() error {
	if err := milestone.ID.Validate(); err != nil {
		return fmt.Errorf("milestone: %w", err)
	}
	if err := milestone.StudentID.Validate(); err != nil {
		return fmt.Errorf("milestone student: %w", err)
	}
	if err := milestone.GoalID.Validate(); err != nil {
		return fmt.Errorf("milestone goal: %w", err)
	}
	if err := requireText("milestone name", milestone.Name); err != nil {
		return err
	}
	if err := milestone.ReachedAt.Validate(); err != nil {
		return fmt.Errorf("milestone reached at: %w", err)
	}
	return nil
}

// AnalyticsSnapshot is an auditable point-in-time summary. Its metrics are
// named and unit-bearing so consumers never need to interpret magic numbers.
type AnalyticsSnapshot struct {
	StudentID          ID
	CapturedAt         Timestamp
	StudyMinutes       int
	SessionsCompleted  int
	ConceptsIntroduced int
	ConceptsMastered   int
	ReviewsDue         int
}

func (snapshot AnalyticsSnapshot) Validate() error {
	if err := snapshot.StudentID.Validate(); err != nil {
		return fmt.Errorf("analytics student: %w", err)
	}
	if err := snapshot.CapturedAt.Validate(); err != nil {
		return fmt.Errorf("analytics captured at: %w", err)
	}
	if snapshot.StudyMinutes < 0 || snapshot.SessionsCompleted < 0 ||
		snapshot.ConceptsIntroduced < 0 || snapshot.ConceptsMastered < 0 || snapshot.ReviewsDue < 0 {
		return fmt.Errorf("analytics metrics cannot be negative")
	}
	if snapshot.ConceptsMastered > snapshot.ConceptsIntroduced {
		return fmt.Errorf("mastered concepts cannot exceed introduced concepts")
	}
	return nil
}

type DailyPlanItemType string

const (
	DailyPlanLearn    DailyPlanItemType = "learn"
	DailyPlanPractice DailyPlanItemType = "practice"
	DailyPlanReview   DailyPlanItemType = "review"
	DailyPlanReflect  DailyPlanItemType = "reflect"
)

func (itemType DailyPlanItemType) Valid() bool {
	switch itemType {
	case DailyPlanLearn, DailyPlanPractice, DailyPlanReview, DailyPlanReflect:
		return true
	default:
		return false
	}
}

type DailyPlanItem struct {
	ID               ID
	Type             DailyPlanItemType
	ConceptIDs       []ID
	EstimatedMinutes int
	Position         int
}

func (item DailyPlanItem) Validate() error {
	if err := item.ID.Validate(); err != nil {
		return fmt.Errorf("daily plan item: %w", err)
	}
	if !item.Type.Valid() {
		return fmt.Errorf("daily plan item type %q is invalid", item.Type)
	}
	if len(item.ConceptIDs) == 0 {
		return fmt.Errorf("daily plan item has no concepts")
	}
	if err := validateIDs("daily plan item concepts", item.ConceptIDs); err != nil {
		return err
	}
	if item.EstimatedMinutes <= 0 {
		return fmt.Errorf("daily plan item duration must be positive")
	}
	if item.Position < 0 {
		return fmt.Errorf("daily plan item position cannot be negative")
	}
	return nil
}

// DailyPlan is a dated, ordered study proposal. Adaptation and selection policy
// are deliberately deferred to the later Daily Plan implementation step.
type DailyPlan struct {
	ID        ID
	StudentID ID
	GoalID    ID
	Date      Timestamp
	CreatedAt Timestamp
	Items     []DailyPlanItem
}

func (plan DailyPlan) Validate() error {
	if err := plan.ID.Validate(); err != nil {
		return fmt.Errorf("daily plan: %w", err)
	}
	if err := plan.StudentID.Validate(); err != nil {
		return fmt.Errorf("daily plan student: %w", err)
	}
	if err := plan.GoalID.Validate(); err != nil {
		return fmt.Errorf("daily plan goal: %w", err)
	}
	if err := plan.Date.Validate(); err != nil {
		return fmt.Errorf("daily plan date: %w", err)
	}
	if err := plan.CreatedAt.Validate(); err != nil {
		return fmt.Errorf("daily plan created at: %w", err)
	}
	seenIDs := make(map[ID]struct{}, len(plan.Items))
	seenPositions := make(map[int]struct{}, len(plan.Items))
	for _, item := range plan.Items {
		if err := item.Validate(); err != nil {
			return err
		}
		if _, exists := seenIDs[item.ID]; exists {
			return fmt.Errorf("daily plan contains duplicate item %q", item.ID)
		}
		if _, exists := seenPositions[item.Position]; exists {
			return fmt.Errorf("daily plan contains duplicate position %d", item.Position)
		}
		seenIDs[item.ID] = struct{}{}
		seenPositions[item.Position] = struct{}{}
	}
	return nil
}
