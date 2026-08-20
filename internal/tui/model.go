package tui

import (
	"context"
	"fmt"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/mishaaac/kelyro/internal/app"
	"github.com/mishaaac/kelyro/internal/config"
	"github.com/mishaaac/kelyro/internal/learning"
	learningapp "github.com/mishaaac/kelyro/internal/learning/application"
	"github.com/mishaaac/kelyro/internal/session"
)

type screen uint8

const (
	screenHome screen = iota
	screenDoctor
	screenConfig
	screenRoadmap
	screenProfile
	screenOnboarding
)

// Model contains terminal-only state. It never discovers a workspace or reads
// configuration directly; asynchronous commands call the application service.
type Model struct {
	ctx               context.Context
	service           Service
	command           app.Command
	snapshot          app.FoundationSnapshot
	screen            screen
	width             int
	height            int
	loading           bool
	saving            bool
	opening           bool
	profile           learning.Student
	profileLoading    bool
	profileErr        error
	onboarding        learningapp.OnboardingView
	setup             learningapp.LearnerSetupView
	onboardingLoading bool
	onboardingErr     error
	onboardingInput   string
	onboardingCursor  int
	diagnosticAnswers map[int]bool
	loadErr           error
	notice            string
	configCursor      int
	session           session.State
	sessionReady      bool
	checkpointing     bool
	checkpointPending bool
	quitting          bool
	forceNoColor      bool
	styles            styles
}

// NewModel creates the initial loading model. forceNoColor represents either
// --no-color or NO_COLOR and cannot be overridden by stored configuration.
func NewModel(ctx context.Context, service Service, command app.Command, forceNoColor bool) Model {
	if ctx == nil {
		ctx = context.Background()
	}
	return Model{
		ctx:          ctx,
		service:      service,
		command:      command,
		width:        80,
		height:       24,
		loading:      true,
		session:      session.Default(),
		forceNoColor: forceNoColor,
		styles:       newStyles(!forceNoColor),
	}
}

func (model Model) Init() tea.Cmd {
	return initializeFoundationCmd(model.ctx, model.service, model.command)
}

