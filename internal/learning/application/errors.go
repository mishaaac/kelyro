// Package application defines the use-case and persistence boundaries for the
// Student & Learning domain. It depends on domain values, never on a concrete
// database or presentation framework.
package application

import (
	"context"
	"errors"
	"fmt"
)

// ErrorKind is the stable classification exposed to CLI and TUI consumers.
// Infrastructure adapters should translate implementation-specific errors to
// one of these kinds before returning from a repository.
type ErrorKind string

const (
	ErrorNotFound           ErrorKind = "not_found"
	ErrorConflict           ErrorKind = "conflict"
	ErrorInvalidState       ErrorKind = "invalid_state"
	ErrorUnavailable        ErrorKind = "unavailable"
	ErrorPersistenceFailure ErrorKind = "persistence_failure"
)

// Error carries a stable kind while preserving the underlying cause for logs
// and diagnostics. Callers should branch with errors.Is or KindOf.
type Error struct {
	Kind      ErrorKind
	Operation string
	Cause     error
}

func (err *Error) Error() string {
	if err == nil {
		return "<nil>"
	}
	message := string(err.Kind)
	if err.Operation != "" {
		message = err.Operation + ": " + message
	}
	if err.Cause != nil {
		message += ": " + err.Cause.Error()
	}
	return message
}

func (err *Error) Unwrap() error {
	if err == nil {
		return nil
	}
	return err.Cause
}

func (err *Error) Is(target error) bool {
	other, ok := target.(*Error)
	return ok && err != nil && err.Kind == other.Kind
}

var (
	ErrNotFound           error = &Error{Kind: ErrorNotFound}
	ErrConflict           error = &Error{Kind: ErrorConflict}
	ErrInvalidState       error = &Error{Kind: ErrorInvalidState}
	ErrUnavailable        error = &Error{Kind: ErrorUnavailable}
	ErrPersistenceFailure error = &Error{Kind: ErrorPersistenceFailure}
)

// Classify annotates an error at an application boundary. It is also the hook
// persistence adapters use to hide driver-specific errors from callers.
func Classify(kind ErrorKind, operation string, cause error) error {
	return &Error{Kind: kind, Operation: operation, Cause: cause}
}

// KindOf returns the stable application classification carried by err.
func KindOf(err error) (ErrorKind, bool) {
	for _, candidate := range []struct {
		kind   ErrorKind
		target error
	}{
		{ErrorNotFound, ErrNotFound},
		{ErrorConflict, ErrConflict},
		{ErrorInvalidState, ErrInvalidState},
		{ErrorUnavailable, ErrUnavailable},
		{ErrorPersistenceFailure, ErrPersistenceFailure},
	} {
		if errors.Is(err, candidate.target) {
			return candidate.kind, true
		}
	}
	return "", false
}

func invalid(operation string, err error) error {
	return Classify(ErrorInvalidState, operation, err)
}

func repositoryError(operation string, err error) error {
	if err == nil {
		return nil
	}
	if kind, classified := KindOf(err); classified {
		return Classify(kind, operation, err)
	}
	if errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) {
		return Classify(ErrorUnavailable, operation, err)
	}
	return Classify(ErrorPersistenceFailure, operation, err)
}

func requireRepository(operation string, repository any) error {
	if repository == nil {
		return Classify(ErrorUnavailable, operation, fmt.Errorf("repository is not configured"))
	}
	return nil
}
