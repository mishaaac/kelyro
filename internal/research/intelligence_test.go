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
		Status: DeprecationDeprecated, Determination: DeprecationExplicitEvidence,
		DeprecatedIn: &deprecatedIn, Replacement: "Current API",
		SourceIDs:   []SourceID{mustSourceID(t, "deprecation-notice")},
		EvidenceIDs: []ID{mustID(t, "evidence.deprecation")}, VerifiedAt: mustTimestamp(t, 11),
		AlgorithmVersion: DeprecationIntelligenceAlgorithmV1,
	}
	if err := deprecation.Validate(); err != nil {
		t.Fatalf("DeprecationRecord.Validate() error = %v", err)
	}
	deprecation.EvidenceIDs = nil
	if err := deprecation.Validate(); err == nil {
		t.Fatal("DeprecationRecord.Validate() accepted no evidence")
	}
}

func TestDeprecationDeterminationRequiresVersionedExplicitOrMultiSourceEvidence(t *testing.T) {
	t.Parallel()
	base := DeprecationRecord{
		ID: mustID(t, "deprecation.fixture"), Subject: "Old practice",
		Status: DeprecationLegacy, Determination: DeprecationMultiSourceStrongInference,
		SourceIDs:   []SourceID{mustSourceID(t, "source.one"), mustSourceID(t, "source.two")},
		EvidenceIDs: []ID{mustID(t, "evidence.one"), mustID(t, "evidence.two")},
		VerifiedAt:  mustTimestamp(t, 12), AlgorithmVersion: DeprecationIntelligenceAlgorithmV1,
	}
	if err := base.Validate(); err != nil {
		t.Fatalf("multi-source record Validate() error = %v", err)
	}
	oneSource := base
	oneSource.SourceIDs = oneSource.SourceIDs[:1]
	if err := oneSource.Validate(); err == nil {
		t.Fatal("DeprecationRecord.Validate() accepted single-source strong inference")
	}
	legacy := base
	legacy.Determination = DeprecationLegacyUnclassified
	legacy.AlgorithmVersion = DeprecationLegacyAlgorithm
	legacy.SourceIDs = legacy.SourceIDs[:1]
	legacy.EvidenceIDs = legacy.EvidenceIDs[:1]
	if err := legacy.Validate(); err != nil {
		t.Fatalf("legacy record Validate() error = %v", err)
	}
	legacy.AlgorithmVersion = DeprecationIntelligenceAlgorithmV1
	if err := legacy.Validate(); err == nil {
		t.Fatal("DeprecationRecord.Validate() accepted unclassified v1 determination")
	}
}

func TestTechnologyReleaseSupportsStablePreviewAndNonSemverIdentities(t *testing.T) {
	t.Parallel()
	releasedAt := mustTimestamp(t, 8)
	verifiedAt := mustTimestamp(t, 9)
	tests := []struct {
		name    string
		version string
		channel ReleaseChannel
		scheme  VersionScheme
	}{
		{"stable semantic", "1.25.0", ReleaseStable, VersionSemantic},
		{"preview semantic", "1.26.0-rc.1", ReleasePreview, VersionSemantic},
		{"stable date based", "2026.08", ReleaseStable, VersionDateBased},
		{"stable non semver", "go1.25", ReleaseStable, VersionOpaque},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			version, err := NewVersionIdentifier(test.version)
			if err != nil {
				t.Fatal(err)
			}
			release := TechnologyRelease{
				ID: mustID(t, "release.fixture"), TechnologyID: mustID(t, "technology.runtime"),
				Version: version, Channel: test.channel, Status: ReleaseCurrent,
				SourceIDs:  []SourceID{mustSourceID(t, "release-notes")},
				ReleasedAt: &releasedAt, VerifiedAt: verifiedAt,
			}
			if err := release.Validate(); err != nil {
				t.Fatalf("TechnologyRelease.Validate() error = %v", err)
			}
			if release.Version.Scheme() != test.scheme {
				t.Fatalf("version scheme = %q, want %q", release.Version.Scheme(), test.scheme)
			}
		})
	}
}

func TestTechnologyReleaseRejectsReleaseDateAfterVerification(t *testing.T) {
	t.Parallel()
	releasedAt := mustTimestamp(t, 10)
	version, err := NewVersionIdentifier("1.0.0")
	if err != nil {
		t.Fatal(err)
	}
	release := TechnologyRelease{
		ID: mustID(t, "release.future"), TechnologyID: mustID(t, "technology.runtime"),
		Version: version, Channel: ReleaseStable, Status: ReleaseCurrent,
		SourceIDs:  []SourceID{mustSourceID(t, "release-notes")},
		ReleasedAt: &releasedAt, VerifiedAt: mustTimestamp(t, 9),
	}
	if err := release.Validate(); err == nil {
		t.Fatal("TechnologyRelease.Validate accepted release date after verification")
	}
}

