package app

import (
	"context"
	"errors"
	"fmt"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/mishaaac/kelyro/internal/artifacts"
	artifactmarkdown "github.com/mishaaac/kelyro/internal/artifacts/markdown"
	"github.com/mishaaac/kelyro/internal/audit"
	"github.com/mishaaac/kelyro/internal/backup"
	"github.com/mishaaac/kelyro/internal/config"
	"github.com/mishaaac/kelyro/internal/doctor"
	"github.com/mishaaac/kelyro/internal/editor"
	"github.com/mishaaac/kelyro/internal/learning"
	learningapp "github.com/mishaaac/kelyro/internal/learning/application"
	"github.com/mishaaac/kelyro/internal/logging"
	"github.com/mishaaac/kelyro/internal/platform"
	"github.com/mishaaac/kelyro/internal/portability"
	"github.com/mishaaac/kelyro/internal/research"
	researchapp "github.com/mishaaac/kelyro/internal/research/application"
	"github.com/mishaaac/kelyro/internal/session"
	"github.com/mishaaac/kelyro/internal/storage"
	"github.com/mishaaac/kelyro/internal/update"
	"github.com/mishaaac/kelyro/internal/workspace"
)

// Action identifies a Foundation operation requested by a presentation
// adapter.
type Action string

const (
	ActionTUI          Action = "tui"
	ActionInit         Action = "init"
	ActionDoctor       Action = "doctor"
	ActionConfig       Action = "config"
	ActionSecrets      Action = "secrets"
	ActionStatus       Action = "status"
	ActionProgress     Action = "progress"
	ActionRoadmap      Action = "roadmap"
	ActionToday        Action = "today"
	ActionOpen         Action = "open"
	ActionLogs         Action = "logs"
	ActionAudit        Action = "audit"
	ActionBackup       Action = "backup"
	ActionExport       Action = "export"
	ActionImport       Action = "import"
	ActionUpdate       Action = "update"
	ActionProfile      Action = "profile"
	ActionGoal         Action = "goal"
	ActionOnboarding   Action = "onboarding"
	ActionMastery      Action = "mastery"
	ActionSetup        Action = "setup"
	ActionMistakes     Action = "mistakes"
	ActionSession      Action = "session"
	ActionHistory      Action = "history"
	ActionTime         Action = "time"
	ActionReviews      Action = "reviews"
	ActionStreak       Action = "streak"
	ActionAchievements Action = "achievements"
	ActionDashboard    Action = "dashboard"
	ActionMaintenance  Action = "maintenance"
	ActionSources      Action = "sources"
	ActionResearch     Action = "research"
)

// Command contains presentation-independent input for a Foundation action.
type Command struct {
	Action                  Action
	Workspace               string
	AllowNested             bool
	ConfigOperation         string
	ConfigScope             config.Scope
	ConfigKey               string
	ConfigValue             string
	ConfigOverrides         config.Settings
	SecretOperation         string
	SecretName              string
	SecretValue             string
	OpenTarget              string
	DoctorContext           doctor.Context
	DoctorExplain           string
	LogOperation            string
	BackupOperation         string
	BackupID                string
	BackupConfirmed         bool
	ExportMode              portability.Mode
	ExportOutput            string
	ImportArchive           string
	ImportDryRun            bool
	ImportConflicts         portability.ConflictStrategy
	UpdateOperation         string
	ProfileOperation        string
	ProfileChanges          learningapp.ProfileChanges
	GoalOperation           string
	GoalInput               learningapp.SetGoalInput
	OnboardingOperation     string
	OnboardingAnswer        string
	MasteryOperation        string
	MasteryThreshold        learning.MasteryThreshold
	SetupOperation          string
	SetupAnswers            []string
	MistakeOperation        string
	MistakeID               learning.ID
	SessionOperation        string
	HistoryToday            bool
	ProgressOperation       string
	MaintenanceOperation    string
	MaintenanceDryRun       bool
	ReviewsDue              bool
	SourceRegistryOperation string
	ResearchCacheOperation  string
	SourceRegistryID        research.ID
	ProvenanceClaimID       research.ClaimID
	Verbose                 bool
}

