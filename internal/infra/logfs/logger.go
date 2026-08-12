// Package logfs writes bounded structured logs inside a Kelyro workspace.
package logfs

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strconv"
	"sync"
	"time"

	"github.com/mishaaac/kelyro/internal/logging"
	"github.com/mishaaac/kelyro/internal/platform"
)

const (
	logFileName    = "kelyro.jsonl"
	defaultMaxSize = int64(1 << 20)
	defaultFiles   = 3
	minimumMaxSize = int64(256)
)

type options struct {
	maxSize int64
	files   int
	now     func() time.Time
}

// Option customizes the bounded file policy.
type Option func(*options)

// WithLimits sets the maximum bytes per file and total retained file count.
func WithLimits(maxSize int64, files int) Option {
	return func(settings *options) {
		settings.maxSize = maxSize
		settings.files = files
	}
}

func withClock(now func() time.Time) Option {
	return func(settings *options) { settings.now = now }
}

// Factory creates workspace-local JSONL loggers.
type Factory struct {
	settings options
}

// New creates a logger factory with a one-MiB file and three-file retention by
// default.
func New(configured ...Option) *Factory {
	settings := options{maxSize: defaultMaxSize, files: defaultFiles, now: time.Now}
	for _, configure := range configured {
		if configure != nil {
			configure(&settings)
		}
	}
	return &Factory{settings: settings}
}

// Path returns the active structured log file path without creating it.
func (*Factory) Path(workspaceRoot string) (string, error) {
	directory, err := platform.WorkspaceLogDir(workspaceRoot)
	if err != nil {
		return "", err
	}
	return filepath.Join(directory, logFileName), nil
}

// Open validates the retention policy and opens the current log for append.
func (factory *Factory) Open(workspaceRoot string, verbose bool) (logging.Logger, error) {
	if factory == nil {
		return nil, errors.New("log factory is unavailable")
	}
	if factory.settings.maxSize < minimumMaxSize {
		return nil, fmt.Errorf("log size limit must be at least %d bytes", minimumMaxSize)
	}
	if factory.settings.files < 1 {
		return nil, errors.New("log retention must keep at least one file")
	}
	if factory.settings.now == nil {
		return nil, errors.New("log clock must not be nil")
	}
	path, err := factory.Path(workspaceRoot)
	if err != nil {
		return nil, err
	}
	info, err := os.Lstat(filepath.Dir(path))
	if err != nil {
		return nil, fmt.Errorf("inspect workspace log directory: %w", err)
	}
	if !info.IsDir() || info.Mode()&os.ModeSymlink != 0 {
		return nil, fmt.Errorf("workspace log path is not a regular directory: %s", filepath.Dir(path))
	}
	file, err := openAppendFile(path)
	if err != nil {
		return nil, fmt.Errorf("open workspace log: %w", err)
	}
	if err := file.Chmod(0o600); err != nil {
		_ = file.Close()
		return nil, fmt.Errorf("restrict workspace log permissions: %w", err)
	}
	return &logger{
		path:    path,
		file:    file,
		verbose: verbose,
		maxSize: factory.settings.maxSize,
		files:   factory.settings.files,
		now:     factory.settings.now,
	}, nil
}

func openAppendFile(path string) (*os.File, error) {
	if info, err := os.Lstat(path); err == nil {
		if !info.Mode().IsRegular() || info.Mode()&os.ModeSymlink != 0 {
			return nil, fmt.Errorf("log target %s is not a regular file", path)
		}
	} else if !errors.Is(err, fs.ErrNotExist) {
		return nil, err
	}

	file, err := os.OpenFile(path, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o600)
	if err != nil {
		return nil, err
	}
	opened, openErr := file.Stat()
	current, pathErr := os.Lstat(path)
	if openErr != nil || pathErr != nil || !current.Mode().IsRegular() || current.Mode()&os.ModeSymlink != 0 || !os.SameFile(opened, current) {
		_ = file.Close()
		return nil, fmt.Errorf("log target %s changed or is not a regular file", path)
	}
	return file, nil
}

type logger struct {
	mu      sync.Mutex
	path    string
	file    *os.File
	verbose bool
	maxSize int64
	files   int
	now     func() time.Time
}

type record struct {
	Timestamp     string            `json:"timestamp"`
	Level         logging.Level     `json:"level"`
	Message       string            `json:"message"`
	Operation     string            `json:"operation"`
	Workspace     string            `json:"workspace"`
	Component     string            `json:"component"`
	ErrorCategory string            `json:"error_category,omitempty"`
	Fields        map[string]string `json:"fields,omitempty"`
}