func TestTechnologyReleaseChannelAndStatusVocabularyIsClosed(t *testing.T) {
	t.Parallel()
	channels := []ReleaseChannel{
		ReleaseStable, ReleasePreview, ReleaseBeta, ReleaseRC,
		ReleaseExperimental, ReleaseNightly, ReleaseChannelUnknown,
	}
	for _, channel := range channels {
		if err := channel.Validate(); err != nil {
			t.Fatalf("ReleaseChannel(%q).Validate() error = %v", channel, err)
		}
	}
	statuses := []ReleaseStatus{
		ReleaseCurrent, ReleaseSuperseded, ReleaseLegacy, ReleaseEOL,
		ReleaseStatusUnknown,
	}
	for _, status := range statuses {
		if err := status.Validate(); err != nil {
			t.Fatalf("ReleaseStatus(%q).Validate() error = %v", status, err)
		}
	}
	if err := ReleaseChannel("canary").Validate(); err == nil {
		t.Fatal("ReleaseChannel.Validate accepted unknown channel")
	}
	if err := ReleaseStatus("maintained").Validate(); err == nil {
		t.Fatal("ReleaseStatus.Validate accepted unknown status")
	}
}

func TestFreshnessTTLHintValidatesSelectorsAndInclusiveBounds(t *testing.T) {
	t.Parallel()
	claimType := ClaimSecurity
	sourceKind := SourceReleaseNotes
	for _, days := range []int{MinimumFreshnessTTLDays, MaximumFreshnessTTLDays} {
		hint := FreshnessTTLHint{ClaimType: &claimType, SourceKind: &sourceKind, TTLDays: days}
		if err := hint.Validate(); err != nil {
			t.Fatalf("FreshnessTTLHint(%d).Validate() error = %v", days, err)
		}
	}
	for _, days := range []int{MinimumFreshnessTTLDays - 1, MaximumFreshnessTTLDays + 1} {
		if err := (FreshnessTTLHint{TTLDays: days}).Validate(); err == nil {
			t.Fatalf("FreshnessTTLHint(%d).Validate() accepted out-of-range TTL", days)
		}
	}
	invalidClaim := ClaimType("rumor")
	if err := (FreshnessTTLHint{ClaimType: &invalidClaim, TTLDays: 30}).Validate(); err == nil {
		t.Fatal("FreshnessTTLHint.Validate() accepted invalid claim selector")
	}
}

func TestConflictAndVerificationRepresentResolvedAndUnresolvedOutcomes(t *testing.T) {
	t.Parallel()

	claims := []ClaimID{mustClaimID(t, "behavior.current"), mustClaimID(t, "behavior.old")}
	conflict := Conflict{
		ID: mustID(t, "conflict.behavior"), Type: ConflictTemporalMismatch,
		ClaimIDs: claims, Confidence: mustConfidence(t, 0.5),
		Reason: "The sources apply to different times.", Unresolved: true,
		DetectedAt: mustTimestamp(t, 11), AlgorithmVersion: ConflictResolverAlgorithmV1,
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
	conflict.ClaimIDs = append(conflict.ClaimIDs, mustClaimID(t, "behavior.third"))
	if err := conflict.Validate(); err == nil {
		t.Fatal("Conflict.Validate() accepted more than two claims for resolver v1")
	}

	verification := VerificationResult{
		ID: mustID(t, "verification.behavior"), ClaimID: claims[0],
		Status: VerificationVerifiedCaveat, SourceIDs: []SourceID{mustSourceID(t, "spec")},
		Requirement: VerificationRequirementLegacy,
		ReasonCodes: []ClaimVerificationReason{VerificationReasonLegacyUnclassified},
		Confidence:  mustConfidence(t, 0.8), VerifiedAt: mustTimestamp(t, 12),
		AlgorithmVersion: VerificationLegacyAlgorithm,
	}
	if err := verification.Validate(); err != nil {
		t.Fatalf("VerificationResult.Validate() error = %v", err)
	}
	verification.Status = VerificationStatus("maybe")
	if err := verification.Validate(); err == nil {
		t.Fatal("VerificationResult.Validate() accepted invalid status")
	}
}

func TestMultiSourceVerificationResultRequiresVersionedMetricsAndReasons(t *testing.T) {
	t.Parallel()
	result := VerificationResult{
		ID: mustID(t, "verification.multi-source"), ClaimID: mustClaimID(t, "recommendation.production"),
		Status: VerificationVerified, Requirement: VerificationRequirementProduction,
		SourceIDs: []SourceID{mustSourceID(t, "docs"), mustSourceID(t, "operations")},
		Metrics: VerificationMetrics{
			SourceCount: 2, IndependentOrganizationCount: 2, ScopeConsistent: true,
			AuthorityDistribution: VerificationAuthorityDistribution{TierA: 1, TierB: 1},
		},
		ReasonCodes: []ClaimVerificationReason{VerificationReasonIndependentSupport},
		Confidence:  mustConfidence(t, 0.9), VerifiedAt: mustTimestamp(t, 12),
		AlgorithmVersion: MultiSourceVerificationAlgorithmV1,
	}
	if err := result.Validate(); err != nil {
		t.Fatalf("VerificationResult.Validate() error = %v", err)
	}
	invalidDistribution := result
	invalidDistribution.Metrics.AuthorityDistribution.TierB = 0
	if err := invalidDistribution.Validate(); err == nil {
		t.Fatal("VerificationResult.Validate() accepted incomplete authority distribution")
	}
	duplicateReason := result
	duplicateReason.ReasonCodes = append(duplicateReason.ReasonCodes, VerificationReasonIndependentSupport)
	if err := duplicateReason.Validate(); err == nil {
		t.Fatal("VerificationResult.Validate() accepted duplicate reasons")
	}
	legacyRequirement := result
	legacyRequirement.Requirement = VerificationRequirementLegacy
	if err := legacyRequirement.Validate(); err == nil {
		t.Fatal("VerificationResult.Validate() accepted legacy requirement for v1")
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
