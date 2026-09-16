// Package cli parses command-line input and dispatches Foundation and Student
// Core operations to application services.
package cli

import (
	"context"
	"fmt"
	"io"
	"strconv"
	"strings"
	"time"

	"github.com/mishaaac/kelyro/internal/app"
	"github.com/mishaaac/kelyro/internal/audit"
	"github.com/mishaaac/kelyro/internal/backup"
	"github.com/mishaaac/kelyro/internal/config"
	"github.com/mishaaac/kelyro/internal/curriculum"
	curriculumapp "github.com/mishaaac/kelyro/internal/curriculum/application"
	"github.com/mishaaac/kelyro/internal/doctor"
	"github.com/mishaaac/kelyro/internal/learning"
	learningapp "github.com/mishaaac/kelyro/internal/learning/application"
	"github.com/mishaaac/kelyro/internal/portability"
	"github.com/mishaaac/kelyro/internal/research"
	researchapp "github.com/mishaaac/kelyro/internal/research/application"
	"github.com/mishaaac/kelyro/internal/update"
	"github.com/mishaaac/kelyro/internal/version"
	"github.com/mishaaac/kelyro/internal/workspace"
)

// Process exit codes used by Kelyro.
const (
	ExitOK      = 0
	ExitFailure = 1
	ExitUsage   = 2
)

const help = `Kelyro is a local-first learning workspace.

Usage:
  kelyro [options]
  kelyro [options] <command>

Commands:
  help     Show this help message
  version  Show build version information
  init     Initialize a workspace
  doctor   Run Foundation diagnostics
  config   Show or update layered configuration
  secrets  Manage secure credential references
  status   Show the active goal and current learning status
  progress Show mastery, completion, study time, and consistency
  roadmap  Show the resolved curriculum roadmap and lock reasons
  today    Show today's explainable learning plan
  open     Open LEARNING.md or the roadmap in an editor
  logs     Inspect workspace diagnostic log location
  audit    Show persistent workspace audit events
  backup   Create, list, or restore workspace backups
  export   Export readable documents or a full portable workspace
  import   Validate and import a portable workspace archive
  update   Check for releases; installation remains unsupported
  profile  Show or edit the persistent learner profile
  goal     Show or manage the persistent learning goal
  mastery  Show or configure the progression mastery threshold
  setup    Show or reset the integrated learner setup
  mistakes Inspect persistent mistake memory
  session  Inspect or stop the active study session
  history  Show the learner-facing study timeline
  time     Show intentional active study time
  reviews  Show scheduled or currently due reviews
  streak   Show study consistency without affecting progress
  sources  Inspect sources, conflicts, provenance, and stale evidence
  research Plan and inspect Research runs, costs, and the offline cache
  curriculum Inspect the active compiled curriculum
  packs    Validate, install, inspect, activate, and upgrade Learning Packs
  maintenance  Run advanced local maintenance operations

Options:
  -h, --help          Show this help message
      --version       Show build version information
      --no-color      Disable colored output
      --verbose       Enable verbose diagnostic logging
      --quiet         Suppress successful command output
      --workspace PATH  Override workspace discovery
      --allow-nested  Confirm initialization inside another workspace
      --yes           Confirm restore, reset, or Learning Pack upgrade
      --full          Include allowlisted machine state in an export
      --output FILE   Set the export archive path
      --dry-run       Preview import, maintenance, or a pack upgrade
      --conflict MODE Resolve import conflicts with fail, keep, or overwrite
      --global        Use global configuration scope
      --project       Use project configuration scope
      --today         Limit study history to the local calendar day

Config commands:
  kelyro config show
  kelyro config path
  kelyro config get <key>
  kelyro config set <key> <value>

Secret commands:
  kelyro secrets status
  kelyro secrets set <name>
  kelyro secrets delete <name>

Open commands:
  kelyro open
  kelyro open roadmap

Doctor commands:
  kelyro doctor
  kelyro doctor --explain <tool>

Log commands:
  kelyro logs path

Backup commands:
  kelyro backup create
  kelyro backup list
  kelyro backup restore <id>

Portability commands:
  kelyro export [--full] [--output <file>]
  kelyro import <file> [--dry-run] [--conflict fail|keep|overwrite]

Update commands:
  kelyro update check
  kelyro update

Profile commands:
  kelyro profile [show]
  kelyro profile edit [--display-name NAME] [--experience LEVEL]
    [--language TAG] [--daily-minutes N] [--weekly-days N]
    [--learning-styles LIST] [--timezone IANA_ZONE]
  LEVEL: novice, beginner, intermediate, advanced
  LIST: comma-separated theory_first, practice, projects, reflection
  Use --display-name= or --learning-styles= to clear optional values.

Goal commands:
  kelyro goal [show]
  kelyro goal set --title TITLE --domain DOMAIN --target-outcome OUTCOME
    [--description TEXT] [--starting-level LEVEL] [--mastery-threshold SCORE]
  kelyro goal pause
  kelyro goal resume
  LEVEL: novice, beginner, intermediate, advanced
  SCORE: number from 0.50 to 0.99 (default: 0.80)

Mastery commands:
  kelyro mastery [threshold]
  kelyro mastery threshold set PERCENT
  kelyro mastery threshold set-default PERCENT
  kelyro mastery threshold reset
  PERCENT: integer from 50 to 99. set writes the workspace override.

Setup commands:
  kelyro setup status
  kelyro setup reset
  reset is available only in development/demo builds and requires confirmation.

Mistake commands:
  kelyro mistakes
  kelyro mistakes show <id>

Study session commands:
  kelyro session status
  kelyro session stop

Study history commands:
  kelyro history
  kelyro history --today
  kelyro time

Progress artifact command:
  kelyro progress export

Review commands:
  kelyro reviews
  kelyro reviews due

Streak command:
  kelyro streak

Source registry commands:
  kelyro sources
  kelyro sources list
  kelyro sources show <source-id>
  kelyro sources registry list
  kelyro sources registry show <id>
  kelyro sources trace <claim-id>
  kelyro sources stale
  kelyro sources conflicts

Research commands:
  kelyro research topic <topic>
  kelyro research status <run-id>
  kelyro research show <run-id>
  kelyro research stats
  kelyro research update-scan
  kelyro research cache status
  kelyro research cache clear

Curriculum commands:
  kelyro curriculum compile
  kelyro curriculum validate
  kelyro curriculum coverage
  kelyro curriculum gaps
  kelyro curriculum audit
  kelyro curriculum build-info

Learning Pack commands:
  kelyro packs validate <path>
  kelyro packs install <path>
  kelyro packs list
  kelyro packs show <id>
  kelyro packs activate <id>@<version>
  kelyro packs upgrade <id> [--dry-run]
  kelyro packs catalog
  kelyro packs search <query>

Advanced maintenance command:
  kelyro maintenance recalculate [--dry-run]
`

var actions = map[string]app.Action{
	"init":        app.ActionInit,
	"doctor":      app.ActionDoctor,
	"config":      app.ActionConfig,
	"secrets":     app.ActionSecrets,
	"status":      app.ActionStatus,
	"progress":    app.ActionProgress,
	"roadmap":     app.ActionRoadmap,
	"today":       app.ActionToday,
	"open":        app.ActionOpen,
	"logs":        app.ActionLogs,
	"audit":       app.ActionAudit,
	"backup":      app.ActionBackup,
	"export":      app.ActionExport,
	"import":      app.ActionImport,
	"update":      app.ActionUpdate,
	"profile":     app.ActionProfile,
	"goal":        app.ActionGoal,
	"mastery":     app.ActionMastery,
	"setup":       app.ActionSetup,
	"mistakes":    app.ActionMistakes,
	"session":     app.ActionSession,
	"history":     app.ActionHistory,
	"time":        app.ActionTime,
	"reviews":     app.ActionReviews,
	"streak":      app.ActionStreak,
	"sources":     app.ActionSources,
	"research":    app.ActionResearch,
	"maintenance": app.ActionMaintenance,
}

// Runner owns CLI parsing and rendering while delegating operations to an
// application service.
type Runner struct {
	service           app.FoundationService
	stdout            io.Writer
	stderr            io.Writer
	secrets           SecretReader
	interactive       InteractiveRunner
	confirmer         Confirmer
	packValidator     curriculumapp.PackValidationService
	packManager       curriculumapp.PackInstallService
	packActivator     curriculumapp.PackActivationService
	packCatalog       curriculumapp.PackCatalogService
	packUpgrade       curriculumapp.PackUpgradeService
	curriculum        curriculumapp.CurriculumInspectionService
	environmentDoctor curriculumapp.WorkspaceEnvironmentDoctorService
	packWorkspaces    workspace.Service
	currentDirectory  func() (string, error)
}

// Confirmer obtains explicit consent before destructive operations.
type Confirmer interface {
	Confirm(prompt string) (bool, error)
}

// InteractiveRunner owns the full-screen terminal lifecycle for the default
// command without coupling CLI parsing to Bubble Tea.
type InteractiveRunner interface {
	Run(ctx context.Context, command app.Command) error
}

// NewRunner creates a testable CLI runner with explicit dependencies.
func NewRunner(service app.FoundationService, stdout, stderr io.Writer) Runner {
	return Runner{service: service, stdout: stdout, stderr: stderr}
}

// WithSecretReader attaches the terminal adapter used to collect secret values
// without placing them in process arguments or normal command output.
func (r Runner) WithSecretReader(reader SecretReader) Runner {
	r.secrets = reader
	return r
}

// WithInteractive attaches the full-screen presentation adapter used when no
// explicit CLI command is provided.
func (r Runner) WithInteractive(interactive InteractiveRunner) Runner {
	r.interactive = interactive
	return r
}

// WithConfirmer attaches interactive confirmation for destructive restore.
func (r Runner) WithConfirmer(confirmer Confirmer) Runner {
	r.confirmer = confirmer
	return r
}

// WithPackValidator attaches the read-only portable pack validator.
func (r Runner) WithPackValidator(validator curriculumapp.PackValidationService) Runner {
	r.packValidator = validator
	return r
}

// WithPackManager attaches immutable global installation and workspace-scoped
// activation. Workspace discovery remains a presentation concern.
func (r Runner) WithPackManager(manager curriculumapp.PackInstallService, workspaces workspace.Service, currentDirectory func() (string, error)) Runner {
	r.packManager = manager
	r.packActivator = manager
	r.packWorkspaces = workspaces
	r.currentDirectory = currentDirectory
	return r
}

// WithPackActivator replaces direct pointer activation with the application
// coordinator that performs the Student Core hand-off.
func (r Runner) WithPackActivator(activator curriculumapp.PackActivationService) Runner {
	r.packActivator = activator
	return r
}

func (r Runner) WithPackCatalog(catalog curriculumapp.PackCatalogService) Runner {
	r.packCatalog = catalog
	return r
}

// WithPackUpgrade attaches the student-safe pack upgrade planner/executor.
func (r Runner) WithPackUpgrade(upgrade curriculumapp.PackUpgradeService) Runner {
	r.packUpgrade = upgrade
	return r
}

// WithCurriculumInspector attaches read-only inspection of the immutable
// compiled artifact contained by an active Learning Pack.
func (r Runner) WithCurriculumInspector(inspector curriculumapp.CurriculumInspectionService) Runner {
	r.curriculum = inspector
	return r
}

func (r Runner) WithEnvironmentDoctor(service curriculumapp.WorkspaceEnvironmentDoctorService) Runner {
	r.environmentDoctor = service
	return r
}

