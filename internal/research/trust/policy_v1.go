package trust

import (
	"fmt"
	"strings"

	"github.com/mishaaac/kelyro/internal/research"
	registrycatalog "github.com/mishaaac/kelyro/internal/research/registry"
)

const PolicyVersionV1 = "trust-policy-v1"

type UseCase string

const (
	UseCaseGeneral               UseCase = "general"
	UseCaseLanguageSpecification UseCase = "language_specification"
	UseCaseSecurityAdvisory      UseCase = "security_advisory"
	UseCasePackageAPI            UseCase = "package_api"
	UseCaseHistoricalBehavior    UseCase = "historical_behavior"
)

func (useCase UseCase) Validate() error {
	switch useCase {
	case UseCaseGeneral, UseCaseLanguageSpecification, UseCaseSecurityAdvisory,
		UseCasePackageAPI, UseCaseHistoricalBehavior:
		return nil
	default:
		return fmt.Errorf("invalid trust use case %q", useCase)
	}
}

type Relevance string

const (
	RelevanceExact     Relevance = "exact"
	RelevanceStrong    Relevance = "strong"
	RelevancePartial   Relevance = "partial"
	RelevanceUnknown   Relevance = "unknown"
	RelevanceUnrelated Relevance = "unrelated"
)

func (relevance Relevance) Validate() error {
	switch relevance {
	case RelevanceExact, RelevanceStrong, RelevancePartial, RelevanceUnknown, RelevanceUnrelated:
		return nil
	default:
		return fmt.Errorf("invalid trust relevance %q", relevance)
	}
}

type Directness string

const (
	DirectnessPrimary    Directness = "primary"
	DirectnessSupporting Directness = "supporting"
	DirectnessIndirect   Directness = "indirect"
	DirectnessUnknown    Directness = "unknown"
)

func (directness Directness) Validate() error {
	switch directness {
	case DirectnessPrimary, DirectnessSupporting, DirectnessIndirect, DirectnessUnknown:
		return nil
	default:
		return fmt.Errorf("invalid trust directness %q", directness)
	}
}

type Stability string

const (
	StabilityStable       Stability = "stable"
	StabilityPreview      Stability = "preview"
	StabilityExperimental Stability = "experimental"
	StabilityLegacy       Stability = "legacy"
	StabilityUnknown      Stability = "unknown"
)

func (stability Stability) Validate() error {
	switch stability {
	case StabilityStable, StabilityPreview, StabilityExperimental, StabilityLegacy, StabilityUnknown:
		return nil
	default:
		return fmt.Errorf("invalid trust stability %q", stability)
	}
}

type Corroboration string

const (
	CorroborationIndependent  Corroboration = "independent"
	CorroborationSingleSource Corroboration = "single_source"
	CorroborationNone         Corroboration = "none"
	CorroborationConflicted   Corroboration = "conflicted"
	CorroborationUnknown      Corroboration = "unknown"
)

func (corroboration Corroboration) Validate() error {
	switch corroboration {
	case CorroborationIndependent, CorroborationSingleSource, CorroborationNone,
		CorroborationConflicted, CorroborationUnknown:
		return nil
	default:
		return fmt.Errorf("invalid trust corroboration %q", corroboration)
	}
}

// Input contains assessed facts, not provider output. Discovery candidates do
// not become trusted merely by being evaluated here.
type Input struct {
	Source        research.Source
	Topic         research.ResearchTopic
	Purpose       research.ResearchPurpose
	UseCase       UseCase
	Freshness     research.FreshnessState
	Relevance     Relevance
	Directness    Directness
	Stability     Stability
	Corroboration Corroboration
	Registry      *research.SourceRegistryEntry
	EvaluatedAt   research.Timestamp
}

