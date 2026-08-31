package researchsearch

import (
	"errors"
	"fmt"
)

// ErrorKind is a stable, non-secret classification of a Brave Search failure.
type ErrorKind string

const (
	ErrorAuthentication ErrorKind = "authentication"
	ErrorRateLimited    ErrorKind = "rate_limited"
	ErrorInvalidRequest ErrorKind = "invalid_request"
	ErrorUnavailable    ErrorKind = "unavailable"
	ErrorResponse       ErrorKind = "response"
	ErrorTransport      ErrorKind = "transport"
)

// Error deliberately excludes request URLs, query text, response bodies, and
// credentials. RateLimit contains only bounded response-header metadata.
type Error struct {
	Kind       ErrorKind
	StatusCode int
	RateLimit  RateLimitMetadata
}

func (err *Error) Error() string {
	if err == nil {
		return "<nil>"
	}
	if err.StatusCode != 0 {
		return fmt.Sprintf("brave search %s (status %d)", err.Kind, err.StatusCode)
	}
	return "brave search " + string(err.Kind)
}

func (err *Error) Is(target error) bool {
	other, ok := target.(*Error)
	return ok && err != nil && err.Kind == other.Kind
}

var (
	ErrAuthentication error = &Error{Kind: ErrorAuthentication}
	ErrRateLimited    error = &Error{Kind: ErrorRateLimited}
	ErrInvalidRequest error = &Error{Kind: ErrorInvalidRequest}
	ErrUnavailable    error = &Error{Kind: ErrorUnavailable}
	ErrResponse       error = &Error{Kind: ErrorResponse}
	ErrTransport      error = &Error{Kind: ErrorTransport}
)

// KindOf returns a provider classification without requiring callers to
// inspect text or vendor response bodies.
func KindOf(err error) (ErrorKind, bool) {
	var target *Error
	if !errors.As(err, &target) {
		return "", false
	}
	return target.Kind, true
}