// Run parses args, renders immediate CLI output, or dispatches one application
// action. It returns a process exit code and does not construct native process
// commands itself.
func (r Runner) Run(ctx context.Context, args []string) int {
	invocation, err := parse(args)
	if err != nil {
		return r.usageError("%v", err)
	}

	if invocation.help {
		fmt.Fprint(r.stdout, help)
		return ExitOK
	}
	if invocation.version {
		fmt.Fprintf(r.stdout, "kelyro %s\n", version.Current())
		return ExitOK
	}
	if invocation.command == "packs" {
		return r.runPacks(ctx, invocation)
	}
	if invocation.command == "curriculum" {
		return r.runCurriculum(ctx, invocation)
	}

	action := app.ActionTUI
	commandName := "tui"
	if invocation.command != "" {
		var found bool
		action, found = actions[invocation.command]
		if !found {
			return r.usageError("unknown command %q", invocation.command)
		}
		commandName = invocation.command
	}

	if r.service == nil {
		fmt.Fprintln(r.stderr, "kelyro: application service is unavailable")
		return ExitFailure
	}

	command := app.Command{
		Action:                  action,
		Workspace:               invocation.workspace,
		AllowNested:             invocation.allowNested,
		ConfigScope:             invocation.configScope,
		OpenTarget:              invocation.openTarget,
		DoctorExplain:           invocation.doctorExplain,
		LogOperation:            invocation.logOperation,
		BackupOperation:         invocation.backupOperation,
		BackupID:                invocation.backupID,
		ExportMode:              invocation.exportMode,
		ExportOutput:            invocation.exportOutput,
		ImportArchive:           invocation.importArchive,
		ImportDryRun:            invocation.importDryRun,
		ImportConflicts:         invocation.importConflicts,
		UpdateOperation:         invocation.updateOperation,
		ProfileOperation:        invocation.profileOperation,
		ProfileChanges:          invocation.profileChanges,
		GoalOperation:           invocation.goalOperation,
		GoalInput:               invocation.goalInput,
		MasteryOperation:        invocation.masteryOperation,
		MasteryThreshold:        invocation.masteryThreshold,
		SetupOperation:          invocation.setupOperation,
		MistakeOperation:        invocation.mistakeOperation,
		MistakeID:               invocation.mistakeID,
		SessionOperation:        invocation.sessionOperation,
		HistoryToday:            invocation.historyToday,
		ProgressOperation:       invocation.progressOperation,
		MaintenanceOperation:    invocation.maintenanceOperation,
		MaintenanceDryRun:       invocation.maintenanceDryRun,
		ReviewsDue:              invocation.reviewsDue,
		SourceRegistryOperation: invocation.sourceRegistryOperation,
		ResearchCacheOperation:  invocation.researchCacheOperation,
		ResearchOperation:       invocation.researchOperation,
		ResearchTopic:           invocation.researchTopic,
		ResearchRunID:           invocation.researchRunID,
		SourceID:                invocation.sourceID,
		SourceRegistryID:        invocation.sourceRegistryID,
		ProvenanceClaimID:       invocation.provenanceClaimID,
		Verbose:                 invocation.verbose,
	}
	if invocation.noColor {
		command.ConfigOverrides = config.Settings{config.KeyUIColor: config.StringValue("never")}
	}
	if action == app.ActionDoctor && invocation.doctorExplain == "" && r.environmentDoctor != nil {
		root, resolveErr := r.resolvePackWorkspace(invocation.workspace)
		if resolveErr != nil {
			fmt.Fprintf(r.stderr, "kelyro doctor: %v\n", resolveErr)
			return ExitFailure
		}
		plan, planErr := r.environmentDoctor.PlanForWorkspace(ctx, root)
		if planErr != nil {
			fmt.Fprintf(r.stderr, "kelyro doctor: %v\n", planErr)
			return ExitFailure
		}
		command.DoctorEnvironmentPlan = plan
	}
	if action == app.ActionTUI && r.interactive != nil {
		if err := r.interactive.Run(ctx, command); err != nil {
			fmt.Fprintf(r.stderr, "kelyro tui: %v\n", err)
			return ExitFailure
		}
		return ExitOK
	}
	if action == app.ActionConfig {
		command.ConfigOperation = invocation.configOperation
		command.ConfigKey = invocation.configKey
		command.ConfigValue = invocation.configValue
	}
	if action == app.ActionSecrets {
		command.SecretOperation = invocation.secretOperation
		command.SecretName = invocation.secretName
		if command.SecretOperation == "set" {
			if r.secrets == nil {
				fmt.Fprintln(r.stderr, "kelyro secrets: secure terminal input is unavailable")
				return ExitFailure
			}
			command.SecretValue, err = r.secrets.ReadSecret("Secret value: ")
			if err != nil {
				fmt.Fprintf(r.stderr, "kelyro secrets: read secret: %v\n", err)
				return ExitFailure
			}
		}
	}
	if action == app.ActionBackup && command.BackupOperation == "restore" {
		command.BackupConfirmed = invocation.yes
		if !command.BackupConfirmed {
			if r.confirmer == nil {
				fmt.Fprintln(r.stderr, "kelyro backup: restore confirmation input is unavailable; use --yes to confirm")
				return ExitFailure
			}
			confirmed, confirmErr := r.confirmer.Confirm(fmt.Sprintf("Restore backup %s? This replaces managed workspace state [y/N]: ", command.BackupID))
			if confirmErr != nil {
				fmt.Fprintf(r.stderr, "kelyro backup: confirm restore: %v\n", confirmErr)
				return ExitFailure
			}
			if !confirmed {
				if !invocation.quiet {
					fmt.Fprintln(r.stdout, "Restore canceled.")
				}
				return ExitOK
			}
			command.BackupConfirmed = true
		}
	}
	if action == app.ActionSetup && command.SetupOperation == "reset" {
		if !invocation.yes {
			if r.confirmer == nil {
				fmt.Fprintln(r.stderr, "kelyro setup: reset confirmation input is unavailable; use --yes to confirm")
				return ExitFailure
			}
			confirmed, confirmErr := r.confirmer.Confirm("Reset learner setup? Profile, goal history, and Foundation data are preserved [y/N]: ")
			if confirmErr != nil {
				fmt.Fprintf(r.stderr, "kelyro setup: confirm reset: %v\n", confirmErr)
				return ExitFailure
			}
			if !confirmed {
				if !invocation.quiet {
					fmt.Fprintln(r.stdout, "Setup reset canceled.")
				}
				return ExitOK
			}
		}
	}

	result, err := r.service.Execute(ctx, command)
	if err != nil {
		fmt.Fprintf(r.stderr, "kelyro %s: %v\n", commandName, err)
		return ExitFailure
	}
	if result.Guidance != nil && !invocation.quiet {
		fmt.Fprintln(r.stdout, formatGuidance(*result.Guidance))
	} else if result.Diagnostics != nil && (!invocation.quiet || result.Failed) {
		fmt.Fprintln(r.stdout, formatDiagnostics(*result.Diagnostics))
	} else if result.Audit != nil && !invocation.quiet {
		fmt.Fprintln(r.stdout, formatAudit(result.Audit))
	} else if result.Backups != nil && !invocation.quiet {
		fmt.Fprintln(r.stdout, formatBackups(result.Backups))
	} else if result.Portability != nil && !invocation.quiet {
		fmt.Fprintln(r.stdout, formatPortability(*result.Portability))
	} else if result.Update != nil && !invocation.quiet {
		fmt.Fprintln(r.stdout, formatUpdate(*result.Update))
	} else if result.Dashboard != nil && !invocation.quiet {
		fmt.Fprintln(r.stdout, formatDashboard(commandName, *result.Dashboard))
	} else if result.Maintenance != nil && !invocation.quiet {
		fmt.Fprintln(r.stdout, formatMaintenance(*result.Maintenance))
	} else if result.Profile != nil && !invocation.quiet {
		fmt.Fprintln(r.stdout, formatProfile(*result.Profile))
	} else if result.Goal != nil && !invocation.quiet {
		fmt.Fprintln(r.stdout, formatGoal(*result.Goal))
	} else if result.Goals != nil && !invocation.quiet {
		fmt.Fprintln(r.stdout, formatGoals(result.Goals))
	} else if result.Mastery != nil && !invocation.quiet {
		fmt.Fprintln(r.stdout, formatMasteryThreshold(*result.Mastery))
	} else if result.Setup != nil && !invocation.quiet {
		fmt.Fprintln(r.stdout, formatLearnerSetup(*result.Setup))
	} else if result.Mistake != nil && !invocation.quiet {
		fmt.Fprintln(r.stdout, formatMistake(*result.Mistake))
	} else if result.Mistakes != nil && !invocation.quiet {
		fmt.Fprintln(r.stdout, formatMistakes(result.Mistakes))
	} else if result.StudySession != nil && !invocation.quiet {
		fmt.Fprintln(r.stdout, formatStudySession(*result.StudySession))
	} else if result.History != nil && !invocation.quiet {
		fmt.Fprintln(r.stdout, formatStudyHistory(*result.History))
	} else if result.StudyTime != nil && !invocation.quiet {
		fmt.Fprintln(r.stdout, formatStudyTime(*result.StudyTime))
	} else if result.Reviews != nil && !invocation.quiet {
		fmt.Fprintln(r.stdout, formatReviews(*result.Reviews))
	} else if result.Streak != nil && !invocation.quiet {
		fmt.Fprintln(r.stdout, formatStreak(*result.Streak))
	} else if result.StaleSources != nil && !invocation.quiet {
		fmt.Fprintln(r.stdout, formatStaleSources(result.StaleSources))
	} else if result.ResearchCacheStatus != nil && !invocation.quiet {
		fmt.Fprintln(r.stdout, formatResearchCacheStatus(*result.ResearchCacheStatus))
	} else if result.ResearchCacheCleared != nil && !invocation.quiet {
		fmt.Fprintln(r.stdout, formatResearchCacheClear(*result.ResearchCacheCleared))
	} else if result.ResearchCostStats != nil && !invocation.quiet {
		fmt.Fprintln(r.stdout, formatResearchCostStats(*result.ResearchCostStats))
	} else if result.UpdateScan != nil && !invocation.quiet {
		fmt.Fprintln(r.stdout, formatUpdateScan(*result.UpdateScan))
	} else if result.ResearchView != nil && !invocation.quiet {
		fmt.Fprintln(r.stdout, formatResearchView(*result.ResearchView))
	} else if result.ResearchAuditView != nil && !invocation.quiet {
		fmt.Fprintln(r.stdout, formatResearchAuditView(*result.ResearchAuditView))
	} else if result.Source != nil && !invocation.quiet {
		fmt.Fprintln(r.stdout, formatSource(*result.Source))
	} else if result.Sources != nil && !invocation.quiet {
		fmt.Fprintln(r.stdout, formatSources(result.Sources))
	} else if result.SourceConflicts != nil && !invocation.quiet {
		fmt.Fprintln(r.stdout, formatSourceConflicts(result.SourceConflicts))
	} else if result.SourceRegistryEntry != nil && !invocation.quiet {
		fmt.Fprintln(r.stdout, formatSourceRegistryEntry(*result.SourceRegistryEntry))
	} else if result.SourceRegistryEntries != nil && !invocation.quiet {
		fmt.Fprintln(r.stdout, formatSourceRegistryEntries(result.SourceRegistryEntries))
	} else if result.ProvenanceGraph != nil && !invocation.quiet {
		explanation, explainErr := result.ProvenanceGraph.Explain()
		if explainErr != nil {
			fmt.Fprintf(r.stderr, "kelyro %s: explain provenance: %v\n", commandName, explainErr)
			return ExitFailure
		}
		fmt.Fprintln(r.stdout, explanation)
	} else if !invocation.quiet && result.Message != "" {
		fmt.Fprintln(r.stdout, result.Message)
	}
	if result.Failed {
		return ExitFailure
	}

	return ExitOK
}

func (r Runner) runCurriculum(ctx context.Context, invocation invocation) int {
	if r.packManager == nil {
		fmt.Fprintln(r.stderr, "kelyro curriculum: pack manager is unavailable")
		return ExitFailure
	}
	if r.curriculum == nil && invocation.curriculumOperation != "build-info" {
		fmt.Fprintln(r.stderr, "kelyro curriculum: curriculum inspector is unavailable")
		return ExitFailure
	}
	root, err := r.resolvePackWorkspace(invocation.workspace)
	if err != nil {
		fmt.Fprintf(r.stderr, "kelyro curriculum %s: %v\n", invocation.curriculumOperation, err)
		return ExitFailure
	}
	installed, err := r.packManager.Active(ctx, root)
	if err != nil {
		fmt.Fprintf(r.stderr, "kelyro curriculum %s: %v\n", invocation.curriculumOperation, err)
		return ExitFailure
	}
	if invocation.curriculumOperation != "build-info" {
		inspection, inspectErr := r.curriculum.Inspect(ctx, installed.Pack)
		if inspectErr != nil {
			fmt.Fprintf(r.stderr, "kelyro curriculum %s: %v\n", invocation.curriculumOperation, inspectErr)
			return ExitFailure
		}
		if !invocation.quiet {
			fmt.Fprintln(r.stdout, formatCurriculumInspection(invocation.curriculumOperation, inspection))
		}
		return ExitOK
	}
	if installed.Pack.BuildInfo == nil {
		fmt.Fprintln(r.stderr, "kelyro curriculum build-info: active pack has no reproducibility metadata")
		return ExitFailure
	}
	if !invocation.quiet {
		fmt.Fprintln(r.stdout, formatCurriculumBuildInfo(installed.Pack, *installed.Pack.BuildInfo))
	}
	return ExitOK
}

func formatCurriculumInspection(operation string, inspection curriculumapp.CurriculumInspection) string {
	definition := inspection.Pack.Curriculum
	identity := []string{
		fmt.Sprintf("Pack: %s@%s", inspection.Pack.Manifest.ID, inspection.Pack.Manifest.Version.String()),
		fmt.Sprintf("Curriculum: %s@%s", definition.ID, definition.Version.String()),
	}
	switch operation {
	case "compile":
		lines := append([]string{"Compiled curriculum artifact"}, identity...)
		lines = append(lines,
			fmt.Sprintf("Structure: %d phases, %d modules, %d lessons, %d topics, %d concepts", len(definition.Phases), len(definition.Modules), len(definition.Lessons), len(definition.Topics), len(definition.Concepts)),
			fmt.Sprintf("Compiler passes retained: %d", len(inspection.CompilationPasses)),
			"Status: immutable artifact verified (no source policy was inferred)",
		)
		return strings.Join(lines, "\n")
	case "validate":
		return strings.Join(append([]string{"Curriculum validation", "Status: valid"}, identity...), "\n")
	case "coverage":
		lines := append([]string{"Curriculum coverage"}, identity...)
		for _, summary := range inspection.Coverage {
			lines = append(lines, fmt.Sprintf("- %s: %d requirements, %d retained gaps", summary.Dimension, summary.RequirementCount, summary.GapCount))
		}
		if report := inspection.Pack.EvidenceReport; report != nil {
			lines = append(lines, fmt.Sprintf("Primary-source claim coverage: %d/%d (%.0f%%)", report.PrimarySourceCoverage.PrimaryClaims, report.PrimarySourceCoverage.ReferencedClaims, report.PrimarySourceCoverage.Ratio*100))
		}
		return strings.Join(lines, "\n")
	case "gaps":
		lines := append([]string{"Curriculum gaps"}, identity...)
		if len(inspection.Gaps) == 0 {
			return strings.Join(append(lines, "No retained compiler gaps."), "\n")
		}
		for _, gap := range inspection.Gaps {
			lines = append(lines, fmt.Sprintf("- [%s/%s] %s: %s", gap.Severity, gap.Kind, gap.ID, gap.Reason))
		}
		return strings.Join(lines, "\n")
	case "audit":
		lines := append([]string{"Curriculum audit"}, identity...)
		for _, audit := range inspection.Audits {
			status := "passed"
			if !audit.Passed {
				status = "failed"
			}
			lines = append(lines, fmt.Sprintf("- %s (%s): %s", audit.Name, audit.Version, status))
			for _, reason := range audit.Reasons {
				lines = append(lines, "  "+reason)
			}
		}
		if len(inspection.EvidenceLinks) > 0 {
			lines = append(lines, "Sources/evidence")
			for _, citation := range inspection.EvidenceLinks {
				lines = append(lines, fmt.Sprintf("- %s: %s", citation.Title, citation.URL))
			}
		}
		return strings.Join(lines, "\n")
	default:
		return ""
	}
}

