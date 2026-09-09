package application_test

import (
	"context"
	"errors"
	"testing"

	"github.com/mishaaac/kelyro/internal/curriculum/application"
)

func TestApplicationErrorClassificationPreservesKindsAndCancellation(t *testing.T) {
	t.Parallel()

	classified := application.Classify(application.ErrorConflict, "save", errors.New("duplicate"))
	if !errors.Is(classified, application.ErrConflict) {
		t.Fatalf("classified error %v does not match conflict", classified)
	}
	if kind, ok := application.KindOf(classified); !ok || kind != application.ErrorConflict {
		t.Fatalf("KindOf() = %q, %v", kind, ok)
	}
	cancelled := application.RepositoryError("load", context.Canceled)
	if !errors.Is(cancelled, application.ErrUnavailable) {
		t.Fatalf("cancellation mapped to %v, want unavailable", cancelled)
	}
	if err := application.RequireDependency("compile", "repository", nil); !errors.Is(err, application.ErrUnavailable) {
		t.Fatalf("missing dependency mapped to %v", err)
	}
}