func (logger *logger) Log(ctx context.Context, entry logging.Entry) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	if !entry.Level.Valid() {
		return fmt.Errorf("logging level %q is invalid", entry.Level)
	}
	if entry.Level == logging.Debug && !logger.verbose {
		return nil
	}
	if entry.Operation == "" || entry.Workspace == "" || entry.Component == "" {
		return errors.New("log operation, workspace, and component are required")
	}
	if entry.Level == logging.Error && entry.ErrorCategory == "" {
		return errors.New("error logs require an error category")
	}

	entry = logging.Sanitize(entry)
	encoded, err := logger.encode(entry)
	if err != nil {
		return err
	}
	if int64(len(encoded)) > logger.maxSize {
		entry.Message = "log entry exceeded the bounded record size"
		entry.Fields = map[string]string{"truncated": "true"}
		entry.Workspace = "[TRUNCATED]"
		entry.Operation = truncate(entry.Operation, 32)
		entry.Component = truncate(entry.Component, 32)
		entry.ErrorCategory = truncate(entry.ErrorCategory, 32)
		encoded, err = logger.encode(entry)
		if err != nil {
			return err
		}
		if int64(len(encoded)) > logger.maxSize {
			return errors.New("structured log record cannot fit within the configured size limit")
		}
	}

	logger.mu.Lock()
	defer logger.mu.Unlock()
	if logger.file == nil {
		return errors.New("workspace log is closed")
	}
	info, err := logger.file.Stat()
	if err != nil {
		return fmt.Errorf("inspect workspace log: %w", err)
	}
	if info.Size() > 0 && info.Size()+int64(len(encoded)) > logger.maxSize {
		if err := logger.rotate(); err != nil {
			return err
		}
	}
	if _, err := logger.file.Write(encoded); err != nil {
		return fmt.Errorf("write workspace log: %w", err)
	}
	if err := logger.file.Sync(); err != nil {
		return fmt.Errorf("sync workspace log: %w", err)
	}
	return nil
}

func truncate(value string, limit int) string {
	if len(value) <= limit {
		return value
	}
	return value[:limit]
}

func (logger *logger) encode(entry logging.Entry) ([]byte, error) {
	encoded, err := json.Marshal(record{
		Timestamp:     logger.now().UTC().Format(time.RFC3339Nano),
		Level:         entry.Level,
		Message:       entry.Message,
		Operation:     entry.Operation,
		Workspace:     entry.Workspace,
		Component:     entry.Component,
		ErrorCategory: entry.ErrorCategory,
		Fields:        entry.Fields,
	})
	if err != nil {
		return nil, fmt.Errorf("encode structured log: %w", err)
	}
	return append(encoded, '\n'), nil
}

func (logger *logger) rotate() error {
	if err := logger.file.Close(); err != nil {
		return fmt.Errorf("close workspace log for rotation: %w", err)
	}
	logger.file = nil

	if logger.files == 1 {
		if err := os.Remove(logger.path); err != nil && !errors.Is(err, fs.ErrNotExist) {
			return fmt.Errorf("remove expired workspace log: %w", err)
		}
	} else {
		oldest := logger.path + "." + strconv.Itoa(logger.files-1)
		if err := os.Remove(oldest); err != nil && !errors.Is(err, fs.ErrNotExist) {
			return fmt.Errorf("remove expired workspace log: %w", err)
		}
		for index := logger.files - 2; index >= 1; index-- {
			from := logger.path + "." + strconv.Itoa(index)
			to := logger.path + "." + strconv.Itoa(index+1)
			if err := os.Rename(from, to); err != nil && !errors.Is(err, fs.ErrNotExist) {
				return fmt.Errorf("rotate workspace log %d: %w", index, err)
			}
		}
		if err := os.Rename(logger.path, logger.path+".1"); err != nil && !errors.Is(err, fs.ErrNotExist) {
			return fmt.Errorf("rotate workspace log: %w", err)
		}
	}

	file, err := os.OpenFile(logger.path, os.O_APPEND|os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0o600)
	if err != nil {
		return fmt.Errorf("open rotated workspace log: %w", err)
	}
	logger.file = file
	return nil
}

func (logger *logger) Close() error {
	logger.mu.Lock()
	defer logger.mu.Unlock()
	if logger.file == nil {
		return nil
	}
	err := logger.file.Close()
	logger.file = nil
	if err != nil {
		return fmt.Errorf("close workspace log: %w", err)
	}
	return nil
}

var _ logging.WorkspaceFactory = (*Factory)(nil)
var _ logging.Logger = (*logger)(nil)
