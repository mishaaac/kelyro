package tui

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/mishaaac/kelyro/internal/app"
	"github.com/mishaaac/kelyro/internal/config"
	"github.com/mishaaac/kelyro/internal/session"
)

func TestModelLoadsFoundationStateThroughCommand(t *testing.T) {
	t.Parallel()

	snapshot := healthySnapshot()
	service := &fakeService{snapshot: snapshot}
	model := NewModel(context.Background(), service, app.Command{Workspace: "/requested"}, true)

	message := model.Init()()
	updated, command := model.Update(message)
	got := updated.(Model)
	if command != nil || got.loading || got.loadErr != nil {
		t.Fatalf("loaded model = %#v, command = %v", got, command)
	}
	if got.snapshot.WorkspaceName != snapshot.WorkspaceName {
		t.Errorf("workspace name = %q", got.snapshot.WorkspaceName)
	}
	if service.loadedCommand.Workspace != "/requested" {
		t.Errorf("load command workspace = %q", service.loadedCommand.Workspace)
	}
}

func TestModelNavigatesFoundationScreens(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		key  string
		want screen
	}{
		{name: "doctor", key: "d", want: screenDoctor},
		{name: "config", key: "c", want: screenConfig},
		{name: "roadmap", key: "r", want: screenRoadmap},
		{name: "continue", key: "enter", want: screenRoadmap},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			model := readyModel(&fakeService{})
			updated, _ := model.Update(tea.KeyMsg{Type: keyType(test.key), Runes: keyRunes(test.key)})
			if got := updated.(Model).screen; got != test.want {
				t.Errorf("screen = %d, want %d", got, test.want)
			}
		})
	}

	model := readyModel(&fakeService{})
	model.screen = screenDoctor
	updated, _ := model.Update(tea.KeyMsg{Type: tea.KeyEsc})
	if got := updated.(Model).screen; got != screenHome {
		t.Errorf("Esc screen = %d, want home", got)
	}
}

func TestModelHandlesResizeAndQuit(t *testing.T) {
	t.Parallel()

	model := readyModel(&fakeService{})
	updated, _ := model.Update(tea.WindowSizeMsg{Width: 34, Height: 12})
	resized := updated.(Model)
	if resized.width != 34 || resized.height != 12 {
		t.Errorf("terminal size = %dx%d", resized.width, resized.height)
	}

	_, command := resized.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'q'}})
	if command == nil {
		t.Fatal("q did not return a quit command")
	}
	if _, ok := command().(tea.QuitMsg); !ok {
		t.Errorf("q command message = %T, want tea.QuitMsg", command())
	}
}

func TestModelResumesViewAndPersistsOnlyMeaningfulTransitions(t *testing.T) {
	service := &fakeService{
		snapshot: healthySnapshot(),
		resume: session.Resume{State: session.State{
			Version:          session.CurrentVersion,
			LastView:         session.ViewConfig,
			SetupFlags:       map[string]bool{},
			SessionStartedAt: time.Date(2026, time.August, 12, 12, 0, 0, 0, time.UTC),
			SafeToResume:     true,
		}},
	}
	model := NewModel(context.Background(), service, app.Command{}, true)
	initialized, _ := model.Update(model.Init()())
	current := initialized.(Model)
	if current.screen != screenConfig || !current.sessionReady {
		t.Fatalf("resumed model = %#v", current)
	}

	unchanged, command := current.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'x'}})
	if command != nil || len(service.checkpoints) != 0 {
		t.Fatalf("ordinary key persisted state: command=%v checkpoints=%d", command, len(service.checkpoints))
	}

	home, command := unchanged.(Model).Update(tea.KeyMsg{Type: tea.KeyEsc})
	if command == nil || home.(Model).session.LastView != session.ViewHome {
		t.Fatal("meaningful view transition did not schedule a checkpoint")
	}
	message := command()
	settled, next := home.(Model).Update(message)
	if next != nil || settled.(Model).checkpointing || len(service.checkpoints) != 1 {
		t.Fatalf("checkpoint result: model=%#v next=%v writes=%d", settled, next, len(service.checkpoints))
	}
}