// Result contains presentation-independent output from a Foundation action.
type Result struct {
	Message               string
	Diagnostics           *doctor.Report
	Guidance              *doctor.Guidance
	Failed                bool
	Audit                 []audit.Entry
	Backups               []backup.Info
	Portability           *portability.Report
	Update                *update.Result
	Profile               *learning.Student
	Goal                  *learning.LearningGoal
	Goals                 []learning.LearningGoal
	Onboarding            *learningapp.OnboardingView
	Mastery               *learning.ResolvedMasteryThreshold
	Setup                 *learningapp.LearnerSetupView
	Mistake               *learningapp.MistakeView
	Mistakes              []learning.Mistake
	StudySession          *learning.StudySession
	History               *learningapp.StudyHistoryView
	StudyTime             *learningapp.StudyTimeSummary
	Reviews               *learningapp.ReviewQueueView
	Streak                *learning.Streak
	Achievements          *learningapp.AchievementRefresh
	Dashboard             *learningapp.ProgressDashboard
	Maintenance           *learningapp.RecalculationImpact
	SourceRegistryEntry   *research.SourceRegistryEntry
	SourceRegistryEntries []research.SourceRegistryEntry
	ProvenanceGraph       *research.ProvenanceGraph
	StaleSources          []researchapp.FreshnessRecord
	ResearchCacheStatus   *researchapp.ResearchCacheStatus
	ResearchCacheCleared  *researchapp.ResearchCacheClearResult
}

// FoundationService executes the operations currently exposed by the CLI.
type FoundationService interface {
	Execute(ctx context.Context, command Command) (Result, error)
}

// Service coordinates implemented Foundation operations while retaining
// explicit placeholders for operations assigned to later steps.
type Service struct {
	workspaces       workspace.Service
	configs          config.Store
	secrets          storage.SecretStore
	artifactStores   artifacts.WorkspaceStoreFactory
	sessionStores    session.WorkspaceStoreFactory
	editors          editor.Service
	diagnostics      DoctorRunner
	loggers          logging.WorkspaceFactory
	audits           audit.WorkspaceStoreFactory
	backups          backup.Service
	portability      portability.Service
	updates          update.Checker
	profiles         learningapp.ProfileStoreFactory
	researchStores   researchapp.SourceRegistryStoreFactory
	researchCaches   researchapp.ResearchCacheServiceFactory
	researchClock    func() time.Time
	currentDirectory func() (string, error)
	bootstrap        BootstrapService
}

// WithBackups attaches safe workspace backup and restore operations.
func (service *Service) WithBackups(backups backup.Service) *Service {
	service.backups = backups
	return service
}

// WithPortability attaches portable workspace archive operations.
func (service *Service) WithPortability(portable portability.Service) *Service {
	service.portability = portable
	return service
}

// WithUpdates attaches version-aware metadata checks. It does not provide an
// installer and therefore cannot modify the running binary.
func (service *Service) WithUpdates(checker update.Checker) *Service {
	service.updates = checker
	return service
}

// WithProfiles attaches the workspace-local learner profile store.
func (service *Service) WithProfiles(profiles learningapp.ProfileStoreFactory) *Service {
	service.profiles = profiles
	return service
}

func (service *Service) WithResearchStores(stores researchapp.SourceRegistryStoreFactory) *Service {
	service.researchStores = stores
	return service
}

// WithResearchCaches attaches workspace-local disposable Research cache.
func (service *Service) WithResearchCaches(caches researchapp.ResearchCacheServiceFactory) *Service {
	service.researchCaches = caches
	return service
}

// WithResearchClock replaces the clock used to evaluate stale schedules.
func (service *Service) WithResearchClock(clock func() time.Time) *Service {
	service.researchClock = clock
	return service
}

// NewService creates the application service with explicit infrastructure
// dependencies.
func NewService(workspaces workspace.Service, currentDirectory func() (string, error)) *Service {
	return &Service{workspaces: workspaces, currentDirectory: currentDirectory, researchClock: time.Now}
}

// WithConfig attaches the configuration persistence adapter used by config
// commands. It keeps construction explicit without coupling the application
// package to a filesystem implementation.
func (service *Service) WithConfig(configs config.Store) *Service {
	service.configs = configs
	return service
}

