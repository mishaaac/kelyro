package app

import (
	"context"
	"errors"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/mishaaac/kelyro/internal/audit"
	"github.com/mishaaac/kelyro/internal/config"
	"github.com/mishaaac/kelyro/internal/logging"
	"github.com/mishaaac/kelyro/internal/workspace"
)

func TestStudentAuthoredAnswersAreRedactedBeforeLogSerialization(t *testing.T) {
	t.Parallel()
	const privateAnswer = "my private diagnostic explanation"
	service := &Service{}
	entry := service.logEntry(Command{
		Action: ActionSetup, SetupOperation: "diagnostic-submit", SetupAnswers: []string{privateAnswer},
	}, "/workspace", logging.Error, "diagnostic answer "+privateAnswer+" is invalid", errors.New("invalid answer"))
	sanitized := logging.Sanitize(entry)
	if strings.Contains(sanitized.Message, privateAnswer) || !strings.Contains(sanitized.Message, logging.Redacted) {
		t.Fatalf("sanitized diagnostic log message = %q", sanitized.Message)
	}
	if len(sanitized.Sensitive) != 0 {
		t.Fatalf("sanitized diagnostic log retained sensitive values: %#v", sanitized.Sensitive)
	}
}

func TestServiceExposesLogPathAndPersistentAuditEntries(t *testing.T) {
	t.Parallel()
	root := filepath.Join(string(filepath.Separator), "learning workspace")
	workspaces := &recordingWorkspaceService{discovered: workspace.Workspace{Root: root}}
	audits := &recordingAuditFactory{store: &recordingAuditStore{entries: []audit.Entry{{
		Timestamp: time.Date(2026, 8, 12, 15, 0, 0, 0, time.UTC),
		Event:     "workspace.initialized", Actor: audit.ActorUser, Subject: root, AppVersion: "dev",
	}}}}
	logs := &recordingLogFactory{path: filepath.Join(root, ".kelyro", "logs", "kelyro.jsonl")}
	service := NewService(workspaces, func() (string, error) { return root, nil }).WithAudit(audits).WithLogging(logs)

	logResult, err := service.Execute(context.Background(), Command{Action: ActionLogs, LogOperation: "path"})
	if err != nil || logResult.Message != logs.path {
		t.Fatalf("Execute(logs path) = (%+v, %v)", logResult, err)
	}
	auditResult, err := service.Execute(context.Background(), Command{Action: ActionAudit})
	if err != nil || len(auditResult.Audit) != 1 || auditResult.Audit[0].Event != "workspace.initialized" {
		t.Fatalf("Execute(audit) = (%+v, %v)", auditResult, err)
	}
	if !audits.store.closed {
		t.Fatal("audit store was not closed")
	}
}

func TestInitializationRecordsWorkspaceAuditEvent(t *testing.T) {
	t.Parallel()
	root := filepath.Join(string(filepath.Separator), "normalized", "project")
	workspaces := &recordingWorkspaceService{workspace: workspace.Workspace{Root: root}}
	audits := &recordingAuditFactory{store: &recordingAuditStore{}}
	service := NewService(workspaces, func() (string, error) { return root, nil }).
		WithArtifactStores(&recordingArtifactStoreFactory{store: &recordingArtifactStore{}}).
		WithAudit(audits)

	if _, err := service.Execute(context.Background(), Command{Action: ActionInit, Workspace: root}); err != nil {
		t.Fatalf("Execute(init) error = %v", err)
	}
	if len(audits.store.events) != 1 || audits.store.events[0].Name != "workspace.initialized" || audits.store.events[0].Actor != audit.ActorUser {
		t.Fatalf("audit events = %+v", audits.store.events)
	}
}

func TestProjectConfigChangeAuditsKeyWithoutValue(t *testing.T) {
	t.Parallel()
	root := filepath.Join(string(filepath.Separator), "normalized", "project")
	workspaces := &recordingWorkspaceService{discovered: workspace.Workspace{Root: root}}
	audits := &recordingAuditFactory{store: &recordingAuditStore{}}
	configs := &recordingConfigStore{projectPath: filepath.Join(root, ".kelyro", "config.toml")}
	service := NewService(workspaces, func() (string, error) { return root, nil }).WithConfig(configs).WithAudit(audits)

	_, err := service.Execute(context.Background(), Command{
		Action: ActionConfig, ConfigOperation: "set", ConfigScope: config.ScopeProject,
		ConfigKey: config.KeyWorkspaceName, ConfigValue: "private learning goal",
	})
	if err != nil {
		t.Fatalf("Execute(config set) error = %v", err)
	}
	if len(audits.store.events) != 1 {
		t.Fatalf("audit events = %+v", audits.store.events)
	}
	event := audits.store.events[0]
	if event.Name != "config.changed" || event.Subject != config.KeyWorkspaceName || event.Metadata["scope"] != "project" {
		t.Fatalf("config audit event = %+v", event)
	}
	for _, value := range event.Metadata {
		if value == "private learning goal" {
			t.Fatal("config audit event persisted the configured value")
		}
	}
}

type recordingAuditFactory struct {
	store *recordingAuditStore
	root  string
}

func (factory *recordingAuditFactory) Open(_ context.Context, root string) (audit.Store, error) {
	factory.root = root
	factory.store.closed = false
	return factory.store, nil
}

type recordingAuditStore struct {
	events  []audit.Event
	entries []audit.Entry
	closed  bool
}

func (store *recordingAuditStore) Record(_ context.Context, event audit.Event) error {
	store.events = append(store.events, event)
	return nil
}
func (store *recordingAuditStore) List(context.Context) ([]audit.Entry, error) {
	return append([]audit.Entry(nil), store.entries...), nil
}
func (store *recordingAuditStore) Close() error { store.closed = true; return nil }

type recordingLogFactory struct {
	path   string
	logger *recordingLogger
}

func (factory *recordingLogFactory) Open(string, bool) (logging.Logger, error) {
	if factory.logger == nil {
		factory.logger = &recordingLogger{}
	}
	return factory.logger, nil
}
func (factory *recordingLogFactory) Path(string) (string, error) { return factory.path, nil }

type recordingLogger struct{ entries []logging.Entry }

func (logger *recordingLogger) Log(_ context.Context, entry logging.Entry) error {
	logger.entries = append(logger.entries, entry)
	return nil
}
func (*recordingLogger) Close() error { return nil }
