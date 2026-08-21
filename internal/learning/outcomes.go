package learning

import (
	"fmt"
	"strconv"
	"strings"
	"time"
)

// Streak is a recalculable learning-consistency projection. LastStudyAt is the
// latest qualifying UTC signal; LastActiveLocalDate is its calendar date under
// the captured profile timezone.
type Streak struct {
	StudentID            ID
	CurrentDays          int
	LongestDays          int
	LastActiveLocalDate  *LocalDate
	TotalActiveDays      int
	LastStudyAt          *Timestamp
	Timezone             string
	MinimumActiveMinutes int
	PolicyVersion        string
}

func (streak Streak) Validate() error {
	if err := streak.StudentID.Validate(); err != nil {
		return fmt.Errorf("streak student: %w", err)
	}
	if streak.CurrentDays < 0 || streak.LongestDays < 0 || streak.TotalActiveDays < 0 {
		return fmt.Errorf("streak days cannot be negative")
	}
	if streak.CurrentDays > streak.LongestDays {
		return fmt.Errorf("current streak cannot exceed longest streak")
	}
	if err := validateOptionalTimestamp("last study at", streak.LastStudyAt); err != nil {
		return err
	}
	switch streak.PolicyVersion {
	case LegacyStreakPolicyVersion:
		if streak.LastActiveLocalDate != nil || streak.TotalActiveDays != 0 || streak.Timezone != "" || streak.MinimumActiveMinutes != 0 {
			return fmt.Errorf("legacy streak contains v1 metadata")
		}
		if streak.CurrentDays > 0 && streak.LastStudyAt == nil {
			return fmt.Errorf("active legacy streak is missing last study timestamp")
		}
		return nil
	case StreakPolicyVersion:
	default:
		return fmt.Errorf("unsupported streak policy %q", streak.PolicyVersion)
	}
	if streak.LongestDays > streak.TotalActiveDays {
		return fmt.Errorf("longest streak cannot exceed total active days")
	}
	if streak.MinimumActiveMinutes < 1 || streak.MinimumActiveMinutes > 24*60 {
		return fmt.Errorf("streak minimum active minutes must be within 1..1440")
	}
	location, err := time.LoadLocation(streak.Timezone)
	if err != nil {
		return fmt.Errorf("streak timezone: %w", err)
	}
	if streak.TotalActiveDays == 0 {
		if streak.CurrentDays != 0 || streak.LongestDays != 0 || streak.LastActiveLocalDate != nil || streak.LastStudyAt != nil {
			return fmt.Errorf("empty streak contains active-day metadata")
		}
		return nil
	}
	if streak.LongestDays == 0 || streak.LastActiveLocalDate == nil || streak.LastStudyAt == nil {
		return fmt.Errorf("non-empty streak is missing active-day metadata")
	}
	if err := streak.LastActiveLocalDate.Validate(); err != nil {
		return err
	}
	if LocalDateFromTime(streak.LastStudyAt.Time(), location) != *streak.LastActiveLocalDate {
		return fmt.Errorf("streak last study instant and local date disagree")
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

const (
	AchievementDefinitionVersion = "achievement-definition/v1"
	AchievementPolicyVersion     = "achievement-v1"
	LegacyAchievementVersion     = "legacy-achievement/v0"
)

type AchievementCriteriaType string

const (
	AchievementFirstSession         AchievementCriteriaType = "first_session"
	AchievementFirstConceptMastered AchievementCriteriaType = "first_concept_mastered"
	AchievementActiveDays           AchievementCriteriaType = "active_days"
	AchievementStudyMinutes         AchievementCriteriaType = "study_minutes"
	AchievementFirstReviewCompleted AchievementCriteriaType = "first_review_completed"
	AchievementModuleMastered       AchievementCriteriaType = "module_mastered"
)

func (criteriaType AchievementCriteriaType) Valid() bool {
	switch criteriaType {
	case AchievementFirstSession, AchievementFirstConceptMastered, AchievementActiveDays,
		AchievementStudyMinutes, AchievementFirstReviewCompleted, AchievementModuleMastered:
		return true
	default:
		return false
	}
}

// AchievementCriteriaConfig contains only the numeric thresholds used by the
// Foundation catalog. Empty config is valid for first/any criteria.
type AchievementCriteriaConfig struct {
	Count   int `json:"count,omitempty"`
	Minutes int `json:"minutes,omitempty"`
}

func (config AchievementCriteriaConfig) Validate(criteriaType AchievementCriteriaType) error {
	switch criteriaType {
	case AchievementActiveDays:
		if config.Count < 1 || config.Minutes != 0 {
			return fmt.Errorf("active-days achievement requires a positive count only")
		}
	case AchievementStudyMinutes:
		if config.Minutes < 1 || config.Count != 0 {
			return fmt.Errorf("study-minutes achievement requires positive minutes only")
		}
	case AchievementFirstSession, AchievementFirstConceptMastered,
		AchievementFirstReviewCompleted, AchievementModuleMastered:
		if config.Count != 0 || config.Minutes != 0 {
			return fmt.Errorf("achievement criterion %q does not accept configuration", criteriaType)
		}
	default:
		return fmt.Errorf("achievement criterion %q is invalid", criteriaType)
	}
	return nil
}

// AchievementDefinition is immutable, versioned data. Criteria are evaluated
// by the policy named on the resulting student achievement.
type AchievementDefinition struct {
	ID          ID
	Title       string
	Description string
	Criteria    AchievementCriteriaType
	Config      AchievementCriteriaConfig
	Hidden      bool
	Version     string
}

func (definition AchievementDefinition) Validate() error {
	if err := definition.ID.Validate(); err != nil {
		return fmt.Errorf("achievement definition: %w", err)
	}
	if err := requireText("achievement title", definition.Title); err != nil {
		return err
	}
	if err := requireText("achievement description", definition.Description); err != nil {
		return err
	}
	if !definition.Criteria.Valid() {
		return fmt.Errorf("achievement criterion %q is invalid", definition.Criteria)
	}
	if err := definition.Config.Validate(definition.Criteria); err != nil {
		return err
	}
	if definition.Version != AchievementDefinitionVersion {
		return fmt.Errorf("achievement definition version %q is unsupported", definition.Version)
	}
	return nil
}

// FoundationAchievementDefinitions returns a defensive copy of the
// deterministic initial catalog. Titles deliberately avoid game language.
func FoundationAchievementDefinitions() []AchievementDefinition {
	definitions := []AchievementDefinition{
		{ID: mustAchievementDefinitionID("first_session"), Title: "First study session", Description: "Completed a first meaningful study session.", Criteria: AchievementFirstSession, Version: AchievementDefinitionVersion},
		{ID: mustAchievementDefinitionID("first_concept_mastered"), Title: "First concept mastered", Description: "Reached the mastery requirement for a first concept.", Criteria: AchievementFirstConceptMastered, Version: AchievementDefinitionVersion},
		{ID: mustAchievementDefinitionID("seven_active_days"), Title: "7 active study days", Description: "Studied meaningfully on seven distinct local calendar days.", Criteria: AchievementActiveDays, Config: AchievementCriteriaConfig{Count: 7}, Version: AchievementDefinitionVersion},
		{ID: mustAchievementDefinitionID("ten_hours_studied"), Title: "10 hours studied", Description: "Accumulated ten hours of active study time.", Criteria: AchievementStudyMinutes, Config: AchievementCriteriaConfig{Minutes: 600}, Version: AchievementDefinitionVersion},
		{ID: mustAchievementDefinitionID("first_review_completed"), Title: "First review completed", Description: "Completed a first scheduled review.", Criteria: AchievementFirstReviewCompleted, Version: AchievementDefinitionVersion},
		{ID: mustAchievementDefinitionID("module_mastered"), Title: "First module mastered", Description: "Mastered every concept in a curriculum module.", Criteria: AchievementModuleMastered, Version: AchievementDefinitionVersion},
	}
	return append([]AchievementDefinition(nil), definitions...)
}

func mustAchievementDefinitionID(value string) ID {
	id, err := NewID(value)
	if err != nil {
		panic(err)
	}
	return id
}

// Achievement represents recognition defined by a stable key. Name is display
// text and may change without losing the student's achievement history.
type Achievement struct {
	ID                ID
	StudentID         ID
	Key               ID
	Name              string
	Description       string
	Criteria          AchievementCriteriaType
	Config            AchievementCriteriaConfig
	Hidden            bool
	DefinitionVersion string
	Status            AchievementStatus
	UnlockedAt        *Timestamp
	Context           map[string]string
	PolicyVersion     string
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
	if achievement.DefinitionVersion == "" && achievement.PolicyVersion == "" {
		if achievement.Description != "" || achievement.Criteria != "" || achievement.Config != (AchievementCriteriaConfig{}) ||
			achievement.Hidden || len(achievement.Context) != 0 {
			return fmt.Errorf("legacy achievement contains versioned metadata")
		}
		return nil
	}
	definition := AchievementDefinition{
		ID: achievement.Key, Title: achievement.Name, Description: achievement.Description,
		Criteria: achievement.Criteria, Config: achievement.Config, Hidden: achievement.Hidden,
		Version: achievement.DefinitionVersion,
	}
	if err := definition.Validate(); err != nil {
		return err
	}
	if achievement.PolicyVersion != AchievementPolicyVersion {
		return fmt.Errorf("achievement policy version %q is unsupported", achievement.PolicyVersion)
	}
	if achievement.Status != AchievementUnlocked {
		return fmt.Errorf("versioned student achievement must be unlocked")
	}
	if err := validateAchievementContext(achievement.Context); err != nil {
		return err
	}
	return nil
}

func validateAchievementContext(context map[string]string) error {
	for key, value := range context {
		if strings.TrimSpace(key) == "" || key != strings.TrimSpace(key) || strings.IndexFunc(key, func(r rune) bool { return r == ' ' || r == '\t' || r == '\n' }) >= 0 {
			return fmt.Errorf("achievement context key %q is invalid", key)
		}
		if strings.TrimSpace(value) == "" || value != strings.TrimSpace(value) {
			return fmt.Errorf("achievement context %q is empty or padded", key)
		}
	}
	return nil
}

func achievementNumber(value int) string { return strconv.Itoa(value) }

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

// AnalyticsSnapshot is the legacy stored summary contract introduced with the
// persistence boundary. Learning Analytics v1 calculates the richer
// LearningAnalyticsSnapshot directly from primary facts instead.
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
