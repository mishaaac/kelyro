package privacy

import (
	"context"
	"errors"
	"fmt"
)

var (
	// ErrNetworkBlocked identifies a valid network operation denied by policy.
	ErrNetworkBlocked = errors.New("network access blocked by privacy policy")
	// ErrInvalidRequest identifies unsafe or incomplete authorization metadata.
	ErrInvalidRequest = errors.New("invalid network authorization request")
)

// Purpose identifies why a component wants network access. Consumers provide
// only a stable operation identifier and a purpose; destinations, workspace
// paths, student content, and credentials never cross this boundary.
type Purpose string

const (
	ExternalResource Purpose = "external-resource"
	AIContent        Purpose = "ai-content"
	UsageTelemetry   Purpose = "usage-telemetry"
)

// Valid reports whether the purpose has an explicit Foundation policy.
func (purpose Purpose) Valid() bool {
	switch purpose {
	case ExternalResource, AIContent, UsageTelemetry:
		return true
	default:
		return false
	}
}

// Policy is the resolved opt-in state used by the network gate. AI content and
// telemetry always require the general network permission as well as their
// dedicated permission.
type Policy struct {
	AllowNetwork        bool
	AllowAIContent      bool
	AllowUsageTelemetry bool
}

// Request contains the minimum safe metadata needed for authorization.
type Request struct {
	Operation string
	Purpose   Purpose
}

// BlockedEvent is safe, bounded metadata for local diagnostic logging.
type BlockedEvent struct {
	Operation string
	Purpose   Purpose
}

// BlockedRecorder observes denials without coupling privacy policy to a log
// format or storage mechanism. Recording is best effort.
type BlockedRecorder interface {
	RecordBlocked(ctx context.Context, event BlockedEvent) error
}

// NetworkGate is the mandatory authorization boundary for future components
// that access external resources, AI providers, or telemetry endpoints.
type NetworkGate interface {
	Authorize(ctx context.Context, request Request) error
}

type networkGate struct {
	policy   Policy
	recorder BlockedRecorder
}

// NewNetworkGate creates a deny-by-default gate from already resolved policy.
func NewNetworkGate(policy Policy, recorder BlockedRecorder) NetworkGate {
	return &networkGate{policy: policy, recorder: recorder}
}

func (gate *networkGate) Authorize(ctx context.Context, request Request) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	if !validOperation(request.Operation) || !request.Purpose.Valid() {
		return ErrInvalidRequest
	}

	allowed := gate.policy.AllowNetwork
	switch request.Purpose {
	case AIContent:
		allowed = allowed && gate.policy.AllowAIContent
	case UsageTelemetry:
		allowed = allowed && gate.policy.AllowUsageTelemetry
	}
	if allowed {
		return nil
	}

	if gate.recorder != nil {
		_ = gate.recorder.RecordBlocked(ctx, BlockedEvent{
			Operation: request.Operation,
			Purpose:   request.Purpose,
		})
	}
	return blockedError{operation: request.Operation, purpose: request.Purpose}
}

type blockedError struct {
	operation string
	purpose   Purpose
}

func (err blockedError) Error() string {
	return fmt.Sprintf("%s: operation %s requires %s permission", ErrNetworkBlocked, err.operation, err.purpose)
}

func (blockedError) Unwrap() error { return ErrNetworkBlocked }

// validOperation accepts identifiers such as update.check while rejecting
// paths, URLs, arbitrary user text, and unbounded log metadata.
func validOperation(operation string) bool {
	if len(operation) == 0 || len(operation) > 80 || operation[0] < 'a' || operation[0] > 'z' {
		return false
	}
	for _, character := range operation[1:] {
		if (character >= 'a' && character <= 'z') ||
			(character >= '0' && character <= '9') ||
			character == '.' || character == '_' || character == '-' {
			continue
		}
		return false
	}
	return true
}