func (input Input) Validate() error {
	checks := []struct {
		name string
		err  error
	}{
		{"source", input.Source.Validate()},
		{"topic", input.Topic.Validate()},
		{"purpose", input.Purpose.Validate()},
		{"use case", input.UseCase.Validate()},
		{"freshness", input.Freshness.Validate()},
		{"relevance", input.Relevance.Validate()},
		{"directness", input.Directness.Validate()},
		{"stability", input.Stability.Validate()},
		{"corroboration", input.Corroboration.Validate()},
		{"evaluated at", input.EvaluatedAt.Validate()},
	}
	for _, check := range checks {
		if check.err != nil {
			return fmt.Errorf("trust %s: %w", check.name, check.err)
		}
	}
	if input.Registry != nil {
		if err := input.Registry.Validate(); err != nil {
			return fmt.Errorf("trust registry entry: %w", err)
		}
		catalog, err := registrycatalog.NewCatalog([]research.SourceRegistryEntry{*input.Registry})
		if err != nil {
			return fmt.Errorf("trust registry entry: %w", err)
		}
		matched, found, err := catalog.MatchLocator(input.Source.Locator)
		if err != nil || !found || matched.ID != input.Registry.ID {
			return fmt.Errorf("trust registry entry does not match source locator")
		}
		if !registrycatalog.AppliesTo(*input.Registry, input.Topic, input.Source.Kind) {
			return fmt.Errorf("trust registry entry does not apply to source kind and topic")
		}
	}
	return nil
}

// PolicyV1 is stateless. Its zero value is ready for deterministic use.
type PolicyV1 struct{}

func (PolicyV1) Evaluate(input Input) (research.TrustDecision, error) {
	if err := input.Validate(); err != nil {
		return research.TrustDecision{}, err
	}

	tier := registryAdjustedTier(input, authorityTier(input.UseCase, input.Source))
	reasons := dimensionReasons(input, tier)
	state, outcomeReason := decide(input, tier, metadataComplete(input))
	reasons = append(reasons, outcomeReason)

	decision := research.TrustDecision{
		SourceID:    input.Source.ID,
		State:       state,
		Tier:        tier,
		Reasons:     reasons,
		Policy:      PolicyVersionV1,
		EvaluatedAt: input.EvaluatedAt,
	}
	if err := decision.Validate(); err != nil {
		return research.TrustDecision{}, fmt.Errorf("trust policy v1 output: %w", err)
	}
	return decision, nil
}

func authorityTier(useCase UseCase, source research.Source) research.AuthorityTier {
	kind := source.Kind
	switch useCase {
	case UseCaseLanguageSpecification:
		switch kind {
		case research.SourceSpecification, research.SourceStandard:
			return research.AuthorityTierA
		case research.SourceOfficialDocumentation, research.SourceCode,
			research.SourcePackageReference, research.SourceOfficialTutorial:
			return research.AuthorityTierB
		}
	case UseCaseSecurityAdvisory:
		switch kind {
		case research.SourceSpecification, research.SourceStandard,
			research.SourceOfficialDocumentation, research.SourceOfficialBlog,
			research.SourceReleaseNotes:
			return research.AuthorityTierA
		case research.SourceCode, research.SourcePackageReference:
			return research.AuthorityTierB
		}
	case UseCasePackageAPI:
		switch kind {
		case research.SourcePackageReference, research.SourceCode:
			return research.AuthorityTierA
		case research.SourceOfficialDocumentation, research.SourceSpecification,
			research.SourceStandard, research.SourceReleaseNotes,
			research.SourceOfficialTutorial:
			return research.AuthorityTierB
		}
	case UseCaseHistoricalBehavior:
		switch kind {
		case research.SourceReleaseNotes:
			return research.AuthorityTierA
		case research.SourceOfficialDocumentation, research.SourceSpecification,
			research.SourceStandard, research.SourceOfficialBlog, research.SourceCode:
			return research.AuthorityTierB
		case research.SourcePaper, research.SourceBookReference, research.SourceCommunityArticle:
			return research.AuthorityTierC
		}
	}

	switch kind {
	case research.SourceSpecification, research.SourceStandard:
		return research.AuthorityTierA
	case research.SourceOfficialDocumentation, research.SourceReleaseNotes,
		research.SourceOfficialBlog, research.SourcePackageReference,
		research.SourceOfficialTutorial, research.SourceCode:
		return research.AuthorityTierB
	case research.SourceIssueTracker, research.SourcePaper, research.SourceBookReference:
		return research.AuthorityTierC
	case research.SourceCommunityArticle, research.SourceCommunityForum, research.SourceVideo:
		return research.AuthorityTierD
	case research.SourcePlayground:
		if source.Specialization != nil && source.Specialization.Playground != nil &&
			source.Specialization.Playground.Affiliation == research.SourceAffiliationOfficial {
			return research.AuthorityTierB
		}
		return research.AuthorityTierD
	default:
		return research.AuthorityTierE
	}
}