func formatCurriculumBuildInfo(pack curriculum.LearningPack, metadata curriculum.ReproducibilityMetadata) string {
	lines := []string{
		"Curriculum build information",
		fmt.Sprintf("Pack: %s@%s", pack.Manifest.ID, pack.Manifest.Version.String()),
		fmt.Sprintf("Curriculum: %s@%s", pack.Curriculum.ID, pack.Curriculum.Version.String()),
		"Built: " + metadata.BuiltAt.Time().Format(time.RFC3339),
		"Compiler: " + metadata.CompilerVersion,
		"Pack schema: " + metadata.PackSchemaVersion,
		"Source policy: " + string(metadata.CompilationConfig.SourcePolicy),
		"Input hash: " + metadata.InputHash,
		"Output hash: " + metadata.OutputHash,
		fmt.Sprintf("Source bundles (%d)", len(metadata.SourceBundles)),
	}
	for _, bundle := range metadata.SourceBundles {
		lines = append(lines, fmt.Sprintf("- %s  %s  %s", bundle.ID, bundle.ContentHash, bundle.AlgorithmVersion))
	}
	lines = append(lines, fmt.Sprintf("Compiler passes (%d)", len(metadata.Passes)))
	for _, pass := range metadata.Passes {
		lines = append(lines, fmt.Sprintf("- %s: %s", pass.Name, pass.Version))
	}
	return strings.Join(lines, "\n")
}

func (r Runner) runPacks(ctx context.Context, invocation invocation) int {
	switch invocation.packOperation {
	case "install", "list", "show", "activate":
		if r.packManager == nil {
			fmt.Fprintln(r.stderr, "kelyro packs: pack manager is unavailable")
			return ExitFailure
		}
		if invocation.packOperation == "activate" && r.packActivator == nil {
			fmt.Fprintln(r.stderr, "kelyro packs: pack activation service is unavailable")
			return ExitFailure
		}
	}
	if invocation.packOperation == "upgrade" {
		if r.packUpgrade == nil {
			fmt.Fprintln(r.stderr, "kelyro packs: pack upgrade service is unavailable")
			return ExitFailure
		}
		root, err := r.resolvePackWorkspace(invocation.workspace)
		if err != nil {
			fmt.Fprintf(r.stderr, "kelyro packs upgrade: %v\n", err)
			return ExitFailure
		}
		confirmed := invocation.yes
		if !invocation.packDryRun && !confirmed {
			if r.confirmer == nil {
				fmt.Fprintln(r.stderr, "kelyro packs upgrade: confirmation input is unavailable; use --yes to confirm")
				return ExitFailure
			}
			confirmed, err = r.confirmer.Confirm(fmt.Sprintf("Upgrade Learning Pack %s? A backup will be created and learner instances migrated [y/N]: ", invocation.packID))
			if err != nil {
				fmt.Fprintf(r.stderr, "kelyro packs upgrade: confirm: %v\n", err)
				return ExitFailure
			}
			if !confirmed {
				if !invocation.quiet {
					fmt.Fprintln(r.stdout, "Learning Pack upgrade canceled.")
				}
				return ExitOK
			}
		}
		result, err := r.packUpgrade.Upgrade(ctx, curriculumapp.PackUpgradeRequest{
			WorkspaceRoot: root, PackID: invocation.packID, TargetVersion: invocation.packVersion,
			DryRun: invocation.packDryRun, Confirmed: confirmed,
		})
		if err != nil {
			fmt.Fprintf(r.stderr, "kelyro packs upgrade: %v\n", err)
			return ExitFailure
		}
		if !invocation.quiet {
			fmt.Fprintln(r.stdout, formatPackUpgrade(result))
		}
		return ExitOK
	}
	if invocation.packOperation == "catalog" || invocation.packOperation == "search" {
		if r.packCatalog == nil {
			fmt.Fprintln(r.stderr, "kelyro packs: pack catalog is unavailable")
			return ExitFailure
		}
		var view curriculumapp.PackCatalogView
		var err error
		if invocation.packOperation == "search" {
			view, err = r.packCatalog.Search(ctx, invocation.packQuery)
		} else {
			view, err = r.packCatalog.Catalog(ctx)
		}
		if err != nil {
			fmt.Fprintf(r.stderr, "kelyro packs %s: %v\n", invocation.packOperation, err)
			return ExitFailure
		}
		if !invocation.quiet {
			title := "Learning Pack Catalog"
			if invocation.packOperation == "search" {
				title = "Learning Pack search: " + invocation.packQuery
			}
			fmt.Fprintln(r.stdout, formatPackCatalog(title, view))
		}
		return ExitOK
	}
	switch invocation.packOperation {
	case "install":
		result, err := r.packManager.Install(ctx, curriculumapp.PackInstallRequest{Source: curriculumapp.PackSource{Path: invocation.packPath}})
		if err != nil {
			fmt.Fprintf(r.stderr, "kelyro packs install: %v\n", err)
			return ExitFailure
		}
		if !invocation.quiet {
			state := "already installed"
			if result.Installed {
				state = "installed"
			}
			fmt.Fprintf(r.stdout, "Learning Pack %s\nPack: %s@%s\nChecksum: %s\n", state, result.Pack.Manifest.ID, result.Pack.Manifest.Version.String(), result.ContentHash)
		}
		return ExitOK
	case "list":
		packs, err := r.packManager.List(ctx)
		if err != nil {
			fmt.Fprintf(r.stderr, "kelyro packs list: %v\n", err)
			return ExitFailure
		}
		if !invocation.quiet {
			fmt.Fprintln(r.stdout, formatInstalledPacks("Installed Learning Packs", packs))
		}
		return ExitOK
	case "show":
		packs, err := r.packManager.Find(ctx, invocation.packID)
		if err != nil {
			fmt.Fprintf(r.stderr, "kelyro packs show: %v\n", err)
			return ExitFailure
		}
		if !invocation.quiet {
			fmt.Fprintln(r.stdout, formatInstalledPacks("Learning Pack "+invocation.packID.String(), packs))
		}
		return ExitOK
	case "activate":
		root, err := r.resolvePackWorkspace(invocation.workspace)
		if err != nil {
			fmt.Fprintf(r.stderr, "kelyro packs activate: %v\n", err)
			return ExitFailure
		}
		activation, err := r.packActivator.Activate(ctx, curriculumapp.PackActivateRequest{WorkspaceRoot: root, PackID: invocation.packID, Version: invocation.packVersion})
		if err != nil {
			fmt.Fprintf(r.stderr, "kelyro packs activate: %v\n", err)
			return ExitFailure
		}
		if !invocation.quiet {
			fmt.Fprintf(r.stdout, "Learning Pack activated\nPack: %s@%s\nWorkspace: %s\n", activation.PackID, activation.Version.String(), root)
		}
		return ExitOK
	}

	if r.packValidator == nil {
		fmt.Fprintln(r.stderr, "kelyro packs: pack validator is unavailable")
		return ExitFailure
	}
	result, err := r.packValidator.Validate(ctx, curriculumapp.PackSource{Path: invocation.packPath})
	if err != nil {
		fmt.Fprintf(r.stderr, "kelyro packs: %v\n", err)
		return ExitFailure
	}
	if len(result.Errors) > 0 {
		fmt.Fprintln(r.stderr, "Learning Pack validation\nStatus: invalid")
		for _, issue := range result.Errors {
			fmt.Fprintf(r.stderr, "- [%s] %s: %s\n", issue.Code, issue.Path, issue.Message)
		}
		return ExitFailure
	}
	if invocation.quiet {
		return ExitOK
	}
	lines := []string{"Learning Pack validation", "Status: valid"}
	if result.Pack != nil {
		lines = append(lines,
			"Pack: "+result.Pack.Manifest.ID.String()+"@"+result.Pack.Manifest.Version.String(),
			"Curriculum: "+result.Pack.Curriculum.ID.String()+"@"+result.Pack.Curriculum.Version.String(),
		)
	}
	for _, warning := range result.Warnings {
		lines = append(lines, fmt.Sprintf("Warning [%s] %s: %s", warning.Code, warning.Path, warning.Message))
	}
	fmt.Fprintln(r.stdout, strings.Join(lines, "\n"))
	return ExitOK
}

func (r Runner) resolvePackWorkspace(override string) (string, error) {
	if r.packWorkspaces == nil {
		return "", fmt.Errorf("workspace discovery is unavailable")
	}
	start := override
	if start == "" {
		if r.currentDirectory == nil {
			return "", fmt.Errorf("current directory is unavailable")
		}
		var err error
		start, err = r.currentDirectory()
		if err != nil {
			return "", err
		}
	}
	found, err := r.packWorkspaces.Discover(start)
	if err != nil {
		return "", err
	}
	return found.Root, nil
}

func formatInstalledPacks(title string, packs []curriculumapp.InstalledPack) string {
	lines := []string{title}
	if len(packs) == 0 {
		return strings.Join(append(lines, "No packs installed."), "\n")
	}
	for _, installed := range packs {
		manifest := installed.Pack.Manifest
		lines = append(lines,
			fmt.Sprintf("- %s@%s — %s", manifest.ID, manifest.Version.String(), manifest.Name),
			"  Status: "+string(manifest.Status),
			"  Curriculum: "+manifest.CurriculumID.String()+"@"+installed.Pack.Curriculum.Version.String(),
			"  Checksum: "+installed.ContentHash,
		)
	}
	return strings.Join(lines, "\n")
}

func formatPackCatalog(title string, view curriculumapp.PackCatalogView) string {
	lines := []string{title}
	if view.Offline {
		lines = append(lines, "Mode: offline cache")
	}
	if view.SourceWarning != "" {
		lines = append(lines, "Source warning: "+view.SourceWarning)
	}
	if len(view.Snapshot.Entries) == 0 {
		return strings.Join(append(lines, "No catalog entries found."), "\n")
	}
	for _, entry := range view.Snapshot.Entries {
		lines = append(lines,
			fmt.Sprintf("- %s — %s", entry.PackID, entry.Name),
			"  "+entry.Description,
			"  Maintainer: "+entry.Maintainer,
			fmt.Sprintf("  Source: %s (%s)", entry.Source.Name, entry.Source.Trust),
		)
		for _, version := range entry.Versions {
			lines = append(lines, fmt.Sprintf("  Version: %s [%s, %s]", version.Version.String(), version.Status, version.Compatibility))
		}
	}
	return strings.Join(lines, "\n")
}

func formatPackUpgrade(result curriculumapp.PackUpgradeResult) string {
	plan := result.MigrationPlan
	counts := make(map[curriculum.CurriculumMigrationActionKind]int)
	for _, action := range plan.Actions {
		counts[action.Kind]++
	}
	status := "dry-run"
	if result.Applied {
		status = "applied"
	}
	lines := []string{
		"Learning Pack upgrade", "Status: " + status,
		fmt.Sprintf("Pack: %s %s -> %s", result.Current.Manifest.ID, result.Current.Manifest.Version.String(), result.Candidate.Manifest.Version.String()),
		fmt.Sprintf("Migration plan: %s", plan.ID),
		fmt.Sprintf("Preserved stable concepts: %d", counts[curriculum.MigrationPreserveState]),
		fmt.Sprintf("New unknown concepts: %d", counts[curriculum.MigrationInitializeUnknown]),
		fmt.Sprintf("Historical removals: %d", counts[curriculum.MigrationPreserveHistorical]),
		fmt.Sprintf("Split/merge mappings without mastery transfer: %d", counts[curriculum.MigrationSplitNoTransfer]+counts[curriculum.MigrationMergeNoTransfer]),
		fmt.Sprintf("Recalculate unlock eligibility: %t", plan.RecalculateUnlockEligibility),
		fmt.Sprintf("Requires student review: %t", plan.RequiresStudentReview),
	}
	if result.BackupID != "" {
		lines = append(lines, "Backup: "+result.BackupID)
	}
	if !result.Applied {
		lines = append(lines, "No pack activation or Student State was written.")
	}
	return strings.Join(lines, "\n")
}

func formatLearnerSetup(view learningapp.LearnerSetupView) string {
	lines := []string{"Learner setup", "Status: " + string(view.Setup.Status)}
	if view.Setup.SetupCompletedAt != nil {
		lines = append(lines, "Completed: "+view.Setup.SetupCompletedAt.Time().Format(time.RFC3339))
	}
	if view.Instance != nil {
		lines = append(lines, "Curriculum: "+view.Instance.Curriculum.ID.String()+"@"+view.Instance.Curriculum.Version, "Source: "+string(view.Instance.Source))
	}
	if view.Setup.DiagnosticOptIn {
		lines = append(lines, "Diagnostic: opted in")
	} else {
		lines = append(lines, "Diagnostic: not selected")
	}
	return strings.Join(lines, "\n")
}

func formatDashboard(command string, dashboard learningapp.ProgressDashboard) string {
	switch command {
	case "progress":
		return formatProgressDashboard(dashboard)
	case "roadmap":
		return formatRoadmapDashboard(dashboard)
	case "today":
		return formatTodayDashboard(dashboard)
	default:
		return formatStatusDashboard(dashboard)
	}
}

func formatMaintenance(impact learningapp.RecalculationImpact) string {
	heading := "Learning-state recalculation"
	verb := "Changed"
	if impact.DryRun {
		heading += " — dry run"
		verb = "Would change"
	}
	lines := []string{
		heading,
		"Previous mastery: " + formatAlgorithmVersions(impact.Previous.Mastery),
		"Target mastery: " + formatAlgorithmVersions(impact.Target.Mastery),
		"Previous retention: " + formatAlgorithmVersions(impact.Previous.Retention),
		"Target retention: " + formatAlgorithmVersions(impact.Target.Retention),
		"Previous daily plan: " + formatAlgorithmVersions(impact.Previous.DailyPlan),
		"Target daily plan: " + formatAlgorithmVersions(impact.Target.DailyPlan),
		fmt.Sprintf("Evidence records read: %d (unchanged)", impact.EvidenceRecords),
		fmt.Sprintf("Concepts scanned: %d", impact.ConceptsScanned),
		fmt.Sprintf("%s concept states: %d", verb, impact.ConceptStatesChanged),
		fmt.Sprintf("%s retention states: %d", verb, impact.RetentionStatesChanged),
		fmt.Sprintf("%s review schedules: %d", verb, impact.ReviewSchedulesChanged),
		fmt.Sprintf("%s review items: %d", verb, impact.ReviewItemsChanged),
		fmt.Sprintf("%s daily plans: %d", verb, impact.DailyPlansChanged),
	}
	if impact.BackupID != "" {
		lines = append(lines, "Backup: "+impact.BackupID)
	}
	if impact.DryRun {
		lines = append(lines, "No learning state was written and no backup was created.")
	}
	return strings.Join(lines, "\n")
}

func formatAlgorithmVersions(versions []string) string {
	if len(versions) == 0 {
		return "none"
	}
	return strings.Join(versions, ", ")
}

