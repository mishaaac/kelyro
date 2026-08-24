package researchhttp

import (
	"errors"
	"fmt"
)

type ErrorKind string

const (
	ErrorInvalidRequest      ErrorKind = "invalid_request"
	ErrorBlockedAddress      ErrorKind = "blocked_address"
	ErrorRedirectLimit       ErrorKind = "redirect_limit"
	ErrorTimeout             ErrorKind = "timeout"
	ErrorHTTPStatus          ErrorKind = "http_status"
	ErrorResponseTooLarge    ErrorKind = "response_too_large"
	ErrorContentType         ErrorKind = "content_type"
	ErrorUnsupportedEncoding ErrorKind = "unsupported_encoding"
	ErrorTransport           ErrorKind = "transport"
	ErrorRateLimitHook       ErrorKind = "rate_limit_hook"
)

type Error struct {
	Kind       ErrorKind
	StatusCode int
	Cause      error
}

func (err *Error) Error() string {
	if err == nil {
		return "<nil>"
	}
	if err.StatusCode != 0 {
		return fmt.Sprintf("research HTTP %s (status %d)", err.Kind, err.StatusCode)
	}
	return "research HTTP " + string(err.Kind)
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
	ErrInvalidRequest      error = &Error{Kind: ErrorInvalidRequest}
	ErrBlockedAddress      error = &Error{Kind: ErrorBlockedAddress}
	ErrRedirectLimit       error = &Error{Kind: ErrorRedirectLimit}
	ErrTimeout             error = &Error{Kind: ErrorTimeout}
	ErrHTTPStatus          error = &Error{Kind: ErrorHTTPStatus}
	ErrResponseTooLarge    error = &Error{Kind: ErrorResponseTooLarge}
	ErrContentType         error = &Error{Kind: ErrorContentType}
	ErrUnsupportedEncoding error = &Error{Kind: ErrorUnsupportedEncoding}
	ErrTransport           error = &Error{Kind: ErrorTransport}
	ErrRateLimitHook       error = &Error{Kind: ErrorRateLimitHook}
)

func classified(kind ErrorKind, cause error) error {
	return &Error{Kind: kind, Cause: cause}
}

func statusError(code int) error {
	return &Error{Kind: ErrorHTTPStatus, StatusCode: code}
}

func KindOf(err error) (ErrorKind, bool) {
	var target *Error
	if !errors.As(err, &target) {
		return "", false
	}
	return target.Kind, true
}
