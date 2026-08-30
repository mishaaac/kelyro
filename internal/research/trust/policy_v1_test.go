package trust

import (
	"reflect"
	"testing"
	"time"

	"github.com/mishaaac/kelyro/internal/research"
	"github.com/mishaaac/kelyro/internal/research/community"
)

func TestPolicyV1AcceptsNormativeLanguageSpecification(t *testing.T) {
	input := trustTestInput(t, research.SourceSpecification)
	input.UseCase = UseCaseLanguageSpecification
	input.Corroboration = CorroborationSingleSource

	decision, err := (PolicyV1{}).Evaluate(input)
	if err != nil {
		t.Fatal(err)
	}
	if decision.State != research.TrustAccepted || decision.Tier != research.AuthorityTierA {
		t.Fatalf("decision = (%s, %s), want (accepted, A)", decision.State, decision.Tier)
	}
	assertReason(t, decision, "authority.tier_a")
	assertReason(t, decision, "decision.accepted")
	if decision.Policy != PolicyVersionV1 {
		t.Fatalf("policy = %q, want %q", decision.Policy, PolicyVersionV1)
	}
}

func TestPolicyV1KeepsNormativeSpecificationAboveImplementationEvidence(t *testing.T) {
	t.Parallel()
	specification := trustTestInput(t, research.SourceSpecification)
	specification.UseCase = UseCaseLanguageSpecification
	specification.Corroboration = CorroborationSingleSource
	implementation := specification
	implementation.Source = trustTestSource(t, research.SourceCode)

	specificationDecision, err := (PolicyV1{}).Evaluate(specification)
	if err != nil {
		t.Fatal(err)
	}
	implementationDecision, err := (PolicyV1{}).Evaluate(implementation)
	if err != nil {
		t.Fatal(err)
	}
	if specificationDecision.Tier != research.AuthorityTierA || implementationDecision.Tier != research.AuthorityTierB {
		t.Fatalf("normative/implementation tiers = %s/%s, want A/B", specificationDecision.Tier, implementationDecision.Tier)
	}
}

func TestPolicyV1DoesNotAllowCommunityOnlyEvidenceToSustainKnowledge(t *testing.T) {
	input := trustTestInput(t, research.SourceCommunityArticle)
	input.Directness = DirectnessSupporting
	input.Corroboration = CorroborationSingleSource

	decision, err := (PolicyV1{}).Evaluate(input)
	if err != nil {
		t.Fatal(err)
	}
	if decision.State != research.TrustRequiresVerification || decision.Tier != research.AuthorityTierD {
		t.Fatalf("decision = (%s, %s), want (requires_verification, D)", decision.State, decision.Tier)
	}
	assertReason(t, decision, "corroboration.single_source")
}

func TestPolicyV1FlagsStaleOfficialSourceWithoutLoweringAuthority(t *testing.T) {
	input := trustTestInput(t, research.SourceOfficialDocumentation)
	input.Freshness = research.FreshnessStale
	input.Corroboration = CorroborationIndependent

	decision, err := (PolicyV1{}).Evaluate(input)
	if err != nil {
		t.Fatal(err)
	}
	if decision.State != research.TrustRequiresVerification || decision.Tier != research.AuthorityTierB {
		t.Fatalf("decision = (%s, %s), want (requires_verification, B)", decision.State, decision.Tier)
	}
	assertReason(t, decision, "freshness.stale")
}