// WithSecrets attaches replaceable secret storage without exposing its native
// backend to application policy.
func (service *Service) WithSecrets(secrets storage.SecretStore) *Service {
	service.secrets = secrets
	return service
}

// WithArtifactStores attaches the per-workspace persistence used for generated
// human-readable documents.
func (service *Service) WithArtifactStores(stores artifacts.WorkspaceStoreFactory) *Service {
	service.artifactStores = stores
	return service
}

// WithSessionStores attaches versioned workspace session persistence.
func (service *Service) WithSessionStores(stores session.WorkspaceStoreFactory) *Service {
	service.sessionStores = stores
	return service
}

// WithEditor attaches the native editor integration behind its replaceable
// application contract.
func (service *Service) WithEditor(editors editor.Service) *Service {
	service.editors = editors
	return service
}

// WithDoctor attaches the presentation-independent diagnostics engine.
func (service *Service) WithDoctor(diagnostics DoctorRunner) *Service {
	service.diagnostics = diagnostics
	return service
}

// WithLogging attaches workspace-local structured diagnostic logging.
func (service *Service) WithLogging(loggers logging.WorkspaceFactory) *Service {
	service.loggers = loggers
	return service
}

// WithAudit attaches the durable audit trail used by critical operations and
// the audit CLI command.
func (service *Service) WithAudit(audits audit.WorkspaceStoreFactory) *Service {
	service.audits = audits
	return service
}

// execute coordinates implemented Foundation actions beneath the public
// observability wrapper.
func (service *Service) execute(ctx context.Context, command Command) (Result, error) {
	if err := ctx.Err(); err != nil {
		return Result{}, err
	}
	if command.Action == ActionConfig {
		return service.executeConfig(ctx, command)
	}
	if command.Action == ActionSecrets {
		return service.executeSecrets(command)
	}
	if command.Action == ActionOpen {
		return service.executeOpen(ctx, command)
	}
	if command.Action == ActionProgress && command.ProgressOperation == "export" {
		return service.executeProgressExport(ctx, command)
	}
	if command.Action == ActionMaintenance {
		return service.executeMaintenance(ctx, command)
	}
	if command.Action == ActionSources {
		return service.executeSourceRegistry(ctx, command)
	}
	if command.Action == ActionResearch {
		return service.executeResearchCache(ctx, command)
	}
	if command.Action == ActionStatus || command.Action == ActionProgress || command.Action == ActionRoadmap || command.Action == ActionToday {
		return service.executeDashboard(ctx, command)
	}
	if command.Action == ActionDoctor {
		return service.executeDoctor(ctx, command)
	}
	if command.Action == ActionLogs {
		return service.executeLogs(command)
	}
	if command.Action == ActionAudit {
		return service.executeAudit(ctx, command)
	}
	if command.Action == ActionBackup {
		return service.executeBackup(ctx, command)
	}
	if command.Action == ActionExport || command.Action == ActionImport {
		return service.executePortability(ctx, command)
	}
	if command.Action == ActionUpdate {
		return service.executeUpdate(ctx, command)
	}
	if command.Action == ActionProfile {
		return service.executeProfile(ctx, command)
	}
	if command.Action == ActionGoal {
		return service.executeGoal(ctx, command)
	}
	if command.Action == ActionOnboarding {
		return service.executeOnboarding(ctx, command)
	}
	if command.Action == ActionMastery {
		return service.executeMastery(ctx, command)
	}
	if command.Action == ActionSetup {
		return service.executeSetup(ctx, command)
	}
	if command.Action == ActionMistakes {
		return service.executeMistakes(ctx, command)
	}
	if command.Action == ActionSession {
		return service.executeStudySession(ctx, command)
	}
	if command.Action == ActionHistory || command.Action == ActionTime {
		return service.executeStudyHistory(ctx, command)
	}
	if command.Action == ActionReviews {
		return service.executeReviews(ctx, command)
	}
	if command.Action == ActionStreak {
		return service.executeStreak(ctx, command)
	}
	if command.Action == ActionAchievements {
		return service.executeAchievements(ctx, command)
	}
	if command.Action == ActionDashboard {
		return service.executeDashboard(ctx, command)
	}
	if command.Action != ActionInit {
		return service.bootstrap.Execute(ctx, command)
	}
	if service.workspaces == nil {
		return Result{}, fmt.Errorf("workspace service is unavailable")
	}

	root := command.Workspace
	if root == "" {
		if service.currentDirectory == nil {
			return Result{}, fmt.Errorf("current directory provider is unavailable")
		}
		var err error
		root, err = service.currentDirectory()
		if err != nil {
			return Result{}, fmt.Errorf("find current directory: %w", err)
		}
	}

	created, err := service.workspaces.Init(root, workspace.InitOptions{AllowNested: command.AllowNested})
	if err != nil {
		return Result{}, err
	}
	if err := service.generateFoundationDocuments(ctx, created); err != nil {
		return Result{}, err
	}
	if err := service.recordAudit(ctx, created.Root, audit.Event{
		Name: "workspace.initialized", Actor: audit.ActorUser, Subject: created.Root,
	}); err != nil {
		return Result{}, err
	}

	return Result{Message: fmt.Sprintf("Kelyro workspace ready at %s", created.Root)}, nil
}

