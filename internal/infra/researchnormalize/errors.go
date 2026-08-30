package researchnormalize

import (
	"errors"
	"fmt"
)

type ErrorKind string

const (
	ErrorUnsupportedContentType ErrorKind = "unsupported_content_type"
	ErrorInvalidDocument        ErrorKind = "invalid_document"
	ErrorOutputLimit            ErrorKind = "output_limit"
)

type Error struct {
	Kind  ErrorKind
	Cause error
}

func (err *Error) Error() string {
	if err == nil {
		return "<nil>"
	}
	return "source normalization " + string(err.Kind)
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
	ErrUnsupportedContentType error = &Error{Kind: ErrorUnsupportedContentType}
	ErrInvalidDocument        error = &Error{Kind: ErrorInvalidDocument}
	ErrOutputLimit            error = &Error{Kind: ErrorOutputLimit}
)

func classified(kind ErrorKind, cause error) error { return &Error{Kind: kind, Cause: cause} }

func KindOf(err error) (ErrorKind, bool) {
	var target *Error
	if !errors.As(err, &target) {
		return "", false
	}
	switch target.Kind {
	case ErrorUnsupportedContentType, ErrorInvalidDocument, ErrorOutputLimit:
		return target.Kind, true
	default:
		return "", false
	}
}

func invalidDocument(message string) error {
	return classified(ErrorInvalidDocument, fmt.Errorf("%s", message))
}
