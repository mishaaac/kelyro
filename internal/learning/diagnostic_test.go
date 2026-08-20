package learning

import (
	"strings"
	"testing"
)

func TestDiagnosticEvaluatorsAreDeterministic(t *testing.T) {
	t.Parallel()
	items := diagnosticFixture(t).Items()
	tests := []struct {
		item   DiagnosticItem
		answer []string
		want   float64
	}{
		{items[0], []string{"multiplicative"}, 1},
		{items[0], []string{"additive"}, 0},
		{items[1], []string{"6:9", "10:15"}, 1},
		{items[1], []string{"10:15"}, 0},
		{items[2], []string{"  THREE   TO two "}, 1},
		{items[3], []string{"somewhat"}, .5},
	}
	for _, test := range tests {
		score, err := test.item.Evaluate(test.answer)
		if err != nil || score.Value() != test.want {
			t.Errorf("Evaluate(%s, %v) = (%v, %v), want %v", test.item.ID, test.answer, score.Value(), err, test.want)
		}
	}
}

func TestDiagnosticResultSeparatesEstimateConfidenceAndUnknown(t *testing.T) {
	t.Parallel()
	diagnostic := diagnosticFixture(t)
	attempt, err := NewDiagnosticAttempt(mustID(t, "attempt.result"), mustID(t, "student.result"), mustID(t, "instance.result"), diagnostic, mustTimestamp(t, 10))
	if err != nil {
		t.Fatal(err)
	}
	attempt, err = attempt.Record(DiagnosticObservation{ItemID: diagnostic.Items()[0].ID, ConceptID: diagnostic.Items()[0].ConceptID, Score: mustScore(t, 1), EvidenceID: mustID(t, "evidence.objective"), AnsweredAt: mustTimestamp(t, 11)})
	if err != nil {
		t.Fatal(err)
	}
	attempt, err = attempt.Record(DiagnosticObservation{ItemID: diagnostic.Items()[3].ID, ConceptID: diagnostic.Items()[3].ConceptID, Score: mustScore(t, .5), EvidenceID: mustID(t, "evidence.self"), AnsweredAt: mustTimestamp(t, 12)})
	if err != nil {
		t.Fatal(err)
	}
	result, err := BuildDiagnosticResult(diagnostic, attempt)
	if err != nil {
		t.Fatal(err)
	}
	if !result.Partial || len(result.Estimates) != 3 {
		t.Fatalf("result = %+v", result)
	}
	if !result.Estimates[0].Known || result.Estimates[0].EstimatedMastery.Value() != 1 || result.Estimates[0].Confidence.Value() != .5 {
		t.Fatalf("objective estimate = %+v", result.Estimates[0])
	}
	if !result.Estimates[1].Known || result.Estimates[1].EstimatedMastery.Value() != .5 || result.Estimates[1].Confidence.Value() != .125 {
		t.Fatalf("self-report estimate = %+v", result.Estimates[1])
	}
	if result.Estimates[2].Known || result.Estimates[2].Confidence.Value() != 0 {
		t.Fatalf("unknown estimate = %+v", result.Estimates[2])
	}
}

func TestDiagnosticAdaptiveBranchingAndRedundancy(t *testing.T) {
	t.Parallel()
	diagnostic := diagnosticFixture(t)
	items := diagnostic.Items()
	attempt, _ := NewDiagnosticAttempt(mustID(t, "attempt.branch"), mustID(t, "student.branch"), mustID(t, "instance.branch"), diagnostic, mustTimestamp(t, 10))
	first, err := NextDiagnosticItem(diagnostic, attempt)
	if err != nil || first == nil || first.ID != items[0].ID {
		t.Fatalf("first = (%+v, %v)", first, err)
	}
	attempt, _ = attempt.Record(DiagnosticObservation{ItemID: items[0].ID, ConceptID: items[0].ConceptID, Score: mustScore(t, 0), EvidenceID: mustID(t, "evidence.branch.fail"), AnsweredAt: mustTimestamp(t, 11)})
	next, err := NextDiagnosticItem(diagnostic, attempt)
	if err != nil || next == nil || next.ID != items[3].ID {
		t.Fatalf("next after fundamental failure = (%+v, %v), want independent branch", next, err)
	}

	positive, _ := NewDiagnosticAttempt(mustID(t, "attempt.redundancy"), mustID(t, "student.redundancy"), mustID(t, "instance.redundancy"), diagnostic, mustTimestamp(t, 10))
	positive, _ = positive.Record(DiagnosticObservation{ItemID: items[0].ID, ConceptID: items[0].ConceptID, Score: mustScore(t, 1), EvidenceID: mustID(t, "evidence.positive.one"), AnsweredAt: mustTimestamp(t, 11)})
	next, _ = NextDiagnosticItem(diagnostic, positive)
	if next == nil || next.ID != items[1].ID {
		t.Fatalf("positive branch next = %+v", next)
	}
	positive, _ = positive.Record(DiagnosticObservation{ItemID: items[1].ID, ConceptID: items[1].ConceptID, Score: mustScore(t, 1), EvidenceID: mustID(t, "evidence.positive.two"), AnsweredAt: mustTimestamp(t, 12)})
	next, _ = NextDiagnosticItem(diagnostic, positive)
	if next == nil || next.ID != items[3].ID {
		t.Fatalf("redundant third item was not skipped: %+v", next)
	}
}

