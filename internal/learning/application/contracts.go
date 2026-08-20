package application

import (
	"context"

	"github.com/mishaaac/kelyro/internal/learning"
)

// ProfileChanges contains only fields explicitly supplied by a profile edit.
// Pointer fields distinguish an omitted value from an intentional zero value,
// such as clearing the optional display name or all study preferences.
type ProfileChanges struct {
	DisplayName       *string
	Experience        *learning.ExperienceLevel
	PreferredLanguage *string
	DailyMinutes      *int
	WeeklyDaysTarget  *int
	Preferences       *[]learning.StudyPreference
	Timezone          *string
}

// ProfileService owns the single local learner profile for a workspace. Show
// creates deterministic defaults on first use; Edit applies a partial change.
type ProfileService interface {
	Show(context.Context) (learning.Student, error)
	Edit(context.Context, ProfileChanges) (learning.Student, error)
}

type SetGoalInput struct {
	Title            string
	Description      string
	Domain           string
	TargetOutcome    string
	StartingLevel    learning.ExperienceLevel
	MasteryThreshold learning.MasteryThreshold
}

// GoalLifecycleService owns the single-active-goal workspace policy while
// retaining every previous goal as history.
type GoalLifecycleService interface {
	Show(context.Context) ([]learning.LearningGoal, error)
	Set(context.Context, SetGoalInput) (learning.LearningGoal, error)
	Pause(context.Context) (learning.LearningGoal, error)
	Resume(context.Context) (learning.LearningGoal, error)
}

// OnboardingView is a presentation-neutral snapshot of the resumable wizard.
// Question is populated only while the interview is in progress.
type OnboardingView struct {
	Interview learning.OnboardingInterview
	Question  learning.OnboardingQuestion
	Position  int
	Total     int
}

type OnboardingConfirmation struct {
	View learning.OnboardingInterview
	Goal learning.LearningGoal
}

// OnboardingService exposes deterministic wizard transitions for TUI and
// future non-interactive adapters. Every successful transition is durable.
type OnboardingService interface {
	Show(context.Context) (OnboardingView, error)
	Start(context.Context) (OnboardingView, error)
	Submit(context.Context, string) (OnboardingView, error)
	Back(context.Context) (OnboardingView, error)
	Cancel(context.Context) (OnboardingView, error)
	Confirm(context.Context) (OnboardingConfirmation, error)
}

// MasteryPolicyService owns durable student/workspace threshold settings and
// resolves the effective requirement with an optional bounded pack override.
type MasteryPolicyService interface {
	Show(context.Context, *learning.PackMasteryOverride) (learning.ResolvedMasteryThreshold, error)
	SetStudentDefault(context.Context, learning.MasteryThreshold) (learning.ResolvedMasteryThreshold, error)
	SetWorkspaceOverride(context.Context, learning.MasteryThreshold) (learning.ResolvedMasteryThreshold, error)
	ClearWorkspaceOverride(context.Context) (learning.ResolvedMasteryThreshold, error)
}

// PrerequisiteService evaluates one concept against one durable student-state
// snapshot and the effective mastery policy. Graph traversal stays in domain.
type PrerequisiteService interface {
	EvaluateIntroduction(context.Context, learning.ID, learning.ID, *learning.PackMasteryOverride) (learning.IntroductionDecision, error)
}

// CurriculumInstanceService owns learner-scoped curriculum identity and lazy
// instance concept state. It never copies evidence into progress state.
type CurriculumInstanceService interface {
	Create(context.Context, learning.ID, learning.Curriculum, learning.CurriculumSourceKind) (learning.CurriculumInstance, error)
	Get(context.Context, learning.ID) (learning.CurriculumInstance, error)
	List(context.Context) ([]learning.CurriculumInstance, error)
	State(context.Context, learning.ID, learning.ID) (learning.InstanceConceptState, error)
	States(context.Context, learning.ID) ([]learning.InstanceConceptState, error)
	SaveState(context.Context, learning.InstanceConceptState) error
}

// DiagnosticView exposes the next deterministic item and estimated result.
// Result always labels in-progress attempts as partial and never represents
// confirmed mastery.
type DiagnosticView struct {
	Attempt learning.DiagnosticAttempt
	Item    *learning.DiagnosticItem
	Result  learning.DiagnosticResult
}

type DiagnosticService interface {
	Start(context.Context, learning.ID, learning.Diagnostic) (DiagnosticView, error)
	Resume(context.Context, learning.ID, learning.Diagnostic) (DiagnosticView, error)
	Submit(context.Context, learning.ID, learning.Diagnostic, []string) (DiagnosticView, error)
	Skip(context.Context, learning.ID, learning.Diagnostic) (DiagnosticView, error)
	Result(context.Context, learning.ID, learning.Diagnostic) (learning.DiagnosticResult, error)
}