func (service *Service) executeOpen(ctx context.Context, command Command) (Result, error) {
	if service.editors == nil {
		return Result{}, fmt.Errorf("editor service is unavailable")
	}
	if service.configs == nil {
		return Result{}, fmt.Errorf("configuration store is unavailable")
	}

	targetWorkspace, err := service.discoverWorkspace(command)
	if err != nil {
		return Result{}, err
	}
	settings, err := service.resolvedConfigForWorkspace(targetWorkspace.Root, command.ConfigOverrides)
	if err != nil {
		return Result{}, err
	}
	configured, _ := settings[config.KeyEditorCommand].StringField()

	var target string
	switch command.OpenTarget {
	case "":
		target, err = platform.WorkspaceLearningPath(targetWorkspace.Root)
	case "roadmap":
		target, err = platform.WorkspaceRoadmapPath(targetWorkspace.Root)
	default:
		return Result{}, fmt.Errorf("unsupported artifact %q", command.OpenTarget)
	}
	if err != nil {
		return Result{}, fmt.Errorf("resolve artifact path: %w", err)
	}

	selection, err := service.editors.Open(ctx, target, configured)
	if err != nil {
		return Result{}, err
	}
	return Result{Message: fmt.Sprintf("Opened %s with %s", filepath.Base(target), selection.Name)}, nil
}

func (service *Service) discoverWorkspace(command Command) (workspace.Workspace, error) {
	if service.workspaces == nil {
		return workspace.Workspace{}, fmt.Errorf("workspace service is unavailable")
	}
	start := command.Workspace
	if start == "" {
		if service.currentDirectory == nil {
			return workspace.Workspace{}, fmt.Errorf("current directory provider is unavailable")
		}
		var err error
		start, err = service.currentDirectory()
		if err != nil {
			return workspace.Workspace{}, fmt.Errorf("find current directory: %w", err)
		}
	}
	found, err := service.workspaces.Discover(start)
	if err != nil {
		return workspace.Workspace{}, err
	}
	return found, nil
}

func (service *Service) resolvedConfigForWorkspace(root string, overrides config.Settings) (config.Settings, error) {
	global, err := service.configs.LoadGlobal()
	if err != nil {
		return nil, err
	}
	project, err := service.configs.LoadProject(root)
	if err != nil {
		return nil, err
	}
	return config.Resolve(global, project, overrides)
}

func (service *Service) generateFoundationDocuments(ctx context.Context, target workspace.Workspace) error {
	if service.artifactStores == nil {
		return fmt.Errorf("workspace artifact store is unavailable")
	}
	documents, err := artifactmarkdown.Generate(artifactmarkdown.Model{Workspace: filepath.Base(target.Root)})
	if err != nil {
		return err
	}
	store, err := service.artifactStores.Open(ctx, target.Root)
	if err != nil {
		return fmt.Errorf("open workspace artifact store: %w", err)
	}

	var writeErr error
	for _, document := range documents {
		_, err := store.Write(ctx, artifacts.WriteRequest{
			Path:            document.Path,
			Ownership:       artifacts.SystemGeneratedHumanReadable,
			CreatedBy:       artifactmarkdown.Creator,
			Content:         document.Content,
			ExpectedVersion: document.TemplateVersion,
		})
		if err != nil {
			writeErr = fmt.Errorf("generate workspace document %s: %w", filepath.ToSlash(document.Path), err)
			break
		}
	}
	if closeErr := store.Close(); closeErr != nil {
		writeErr = errors.Join(writeErr, closeErr)
	}
	return writeErr
}