func formatStatusDashboard(dashboard learningapp.ProgressDashboard) string {
	if dashboard.Goal == nil {
		return strings.Join([]string{
			"Learning status",
			"Goal: no active learning goal",
			"Current: unavailable",
			"Run `kelyro setup status` to inspect setup, or `kelyro goal` to inspect learning goals.",
		}, "\n")
	}
	current := "no active curriculum"
	if dashboard.Current != nil {
		current = strings.Join([]string{
			dashboard.Current.Phase.Title,
			dashboard.Current.Module.Title,
			dashboard.Current.Lesson.Title,
			dashboard.Current.Topic.Title,
			dashboard.Current.Concept.Title,
		}, " / ")
	}
	progress := dashboard.OverallProgress
	return strings.Join([]string{
		"Learning status",
		"Goal: " + dashboard.Goal.Title,
		"Current: " + current,
		fmt.Sprintf("Mastery threshold: %.0f%%", dashboardMasteryThreshold(dashboard)*100),
		"",
		"Concepts",
		fmt.Sprintf("Mastered: %d", progress.ConceptsMastered.Value),
		fmt.Sprintf("Learning: %d", progress.ConceptsLearning.Value),
		fmt.Sprintf("Review due: %d", dashboard.ReviewsDue.Value),
	}, "\n")
}

func formatProgressDashboard(dashboard learningapp.ProgressDashboard) string {
	if dashboard.Goal == nil {
		return "Progress\nNo active learning goal. Run `kelyro setup status` to inspect setup, or `kelyro goal` to inspect learning goals."
	}
	progress := dashboard.OverallProgress
	average := "unknown"
	if dashboard.Mastery.AverageKnown.Value != nil {
		average = fmt.Sprintf("%.0f%%", dashboard.Mastery.AverageKnown.Value.Value()*100)
	}
	lines := []string{
		"Progress",
		"Goal: " + dashboard.Goal.Title,
		fmt.Sprintf("Completion: %.0f%%", progress.Completion.Value),
		fmt.Sprintf("Mastered: %d of %d concepts", progress.ConceptsMastered.Value, progress.ConceptsTotal.Value),
		fmt.Sprintf("Learning: %d", progress.ConceptsLearning.Value),
		fmt.Sprintf("Introduced: %d", progress.ConceptsIntroduced.Value),
		"Average mastery (known concepts): " + average,
		fmt.Sprintf("Mastery threshold: %.0f%%", dashboardMasteryThreshold(dashboard)*100),
		fmt.Sprintf("Reviews due: %d", dashboard.ReviewsDue.Value),
		"Study today: " + formatDashboardDuration(dashboard.StudyTime.Today.Value),
		"Study this week: " + formatDashboardDuration(dashboard.StudyTime.Week.Value),
		fmt.Sprintf("Current streak: %d %s", dashboard.Streak.CurrentStreak.Value, pluralDays(dashboard.Streak.CurrentStreak.Value)),
		"Meaning: completion counts mastered curriculum concepts; average mastery excludes unknown concepts.",
	}
	if dashboard.RecentMilestone != nil {
		lines = append(lines, "Recent milestone: "+dashboard.RecentMilestone.Name)
	}
	if len(dashboard.WeakConcepts) > 0 {
		lines = append(lines, "", "Needs reinforcement")
		for _, concept := range dashboard.WeakConcepts {
			lines = append(lines, fmt.Sprintf("- %s: %.0f%% mastery", concept.Title, concept.Mastery.Value()*100))
		}
	}
	return strings.Join(lines, "\n")
}

func formatRoadmapDashboard(dashboard learningapp.ProgressDashboard) string {
	if dashboard.Curriculum == nil {
		return "Roadmap\nNo active curriculum. Run `kelyro setup status` to inspect setup, or activate a learning goal to create one."
	}
	lines := []string{"Roadmap"}
	for _, node := range dashboard.Roadmap {
		indent := strings.Repeat("  ", node.Depth)
		if node.Type != learning.CurriculumNodeConcept {
			label := string(node.Type)
			if label != "" {
				label = strings.ToUpper(label[:1]) + label[1:]
			}
			lines = append(lines, indent+label+": "+node.Title)
			continue
		}
		status := strings.ReplaceAll(string(node.Status), "_", " ")
		line := indent + "- " + node.Title + " [" + status + "]"
		if node.Mastery != nil {
			line += fmt.Sprintf(" %.0f%% mastery", node.Mastery.Value()*100)
		}
		lines = append(lines, line)
		for _, reason := range node.LockReasons {
			lines = append(lines, indent+"  Why: "+reason)
		}
	}
	if len(dashboard.Roadmap) == 0 {
		lines = append(lines, "No curriculum nodes are available.")
	}
	lines = append(lines, "", "Legend: mastered, current, available, locked, review due")
	return strings.Join(lines, "\n")
}

func formatTodayDashboard(dashboard learningapp.ProgressDashboard) string {
	if dashboard.Goal == nil {
		return "Today\nNo active learning goal. Run `kelyro setup status` to inspect setup, or `kelyro goal` to inspect learning goals."
	}
	if dashboard.TodayPlan == nil {
		return "Today\nGoal: " + dashboard.Goal.Title + "\nNo daily plan is available yet."
	}
	plan := dashboard.TodayPlan
	lines := []string{
		"Today",
		"Goal: " + dashboard.Goal.Title,
		fmt.Sprintf("Planned: %d of %d minutes", plan.PlannedMinutes, plan.AvailableMinutes),
	}
	if len(plan.Items) == 0 {
		message := "Nothing urgent today. Your current progress has no scheduled work."
		if plan.Status == learning.DailyPlanTimeLimited {
			message = "Today's time budget is too small for the next useful study item."
		}
		return strings.Join(append(lines, message), "\n")
	}
	for index, item := range plan.Items {
		title := "general learning activity"
		if len(item.ConceptIDs) > 0 {
			title = dashboardConceptTitle(dashboard, item.ConceptIDs[0])
		}
		role := strings.ReplaceAll(string(item.Role), "_", " ")
		lines = append(lines, fmt.Sprintf("%d. %s — %s (%d min)", index+1, role, title, item.EstimatedMinutes))
		explanation := item.Explanation
		for _, conceptID := range item.ConceptIDs {
			explanation = strings.ReplaceAll(explanation, conceptID.String(), dashboardConceptTitle(dashboard, conceptID))
		}
		if explanation != "" {
			lines = append(lines, "   "+explanation)
		}
	}
	return strings.Join(lines, "\n")
}

func dashboardMasteryThreshold(dashboard learningapp.ProgressDashboard) float64 {
	if dashboard.MasteryRequirement.PolicyVersion == learning.MasteryThresholdPolicyVersion {
		return dashboard.MasteryRequirement.Requirement.Threshold.Value()
	}
	if dashboard.Goal != nil {
		return dashboard.Goal.MasteryThreshold.Value()
	}
	return 0
}

func dashboardConceptTitle(dashboard learningapp.ProgressDashboard, conceptID learning.ID) string {
	for _, node := range dashboard.Roadmap {
		if node.ID == conceptID {
			return node.Title
		}
	}
	return conceptID.String()
}

func formatDashboardDuration(duration time.Duration) string {
	minutes := int(duration / time.Minute)
	if minutes < 60 {
		return fmt.Sprintf("%dm", minutes)
	}
	return fmt.Sprintf("%dh%02dm", minutes/60, minutes%60)
}

func formatMistakes(mistakes []learning.Mistake) string {
	if len(mistakes) == 0 {
		return "No remembered mistakes."
	}
	lines := []string{fmt.Sprintf("Mistakes (%d)", len(mistakes))}
	for _, mistake := range mistakes {
		lines = append(lines, fmt.Sprintf("[%s] %s (%s) — %s, %d occurrence(s), last seen %s",
			mistake.Status, mistake.Summary, mistake.ID, mistake.Category, mistake.Occurrences,
			mistake.LastSeenAt.Time().Format(time.RFC3339)))
	}
	return strings.Join(lines, "\n")
}

func formatMistake(view learningapp.MistakeView) string {
	mistake := view.Mistake
	resolved := "<not resolved>"
	if mistake.ResolvedAt != nil {
		resolved = mistake.ResolvedAt.Time().Format(time.RFC3339)
	}
	lines := []string{
		"Mistake memory",
		"ID: " + mistake.ID.String(),
		"Concept: " + mistake.ConceptID.String(),
		"Key: " + string(mistake.Key),
		"Category: " + string(mistake.Category),
		"Summary: " + mistake.Summary,
		"Status: " + string(mistake.Status),
		fmt.Sprintf("Occurrences: %d", mistake.Occurrences),
		"First seen: " + mistake.FirstSeenAt.Time().Format(time.RFC3339),
		"Last seen: " + mistake.LastSeenAt.Time().Format(time.RFC3339),
		"Latest source: " + mistake.SourceRef,
		"Resolved: " + resolved,
		fmt.Sprintf("History (%d)", len(view.History)),
	}
	for _, event := range view.History {
		lines = append(lines, fmt.Sprintf("- %s at %s — %s", event.Type, event.OccurredAt.Time().Format(time.RFC3339), event.SourceRef))
	}
	return strings.Join(lines, "\n")
}

func formatStudySession(session learning.StudySession) string {
	lines := []string{
		"Study session",
		"Status: " + string(session.Status),
		"Goal: " + session.GoalID.String(),
		"Curriculum instance: " + session.CurriculumInstanceID.String(),
		"Started: " + session.StartedAt.Time().Format(time.RFC3339),
		"Last activity: " + session.LastActivityAt.Time().Format(time.RFC3339),
		"Active time: " + session.ActiveDuration.String(),
		fmt.Sprintf("Activities: %d", session.ActivityCount),
		"Idle timeout: " + session.IdleTimeout.String(),
		"Policy: " + session.PolicyVersion,
	}
	if session.EndedAt != nil {
		lines = append(lines, "Ended: "+session.EndedAt.Time().Format(time.RFC3339))
	}
	return strings.Join(lines, "\n")
}

func formatStudyHistory(view learningapp.StudyHistoryView) string {
	heading := "Study history"
	if view.Period == learning.StudyPeriodToday {
		heading += " — today"
	}
	if len(view.Events) == 0 {
		return heading + "\nNo study events."
	}
	location, err := time.LoadLocation(view.Timezone)
	if err != nil {
		location = time.UTC
	}
	lines := []string{heading, "Timezone: " + view.Timezone}
	for _, event := range view.Events {
		scope := make([]string, 0, 3)
		if event.GoalID != nil {
			scope = append(scope, "goal="+event.GoalID.String())
		}
		if event.CurriculumInstanceID != nil {
			scope = append(scope, "instance="+event.CurriculumInstanceID.String())
		}
		if event.ConceptID != nil {
			scope = append(scope, "concept="+event.ConceptID.String())
		}
		suffix := ""
		if len(scope) > 0 {
			suffix = " — " + strings.Join(scope, ", ")
		}
		lines = append(lines, fmt.Sprintf("- %s  %s%s", event.OccurredAt.Time().In(location).Format(time.RFC3339), event.Type, suffix))
	}
	return strings.Join(lines, "\n")
}

func formatStudyTime(summary learningapp.StudyTimeSummary) string {
	lines := []string{
		"Study time",
		fmt.Sprintf("Today: %s (%d sessions)", summary.Today, summary.TodaySessions),
		fmt.Sprintf("This week: %s (%d sessions)", summary.Week, summary.WeekSessions),
		fmt.Sprintf("This month: %s (%d sessions)", summary.Month, summary.MonthSessions),
		fmt.Sprintf("Total: %s (%d sessions)", summary.Total, summary.TotalSessions),
		"Timezone: " + summary.Timezone,
		"By concept: " + formatStudyBreakdowns(summary.ByConcept),
		"By module: " + formatStudyBreakdowns(summary.ByModule),
		"Policy: " + summary.PolicyVersion,
		"Meaning: intentional active study time; concept and module totals appear only for unambiguous sessions.",
	}
	return strings.Join(lines, "\n")
}

func formatReviews(view learningapp.ReviewQueueView) string {
	heading := "Scheduled reviews"
	if view.DueOnly {
		heading = "Reviews — due"
	}
	lines := []string{heading, "Timezone: " + view.Timezone, fmt.Sprintf("Pending: %d", view.Pending)}
	if view.DueOnly {
		lines = append(lines,
			fmt.Sprintf("Daily budget: %d minutes; selected: %d minutes; total due: %d minutes", view.BudgetMinutes, view.UsedMinutes, view.TotalDueMinutes),
			fmt.Sprintf("Deferred by budget: %d", len(view.Deferred)))
	}
	location, err := time.LoadLocation(view.Timezone)
	if err != nil {
		location = time.UTC
	}
	for _, queued := range view.Items {
		labels := []string{string(queued.Item.Type), string(queued.Status)}
		if queued.Overdue {
			labels = append(labels, "overdue")
		}
		if queued.Critical {
			labels = append(labels, "critical-prerequisite")
		}
		lines = append(lines, fmt.Sprintf("%s  %s  %s  %d min  strength %.0f%%  [%s]",
			queued.Item.DueAt.Time().In(location).Format(time.RFC3339), queued.Item.ID, queued.Item.ConceptID,
			queued.Item.EstimatedMinutes, queued.Strength.Value()*100, strings.Join(labels, ", ")))
	}
	if len(view.Items) == 0 {
		if view.DueOnly {
			lines = append(lines, "No reviews fit the due queue today.")
		} else {
			lines = append(lines, "No reviews are scheduled.")
		}
	}
	lines = append(lines, "Policy: "+view.AlgorithmVersion)
	return strings.Join(lines, "\n")
}

func formatStreak(streak learning.Streak) string {
	lastActive := "none yet"
	if streak.LastActiveLocalDate != nil {
		lastActive = streak.LastActiveLocalDate.String()
	}
	return strings.Join([]string{
		fmt.Sprintf("Streak: %d %s", streak.CurrentDays, pluralDays(streak.CurrentDays)),
		fmt.Sprintf("Longest: %d %s", streak.LongestDays, pluralDays(streak.LongestDays)),
		fmt.Sprintf("Total active days: %d", streak.TotalActiveDays),
		"Last active date: " + lastActive,
		"Timezone: " + streak.Timezone,
		fmt.Sprintf("Policy: %s (%d active minutes or one completed educational activity)", streak.PolicyVersion, streak.MinimumActiveMinutes),
		"Meaning: study consistency only; it does not change mastery or block learning.",
	}, "\n")
}

