package bundle

import (
	"fmt"
	"sort"
	"strings"

	"github.com/mishaaac/kelyro/internal/research"
)

type ClaimObservation struct {
	Claim              research.Claim
	Verification       *research.VerificationResult
	MissingEvidenceIDs []research.ID
}

type SourceObservation struct {
	Source        research.Source
	TrustDecision *research.TrustDecision
}

type FreshnessObservation struct {
	ClaimID          research.ClaimID
	State            research.FreshnessState
	Score            research.FreshnessScore
	LastVerifiedAt   research.Timestamp
	AlgorithmVersion string
}

func (observation FreshnessObservation) Validate() error {
	if err := observation.ClaimID.Validate(); err != nil {
		return err
	}
	if err := observation.State.Validate(); err != nil {
		return err
	}
	if observation.State == research.FreshnessUnknown {
		return fmt.Errorf("persisted freshness observation cannot be unknown")
	}
	if err := observation.Score.Validate(); err != nil {
		return err
	}
	if err := observation.LastVerifiedAt.Validate(); err != nil {
		return err
	}
	if strings.TrimSpace(observation.AlgorithmVersion) == "" || observation.AlgorithmVersion != strings.TrimSpace(observation.AlgorithmVersion) {
		return fmt.Errorf("freshness observation algorithm version is invalid")
	}
	return nil
}

type Input struct {
	ID               research.ID
	Request          research.ResearchRequest
	Run              research.ResearchRun
	Claims           []ClaimObservation
	Sources          []SourceObservation
	Conflicts        []research.Conflict
	Freshness        []FreshnessObservation
	MissingFreshness []research.ClaimID
	VerifiedAt       research.Timestamp
}

// AssembleV1 derives a conservative bundle state from already persisted
// records. Missing evidence or verification remains visible as incomplete;
// it is never filled with inferred content.
func AssembleV1(input Input) (research.SourceBundle, error) {
	if err := validateInput(input); err != nil {
		return research.SourceBundle{}, err
	}
	issues := make([]research.SourceBundleIssue, 0)
	claimIDs := make([]research.ClaimID, 0, len(input.Claims))
	for _, observation := range input.Claims {
		claimIDs = append(claimIDs, observation.Claim.ID)
		if len(observation.MissingEvidenceIDs) > 0 {
			issues = appendIssue(issues, research.BundleIssueMissingEvidence)
		}
		if observation.Verification == nil {
			issues = appendIssue(issues, research.BundleIssueMissingVerification)
			continue
		}
		switch observation.Verification.AlgorithmVersion {
		case research.VerificationLegacyAlgorithm:
			issues = appendIssue(issues, research.BundleIssueLegacyVerification)
		case research.MultiSourceVerificationAlgorithmV1:
			switch observation.Verification.Status {
			case research.VerificationVerified:
			case research.VerificationVerifiedCaveat:
				issues = appendIssue(issues, research.BundleIssueVerificationCaveat)
			case research.VerificationInsufficient:
				issues = appendIssue(issues, research.BundleIssueInsufficientEvidence)
			case research.VerificationConflicted:
				issues = appendIssue(issues, research.BundleIssueUnresolvedConflict)
			case research.VerificationRejected:
				issues = appendIssue(issues, research.BundleIssueRejectedClaim)
			}
		}
	}

	sources := make([]research.SourceBundleSource, 0, len(input.Sources))
	for _, observation := range input.Sources {
		item, err := research.NewSourceBundleSource(observation.Source)
		if err != nil {
			return research.SourceBundle{}, fmt.Errorf("bundle source %q: %w", observation.Source.ID, err)
		}
		if item.TemporalScope == research.SourceTemporalVersionBound &&
			(input.Request.TargetVersion == nil || item.VersionScope == nil || *input.Request.TargetVersion != *item.VersionScope) {
			item.Role = research.BundleSourceHistorical
		}
		if item.Role != research.BundleSourceHistorical && isPrimary(observation) {
			item.Role = research.BundleSourcePrimary
		}
		if item.Role == research.BundleSourceHistorical {
			issues = appendIssue(issues, research.BundleIssueNonCurrentSource)
		}
		sources = append(sources, item)
	}

	conflictIDs := make([]research.ID, 0, len(input.Conflicts))
	for _, item := range input.Conflicts {
		conflictIDs = append(conflictIDs, item.ID)
		if item.Unresolved {
			issues = appendIssue(issues, research.BundleIssueUnresolvedConflict)
		} else {
			issues = appendIssue(issues, research.BundleIssueResolvedConflict)
		}
	}

	freshness, freshnessIssues, err := aggregateFreshness(input.Claims, input.Freshness, input.MissingFreshness)
	if err != nil {
		return research.SourceBundle{}, err
	}
	for _, issue := range freshnessIssues {
		issues = appendIssue(issues, issue)
	}
	bundle := research.SourceBundle{
		ID: input.ID, RunID: input.Run.ID, Topic: input.Request.Topic,
		Purpose: input.Request.Purpose, TargetVersion: cloneVersion(input.Request.TargetVersion),
		ClaimIDs: claimIDs, Sources: sources, ConflictIDs: conflictIDs,
		Freshness: freshness, Issues: issues, VerifiedAt: input.VerifiedAt,
	}
	return research.SealSourceBundleV1(bundle)
}

