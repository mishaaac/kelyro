package research

import "testing"

func TestFreshnessReleaseAndDeprecationVocabularyValidatesState(t *testing.T) {
	t.Parallel()

	for _, state := range []FreshnessState{FreshnessFresh, FreshnessAging, FreshnessStale, FreshnessUnknown} {
		if err := state.Validate(); err != nil {
			t.Errorf("%q.Validate() error = %v", state, err)
		}
	}
	if err := FreshnessState("recent-ish").Validate(); err == nil {
		t.Fatal("FreshnessState.Validate() accepted invalid state")
	}

	releasedAt := mustTimestamp(t, 9)
	release := ReleaseRecord{
		ID: mustID(t, "release.go1.25"), TechnologyID: mustID(t, "technology.go"),
		Version: mustVersion(t, "go1.25"), Channel: ReleaseStable, Status: ReleaseCurrent,
		SourceIDs:  []SourceID{mustSourceID(t, "release-notes")},
		ReleasedAt: &releasedAt, VerifiedAt: mustTimestamp(t, 10),
	}
	if err := release.Validate(); err != nil {
		t.Fatalf("ReleaseRecord.Validate() error = %v", err)
	}
	release.Version = ""
	if err := release.Validate(); err == nil {
		t.Fatal("ReleaseRecord.Validate() accepted empty version")
	}

	deprecatedIn := mustVersion(t, "2.0")
	deprecation := DeprecationRecord{
		ID: mustID(t, "deprecation.api"), Subject: "Legacy API",
		Status: DeprecationDeprecated, DeprecatedIn: &deprecatedIn, Replacement: "Current API",
		SourceIDs:   []SourceID{mustSourceID(t, "deprecation-notice")},
		EvidenceIDs: []ID{mustID(t, "evidence.deprecation")}, VerifiedAt: mustTimestamp(t, 11),
	}
	if err := deprecation.Validate(); err != nil {
		t.Fatalf("DeprecationRecord.Validate() error = %v", err)
	}
	deprecation.EvidenceIDs = nil
	if err := deprecation.Validate(); err == nil {
		t.Fatal("DeprecationRecord.Validate() accepted no evidence")
	}
}

func TestConflictAndVerificationRepresentResolvedAndUnresolvedOutcomes(t *testing.T) {
	t.Parallel()

	claims := []ClaimID{mustClaimID(t, "behavior.current"), mustClaimID(t, "behavior.old")}
	conflict := Conflict{
		ID: mustID(t, "conflict.behavior"), Type: ConflictTemporalMismatch,
		ClaimIDs: claims, Unresolved: true, DetectedAt: mustTimestamp(t, 11),
	}
	if err := conflict.Validate(); err != nil {
		t.Fatalf("Conflict.Validate() error = %v", err)
	}
	conflict.Unresolved = false
	if err := conflict.Validate(); err == nil {
		t.Fatal("Conflict.Validate() accepted resolved conflict without explanation")
	}
	conflict.Resolution = "The claims apply to different versions."
	if err := conflict.Validate(); err != nil {
		t.Fatalf("resolved Conflict.Validate() error = %v", err)
	}

	verification := VerificationResult{
		ID: mustID(t, "verification.behavior"), ClaimID: claims[0],
		Status: VerificationVerifiedCaveat, SourceIDs: []SourceID{mustSourceID(t, "spec")},
		Confidence: mustConfidence(t, 0.8), VerifiedAt: mustTimestamp(t, 12),
	}
	if err := verification.Validate(); err != nil {
		t.Fatalf("VerificationResult.Validate() error = %v", err)
	}
	verification.Status = VerificationStatus("maybe")
	if err := verification.Validate(); err == nil {
		t.Fatal("VerificationResult.Validate() accepted invalid status")
	}
}

func TestDriftAndImpactReportsKeepExplicitAffectedRelationships(t *testing.T) {
	t.Parallel()

	claimID := mustClaimID(t, "behavior")
	newBundleID := mustID(t, "bundle.new")
	drift := DriftReport{
		ID: mustID(t, "drift.behavior"), OldBundleID: mustID(t, "bundle.old"),
		NewBundleID: &newBundleID, Type: DriftRecommendationChanged, Severity: SeverityImportant,
		AffectedClaims: []ClaimID{claimID}, OldEvidence: []ID{mustID(t, "evidence.old")},
		NewEvidence: []ID{mustID(t, "evidence.new")}, DetectedAt: mustTimestamp(t, 12),
	}
	if err := drift.Validate(); err != nil {
		t.Fatalf("DriftReport.Validate() error = %v", err)
	}

	impact := ImpactReport{
		ID: mustID(t, "impact.behavior"), DriftReportID: drift.ID,
		AffectedBundleIDs: []ID{drift.OldBundleID}, AffectedClaimIDs: []ClaimID{claimID},
		Severity: SeverityImportant, RecommendedAction: ActionReviewCurriculum,
		AssessedAt: mustTimestamp(t, 13),
	}
	if err := impact.Validate(); err != nil {
		t.Fatalf("ImpactReport.Validate() error = %v", err)
	}
	impact.AffectedClaimIDs = nil
	if err := impact.Validate(); err == nil {
		t.Fatal("ImpactReport.Validate() accepted no affected claims")
	}
}
