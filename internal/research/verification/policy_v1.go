package verification

import (
	"fmt"
	"sort"
	"strings"
	"unicode"
	"unicode/utf8"

	"github.com/mishaaac/kelyro/internal/research"
)

type Observation struct {
	Source               research.Source
	TrustDecision        *research.TrustDecision
	RegistryOrganization string
	RegistryStatus       *research.RegistryStatus
}

func (observation Observation) Validate() error {
	if err := observation.Source.Validate(); err != nil {
		return fmt.Errorf("verification source: %w", err)
	}
	if observation.TrustDecision != nil {
		if err := observation.TrustDecision.Validate(); err != nil {
			return fmt.Errorf("verification trust decision: %w", err)
		}
		if observation.TrustDecision.SourceID != observation.Source.ID {
			return fmt.Errorf("verification trust decision source does not match")
		}
	}
	if (observation.RegistryOrganization == "") != (observation.RegistryStatus == nil) {
		return fmt.Errorf("verification registry organization and status must be present together")
	}
	if observation.RegistryOrganization != "" {
		if strings.TrimSpace(observation.RegistryOrganization) != observation.RegistryOrganization {
			return fmt.Errorf("verification registry organization has surrounding whitespace")
		}
		if err := observation.RegistryStatus.Validate(); err != nil {
			return err
		}
	}
	return nil
}

type Input struct {
	ID           research.ID
	Claim        research.Claim
	Observations []Observation
	Conflicts    []research.Conflict
	VerifiedAt   research.Timestamp
}

func (input Input) Validate() error {
	if err := input.ID.Validate(); err != nil {
		return fmt.Errorf("verification input: %w", err)
	}
	if err := input.Claim.Validate(); err != nil {
		return fmt.Errorf("verification claim: %w", err)
	}
	if err := input.VerifiedAt.Validate(); err != nil {
		return fmt.Errorf("verification time: %w", err)
	}
	if input.Claim.CreatedAt.After(input.VerifiedAt) {
		return fmt.Errorf("verification claim was created after verification")
	}
	if len(input.Observations) != len(input.Claim.SourceIDs) {
		return fmt.Errorf("verification must assess every claim source")
	}
	observed := make(map[research.SourceID]struct{}, len(input.Observations))
	for index, observation := range input.Observations {
		if err := observation.Validate(); err != nil {
			return fmt.Errorf("verification observation %d: %w", index, err)
		}
		if observation.Source.CreatedAt.After(input.VerifiedAt) {
			return fmt.Errorf("verification source %d was created after verification", index)
		}
		if observation.TrustDecision != nil && observation.TrustDecision.EvaluatedAt.After(input.VerifiedAt) {
			return fmt.Errorf("verification trust decision %d is after verification", index)
		}
		if _, exists := observed[observation.Source.ID]; exists {
			return fmt.Errorf("verification repeats source %q", observation.Source.ID)
		}
		observed[observation.Source.ID] = struct{}{}
	}
	for _, sourceID := range input.Claim.SourceIDs {
		if _, exists := observed[sourceID]; !exists {
			return fmt.Errorf("verification is missing claim source %q", sourceID)
		}
	}
	for index, item := range input.Conflicts {
		if err := item.Validate(); err != nil {
			return fmt.Errorf("verification conflict %d: %w", index, err)
		}
		found := false
		for _, claimID := range item.ClaimIDs {
			if claimID == input.Claim.ID {
				found = true
				break
			}
		}
		if !found {
			return fmt.Errorf("verification conflict %d does not reference claim", index)
		}
		if item.DetectedAt.After(input.VerifiedAt) {
			return fmt.Errorf("verification conflict %d is after verification", index)
		}
	}
	return nil
}

func Verify(input Input) (research.VerificationResult, error) {
	return verify(input, research.MultiSourceVerificationAlgorithmV1)
}

// VerifyV2 closes the trust-to-verification handoff without treating a
// requires-verification decision as accepted trust. A topically anchored,
// authoritative Tier A/B source may support only a caveated result; security
// and community requirements retain the stricter v1 thresholds.
func VerifyV2(input Input) (research.VerificationResult, error) {
	return verify(input, research.MultiSourceVerificationAlgorithmV2)
}