func validateInput(input Input) error {
	if err := input.ID.Validate(); err != nil {
		return fmt.Errorf("bundle id: %w", err)
	}
	if err := input.Request.Validate(); err != nil {
		return fmt.Errorf("bundle request: %w", err)
	}
	if err := input.Run.Validate(); err != nil {
		return fmt.Errorf("bundle run: %w", err)
	}
	if input.Run.RequestID != input.Request.ID {
		return fmt.Errorf("bundle run does not belong to request")
	}
	if input.Run.Status != research.ResearchRunCompleted || input.Run.CompletedAt == nil {
		return fmt.Errorf("source bundle requires a completed research run")
	}
	if err := input.VerifiedAt.Validate(); err != nil {
		return fmt.Errorf("bundle verified at: %w", err)
	}
	if input.VerifiedAt.Before(*input.Run.CompletedAt) {
		return fmt.Errorf("bundle verification precedes research run completion")
	}
	if len(input.Claims) == 0 || len(input.Claims) > research.MaximumSourceBundleItems {
		return fmt.Errorf("bundle claim count must be between 1 and %d", research.MaximumSourceBundleItems)
	}
	claimSet := make(map[research.ClaimID]research.Claim, len(input.Claims))
	requiredSources := make(map[research.SourceID]struct{})
	for index, observation := range input.Claims {
		claim := observation.Claim
		if err := claim.Validate(); err != nil {
			return fmt.Errorf("bundle claim %d: %w", index, err)
		}
		if _, exists := claimSet[claim.ID]; exists {
			return fmt.Errorf("bundle contains duplicate claim %q", claim.ID)
		}
		if claim.Topic != input.Request.Topic {
			return fmt.Errorf("bundle claim %q topic does not match request", claim.ID)
		}
		if claim.CreatedAt.After(input.VerifiedAt) {
			return fmt.Errorf("bundle claim %q is after bundle", claim.ID)
		}
		if input.Request.TargetVersion != nil && claim.VersionScope != nil && *input.Request.TargetVersion != *claim.VersionScope {
			return fmt.Errorf("bundle claim %q version does not match request", claim.ID)
		}
		claimSet[claim.ID] = claim
		for _, sourceID := range claim.SourceIDs {
			requiredSources[sourceID] = struct{}{}
		}
		seenMissing := make(map[research.ID]struct{}, len(observation.MissingEvidenceIDs))
		for _, evidenceID := range observation.MissingEvidenceIDs {
			if err := evidenceID.Validate(); err != nil {
				return fmt.Errorf("bundle missing evidence: %w", err)
			}
			if _, exists := seenMissing[evidenceID]; exists {
				return fmt.Errorf("bundle repeats missing evidence %q", evidenceID)
			}
			seenMissing[evidenceID] = struct{}{}
			if !containsID(claim.EvidenceIDs, evidenceID) {
				return fmt.Errorf("missing evidence %q is not declared by claim %q", evidenceID, claim.ID)
			}
		}
		if observation.Verification != nil {
			if err := observation.Verification.Validate(); err != nil {
				return fmt.Errorf("bundle claim %q verification: %w", claim.ID, err)
			}
			if observation.Verification.ClaimID != claim.ID || !sameSourceSet(observation.Verification.SourceIDs, claim.SourceIDs) {
				return fmt.Errorf("bundle claim %q verification relationship does not match", claim.ID)
			}
			if observation.Verification.VerifiedAt.After(input.VerifiedAt) {
				return fmt.Errorf("bundle claim %q verification is after bundle", claim.ID)
			}
		}
	}
	if len(input.Sources) != len(requiredSources) {
		return fmt.Errorf("bundle sources do not match claim source union")
	}
	seenSources := make(map[research.SourceID]struct{}, len(input.Sources))
	for index, observation := range input.Sources {
		if err := observation.Source.Validate(); err != nil {
			return fmt.Errorf("bundle source %d: %w", index, err)
		}
		if _, exists := requiredSources[observation.Source.ID]; !exists {
			return fmt.Errorf("bundle source %q is not declared by a claim", observation.Source.ID)
		}
		if observation.Source.CreatedAt.After(input.VerifiedAt) {
			return fmt.Errorf("bundle source %q is after bundle", observation.Source.ID)
		}
		if _, exists := seenSources[observation.Source.ID]; exists {
			return fmt.Errorf("bundle repeats source %q", observation.Source.ID)
		}
		seenSources[observation.Source.ID] = struct{}{}
		if observation.TrustDecision != nil {
			if err := observation.TrustDecision.Validate(); err != nil {
				return fmt.Errorf("bundle source %q trust: %w", observation.Source.ID, err)
			}
			if observation.TrustDecision.SourceID != observation.Source.ID || observation.TrustDecision.EvaluatedAt.After(input.VerifiedAt) {
				return fmt.Errorf("bundle source %q trust relationship or chronology does not match", observation.Source.ID)
			}
		}
	}
	for index, item := range input.Conflicts {
		if err := item.Validate(); err != nil {
			return fmt.Errorf("bundle conflict %d: %w", index, err)
		}
		if item.DetectedAt.After(input.VerifiedAt) {
			return fmt.Errorf("bundle conflict %q is after bundle", item.ID)
		}
		relevant := false
		for _, claimID := range item.ClaimIDs {
			if _, exists := claimSet[claimID]; exists {
				relevant = true
				break
			}
		}
		if !relevant {
			return fmt.Errorf("bundle conflict %q is unrelated to its claims", item.ID)
		}
	}
	return validateFreshnessInput(input, claimSet)
}