// LearnerSetupView is the presentation-neutral projection of the first
// educational flow. Exactly one interactive child is normally active.
type LearnerSetupView struct {
	Setup      learning.LearnerSetup
	Onboarding *OnboardingView
	Diagnostic *DiagnosticView
	Instance   *learning.CurriculumInstance
}

type LearnerSetupService interface {
	Show(context.Context) (LearnerSetupView, error)
	Start(context.Context) (LearnerSetupView, error)
	SubmitOnboarding(context.Context, string) (LearnerSetupView, error)
	Back(context.Context) (LearnerSetupView, error)
	Cancel(context.Context) (LearnerSetupView, error)
	Confirm(context.Context) (LearnerSetupView, error)
	SubmitDiagnostic(context.Context, []string) (LearnerSetupView, error)
	SkipDiagnostic(context.Context) (LearnerSetupView, error)
	ResetDevelopment(context.Context) (LearnerSetupView, error)
}

// ProfileStore scopes Student Core profile and goal operations, plus their
// database lifetime, to one workspace without exposing SQLite to application
// or presentation packages. The historical name is retained for compatibility.
type ProfileStore interface {
	Profiles() ProfileService
	Goals() GoalLifecycleService
	Onboarding() OnboardingService
	Mastery() MasteryPolicyService
	CurriculumInstances() CurriculumInstanceService
	Diagnostics() DiagnosticService
	Setup() LearnerSetupService
	Close() error
}

type ProfileStoreFactory interface {
	Open(context.Context, string) (ProfileStore, error)
}

// Repository interfaces are intentionally separated by aggregate/use case.
// Implementations must return classified errors and must not expose storage
// handles, rows, or driver-specific values.
type StudentRepository interface {
	Create(context.Context, learning.Student) error
	Get(context.Context, learning.ID) (learning.Student, error)
	Update(context.Context, learning.Student) error
}

type GoalRepository interface {
	Create(context.Context, learning.LearningGoal) error
	Get(context.Context, learning.ID) (learning.LearningGoal, error)
	ListByStudent(context.Context, learning.ID) ([]learning.LearningGoal, error)
	Update(context.Context, learning.LearningGoal) error
}

type OnboardingRepository interface {
	Get(context.Context, learning.ID) (learning.OnboardingInterview, error)
	Save(context.Context, learning.OnboardingInterview) error
}

type MasteryThresholdRepository interface {
	Get(context.Context, learning.ID) (learning.MasteryThresholdSettings, error)
	Save(context.Context, learning.MasteryThresholdSettings) error
}

// CurriculumStateRepository is the compact read port introduced before the
// full curriculum definition contract. YAML decoding is a separate adapter;
// graph traversal and progression policies belong to later I-02 steps.
type CurriculumStateRepository interface {
	Concept(context.Context, learning.CurriculumRef, learning.ID) (learning.Concept, error)
	Concepts(context.Context, learning.CurriculumRef) ([]learning.Concept, error)
	Prerequisites(context.Context, learning.CurriculumRef, learning.ID) ([]learning.Prerequisite, error)
}

type CurriculumDefinitionRepository interface {
	Install(context.Context, learning.Curriculum) error
}

type CurriculumInstanceRepository interface {
	Create(context.Context, learning.CurriculumInstance) error
	Get(context.Context, learning.ID) (learning.CurriculumInstance, error)
	ListByStudent(context.Context, learning.ID) ([]learning.CurriculumInstance, error)
}

type InstanceConceptStateRepository interface {
	Get(context.Context, learning.ID, learning.ID) (learning.InstanceConceptState, error)
	ListByInstance(context.Context, learning.ID) ([]learning.InstanceConceptState, error)
	Save(context.Context, learning.InstanceConceptState) error
}

type DiagnosticRepository interface {
	Create(context.Context, learning.DiagnosticAttempt) error
	Get(context.Context, learning.ID) (learning.DiagnosticAttempt, error)
	Find(context.Context, learning.ID, learning.ID, learning.DiagnosticRef) (learning.DiagnosticAttempt, error)
	Save(context.Context, learning.DiagnosticAttempt) error
}

type LearnerSetupRepository interface {
	Get(context.Context, learning.ID) (learning.LearnerSetup, error)
	Save(context.Context, learning.LearnerSetup) error
	ResetDevelopment(context.Context, learning.ID) error
}

type ConceptStateRepository interface {
	Get(context.Context, learning.ID, learning.ID) (learning.ConceptState, error)
	ListByStudent(context.Context, learning.ID) ([]learning.ConceptState, error)
	Save(context.Context, learning.ConceptState) error
}

type EvidenceRepository interface {
	Append(context.Context, learning.Evidence) error
	ListByConcept(context.Context, learning.ID, learning.ID) ([]learning.Evidence, error)
}

type MistakeRepository interface {
	Create(context.Context, learning.Mistake) error
	ListByConcept(context.Context, learning.ID, learning.ID) ([]learning.Mistake, error)
	Update(context.Context, learning.Mistake) error
}

