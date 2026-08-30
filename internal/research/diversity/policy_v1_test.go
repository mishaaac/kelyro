package diversity

import (
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/mishaaac/kelyro/internal/research"
)

func TestPolicyV1AcceptsOneUniqueNormativeSourceWithoutDiversityChasing(t *testing.T) {
	t.Parallel()
	claim, reviews := diversityFixture(t, research.ClaimDefinition,
		reviewFixture{"standard", research.SourceStandard, research.AuthorityTierA, "Standards Body", "", PerspectiveNormative, TechnicalRoleReference})
	got, err := (PolicyV1{}).Assess(Input{Claim: claim, Sources: reviews})
	if err != nil {
		t.Fatal(err)
	}
	if got.State != StateNormativeSourceSufficient || got.EligibleSourceCount != 1 || len(got.Warnings) != 0 {
		t.Fatalf("normative assessment = %+v", got)
	}
	if !reflect.DeepEqual(got.DeferredDimensions, []DeferredDimension{DeferredGeography, DeferredLanguage}) {
		t.Fatalf("deferred dimensions = %v", got.DeferredDimensions)
	}
}

func TestPolicyV1DetectsSameOrganizationAndSharedUpstreamDependency(t *testing.T) {
	t.Parallel()
	claim, sameOrganization := diversityFixture(t, research.ClaimRecommendation,
		reviewFixture{"docs", research.SourceOfficialDocumentation, research.AuthorityTierA, "Runtime Org", "", PerspectiveMaintainer, TechnicalRoleReference},
		reviewFixture{"blog", research.SourceOfficialBlog, research.AuthorityTierB, "Runtime Org", "", PerspectiveMaintainer, TechnicalRoleExplanation})
	got, err := (PolicyV1{}).Assess(Input{Claim: claim, Sources: sameOrganization})
	if err != nil {
		t.Fatal(err)
	}
	if got.State != StateConcentrated || got.IndependentSourceCount != 1 || !hasDiversityWarning(got, WarningOrganizationConcentrated) {
		t.Fatalf("same-organization assessment = %+v", got)
	}

	claim, mirrors := diversityFixture(t, research.ClaimRecommendation,
		reviewFixture{"vendor", research.SourceOfficialDocumentation, research.AuthorityTierA, "Vendor Org", "upstream:portable-guide", PerspectiveVendor, TechnicalRoleReference},
		reviewFixture{"mirror", research.SourcePaper, research.AuthorityTierB, "Review Institute", "upstream:portable-guide", PerspectiveIndependentReview, TechnicalRoleImplementation})
	got, err = (PolicyV1{}).Assess(Input{Claim: claim, Sources: mirrors})
	if err != nil {
		t.Fatal(err)
	}
	if got.IndependentSourceCount != 1 || got.OrganizationCount != 2 || !hasDiversityWarning(got, WarningSharedDependency) {
		t.Fatalf("shared-dependency assessment = %+v", got)
	}
}

func TestPolicyV1ReportsIndependentDimensionsWithoutRequiringCosmeticVariety(t *testing.T) {
	t.Parallel()
	claim, reviews := diversityFixture(t, research.ClaimRecommendation,
		reviewFixture{"reference", research.SourceOfficialDocumentation, research.AuthorityTierA, "Runtime Org", "", PerspectiveMaintainer, TechnicalRoleReference},
		reviewFixture{"implementation", research.SourceCode, research.AuthorityTierB, "Independent Implementers", "", PerspectivePractitioner, TechnicalRoleImplementation})
	got, err := (PolicyV1{}).Assess(Input{Claim: claim, Sources: reviews})
	if err != nil {
		t.Fatal(err)
	}
	if got.State != StateSufficient || got.IndependentSourceCount != 2 || got.OrganizationCount != 2 ||
		got.SourceKindCount != 2 || got.PerspectiveCount != 2 || !got.ReferencePresent || !got.ImplementationPresent {
		t.Fatalf("independent assessment = %+v", got)
	}
	if len(got.Warnings) != 0 {
		t.Fatalf("independent warnings = %+v", got.Warnings)
	}
}

