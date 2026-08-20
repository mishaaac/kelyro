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

func (factory *Factory) Open(ctx context.Context, workspaceRoot string) (application.ProfileStore, error) {
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
		application.WithOnboardingClock(now), application.WithOnboardingMasteryPolicy(mastery))
	curriculum, diagnostic, fixtureErr := developmentfixture.FoundationDemo()
	if fixtureErr != nil {
		_ = database.Close()
		return nil, fixtureErr
	}
	setup := application.NewLearnerSetupService(profiles, onboarding, curriculumInstances, diagnostics, database, curriculum, diagnostic,
		application.WithLearnerSetupClock(now), application.WithDevelopmentSetupReset(version.IsDevelopment(factory.appVersion)))
	return &store{
		database: database, profiles: profiles,
		goals: goals, mastery: mastery, curriculumInstances: curriculumInstances, diagnostics: diagnostics,
		onboarding: onboarding, setup: setup,
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

func (store *store) Close() error {
	if err := store.database.Close(); err != nil {
		return fmt.Errorf("close learner profile database: %w", err)
	}
	return nil
}

var _ application.ProfileStoreFactory = (*Factory)(nil)
var _ application.ProfileStore = (*store)(nil)
