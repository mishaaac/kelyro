package application_test

import (
	"context"
	"errors"
	"testing"

	"github.com/mishaaac/kelyro/internal/research"
	"github.com/mishaaac/kelyro/internal/research/application"
	"github.com/mishaaac/kelyro/internal/research/application/memory"
)

func TestDeprecationIntelligenceRecordsExplicitEvidenceAndPreservesHistory(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	repositories := memory.New().Repositories()
	signal := appendDeprecationSignal(t, repositories, "official", application.DeprecationSignalExplicitStatement, 0.6)
	service := application.NewDeprecationIntelligenceService(
		repositories.Deprecations, repositories.Claims, repositories.Evidence,
		fixedClock{now: testTimestamp(t, 20)},
	)
	deprecatedIn := testVersion(t, "2.0.0")
	signal.DeprecatedIn = &deprecatedIn
	request := application.DeprecationAssessmentRequest{
		Subject: "Old API", Signals: []application.DeprecationSignal{signal},
	}
	result, err := service.Assess(ctx, request)
	if err != nil {
		t.Fatal(err)
	}
	if result.Record.Determination != research.DeprecationExplicitEvidence ||
		result.Record.AlgorithmVersion != research.DeprecationIntelligenceAlgorithmV1 ||
		result.Record.Replacement != "" {
		t.Fatalf("explicit assessment = %+v", result.Record)
	}
	deprecatedIn = testVersion(t, "mutated")
	stored, err := service.Get(ctx, result.Record.ID)
	if err != nil || stored.DeprecatedIn == nil || stored.DeprecatedIn.String() != "2.0.0" {
		t.Fatalf("stored assessment = (%+v, %v)", stored, err)
	}

	removedIn := testVersion(t, "3.0.0")
	removedDeprecatedIn := testVersion(t, "2.0.0")
	removedSignal := appendDeprecationSignal(t, repositories, "removed", application.DeprecationSignalExplicitStatement, 0.7)
	removedSignal.Status = research.DeprecationRemoved
	removedSignal.DeprecatedIn = &removedDeprecatedIn
	removedSignal.RemovedIn = &removedIn
	removedSignal.Replacement = "Current API"
	removed := application.DeprecationAssessmentRequest{Subject: "Old API", Signals: []application.DeprecationSignal{removedSignal}}
	service = application.NewDeprecationIntelligenceService(
		repositories.Deprecations, repositories.Claims, repositories.Evidence,
		fixedClock{now: testTimestamp(t, 21)},
	)
	if _, err := service.Assess(ctx, removed); err != nil {
		t.Fatal(err)
	}
	history, err := service.History(ctx, "Old API")
	if err != nil || len(history) != 2 || history[0].Status != research.DeprecationDeprecated ||
		history[1].Status != research.DeprecationRemoved {
		t.Fatalf("history = (%+v, %v)", history, err)
	}
}

func TestDeprecationIntelligenceMarksOnlyCorroboratedStrongInference(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	repositories := memory.New().Repositories()
	first := appendDeprecationSignal(t, repositories, "inference-one", application.DeprecationSignalStrongInference, 0.8)
	second := appendDeprecationSignal(t, repositories, "inference-two", application.DeprecationSignalStrongInference, 0.95)
	first.Status = research.DeprecationSuperseded
	first.Replacement = "Current practice"
	second.Status = first.Status
	second.Replacement = first.Replacement
	service := application.NewDeprecationIntelligenceService(
		repositories.Deprecations, repositories.Claims, repositories.Evidence,
		fixedClock{now: testTimestamp(t, 20)},
	)
	request := application.DeprecationAssessmentRequest{
		Subject: "Old practice", Signals: []application.DeprecationSignal{second, first},
	}
	result, err := service.Assess(ctx, request)
	if err != nil {
		t.Fatal(err)
	}
	if result.Record.Determination != research.DeprecationMultiSourceStrongInference ||
		len(result.Record.SourceIDs) != 2 || result.Record.SourceIDs[0].String() > result.Record.SourceIDs[1].String() {
		t.Fatalf("strong inference assessment = %+v", result.Record)
	}

	oneSource := request
	oneSource.Subject = "Uncorroborated practice"
	oneSource.Signals = oneSource.Signals[:1]
	if _, err := service.Assess(ctx, oneSource); !errors.Is(err, application.ErrInvalidState) {
		t.Fatalf("single-source inference error = %v, want invalid_state", err)
	}
	disagreement := request
	disagreement.Subject = "Disputed practice"
	disagreement.Signals = append([]application.DeprecationSignal(nil), request.Signals...)
	disagreement.Signals[1].Replacement = "Different practice"
	if _, err := service.Assess(ctx, disagreement); !errors.Is(err, application.ErrInvalidState) {
		t.Fatalf("disagreeing inference error = %v, want invalid_state", err)
	}
	mixed := request
	mixed.Subject = "Mixed evidence practice"
	mixed.Signals = append([]application.DeprecationSignal(nil), request.Signals...)
	mixed.Signals[1].Kind = application.DeprecationSignalExplicitStatement
	if _, err := service.Assess(ctx, mixed); !errors.Is(err, application.ErrInvalidState) {
		t.Fatalf("mixed signal error = %v, want invalid_state", err)
	}
}

