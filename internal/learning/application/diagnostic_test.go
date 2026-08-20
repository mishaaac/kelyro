package application_test

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/mishaaac/kelyro/internal/learning"
	"github.com/mishaaac/kelyro/internal/learning/application"
	"github.com/mishaaac/kelyro/internal/learning/application/memory"
)

func TestDiagnosticServiceCompletesResumesAndCreatesEvidence(t *testing.T) {
	t.Parallel()
	fixture := newDiagnosticServiceFixture(t)
	ctx := context.Background()
	view, err := fixture.service.Start(ctx, fixture.instance.ID, fixture.diagnostic)
	if err != nil || view.Item == nil || view.Attempt.Status != learning.DiagnosticInProgress {
		t.Fatalf("Start() = (%+v, %v)", view, err)
	}
	view, err = fixture.service.Submit(ctx, view.Attempt.ID, fixture.diagnostic, []string{"a"})
	if err != nil || view.Item == nil || len(view.Attempt.Observations) != 1 || !view.Result.Partial {
		t.Fatalf("first Submit() = (%+v, %v)", view, err)
	}
	resumed, err := fixture.service.Start(ctx, fixture.instance.ID, fixture.diagnostic)
	if err != nil || resumed.Attempt.ID != view.Attempt.ID || resumed.Item == nil || resumed.Item.ID != view.Item.ID {
		t.Fatalf("resumed Start() = (%+v, %v)", resumed, err)
	}
	view, err = fixture.service.Submit(ctx, view.Attempt.ID, fixture.diagnostic, []string{"b"})
	if err != nil || view.Item != nil || view.Attempt.Status != learning.DiagnosticCompleted || view.Result.Partial {
		t.Fatalf("completed Submit() = (%+v, %v)", view, err)
	}
	for _, conceptID := range []learning.ID{testID(t, "concept.a"), testID(t, "concept.b")} {
		evidence, listErr := fixture.repositories.Evidence.ListByConcept(ctx, fixture.student.ID, conceptID)
		if listErr != nil || len(evidence) != 1 || evidence[0].Type != learning.EvidenceDiagnostic || !strings.Contains(evidence[0].Source, fixture.instance.ID.String()) {
			t.Fatalf("evidence for %s = (%+v, %v)", conceptID, evidence, listErr)
		}
	}
	result, err := fixture.service.Result(ctx, view.Attempt.ID, fixture.diagnostic)
	if err != nil || result.Partial || len(result.Estimates) != 2 || result.Estimates[0].Confidence.Value() != .5 {
		t.Fatalf("Result() = (%+v, %v)", result, err)
	}
}

func TestDiagnosticServiceCanSkipButCannotSkipPartialAttempt(t *testing.T) {
	t.Parallel()
	fixture := newDiagnosticServiceFixture(t)
	ctx := context.Background()
	skipped, err := fixture.service.Skip(ctx, fixture.instance.ID, fixture.diagnostic)
	if err != nil || skipped.Attempt.Status != learning.DiagnosticSkipped || skipped.Item != nil {
		t.Fatalf("Skip() = (%+v, %v)", skipped, err)
	}
	if evidence, err := fixture.repositories.Evidence.ListByConcept(ctx, fixture.student.ID, testID(t, "concept.a")); err != nil || len(evidence) != 0 {
		t.Fatalf("skipped evidence = (%+v, %v)", evidence, err)
	}

	partial := newDiagnosticServiceFixture(t)
	view, err := partial.service.Start(ctx, partial.instance.ID, partial.diagnostic)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := partial.service.Submit(ctx, view.Attempt.ID, partial.diagnostic, []string{"a"}); err != nil {
		t.Fatal(err)
	}
	if _, err := partial.service.Skip(ctx, partial.instance.ID, partial.diagnostic); !errors.Is(err, application.ErrInvalidState) {
		t.Fatalf("Skip(partial) error = %v, want invalid state", err)
	}
}