func TestModelNormalQuitAndCtrlCCompleteSession(t *testing.T) {
	for _, key := range []tea.KeyMsg{
		{Type: tea.KeyRunes, Runes: []rune{'q'}},
		{Type: tea.KeyCtrlC},
	} {
		service := &fakeService{snapshot: healthySnapshot()}
		model := NewModel(context.Background(), service, app.Command{}, true)
		initialized, _ := model.Update(model.Init()())

		quitting, command := initialized.(Model).Update(key)
		if command == nil || !quitting.(Model).quitting {
			t.Fatalf("key %q did not start graceful completion", key.String())
		}
		message := command()
		finished, quit := quitting.(Model).Update(message)
		if quit == nil || len(service.completed) != 1 {
			t.Fatalf("key %q completion = model %#v, quit %v, writes %d", key.String(), finished, quit, len(service.completed))
		}
		if _, ok := quit().(tea.QuitMsg); !ok {
			t.Fatalf("key %q final command = %T", key.String(), quit())
		}
	}
}

func TestModelSerializesCheckpointBeforeQuit(t *testing.T) {
	service := &fakeService{snapshot: healthySnapshot()}
	model := NewModel(context.Background(), service, app.Command{}, true)
	initialized, _ := model.Update(model.Init()())

	roadmap, checkpoint := initialized.(Model).Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'r'}})
	quitting, complete := roadmap.(Model).Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'q'}})
	if complete != nil || !quitting.(Model).quitting {
		t.Fatal("quit did not wait for the in-flight checkpoint")
	}

	checkpointMessage := checkpoint()
	afterCheckpoint, complete := quitting.(Model).Update(checkpointMessage)
	if complete == nil {
		t.Fatal("completion was not scheduled after checkpoint settled")
	}
	completedMessage := complete()
	_, quit := afterCheckpoint.(Model).Update(completedMessage)
	if quit == nil || len(service.checkpoints) != 1 || len(service.completed) != 1 {
		t.Fatalf("serialized writes: checkpoints=%d completed=%d quit=%v", len(service.checkpoints), len(service.completed), quit)
	}
	if service.completed[0].LastView != session.ViewRoadmap {
		t.Errorf("completed view = %q", service.completed[0].LastView)
	}
}

func TestModelWaitsForInitializationBeforeGracefulQuit(t *testing.T) {
	service := &fakeService{snapshot: healthySnapshot()}
	model := NewModel(context.Background(), service, app.Command{}, true)
	initialization := model.Init()

	waiting, command := model.Update(tea.KeyMsg{Type: tea.KeyCtrlC})
	if command != nil || !waiting.(Model).quitting {
		t.Fatal("Ctrl+C during initialization did not wait for session startup")
	}
	initializedMessage := initialization()
	initialized, complete := waiting.(Model).Update(initializedMessage)
	if complete == nil || !initialized.(Model).sessionReady {
		t.Fatal("session completion was not scheduled after initialization")
	}
	completedMessage := complete()
	_, quit := initialized.(Model).Update(completedMessage)
	if quit == nil || len(service.completed) != 1 {
		t.Fatalf("completion writes=%d quit=%v", len(service.completed), quit)
	}
}

func TestModelRetriesInitialError(t *testing.T) {
	t.Parallel()

	service := &fakeService{loadErr: errors.New("not ready")}
	model := NewModel(context.Background(), service, app.Command{}, true)
	failed, _ := model.Update(model.Init()())
	failedModel := failed.(Model)
	if failedModel.loadErr == nil || failedModel.loading {
		t.Fatalf("failed model = %#v", failedModel)
	}

	service.loadErr = nil
	service.snapshot = healthySnapshot()
	retrying, command := failedModel.Update(tea.KeyMsg{Type: tea.KeyEnter})
	if !retrying.(Model).loading || command == nil {
		t.Fatal("Enter did not retry failed load")
	}
	loaded, _ := retrying.(Model).Update(command())
	if loaded.(Model).loading || loaded.(Model).loadErr != nil {
		t.Fatalf("retried model = %#v", loaded)
	}
}