func TestPolicyV1ExposesOfficialHistoricalConflictAndContextualPrecedence(t *testing.T) {
	historical := trustTestInput(t, research.SourceReleaseNotes)
	historical.UseCase = UseCaseHistoricalBehavior
	historical.Purpose = research.PurposeVersionBehavior
	historical.Corroboration = CorroborationConflicted
	current := historical
	current.Source = trustTestSource(t, research.SourceOfficialDocumentation)

	historicalDecision, err := (PolicyV1{}).Evaluate(historical)
	if err != nil {
		t.Fatal(err)
	}
	currentDecision, err := (PolicyV1{}).Evaluate(current)
	if err != nil {
		t.Fatal(err)
	}
	if historicalDecision.Tier != research.AuthorityTierA || currentDecision.Tier != research.AuthorityTierB {
		t.Fatalf("historical/current tiers = %s/%s, want A/B", historicalDecision.Tier, currentDecision.Tier)
	}
	if historicalDecision.State != research.TrustRequiresVerification || currentDecision.State != research.TrustRequiresVerification {
		t.Fatalf("conflicted states = %s/%s", historicalDecision.State, currentDecision.State)
	}
	assertReason(t, historicalDecision, "authority.historical_primary")
	assertReason(t, historicalDecision, "corroboration.conflicted")
}

func TestPolicyV1RejectsLowQualitySource(t *testing.T) {
	input := trustTestInput(t, research.SourceOther)
	input.Relevance = RelevancePartial
	input.Directness = DirectnessIndirect
	input.Stability = StabilityUnknown
	input.Corroboration = CorroborationNone

	decision, err := (PolicyV1{}).Evaluate(input)
	if err != nil {
		t.Fatal(err)
	}
	if decision.State != research.TrustRejected || decision.Tier != research.AuthorityTierE {
		t.Fatalf("decision = (%s, %s), want (rejected, E)", decision.State, decision.Tier)
	}
	assertReason(t, decision, "decision.rejected_low_quality")
}

func TestPolicyV1RequiresVerificationForMissingMetadata(t *testing.T) {
	input := trustTestInput(t, research.SourceSpecification)
	input.Source.Metadata.Publisher = ""

	decision, err := (PolicyV1{}).Evaluate(input)
	if err != nil {
		t.Fatal(err)
	}
	if decision.State != research.TrustRequiresVerification || decision.Tier != research.AuthorityTierA {
		t.Fatalf("decision = (%s, %s), want (requires_verification, A)", decision.State, decision.Tier)
	}
	assertReason(t, decision, "metadata.incomplete")
}

func TestPolicyV1SecurityAdvisoryRequiresIndependentCorroboration(t *testing.T) {
	input := trustTestInput(t, research.SourceOfficialBlog)
	input.UseCase = UseCaseSecurityAdvisory
	input.Purpose = research.PurposeSecurityGuidance
	input.Corroboration = CorroborationSingleSource

	decision, err := (PolicyV1{}).Evaluate(input)
	if err != nil {
		t.Fatal(err)
	}
	if decision.State != research.TrustRequiresVerification {
		t.Fatalf("single-source security state = %s", decision.State)
	}
	assertReason(t, decision, "security.independent_corroboration_required")

	input.Corroboration = CorroborationIndependent
	decision, err = (PolicyV1{}).Evaluate(input)
	if err != nil {
		t.Fatal(err)
	}
	if decision.State != research.TrustAccepted || decision.Tier != research.AuthorityTierA {
		t.Fatalf("corroborated security decision = (%s, %s)", decision.State, decision.Tier)
	}
}

func TestPolicyV1PackageAPIPrefersReferenceOverTutorial(t *testing.T) {
	reference := trustTestInput(t, research.SourcePackageReference)
	reference.UseCase = UseCasePackageAPI
	reference.Purpose = research.PurposeCurrentUsage
	reference.Corroboration = CorroborationSingleSource
	tutorial := reference
	tutorial.Source = trustTestSource(t, research.SourceOfficialTutorial)
	tutorial.Directness = DirectnessSupporting

	referenceDecision, err := (PolicyV1{}).Evaluate(reference)
	if err != nil {
		t.Fatal(err)
	}
	tutorialDecision, err := (PolicyV1{}).Evaluate(tutorial)
	if err != nil {
		t.Fatal(err)
	}
	if referenceDecision.Tier != research.AuthorityTierA || referenceDecision.State != research.TrustAccepted {
		t.Fatalf("reference decision = (%s, %s)", referenceDecision.State, referenceDecision.Tier)
	}
	if tutorialDecision.Tier != research.AuthorityTierB || tutorialDecision.State != research.TrustAcceptedSupplement {
		t.Fatalf("tutorial decision = (%s, %s)", tutorialDecision.State, tutorialDecision.Tier)
	}
}

