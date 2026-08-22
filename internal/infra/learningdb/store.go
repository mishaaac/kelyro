// Package learningdb binds Student Core application services to a
// workspace-local SQLite database.
package learningdb

import (
	"context"
	"fmt"
	"time"

	"github.com/mishaaac/kelyro/internal/infra/developmentfixture"
	"github.com/mishaaac/kelyro/internal/learning"
	"github.com/mishaaac/kelyro/internal/learning/application"
	"github.com/mishaaac/kelyro/internal/storage/sqlite"
	"github.com/mishaaac/kelyro/internal/version"
)

// Factory opens one profile store per workspace operation.
type Factory struct {
	appVersion           string
	backup               sqlite.BackupFunc
	now                  func() time.Time
	goalID               func() (learning.ID, error)
	curriculumInstanceID func() (learning.ID, error)
	studySessionID       func() (learning.ID, error)
	studySessionIdle     time.Duration
	algorithmSuite       *application.LearningAlgorithmSuite
}

// WithLearningAlgorithmSuite selects the internally supported derived-state
// algorithms used by maintenance recalculation.
func (factory *Factory) WithLearningAlgorithmSuite(suite application.LearningAlgorithmSuite) *Factory {
	factory.algorithmSuite = &suite
	return factory
}

func NewFactory(appVersion ...string) *Factory {
	version := "unknown"
	if len(appVersion) > 0 {
		version = appVersion[0]
	}
	return &Factory{appVersion: version, now: time.Now}
}

func (factory *Factory) WithMigrationBackup(create sqlite.BackupFunc) *Factory {
	factory.backup = create
	return factory
}

// WithStudySessionIdleTimeout configures the maximum inactive interval counted
// by newly started study sessions. Existing sessions keep their captured value.
func (factory *Factory) WithStudySessionIdleTimeout(timeout time.Duration) *Factory {
	factory.studySessionIdle = timeout
	return factory
}

func (factory *Factory) Open(ctx context.Context, workspaceRoot string) (application.ProfileStore, error) {
	if factory.studySessionIdle < 0 {
		return nil, fmt.Errorf("study session idle timeout cannot be negative")
	}
	database, err := sqlite.Open(ctx, workspaceRoot, sqlite.WithAppVersion(factory.appVersion), sqlite.WithDestructiveMigrationBackup(factory.backup))
	if err != nil {
		return nil, err
	}
	now := factory.now
	if now == nil {
		now = time.Now
	}
	students := application.NewStudentService(database.LearningRepositories().Students)
	profiles := application.NewProfileService(students, application.WithProfileClock(now))
	goalOptions := []application.GoalLifecycleOption{application.WithGoalClock(now)}
	if factory.goalID != nil {
		goalOptions = append(goalOptions, application.WithGoalIDGenerator(factory.goalID))
	}
	goals := application.NewGoalLifecycleService(profiles, database, goalOptions...)
	mastery := application.NewMasteryPolicyService(profiles, database.LearningRepositories().Mastery,
		application.WithMasteryPolicyClock(now))
	instanceOptions := []application.CurriculumInstanceOption{application.WithCurriculumInstanceClock(now)}
	if factory.curriculumInstanceID != nil {
		instanceOptions = append(instanceOptions, application.WithCurriculumInstanceIDGenerator(factory.curriculumInstanceID))
	}
	curriculumInstances := application.NewCurriculumInstanceService(profiles, database, instanceOptions...)
	diagnostics := application.NewDiagnosticService(profiles, database, application.WithDiagnosticClock(now))
	onboarding := application.NewOnboardingService(profiles, goals, database.LearningRepositories().Onboarding,
		application.WithOnboardingClock(now), application.WithOnboardingMasteryPolicy(mastery),
		application.WithOnboardingHistory(database.LearningRepositories().History))
	curriculum, diagnostic, fixtureErr := developmentfixture.FoundationDemo()
	if fixtureErr != nil {
		_ = database.Close()
		return nil, fixtureErr
	}
	setup := application.NewLearnerSetupService(profiles, onboarding, curriculumInstances, diagnostics, database, curriculum, diagnostic,
		application.WithLearnerSetupClock(now), application.WithDevelopmentSetupReset(version.IsDevelopment(factory.appVersion)))
	mistakes := application.NewMistakeMemoryService(profiles, database, application.WithMistakeMemoryClock(now))
	studySessionOptions := []application.StudySessionOption{application.WithStudySessionClock(now)}
	if factory.studySessionID != nil {
		studySessionOptions = append(studySessionOptions, application.WithStudySessionIDGenerator(factory.studySessionID))
	}
	if factory.studySessionIdle > 0 {
		studySessionOptions = append(studySessionOptions, application.WithStudySessionIdleTimeout(factory.studySessionIdle))
	}
	studySessions := application.NewStudySessionLifecycleService(profiles, database, studySessionOptions...)
	history := application.NewStudyHistoryService(profiles, database, application.WithStudyHistoryClock(now))
	retention := application.NewRetentionService(profiles, database, application.WithRetentionClock(now))
	reviews := application.NewReviewSchedulerService(profiles, database, application.WithReviewSchedulerClock(now))
	warmUps := application.NewWarmUpSelectorService(profiles, database, application.WithWarmUpSelectorClock(now))
	streaks := application.NewStreakService(profiles, database, application.WithStreakClock(now))
	achievements := application.NewAchievementService(profiles, database, application.WithAchievementClock(now))
	analytics := application.NewLearningAnalyticsService(profiles, database, application.WithLearningAnalyticsClock(now))
	dailyPlan := application.NewAdaptiveDailyPlanService(profiles, mastery, database, application.WithAdaptiveDailyPlanClock(now))
	dashboard := application.NewProgressDashboardService(profiles, mastery, dailyPlan, database, application.WithProgressDashboardClock(now))
	maintenanceOptions := []application.MaintenanceRecalculationOption{application.WithMaintenanceRecalculationClock(now)}
	if factory.algorithmSuite != nil {
		maintenanceOptions = append(maintenanceOptions, application.WithLearningAlgorithmSuite(*factory.algorithmSuite))
	}
	maintenance := application.NewMaintenanceRecalculationService(profiles, mastery, database, maintenanceOptions...)
	return &store{
		database: database, profiles: profiles,
		goals: goals, mastery: mastery, curriculumInstances: curriculumInstances, diagnostics: diagnostics,
		onboarding: onboarding, setup: setup, mistakes: mistakes, studySessions: studySessions, history: history,
		retention: retention, reviews: reviews, warmUps: warmUps, streaks: streaks, achievements: achievements,
		analytics: analytics, dailyPlan: dailyPlan, dashboard: dashboard, maintenance: maintenance,
	}, nil
}