func TestDiagnosticServiceRejectsWrongCurriculumAndDefinitionMutation(t *testing.T) {
	t.Parallel()
	fixture := newDiagnosticServiceFixture(t)
	ctx := context.Background()
	wrong := fixture.diagnostic
	wrong.Curriculum.Version = "2.0.0"
	if _, err := fixture.service.Start(ctx, fixture.instance.ID, wrong); !errors.Is(err, application.ErrInvalidState) {
		t.Fatalf("Start(wrong curriculum) error = %v", err)
	}
	view, err := fixture.service.Start(ctx, fixture.instance.ID, fixture.diagnostic)
	if err != nil {
		t.Fatal(err)
	}
	changed := fixture.diagnostic
	changed.Sections = append([]learning.DiagnosticSection(nil), changed.Sections...)
	changed.Sections[0].Items = append([]learning.DiagnosticItem(nil), changed.Sections[0].Items...)
	changed.Sections[0].Items[0].Prompt = "Mutated prompt"
	if _, err := fixture.service.Resume(ctx, view.Attempt.ID, changed); !errors.Is(err, application.ErrInvalidState) {
		t.Fatalf("Resume(mutated definition) error = %v", err)
	}
}

type diagnosticServiceFixture struct {
	service      application.DiagnosticService
	repositories application.Repositories
	student      learning.Student
	instance     learning.CurriculumInstance
	diagnostic   learning.Diagnostic
}

func newDiagnosticServiceFixture(t *testing.T) diagnosticServiceFixture {
	t.Helper()
	ctx := context.Background()
	store := memory.New()
	repositories := store.Repositories()
	now := time.Date(2026, time.August, 19, 10, 0, 0, 0, time.UTC)
	clock := func() time.Time { current := now; now = now.Add(time.Minute); return current }
	profiles := application.NewProfileService(application.NewStudentService(repositories.Students), application.WithProfileClock(clock))
	student, err := profiles.Show(ctx)
	if err != nil {
		t.Fatal(err)
	}
	goals := application.NewGoalLifecycleService(profiles, store, application.WithGoalClock(clock), application.WithGoalIDGenerator(func() (learning.ID, error) { return testID(t, "goal.diagnostic"), nil }))
	goal, err := goals.Set(ctx, goalInput(t, "Diagnostic", "Assess foundations"))
	if err != nil {
		t.Fatal(err)
	}
	curriculum := instanceTestCurriculum(t, "1.0.0")
	instances := application.NewCurriculumInstanceService(profiles, store, application.WithCurriculumInstanceClock(clock), application.WithCurriculumInstanceIDGenerator(func() (learning.ID, error) { return testID(t, "instance.diagnostic"), nil }))
	instance, err := instances.Create(ctx, goal.ID, curriculum, learning.CurriculumSourceFixture)
	if err != nil {
		t.Fatal(err)
	}
	diagnostic := applicationDiagnosticFixture(t, curriculum.Reference)
	evidenceIndex := 0
	service := application.NewDiagnosticService(profiles, store, application.WithDiagnosticClock(clock),
		application.WithDiagnosticAttemptIDGenerator(func() (learning.ID, error) { return testID(t, "attempt.diagnostic"), nil }),
		application.WithDiagnosticEvidenceIDGenerator(func() (learning.ID, error) {
			evidenceIndex++
			return testID(t, "evidence.diagnostic."+string(rune('0'+evidenceIndex))), nil
		}))
	return diagnosticServiceFixture{service: service, repositories: repositories, student: student, instance: instance, diagnostic: diagnostic}
}

func applicationDiagnosticFixture(t *testing.T, curriculum learning.CurriculumRef) learning.Diagnostic {
	t.Helper()
	choice := func(value string) learning.DiagnosticOption {
		return learning.DiagnosticOption{Value: value, Label: strings.ToUpper(value)}
	}
	diagnostic, err := learning.NewDiagnostic(learning.DiagnosticContractVersion, learning.DiagnosticScoringPolicyVersion,
		learning.DiagnosticRef{ID: testID(t, "diagnostic.foundation"), Version: "1.0.0"}, curriculum, "Foundation diagnostic",
		[]learning.DiagnosticSection{{ID: testID(t, "diagnostic.section"), Title: "Foundations", Items: []learning.DiagnosticItem{
			{ID: testID(t, "diagnostic.item.a"), ConceptID: testID(t, "concept.a"), Kind: learning.DiagnosticSingleChoice, Prompt: "Choose A", Options: []learning.DiagnosticOption{choice("a"), choice("x")}, AcceptedAnswers: []string{"a"}},
			{ID: testID(t, "diagnostic.item.b"), ConceptID: testID(t, "concept.b"), Kind: learning.DiagnosticShortAnswer, Prompt: "Write B", AcceptedAnswers: []string{"b"}},
		}}})
	if err != nil {
		t.Fatal(err)
	}
	return diagnostic
}