func (service *Service) executeConfig(ctx context.Context, command Command) (Result, error) {
	if service.configs == nil {
		return Result{}, fmt.Errorf("configuration store is unavailable")
	}

	switch command.ConfigOperation {
	case "show", "get":
		settings, err := service.resolvedConfig(command)
		if err != nil {
			return Result{}, err
		}
		if command.ConfigOperation == "get" {
			value, ok := settings[command.ConfigKey]
			if !ok {
				return Result{}, fmt.Errorf("unknown configuration key %q", command.ConfigKey)
			}
			return Result{Message: value.String()}, nil
		}
		message := formatSettings(settings)
		if service.secrets != nil {
			statuses, err := service.secrets.Status()
			if err != nil {
				return Result{}, err
			}
			if rendered := formatSecretStatuses(statuses); rendered != "" {
				message += "\n" + rendered
			}
		}
		return Result{Message: message}, nil
	case "path":
		return service.configPaths(command)
	case "set":
		return service.setConfig(ctx, command)
	default:
		return Result{}, fmt.Errorf("unsupported config operation %q", command.ConfigOperation)
	}
}

func (service *Service) executeSecrets(command Command) (Result, error) {
	if service.secrets == nil {
		return Result{}, fmt.Errorf("secret store is unavailable")
	}

	switch command.SecretOperation {
	case "status":
		statuses, err := service.secrets.Status()
		if err != nil {
			return Result{}, err
		}
		message := formatSecretStatuses(statuses)
		if err := service.secrets.Availability(); err != nil {
			message += "\nkeychain: unavailable (" + err.Error() + ")"
		} else {
			message += "\nkeychain: available"
		}
		return Result{Message: message}, nil
	case "set":
		if err := service.secrets.Set(command.SecretName, command.SecretValue); err != nil {
			return Result{}, errors.New(storage.Redact(err.Error(), command.SecretValue))
		}
		return Result{Message: fmt.Sprintf("Secret %q configured in the OS keychain (reference: %s)", command.SecretName, command.SecretName)}, nil
	case "delete":
		if err := service.secrets.Delete(command.SecretName); err != nil {
			return Result{}, err
		}
		return Result{Message: fmt.Sprintf("Secret %q deleted from the OS keychain; environment variables are unchanged", command.SecretName)}, nil
	default:
		return Result{}, fmt.Errorf("unsupported secrets operation %q", command.SecretOperation)
	}
}

func (service *Service) resolvedConfig(command Command) (config.Settings, error) {
	global, err := service.configs.LoadGlobal()
	if err != nil {
		return nil, err
	}
	layers := []config.Settings{global}
	if command.ConfigScope != config.ScopeGlobal {
		root, found, err := service.configWorkspace(command)
		if err != nil {
			return nil, err
		}
		if found {
			project, err := service.configs.LoadProject(root)
			if err != nil {
				return nil, err
			}
			layers = append(layers, project)
		}
	}
	layers = append(layers, command.ConfigOverrides)
	return config.Resolve(layers...)
}

func (service *Service) configPaths(command Command) (Result, error) {
	globalPath, err := service.configs.GlobalPath()
	if err != nil {
		return Result{}, err
	}
	if command.ConfigScope == config.ScopeGlobal {
		return Result{Message: globalPath}, nil
	}

	root, found, err := service.configWorkspace(command)
	if err != nil {
		return Result{}, err
	}
	if !found {
		return Result{Message: globalPath}, nil
	}
	projectPath, err := service.configs.ProjectPath(root)
	if err != nil {
		return Result{}, err
	}
	if command.ConfigScope == config.ScopeProject {
		return Result{Message: projectPath}, nil
	}
	return Result{Message: fmt.Sprintf("global: %s\nproject: %s", globalPath, projectPath)}, nil
}

