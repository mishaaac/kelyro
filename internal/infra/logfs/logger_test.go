package logfs

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"

	"github.com/mishaaac/kelyro/internal/logging"
	"github.com/mishaaac/kelyro/internal/platform"
)

var logTestTime = time.Date(2026, 8, 12, 15, 30, 0, 0, time.UTC)

func TestLoggerWritesRedactedStructuredJSON(t *testing.T) {
	root := logWorkspace(t)
	factory := New(withClock(func() time.Time { return logTestTime }))
	logger, err := factory.Open(root, true)
	if err != nil {
		t.Fatalf("Open() error = %v", err)
	}
	secret := "sk-sensitive-value"
	err = logger.Log(context.Background(), logging.Entry{
		Level: logging.Error, Message: "request failed for " + secret,
		Operation: "secrets.set", Workspace: root, Component: "application", ErrorCategory: "operation",
		Fields:    map[string]string{"api_token": secret, "student_content": "full essay", "attempt": "1"},
		Sensitive: []string{secret},
	})
	if err != nil {
		t.Fatalf("Log() error = %v", err)
	}
	if err := logger.Close(); err != nil {
		t.Fatalf("Close() error = %v", err)
	}

	path, _ := factory.Path(root)
	encoded, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("ReadFile() error = %v", err)
	}
	if strings.Contains(string(encoded), secret) || strings.Contains(string(encoded), "full essay") {
		t.Fatalf("log exposed protected data: %s", encoded)
	}
	var got record
	if err := json.Unmarshal(encoded, &got); err != nil {
		t.Fatalf("decode log: %v", err)
	}
	if got.Timestamp != logTestTime.Format(time.RFC3339Nano) || got.Level != logging.Error || got.Operation != "secrets.set" || got.ErrorCategory != "operation" {
		t.Fatalf("structured record = %+v", got)
	}
	if got.Fields["api_token"] != logging.Redacted || got.Fields["student_content"] != logging.Omitted {
		t.Fatalf("redacted fields = %#v", got.Fields)
	}
}

func TestLoggerRotationBoundsFilesAndDebugRequiresVerbose(t *testing.T) {
	root := logWorkspace(t)
	factory := New(WithLimits(400, 3), withClock(func() time.Time { return logTestTime }))
	logger, err := factory.Open(root, false)
	if err != nil {
		t.Fatalf("Open() error = %v", err)
	}
	base := logging.Entry{
		Operation: "doctor", Workspace: root, Component: "application",
		Fields: map[string]string{"detail": strings.Repeat("x", 90)},
	}
	base.Level, base.Message = logging.Debug, "debug hidden"
	if err := logger.Log(context.Background(), base); err != nil {
		t.Fatalf("debug Log() error = %v", err)
	}
	for index := 0; index < 12; index++ {
		base.Level, base.Message = logging.Info, "bounded diagnostic entry"
		if err := logger.Log(context.Background(), base); err != nil {
			t.Fatalf("Log(%d) error = %v", index, err)
		}
	}
	if err := logger.Close(); err != nil {
		t.Fatalf("Close() error = %v", err)
	}

	path, _ := factory.Path(root)
	files, err := filepath.Glob(path + "*")
	if err != nil {
		t.Fatalf("Glob() error = %v", err)
	}
	if len(files) != 3 {
		t.Fatalf("retained log files = %d, want 3 (%v)", len(files), files)
	}
	for _, file := range files {
		info, err := os.Stat(file)
		if err != nil {
			t.Fatalf("Stat(%s) error = %v", file, err)
		}
		if info.Size() > 400 {
			t.Errorf("log %s size = %d, exceeds limit", file, info.Size())
		}
		encoded, err := os.ReadFile(file)
		if err != nil {
			t.Fatalf("ReadFile(%s) error = %v", file, err)
		}
		if strings.Contains(string(encoded), "debug hidden") {
			t.Errorf("non-verbose log contains debug entry: %s", encoded)
		}
	}
}

func TestOpenRejectsSymlinkedLogTargets(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("creating symlinks may require elevated Windows privileges")
	}

	t.Run("file", func(t *testing.T) {
		root := logWorkspace(t)
		factory := New()
		path, _ := factory.Path(root)
		target := filepath.Join(t.TempDir(), "outside.log")
		if err := os.WriteFile(target, []byte("must remain unchanged"), 0o600); err != nil {
			t.Fatal(err)
		}
		if err := os.Symlink(target, path); err != nil {
			t.Fatal(err)
		}
		if logger, err := factory.Open(root, false); err == nil || logger != nil {
			t.Fatalf("Open() = (%v, %v), want symlink rejection", logger, err)
		}
		encoded, err := os.ReadFile(target)
		if err != nil || string(encoded) != "must remain unchanged" {
			t.Fatalf("outside target changed: %q, %v", encoded, err)
		}
	})

	t.Run("directory", func(t *testing.T) {
		root := t.TempDir()
		directory, _ := platform.WorkspaceLogDir(root)
		if err := os.MkdirAll(filepath.Dir(directory), 0o700); err != nil {
			t.Fatal(err)
		}
		if err := os.Symlink(t.TempDir(), directory); err != nil {
			t.Fatal(err)
		}
		if logger, err := New().Open(root, false); err == nil || logger != nil {
			t.Fatalf("Open() = (%v, %v), want symlink rejection", logger, err)
		}
	})
}

func logWorkspace(t *testing.T) string {
	t.Helper()
	root := t.TempDir()
	directory, err := platform.WorkspaceLogDir(root)
	if err != nil {
		t.Fatalf("WorkspaceLogDir() error = %v", err)
	}
	if err := os.MkdirAll(directory, 0o755); err != nil {
		t.Fatalf("MkdirAll() error = %v", err)
	}
	return root
}