type RetentionRepository interface {
	Get(context.Context, learning.ID, learning.ID) (learning.RetentionState, error)
	Save(context.Context, learning.RetentionState) error
}

type SessionRepository interface {
	Append(context.Context, learning.LearningSession) error
	Get(context.Context, learning.ID) (learning.LearningSession, error)
	ListByGoal(context.Context, learning.ID, learning.ID) ([]learning.LearningSession, error)
}

type ReviewRepository interface {
	GetSchedule(context.Context, learning.ID, learning.ID) (learning.ReviewSchedule, error)
	SaveSchedule(context.Context, learning.ReviewSchedule) error
	CreateItem(context.Context, learning.ReviewItem) error
	UpdateItem(context.Context, learning.ReviewItem) error
	ListDue(context.Context, learning.ID, learning.Timestamp) ([]learning.ReviewItem, error)
}

type StreakRepository interface {
	Get(context.Context, learning.ID) (learning.Streak, error)
	Save(context.Context, learning.Streak) error
}

type AchievementRepository interface {
	Get(context.Context, learning.ID) (learning.Achievement, error)
	ListByStudent(context.Context, learning.ID) ([]learning.Achievement, error)
	Save(context.Context, learning.Achievement) error
	AppendMilestone(context.Context, learning.Milestone) error
	ListMilestones(context.Context, learning.ID, learning.ID) ([]learning.Milestone, error)
}

type AnalyticsRepository interface {
	Append(context.Context, learning.AnalyticsSnapshot) error
	Latest(context.Context, learning.ID) (learning.AnalyticsSnapshot, error)
}

type DailyPlanRepository interface {
	Save(context.Context, learning.DailyPlan) error
	ForDate(context.Context, learning.ID, learning.ID, learning.Timestamp) (learning.DailyPlan, error)
}

// Repositories groups separate ports only to give a transaction callback a
// coherent set backed by the same transaction. It is not a mega-repository and
// is never passed to presentation code.
type Repositories struct {
	Students              StudentRepository
	Goals                 GoalRepository
	Onboarding            OnboardingRepository
	Mastery               MasteryThresholdRepository
	Definitions           CurriculumDefinitionRepository
	Curricula             CurriculumStateRepository
	CurriculumInstances   CurriculumInstanceRepository
	Concepts              ConceptStateRepository
	InstanceConceptStates InstanceConceptStateRepository
	Diagnostics           DiagnosticRepository
	Setup                 LearnerSetupRepository
	Evidence              EvidenceRepository
	Mistakes              MistakeRepository
	Retention             RetentionRepository
	Sessions              SessionRepository
	Reviews               ReviewRepository
	Streaks               StreakRepository
	Achievements          AchievementRepository
	Analytics             AnalyticsRepository
	DailyPlans            DailyPlanRepository
}

// UnitOfWork supplies repositories that commit or roll back together. A future
// SQLite adapter may back this with sql.Tx without exposing it to this package.
type UnitOfWork interface {
	WithinTransaction(context.Context, func(Repositories) error) error
}

type StudentService interface {
	Create(context.Context, learning.Student) error
	Get(context.Context, learning.ID) (learning.Student, error)
	Update(context.Context, learning.Student) error
}

type GoalService interface {
	Create(context.Context, learning.LearningGoal) error
	Get(context.Context, learning.ID) (learning.LearningGoal, error)
	List(context.Context, learning.ID) ([]learning.LearningGoal, error)
	Update(context.Context, learning.LearningGoal) error
}

// ConceptProgress is a read model assembled from independent persistence
// ports. It reports stored facts and deliberately performs no mastery policy.
type ConceptProgress struct {
	State    learning.ConceptState
	Evidence []learning.Evidence
	Mistakes []learning.Mistake
}

type ProgressService interface {
	Concept(context.Context, learning.ID, learning.ID) (ConceptProgress, error)
	RecordEvidence(context.Context, learning.Evidence) error
	RecordMistake(context.Context, learning.Mistake) error
	SaveConceptState(context.Context, learning.ConceptState) error
}

type SessionService interface {
	Record(context.Context, learning.LearningSession) error
	Get(context.Context, learning.ID) (learning.LearningSession, error)
	List(context.Context, learning.ID, learning.ID) ([]learning.LearningSession, error)
}

type ReviewService interface {
	SaveSchedule(context.Context, learning.ReviewSchedule) error
	Create(context.Context, learning.ReviewItem) error
	Update(context.Context, learning.ReviewItem) error
	Due(context.Context, learning.ID, learning.Timestamp) ([]learning.ReviewItem, error)
}

type AnalyticsService interface {
	Record(context.Context, learning.AnalyticsSnapshot) error
	Latest(context.Context, learning.ID) (learning.AnalyticsSnapshot, error)
}

type DailyPlanService interface {
	Save(context.Context, learning.DailyPlan) error
	ForDate(context.Context, learning.ID, learning.ID, learning.Timestamp) (learning.DailyPlan, error)
}