func decide(input Input, tier research.AuthorityTier, completeMetadata bool) (research.TrustDecisionState, research.TrustReason) {
	if input.Registry != nil && input.Registry.Status == research.RegistryBlocked {
		return research.TrustRejected, reason("decision.rejected_registry_blocked", "The matched registry entry explicitly blocks this source family.")
	}
	if input.Relevance == RelevanceUnrelated || tier == research.AuthorityTierE {
		return research.TrustRejected, reason("decision.rejected_low_quality", "The source is unrelated or has unverified authority.")
	}
	if tier == research.AuthorityTierD &&
		(input.Relevance == RelevancePartial || input.Relevance == RelevanceUnknown) &&
		(input.Directness == DirectnessIndirect || input.Directness == DirectnessUnknown) &&
		(input.Corroboration == CorroborationNone || input.Corroboration == CorroborationUnknown) {
		return research.TrustRejected, reason("decision.rejected_low_quality", "Multiple weak dimensions make the source unsuitable even as a supplement.")
	}

	requiresVerification := !completeMetadata ||
		input.Freshness == research.FreshnessStale || input.Freshness == research.FreshnessUnknown ||
		input.Relevance == RelevancePartial || input.Relevance == RelevanceUnknown ||
		input.Directness == DirectnessUnknown ||
		input.Stability == StabilityPreview || input.Stability == StabilityExperimental || input.Stability == StabilityUnknown ||
		(input.Stability == StabilityLegacy && input.UseCase != UseCaseHistoricalBehavior) ||
		input.Corroboration == CorroborationNone || input.Corroboration == CorroborationUnknown ||
		input.Corroboration == CorroborationConflicted ||
		(tier == research.AuthorityTierD && input.Corroboration != CorroborationIndependent)
	if input.Registry != nil {
		requiresVerification = requiresVerification || input.Registry.Status == research.RegistryConditional ||
			input.Registry.Status == research.RegistryDeprecated ||
			(input.Registry.Status == research.RegistryHistorical && input.UseCase != UseCaseHistoricalBehavior)
	}

	if input.UseCase == UseCaseSecurityAdvisory &&
		(tierRank(tier) > tierRank(research.AuthorityTierB) ||
			input.Corroboration != CorroborationIndependent ||
			(input.Directness != DirectnessPrimary && input.Directness != DirectnessSupporting)) {
		requiresVerification = true
	}

	if requiresVerification {
		return research.TrustRequiresVerification, reason("decision.requires_verification", "The source cannot sustain the claim until the flagged dimensions are verified.")
	}
	if tier == research.AuthorityTierC || tier == research.AuthorityTierD || input.Directness != DirectnessPrimary {
		return research.TrustAcceptedSupplement, reason("decision.accepted_as_supplement", "The source is appropriate only as supporting evidence.")
	}
	return research.TrustAccepted, reason("decision.accepted", "The source is appropriate to sustain knowledge in this context.")
}