func verify(input Input, algorithmVersion string) (research.VerificationResult, error) {
	if err := input.Validate(); err != nil {
		return research.VerificationResult{}, err
	}
	observations := append([]Observation(nil), input.Observations...)
	sort.Slice(observations, func(i, j int) bool {
		return observations[i].Source.ID.String() < observations[j].Source.ID.String()
	})
	requirement := requirementFor(input.Claim.Type)
	metrics, summary := assessSources(input.Claim, observations, algorithmVersion == research.MultiSourceVerificationAlgorithmV2)
	unresolved, loses := conflictDisposition(input.Claim.ID, input.Conflicts)
	status, reasons := decide(requirement, metrics, summary, unresolved, loses)
	if algorithmVersion == research.MultiSourceVerificationAlgorithmV2 {
		status, reasons = applyAuthoritativeCaveatV2(requirement, metrics, summary, status, reasons)
	}
	confidence, err := research.NewClaimConfidence(cappedConfidence(input.Claim.Confidence.Value(), status))
	if err != nil {
		return research.VerificationResult{}, err
	}
	sourceIDs := make([]research.SourceID, len(observations))
	for index, observation := range observations {
		sourceIDs[index] = observation.Source.ID
	}
	result := research.VerificationResult{
		ID: input.ID, ClaimID: input.Claim.ID, Status: status, Requirement: requirement,
		SourceIDs: sourceIDs, Metrics: metrics, ReasonCodes: reasons,
		Confidence: confidence, VerifiedAt: input.VerifiedAt,
		AlgorithmVersion: algorithmVersion,
	}
	if err := result.Validate(); err != nil {
		return research.VerificationResult{}, err
	}
	return result, nil
}

type sourceSummary struct {
	acceptedScoped                 int
	strongScoped                   int
	strongOrganizations            int
	securityAuthorityScoped        int
	primaryStrongScoped            int
	provisionalAuthoritativeScoped int
	provisionalPrimaryScoped       int
	explicitlyRejected             int
	unknownOrganization            bool
	multipleSameOrganization       bool
}

func assessSources(claim research.Claim, observations []Observation, allowProvisional bool) (research.VerificationMetrics, sourceSummary) {
	distribution := research.VerificationAuthorityDistribution{}
	allScopeConsistent := true
	organizations := make(map[string]struct{})
	strongOrganizations := make(map[string]struct{})
	knownOrganizationSources := 0
	summary := sourceSummary{}
	for _, observation := range observations {
		addAuthority(&distribution, observation.TrustDecision)
		scopeConsistent := sourceScopeConsistent(claim, observation.Source)
		if !scopeConsistent {
			allScopeConsistent = false
		}
		accepted, strong := supportState(observation)
		if isExplicitlyRejected(observation) {
			summary.explicitlyRejected++
		}
		if allowProvisional && scopeConsistent && provisionalAuthoritativeSupportV2(claim, observation) {
			summary.provisionalAuthoritativeScoped++
			if isPrimaryKind(observation.Source.Kind) {
				summary.provisionalPrimaryScoped++
			}
			if normalizeOrganization(observation.RegistryOrganization) == "" {
				summary.unknownOrganization = true
			}
		}
		if !accepted || !scopeConsistent {
			continue
		}
		summary.acceptedScoped++
		organization := normalizeOrganization(observation.RegistryOrganization)
		if organization == "" {
			summary.unknownOrganization = true
		} else {
			organizations[organization] = struct{}{}
			knownOrganizationSources++
		}
		if !strong {
			continue
		}
		summary.strongScoped++
		if organization != "" {
			strongOrganizations[organization] = struct{}{}
		}
		if isPrimaryKind(observation.Source.Kind) {
			summary.primaryStrongScoped++
		}
		if observation.TrustDecision.Tier == research.AuthorityTierA && isSecurityAuthorityKind(observation.Source.Kind) {
			summary.securityAuthorityScoped++
		}
	}
	summary.strongOrganizations = len(strongOrganizations)
	summary.multipleSameOrganization = knownOrganizationSources >= 2 && len(organizations) == 1
	metrics := research.VerificationMetrics{
		SourceCount: len(observations), IndependentOrganizationCount: len(organizations),
		AuthorityDistribution: distribution, ScopeConsistent: allScopeConsistent,
	}
	return metrics, summary
}

