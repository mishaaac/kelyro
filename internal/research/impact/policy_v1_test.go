package impact_test

import (
	"testing"
	"time"

	"github.com/mishaaac/kelyro/internal/research"
	"github.com/mishaaac/kelyro/internal/research/impact"
)

func TestAnalyzeV1ProjectsExplicitReferencesAndDeduplicatesEvidence(t *testing.T) {
	drift := impactDrift(t, research.DriftVersionSuperseded, research.SeverityImportant)
	concept := impactID(t, "concept.range")
	lesson := impactID(t, "lesson.range")
	technology := impactID(t, "technology.go")
	assessment, err := impact.AnalyzeV1(impact.Input{
		Drift: drift, AssessedAt: impactTime(t, 13),
		References: []research.ClaimImpactReference{{
			ClaimID: drift.AffectedClaims[0], FutureConceptRefs: []research.ID{concept},
			FutureLessonRefs: []research.ID{lesson}, TechnologyVersionRefs: []research.TechnologyVersionReference{{TechnologyID: technology, Version: impactVersion(t, "1.24.0")}},
		}},
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(assessment.AffectedEvidenceIDs) != 2 || len(assessment.AffectedBundleIDs) != 2 {
		t.Fatalf("affected identities = %+v", assessment)
	}
	if len(assessment.FutureConceptRefs) != 1 || assessment.FutureConceptRefs[0] != concept || len(assessment.FutureLessonRefs) != 1 || assessment.FutureLessonRefs[0] != lesson {
		t.Fatalf("future references = %+v", assessment)
	}
	if assessment.RecommendedAction != research.ActionRecompileFuture || assessment.Severity != drift.Severity {
		t.Fatalf("decision = %s/%s", assessment.RecommendedAction, assessment.Severity)
	}
}

func TestAnalyzeV1ActionPrecedence(t *testing.T) {
	tests := []struct {
		name     string
		typeName research.DriftType
		severity research.Severity
		want     research.RecommendedAction
	}{
		{"informational", research.DriftSourceChanged, research.SeverityInformational, research.ActionNoAction},
		{"source", research.DriftSourceChanged, research.SeverityMinor, research.ActionReverify},
		{"semantic", research.DriftScopeChanged, research.SeverityImportant, research.ActionReviewCurriculum},
		{"version", research.DriftVersionSuperseded, research.SeverityImportant, research.ActionRecompileFuture},
		{"critical", research.DriftDeprecationIntroduced, research.SeverityCritical, research.ActionManualReview},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			assessment, err := impact.AnalyzeV1(impact.Input{Drift: impactDrift(t, test.typeName, test.severity), AssessedAt: impactTime(t, 13)})
			if err != nil || assessment.RecommendedAction != test.want {
				t.Fatalf("AnalyzeV1() = (%+v, %v), want action %s", assessment, err, test.want)
			}
		})
	}
}

func TestAnalyzeV1RejectsInferredOrInvalidRelationships(t *testing.T) {
	drift := impactDrift(t, research.DriftScopeChanged, research.SeverityImportant)
	_, err := impact.AnalyzeV1(impact.Input{Drift: drift, AssessedAt: impactTime(t, 13), References: []research.ClaimImpactReference{{ClaimID: impactClaimID(t, "unrelated")}}})
	if err == nil {
		t.Fatal("AnalyzeV1 accepted a reference for an unaffected claim")
	}
	legacy := drift
	legacy.AlgorithmVersion = research.DriftLegacyAlgorithm
	legacy.Confidence = research.ClaimConfidence{}
	if _, err := impact.AnalyzeV1(impact.Input{Drift: legacy, AssessedAt: impactTime(t, 13)}); err == nil {
		t.Fatal("AnalyzeV1 accepted legacy unversioned drift")
	}
}

func impactDrift(t *testing.T, kind research.DriftType, severity research.Severity) research.DriftReport {
	t.Helper()
	newBundle := impactID(t, "bundle.new")
	return research.DriftReport{
		ID: impactID(t, "drift.current"), OldBundleID: impactID(t, "bundle.old"), NewBundleID: &newBundle,
		Type: kind, Severity: severity, AffectedClaims: []research.ClaimID{impactClaimID(t, "claim.current")},
		OldEvidence: []research.ID{impactID(t, "evidence.old")}, NewEvidence: []research.ID{impactID(t, "evidence.old"), impactID(t, "evidence.new")},
		Confidence: impactConfidence(t, .8), DetectedAt: impactTime(t, 12), AlgorithmVersion: research.DriftAlgorithmV1,
	}
}

func impactID(t *testing.T, value string) research.ID {
	t.Helper()
	id, err := research.NewID(value)
	if err != nil {
		t.Fatal(err)
	}
	return id
}
func impactClaimID(t *testing.T, value string) research.ClaimID {
	t.Helper()
	id, err := research.NewClaimID(value)
	if err != nil {
		t.Fatal(err)
	}
	return id
}
func impactVersion(t *testing.T, value string) research.VersionIdentifier {
	t.Helper()
	version, err := research.NewVersionIdentifier(value)
	if err != nil {
		t.Fatal(err)
	}
	return version
}
func impactConfidence(t *testing.T, value float64) research.ClaimConfidence {
	t.Helper()
	score, err := research.NewClaimConfidence(value)
	if err != nil {
		t.Fatal(err)
	}
	return score
}
func impactTime(t *testing.T, hour int) research.Timestamp {
	t.Helper()
	value, err := research.NewTimestamp(time.Date(2026, 8, 29, hour, 0, 0, 0, time.UTC))
	if err != nil {
		t.Fatal(err)
	}
	return value
}