func metadataComplete(input Input) bool {
	metadata := input.Source.Metadata
	if strings.TrimSpace(metadata.Publisher) == "" {
		return false
	}
	timeSensitive := input.UseCase == UseCaseSecurityAdvisory ||
		input.UseCase == UseCasePackageAPI || input.UseCase == UseCaseHistoricalBehavior ||
		input.Purpose == research.PurposeCurrentUsage || input.Purpose == research.PurposeVersionBehavior ||
		input.Purpose == research.PurposeReleaseStatus || input.Purpose == research.PurposeDeprecationCheck ||
		input.Purpose == research.PurposeSecurityGuidance
	return !timeSensitive || metadata.PublishedAt != nil || metadata.UpdatedAt != nil
}

func dimensionReasons(input Input, tier research.AuthorityTier) []research.TrustReason {
	reasons := []research.TrustReason{
		reason("authority.tier_"+strings.ToLower(string(tier)), authorityDetail(tier)),
		reason("freshness."+string(input.Freshness), "Freshness was assessed independently of authority."),
		reason("relevance."+string(input.Relevance), "Relevance is scoped to the requested topic."),
		reason("directness."+string(input.Directness), "Directness describes how directly the source supports the claim."),
		reason("stability."+string(input.Stability), "Stability distinguishes durable guidance from preview or experimental material."),
		reason("corroboration."+string(input.Corroboration), "Corroboration records independent support or explicit conflict."),
	}
	if input.Registry != nil {
		reasons = append(reasons, reason("registry."+string(input.Registry.Status), "Registry status is contextual policy input, not evidence or an automatic trust decision."))
		for _, hint := range input.Registry.AuthorityHints {
			if hint.SourceKind == input.Source.Kind {
				reasons = append(reasons, reason("registry.authority_hint", hint.Reason))
				break
			}
		}
	}
	if metadataComplete(input) {
		reasons = append(reasons, reason("metadata.complete", "Required publisher and temporal metadata are present."))
	} else {
		reasons = append(reasons, reason("metadata.incomplete", "Required publisher or temporal metadata is missing."))
	}
	if input.UseCase == UseCaseHistoricalBehavior && input.Source.Kind == research.SourceReleaseNotes {
		reasons = append(reasons, reason("authority.historical_primary", "Release notes are primary evidence for historical behavior."))
	}
	if input.Source.Kind == research.SourcePlayground {
		affiliation := input.Source.Specialization.Playground.Affiliation
		reasons = append(reasons, reason("authority.playground_"+string(affiliation), "Playground affiliation is explicit specialized metadata; interactivity does not make it primary evidence."))
	}
	if input.UseCase == UseCaseSecurityAdvisory && input.Corroboration != CorroborationIndependent {
		reasons = append(reasons, reason("security.independent_corroboration_required", "Security guidance requires vendor or normative evidence plus independent recognized support."))
	}
	return reasons
}

func registryAdjustedTier(input Input, baseline research.AuthorityTier) research.AuthorityTier {
	if input.Registry == nil {
		return baseline
	}
	if input.Registry.Status == research.RegistryBlocked {
		return research.AuthorityTierE
	}
	for _, hint := range input.Registry.AuthorityHints {
		if hint.SourceKind == input.Source.Kind && tierRank(hint.Tier) > tierRank(baseline) {
			return hint.Tier
		}
	}
	return baseline
}

func authorityDetail(tier research.AuthorityTier) string {
	switch tier {
	case research.AuthorityTierA:
		return "Normative or official primary evidence for this use case."
	case research.AuthorityTierB:
		return "Official supporting evidence for this use case."
	case research.AuthorityTierC:
		return "Strong secondary expert evidence."
	case research.AuthorityTierD:
		return "Community evidence suitable only as a supplement."
	default:
		return "Unverified or low-authority evidence."
	}
}

func tierRank(tier research.AuthorityTier) int {
	switch tier {
	case research.AuthorityTierA:
		return 1
	case research.AuthorityTierB:
		return 2
	case research.AuthorityTierC:
		return 3
	case research.AuthorityTierD:
		return 4
	default:
		return 5
	}
}

func reason(code, detail string) research.TrustReason {
	return research.TrustReason{Code: code, Detail: detail}
}