func applyAuthoritativeCaveatV2(
	requirement research.ClaimVerificationRequirement,
	metrics research.VerificationMetrics,
	summary sourceSummary,
	status research.VerificationStatus,
	reasons []research.ClaimVerificationReason,
) (research.VerificationStatus, []research.ClaimVerificationReason) {
	if status != research.VerificationInsufficient || !metrics.ScopeConsistent || summary.explicitlyRejected > 0 {
		return status, reasons
	}
	eligible := false
	switch requirement {
	case research.VerificationRequirementNormativePrimary:
		eligible = summary.provisionalPrimaryScoped > 0
	case research.VerificationRequirementProduction, research.VerificationRequirementGeneral:
		eligible = summary.provisionalAuthoritativeScoped > 0
	case research.VerificationRequirementSecurity, research.VerificationRequirementCommunity:
		return status, reasons
	}
	if !eligible {
		return status, reasons
	}
	reasons = []research.ClaimVerificationReason{research.VerificationReasonAuthoritativeCaveat}
	if summary.unknownOrganization {
		reasons = append(reasons, research.VerificationReasonOrganizationUnknown)
	}
	return research.VerificationVerifiedCaveat, reasons
}

func decide(
	requirement research.ClaimVerificationRequirement,
	metrics research.VerificationMetrics,
	summary sourceSummary,
	unresolved, loses bool,
) (research.VerificationStatus, []research.ClaimVerificationReason) {
	reasons := make([]research.ClaimVerificationReason, 0, 4)
	if unresolved {
		return research.VerificationConflicted, []research.ClaimVerificationReason{research.VerificationReasonUnresolvedConflict}
	}
	if loses {
		return research.VerificationRejected, []research.ClaimVerificationReason{research.VerificationReasonLosesResolvedConflict}
	}
	if summary.acceptedScoped == 0 && summary.explicitlyRejected == metrics.SourceCount {
		return research.VerificationRejected, []research.ClaimVerificationReason{research.VerificationReasonSourcesRejected}
	}

	status := research.VerificationInsufficient
	switch requirement {
	case research.VerificationRequirementNormativePrimary:
		switch {
		case summary.primaryStrongScoped > 0:
			status = research.VerificationVerified
			reasons = append(reasons, research.VerificationReasonPrimarySourceSufficient)
		case summary.strongScoped >= 2 && summary.strongOrganizations >= 2:
			status = research.VerificationVerified
			reasons = append(reasons, research.VerificationReasonIndependentSupport)
		case summary.acceptedScoped > 0:
			status = research.VerificationVerifiedCaveat
			reasons = append(reasons, research.VerificationReasonWeakSupport)
		default:
			reasons = append(reasons, research.VerificationReasonCorroborationMissing)
		}
	case research.VerificationRequirementProduction:
		switch {
		case summary.strongScoped >= 2 && summary.strongOrganizations >= 2:
			status = research.VerificationVerified
			reasons = append(reasons, research.VerificationReasonIndependentSupport)
		case summary.strongScoped == 1:
			status = research.VerificationVerifiedCaveat
			reasons = append(reasons, research.VerificationReasonSingleStrongSource)
		case summary.strongScoped >= 2:
			status = research.VerificationVerifiedCaveat
			reasons = append(reasons, research.VerificationReasonCorroborationMissing)
		default:
			reasons = append(reasons, research.VerificationReasonCorroborationMissing)
		}
	case research.VerificationRequirementSecurity:
		if summary.securityAuthorityScoped > 0 {
			status = research.VerificationVerified
			reasons = append(reasons, research.VerificationReasonSecurityAuthority)
		} else {
			reasons = append(reasons, research.VerificationReasonSecurityAuthorityAbsent)
		}
	case research.VerificationRequirementCommunity:
		if summary.acceptedScoped >= 2 && metrics.IndependentOrganizationCount >= 2 {
			status = research.VerificationVerified
			reasons = append(reasons, research.VerificationReasonIndependentSupport)
		} else {
			reasons = append(reasons, research.VerificationReasonCorroborationMissing)
		}
	default:
		switch {
		case summary.strongScoped > 0:
			status = research.VerificationVerified
			reasons = append(reasons, research.VerificationReasonPrimarySourceSufficient)
		case summary.acceptedScoped > 0:
			status = research.VerificationVerifiedCaveat
			reasons = append(reasons, research.VerificationReasonWeakSupport)
		default:
			reasons = append(reasons, research.VerificationReasonCorroborationMissing)
		}
	}
	if status == research.VerificationVerified && !metrics.ScopeConsistent {
		status = research.VerificationVerifiedCaveat
	}
	if !metrics.ScopeConsistent {
		reasons = appendUniqueReason(reasons, research.VerificationReasonScopeInconsistent)
	}
	if summary.multipleSameOrganization {
		reasons = appendUniqueReason(reasons, research.VerificationReasonSameOrganization)
	}
	if summary.unknownOrganization {
		reasons = appendUniqueReason(reasons, research.VerificationReasonOrganizationUnknown)
	}
	return status, reasons
}