func (service *Service) setConfig(ctx context.Context, command Command) (Result, error) {
	value, err := config.ParseValue(command.ConfigKey, command.ConfigValue)
	if err != nil {
		return Result{}, err
	}

	scope := command.ConfigScope
	root := ""
	if scope != config.ScopeGlobal {
		var found bool
		root, found, err = service.configWorkspace(command)
		if err != nil {
			return Result{}, err
		}
		if scope == config.ScopeProject && !found {
			return Result{}, fmt.Errorf("project configuration requires a Kelyro workspace")
		}
		if scope == "" {
			if found {
				scope = config.ScopeProject
			} else {
				scope = config.ScopeGlobal
			}
		}
	}

	var path string
	if scope == config.ScopeGlobal {
		if err := service.configs.SetGlobal(command.ConfigKey, value); err != nil {
			return Result{}, err
		}
		path, err = service.configs.GlobalPath()
	} else {
		if err := service.configs.SetProject(root, command.ConfigKey, value); err != nil {
			return Result{}, err
		}
		path, err = service.configs.ProjectPath(root)
	}
	if err != nil {
		return Result{}, err
	}
	auditRoot := root
	if auditRoot == "" {
		if discovered, found, discoverErr := service.configWorkspace(command); discoverErr == nil && found {
			auditRoot = discovered
		}
	}
	if auditRoot != "" {
		if err := service.recordAudit(ctx, auditRoot, audit.Event{
			Name: "config.changed", Actor: audit.ActorUser, Subject: command.ConfigKey,
			Metadata: map[string]string{"scope": string(scope)},
		}); err != nil {
			return Result{}, err
		}
	}
	return Result{Message: fmt.Sprintf("Set %s in %s", command.ConfigKey, path)}, nil
}

func (service *Service) configWorkspace(command Command) (string, bool, error) {
	if service.workspaces == nil {
		if command.ConfigScope == config.ScopeProject || command.Workspace != "" {
			return "", false, fmt.Errorf("workspace service is unavailable")
		}
		return "", false, nil
	}

	start := command.Workspace
	if start == "" {
		if service.currentDirectory == nil {
			return "", false, fmt.Errorf("current directory provider is unavailable")
		}
		var err error
		start, err = service.currentDirectory()
		if err != nil {
			return "", false, fmt.Errorf("find current directory: %w", err)
		}
	}

	found, err := service.workspaces.Discover(start)
	if err == nil {
		return found.Root, true, nil
	}
	if errors.Is(err, workspace.ErrNotFound) && command.Workspace == "" && command.ConfigScope != config.ScopeProject {
		return "", false, nil
	}
	return "", false, err
}

func formatSettings(settings config.Settings) string {
	var lines []string
	for _, key := range config.Keys() {
		value, ok := settings[key]
		if !ok {
			continue
		}
		rendered := value.String()
		if text, stringValue := value.StringField(); stringValue {
			rendered = strconv.Quote(text)
		}
		lines = append(lines, fmt.Sprintf("%s = %s", key, rendered))
	}
	return strings.Join(lines, "\n")
}

func formatSecretStatuses(statuses []storage.SecretStatus) string {
	lines := make([]string, 0, len(statuses))
	for _, status := range statuses {
		state := "not configured"
		if status.Configured {
			state = "configured"
		}
		label := "secret." + status.Name
		if status.Name == "<name>" {
			label = "secret"
		}
		lines = append(lines, fmt.Sprintf("%s = %s (reference: %s)", label, state, status.Reference))
	}
	return strings.Join(lines, "\n")
}

// BootstrapService provides explicit placeholders for Foundation operations
// assigned to later steps.
type BootstrapService struct{}

// Execute returns an explicit placeholder for each reserved Foundation action.
func (BootstrapService) Execute(ctx context.Context, command Command) (Result, error) {
	if err := ctx.Err(); err != nil {
		return Result{}, err
	}

	switch command.Action {
	case ActionTUI:
		return Result{Message: "Kelyro TUI bootstrap: interactive mode is not implemented yet."}, nil
	default:
		return Result{}, fmt.Errorf("unsupported Foundation action %q", command.Action)
	}
}
