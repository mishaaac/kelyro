package application_test

import (
	"context"
	"errors"
	"reflect"
	"testing"

	"github.com/mishaaac/kelyro/internal/research"
	"github.com/mishaaac/kelyro/internal/research/application"
	"github.com/mishaaac/kelyro/internal/research/application/memory"
)

func TestImpactServiceAssessesPersistedDriftBeforeExplicitPersistence(t *testing.T) {
	repositories := memory.New().Repositories()
	drift := testDrift(t)
	if err := repositories.Drift.Append(context.Background(), drift); err != nil {
		t.Fatal(err)
	}
	service := application.NewImpactService(repositories.Drift, repositories.Impact)
	request := application.ImpactAssessmentRequest{
		DriftReportID: drift.ID, AssessedAt: testTimestamp(t, 13),
		References: []research.ClaimImpactReference{{ClaimID: drift.AffectedClaims[0], FutureConceptRefs: []research.ID{testID(t, "concept.current")}}},
	}
	first, err := service.Assess(context.Background(), request)
	if err != nil {
		t.Fatal(err)
	}
	second, err := service.Assess(context.Background(), request)
	if err != nil || !reflect.DeepEqual(first, second) {
		t.Fatalf("deterministic assessment = (%+v, %+v, %v)", first, second, err)
	}
	if _, err := service.Get(context.Background(), first.ID); !errors.Is(err, application.ErrNotFound) {
		t.Fatalf("Assess persisted report: %v", err)
	}
	if err := service.Record(context.Background(), first); err != nil {
		t.Fatal(err)
	}
	if stored, err := service.Get(context.Background(), first.ID); err != nil || !reflect.DeepEqual(stored, first) {
		t.Fatalf("stored impact = (%+v, %v)", stored, err)
	}
}

func TestImpactServiceRequiresPersistedCurrentDrift(t *testing.T) {
	repositories := memory.New().Repositories()
	service := application.NewImpactService(repositories.Drift, repositories.Impact)
	_, err := service.Assess(context.Background(), application.ImpactAssessmentRequest{DriftReportID: testID(t, "drift.missing"), AssessedAt: testTimestamp(t, 13)})
	if !errors.Is(err, application.ErrNotFound) {
		t.Fatalf("Assess missing drift error = %v", err)
	}
}
