// Package audit defines the boundary for persistent security- and
// integrity-relevant actions.
package audit

import (
	"context"
	"fmt"
	"strings"
	"time"
)

// Actor identifies who initiated an audited action.
type Actor string

const (
	ActorSystem Actor = "system"
	ActorUser   Actor = "user"
	ActorPlugin Actor = "plugin"
)

const (
	redacted = "[REDACTED]"
	omitted  = "[OMITTED]"
)

// Event describes a critical action without coupling callers to a database or
// serialized representation. Timestamps are assigned by the persistence
// adapter so callers cannot forge ordering accidentally.
type Event struct {
	Name       string
	Actor      Actor
	Subject    string
	Metadata   map[string]string
	AppVersion string
}

// Entry is one persisted event returned in stable chronological order.
type Entry struct {
	Timestamp  time.Time
	Event      string
	Actor      Actor
	Subject    string
	Metadata   map[string]string
	AppVersion string
}

// Recorder persists critical actions.
type Recorder interface {
	Record(ctx context.Context, event Event) error
}

// Reader loads the durable trail in chronological order.
type Reader interface {
	List(ctx context.Context) ([]Entry, error)
}

// Trail combines the read and write repository operations.
type Trail interface {
	Recorder
	Reader
}

// Store owns the resources used by one workspace audit trail.
type Store interface {
	Trail
	Close() error
}

// WorkspaceStoreFactory opens the audit trail for one workspace.
type WorkspaceStoreFactory interface {
	Open(ctx context.Context, workspaceRoot string) (Store, error)
}

// Valid reports whether the actor is an accepted Foundation value.
func (actor Actor) Valid() bool {
	switch actor {
	case ActorSystem, ActorUser, ActorPlugin:
		return true
	default:
		return false
	}
}

// Validate rejects incomplete or unsupported events before persistence.
func (event Event) Validate() error {
	if strings.TrimSpace(event.Name) == "" {
		return fmt.Errorf("audit event must not be empty")
	}
	if !event.Actor.Valid() {
		return fmt.Errorf("audit actor %q is invalid", event.Actor)
	}
	if strings.TrimSpace(event.Subject) == "" {
		return fmt.Errorf("audit subject must not be empty")
	}
	return nil
}

// SafeMetadata replaces values whose keys indicate credentials or complete
// student-authored content. Audit callers should still prefer identifiers and
// state transitions over raw content.
func SafeMetadata(metadata map[string]string) map[string]string {
	if len(metadata) == 0 {
		return map[string]string{}
	}
	result := make(map[string]string, len(metadata))
	for key, value := range metadata {
		normalized := strings.ToLower(strings.ReplaceAll(strings.ReplaceAll(key, "-", "_"), ".", "_"))
		switch {
		case containsAny(normalized, "secret", "token", "password", "credential", "api_key", "apikey"):
			result[key] = redacted
		case containsAny(normalized, "student_content", "document_content", "submission", "answer", "prompt") || normalized == "content":
			result[key] = omitted
		default:
			result[key] = value
		}
	}
	return result
}

func containsAny(value string, candidates ...string) bool {
	for _, candidate := range candidates {
		if strings.Contains(value, candidate) {
			return true
		}
	}
	return false
}