func (model Model) Update(message tea.Msg) (tea.Model, tea.Cmd) {
	switch message := message.(type) {
	case tea.WindowSizeMsg:
		model.width = message.Width
		model.height = message.Height
		return model, nil
	case foundationLoadedMsg:
		model.snapshot = message.snapshot
		model.loading = false
		model.loadErr = nil
		model.notice = ""
		model.applyConfiguredColor()
		return model, nil
	case foundationInitializedMsg:
		model.snapshot = message.snapshot
		model.loading = false
		model.loadErr = nil
		model.notice = ""
		model.applyConfiguredColor()
		if message.sessionErr != nil {
			model.notice = "Session state unavailable; continuing with defaults: " + message.sessionErr.Error()
			if message.resume.State.Version == 0 {
				if model.quitting {
					return model, tea.Quit
				}
				return model, nil
			}
		}
		model.sessionReady = true
		model.session = message.resume.State.Clone()
		if model.session.SafeToResume {
			model.screen = screenFromSession(model.session.LastView)
		}
		switch {
		case message.resume.Recovered && message.resume.PreviousIncomplete:
			model.notice = "Recovered invalid session state and detected an incomplete previous session."
		case message.resume.Recovered:
			model.notice = "Recovered invalid session state using safe defaults."
		case message.resume.PreviousIncomplete:
			model.notice = "Resumed after an incomplete previous session."
		case message.resume.MigratedFrom != 0:
			model.notice = fmt.Sprintf("Session state upgraded from version %d.", message.resume.MigratedFrom)
		}
		if model.quitting {
			return model, completeSessionCmd(model.ctx, model.service, model.command, model.session)
		}
		if model.screen == screenProfile {
			model.profileLoading = true
			return model, loadProfileCmd(model.ctx, model.service, model.command)
		}
		if !model.snapshot.LearningPath {
			model.screen = screenOnboarding
			model.onboardingLoading = true
			return model, onboardingCmd(model.ctx, model.service, model.command, "start")
		}
		if model.screen == screenOnboarding {
			model.onboardingLoading = true
			return model, onboardingCmd(model.ctx, model.service, model.command, "start")
		}
		return model, nil
	case foundationLoadFailedMsg:
		model.loading = false
		model.loadErr = message.err
		if model.quitting {
			return model, tea.Quit
		}
		return model, nil
	case configSavedMsg:
		model.snapshot.Settings = cloneSettings(model.snapshot.Settings)
		model.snapshot.Settings[message.key] = message.value
		model.saving = false
		model.notice = message.message
		model.applyConfiguredColor()
		model.session.LastCommand = "config set " + message.key
		model.session.SetupFlags["configuration_touched"] = true
		return model.queueCheckpoint()
	case configSaveFailedMsg:
		model.saving = false
		model.notice = "Could not save configuration: " + message.err.Error()
		return model, nil
	case roadmapOpenedMsg:
		model.opening = false
		model.notice = message.message
		model.session.LastArtifact = "00-roadmap/ROADMAP.md"
		model.session.LastCommand = "open roadmap"
		return model.queueCheckpoint()
	case roadmapOpenFailedMsg:
		model.opening = false
		model.notice = "Could not open roadmap: " + message.err.Error()
		return model, nil
	case profileLoadedMsg:
		model.profile = message.student
		model.profileLoading = false
		model.profileErr = nil
		model.notice = ""
		if model.session.LastView != session.ViewProfile {
			model.session.LastView = session.ViewProfile
			return model.queueCheckpoint()
		}
		return model, nil
	case profileLoadFailedMsg:
		model.profileLoading = false
		model.profileErr = message.err
		if model.session.LastView != session.ViewProfile {
			model.session.LastView = session.ViewProfile
			return model.queueCheckpoint()
		}
		return model, nil
	case onboardingLoadedMsg:
		model.setup = message.view
		if message.view.Onboarding != nil {
			model.onboarding = *message.view.Onboarding
		}
		model.onboardingLoading = false
		model.onboardingErr = nil
		model.onboardingInput = ""
		model.onboardingCursor = 0
		model.diagnosticAnswers = make(map[int]bool)
		model.prepareOnboardingQuestion()
		if message.view.Setup.Status == learning.SetupCompleted {
			model.snapshot.LearningPath = true
		}
		if model.session.LastView != session.ViewOnboarding {
			model.session.LastView = session.ViewOnboarding
			return model.queueCheckpoint()
		}
		return model, nil
	case onboardingFailedMsg:
		model.onboardingLoading = false
		model.onboardingErr = message.err
		return model, nil
	case sessionCheckpointedMsg:
		return model.checkpointFinished(nil)
	case sessionCheckpointFailedMsg:
		return model.checkpointFinished(message.err)
	case sessionCompletedMsg, sessionCompleteFailedMsg:
		return model, tea.Quit
	}

	key, ok := message.(tea.KeyMsg)
	if !ok {
		return model, nil
	}
	keyName := key.String()
	if keyName == "ctrl+c" || (keyName == "q" && model.screen != screenOnboarding) {
		return model.beginQuit()
	}
	if model.loading {
		return model, nil
	}
	if model.loadErr != nil {
		if keyName == "r" || keyName == "enter" {
			model.loading = true
			model.loadErr = nil
			return model, loadFoundationCmd(model.ctx, model.service, model.command)
		}
		return model, nil
	}

	if model.screen != screenOnboarding && (keyName == "esc" || keyName == "backspace" || keyName == "h") {
		changed := model.screen != screenHome
		model.screen = screenHome
		model.notice = ""
		if changed {
			model.session.LastView = session.ViewHome
			return model.queueCheckpoint()
		}
		return model, nil
	}

	switch model.screen {
	case screenHome:
		previous := model.screen
		switch keyName {
		case "enter", "r":
			model.screen = screenRoadmap
		case "d":
			model.screen = screenDoctor
		case "c":
			model.screen = screenConfig
		case "p":
			model.screen = screenProfile
			model.profileLoading = true
			model.profileErr = nil
			return model, loadProfileCmd(model.ctx, model.service, model.command)
		case "s":
			model.screen = screenOnboarding
			model.onboardingLoading = true
			model.onboardingErr = nil
			return model, onboardingCmd(model.ctx, model.service, model.command, "start")
		}
		if model.screen != previous {
			model.session.LastView = sessionView(model.screen)
			return model.queueCheckpoint()
		}
	case screenDoctor:
		if keyName == "r" {
			model.loading = true
			return model, loadFoundationCmd(model.ctx, model.service, model.command)
		}
	case screenConfig:
		return model.updateConfig(keyName)
	case screenRoadmap:
		if keyName == "o" && !model.opening {
			model.opening = true
			model.notice = "Opening roadmap..."
			return model, openRoadmapCmd(model.ctx, model.service, model.command)
		}
	case screenProfile:
		if keyName == "r" && !model.profileLoading {
			model.profileLoading = true
			model.profileErr = nil
			return model, loadProfileCmd(model.ctx, model.service, model.command)
		}
	case screenOnboarding:
		return model.updateOnboarding(key)
	}
	return model, nil
}