func pluralDays(days int) string {
	if days == 1 {
		return "day"
	}
	return "days"
}

func formatStudyBreakdowns(items []learning.StudyTimeBreakdown) string {
	if len(items) == 0 {
		return "unavailable"
	}
	values := make([]string, 0, len(items))
	for _, item := range items {
		values = append(values, fmt.Sprintf("%s=%s (%d sessions)", item.ID, item.Duration, item.Sessions))
	}
	return strings.Join(values, ", ")
}

func formatMasteryThreshold(resolved learning.ResolvedMasteryThreshold) string {
	percentage := resolved.Requirement.Threshold.Value() * 100
	return strings.Join([]string{
		"Required mastery: " + strconv.FormatFloat(percentage, 'f', -1, 64) + "%",
		"Mode: " + resolved.Requirement.Mode.DisplayName(),
		"Source: " + resolved.Source.DisplayName(),
		"Policy: " + resolved.PolicyVersion,
		"Meaning: minimum calculated mastery required to advance; not an assessment grade.",
	}, "\n")
}

func formatGoal(goal learning.LearningGoal) string {
	description := goal.Description
	if description == "" {
		description = "<not set>"
	}
	activated := "<not yet>"
	if goal.ActivatedAt != nil {
		activated = goal.ActivatedAt.Time().Format(time.RFC3339)
	}
	completed := "<not yet>"
	if goal.CompletedAt != nil {
		completed = goal.CompletedAt.Time().Format(time.RFC3339)
	}
	return strings.Join([]string{
		"Learning goal",
		"ID: " + goal.ID.String(),
		"Title: " + goal.Title,
		"Description: " + description,
		"Domain: " + goal.Domain,
		"Target outcome: " + goal.TargetOutcome,
		"Starting level: " + string(goal.StartingLevel),
		"Status: " + string(goal.Status),
		fmt.Sprintf("Mastery threshold: %.2f", goal.MasteryThreshold.Value()),
		"Activated: " + activated,
		"Completed: " + completed,
	}, "\n")
}

func formatGoals(goals []learning.LearningGoal) string {
	if len(goals) == 0 {
		return "No learning goals."
	}
	lines := []string{fmt.Sprintf("Learning goals (%d)", len(goals))}
	for _, goal := range goals {
		lines = append(lines, fmt.Sprintf("[%s] %s (%s) — %s", goal.Status, goal.Title, goal.ID, goal.TargetOutcome))
	}
	return strings.Join(lines, "\n")
}

func formatProfile(student learning.Student) string {
	displayName := student.Profile.DisplayName
	if displayName == "" {
		displayName = "<not set>"
	}
	preferences := make([]string, len(student.Profile.Preferences))
	for index, preference := range student.Profile.Preferences {
		preferences[index] = string(preference)
	}
	styles := strings.Join(preferences, ", ")
	if styles == "" {
		styles = "<none>"
	}
	return strings.Join([]string{
		"Learner profile",
		"Display name: " + displayName,
		"General experience: " + string(student.Profile.Experience),
		"Preferred language: " + student.Profile.PreferredLanguage,
		fmt.Sprintf("Daily time budget: %d minutes", student.Profile.Availability.DailyMinutes),
		fmt.Sprintf("Weekly study target: %d days", student.Profile.Availability.WeeklyDaysTarget),
		"Learning styles: " + styles,
		"Timezone: " + student.Profile.Timezone,
	}, "\n")
}

func formatUpdate(result update.Result) string {
	source := ""
	if result.Source != update.SourceNone {
		source = fmt.Sprintf("; source=%s", result.Source)
	}
	switch result.Status {
	case update.UpdateAvailable:
		message := fmt.Sprintf("Update available: %s -> %s (channel=%s%s)", result.CurrentVersion, result.LatestVersion, result.Channel, source)
		if result.ReleaseURL != "" {
			message += "\nRelease: " + result.ReleaseURL
		}
		return message + "\nAutomatic installation is unavailable until signed artifacts and checksums can be verified."
	case update.UpToDate:
		if result.LatestVersion == "" {
			return fmt.Sprintf("No published releases found (current=%s channel=%s%s).", result.CurrentVersion, result.Channel, source)
		}
		message := fmt.Sprintf("Kelyro %s is up to date (latest=%s channel=%s%s).", result.CurrentVersion, result.LatestVersion, result.Channel, source)
		if result.Detail != "" {
			message += " " + result.Detail
		}
		return message
	case update.Unavailable:
		return fmt.Sprintf("Update check unavailable (current=%s channel=%s): %s.", result.CurrentVersion, result.Channel, result.Detail)
	default:
		return "Update check returned an unknown status."
	}
}

func formatPortability(report portability.Report) string {
	if report.Destination == "" {
		return fmt.Sprintf("Exported %s workspace to %s (files=%d bytes=%d)", report.Mode, report.ArchivePath, report.FileCount, report.TotalSize)
	}
	prefix := "Imported"
	if report.DryRun {
		prefix = "Import dry run"
	}
	line := fmt.Sprintf("%s %s into %s (files=%d create=%d replace=%d skip=%d conflicts=%d)",
		prefix, report.ArchivePath, report.Destination, report.FileCount,
		len(report.Creates), len(report.Replaces), len(report.Skips), len(report.Conflicts))
	if len(report.Conflicts) > 0 {
		line += "\nConflicts: " + strings.Join(report.Conflicts, ", ")
	}
	return line
}

func formatBackups(backups []backup.Info) string {
	if len(backups) == 0 {
		return "No backups."
	}
	lines := make([]string, 0, len(backups))
	for _, item := range backups {
		lines = append(lines, fmt.Sprintf("%s  %s  reason=%s  schema=%d  files=%d  bytes=%d  version=%s",
			item.ID, item.CreatedAt.UTC().Format("2006-01-02T15:04:05Z07:00"), item.Reason,
			item.DatabaseSchemaVersion, item.FileCount, item.TotalSize, item.AppVersion))
	}
	return strings.Join(lines, "\n")
}

func formatAudit(entries []audit.Entry) string {
	if len(entries) == 0 {
		return "No audit events."
	}
	lines := make([]string, 0, len(entries))
	for _, entry := range entries {
		line := fmt.Sprintf("%s  %s  actor=%s  subject=%s  version=%s",
			entry.Timestamp.UTC().Format("2006-01-02T15:04:05.000000000Z07:00"),
			entry.Event,
			entry.Actor,
			entry.Subject,
			entry.AppVersion,
		)
		lines = append(lines, line)
	}
	return strings.Join(lines, "\n")
}

func formatGuidance(guidance doctor.Guidance) string {
	requirement := string(guidance.Requirement)
	if requirement != "" {
		requirement = strings.ToUpper(requirement[:1]) + requirement[1:]
	}
	lines := []string{guidance.DisplayName + " — " + requirement}
	if guidance.Description != "" {
		lines = append(lines, "", "What it is:", guidance.Description)
	}
	if guidance.WhyNeeded != "" {
		lines = append(lines, "", "Why:", guidance.WhyNeeded)
	}
	if guidance.FoundationFirst != "" {
		lines = append(lines, "", "Foundation first:", guidance.FoundationFirst)
	}
	if guidance.PlatformGuidance != "" {
		lines = append(lines, "", "On "+guidance.Platform+":", guidance.PlatformGuidance)
	}
	if guidance.LearnMore != "" {
		lines = append(lines, "", "Official documentation:", guidance.LearnMore)
	}
	return strings.Join(lines, "\n")
}

func formatDiagnostics(report doctor.Report) string {
	var lines []string
	for _, section := range report.Sections() {
		if len(lines) > 0 {
			lines = append(lines, "")
		}
		lines = append(lines, section)
		for _, check := range report.ChecksIn(section) {
			marker := "✓"
			if check.State == doctor.Fail {
				marker = "✗"
			} else if check.State == doctor.Miss {
				marker = "○"
			} else if check.State == doctor.Deferred {
				marker = "·"
			}
			label := check.DisplayName
			if check.Requirement != doctor.Required {
				label += " [" + string(check.Requirement) + "]"
			}
			if check.Detail != "" {
				label += " — " + check.Detail
			}
			lines = append(lines, marker+" "+label)
			if check.WhyNeeded != "" {
				lines = append(lines, "  Why: "+check.WhyNeeded)
			}
			if check.MinimumVersion != "" {
				lines = append(lines, "  Minimum version: "+check.MinimumVersion)
			}
			if check.OfficialSource != "" {
				lines = append(lines, "  Official source: "+check.OfficialSource)
			}
			if check.InstallGuidance != "" {
				lines = append(lines, "  Install guidance: "+check.InstallGuidance)
			}
			if check.LearnMore != "" && (check.State != doctor.Pass || check.OfficialSource != "") {
				lines = append(lines, "  Learn more: "+check.LearnMore)
			}
		}
	}
	return strings.Join(lines, "\n")
}

type invocation struct {
	command                 string
	workspace               string
	help                    bool
	version                 bool
	noColor                 bool
	verbose                 bool
	quiet                   bool
	allowNested             bool
	arguments               []string
	configScope             config.Scope
	configOperation         string
	configKey               string
	configValue             string
	secretOperation         string
	secretName              string
	openTarget              string
	doctorExplain           string
	logOperation            string
	backupOperation         string
	backupID                string
	yes                     bool
	exportMode              portability.Mode
	exportOutput            string
	importArchive           string
	importDryRun            bool
	importConflicts         portability.ConflictStrategy
	conflictSet             bool
	updateOperation         string
	profileOperation        string
	profileChanges          learningapp.ProfileChanges
	profileFlagsSet         bool
	goalOperation           string
	goalInput               learningapp.SetGoalInput
	goalFlagsSet            bool
	masteryOperation        string
	masteryThreshold        learning.MasteryThreshold
	setupOperation          string
	mistakeOperation        string
	mistakeID               learning.ID
	sessionOperation        string
	historyToday            bool
	progressOperation       string
	maintenanceOperation    string
	maintenanceDryRun       bool
	reviewsDue              bool
	sourceRegistryOperation string
	researchCacheOperation  string
	researchOperation       string
	researchTopic           string
	researchRunID           research.ID
	sourceID                research.SourceID
	sourceRegistryID        research.ID
	provenanceClaimID       research.ClaimID
	packPath                string
	packOperation           string
	packID                  curriculum.ID
	packVersion             curriculum.PackVersion
	packQuery               string
	packDryRun              bool
	curriculumOperation     string
}

