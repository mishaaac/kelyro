package learning

import (
	"encoding/hex"
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

type DailyPlanItemRole string

const (
	DailyPlanRoleWarmUp        DailyPlanItemRole = "warm_up"
	DailyPlanRoleReview        DailyPlanItemRole = "review"
	DailyPlanRoleNewLearning   DailyPlanItemRole = "new_learning"
	DailyPlanRoleReinforcement DailyPlanItemRole = "reinforcement"
)

func (role DailyPlanItemRole) Valid() bool {
	switch role {
	case DailyPlanRoleWarmUp, DailyPlanRoleReview, DailyPlanRoleNewLearning, DailyPlanRoleReinforcement:
		return true
	default:
		return false
	}
}

type DailyPlanSelectionReason string

const (
	DailyPlanCriticalOverduePrerequisite DailyPlanSelectionReason = "critical_overdue_prerequisite"
	DailyPlanImportantDueReview          DailyPlanSelectionReason = "important_due_review"
	DailyPlanBlockingWeakness            DailyPlanSelectionReason = "blocking_weakness"
	DailyPlanNextEligibleConcept         DailyPlanSelectionReason = "next_eligible_concept"
	DailyPlanOptionalExtraPractice       DailyPlanSelectionReason = "optional_extra_practice"
)

func (reason DailyPlanSelectionReason) Valid() bool {
	switch reason {
	case DailyPlanCriticalOverduePrerequisite, DailyPlanImportantDueReview, DailyPlanBlockingWeakness,
		DailyPlanNextEligibleConcept, DailyPlanOptionalExtraPractice:
		return true
	default:
		return false
	}
}

type DailyPlanItem struct {
	ID               ID
	Type             DailyPlanItemType
	Role             DailyPlanItemRole
	Reason           DailyPlanSelectionReason
	Explanation      string
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

type DailyPlanStatus string

const (
	DailyPlanReady         DailyPlanStatus = "ready"
	DailyPlanReviewOnly    DailyPlanStatus = "review_only"
	DailyPlanNothingUrgent DailyPlanStatus = "nothing_urgent"
	DailyPlanTimeLimited   DailyPlanStatus = "time_limited"
)

func (status DailyPlanStatus) Valid() bool {
	switch status {
	case DailyPlanReady, DailyPlanReviewOnly, DailyPlanNothingUrgent, DailyPlanTimeLimited:
		return true
	default:
		return false
	}
}

type DailyPlanGenerationReason string

const (
	DailyPlanGeneratedInitial       DailyPlanGenerationReason = "initial"
	DailyPlanGeneratedSourceChanged DailyPlanGenerationReason = "source_changed"
	DailyPlanGeneratedPolicyChanged DailyPlanGenerationReason = "policy_changed"
)

func (reason DailyPlanGenerationReason) Valid() bool {
	switch reason {
	case DailyPlanGeneratedInitial, DailyPlanGeneratedSourceChanged, DailyPlanGeneratedPolicyChanged:
		return true
	default:
		return false
	}
}

// DailyPlan is a dated, ordered study proposal. V1 plans are explainable
// snapshots; empty policy metadata remains the published legacy contract.
type DailyPlan struct {
	ID                   ID
	StudentID            ID
	GoalID               ID
	CurriculumInstanceID ID
	Date                 Timestamp
	CreatedAt            Timestamp
	Timezone             string
	AvailableMinutes     int
	PlannedMinutes       int
	BufferMinutes        int
	Status               DailyPlanStatus
	GenerationReason     DailyPlanGenerationReason
	SourceFingerprint    string
	PolicyVersion        string
	Items                []DailyPlanItem
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
	v1 := plan.PolicyVersion == DailyPlanPolicyVersion
	if plan.PolicyVersion != "" && plan.PolicyVersion != LegacyDailyPlanPolicyVersion && !v1 {
		return fmt.Errorf("daily plan policy %q is unsupported", plan.PolicyVersion)
	}
	if v1 {
		if err := plan.validateV1Metadata(); err != nil {
			return err
		}
	} else if plan.CurriculumInstanceID != (ID{}) || plan.Timezone != "" || plan.AvailableMinutes != 0 ||
		plan.PlannedMinutes != 0 || plan.BufferMinutes != 0 || plan.Status != "" || plan.GenerationReason != "" ||
		plan.SourceFingerprint != "" {
		return fmt.Errorf("legacy daily plan contains v1 metadata")
	}
	seenIDs := make(map[ID]struct{}, len(plan.Items))
	seenPositions := make(map[int]struct{}, len(plan.Items))
	seenConcepts := make(map[ID]struct{}, len(plan.Items))
	plannedMinutes := 0
	hasNewLearning := false
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
		if v1 {
			if err := validateDailyPlanItemV1(item); err != nil {
				return err
			}
			if _, exists := seenConcepts[item.ConceptIDs[0]]; exists {
				return fmt.Errorf("daily plan v1 selects concept %q more than once", item.ConceptIDs[0])
			}
			seenConcepts[item.ConceptIDs[0]] = struct{}{}
			plannedMinutes += item.EstimatedMinutes
			hasNewLearning = hasNewLearning || item.Role == DailyPlanRoleNewLearning
		} else if item.Role != "" || item.Reason != "" || item.Explanation != "" {
			return fmt.Errorf("legacy daily plan item contains v1 metadata")
		}
	}
	if v1 {
		for position := range plan.Items {
			if _, exists := seenPositions[position]; !exists {
				return fmt.Errorf("daily plan v1 item positions must be contiguous")
			}
		}
		if plannedMinutes != plan.PlannedMinutes {
			return fmt.Errorf("daily plan v1 planned minutes do not match items")
		}
		switch plan.Status {
		case DailyPlanReady:
			if !hasNewLearning {
				return fmt.Errorf("ready daily plan has no new learning item")
			}
		case DailyPlanReviewOnly:
			if len(plan.Items) == 0 || hasNewLearning {
				return fmt.Errorf("review-only daily plan item mix is invalid")
			}
		case DailyPlanNothingUrgent, DailyPlanTimeLimited:
			if len(plan.Items) != 0 {
				return fmt.Errorf("empty daily plan status %q contains items", plan.Status)
			}
		}
	}
	return nil
}

func (plan DailyPlan) validateV1Metadata() error {
	if err := plan.CurriculumInstanceID.Validate(); err != nil {
		return fmt.Errorf("daily plan curriculum instance: %w", err)
	}
	location, err := time.LoadLocation(plan.Timezone)
	if err != nil {
		return fmt.Errorf("daily plan timezone: %w", err)
	}
	localDate := plan.Date.Time().In(location)
	if localDate.Hour() != 0 || localDate.Minute() != 0 || localDate.Second() != 0 || localDate.Nanosecond() != 0 {
		return fmt.Errorf("daily plan date is not local midnight")
	}
	if LocalDateFromTime(plan.Date.Time(), location) != LocalDateFromTime(plan.CreatedAt.Time(), location) {
		return fmt.Errorf("daily plan creation is outside its local date")
	}
	if plan.AvailableMinutes < 0 || plan.AvailableMinutes > 24*60 || plan.PlannedMinutes < 0 || plan.BufferMinutes < 0 ||
		plan.PlannedMinutes+plan.BufferMinutes > plan.AvailableMinutes {
		return fmt.Errorf("daily plan v1 time budget is inconsistent")
	}
	if !plan.Status.Valid() {
		return fmt.Errorf("daily plan status %q is invalid", plan.Status)
	}
	if !plan.GenerationReason.Valid() {
		return fmt.Errorf("daily plan generation reason %q is invalid", plan.GenerationReason)
	}
	if len(plan.SourceFingerprint) != 71 || !strings.HasPrefix(plan.SourceFingerprint, "sha256:") {
		return fmt.Errorf("daily plan source fingerprint is invalid")
	}
	if _, err := hex.DecodeString(strings.TrimPrefix(plan.SourceFingerprint, "sha256:")); err != nil {
		return fmt.Errorf("daily plan source fingerprint is invalid")
	}
	return nil
}

func validateDailyPlanItemV1(item DailyPlanItem) error {
	if len(item.ConceptIDs) != 1 {
		return fmt.Errorf("daily plan v1 item must select exactly one concept")
	}
	if !item.Role.Valid() {
		return fmt.Errorf("daily plan item role %q is invalid", item.Role)
	}
	if !item.Reason.Valid() {
		return fmt.Errorf("daily plan item reason %q is invalid", item.Reason)
	}
	if err := requireText("daily plan item explanation", item.Explanation); err != nil {
		return err
	}
	validCombination := false
	switch item.Role {
	case DailyPlanRoleWarmUp:
		validCombination = item.Type == DailyPlanReview && item.Reason == DailyPlanCriticalOverduePrerequisite
	case DailyPlanRoleReview:
		validCombination = item.Type == DailyPlanReview && item.Reason == DailyPlanImportantDueReview
	case DailyPlanRoleNewLearning:
		validCombination = item.Type == DailyPlanLearn && item.Reason == DailyPlanNextEligibleConcept
	case DailyPlanRoleReinforcement:
		validCombination = item.Type == DailyPlanPractice &&
			(item.Reason == DailyPlanBlockingWeakness || item.Reason == DailyPlanOptionalExtraPractice)
	}
	if !validCombination {
		return fmt.Errorf("daily plan item role, type, and reason are inconsistent")
	}
	return nil
}