func (model Model) queueCheckpoint() (tea.Model, tea.Cmd) {
	if !model.sessionReady || model.quitting {
		return model, nil
	}
	if model.checkpointing {
		model.checkpointPending = true
		return model, nil
	}
	model.checkpointing = true
	return model, checkpointSessionCmd(model.ctx, model.service, model.command, model.session)
}

func (model Model) checkpointFinished(err error) (tea.Model, tea.Cmd) {
	model.checkpointing = false
	if err != nil && !model.quitting {
		model.notice = "Could not save session state: " + err.Error()
	}
	if model.quitting {
		model.checkpointPending = false
		return model, completeSessionCmd(model.ctx, model.service, model.command, model.session)
	}
	if model.checkpointPending {
		model.checkpointPending = false
		model.checkpointing = true
		return model, checkpointSessionCmd(model.ctx, model.service, model.command, model.session)
	}
	return model, nil
}

func (model Model) beginQuit() (tea.Model, tea.Cmd) {
	if model.quitting {
		return model, nil
	}
	if !model.sessionReady {
		if model.loading {
			model.quitting = true
			return model, nil
		}
		return model, tea.Quit
	}
	model.quitting = true
	model.checkpointPending = false
	if model.checkpointing {
		return model, nil
	}
	return model, completeSessionCmd(model.ctx, model.service, model.command, model.session)
}

func sessionView(current screen) session.View {
	switch current {
	case screenDoctor:
		return session.ViewDoctor
	case screenConfig:
		return session.ViewConfig
	case screenRoadmap:
		return session.ViewRoadmap
	case screenProfile:
		return session.ViewProfile
	case screenOnboarding:
		return session.ViewOnboarding
	default:
		return session.ViewHome
	}
}

func screenFromSession(view session.View) screen {
	switch view {
	case session.ViewDoctor:
		return screenDoctor
	case session.ViewConfig:
		return screenConfig
	case session.ViewRoadmap:
		return screenRoadmap
	case session.ViewProfile:
		return screenProfile
	case session.ViewOnboarding:
		return screenOnboarding
	default:
		return screenHome
	}
}