func requirementFor(claimType research.ClaimType) research.ClaimVerificationRequirement {
	switch claimType {
	case research.ClaimDefinition, research.ClaimRequirement:
		return research.VerificationRequirementNormativePrimary
	case research.ClaimRecommendation:
		return research.VerificationRequirementProduction
	case research.ClaimSecurity:
		return research.VerificationRequirementSecurity
	case research.ClaimExample:
		return research.VerificationRequirementCommunity
	default:
		return research.VerificationRequirementGeneral
	}
}

func supportState(observation Observation) (accepted, strong bool) {
	if observation.TrustDecision == nil || registryBlocksSupport(observation.RegistryStatus) {
		return false, false
	}
	decision := observation.TrustDecision
	accepted = decision.State == research.TrustAccepted || decision.State == research.TrustAcceptedSupplement
	strong = decision.State == research.TrustAccepted &&
		(decision.Tier == research.AuthorityTierA || decision.Tier == research.AuthorityTierB)
	return accepted, accepted && strong
}

func provisionalAuthoritativeSupportV2(claim research.Claim, observation Observation) bool {
	if observation.TrustDecision == nil || registryBlocksSupport(observation.RegistryStatus) {
		return false
	}
	decision := observation.TrustDecision
	if decision.State != research.TrustRequiresVerification ||
		(decision.Tier != research.AuthorityTierA && decision.Tier != research.AuthorityTierB) ||
		!isAuthoritativeKindV2(observation.Source.Kind) {
		return false
	}
	return statementAnchoredToTopicV2(claim.Statement, claim.Topic.Subject)
}

func isAuthoritativeKindV2(kind research.SourceKind) bool {
	switch kind {
	case research.SourceSpecification, research.SourceStandard,
		research.SourceOfficialDocumentation, research.SourceReleaseNotes,
		research.SourceOfficialBlog, research.SourcePackageReference,
		research.SourceOfficialTutorial, research.SourceCode:
		return true
	default:
		return false
	}
}

func statementAnchoredToTopicV2(statement, subject string) bool {
	statementWords := verificationWordSet(verificationWords(statement))
	subjectWords := distinctVerificationWords(verificationWords(subject))
	if len(subjectWords) == 0 {
		return false
	}
	overlap := 0
	meaningful := false
	for _, word := range subjectWords {
		if _, matched := statementWords[word]; !matched {
			continue
		}
		overlap++
		if utf8.RuneCountInString(word) > 2 || strings.IndexFunc(word, unicode.IsDigit) >= 0 {
			meaningful = true
		}
	}
	required := (len(subjectWords) + 1) / 2
	if required > 3 {
		required = 3
	}
	return overlap >= required && meaningful
}

func verificationWords(value string) []string {
	var result []string
	var current strings.Builder
	flush := func() {
		if current.Len() == 0 {
			return
		}
		result = append(result, current.String())
		current.Reset()
	}
	for _, item := range strings.ToLower(value) {
		if unicode.IsLetter(item) || unicode.IsDigit(item) {
			current.WriteRune(item)
		} else {
			flush()
		}
	}
	flush()
	return result
}

func verificationWordSet(words []string) map[string]struct{} {
	result := make(map[string]struct{}, len(words))
	for _, word := range words {
		result[word] = struct{}{}
	}
	return result
}