func parse(args []string) (invocation, error) {
	defaultThreshold, _ := learning.NewMasteryThreshold(0.8)
	result := invocation{
		exportMode: portability.ModeHuman, importConflicts: portability.ConflictFail,
		goalInput: learningapp.SetGoalInput{StartingLevel: learning.ExperienceNovice, MasteryThreshold: defaultThreshold},
	}

	for index := 0; index < len(args); index++ {
		argument := args[index]
		switch {
		case argument == "-h" || argument == "--help":
			result.help = true
		case argument == "--version":
			result.version = true
		case argument == "--no-color":
			result.noColor = true
		case argument == "--verbose":
			result.verbose = true
		case argument == "--quiet":
			result.quiet = true
		case argument == "--allow-nested":
			result.allowNested = true
		case argument == "--yes":
			result.yes = true
		case argument == "--full":
			result.exportMode = portability.ModeFull
		case argument == "--dry-run":
			result.importDryRun = true
			result.maintenanceDryRun = true
			result.packDryRun = true
		case argument == "--today":
			result.historyToday = true
		case argument == "--display-name":
			index++
			if index >= len(args) {
				return invocation{}, fmt.Errorf("option --display-name requires a value")
			}
			value := args[index]
			result.profileChanges.DisplayName = &value
			result.profileFlagsSet = true
		case strings.HasPrefix(argument, "--display-name="):
			value := strings.TrimPrefix(argument, "--display-name=")
			result.profileChanges.DisplayName = &value
			result.profileFlagsSet = true
		case argument == "--experience":
			index++
			if index >= len(args) {
				return invocation{}, fmt.Errorf("option --experience requires a level")
			}
			value := learning.ExperienceLevel(args[index])
			result.profileChanges.Experience = &value
			result.profileFlagsSet = true
		case strings.HasPrefix(argument, "--experience="):
			value := learning.ExperienceLevel(strings.TrimPrefix(argument, "--experience="))
			result.profileChanges.Experience = &value
			result.profileFlagsSet = true
		case argument == "--language":
			index++
			if index >= len(args) {
				return invocation{}, fmt.Errorf("option --language requires a language tag")
			}
			value := args[index]
			result.profileChanges.PreferredLanguage = &value
			result.profileFlagsSet = true
		case strings.HasPrefix(argument, "--language="):
			value := strings.TrimPrefix(argument, "--language=")
			result.profileChanges.PreferredLanguage = &value
			result.profileFlagsSet = true
		case argument == "--daily-minutes" || strings.HasPrefix(argument, "--daily-minutes="):
			value, next, err := integerOption(args, index, "--daily-minutes", argument)
			if err != nil {
				return invocation{}, err
			}
			index = next
			result.profileChanges.DailyMinutes = &value
			result.profileFlagsSet = true
		case argument == "--weekly-days" || strings.HasPrefix(argument, "--weekly-days="):
			value, next, err := integerOption(args, index, "--weekly-days", argument)
			if err != nil {
				return invocation{}, err
			}
			index = next
			result.profileChanges.WeeklyDaysTarget = &value
			result.profileFlagsSet = true
		case argument == "--learning-styles":
			index++
			if index >= len(args) {
				return invocation{}, fmt.Errorf("option --learning-styles requires a comma-separated list")
			}
			styles := parseStudyPreferences(args[index])
			result.profileChanges.Preferences = &styles
			result.profileFlagsSet = true
		case strings.HasPrefix(argument, "--learning-styles="):
			styles := parseStudyPreferences(strings.TrimPrefix(argument, "--learning-styles="))
			result.profileChanges.Preferences = &styles
			result.profileFlagsSet = true
		case argument == "--timezone":
			index++
			if index >= len(args) {
				return invocation{}, fmt.Errorf("option --timezone requires an IANA timezone")
			}
			value := args[index]
			result.profileChanges.Timezone = &value
			result.profileFlagsSet = true
		case strings.HasPrefix(argument, "--timezone="):
			value := strings.TrimPrefix(argument, "--timezone=")
			result.profileChanges.Timezone = &value
			result.profileFlagsSet = true
		case argument == "--title" || strings.HasPrefix(argument, "--title="):
			value, next, optionErr := textOption(args, index, "--title", argument, false)
			if optionErr != nil {
				return invocation{}, optionErr
			}
			index = next
			result.goalInput.Title = value
			result.goalFlagsSet = true
		case argument == "--description" || strings.HasPrefix(argument, "--description="):
			value, next, optionErr := textOption(args, index, "--description", argument, true)
			if optionErr != nil {
				return invocation{}, optionErr
			}
			index = next
			result.goalInput.Description = value
			result.goalFlagsSet = true
		case argument == "--domain" || strings.HasPrefix(argument, "--domain="):
			value, next, optionErr := textOption(args, index, "--domain", argument, false)
			if optionErr != nil {
				return invocation{}, optionErr
			}
			index = next
			result.goalInput.Domain = value
			result.goalFlagsSet = true
		case argument == "--target-outcome" || strings.HasPrefix(argument, "--target-outcome="):
			value, next, optionErr := textOption(args, index, "--target-outcome", argument, false)
			if optionErr != nil {
				return invocation{}, optionErr
			}
			index = next
			result.goalInput.TargetOutcome = value
			result.goalFlagsSet = true
		case argument == "--starting-level" || strings.HasPrefix(argument, "--starting-level="):
			value, next, optionErr := textOption(args, index, "--starting-level", argument, false)
			if optionErr != nil {
				return invocation{}, optionErr
			}
			index = next
			result.goalInput.StartingLevel = learning.ExperienceLevel(value)
			result.goalFlagsSet = true
		case argument == "--mastery-threshold" || strings.HasPrefix(argument, "--mastery-threshold="):
			value, next, optionErr := floatOption(args, index, "--mastery-threshold", argument)
			if optionErr != nil {
				return invocation{}, optionErr
			}
			index = next
			threshold, thresholdErr := learning.NewMasteryThreshold(value)
			if thresholdErr != nil {
				return invocation{}, fmt.Errorf("option --mastery-threshold must be between 0.50 and 0.99")
			}
			if _, thresholdErr := learning.MasteryRequirementFromThreshold(threshold); thresholdErr != nil {
				return invocation{}, fmt.Errorf("option --mastery-threshold must be between 0.50 and 0.99")
			}
			result.goalInput.MasteryThreshold = threshold
			result.goalFlagsSet = true
		case argument == "--output":
			index++
			if index >= len(args) || strings.TrimSpace(args[index]) == "" || strings.HasPrefix(args[index], "-") {
				return invocation{}, fmt.Errorf("option --output requires a file")
			}
			result.exportOutput = args[index]
		case strings.HasPrefix(argument, "--output="):
			result.exportOutput = strings.TrimSpace(strings.TrimPrefix(argument, "--output="))
			if result.exportOutput == "" {
				return invocation{}, fmt.Errorf("option --output requires a file")
			}
		case argument == "--conflict":
			index++
			if index >= len(args) {
				return invocation{}, fmt.Errorf("option --conflict requires fail, keep, or overwrite")
			}
			result.importConflicts = portability.ConflictStrategy(args[index])
			result.conflictSet = true
		case strings.HasPrefix(argument, "--conflict="):
			result.importConflicts = portability.ConflictStrategy(strings.TrimSpace(strings.TrimPrefix(argument, "--conflict=")))
			result.conflictSet = true
		case argument == "--explain":
			index++
			if index >= len(args) || strings.TrimSpace(args[index]) == "" || strings.HasPrefix(args[index], "-") {
				return invocation{}, fmt.Errorf("option --explain requires a tool id")
			}
			result.doctorExplain = args[index]
		case strings.HasPrefix(argument, "--explain="):
			result.doctorExplain = strings.TrimSpace(strings.TrimPrefix(argument, "--explain="))
			if result.doctorExplain == "" {
				return invocation{}, fmt.Errorf("option --explain requires a tool id")
			}
		case argument == "--global":
			if result.configScope == config.ScopeProject {
				return invocation{}, fmt.Errorf("options --global and --project cannot be combined")
			}
			result.configScope = config.ScopeGlobal
		case argument == "--project":
			if result.configScope == config.ScopeGlobal {
				return invocation{}, fmt.Errorf("options --global and --project cannot be combined")
			}
			result.configScope = config.ScopeProject
		case argument == "--workspace":
			index++
			if index >= len(args) || args[index] == "" {
				return invocation{}, fmt.Errorf("option --workspace requires a path")
			}
			result.workspace = args[index]
		case strings.HasPrefix(argument, "--workspace="):
			result.workspace = strings.TrimPrefix(argument, "--workspace=")
			if result.workspace == "" {
				return invocation{}, fmt.Errorf("option --workspace requires a path")
			}
		case strings.HasPrefix(argument, "-"):
			return invocation{}, fmt.Errorf("unknown option %q", argument)
		case result.command == "":
			result.command = argument
		default:
			result.arguments = append(result.arguments, argument)
		}
	}

	if result.help && result.version {
		return invocation{}, fmt.Errorf("options --help and --version cannot be combined")
	}
	if result.verbose && result.quiet {
		return invocation{}, fmt.Errorf("options --verbose and --quiet cannot be combined")
	}
	if result.allowNested && result.command != "init" {
		return invocation{}, fmt.Errorf("option --allow-nested requires the init command")
	}
	if result.yes && result.command != "backup" && result.command != "setup" && result.command != "packs" {
		return invocation{}, fmt.Errorf("option --yes requires backup restore or setup reset, or packs upgrade")
	}
	if result.configScope != "" && result.command != "config" {
		return invocation{}, fmt.Errorf("configuration scope options require the config command")
	}
	if result.doctorExplain != "" && result.command != "doctor" {
		return invocation{}, fmt.Errorf("option --explain requires the doctor command")
	}
	if result.help {
		result.command = "help"
	}
	if result.version {
		if result.command != "" && result.command != "version" {
			return invocation{}, fmt.Errorf("option --version cannot be combined with a command")
		}
		result.command = "version"
	}

	switch result.command {
	case "help":
		result.help = true
	case "version":
		result.version = true
	case "config":
		if err := parseConfigArguments(&result); err != nil {
			return invocation{}, err
		}
	case "secrets":
		if err := parseSecretArguments(&result); err != nil {
			return invocation{}, err
		}
	case "open":
		if err := parseOpenArguments(&result); err != nil {
			return invocation{}, err
		}
	case "doctor":
		if len(result.arguments) > 0 {
			return invocation{}, fmt.Errorf("doctor does not accept positional arguments")
		}
	case "logs":
		if err := parseLogArguments(&result); err != nil {
			return invocation{}, err
		}
	case "audit":
		if len(result.arguments) > 0 {
			return invocation{}, fmt.Errorf("audit does not accept positional arguments")
		}
	case "backup":
		if err := parseBackupArguments(&result); err != nil {
			return invocation{}, err
		}
	case "export":
		if len(result.arguments) != 0 {
			return invocation{}, fmt.Errorf("export does not accept positional arguments")
		}
	case "import":
		if len(result.arguments) != 1 {
			return invocation{}, fmt.Errorf("import requires exactly one archive file")
		}
		result.importArchive = result.arguments[0]
	case "update":
		if err := parseUpdateArguments(&result); err != nil {
			return invocation{}, err
		}
	case "progress":
		if len(result.arguments) == 0 {
			result.progressOperation = "show"
		} else if len(result.arguments) == 1 && result.arguments[0] == "export" {
			result.progressOperation = "export"
		} else {
			return invocation{}, fmt.Errorf("progress accepts no arguments or export")
		}
	case "status", "roadmap", "today":
		if len(result.arguments) != 0 {
			return invocation{}, fmt.Errorf("%s does not accept positional arguments", result.command)
		}
	case "profile":
		if err := parseProfileArguments(&result); err != nil {
			return invocation{}, err
		}
	case "goal":
		if err := parseGoalArguments(&result); err != nil {
			return invocation{}, err
		}
	case "mastery":
		if err := parseMasteryArguments(&result); err != nil {
			return invocation{}, err
		}
	case "setup":
		if err := parseSetupArguments(&result); err != nil {
			return invocation{}, err
		}
	case "mistakes":
		if err := parseMistakeArguments(&result); err != nil {
			return invocation{}, err
		}
	case "session":
		if err := parseSessionArguments(&result); err != nil {
			return invocation{}, err
		}
	case "history":
		if len(result.arguments) != 0 {
			return invocation{}, fmt.Errorf("history does not accept positional arguments")
		}
	case "time":
		if len(result.arguments) != 0 {
			return invocation{}, fmt.Errorf("time does not accept positional arguments")
		}
	case "reviews":
		if len(result.arguments) == 0 {
			break
		}
		if len(result.arguments) != 1 || result.arguments[0] != "due" {
			return invocation{}, fmt.Errorf("reviews accepts no arguments or due")
		}
		result.reviewsDue = true
	case "streak":
		if len(result.arguments) != 0 {
			return invocation{}, fmt.Errorf("streak does not accept positional arguments")
		}
	case "sources":
		if err := parseSourcesArguments(&result); err != nil {
			return invocation{}, err
		}
	case "research":
		if err := parseResearchArguments(&result); err != nil {
			return invocation{}, err
		}
	case "packs":
		if err := parsePackArguments(&result); err != nil {
			return invocation{}, err
		}
	case "curriculum":
		if len(result.arguments) != 1 {
			return invocation{}, fmt.Errorf("curriculum requires compile, validate, coverage, gaps, audit, or build-info")
		}
		switch result.arguments[0] {
		case "compile", "validate", "coverage", "gaps", "audit", "build-info":
		default:
			return invocation{}, fmt.Errorf("curriculum requires compile, validate, coverage, gaps, audit, or build-info")
		}
		result.curriculumOperation = result.arguments[0]
	case "maintenance":
		if len(result.arguments) != 1 || result.arguments[0] != "recalculate" {
			return invocation{}, fmt.Errorf("maintenance requires recalculate")
		}
		result.maintenanceOperation = "recalculate"
	default:
		if len(result.arguments) > 0 {
			return invocation{}, fmt.Errorf("unexpected argument %q", result.arguments[0])
		}
	}
	if result.yes && !((result.command == "backup" && result.backupOperation == "restore") || (result.command == "setup" && result.setupOperation == "reset") || (result.command == "packs" && result.packOperation == "upgrade")) {
		return invocation{}, fmt.Errorf("option --yes requires backup restore or setup reset, or packs upgrade")
	}
	if result.exportMode == portability.ModeFull && result.command != "export" {
		return invocation{}, fmt.Errorf("option --full requires the export command")
	}
	if result.exportOutput != "" && result.command != "export" {
		return invocation{}, fmt.Errorf("option --output requires the export command")
	}
	if result.importDryRun && result.command != "import" && result.command != "maintenance" && result.command != "packs" {
		return invocation{}, fmt.Errorf("option --dry-run requires the import command, maintenance recalculate, or packs upgrade")
	}
	if result.conflictSet && result.command != "import" {
		return invocation{}, fmt.Errorf("option --conflict requires the import command")
	}
	if !result.importConflicts.Valid() {
		return invocation{}, fmt.Errorf("option --conflict requires fail, keep, or overwrite")
	}
	if result.profileFlagsSet && (result.command != "profile" || result.profileOperation != "edit") {
		return invocation{}, fmt.Errorf("profile edit options require the profile edit command")
	}
	if result.goalFlagsSet && (result.command != "goal" || result.goalOperation != "set") {
		return invocation{}, fmt.Errorf("learning goal options require the goal set command")
	}
	if result.historyToday && result.command != "history" {
		return invocation{}, fmt.Errorf("option --today requires the history command")
	}

	return result, nil
}

func parseSourcesArguments(result *invocation) error {
	if len(result.arguments) == 0 || (len(result.arguments) == 1 && result.arguments[0] == "list") {
		result.sourceRegistryOperation = "sources-list"
		return nil
	}
	if len(result.arguments) == 2 && result.arguments[0] == "show" {
		id, err := research.NewSourceID(result.arguments[1])
		if err != nil {
			return fmt.Errorf("sources show: invalid source id: %w", err)
		}
		result.sourceRegistryOperation = "source-show"
		result.sourceID = id
		return nil
	}
	if len(result.arguments) == 1 && result.arguments[0] == "conflicts" {
		result.sourceRegistryOperation = "conflicts"
		return nil
	}
	if len(result.arguments) == 1 && result.arguments[0] == "stale" {
		result.sourceRegistryOperation = "stale"
		return nil
	}
	if len(result.arguments) == 2 && result.arguments[0] == "trace" {
		id, err := research.NewClaimID(result.arguments[1])
		if err != nil {
			return fmt.Errorf("sources trace: invalid claim id: %w", err)
		}
		result.sourceRegistryOperation = "trace"
		result.provenanceClaimID = id
		return nil
	}
	if len(result.arguments) == 2 && result.arguments[0] == "registry" && result.arguments[1] == "list" {
		result.sourceRegistryOperation = "list"
		return nil
	}
	if len(result.arguments) == 3 && result.arguments[0] == "registry" && result.arguments[1] == "show" {
		id, err := research.NewID(result.arguments[2])
		if err != nil {
			return fmt.Errorf("sources registry show: invalid id: %w", err)
		}
		result.sourceRegistryOperation = "show"
		result.sourceRegistryID = id
		return nil
	}
	return fmt.Errorf("sources requires list, show <source-id>, conflicts, registry list, registry show <id>, trace <claim-id>, or stale")
}

