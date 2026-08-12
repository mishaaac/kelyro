package app

import (
	"context"
	"path/filepath"
	"testing"

	"github.com/mishaaac/kelyro/internal/portability"
	"github.com/mishaaac/kelyro/internal/workspace"
)

func TestServiceCoordinatesPortableExportAndImport(t *testing.T) {
	root := filepath.Join(t.TempDir(), "source workspace")
	destination := filepath.Join(t.TempDir(), "destination workspace")
	workspaces := &recordingWorkspaceService{discovered: workspace.Workspace{Root: root}}
	portable := &recordingPortabilityService{
		exported: portability.Report{ArchivePath: "/exports/workspace.tar.gz", Mode: portability.ModeFull, FileCount: 4},
		imported: portability.Report{ArchivePath: "/exports/workspace.tar.gz", Destination: destination, Mode: portability.ModeFull, FileCount: 4},
	}
	audits := &recordingAuditFactory{store: &recordingAuditStore{}}
	service := NewService(workspaces, func() (string, error) { return root, nil }).
		WithPortability(portable).
		WithAudit(audits)

	exported, err := service.Execute(context.Background(), Command{
		Action: ActionExport, ExportMode: portability.ModeFull, ExportOutput: "/exports/workspace.tar.gz",
	})
	if err != nil || exported.Portability == nil || exported.Portability.Mode != portability.ModeFull {
		t.Fatalf("Execute(export) = (%+v, %v)", exported, err)
	}
	if portable.exportRoot != root || portable.exportOptions.Mode != portability.ModeFull {
		t.Fatalf("Export() = root %q options %+v", portable.exportRoot, portable.exportOptions)
	}

	imported, err := service.Execute(context.Background(), Command{
		Action: ActionImport, Workspace: destination, ImportArchive: "/exports/workspace.tar.gz",
		ImportConflicts: portability.ConflictOverwrite,
	})
	if err != nil || imported.Portability == nil || imported.Portability.Destination != destination {
		t.Fatalf("Execute(import) = (%+v, %v)", imported, err)
	}
	if portable.importOptions.Destination != destination || portable.importOptions.Conflicts != portability.ConflictOverwrite {
		t.Fatalf("Import() options = %+v", portable.importOptions)
	}
	if len(audits.store.events) != 1 || audits.store.events[0].Name != "import.completed" || audits.store.events[0].Metadata["mode"] != "full" {
		t.Fatalf("audit events = %+v", audits.store.events)
	}
}

func TestDryRunDoesNotAuditImport(t *testing.T) {
	destination := t.TempDir()
	portable := &recordingPortabilityService{imported: portability.Report{
		ArchivePath: "archive.tar.gz", Destination: destination, Mode: portability.ModeHuman, DryRun: true,
	}}
	audits := &recordingAuditFactory{store: &recordingAuditStore{}}
	service := NewService(&recordingWorkspaceService{}, func() (string, error) { return destination, nil }).
		WithPortability(portable).WithAudit(audits)
	_, err := service.Execute(context.Background(), Command{
		Action: ActionImport, ImportArchive: "archive.tar.gz", ImportDryRun: true, ImportConflicts: portability.ConflictFail,
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(audits.store.events) != 0 {
		t.Fatalf("dry-run audit events = %+v", audits.store.events)
	}
}

type recordingPortabilityService struct {
	exportRoot    string
	exportOptions portability.ExportOptions
	importOptions portability.ImportOptions
	exported      portability.Report
	imported      portability.Report
	err           error
}

func (service *recordingPortabilityService) Export(_ context.Context, root string, options portability.ExportOptions) (portability.Report, error) {
	service.exportRoot = root
	service.exportOptions = options
	return service.exported, service.err
}

func (service *recordingPortabilityService) Import(_ context.Context, options portability.ImportOptions) (portability.Report, error) {
	service.importOptions = options
	return service.imported, service.err
}

var _ portability.Service = (*recordingPortabilityService)(nil)
