// Package memory provides deterministic in-memory repository fakes for
// Student Core application tests. It is not a production persistence format.
package memory

import (
	"context"
	"errors"
	"sync"

	"github.com/mishaaac/kelyro/internal/learning"
	"github.com/mishaaac/kelyro/internal/learning/application"
)

type studentConceptKey struct {
	student learning.ID
	concept learning.ID
}

type curriculumKey struct {
	id      learning.ID
	version string
}

type instanceConceptKey struct {
	instance learning.ID
	concept  learning.ID
}

type planKey struct {
	student learning.ID
	goal    learning.ID
	date    int64
}

type curriculumFixture struct {
	concepts      map[learning.ID]learning.Concept
	prerequisites []learning.Prerequisite
	fingerprint   string
}

// Store owns independent maps for every repository port. New returns an empty
// store and Repositories exposes narrow adapters over it.
type Store struct {
	mu sync.RWMutex

	students       map[learning.ID]learning.Student
	goals          map[learning.ID]learning.LearningGoal
	onboarding     map[learning.ID]learning.OnboardingInterview
	mastery        map[learning.ID]learning.MasteryThresholdSettings
	curricula      map[curriculumKey]curriculumFixture
	instances      map[learning.ID]learning.CurriculumInstance
	instanceStates map[instanceConceptKey]learning.InstanceConceptState
	diagnostics    map[learning.ID]learning.DiagnosticAttempt
	concepts       map[studentConceptKey]learning.ConceptState
	evidence       map[learning.ID]learning.Evidence
	mistakes       map[learning.ID]learning.Mistake
	retention      map[studentConceptKey]learning.RetentionState
	sessions       map[learning.ID]learning.LearningSession
	schedules      map[studentConceptKey]learning.ReviewSchedule
	reviewItems    map[learning.ID]learning.ReviewItem
	streaks        map[learning.ID]learning.Streak
	achievements   map[learning.ID]learning.Achievement
	milestones     map[learning.ID]learning.Milestone
	analytics      map[learning.ID][]learning.AnalyticsSnapshot
	dailyPlans     map[planKey]learning.DailyPlan
}

func New() *Store {
	return &Store{
		students:       make(map[learning.ID]learning.Student),
		goals:          make(map[learning.ID]learning.LearningGoal),
		onboarding:     make(map[learning.ID]learning.OnboardingInterview),
		mastery:        make(map[learning.ID]learning.MasteryThresholdSettings),
		curricula:      make(map[curriculumKey]curriculumFixture),
		instances:      make(map[learning.ID]learning.CurriculumInstance),
		instanceStates: make(map[instanceConceptKey]learning.InstanceConceptState),
		diagnostics:    make(map[learning.ID]learning.DiagnosticAttempt),
		concepts:       make(map[studentConceptKey]learning.ConceptState),
		evidence:       make(map[learning.ID]learning.Evidence),
		mistakes:       make(map[learning.ID]learning.Mistake),
		retention:      make(map[studentConceptKey]learning.RetentionState),
		sessions:       make(map[learning.ID]learning.LearningSession),
		schedules:      make(map[studentConceptKey]learning.ReviewSchedule),
		reviewItems:    make(map[learning.ID]learning.ReviewItem),
		streaks:        make(map[learning.ID]learning.Streak),
		achievements:   make(map[learning.ID]learning.Achievement),
		milestones:     make(map[learning.ID]learning.Milestone),
		analytics:      make(map[learning.ID][]learning.AnalyticsSnapshot),
		dailyPlans:     make(map[planKey]learning.DailyPlan),
	}
}

func (store *Store) Repositories() application.Repositories {
	return application.Repositories{
		Students:              studentRepository{store},
		Goals:                 goalRepository{store},
		Onboarding:            onboardingRepository{store},
		Mastery:               masteryThresholdRepository{store},
		Definitions:           curriculumDefinitionRepository{store},
		Curricula:             curriculumRepository{store},
		CurriculumInstances:   curriculumInstanceRepository{store},
		InstanceConceptStates: instanceConceptStateRepository{store},
		Diagnostics:           diagnosticRepository{store},
		Concepts:              conceptRepository{store},
		Evidence:              evidenceRepository{store},
		Mistakes:              mistakeRepository{store},
		Retention:             retentionRepository{store},
		Sessions:              sessionRepository{store},
		Reviews:               reviewRepository{store},
		Streaks:               streakRepository{store},
		Achievements:          achievementRepository{store},
		Analytics:             analyticsRepository{store},
		DailyPlans:            dailyPlanRepository{store},
	}
}