type store struct {
	database            *sqlite.Database
	profiles            application.ProfileService
	goals               application.GoalLifecycleService
	onboarding          application.OnboardingService
	mastery             application.MasteryPolicyService
	curriculumInstances application.CurriculumInstanceService
	diagnostics         application.DiagnosticService
	setup               application.LearnerSetupService
	mistakes            application.MistakeMemoryService
	studySessions       application.StudySessionLifecycleService
	history             application.StudyHistoryService
	retention           application.RetentionService
	reviews             application.ReviewSchedulerService
	warmUps             application.WarmUpSelectorService
	streaks             application.StreakService
	achievements        application.AchievementService
	analytics           application.LearningAnalyticsService
	dailyPlan           application.AdaptiveDailyPlanService
	dashboard           application.ProgressDashboardService
	maintenance         application.MaintenanceRecalculationService
}

func (store *store) Profiles() application.ProfileService      { return store.profiles }
func (store *store) Goals() application.GoalLifecycleService   { return store.goals }
func (store *store) Onboarding() application.OnboardingService { return store.onboarding }
func (store *store) Mastery() application.MasteryPolicyService { return store.mastery }
func (store *store) CurriculumInstances() application.CurriculumInstanceService {
	return store.curriculumInstances
}
func (store *store) Diagnostics() application.DiagnosticService { return store.diagnostics }
func (store *store) Setup() application.LearnerSetupService     { return store.setup }
func (store *store) Mistakes() application.MistakeMemoryService { return store.mistakes }
func (store *store) StudySessions() application.StudySessionLifecycleService {
	return store.studySessions
}
func (store *store) History() application.StudyHistoryService        { return store.history }
func (store *store) Retention() application.RetentionService         { return store.retention }
func (store *store) Reviews() application.ReviewSchedulerService     { return store.reviews }
func (store *store) WarmUps() application.WarmUpSelectorService      { return store.warmUps }
func (store *store) Streaks() application.StreakService              { return store.streaks }
func (store *store) Achievements() application.AchievementService    { return store.achievements }
func (store *store) Analytics() application.LearningAnalyticsService { return store.analytics }
func (store *store) DailyPlan() application.AdaptiveDailyPlanService { return store.dailyPlan }
func (store *store) Dashboard() application.ProgressDashboardService { return store.dashboard }
func (store *store) Maintenance() application.MaintenanceRecalculationService {
	return store.maintenance
}

func (store *store) Close() error {
	if err := store.database.Close(); err != nil {
		return fmt.Errorf("close learner profile database: %w", err)
	}
	return nil
}

var _ application.ProfileStoreFactory = (*Factory)(nil)
var _ application.ProfileStore = (*store)(nil)