func validateFreshnessInput(input Input, claims map[research.ClaimID]research.Claim) error {
	seen := make(map[research.ClaimID]struct{}, len(input.Freshness)+len(input.MissingFreshness))
	for _, observation := range input.Freshness {
		if err := observation.Validate(); err != nil {
			return err
		}
		if _, exists := claims[observation.ClaimID]; !exists {
			return fmt.Errorf("freshness claim %q is not in bundle", observation.ClaimID)
		}
		if _, exists := seen[observation.ClaimID]; exists {
			return fmt.Errorf("bundle repeats freshness for claim %q", observation.ClaimID)
		}
		if observation.LastVerifiedAt.After(input.VerifiedAt) {
			return fmt.Errorf("freshness for claim %q is after bundle", observation.ClaimID)
		}
		seen[observation.ClaimID] = struct{}{}
	}
	for _, claimID := range input.MissingFreshness {
		if err := claimID.Validate(); err != nil {
			return err
		}
		if _, exists := claims[claimID]; !exists {
			return fmt.Errorf("missing freshness claim %q is not in bundle", claimID)
		}
		if _, exists := seen[claimID]; exists {
			return fmt.Errorf("bundle repeats freshness for claim %q", claimID)
		}
		seen[claimID] = struct{}{}
	}
	if len(seen) != len(claims) {
		return fmt.Errorf("bundle freshness does not cover every claim")
	}
	return nil
}