func TestPolicyV1KeepsConcentrationWarningsEvenWhenOriginsAreIndependent(t *testing.T) {
	t.Parallel()
	claim, reviews := diversityFixture(t, research.ClaimRecommendation,
		reviewFixture{"one", research.SourcePaper, research.AuthorityTierB, "Institute One", "", PerspectiveAcademic, TechnicalRoleExplanation},
		reviewFixture{"two", research.SourcePaper, research.AuthorityTierB, "Institute Two", "", PerspectiveAcademic, TechnicalRoleExplanation})
	got, err := (PolicyV1{}).Assess(Input{Claim: claim, Sources: reviews})
	if err != nil {
		t.Fatal(err)
	}
	if got.State != StateSufficient || got.IndependentSourceCount != 2 ||
		!hasDiversityWarning(got, WarningSourceKindConcentrated) ||
		!hasDiversityWarning(got, WarningPerspectiveConcentrated) ||
		!hasDiversityWarning(got, WarningReferenceAbsent) ||
		!hasDiversityWarning(got, WarningImplementationAbsent) {
		t.Fatalf("dimension assessment = %+v", got)
	}
}

func TestPolicyV1NeverAssumesUnknownIndependenceOrPerspective(t *testing.T) {
	t.Parallel()
	claim, reviews := diversityFixture(t, research.ClaimExample,
		reviewFixture{"unknown-one", research.SourceCommunityArticle, research.AuthorityTierC, "", "", PerspectiveUnknown, TechnicalRoleUnknown},
		reviewFixture{"unknown-two", research.SourceCommunityForum, research.AuthorityTierC, "", "", PerspectiveUnknown, TechnicalRoleUnknown})
	got, err := (PolicyV1{}).Assess(Input{Claim: claim, Sources: reviews})
	if err != nil {
		t.Fatal(err)
	}
	if got.State != StateConcentrated || got.IndependentSourceCount != 0 ||
		!hasDiversityWarning(got, WarningIndependenceUnknown) ||
		!hasDiversityWarning(got, WarningPerspectiveUnknown) ||
		!hasDiversityWarning(got, WarningTechnicalRoleUnknown) {
		t.Fatalf("unknown assessment = %+v", got)
	}
}

func TestPolicyV1ReportsUnknownWhenNoSourceHasAcceptedTrust(t *testing.T) {
	t.Parallel()
	claim, reviews := diversityFixture(t, research.ClaimRecommendation,
		reviewFixture{"unreviewed", research.SourceOfficialDocumentation, research.AuthorityTierB, "Runtime Org", "", PerspectiveMaintainer, TechnicalRoleReference})
	reviews[0].TrustDecision = nil
	got, err := (PolicyV1{}).Assess(Input{Claim: claim, Sources: reviews})
	if err != nil {
		t.Fatal(err)
	}
	if got.State != StateUnknown || got.EligibleSourceCount != 0 || got.IndependentSourceCount != 0 ||
		!hasDiversityWarning(got, WarningNoAcceptedSources) {
		t.Fatalf("unknown trust assessment = %+v", got)
	}
}

func TestPolicyV1IsDeterministicAndReturnsDefensiveWarnings(t *testing.T) {
	t.Parallel()
	claim, reviews := diversityFixture(t, research.ClaimRecommendation,
		reviewFixture{"a", research.SourcePaper, research.AuthorityTierB, "Same Org", "", PerspectiveAcademic, TechnicalRoleExplanation},
		reviewFixture{"b", research.SourcePaper, research.AuthorityTierB, "Same Org", "", PerspectiveAcademic, TechnicalRoleExplanation})
	forward, err := (PolicyV1{}).Assess(Input{Claim: claim, Sources: reviews})
	if err != nil {
		t.Fatal(err)
	}
	reverse, err := (PolicyV1{}).Assess(Input{Claim: claim, Sources: []SourceReview{reviews[1], reviews[0]}})
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(forward, reverse) {
		t.Fatalf("non-deterministic assessments:\n%+v\n%+v", forward, reverse)
	}
	forward.Warnings[0].SourceIDs[0] = research.SourceID{}
	again, err := (PolicyV1{}).Assess(Input{Claim: claim, Sources: reviews})
	if err != nil || again.Warnings[0].SourceIDs[0].Validate() != nil {
		t.Fatalf("warning storage leaked: (%+v,%v)", again.Warnings, err)
	}
}

