// Package artifacts classifies files by ownership so infrastructure adapters
// can apply safe write policies.
package artifacts

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"path/filepath"
	"strings"
	"time"
)

// WriteRequest describes generated content and the integrity metadata that
// must accompany it.
type WriteRequest struct {
	Path            string
	Ownership       Ownership
	CreatedBy       string
	Content         []byte
	ExpectedVersion string
}

// WorkspaceStore writes artifacts for one workspace and owns any persistence
// resources opened for that workspace.
type WorkspaceStore interface {
	Write(ctx context.Context, request WriteRequest) (Artifact, error)
	Close() error
}

// WorkspaceStoreFactory opens ownership-aware stores without exposing their
// persistence implementation to application services.
type WorkspaceStoreFactory interface {
	Open(ctx context.Context, workspaceRoot string) (WorkspaceStore, error)
}

// Ownership describes who is allowed to author or overwrite an artifact.
type Ownership string

const (
	// MachineOwned identifies opaque files managed exclusively by Kelyro.
	MachineOwned Ownership = "machine-owned"
	// SystemGeneratedHumanReadable identifies generated files intended for
	// people to inspect but not to edit as their primary source.
	SystemGeneratedHumanReadable Ownership = "system-generated-human-readable"
	// StudentOwned identifies human-authored learning material that Kelyro must
	// never overwrite without explicit confirmation.
	StudentOwned Ownership = "student-owned"
)

// Artifact associates a path with its ownership policy.
type Artifact struct {
	Path            string
	Ownership       Ownership
	CreatedBy       string
	ContentHash     string
	CreatedAt       time.Time
	LastGeneratedAt time.Time
	ExpectedVersion string
}

// Index persists workspace-relative ownership and content-integrity metadata.
type Index interface {
	Get(ctx context.Context, path string) (artifact Artifact, found bool, err error)
	Put(ctx context.Context, artifact Artifact) error
	Delete(ctx context.Context, path string) error
}

// Classify returns the default ownership policy for a workspace-relative path.
// Visible files are student-owned unless they are known generated documents.
func Classify(path string) Ownership {
	cleaned := filepath.Clean(path)
	internal := ".kelyro"
	if cleaned == internal || strings.HasPrefix(cleaned, internal+string(filepath.Separator)) {
		return MachineOwned
	}

	if cleaned == filepath.Join("00-roadmap", "PROGRESS.md") {
		return SystemGeneratedHumanReadable
	}

	switch filepath.Base(cleaned) {
	case "LEARNING.md", "ROADMAP.md", "LESSON.md":
		return SystemGeneratedHumanReadable
	default:
		return StudentOwned
	}
}

// Hash computes the canonical SHA-256 digest stored in the artifact index.
func Hash(content []byte) string {
	digest := sha256.Sum256(content)
	return hex.EncodeToString(digest[:])
}

// Validate checks metadata written by current Kelyro versions.
func (artifact Artifact) Validate() error {
	if strings.TrimSpace(artifact.Path) == "" {
		return fmt.Errorf("artifact path is empty")
	}
	if !artifact.Ownership.Valid() {
		return fmt.Errorf("artifact ownership %q is invalid", artifact.Ownership)
	}
	if strings.TrimSpace(artifact.CreatedBy) == "" {
		return fmt.Errorf("artifact creator is empty")
	}
	if len(artifact.ContentHash) != sha256.Size*2 {
		return fmt.Errorf("artifact content hash must be a SHA-256 digest")
	}
	if _, err := hex.DecodeString(artifact.ContentHash); err != nil {
		return fmt.Errorf("artifact content hash is invalid: %w", err)
	}
	if artifact.CreatedAt.IsZero() {
		return fmt.Errorf("artifact creation time is empty")
	}
	if artifact.LastGeneratedAt.IsZero() {
		return fmt.Errorf("artifact generation time is empty")
	}
	if artifact.LastGeneratedAt.Before(artifact.CreatedAt) {
		return fmt.Errorf("artifact generation time precedes creation time")
	}
	return nil
}

// Valid reports whether ownership is one of the defined policies.
func (ownership Ownership) Valid() bool {
	switch ownership {
	case MachineOwned, SystemGeneratedHumanReadable, StudentOwned:
		return true
	default:
		return false
	}
}