func TestDiagnosticFingerprintProtectsDefinitionAndAttemptLifecycle(t *testing.T) {
	t.Parallel()
	diagnostic := diagnosticFixture(t)
	fingerprint, err := DiagnosticFingerprint(diagnostic)
	if err != nil || !strings.HasPrefix(fingerprint, "sha256:") || len(fingerprint) != 71 {
		t.Fatalf("fingerprint = (%q, %v)", fingerprint, err)
	}
	changed := diagnostic
	changed.Sections = cloneDiagnosticSections(diagnostic.Sections)
	changed.Sections[0].Items[0].Prompt = "Changed prompt"
	other, err := DiagnosticFingerprint(changed)
	if err != nil || other == fingerprint {
		t.Fatalf("changed fingerprint = (%q, %v)", other, err)
	}
	attempt, _ := NewDiagnosticAttempt(mustID(t, "attempt.lifecycle"), mustID(t, "student.lifecycle"), mustID(t, "instance.lifecycle"), diagnostic, mustTimestamp(t, 10))
	skipped, err := attempt.Skip(mustTimestamp(t, 11))
	if err != nil || skipped.Status != DiagnosticSkipped || skipped.SkippedAt == nil {
		t.Fatalf("Skip() = (%+v, %v)", skipped, err)
	}
	if _, err := skipped.Complete(mustTimestamp(t, 12)); err == nil {
		t.Fatal("completed a skipped diagnostic")
	}
}

func diagnosticFixture(t *testing.T) Diagnostic {
	t.Helper()
	ratio := mustID(t, "concept.ratio")
	equivalent := mustID(t, "concept.equivalent")
	unknown := mustID(t, "concept.unknown")
	items := []DiagnosticItem{
		{ID: mustID(t, "item.fundamental"), ConceptID: ratio, Kind: DiagnosticSingleChoice, Prompt: "What comparison is a ratio?", Options: []DiagnosticOption{{Value: "multiplicative", Label: "Multiplicative"}, {Value: "additive", Label: "Additive"}}, AcceptedAnswers: []string{"multiplicative"}},
		{ID: mustID(t, "item.multiple"), ConceptID: ratio, Kind: DiagnosticMultipleChoice, Prompt: "Select equivalent ratios.", Options: []DiagnosticOption{{Value: "6:9", Label: "6:9"}, {Value: "10:15", Label: "10:15"}, {Value: "3:5", Label: "3:5"}}, AcceptedAnswers: []string{"6:9", "10:15"}, Requirements: []DiagnosticBranchRequirement{{ItemID: mustID(t, "item.fundamental"), MinimumScore: mustScore(t, 1)}}},
		{ID: mustID(t, "item.short"), ConceptID: ratio, Kind: DiagnosticShortAnswer, Prompt: "Write 3:2 in words.", AcceptedAnswers: []string{"three to two"}, Requirements: []DiagnosticBranchRequirement{{ItemID: mustID(t, "item.multiple"), MinimumScore: mustScore(t, 1)}}},
		{ID: mustID(t, "item.self"), ConceptID: equivalent, Kind: DiagnosticSelfReport, Prompt: "How comfortable are you?", Options: []DiagnosticOption{{Value: "new", Label: "New", Score: mustScore(t, 0)}, {Value: "somewhat", Label: "Somewhat", Score: mustScore(t, .5)}, {Value: "confident", Label: "Confident", Score: mustScore(t, 1)}}},
		{ID: mustID(t, "item.unknown"), ConceptID: unknown, Kind: DiagnosticSingleChoice, Prompt: "Unknown branch question", Options: []DiagnosticOption{{Value: "yes", Label: "Yes"}, {Value: "no", Label: "No"}}, AcceptedAnswers: []string{"yes"}, Requirements: []DiagnosticBranchRequirement{{ItemID: mustID(t, "item.self"), MinimumScore: mustScore(t, 1)}}},
	}
	diagnostic, err := NewDiagnostic(DiagnosticContractVersion, DiagnosticScoringPolicyVersion, DiagnosticRef{ID: mustID(t, "diagnostic.ratios"), Version: "1.0.0"}, CurriculumRef{ID: mustID(t, "curriculum.ratios"), Version: "1.0.0"}, "Ratios diagnostic", []DiagnosticSection{{ID: mustID(t, "section.core"), Title: "Core", Items: items}})
	if err != nil {
		t.Fatal(err)
	}
	return diagnostic
}