// WithinTransaction runs work against an isolated copy and publishes all
// writes only when work succeeds. It gives tests real commit/rollback behavior
// without SQLite.
func (store *Store) WithinTransaction(ctx context.Context, work func(application.Repositories) error) error {
	if work == nil {
		return application.Classify(application.ErrorInvalidState, "run memory transaction", errors.New("transaction callback is nil"))
	}
	if err := contextError("run memory transaction", ctx); err != nil {
		return err
	}

	store.mu.Lock()
	defer store.mu.Unlock()

	transaction := store.cloneLocked()
	if err := work(transaction.Repositories()); err != nil {
		return err
	}
	if err := contextError("commit memory transaction", ctx); err != nil {
		return err
	}
	store.replaceLocked(transaction)
	return nil
}

// SeedCurriculum installs a deterministic versioned fixture for application
// tests. Production curriculum ingestion is intentionally outside this step.
func (store *Store) SeedCurriculum(reference learning.CurriculumRef, concepts []learning.Concept, prerequisites []learning.Prerequisite) error {
	if err := reference.Validate(); err != nil {
		return application.Classify(application.ErrorInvalidState, "seed memory curriculum", err)
	}
	fixture := curriculumFixture{concepts: make(map[learning.ID]learning.Concept, len(concepts))}
	for _, concept := range concepts {
		if err := concept.Validate(); err != nil {
			return application.Classify(application.ErrorInvalidState, "seed memory curriculum", err)
		}
		if _, exists := fixture.concepts[concept.ID]; exists {
			return conflict("seed memory curriculum")
		}
		fixture.concepts[concept.ID] = concept
	}
	for _, prerequisite := range prerequisites {
		if err := prerequisite.Validate(); err != nil {
			return application.Classify(application.ErrorInvalidState, "seed memory curriculum", err)
		}
		fixture.prerequisites = append(fixture.prerequisites, prerequisite)
	}

	store.mu.Lock()
	defer store.mu.Unlock()
	key := curriculumKey{id: reference.ID, version: reference.Version}
	if _, exists := store.curricula[key]; exists {
		return conflict("seed memory curriculum")
	}
	store.curricula[key] = fixture
	return nil
}

func (store *Store) cloneLocked() *Store {
	clone := New()
	for key, value := range store.students {
		clone.students[key] = cloneStudent(value)
	}
	for key, value := range store.goals {
		clone.goals[key] = cloneGoal(value)
	}
	for key, value := range store.onboarding {
		clone.onboarding[key] = cloneOnboarding(value)
	}
	for key, value := range store.mastery {
		clone.mastery[key] = cloneMasterySettings(value)
	}
	for key, value := range store.curricula {
		fixture := curriculumFixture{concepts: make(map[learning.ID]learning.Concept, len(value.concepts)), fingerprint: value.fingerprint}
		for conceptID, concept := range value.concepts {
			fixture.concepts[conceptID] = concept
		}
		fixture.prerequisites = append([]learning.Prerequisite(nil), value.prerequisites...)
		clone.curricula[key] = fixture
	}
	for key, value := range store.instances {
		clone.instances[key] = value
	}
	for key, value := range store.instanceStates {
		clone.instanceStates[key] = cloneInstanceConceptState(value)
	}
	for key, value := range store.diagnostics {
		clone.diagnostics[key] = cloneDiagnosticAttempt(value)
	}
	for key, value := range store.concepts {
		clone.concepts[key] = cloneConceptState(value)
	}
	for key, value := range store.evidence {
		clone.evidence[key] = value
	}
	for key, value := range store.mistakes {
		clone.mistakes[key] = cloneMistake(value)
	}
	for key, value := range store.retention {
		clone.retention[key] = value
	}
	for key, value := range store.sessions {
		clone.sessions[key] = cloneSession(value)
	}
	for key, value := range store.schedules {
		clone.schedules[key] = cloneSchedule(value)
	}
	for key, value := range store.reviewItems {
		clone.reviewItems[key] = cloneReviewItem(value)
	}
	for key, value := range store.streaks {
		clone.streaks[key] = cloneStreak(value)
	}
	for key, value := range store.achievements {
		clone.achievements[key] = cloneAchievement(value)
	}
	for key, value := range store.milestones {
		clone.milestones[key] = value
	}
	for key, values := range store.analytics {
		clone.analytics[key] = append([]learning.AnalyticsSnapshot(nil), values...)
	}
	for key, value := range store.dailyPlans {
		clone.dailyPlans[key] = cloneDailyPlan(value)
	}
	return clone
}