func aggregateFreshness(claims []ClaimObservation, observations []FreshnessObservation, missing []research.ClaimID) (research.SourceBundleFreshness, []research.SourceBundleIssue, error) {
	score, _ := research.NewFreshnessScore(0)
	result := research.SourceBundleFreshness{
		State: research.FreshnessUnknown, Score: score,
		MissingClaimIDs:  append([]research.ClaimID(nil), missing...),
		AlgorithmVersion: research.SourceBundleFreshnessV1,
	}
	issues := make([]research.SourceBundleIssue, 0, 1)
	if len(observations) > 0 {
		minimum := observations[0].Score.Value()
		oldest := observations[0].LastVerifiedAt
		state := research.FreshnessFresh
		algorithms := make(map[string]struct{})
		for _, observation := range observations {
			if observation.Score.Value() < minimum {
				minimum = observation.Score.Value()
			}
			if observation.LastVerifiedAt.Before(oldest) {
				oldest = observation.LastVerifiedAt
			}
			if freshnessRank(observation.State) > freshnessRank(state) {
				state = observation.State
			}
			algorithms[observation.AlgorithmVersion] = struct{}{}
		}
		result.Score, _ = research.NewFreshnessScore(minimum)
		result.LastVerifiedAt = &oldest
		result.SourceAlgorithms = make([]string, 0, len(algorithms))
		for algorithm := range algorithms {
			result.SourceAlgorithms = append(result.SourceAlgorithms, algorithm)
		}
		result.State = state
	}
	if len(missing) > 0 {
		result.State = research.FreshnessUnknown
		issues = append(issues, research.BundleIssueMissingFreshness)
	} else {
		switch result.State {
		case research.FreshnessAging:
			issues = append(issues, research.BundleIssueAgingFreshness)
		case research.FreshnessStale:
			issues = append(issues, research.BundleIssueStaleFreshness)
		}
	}
	if err := result.Validate(); err != nil {
		return research.SourceBundleFreshness{}, nil, err
	}
	return result, issues, nil
}

func freshnessRank(state research.FreshnessState) int {
	switch state {
	case research.FreshnessFresh:
		return 0
	case research.FreshnessAging:
		return 1
	case research.FreshnessStale:
		return 2
	default:
		return 3
	}
}

func isPrimary(observation SourceObservation) bool {
	decision := observation.TrustDecision
	if decision == nil || decision.State != research.TrustAccepted ||
		(decision.Tier != research.AuthorityTierA && decision.Tier != research.AuthorityTierB) {
		return false
	}
	switch observation.Source.Kind {
	case research.SourceSpecification, research.SourceStandard,
		research.SourceOfficialDocumentation, research.SourcePackageReference,
		research.SourceReleaseNotes, research.SourceCode:
		return true
	default:
		return false
	}
}

func appendIssue(issues []research.SourceBundleIssue, issue research.SourceBundleIssue) []research.SourceBundleIssue {
	for _, existing := range issues {
		if existing == issue {
			return issues
		}
	}
	return append(issues, issue)
}

func sameSourceSet(left, right []research.SourceID) bool {
	if len(left) != len(right) {
		return false
	}
	values := make(map[research.SourceID]struct{}, len(left))
	for _, id := range left {
		values[id] = struct{}{}
	}
	for _, id := range right {
		if _, exists := values[id]; !exists {
			return false
		}
	}
	return true
}

func containsID(values []research.ID, target research.ID) bool {
	for _, value := range values {
		if value == target {
			return true
		}
	}
	return false
}

func cloneVersion(version *research.SourceVersion) *research.SourceVersion {
	if version == nil {
		return nil
	}
	clone := *version
	return &clone
}

// LatestConflicts returns one immutable latest decision per canonical Claim
// pair, with deterministic output ordering.
func LatestConflicts(values []research.Conflict) []research.Conflict {
	latest := make(map[string]research.Conflict)
	for _, item := range values {
		claims := append([]research.ClaimID(nil), item.ClaimIDs...)
		sort.Slice(claims, func(i, j int) bool { return claims[i].String() < claims[j].String() })
		parts := make([]string, len(claims))
		for index, id := range claims {
			parts[index] = id.String()
		}
		key := strings.Join(parts, "\x00")
		current, exists := latest[key]
		if !exists || item.DetectedAt.After(current.DetectedAt) ||
			(item.DetectedAt.Time().Equal(current.DetectedAt.Time()) && item.ID.String() > current.ID.String()) {
			latest[key] = item
		}
	}
	result := make([]research.Conflict, 0, len(latest))
	for _, item := range latest {
		result = append(result, item)
	}
	sort.Slice(result, func(i, j int) bool { return result[i].ID.String() < result[j].ID.String() })
	return result
}
