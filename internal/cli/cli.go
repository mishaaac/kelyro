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
	"github.com/mishaaac/kelyro/internal/doctor"
	"github.com/mishaaac/kelyro/internal/learning"
	learningapp "github.com/mishaaac/kelyro/internal/learning/application"
	"github.com/mishaaac/kelyro/internal/portability"
	"github.com/mishaaac/kelyro/internal/update"
	"github.com/mishaaac/kelyro/internal/version"
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

Options:
  -h, --help          Show this help message
      --version       Show build version information
      --no-color      Disable colored output
      --verbose       Enable verbose diagnostic logging
      --quiet         Suppress successful command output
      --workspace PATH  Override workspace discovery
      --allow-nested  Confirm initialization inside another workspace
      --yes           Confirm backup restore or development setup reset
      --full          Include allowlisted machine state in an export
      --output FILE   Set the export archive path
      --dry-run       Validate and preview an import without writing
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
`

var actions = map[string]app.Action{
	"init":     app.ActionInit,
	"doctor":   app.ActionDoctor,
	"config":   app.ActionConfig,
	"secrets":  app.ActionSecrets,
	"status":   app.ActionStatus,
	"progress": app.ActionProgress,
	"roadmap":  app.ActionRoadmap,
	"today":    app.ActionToday,
	"open":     app.ActionOpen,
	"logs":     app.ActionLogs,
	"audit":    app.ActionAudit,
	"backup":   app.ActionBackup,
	"export":   app.ActionExport,
	"import":   app.ActionImport,
	"update":   app.ActionUpdate,
	"profile":  app.ActionProfile,
	"goal":     app.ActionGoal,
	"mastery":  app.ActionMastery,
	"setup":    app.ActionSetup,
	"mistakes": app.ActionMistakes,
	"session":  app.ActionSession,
	"history":  app.ActionHistory,
	"time":     app.ActionTime,
	"reviews":  app.ActionReviews,
	"streak":   app.ActionStreak,
}

// Runner owns CLI parsing and rendering while delegating operations to an
// application service.
type Runner struct {
	service     app.FoundationService
	stdout      io.Writer
	stderr      io.Writer
	secrets     SecretReader
	interactive InteractiveRunner
	confirmer   Confirmer
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
		Action:            action,
		Workspace:         invocation.workspace,
		AllowNested:       invocation.allowNested,
		ConfigScope:       invocation.configScope,
		OpenTarget:        invocation.openTarget,
		DoctorExplain:     invocation.doctorExplain,
		LogOperation:      invocation.logOperation,
		BackupOperation:   invocation.backupOperation,
		BackupID:          invocation.backupID,
		ExportMode:        invocation.exportMode,
		ExportOutput:      invocation.exportOutput,
		ImportArchive:     invocation.importArchive,
		ImportDryRun:      invocation.importDryRun,
		ImportConflicts:   invocation.importConflicts,
		UpdateOperation:   invocation.updateOperation,
		ProfileOperation:  invocation.profileOperation,
		ProfileChanges:    invocation.profileChanges,
		GoalOperation:     invocation.goalOperation,
		GoalInput:         invocation.goalInput,
		MasteryOperation:  invocation.masteryOperation,
		MasteryThreshold:  invocation.masteryThreshold,
		SetupOperation:    invocation.setupOperation,
		MistakeOperation:  invocation.mistakeOperation,
		MistakeID:         invocation.mistakeID,
		SessionOperation:  invocation.sessionOperation,
		HistoryToday:      invocation.historyToday,
		ProgressOperation: invocation.progressOperation,
		ReviewsDue:        invocation.reviewsDue,
		Verbose:           invocation.verbose,
	}
	if invocation.noColor {
		command.ConfigOverrides = config.Settings{config.KeyUIColor: config.StringValue("never")}
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
	} else if !invocation.quiet && result.Message != "" {
		fmt.Fprintln(r.stdout, result.Message)
	}
	if result.Failed {
		return ExitFailure
	}

	return ExitOK
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
			if check.State != doctor.Pass && check.LearnMore != "" {
				lines = append(lines, "  Learn more: "+check.LearnMore)
			}
		}
	}
	return strings.Join(lines, "\n")
}

type invocation struct {
	command           string
	workspace         string
	help              bool
	version           bool
	noColor           bool
	verbose           bool
	quiet             bool
	allowNested       bool
	arguments         []string
	configScope       config.Scope
	configOperation   string
	configKey         string
	configValue       string
	secretOperation   string
	secretName        string
	openTarget        string
	doctorExplain     string
	logOperation      string
	backupOperation   string
	backupID          string
	yes               bool
	exportMode        portability.Mode
	exportOutput      string
	importArchive     string
	importDryRun      bool
	importConflicts   portability.ConflictStrategy
	conflictSet       bool
	updateOperation   string
	profileOperation  string
	profileChanges    learningapp.ProfileChanges
	profileFlagsSet   bool
	goalOperation     string
	goalInput         learningapp.SetGoalInput
	goalFlagsSet      bool
	masteryOperation  string
	masteryThreshold  learning.MasteryThreshold
	setupOperation    string
	mistakeOperation  string
	mistakeID         learning.ID
	sessionOperation  string
	historyToday      bool
	progressOperation string
	reviewsDue        bool
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
	if result.yes && result.command != "backup" && result.command != "setup" {
		return invocation{}, fmt.Errorf("option --yes requires backup restore or setup reset")
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
	default:
		if len(result.arguments) > 0 {
			return invocation{}, fmt.Errorf("unexpected argument %q", result.arguments[0])
		}
	}
	if result.yes && !((result.command == "backup" && result.backupOperation == "restore") || (result.command == "setup" && result.setupOperation == "reset")) {
		return invocation{}, fmt.Errorf("option --yes requires backup restore or setup reset")
	}
	if result.exportMode == portability.ModeFull && result.command != "export" {
		return invocation{}, fmt.Errorf("option --full requires the export command")
	}
	if result.exportOutput != "" && result.command != "export" {
		return invocation{}, fmt.Errorf("option --output requires the export command")
	}
	if result.importDryRun && result.command != "import" {
		return invocation{}, fmt.Errorf("option --dry-run requires the import command")
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