func TestPolicyV1UsesExplicitPlaygroundAffiliationWithoutMakingItPrimary(t *testing.T) {
	t.Parallel()
	official := trustTestInput(t, research.SourceOther)
	official.Source.Kind = research.SourcePlayground
	official.Source.Specialization = playgroundSpecialization(t, research.SourceAffiliationOfficial)
	community := official
	community.Source.Specialization = playgroundSpecialization(t, research.SourceAffiliationCommunity)

	officialDecision, err := (PolicyV1{}).Evaluate(official)
	if err != nil {
		t.Fatal(err)
	}
	communityDecision, err := (PolicyV1{}).Evaluate(community)
	if err != nil {
		t.Fatal(err)
	}
	if officialDecision.Tier != research.AuthorityTierB || officialDecision.State != research.TrustAccepted {
		t.Fatalf("official playground decision = (%s,%s)", officialDecision.State, officialDecision.Tier)
	}
	if communityDecision.Tier != research.AuthorityTierD || communityDecision.State != research.TrustAcceptedSupplement {
		t.Fatalf("community playground decision = (%s,%s)", communityDecision.State, communityDecision.Tier)
	}
	assertReason(t, officialDecision, "authority.playground_official")
	assertReason(t, communityDecision, "authority.playground_community")
}

func TestPolicyV1UsesVideoAffiliationButAlwaysKeepsVideoSupplementary(t *testing.T) {
	t.Parallel()
	official := trustTestInput(t, research.SourceVideo)
	official.Source.Video = trustVideoMetadata(official.Source.Locator, research.SourceAffiliationOfficial)
	communityVideo := trustTestInput(t, research.SourceVideo)
	communityVideo.Source.Video = trustVideoMetadata(communityVideo.Source.Locator, research.SourceAffiliationCommunity)

	officialDecision, err := (PolicyV1{}).Evaluate(official)
	if err != nil {
		t.Fatal(err)
	}
	communityDecision, err := (PolicyV1{}).Evaluate(communityVideo)
	if err != nil {
		t.Fatal(err)
	}
	if officialDecision.Tier != research.AuthorityTierB || officialDecision.State != research.TrustAcceptedSupplement {
		t.Fatalf("official video decision = (%s,%s)", officialDecision.State, officialDecision.Tier)
	}
	if communityDecision.Tier != research.AuthorityTierD || communityDecision.State != research.TrustAcceptedSupplement {
		t.Fatalf("community video decision = (%s,%s)", communityDecision.State, communityDecision.Tier)
	}
	assertReason(t, officialDecision, "authority.video_official")
	assertReason(t, communityDecision, "authority.video_community")
}