func TestPolicyV1RejectsIncompleteCoverageAndDivergentOutput(t *testing.T) {
	t.Parallel()
	claim, reviews := diversityFixture(t, research.ClaimRecommendation,
		reviewFixture{"one", research.SourcePaper, research.AuthorityTierB, "Institute", "", PerspectiveAcademic, TechnicalRoleExplanation})
	if _, err := (PolicyV1{}).Assess(Input{Claim: claim, Sources: nil}); err == nil || !strings.Contains(err.Error(), "between 1") {
		t.Fatalf("missing review error = %v", err)
	}
	reviews[0].Perspective = "popular"
	if _, err := (PolicyV1{}).Assess(Input{Claim: claim, Sources: reviews}); err == nil || !strings.Contains(err.Error(), "perspective") {
		t.Fatalf("invalid perspective error = %v", err)
	}
	claim, reviews = diversityFixture(t, research.ClaimRecommendation,
		reviewFixture{"valid", research.SourcePaper, research.AuthorityTierB, "Institute", "", PerspectiveAcademic, TechnicalRoleExplanation})
	valid, err := (PolicyV1{}).Assess(Input{Claim: claim, Sources: reviews})
	if err != nil {
		t.Fatal(err)
	}
	valid.State = StateSufficient
	if err := valid.Validate(); err == nil || !strings.Contains(err.Error(), "two independent") {
		t.Fatalf("divergent state error = %v", err)
	}
}

type reviewFixture struct {
	name          string
	kind          research.SourceKind
	tier          research.AuthorityTier
	organization  string
	dependency    string
	perspective   Perspective
	technicalRole TechnicalRole
}

func diversityFixture(t *testing.T, claimType research.ClaimType, fixtures ...reviewFixture) (research.Claim, []SourceReview) {
	t.Helper()
	created := diversityTimestamp(t)
	topic, _ := research.NewResearchTopic("portable concurrency", "software", "runtime")
	confidence, _ := research.NewClaimConfidence(.8)
	claimID, _ := research.NewClaimID("claim.diversity.fixture")
	claim := research.Claim{
		ID: claimID, Topic: topic, Statement: "Reviewed sources support portable concurrency.",
		Type: claimType, Scope: "general", StatusScope: research.ClaimStatusStable,
		Confidence: confidence, CreatedAt: created,
	}
	reviews := make([]SourceReview, len(fixtures))
	for index, fixture := range fixtures {
		sourceID, _ := research.NewSourceID("source.diversity." + fixture.name)
		locator, _ := research.NewSourceLocator("https://" + fixture.name + ".example.test/resource")
		source := research.Source{
			ID: sourceID, Kind: fixture.kind, Locator: locator,
			TemporalScope: research.SourceTemporalCurrent,
			Metadata:      research.SourceMetadata{Title: "Diversity " + fixture.name}, CreatedAt: created,
		}
		decision := research.TrustDecision{
			SourceID: sourceID, State: research.TrustAccepted, Tier: fixture.tier,
			Reasons: []research.TrustReason{{Code: "fixture.accepted", Detail: "Accepted for diversity fixture."}},
			Policy:  "trust-policy-v1", EvaluatedAt: created,
		}
		reviews[index] = SourceReview{
			Source: source, TrustDecision: &decision, Organization: fixture.organization,
			DependencyGroup: fixture.dependency, Perspective: fixture.perspective,
			TechnicalRole: fixture.technicalRole,
		}
		claim.SourceIDs = append(claim.SourceIDs, sourceID)
		evidenceID, _ := research.NewID("evidence.diversity." + fixture.name)
		claim.EvidenceIDs = append(claim.EvidenceIDs, evidenceID)
	}
	return claim, reviews
}

func diversityTimestamp(t *testing.T) research.Timestamp {
	t.Helper()
	value, err := research.NewTimestamp(time.Date(2026, time.August, 27, 14, 0, 0, 0, time.UTC))
	if err != nil {
		t.Fatal(err)
	}
	return value
}

func hasDiversityWarning(assessment Assessment, code WarningCode) bool {
	for _, warning := range assessment.Warnings {
		if warning.Code == code {
			return true
		}
	}
	return false
}