func TestConfigWizardReadsAndUpdatesSupportedValues(t *testing.T) {
	t.Parallel()

	service := &fakeService{result: app.Result{Message: "saved"}}
	model := readyModel(service)
	model.screen = screenConfig
	// Sorted keys place editor.prompt immediately after read-only editor.command.
	selected, _ := model.Update(tea.KeyMsg{Type: tea.KeyDown})
	saving, command := selected.(Model).Update(tea.KeyMsg{Type: tea.KeyEnter})
	if command == nil || !saving.(Model).saving {
		t.Fatal("editable config entry did not start a save")
	}

	saved, _ := saving.(Model).Update(command())
	got := saved.(Model)
	if got.saving || got.notice != "saved" {
		t.Errorf("saved state: saving=%v notice=%q", got.saving, got.notice)
	}
	if value, _ := got.snapshot.Settings[config.KeyEditorPrompt].BoolField(); value {
		t.Error("editor.prompt was not toggled to false")
	}
	if len(service.executed) != 1 {
		t.Fatalf("execute calls = %d", len(service.executed))
	}
	call := service.executed[0]
	if call.Action != app.ActionConfig || call.ConfigOperation != "set" || call.ConfigKey != config.KeyEditorPrompt || call.ConfigValue != "false" {
		t.Errorf("config command = %#v", call)
	}

	model = readyModel(service)
	model.screen = screenConfig
	readOnly, command := model.Update(tea.KeyMsg{Type: tea.KeyEnter})
	if command != nil || !strings.Contains(readOnly.(Model).notice, "read-only") {
		t.Errorf("read-only selection command = %v, notice = %q", command, readOnly.(Model).notice)
	}
}

func TestRoadmapOpenUsesApplicationService(t *testing.T) {
	t.Parallel()

	service := &fakeService{result: app.Result{Message: "Opened ROADMAP.md"}}
	model := readyModel(service)
	model.screen = screenRoadmap
	opening, command := model.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'o'}})
	if command == nil || !opening.(Model).opening {
		t.Fatal("o did not start roadmap open")
	}
	opened, _ := opening.(Model).Update(command())
	if opened.(Model).opening || opened.(Model).notice != "Opened ROADMAP.md" {
		t.Errorf("roadmap open state = %#v", opened)
	}
	if len(service.executed) != 1 || service.executed[0].OpenTarget != "roadmap" {
		t.Errorf("open commands = %#v", service.executed)
	}
}

func TestViewsRemainWithinTerminalWidth(t *testing.T) {
	t.Parallel()

	for _, width := range []int{24, 40, 80, 120} {
		model := readyModel(&fakeService{})
		model.width = width
		for _, current := range []screen{screenHome, screenDoctor, screenConfig, screenRoadmap} {
			model.screen = current
			for _, line := range strings.Split(model.View(), "\n") {
				if got := lipgloss.Width(line); got > width {
					t.Errorf("width %d screen %d rendered line width %d: %q", width, current, got, line)
				}
			}
		}
	}
}

func readyModel(service Service) Model {
	model := NewModel(context.Background(), service, app.Command{}, true)
	updated, _ := model.Update(foundationLoadedMsg{snapshot: healthySnapshot()})
	return updated.(Model)
}

func healthySnapshot() app.FoundationSnapshot {
	return app.FoundationSnapshot{
		WorkspaceName: "foundation lab",
		WorkspaceRoot: "/workspaces/foundation-lab",
		Checks: []app.FoundationCheck{
			{Name: "Workspace initialized", OK: true},
			{Name: "Database healthy", OK: true},
			{Name: "Configuration loaded", OK: true},
		},
		Settings: config.Defaults(),
	}
}

func keyType(key string) tea.KeyType {
	if key == "enter" {
		return tea.KeyEnter
	}
	return tea.KeyRunes
}

func keyRunes(key string) []rune {
	if key == "enter" {
		return nil
	}
	return []rune(key)
}

type fakeService struct {
	snapshot      app.FoundationSnapshot
	loadErr       error
	result        app.Result
	executeErr    error
	loadedCommand app.Command
	executed      []app.Command
	resume        session.Resume
	sessionErr    error
	checkpoints   []session.State
	checkpointErr error
	completed     []session.State
	completeErr   error
}

func (service *fakeService) LoadFoundation(_ context.Context, command app.Command) (app.FoundationSnapshot, error) {
	service.loadedCommand = command
	return service.snapshot, service.loadErr
}

func (service *fakeService) Execute(_ context.Context, command app.Command) (app.Result, error) {
	service.executed = append(service.executed, command)
	return service.result, service.executeErr
}

func (service *fakeService) ResumeSession(_ context.Context, _ app.Command) (session.Resume, error) {
	if service.resume.State.Version == 0 {
		service.resume.State = session.Default()
		service.resume.State.SessionStartedAt = time.Date(2026, time.August, 12, 12, 0, 0, 0, time.UTC)
	}
	return service.resume, service.sessionErr
}

func (service *fakeService) CheckpointSession(_ context.Context, _ app.Command, state session.State) error {
	service.checkpoints = append(service.checkpoints, state.Clone())
	return service.checkpointErr
}

func (service *fakeService) CompleteSession(_ context.Context, _ app.Command, state session.State) error {
	service.completed = append(service.completed, state.Clone())
	return service.completeErr
}