func TestPolicyV1ConsumesCommunityPolicyWithoutMakingItNormative(t *testing.T) {
	t.Parallel()
	input := trustTestInput(t, research.SourceCode)
	input.Source.ID, _ = research.NewSourceID("source.community-example")
	input.Source.Kind = research.SourceCode
	profileID, _ := research.NewID("authority.community-trust")
	profile := research.AuthorityProfile{
		ID: profileID, Version: "authority-profile/v1", Domain: "software", TopicPattern: "example/*",
		PreferredKinds: []research.SourceKind{research.SourceCode}, PreferredDomains: []string{"example.com"},
		MinimumCorroboration: 1, MinimumTier: research.AuthorityTierC, CreatedAt: input.EvaluatedAt,
	}
	assessment, err := (community.PolicyV1{}).Evaluate(community.Input{
		Source: input.Source, Topic: input.Topic, ResourceType: community.ResourceRepositoryExample,
		Contribution: community.ContributionResource, Freshness: input.Freshness, AuthorityProfile: &profile,
	})
	if err != nil {
		t.Fatal(err)
	}
	input.Community = &assessment

	decision, err := (PolicyV1{}).Evaluate(input)
	if err != nil {
		t.Fatal(err)
	}
	if decision.Tier != research.AuthorityTierC || decision.State != research.TrustAcceptedSupplement {
		t.Fatalf("community repository decision = (%s,%s)", decision.State, decision.Tier)
	}
	assertReason(t, decision, "community.recognized_supplementary")

	assessment.Contribution = community.ContributionComment
	assessment.Role = community.RoleContextOnly
	assessment.Tier = research.AuthorityTierD
	assessment.AuthorityElevated = false
	assessment.RequiresVerification = true
	assessment.Reasons[1] = community.Reason{Code: community.ReasonCommentContext, Detail: "Discussion comment remains context only."}
	input.Community = &assessment
	decision, err = (PolicyV1{}).Evaluate(input)
	if err != nil {
		t.Fatal(err)
	}
	if decision.State != research.TrustRequiresVerification || decision.Tier != research.AuthorityTierD {
		t.Fatalf("community comment decision = (%s,%s)", decision.State, decision.Tier)
	}
}

func TestPolicyV1IsDeterministicAndRejectsInvalidInput(t *testing.T) {
	input := trustTestInput(t, research.SourceStandard)
	first, err := (PolicyV1{}).Evaluate(input)
	if err != nil {
		t.Fatal(err)
	}
	second, err := (PolicyV1{}).Evaluate(input)
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(first, second) {
		t.Fatalf("non-deterministic decisions:\n%+v\n%+v", first, second)
	}

	input.Relevance = "invented"
	if _, err := (PolicyV1{}).Evaluate(input); err == nil {
		t.Fatal("Evaluate() accepted an invalid relevance")
	}
}

func TestPolicyV1UsesRegistryAsContextWithoutTreatingItAsTruth(t *testing.T) {
	blocked := trustTestInput(t, research.SourceOfficialDocumentation)
	blocked.Registry = trustRegistryEntry(t, blocked, research.RegistryBlocked, research.AuthorityTierE)
	decision, err := (PolicyV1{}).Evaluate(blocked)
	if err != nil {
		t.Fatal(err)
	}
	if decision.State != research.TrustRejected || decision.Tier != research.AuthorityTierE {
		t.Fatalf("blocked registry decision = (%s, %s)", decision.State, decision.Tier)
	}
	assertReason(t, decision, "registry.blocked")
	assertReason(t, decision, "decision.rejected_registry_blocked")

	historical := trustTestInput(t, research.SourceOfficialDocumentation)
	historical.Registry = trustRegistryEntry(t, historical, research.RegistryHistorical, research.AuthorityTierB)
	decision, err = (PolicyV1{}).Evaluate(historical)
	if err != nil {
		t.Fatal(err)
	}
	if decision.State != research.TrustRequiresVerification {
		t.Fatalf("historical current-guidance state = %s", decision.State)
	}
	historical.UseCase = UseCaseHistoricalBehavior
	decision, err = (PolicyV1{}).Evaluate(historical)
	if err != nil {
		t.Fatal(err)
	}
	if decision.State != research.TrustAccepted || decision.Tier != research.AuthorityTierB {
		t.Fatalf("historical behavior decision = (%s, %s)", decision.State, decision.Tier)
	}

	trusted := trustTestInput(t, research.SourceOfficialDocumentation)
	trusted.Registry = trustRegistryEntry(t, trusted, research.RegistryTrusted, research.AuthorityTierA)
	decision, err = (PolicyV1{}).Evaluate(trusted)
	if err != nil {
		t.Fatal(err)
	}
	if decision.Tier != research.AuthorityTierB {
		t.Fatalf("trusted registry elevated baseline tier to %s", decision.Tier)
	}
}

