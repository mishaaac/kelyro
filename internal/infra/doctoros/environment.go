// Package doctoros implements Doctor environment probes with bounded, direct
// process execution and filesystem operations.
package doctoros

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
)

const maxVersionOutput = 16 * 1024

// Environment is the native implementation of doctor.Environment.
type Environment struct{}

// New creates the native diagnostics environment.
func New() *Environment { return &Environment{} }

func (*Environment) Platform() string { return runtime.GOOS }

// Writable verifies actual create/remove access without altering existing
// files. The temporary probe is always removed on a best-effort basis.
func (*Environment) Writable(path string) (resultErr error) {
	file, err := os.CreateTemp(path, ".kelyro-doctor-*.tmp")
	if err != nil {
		return fmt.Errorf("write probe in %s: %w", path, err)
	}
	name := file.Name()
	defer func() {
		if removeErr := os.Remove(name); removeErr != nil && !errors.Is(removeErr, os.ErrNotExist) {
			resultErr = errors.Join(resultErr, fmt.Errorf("remove write probe: %w", removeErr))
		}
	}()
	if err := file.Close(); err != nil {
		return fmt.Errorf("close write probe: %w", err)
	}
	return nil
}

func (*Environment) Resolve(candidates []string) (string, bool) {
	for _, candidate := range candidates {
		path, err := exec.LookPath(candidate)
		if err == nil {
			if !filepath.IsAbs(path) {
				if absolute, absoluteErr := filepath.Abs(path); absoluteErr == nil {
					path = absolute
				}
			}
			return path, true
		}
	}
	return "", false
}

// Version executes only the resolved executable with registry-owned arguments;
// it never invokes a shell and bounds captured output.
func (*Environment) Version(ctx context.Context, executable string, args []string) (string, error) {
	workingDirectory, err := os.MkdirTemp("", "kelyro-doctor-")
	if err != nil {
		return "", fmt.Errorf("create isolated version directory: %w", err)
	}
	defer os.RemoveAll(workingDirectory)

	command := exec.CommandContext(ctx, executable, args...)
	command.Dir = workingDirectory
	output := &limitedBuffer{limit: maxVersionOutput}
	command.Stdout = output
	command.Stderr = output
	err = command.Run()
	if err != nil {
		if ctxErr := ctx.Err(); ctxErr != nil {
			return output.String(), ctxErr
		}
		return output.String(), fmt.Errorf("query version: %w", err)
	}
	return output.String(), nil
}

type limitedBuffer struct {
	buffer bytes.Buffer
	limit  int
}

func (buffer *limitedBuffer) Write(data []byte) (int, error) {
	written := len(data)
	remaining := buffer.limit - buffer.buffer.Len()
	if remaining > 0 {
		if len(data) > remaining {
			data = data[:remaining]
		}
		_, _ = buffer.buffer.Write(data)
	}
	return written, nil
}

func (buffer *limitedBuffer) String() string { return buffer.buffer.String() }