func parseResearchArguments(result *invocation) error {
	if len(result.arguments) >= 2 && result.arguments[0] == "topic" {
		topic := strings.Join(result.arguments[1:], " ")
		if strings.TrimSpace(topic) == "" {
			return fmt.Errorf("research topic requires a topic")
		}
		result.researchOperation = "topic"
		result.researchTopic = topic
		return nil
	}
	if len(result.arguments) == 2 && result.arguments[0] == "status" {
		id, err := research.NewID(result.arguments[1])
		if err != nil {
			return fmt.Errorf("research status: invalid run id: %w", err)
		}
		result.researchOperation = "status"
		result.researchRunID = id
		return nil
	}
	if len(result.arguments) == 2 && result.arguments[0] == "show" {
		id, err := research.NewID(result.arguments[1])
		if err != nil {
			return fmt.Errorf("research show: invalid run id: %w", err)
		}
		result.researchOperation = "show"
		result.researchRunID = id
		return nil
	}
	if len(result.arguments) == 1 && result.arguments[0] == "stats" {
		result.researchOperation = "stats"
		return nil
	}
	if len(result.arguments) == 1 && result.arguments[0] == "update-scan" {
		result.researchOperation = "update-scan"
		return nil
	}
	if len(result.arguments) == 2 && result.arguments[0] == "cache" &&
		(result.arguments[1] == "status" || result.arguments[1] == "clear") {
		result.researchCacheOperation = result.arguments[1]
		return nil
	}
	return fmt.Errorf("research requires topic <topic>, status <run-id>, show <run-id>, stats, update-scan, cache status, or cache clear")
}

func parsePackArguments(result *invocation) error {
	if len(result.arguments) == 0 {
		return fmt.Errorf("packs requires validate <path>, install <path>, list, show <id>, activate <id>@<version>, upgrade <id>, catalog, or search <query>")
	}
	result.packOperation = result.arguments[0]
	switch result.packOperation {
	case "validate", "install":
		if len(result.arguments) != 2 || strings.TrimSpace(result.arguments[1]) == "" {
			return fmt.Errorf("packs %s requires exactly one path", result.packOperation)
		}
		result.packPath = result.arguments[1]
		return nil
	case "list":
		if len(result.arguments) != 1 {
			return fmt.Errorf("packs list does not accept positional arguments")
		}
		return nil
	case "catalog":
		if len(result.arguments) != 1 {
			return fmt.Errorf("packs catalog does not accept positional arguments")
		}
		return nil
	case "search":
		if len(result.arguments) < 2 {
			return fmt.Errorf("packs search requires a query")
		}
		result.packQuery = strings.TrimSpace(strings.Join(result.arguments[1:], " "))
		if result.packQuery == "" {
			return fmt.Errorf("packs search requires a query")
		}
		return nil
	case "show":
		if len(result.arguments) != 2 {
			return fmt.Errorf("packs show requires exactly one pack id")
		}
		id, err := curriculum.NewID(result.arguments[1])
		if err != nil {
			return fmt.Errorf("packs show: invalid pack id: %w", err)
		}
		result.packID = id
		return nil
	case "upgrade":
		if len(result.arguments) != 2 {
			return fmt.Errorf("packs upgrade requires exactly one pack id or <id>@<version>")
		}
		value := result.arguments[1]
		separator := strings.LastIndex(value, "@")
		idValue := value
		if separator >= 0 {
			if separator == 0 || separator == len(value)-1 {
				return fmt.Errorf("packs upgrade requires a pack id or <id>@<version>")
			}
			idValue = value[:separator]
			version, err := curriculum.NewPackVersion(value[separator+1:])
			if err != nil {
				return fmt.Errorf("packs upgrade: invalid pack version: %w", err)
			}
			result.packVersion = version
		}
		id, err := curriculum.NewID(idValue)
		if err != nil {
			return fmt.Errorf("packs upgrade: invalid pack id: %w", err)
		}
		result.packID = id
		return nil
	case "activate":
		if len(result.arguments) != 2 {
			return fmt.Errorf("packs activate requires exactly one <id>@<version>")
		}
		separator := strings.LastIndex(result.arguments[1], "@")
		if separator <= 0 || separator == len(result.arguments[1])-1 {
			return fmt.Errorf("packs activate requires <id>@<version>")
		}
		id, err := curriculum.NewID(result.arguments[1][:separator])
		if err != nil {
			return fmt.Errorf("packs activate: invalid pack id: %w", err)
		}
		packVersion, err := curriculum.NewPackVersion(result.arguments[1][separator+1:])
		if err != nil {
			return fmt.Errorf("packs activate: invalid pack version: %w", err)
		}
		result.packID, result.packVersion = id, packVersion
		return nil
	default:
		return fmt.Errorf("packs requires validate <path>, install <path>, list, show <id>, activate <id>@<version>, upgrade <id>, catalog, or search <query>")
	}
}

func formatResearchView(view app.ResearchCLIView) string {
	status := string(view.Run.Status)
	primary, supporting, conflicts := 0, 0, 0
	lastVerified := "not available"
	if view.Bundle != nil && view.Progress == nil {
		status = strings.ReplaceAll(string(view.Bundle.State), "_", " ")
	}
	if view.Bundle != nil {
		for _, source := range view.Bundle.Sources {
			switch source.Role {
			case research.BundleSourcePrimary:
				primary++
			case research.BundleSourceSupporting:
				supporting++
			}
		}
		conflicts = len(view.Bundle.ConflictIDs)
		lastVerified = view.Bundle.VerifiedAt.Time().Format(time.RFC3339)
	}
	lines := []string{
		"Research: " + view.Request.Topic.Subject,
		"Run: " + view.Run.ID.String(),
		"", "Status: " + status,
		fmt.Sprintf("Primary sources: %d", primary),
		fmt.Sprintf("Supporting sources: %d", supporting),
		fmt.Sprintf("Conflicts: %d", conflicts),
		"Last verified: " + lastVerified,
	}
	if view.Progress != nil {
		lines = append(lines, formatResearchProgress(*view.Progress)...)
	}
	if view.Execution != nil {
		artifacts := view.Execution.Orchestration.Artifacts
		queriesPlanned := 0
		if view.Plan != nil {
			queriesPlanned = len(view.Plan.Queries)
		}
		verified := 0
		for _, result := range artifacts.Verifications {
			if result.Status == research.VerificationVerified || result.Status == research.VerificationVerifiedCaveat {
				verified++
			}
		}
		lines = append(lines,
			"",
			fmt.Sprintf("Queries planned: %d", queriesPlanned),
			fmt.Sprintf("Sources discovered: %d", len(artifacts.Sources)),
			fmt.Sprintf("Sources fetched: %d", len(artifacts.FetchedSources)),
			fmt.Sprintf("Evidence items: %d", len(artifacts.Evidence)),
			fmt.Sprintf("Claims: %d", len(artifacts.Claims)),
			fmt.Sprintf("Verified: %d", verified),
		)
		if view.Bundle != nil {
			lines = append(lines, "", "Source Bundle:", view.Bundle.ID.String())
		}
	}
	if view.DiscoveryPending {
		policy := "blocked by privacy.allow_network"
		if view.NetworkAllowed {
			policy = "allowed; no live discovery adapter configured"
		}
		lines = append(lines, "Discovery: pending ("+policy+")")
	}
	if view.QueueItem != nil {
		triggers := make([]string, len(view.QueueItem.Triggers))
		for index, trigger := range view.QueueItem.Triggers {
			triggers[index] = string(trigger)
		}
		lines = append(lines, "Trigger: "+strings.Join(triggers, ", ")+" ("+string(view.QueueItem.Priority)+")")
	}
	if view.Plan != nil {
		lines = append(lines, "Query plan: "+view.Plan.AlgorithmVersion)
		for _, query := range view.Plan.Queries {
			lines = append(lines, fmt.Sprintf("- %d. %s [%s, tier %s+]", query.Priority, query.Query, query.DesiredSourceKind, query.RequiredAuthority))
		}
	}
	return strings.Join(lines, "\n")
}

func formatResearchAuditView(view app.ResearchAuditCLIView) string {
	lines := []string{
		"Research audit: " + view.Run.ID.String(),
		"Topic: " + view.Request.Topic.Subject,
		"Run status: " + string(view.Run.Status),
		fmt.Sprintf("Checkpoints: %d", len(view.Records)),
	}
	lines = append(lines, formatResearchProgress(view.Progress)...)
	if len(view.Records) == 0 {
		lines = append(lines, "Audit metadata: not recorded")
	}
	for index, record := range view.Records {
		completed := "not completed"
		if record.CompletedAt != nil {
			completed = record.CompletedAt.Time().Format(time.RFC3339)
		}
		providers := "none"
		if len(record.ProvidersUsed) > 0 {
			providers = strings.Join(record.ProvidersUsed, ", ")
		}
		network := string(record.NetworkMode)
		if record.NetworkAllowed {
			network += " (allowed)"
		} else {
			network += " (disabled by privacy gate)"
		}
		lines = append(lines,
			"",
			fmt.Sprintf("Checkpoint %d: %s", index+1, record.ID.String()),
			"Recorded: "+record.RecordedAt.Time().Format(time.RFC3339),
			"Outcome: "+string(record.Outcome),
			"Started: "+record.StartedAt.Time().Format(time.RFC3339),
			"Completed: "+completed,
			"Query planner: "+record.QueryPlannerVersion,
			"Trust policy: "+record.TrustPolicyVersion,
			"Freshness: "+record.FreshnessVersion,
			"Conflict resolver: "+record.ConflictResolverVersion,
			"Providers: "+providers,
			"Network: "+network,
			fmt.Sprintf("Usage: %d cache hits, %d sources, %d bytes fetched", record.CacheHits, record.SourceCount, record.BytesFetched),
			"Audit algorithm: "+record.AlgorithmVersion,
			"Audit hash: "+record.ContentHash,
		)
		if record.TargetTechnology != "" {
			target := record.TargetTechnology
			if record.TargetVersion != nil {
				target += " " + record.TargetVersion.String()
			}
			lines = append(lines, "Target: "+target)
		}
		lines = append(lines, "Queries:")
		for _, query := range record.Queries {
			lines = append(lines, "- "+query)
		}
		if len(record.Sources) > 0 {
			lines = append(lines, "Snapshots:")
			for _, source := range record.Sources {
				lines = append(lines, fmt.Sprintf("- %s — %s — %s", source.SourceID, source.Locator, source.SnapshotHash))
			}
		}
		if len(record.AdditionalAlgorithms) > 0 {
			lines = append(lines, "Additional algorithms:")
			for _, algorithm := range record.AdditionalAlgorithms {
				lines = append(lines, fmt.Sprintf("- %s: %s", algorithm.Stage, algorithm.Version))
			}
		}
	}
	return strings.Join(append(lines, "", research.ResearchAuditInternetDisclaimer), "\n")
}

func formatResearchProgress(progress app.ResearchRunProgressCLIView) []string {
	lines := []string{"Phase: " + progress.Phase}
	lines = append(lines, fmt.Sprintf("Queries: %d", len(progress.Queries)))
	for _, query := range progress.Queries {
		lines = append(lines, "- "+query)
	}
	if len(progress.Providers) == 0 {
		lines = append(lines, "Provider: none")
	} else {
		for _, provider := range progress.Providers {
			lines = append(lines, fmt.Sprintf("Provider: %s (%s, %d API calls)", provider.ProviderID, provider.AdapterVersion, provider.APICalls))
		}
	}
	lines = append(lines, fmt.Sprintf("Sources: %d results, %d fetch attempts, %d snapshots", progress.Results, progress.Fetches, progress.Snapshots))
	if len(progress.Warnings) == 0 {
		lines = append(lines, "Warnings: none")
	} else {
		lines = append(lines, "Warnings: "+strings.Join(progress.Warnings, ", "))
	}
	bundle := "none"
	if progress.BundleID != nil {
		bundle = progress.BundleID.String()
		if progress.BundleState != "" {
			bundle += " (" + strings.ReplaceAll(string(progress.BundleState), "_", " ") + ")"
		}
	}
	lines = append(lines, "Bundle: "+bundle)
	failure := progress.FailureReason
	if failure == "" {
		failure = "none"
	}
	return append(lines, "Failure reason: "+failure)
}

func formatSources(sources []research.Source) string {
	lines := []string{"Research sources"}
	if len(sources) == 0 {
		return strings.Join(append(lines, "No sources recorded."), "\n")
	}
	for _, source := range sources {
		lines = append(lines, fmt.Sprintf("- %s [%s, %s] — %s — %s", source.ID, source.Kind, source.TemporalScope, source.Metadata.Title, source.Locator))
	}
	return strings.Join(lines, "\n")
}

func formatSource(view app.SourceCLIView) string {
	source := view.Source
	lines := []string{
		"Research source", "ID: " + source.ID.String(), "Title: " + source.Metadata.Title,
		"Kind: " + string(source.Kind), "Temporal scope: " + string(source.TemporalScope),
		"Locator: " + source.Locator.String(), "Publisher: " + source.Metadata.Publisher,
	}
	if view.LatestSnapshot == nil {
		lines = append(lines, "Latest snapshot: not available")
	} else {
		lines = append(lines, "Latest snapshot: "+view.LatestSnapshot.ID.String(), "Fetched: "+view.LatestSnapshot.FetchedAt.Time().Format(time.RFC3339), "Content hash: "+view.LatestSnapshot.Fetch.ContentHash)
	}
	return strings.Join(lines, "\n")
}

func formatSourceConflicts(conflicts []research.Conflict) string {
	lines := []string{"Unresolved source conflicts"}
	if len(conflicts) == 0 {
		return strings.Join(append(lines, "No unresolved conflicts."), "\n")
	}
	for _, conflict := range conflicts {
		claims := make([]string, len(conflict.ClaimIDs))
		for index, claimID := range conflict.ClaimIDs {
			claims[index] = claimID.String()
		}
		lines = append(lines, fmt.Sprintf("- %s [%s] — claims %s — %s", conflict.ID, conflict.Type, strings.Join(claims, ", "), conflict.Reason))
	}
	return strings.Join(lines, "\n")
}

func formatResearchCostStats(stats researchapp.ResearchCostStats) string {
	return strings.Join([]string{
		"Research cost stats",
		"Algorithm: " + stats.AlgorithmVersion,
		fmt.Sprintf("Runs: %d (%d stopped by budget)", stats.Runs, stats.BudgetStoppedRuns),
		fmt.Sprintf("Used: %d searches, %d fetches, %d bytes, %d provider API calls, %d model calls", stats.Used.SearchRequests, stats.Used.FetchRequests, stats.Used.Bytes, stats.Used.ProviderAPICalls, stats.Used.ModelCalls),
		fmt.Sprintf("Today: %d searches, %d fetches, %d bytes, %d provider API calls, %d model calls", stats.TodayUsed.SearchRequests, stats.TodayUsed.FetchRequests, stats.TodayUsed.Bytes, stats.TodayUsed.ProviderAPICalls, stats.TodayUsed.ModelCalls),
		fmt.Sprintf("Saved by valid cache: %d searches, %d fetches, %d bytes, %d provider API calls, %d model calls", stats.CacheSavings.SearchRequests, stats.CacheSavings.FetchRequests, stats.CacheSavings.Bytes, stats.CacheSavings.ProviderAPICalls, stats.CacheSavings.ModelCalls),
	}, "\n")
}