func trustTestInput(t *testing.T, kind research.SourceKind) Input {
	t.Helper()
	return Input{
		Source:        trustTestSource(t, kind),
		Topic:         trustTestTopic(t),
		Purpose:       research.PurposeConceptDefinition,
		UseCase:       UseCaseGeneral,
		Freshness:     research.FreshnessFresh,
		Relevance:     RelevanceExact,
		Directness:    DirectnessPrimary,
		Stability:     StabilityStable,
		Corroboration: CorroborationIndependent,
		EvaluatedAt:   trustTestTimestamp(t, time.Date(2026, time.August, 24, 18, 0, 0, 0, time.UTC)),
	}
}

func trustTestSource(t *testing.T, kind research.SourceKind) research.Source {
	t.Helper()
	published := trustTestTimestamp(t, time.Date(2026, time.August, 1, 0, 0, 0, 0, time.UTC))
	id, err := research.NewSourceID("source." + string(kind))
	if err != nil {
		t.Fatal(err)
	}
	locator, err := research.NewSourceLocator("https://example.com/" + string(kind))
	if err != nil {
		t.Fatal(err)
	}
	return research.Source{
		ID:            id,
		Kind:          kind,
		Locator:       locator,
		TemporalScope: research.SourceTemporalCurrent,
		Metadata: research.SourceMetadata{
			Title:       "Source " + string(kind),
			Publisher:   "Example Authority",
			Language:    "en",
			PublishedAt: &published,
		},
		CreatedAt: published,
	}
}

func trustTestTopic(t *testing.T) research.ResearchTopic {
	t.Helper()
	topic, err := research.NewResearchTopic("request routing", "software", "example")
	if err != nil {
		t.Fatal(err)
	}
	return topic
}

func playgroundSpecialization(t *testing.T, affiliation research.SourceAffiliation) *research.SourceSpecialization {
	t.Helper()
	locator, err := research.NewSourceLocator("https://example.com/playground/share/fixture")
	if err != nil {
		t.Fatal(err)
	}
	return &research.SourceSpecialization{
		Kind: research.SourcePlayground, AlgorithmVersion: research.SpecializedSourceMetadataV1,
		Playground: &research.PlaygroundDetails{
			Interactive: true, LanguageRuntime: "Portable runtime", Affiliation: affiliation, ShareableLocator: locator,
		},
	}
}

func trustVideoMetadata(locator research.SourceLocator, affiliation research.SourceAffiliation) *research.VideoSupplementMetadata {
	return &research.VideoSupplementMetadata{
		VideoLocator: locator, Channel: "Portable Conference", DurationSeconds: 600,
		Affiliation: affiliation, TranscriptAvailability: research.TranscriptUnknown,
		AlgorithmVersion: research.VideoSupplementMetadataV1,
	}
}

func trustTestTimestamp(t *testing.T, value time.Time) research.Timestamp {
	t.Helper()
	timestamp, err := research.NewTimestamp(value)
	if err != nil {
		t.Fatal(err)
	}
	return timestamp
}

func trustRegistryEntry(t *testing.T, input Input, status research.RegistryStatus, tier research.AuthorityTier) *research.SourceRegistryEntry {
	t.Helper()
	id, _ := research.NewID("registry.example")
	domain, _ := research.NewCanonicalDomain("example.com")
	entry := research.SourceRegistryEntry{
		ID: id, Organization: "Example Authority", CanonicalDomains: []research.CanonicalDomain{domain},
		SourceKinds:     []research.SourceKind{input.Source.Kind},
		AuthorityHints:  []research.RegistryAuthorityHint{{SourceKind: input.Source.Kind, Tier: tier, Reason: "Registry fixture hint."}},
		ResearchDomains: []string{"software"}, TopicPatterns: []string{"example/*"}, Status: status,
		AddedAt: input.Source.CreatedAt, LastReviewedAt: input.EvaluatedAt,
	}
	return &entry
}

func assertReason(t *testing.T, decision research.TrustDecision, code string) {
	t.Helper()
	for _, item := range decision.Reasons {
		if item.Code == code {
			return
		}
	}
	t.Fatalf("decision reasons do not contain %q: %+v", code, decision.Reasons)
}
