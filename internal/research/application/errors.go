package application

import (
	"context"
	"errors"
	"fmt"
)

type ErrorKind string

const (
	ErrorNotFound           ErrorKind = "not_found"
	ErrorConflict           ErrorKind = "conflict"
	ErrorInvalidState       ErrorKind = "invalid_state"
	ErrorUnavailable        ErrorKind = "unavailable"
	ErrorPersistenceFailure ErrorKind = "persistence_failure"
	ErrorExternalFailure    ErrorKind = "external_failure"
)

// Error carries a stable application classification while preserving its
// cause for diagnostics. Presentation callers branch with errors.Is or KindOf.
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
	ErrExternalFailure    error = &Error{Kind: ErrorExternalFailure}
)

func Classify(kind ErrorKind, operation string, cause error) error {
	return &Error{Kind: kind, Operation: operation, Cause: cause}
}

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
		{ErrorExternalFailure, ErrExternalFailure},
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
	return boundaryError(ErrorPersistenceFailure, operation, err)
}

func externalError(operation string, err error) error {
	return boundaryError(ErrorExternalFailure, operation, err)
}

func boundaryError(fallback ErrorKind, operation string, err error) error {
	if err == nil {
		return nil
	}
	if kind, classified := KindOf(err); classified {
		return Classify(kind, operation, err)
	}
	if errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) {
		return Classify(ErrorUnavailable, operation, err)
	}
	return Classify(fallback, operation, err)
}

func requireDependency(operation, name string, dependency any) error {
	if dependency == nil {
		return Classify(ErrorUnavailable, operation, fmt.Errorf("%s is not configured", name))
	}
	return nil
}