func (store *Store) replaceLocked(replacement *Store) {
	store.students = replacement.students
	store.goals = replacement.goals
	store.onboarding = replacement.onboarding
	store.mastery = replacement.mastery
	store.curricula = replacement.curricula
	store.instances = replacement.instances
	store.instanceStates = replacement.instanceStates
	store.diagnostics = replacement.diagnostics
	store.concepts = replacement.concepts
	store.evidence = replacement.evidence
	store.mistakes = replacement.mistakes
	store.retention = replacement.retention
	store.sessions = replacement.sessions
	store.schedules = replacement.schedules
	store.reviewItems = replacement.reviewItems
	store.streaks = replacement.streaks
	store.achievements = replacement.achievements
	store.milestones = replacement.milestones
	store.analytics = replacement.analytics
	store.dailyPlans = replacement.dailyPlans
}

func contextError(operation string, ctx context.Context) error {
	if err := ctx.Err(); err != nil {
		return application.Classify(application.ErrorUnavailable, operation, err)
	}
	return nil
}

func notFound(operation string) error {
	return application.Classify(application.ErrorNotFound, operation, errors.New("record does not exist"))
}

func conflict(operation string) error {
	return application.Classify(application.ErrorConflict, operation, errors.New("record already exists"))
}

func cloneStudent(value learning.Student) learning.Student {
	value.Profile.Preferences = append([]learning.StudyPreference(nil), value.Profile.Preferences...)
	value.Profile.Availability.PreferredDays = append([]int(nil), value.Profile.Availability.PreferredDays...)
	return value
}

func cloneGoal(value learning.LearningGoal) learning.LearningGoal {
	if value.ActivatedAt != nil {
		copy := *value.ActivatedAt
		value.ActivatedAt = &copy
	}
	if value.CompletedAt != nil {
		copy := *value.CompletedAt
		value.CompletedAt = &copy
	}
	return value
}

func cloneConceptState(value learning.ConceptState) learning.ConceptState {
	if value.IntroducedAt != nil {
		copy := *value.IntroducedAt
		value.IntroducedAt = &copy
	}
	return value
}

func cloneInstanceConceptState(value learning.InstanceConceptState) learning.InstanceConceptState {
	if value.FirstSeenAt != nil {
		copy := *value.FirstSeenAt
		value.FirstSeenAt = &copy
	}
	if value.LastSeenAt != nil {
		copy := *value.LastSeenAt
		value.LastSeenAt = &copy
	}
	if value.MasteredAt != nil {
		copy := *value.MasteredAt
		value.MasteredAt = &copy
	}
	if value.ReviewDueAt != nil {
		copy := *value.ReviewDueAt
		value.ReviewDueAt = &copy
	}
	value.ManualFlags = append([]string(nil), value.ManualFlags...)
	return value
}

func cloneDiagnosticAttempt(value learning.DiagnosticAttempt) learning.DiagnosticAttempt {
	value.Observations = append([]learning.DiagnosticObservation(nil), value.Observations...)
	if value.CompletedAt != nil {
		copy := *value.CompletedAt
		value.CompletedAt = &copy
	}
	if value.SkippedAt != nil {
		copy := *value.SkippedAt
		value.SkippedAt = &copy
	}
	return value
}

func cloneMistake(value learning.Mistake) learning.Mistake {
	if value.ResolvedAt != nil {
		copy := *value.ResolvedAt
		value.ResolvedAt = &copy
	}
	return value
}

func cloneSession(value learning.LearningSession) learning.LearningSession {
	value.Activities = append([]learning.StudyActivity(nil), value.Activities...)
	for index := range value.Activities {
		value.Activities[index].ConceptIDs = append([]learning.ID(nil), value.Activities[index].ConceptIDs...)
	}
	return value
}

func cloneSchedule(value learning.ReviewSchedule) learning.ReviewSchedule {
	if value.IntroducedAt != nil {
		copy := *value.IntroducedAt
		value.IntroducedAt = &copy
	}
	return value
}

func cloneReviewItem(value learning.ReviewItem) learning.ReviewItem {
	if value.CompletedAt != nil {
		copy := *value.CompletedAt
		value.CompletedAt = &copy
	}
	return value
}

func cloneStreak(value learning.Streak) learning.Streak {
	if value.LastStudyAt != nil {
		copy := *value.LastStudyAt
		value.LastStudyAt = &copy
	}
	return value
}

func cloneAchievement(value learning.Achievement) learning.Achievement {
	if value.UnlockedAt != nil {
		copy := *value.UnlockedAt
		value.UnlockedAt = &copy
	}
	return value
}

func cloneDailyPlan(value learning.DailyPlan) learning.DailyPlan {
	value.Items = append([]learning.DailyPlanItem(nil), value.Items...)
	for index := range value.Items {
		value.Items[index].ConceptIDs = append([]learning.ID(nil), value.Items[index].ConceptIDs...)
	}
	return value
}