func distinctVerificationWords(words []string) []string {
	seen := make(map[string]struct{}, len(words))
	result := make([]string, 0, len(words))
	for _, word := range words {
		if _, exists := seen[word]; exists {
			continue
		}
		seen[word] = struct{}{}
		result = append(result, word)
	}
	return result
}

func registryBlocksSupport(status *research.RegistryStatus) bool {
	return status != nil && (*status == research.RegistryBlocked || *status == research.RegistryDeprecated)
}

func isExplicitlyRejected(observation Observation) bool {
	return registryBlocksSupport(observation.RegistryStatus) ||
		(observation.TrustDecision != nil && observation.TrustDecision.State == research.TrustRejected)
}

func sourceScopeConsistent(claim research.Claim, source research.Source) bool {
	versionsMatch := claim.VersionScope != nil && source.Version != nil && *claim.VersionScope == *source.Version
	versionsConflict := claim.VersionScope != nil && source.Version != nil && *claim.VersionScope != *source.Version
	if versionsConflict {
		return false
	}
	switch source.TemporalScope {
	case research.SourceTemporalCurrent:
		return true
	case research.SourceTemporalVersionBound:
		return versionsMatch
	case research.SourceTemporalHistorical, research.SourceTemporalArchived:
		return claim.Type == research.ClaimHistorical || versionsMatch
	default:
		return false
	}
}

func conflictDisposition(claimID research.ClaimID, conflicts []research.Conflict) (unresolved, loses bool) {
	latest := make(map[string]research.Conflict)
	for _, item := range conflicts {
		ids := append([]research.ClaimID(nil), item.ClaimIDs...)
		sort.Slice(ids, func(i, j int) bool { return ids[i].String() < ids[j].String() })
		parts := make([]string, len(ids))
		for index, id := range ids {
			parts[index] = id.String()
		}
		key := strings.Join(parts, "\x00")
		current, exists := latest[key]
		if !exists || item.DetectedAt.After(current.DetectedAt) ||
			(item.DetectedAt.Time().Equal(current.DetectedAt.Time()) && item.ID.String() > current.ID.String()) {
			latest[key] = item
		}
	}
	for _, item := range latest {
		if item.Unresolved {
			unresolved = true
			continue
		}
		if item.WinningClaimID != nil && *item.WinningClaimID != claimID {
			loses = true
		}
	}
	return unresolved, loses
}

func addAuthority(distribution *research.VerificationAuthorityDistribution, decision *research.TrustDecision) {
	if decision == nil {
		distribution.Unknown++
		return
	}
	switch decision.Tier {
	case research.AuthorityTierA:
		distribution.TierA++
	case research.AuthorityTierB:
		distribution.TierB++
	case research.AuthorityTierC:
		distribution.TierC++
	case research.AuthorityTierD:
		distribution.TierD++
	case research.AuthorityTierE:
		distribution.TierE++
	}
}

func isPrimaryKind(kind research.SourceKind) bool {
	switch kind {
	case research.SourceSpecification, research.SourceStandard,
		research.SourceOfficialDocumentation, research.SourcePackageReference,
		research.SourceCode:
		return true
	default:
		return false
	}
}

func isSecurityAuthorityKind(kind research.SourceKind) bool {
	switch kind {
	case research.SourceSpecification, research.SourceStandard,
		research.SourceOfficialDocumentation, research.SourceCode:
		return true
	default:
		return false
	}
}

func normalizeOrganization(value string) string {
	return strings.ToLower(strings.Join(strings.Fields(value), " "))
}

func appendUniqueReason(values []research.ClaimVerificationReason, value research.ClaimVerificationReason) []research.ClaimVerificationReason {
	for _, existing := range values {
		if existing == value {
			return values
		}
	}
	return append(values, value)
}

func cappedConfidence(claimConfidence float64, status research.VerificationStatus) float64 {
	cap := 0.4
	switch status {
	case research.VerificationVerified:
		cap = 0.95
	case research.VerificationVerifiedCaveat:
		cap = 0.75
	case research.VerificationConflicted:
		cap = 0.3
	case research.VerificationRejected:
		cap = 0.1
	}
	if claimConfidence < cap {
		return claimConfidence
	}
	return cap
}