func formatUpdateScan(scan research.UpdateScan) string {
	status := "complete"
	if !scan.Complete() {
		reasons := make([]string, len(scan.IncompleteReasons))
		for index, reason := range scan.IncompleteReasons {
			reasons[index] = string(reason)
		}
		status = "incomplete (" + strings.Join(reasons, ", ") + ")"
	}
	lines := []string{
		"Research update scan",
		"Status: " + status,
		"Scanned: " + scan.ScannedAt.Time().Format(time.RFC3339),
		fmt.Sprintf("Inventory: %d technologies, %d releases, %d tracked sources, %d freshness due",
			scan.Inventory.KnownTechnologies, scan.Inventory.KnownReleases,
			scan.Inventory.TrackedSources, scan.Inventory.FreshnessDue),
		fmt.Sprintf("Signals: %d", len(scan.Signals)),
	}
	for _, signal := range scan.Signals {
		lines = append(lines, fmt.Sprintf("- %s: %s — %s [%s]", signal.Type, signal.Reference, signal.Detail, signal.Origin))
	}
	if len(scan.Signals) == 0 {
		lines = append(lines, "No stored change signals found.")
	}
	lines = append(lines, "Algorithm: "+scan.AlgorithmVersion, "This report does not modify curriculum or student state.")
	return strings.Join(lines, "\n")
}

func formatResearchCacheStatus(status researchapp.ResearchCacheStatus) string {
	lines := []string{
		"Research cache",
		"Algorithm: " + status.AlgorithmVersion,
		fmt.Sprintf("Total: %d entries, %d bytes, %d stale", status.TotalEntries, status.TotalPayloadBytes, status.StaleEntries),
		fmt.Sprintf("Corrupt: %d entries, %d bytes", status.CorruptEntries, status.CorruptBytes),
	}
	for _, layer := range status.Layers {
		lines = append(lines, fmt.Sprintf("- %s: %d entries, %d bytes, %d stale", layer.Layer, layer.Entries, layer.PayloadBytes, layer.StaleEntries))
	}
	return strings.Join(lines, "\n")
}

func formatResearchCacheClear(result researchapp.ResearchCacheClearResult) string {
	return fmt.Sprintf("Research cache cleared\nRemoved: %d entries, %d bytes\nPersisted snapshots and evidence were not modified.", result.RemovedEntries, result.RemovedBytes)
}

func formatStaleSources(records []researchapp.FreshnessRecord) string {
	lines := []string{"Sources and claims due for reverification"}
	if len(records) == 0 {
		return strings.Join(append(lines, "Nothing is currently due."), "\n")
	}
	for _, record := range records {
		lines = append(lines, fmt.Sprintf("- %s [%s] — %s — due %s — last verified %s",
			record.SubjectID, record.Priority, record.VerificationReason,
			record.NextVerifyAt.Time().Format(time.RFC3339), record.LastVerifiedAt.Time().Format(time.RFC3339)))
	}
	return strings.Join(lines, "\n")
}

func formatSourceRegistryEntries(entries []research.SourceRegistryEntry) string {
	lines := []string{"Trusted source registry"}
	if len(entries) == 0 {
		return strings.Join(append(lines, "No registry entries."), "\n")
	}
	for _, entry := range entries {
		domains := make([]string, len(entry.CanonicalDomains))
		for index, domain := range entry.CanonicalDomains {
			domains[index] = domain.String()
		}
		lines = append(lines, fmt.Sprintf("- %s [%s] — %s — %s", entry.ID, entry.Status, entry.Organization, strings.Join(domains, ", ")))
	}
	return strings.Join(lines, "\n")
}

func formatSourceRegistryEntry(entry research.SourceRegistryEntry) string {
	domains := make([]string, len(entry.CanonicalDomains))
	for index, domain := range entry.CanonicalDomains {
		domains[index] = domain.String()
	}
	kinds := make([]string, len(entry.SourceKinds))
	for index, kind := range entry.SourceKinds {
		kinds[index] = string(kind)
	}
	lines := []string{
		"Trusted source registry entry",
		"ID: " + entry.ID.String(),
		"Organization: " + entry.Organization,
		"Status: " + string(entry.Status),
		"Canonical domains: " + strings.Join(domains, ", "),
		"Source kinds: " + strings.Join(kinds, ", "),
		"Research domains: " + strings.Join(entry.ResearchDomains, ", "),
		"Topic patterns: " + strings.Join(entry.TopicPatterns, ", "),
		"Added: " + entry.AddedAt.Time().Format(time.RFC3339),
		"Last reviewed: " + entry.LastReviewedAt.Time().Format(time.RFC3339),
	}
	for _, hint := range entry.AuthorityHints {
		lines = append(lines, fmt.Sprintf("Authority hint: %s → tier %s — %s", hint.SourceKind, hint.Tier, hint.Reason))
	}
	if entry.Notes != "" {
		lines = append(lines, "Notes: "+entry.Notes)
	}
	lines = append(lines, "Registry metadata is contextual input, not evidence or an automatic trust decision.")
	return strings.Join(lines, "\n")
}

func parseMistakeArguments(result *invocation) error {
	if len(result.arguments) == 0 {
		result.mistakeOperation = "list"
		return nil
	}
	if len(result.arguments) != 2 || result.arguments[0] != "show" {
		return fmt.Errorf("mistakes accepts no arguments or show <id>")
	}
	id, err := learning.NewID(result.arguments[1])
	if err != nil {
		return fmt.Errorf("mistakes show requires a valid id: %w", err)
	}
	result.mistakeOperation = "show"
	result.mistakeID = id
	return nil
}

func parseSessionArguments(result *invocation) error {
	if len(result.arguments) != 1 || (result.arguments[0] != "status" && result.arguments[0] != "stop") {
		return fmt.Errorf("session requires status or stop")
	}
	result.sessionOperation = result.arguments[0]
	return nil
}

func parseSetupArguments(result *invocation) error {
	if len(result.arguments) != 1 || (result.arguments[0] != "status" && result.arguments[0] != "reset") {
		return fmt.Errorf("setup requires status or reset")
	}
	result.setupOperation = result.arguments[0]
	return nil
}

func parseMasteryArguments(result *invocation) error {
	if len(result.arguments) == 0 {
		result.masteryOperation = "show"
		return nil
	}
	if len(result.arguments) == 1 && result.arguments[0] == "threshold" {
		result.masteryOperation = "show"
		return nil
	}
	if len(result.arguments) == 2 && result.arguments[0] == "threshold" && result.arguments[1] == "reset" {
		result.masteryOperation = "reset"
		return nil
	}
	if len(result.arguments) == 3 && result.arguments[0] == "threshold" && (result.arguments[1] == "set" || result.arguments[1] == "set-default") {
		percentage, err := strconv.Atoi(result.arguments[2])
		if err != nil || percentage < 50 || percentage > 99 {
			return fmt.Errorf("mastery threshold percentage must be an integer from 50 to 99")
		}
		requirement, err := learning.NewMasteryRequirement(float64(percentage) / 100)
		if err != nil {
			return fmt.Errorf("mastery threshold percentage must be an integer from 50 to 99")
		}
		result.masteryOperation = result.arguments[1]
		result.masteryThreshold = requirement.Threshold
		return nil
	}
	return fmt.Errorf("mastery requires threshold, optionally followed by set PERCENT, set-default PERCENT, or reset")
}

func parseGoalArguments(result *invocation) error {
	if len(result.arguments) == 0 {
		result.goalOperation = "show"
		return nil
	}
	if len(result.arguments) != 1 {
		return fmt.Errorf("goal requires show, set, pause, or resume")
	}
	result.goalOperation = result.arguments[0]
	switch result.goalOperation {
	case "show", "pause", "resume":
		return nil
	case "set":
		if strings.TrimSpace(result.goalInput.Title) == "" || strings.TrimSpace(result.goalInput.Domain) == "" || strings.TrimSpace(result.goalInput.TargetOutcome) == "" {
			return fmt.Errorf("goal set requires --title, --domain, and --target-outcome")
		}
		if !result.goalInput.StartingLevel.Valid() {
			return fmt.Errorf("option --starting-level requires novice, beginner, intermediate, or advanced")
		}
		return nil
	default:
		return fmt.Errorf("goal requires show, set, pause, or resume")
	}
}

func parseProfileArguments(result *invocation) error {
	if len(result.arguments) == 0 {
		result.profileOperation = "show"
		return nil
	}
	if len(result.arguments) != 1 || (result.arguments[0] != "show" && result.arguments[0] != "edit") {
		return fmt.Errorf("profile requires show or edit")
	}
	result.profileOperation = result.arguments[0]
	if result.profileOperation == "edit" && !result.profileFlagsSet {
		return fmt.Errorf("profile edit requires at least one profile option")
	}
	return nil
}

func integerOption(args []string, index int, name, argument string) (int, int, error) {
	valueText := ""
	if argument == name {
		index++
		if index >= len(args) {
			return 0, index, fmt.Errorf("option %s requires an integer", name)
		}
		valueText = args[index]
	} else {
		valueText = strings.TrimPrefix(argument, name+"=")
	}
	value, err := strconv.Atoi(valueText)
	if err != nil {
		return 0, index, fmt.Errorf("option %s requires an integer", name)
	}
	return value, index, nil
}

func floatOption(args []string, index int, name, argument string) (float64, int, error) {
	valueText := ""
	if argument == name {
		index++
		if index >= len(args) {
			return 0, index, fmt.Errorf("option %s requires a number", name)
		}
		valueText = args[index]
	} else {
		valueText = strings.TrimPrefix(argument, name+"=")
	}
	value, err := strconv.ParseFloat(valueText, 64)
	if err != nil {
		return 0, index, fmt.Errorf("option %s requires a number", name)
	}
	return value, index, nil
}

func textOption(args []string, index int, name, argument string, allowEmpty bool) (string, int, error) {
	value := ""
	if argument == name {
		index++
		if index >= len(args) {
			return "", index, fmt.Errorf("option %s requires a value", name)
		}
		value = args[index]
	} else {
		value = strings.TrimPrefix(argument, name+"=")
	}
	value = strings.TrimSpace(value)
	if value == "" && !allowEmpty {
		return "", index, fmt.Errorf("option %s requires a value", name)
	}
	return value, index, nil
}

func parseStudyPreferences(value string) []learning.StudyPreference {
	if strings.TrimSpace(value) == "" {
		return []learning.StudyPreference{}
	}
	parts := strings.Split(value, ",")
	preferences := make([]learning.StudyPreference, 0, len(parts))
	for _, part := range parts {
		preferences = append(preferences, learning.StudyPreference(strings.TrimSpace(part)))
	}
	return preferences
}

func parseUpdateArguments(result *invocation) error {
	if len(result.arguments) == 0 {
		result.updateOperation = "install"
		return nil
	}
	if len(result.arguments) == 1 && result.arguments[0] == "check" {
		result.updateOperation = "check"
		return nil
	}
	return fmt.Errorf("update accepts only the optional check command")
}

func parseBackupArguments(result *invocation) error {
	if len(result.arguments) == 0 {
		return fmt.Errorf("backup requires create, list, or restore")
	}
	result.backupOperation = result.arguments[0]
	switch result.backupOperation {
	case "create", "list":
		if len(result.arguments) != 1 {
			return fmt.Errorf("backup %s does not accept arguments", result.backupOperation)
		}
	case "restore":
		if len(result.arguments) != 2 {
			return fmt.Errorf("backup restore requires exactly one id")
		}
		result.backupID = result.arguments[1]
	default:
		return fmt.Errorf("unknown backup command %q", result.backupOperation)
	}
	return nil
}

func parseLogArguments(result *invocation) error {
	if len(result.arguments) != 1 || result.arguments[0] != "path" {
		return fmt.Errorf("logs requires the path command")
	}
	result.logOperation = "path"
	return nil
}

func parseOpenArguments(result *invocation) error {
	if len(result.arguments) == 0 {
		return nil
	}
	if len(result.arguments) != 1 || result.arguments[0] != "roadmap" {
		return fmt.Errorf("open accepts only the optional roadmap artifact")
	}
	result.openTarget = "roadmap"
	return nil
}

func parseSecretArguments(result *invocation) error {
	if len(result.arguments) == 0 {
		return fmt.Errorf("secrets requires status, set, or delete")
	}
	result.secretOperation = result.arguments[0]
	switch result.secretOperation {
	case "status":
		if len(result.arguments) != 1 {
			return fmt.Errorf("secrets status does not accept arguments")
		}
	case "set", "delete":
		if len(result.arguments) != 2 {
			return fmt.Errorf("secrets %s requires exactly one name", result.secretOperation)
		}
		result.secretName = result.arguments[1]
	default:
		return fmt.Errorf("unknown secrets command %q", result.secretOperation)
	}
	return nil
}

func parseConfigArguments(result *invocation) error {
	if len(result.arguments) == 0 {
		result.configOperation = "show"
		return nil
	}
	result.configOperation = result.arguments[0]
	switch result.configOperation {
	case "show", "path":
		if len(result.arguments) != 1 {
			return fmt.Errorf("config %s does not accept arguments", result.configOperation)
		}
	case "get":
		if len(result.arguments) != 2 {
			return fmt.Errorf("config get requires exactly one key")
		}
		result.configKey = result.arguments[1]
	case "set":
		if len(result.arguments) != 3 {
			return fmt.Errorf("config set requires a key and value")
		}
		result.configKey = result.arguments[1]
		result.configValue = result.arguments[2]
	default:
		return fmt.Errorf("unknown config command %q", result.configOperation)
	}
	return nil
}

func (r Runner) usageError(format string, args ...any) int {
	fmt.Fprintf(r.stderr, "kelyro: "+format+"\n", args...)
	fmt.Fprintln(r.stderr, "Run 'kelyro help' for usage.")
	return ExitUsage
}
