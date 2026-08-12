// Package artifacts classifies files by ownership so infrastructure adapters
// can apply safe write policies.
package artifacts

import "context"

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
	Path      string
	Ownership Ownership
}

// Index persists the known ownership of workspace artifacts. Content-integrity
// policy remains outside this minimal Foundation repository.
type Index interface {
	Get(ctx context.Context, path string) (artifact Artifact, found bool, err error)
	Put(ctx context.Context, artifact Artifact) error
	Delete(ctx context.Context, path string) error
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