func (model Model) updateOnboarding(key tea.KeyMsg) (tea.Model, tea.Cmd) {
	keyName := key.String()
	if keyName == "esc" {
		model.screen = screenHome
		model.onboardingErr = nil
		model.session.LastView = session.ViewHome
		return model.queueCheckpoint()
	}
	if model.onboardingLoading {
		return model, nil
	}
	if model.setup.Setup.Status == learning.SetupCompleted {
		if keyName == "enter" || keyName == "esc" {
			model.screen = screenHome
			model.session.LastView = session.ViewHome
			return model.queueCheckpoint()
		}
		return model, nil
	}
	if model.setup.Diagnostic != nil {
		return model.updateDiagnostic(key)
	}
	if keyName == "ctrl+x" && model.onboarding.Interview.Status == learning.OnboardingInProgress {
		model.onboardingLoading = true
		return model, onboardingCmd(model.ctx, model.service, model.command, "cancel")
	}
	if keyName == "ctrl+b" && model.onboarding.Interview.Status == learning.OnboardingInProgress {
		model.onboardingLoading = true
		return model, onboardingCmd(model.ctx, model.service, model.command, "back")
	}
	if model.onboarding.Interview.Status == learning.OnboardingCompleted || model.onboarding.Interview.Status == learning.OnboardingCancelled {
		if keyName == "enter" {
			model.screen = screenHome
			model.session.LastView = session.ViewHome
			return model.queueCheckpoint()
		}
		return model, nil
	}
	question := model.onboarding.Question
	switch question.Kind {
	case learning.OnboardingChoiceQuestion:
		switch keyName {
		case "up", "k":
			if model.onboardingCursor > 0 {
				model.onboardingCursor--
			}
		case "down", "j":
			if model.onboardingCursor < len(question.Options)-1 {
				model.onboardingCursor++
			}
		case "enter":
			if len(question.Options) > 0 {
				model.onboardingLoading = true
				return model, onboardingCmd(model.ctx, model.service, model.command, "onboarding-submit", question.Options[model.onboardingCursor].Value)
			}
		}
	case learning.OnboardingTextQuestion:
		switch key.Type {
		case tea.KeyRunes:
			model.onboardingInput += string(key.Runes)
		case tea.KeySpace:
			model.onboardingInput += " "
		case tea.KeyBackspace, tea.KeyDelete:
			runes := []rune(model.onboardingInput)
			if len(runes) > 0 {
				model.onboardingInput = string(runes[:len(runes)-1])
			}
		case tea.KeyEnter:
			model.onboardingLoading = true
			return model, onboardingCmd(model.ctx, model.service, model.command, "onboarding-submit", model.onboardingInput)
		}
	case learning.OnboardingReviewQuestion:
		if keyName == "enter" {
			model.onboardingLoading = true
			return model, onboardingCmd(model.ctx, model.service, model.command, "onboarding-submit", "")
		}
	case learning.OnboardingConfirmQuestion:
		if keyName == "enter" {
			model.onboardingLoading = true
			return model, onboardingCmd(model.ctx, model.service, model.command, "confirm")
		}
	}
	return model, nil
}