func TestDeprecationIntelligenceRejectsAbsenceWeakAndUnrelatedClaims(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	repositories := memory.New().Repositories()
	weak := appendDeprecationSignal(t, repositories, "weak", application.DeprecationSignalStrongInference, 0.79)
	strong := appendDeprecationSignal(t, repositories, "strong", application.DeprecationSignalStrongInference, 0.9)
	service := application.NewDeprecationIntelligenceService(
		repositories.Deprecations, repositories.Claims, repositories.Evidence,
		fixedClock{now: testTimestamp(t, 20)},
	)
	request := application.DeprecationAssessmentRequest{
		Subject: "Unproven API", Signals: []application.DeprecationSignal{weak, strong},
	}
	if _, err := service.Assess(ctx, request); !errors.Is(err, application.ErrInvalidState) {
		t.Fatalf("weak inference error = %v, want invalid_state", err)
	}

	absence := request
	absence.Signals[0].Kind = application.DeprecationSignalKind("absence_from_docs")
	absence.Signals[1].Kind = application.DeprecationSignalKind("absence_from_docs")
	if _, err := service.Assess(ctx, absence); !errors.Is(err, application.ErrInvalidState) {
		t.Fatalf("absence signal error = %v, want invalid_state", err)
	}

	unrelated := appendDeprecationSignal(t, repositories, "unrelated", application.DeprecationSignalExplicitStatement, 0.9)
	claim, err := repositories.Claims.Get(ctx, unrelated.ClaimID)
	if err != nil {
		t.Fatal(err)
	}
	claim.ID = testClaimID(t, "unrelated.behavior")
	claim.Type = research.ClaimBehavior
	if err := repositories.Claims.Append(ctx, claim); err != nil {
		t.Fatal(err)
	}
	unrelated.ClaimID = claim.ID
	request = application.DeprecationAssessmentRequest{
		Subject: "Behavior claim", Signals: []application.DeprecationSignal{unrelated},
	}
	if _, err := service.Assess(ctx, request); !errors.Is(err, application.ErrInvalidState) {
		t.Fatalf("non-deprecation claim error = %v, want invalid_state", err)
	}
}

func appendDeprecationSignal(
	t *testing.T,
	repositories application.Repositories,
	suffix string,
	kind application.DeprecationSignalKind,
	confidenceValue float64,
) application.DeprecationSignal {
	t.Helper()
	ctx := context.Background()
	source := testSource(t, "deprecation-"+suffix)
	if err := repositories.Sources.Create(ctx, source); err != nil {
		t.Fatal(err)
	}
	snapshot := testSnapshot(t, source, "deprecation-"+suffix, 10)
	if err := repositories.Snapshots.Append(ctx, snapshot); err != nil {
		t.Fatal(err)
	}
	excerpt := "The old subject is no longer current: " + suffix
	evidence := research.Evidence{
		ID: testID(t, "evidence.deprecation."+suffix), SourceID: source.ID, SnapshotID: snapshot.ID,
		Location: "deprecations", Excerpt: excerpt,
		ExcerptHash: research.CanonicalEvidenceExcerptHashV1(excerpt),
		ExtractedAt: testTimestamp(t, 11), ExtractorVersion: "deprecation-fixture-v1",
	}
	if err := repositories.Evidence.Append(ctx, evidence); err != nil {
		t.Fatal(err)
	}
	topic, err := research.NewResearchTopic("Old subject", "software", "Fixture")
	if err != nil {
		t.Fatal(err)
	}
	claim := research.Claim{
		ID: testClaimID(t, "deprecation."+suffix), Topic: topic,
		Statement: "The old subject is deprecated", Type: research.ClaimDeprecation,
		Scope: "fixture API", StatusScope: research.ClaimStatusAll,
		Confidence: testConfidence(t, confidenceValue), SourceIDs: []research.SourceID{source.ID},
		EvidenceIDs: []research.ID{evidence.ID}, CreatedAt: testTimestamp(t, 12),
	}
	if err := repositories.Claims.Append(ctx, claim); err != nil {
		t.Fatal(err)
	}
	return application.DeprecationSignal{
		Kind: kind, ClaimID: claim.ID, EvidenceID: evidence.ID, SourceID: source.ID,
		Status: research.DeprecationDeprecated,
	}
}
