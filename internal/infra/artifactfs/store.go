package artifactfs

import (
	"context"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"time"

	"github.com/mishaaac/kelyro/internal/artifacts"
	"github.com/mishaaac/kelyro/internal/audit"
)

var (
	// ErrModified reports that a tracked human-readable artifact was edited
	// after Kelyro last generated it.
	ErrModified = errors.New("generated artifact was modified externally")
	// ErrUntracked reports that an existing human-readable file has no index
	// entry proving that Kelyro created its current content.
	ErrUntracked = errors.New("existing artifact is not tracked")
	// ErrStudentOwned reports an attempted Kelyro write to student-owned content.
	ErrStudentOwned = errors.New("student-owned artifact cannot be written")
	// ErrOwnershipMismatch reports a request that assigns an unsafe category to
	// a path instead of using its default classification.
	ErrOwnershipMismatch = errors.New("artifact ownership does not match path classification")
)

// WriteRequest is retained as the filesystem adapter's public spelling while
// the contract itself remains infrastructure-neutral.
type WriteRequest = artifacts.WriteRequest

// Store writes artifacts beneath a workspace root and records their integrity.
type Store struct {
	sandbox *Sandbox
	index   artifacts.Index
	now     func() time.Time
	replace func(string, string) error
	audit   audit.Recorder
}

// New creates an ownership-aware artifact store.
func New(root string, index artifacts.Index, recorders ...audit.Recorder) (*Store, error) {
	if index == nil {
		return nil, fmt.Errorf("artifact index is required")
	}
	sandbox, err := NewSandbox(root)
	if err != nil {
		return nil, err
	}
	var recorder audit.Recorder
	if len(recorders) > 0 {
		recorder = recorders[0]
	}
	return &Store{sandbox: sandbox, index: index, now: time.Now, replace: replaceFile, audit: recorder}, nil
}

// Write atomically creates or regenerates an artifact. A conflict is returned
// without changing either the file or index.
func (store *Store) Write(ctx context.Context, request WriteRequest) (artifacts.Artifact, error) {
	destination, err := store.sandbox.Resolve(request.Path)
	if err != nil {
		return artifacts.Artifact{}, err
	}
	nativeRelative := filepath.Clean(request.Path)
	relative := filepath.ToSlash(nativeRelative)
	classification := artifacts.Classify(nativeRelative)
	if request.Ownership != classification {
		return artifacts.Artifact{}, fmt.Errorf("%w: %s is %s", ErrOwnershipMismatch, relative, classification)
	}
	if request.Ownership == artifacts.StudentOwned {
		return artifacts.Artifact{}, fmt.Errorf("%w: %s", ErrStudentOwned, relative)
	}

	known, found, err := store.index.Get(ctx, relative)
	if err != nil {
		return artifacts.Artifact{}, fmt.Errorf("read artifact index for %s: %w", relative, err)
	}
	if found && known.Ownership != request.Ownership {
		return artifacts.Artifact{}, fmt.Errorf("%w: index records %s as %s", ErrOwnershipMismatch, relative, known.Ownership)
	}

	current, readErr := os.ReadFile(destination)
	exists := readErr == nil
	if readErr != nil && !errors.Is(readErr, fs.ErrNotExist) {
		return artifacts.Artifact{}, fmt.Errorf("read artifact %s: %w", relative, readErr)
	}
	if request.Ownership == artifacts.SystemGeneratedHumanReadable && exists {
		switch {
		case !found:
			store.record(ctx, "artifact.regeneration_blocked", relative, map[string]string{"reason": "untracked"})
			return artifacts.Artifact{}, fmt.Errorf("%w: %s", ErrUntracked, relative)
		case known.ContentHash == "" || artifacts.Hash(current) != known.ContentHash:
			store.record(ctx, "artifact.regeneration_blocked", relative, map[string]string{"reason": "modified"})
			return artifacts.Artifact{}, fmt.Errorf("%w: %s", ErrModified, relative)
		}
	}

	now := store.now().UTC()
	createdAt := now
	if found && !known.CreatedAt.IsZero() {
		createdAt = known.CreatedAt
	}
	next := artifacts.Artifact{
		Path:            relative,
		Ownership:       request.Ownership,
		CreatedBy:       request.CreatedBy,
		ContentHash:     artifacts.Hash(request.Content),
		CreatedAt:       createdAt,
		LastGeneratedAt: now,
		ExpectedVersion: request.ExpectedVersion,
	}
	if err := next.Validate(); err != nil {
		return artifacts.Artifact{}, err
	}

	if !exists || artifacts.Hash(current) != next.ContentHash {
		permissions := fs.FileMode(0o644)
		if request.Ownership == artifacts.MachineOwned {
			permissions = 0o600
		}
		if err := store.writeAtomic(destination, request.Content, permissions); err != nil {
			return artifacts.Artifact{}, fmt.Errorf("write artifact %s: %w", relative, err)
		}
	}
	if err := store.index.Put(ctx, next); err != nil {
		return artifacts.Artifact{}, fmt.Errorf("record artifact %s: %w", relative, err)
	}
	store.record(ctx, "artifact.generated", relative, map[string]string{
		"created_by":       request.CreatedBy,
		"ownership":        string(request.Ownership),
		"expected_version": request.ExpectedVersion,
	})
	return next, nil
}

func (store *Store) record(ctx context.Context, name, subject string, metadata map[string]string) {
	if store.audit == nil {
		return
	}
	_ = store.audit.Record(ctx, audit.Event{
		Name: name, Actor: audit.ActorSystem, Subject: subject, Metadata: metadata,
	})
}

func (store *Store) writeAtomic(destination string, content []byte, permissions fs.FileMode) (err error) {
	directory := filepath.Dir(destination)
	if err := os.MkdirAll(directory, 0o755); err != nil {
		return fmt.Errorf("create artifact directory: %w", err)
	}
	if _, err := store.sandbox.Resolve(mustRelative(store.sandbox.root, destination)); err != nil {
		return err
	}

	temporary, err := os.CreateTemp(directory, ".kelyro-artifact-*.tmp")
	if err != nil {
		return fmt.Errorf("create artifact staging file: %w", err)
	}
	temporaryPath := temporary.Name()
	defer func() {
		_ = temporary.Close()
		_ = os.Remove(temporaryPath)
	}()

	if err := temporary.Chmod(permissions); err != nil {
		return fmt.Errorf("set artifact staging permissions: %w", err)
	}
	if _, err := temporary.Write(content); err != nil {
		return fmt.Errorf("write artifact staging file: %w", err)
	}
	if err := temporary.Sync(); err != nil {
		return fmt.Errorf("sync artifact staging file: %w", err)
	}
	if err := temporary.Close(); err != nil {
		return fmt.Errorf("close artifact staging file: %w", err)
	}
	if err := store.replace(temporaryPath, destination); err != nil {
		return fmt.Errorf("commit artifact file: %w", err)
	}
	return nil
}

func mustRelative(root, target string) string {
	relative, err := filepath.Rel(root, target)
	if err != nil {
		return target // Resolve rejects this value; retaining the error path is safe.
	}
	return relative
}