func (model Model) updateDiagnostic(key tea.KeyMsg) (tea.Model, tea.Cmd) {
	keyName := key.String()
	if keyName == "x" {
		model.onboardingLoading = true
		return model, onboardingCmd(model.ctx, model.service, model.command, "diagnostic-skip")
	}
	item := model.setup.Diagnostic.Item
	if item == nil {
		return model, nil
	}
	switch item.Kind {
	case learning.DiagnosticSingleChoice, learning.DiagnosticSelfReport:
		switch keyName {
		case "up", "k":
			if model.onboardingCursor > 0 {
				model.onboardingCursor--
			}
		case "down", "j":
			if model.onboardingCursor < len(item.Options)-1 {
				model.onboardingCursor++
			}
		case "enter":
			if len(item.Options) > 0 {
				model.onboardingLoading = true
				return model, onboardingCmd(model.ctx, model.service, model.command, "diagnostic-submit", item.Options[model.onboardingCursor].Value)
			}
		}
	case learning.DiagnosticMultipleChoice:
		switch keyName {
		case "up", "k":
			if model.onboardingCursor > 0 {
				model.onboardingCursor--
			}
		case "down", "j":
			if model.onboardingCursor < len(item.Options)-1 {
				model.onboardingCursor++
			}
		case " ", "space":
			model.diagnosticAnswers[model.onboardingCursor] = !model.diagnosticAnswers[model.onboardingCursor]
		case "enter":
			answers := make([]string, 0, len(model.diagnosticAnswers))
			for index, option := range item.Options {
				if model.diagnosticAnswers[index] {
					answers = append(answers, option.Value)
				}
			}
			if len(answers) > 0 {
				model.onboardingLoading = true
				return model, onboardingCmd(model.ctx, model.service, model.command, "diagnostic-submit", answers...)
			}
		}
	case learning.DiagnosticShortAnswer:
		switch key.Type {
		case tea.KeyRunes:
			model.onboardingInput += string(key.Runes)
		case tea.KeySpace:
			model.onboardingInput += " "
		case tea.KeyBackspace, tea.KeyDelete:
			runes := []rune(model.onboardingInput)
			if len(runes) > 0 {
				model.onboardingInput = string(runes[:len(runes)-1])
			}
		case tea.KeyEnter:
			if model.onboardingInput != "" {
				model.onboardingLoading = true
				return model, onboardingCmd(model.ctx, model.service, model.command, "diagnostic-submit", model.onboardingInput)
			}
		}
	}
	return model, nil
}

func (model *Model) prepareOnboardingQuestion() {
	question := model.onboarding.Question
	if question.Kind == learning.OnboardingTextQuestion {
		model.onboardingInput = model.onboarding.Interview.Answers[question.ID]
	}
	if question.Kind == learning.OnboardingChoiceQuestion {
		selected := model.onboarding.Interview.Answers[question.ID]
		for index, option := range question.Options {
			if option.Value == selected {
				model.onboardingCursor = index
				break
			}
		}
	}
}

func (model Model) updateConfig(key string) (tea.Model, tea.Cmd) {
	keys := config.CommonKeys()
	switch key {
	case "up", "k":
		if model.configCursor > 0 {
			model.configCursor--
		}
	case "down", "j":
		if model.configCursor < len(keys)-1 {
			model.configCursor++
		}
	case "enter", " ":
		if model.saving {
			return model, nil
		}
		selected := keys[model.configCursor]
		value, editable := nextConfigValue(selected, model.snapshot.Settings[selected])
		if !editable {
			model.notice = fmt.Sprintf("%s is read-only in this minimal wizard; use `kelyro config set`.", selected)
			return model, nil
		}
		model.saving = true
		model.notice = "Saving configuration..."
		return model, saveConfigCmd(model.ctx, model.service, model.command, selected, value)
	}
	return model, nil
}

func nextConfigValue(key string, current config.Value) (config.Value, bool) {
	switch key {
	case config.KeyUIColor:
		values := []string{"auto", "always", "never"}
		for index, value := range values {
			if current.String() == value {
				return config.StringValue(values[(index+1)%len(values)]), true
			}
		}
		return config.StringValue("auto"), true
	case config.KeyEditorPrompt, config.KeyAllowNetwork, config.KeyUpdateCheck:
		value, ok := current.BoolField()
		if !ok {
			return config.Value{}, false
		}
		return config.BoolValue(!value), true
	default:
		return config.Value{}, false
	}
}

func (model *Model) applyConfiguredColor() {
	color := "auto"
	if value, ok := model.snapshot.Settings[config.KeyUIColor]; ok {
		color = value.String()
	}
	model.styles = newStyles(!model.forceNoColor && color != "never")
}

func cloneSettings(source config.Settings) config.Settings {
	result := make(config.Settings, len(source))
	for key, value := range source {
		result[key] = value
	}
	return result
}
