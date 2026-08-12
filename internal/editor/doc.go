// Package editor defines editor discovery and file-opening contracts without
// depending on a particular operating system or process implementation.
package editor

import (
	"context"
	"errors"
)

// ErrUnavailable means neither a configured/detected editor nor the native
// system-default opener can be executed.
var ErrUnavailable = errors.New("no editor or system-default opener is available")

// Selection describes the executable chosen to open an artifact. Executable
// is the real path returned by the operating system lookup.
type Selection struct {
	Name          string
	Executable    string
	SystemDefault bool
}

// Service detects installed editors and safely opens one complete path.
// Configured is an executable name or path, not a shell command string.
type Service interface {
	Detect(configured string) (Selection, error)
	Open(ctx context.Context, target, configured string) (Selection, error)
}
