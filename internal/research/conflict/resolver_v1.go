package conflict

import (
	"fmt"

	"github.com/mishaaac/kelyro/internal/research"
)

// Relation records the bounded semantic signal supplied by the caller. The
// resolver deliberately does not infer contradiction from arbitrary prose.
type Relation string

const (
	RelationContradiction              Relation = "contradiction"
	RelationRecommendationDisagreement Relation = "recommendation_disagreement"
)

func (relation Relation) Validate() error {
	switch relation {
	case RelationContradiction, RelationRecommendationDisagreement:
		return nil
	default:
		return fmt.Errorf("invalid conflict relation %q", relation)
	}
}

// Observation binds one conflicting Claim to the exact source and reviewed
// authority tier used for this assessment.
type Observation struct {
	Claim         research.Claim
	Source        research.Source
	AuthorityTier research.AuthorityTier
}

func (observation Observation) Validate() error {
	if err := observation.Claim.Validate(); err != nil {
		return fmt.Errorf("conflict claim: %w", err)
	}
	if err := observation.Source.Validate(); err != nil {
		return fmt.Errorf("conflict source: %w", err)
	}
	if err := observation.AuthorityTier.Validate(); err != nil {
		return err
	}
	for _, sourceID := range observation.Claim.SourceIDs {
		if sourceID == observation.Source.ID {
			return nil
		}
	}
	return fmt.Errorf("conflict source is not declared by claim")
}

// Input contains exactly two observations because v1 resolves pairwise
// conflicts. Larger conflict sets are represented as multiple durable pairs.
type Input struct {
	ID           research.ID
	Relation     Relation
	Observations []Observation
	DetectedAt   research.Timestamp
}

func (input Input) Validate() error {
	if err := input.ID.Validate(); err != nil {
		return fmt.Errorf("conflict input: %w", err)
	}
	if err := input.Relation.Validate(); err != nil {
		return err
	}
	if len(input.Observations) != 2 {
		return fmt.Errorf("conflict-resolver-v1 requires exactly 2 observations")
	}
	for index, observation := range input.Observations {
		if err := observation.Validate(); err != nil {
			return fmt.Errorf("conflict observation %d: %w", index, err)
		}
		if observation.Claim.CreatedAt.After(input.DetectedAt) || observation.Source.CreatedAt.After(input.DetectedAt) {
			return fmt.Errorf("conflict observation %d was recorded after detection", index)
		}
	}
	left, right := input.Observations[0], input.Observations[1]
	if left.Claim.ID == right.Claim.ID {
		return fmt.Errorf("conflict observations repeat a claim")
	}
	if left.Claim.Topic != right.Claim.Topic {
		return fmt.Errorf("conflicting claims must address the same topic")
	}
	if left.Claim.Statement == right.Claim.Statement {
		return fmt.Errorf("conflicting claims have identical statements")
	}
	return input.DetectedAt.Validate()
}

// Resolve applies conflict-resolver-v1. Its decisions use explicit scope,
// version, temporal, source-kind and authority data; claim prose is opaque.
func Resolve(input Input) (research.Conflict, error) {
	if err := input.Validate(); err != nil {
		return research.Conflict{}, err
	}
	left, right := input.Observations[0], input.Observations[1]
	conflictType := detectType(input.Relation, left, right)
	result := research.Conflict{
		ID: input.ID, Type: conflictType,
		ClaimIDs:   []research.ClaimID{left.Claim.ID, right.Claim.ID},
		DetectedAt: input.DetectedAt, AlgorithmVersion: research.ConflictResolverAlgorithmV1,
	}

	switch conflictType {
	case research.ConflictTemporalMismatch:
		resolveTemporal(&result, left, right)
	case research.ConflictVersionMismatch:
		result.Resolution = "Keep the claims separated by their recorded version applicability; neither replaces the other globally."
		result.Reason = "The claims have different version scopes, so their assertions are not interchangeable."
		result.Confidence = confidence(0.95)
	case research.ConflictScopeMismatch:
		result.Resolution = "Keep each claim within its declared applicability scope."
		result.Reason = "The claims differ by applicability or release-status scope."
		result.Confidence = confidence(0.9)
	case research.ConflictAuthorityMismatch:
		resolveAuthority(&result, left, right)
	case research.ConflictRecommendationDisagreement:
		if winner, ok, score, reason := authorityWinner(left, right); ok {
			setWinner(&result, winner, score,
				"Prefer the more authoritative recommendation within its declared scope.", reason)
		} else {
			setUnresolved(&result, "Comparable sources make different recommendations; v1 has no contextual rule that safely chooses one.")
		}
	default:
		setUnresolved(&result, "The claims directly contradict each other and no version, temporal, scope, or clear authority rule separates them.")
	}
	if err := result.Validate(); err != nil {
		return research.Conflict{}, err
	}
	return result, nil
}

func detectType(relation Relation, left, right Observation) research.ConflictType {
	if currentMismatch(left.Source.TemporalScope, right.Source.TemporalScope) {
		return research.ConflictTemporalMismatch
	}
	if versionsDiffer(left.Claim.VersionScope, right.Claim.VersionScope) {
		return research.ConflictVersionMismatch
	}
	if left.Claim.Scope != right.Claim.Scope || left.Claim.StatusScope != right.Claim.StatusScope {
		return research.ConflictScopeMismatch
	}
	if relation == RelationRecommendationDisagreement ||
		left.Claim.Type == research.ClaimRecommendation || right.Claim.Type == research.ClaimRecommendation {
		return research.ConflictRecommendationDisagreement
	}
	if _, ok, _, _ := authorityWinner(left, right); ok || left.AuthorityTier != right.AuthorityTier {
		return research.ConflictAuthorityMismatch
	}
	return research.ConflictDirectContradiction
}

func resolveTemporal(result *research.Conflict, left, right Observation) {
	winner := left
	if right.Source.TemporalScope == research.SourceTemporalCurrent {
		winner = right
	}
	setWinner(result, winner, 0.9,
		"Use the current source for current guidance and retain the other claim only as historical or version-bound context.",
		"Exactly one source is current; non-current material cannot silently override current guidance.")
	result.WinningScope = string(research.SourceTemporalCurrent)
}

func resolveAuthority(result *research.Conflict, left, right Observation) {
	winner, ok, score, reason := authorityWinner(left, right)
	if !ok {
		setUnresolved(result, "The authority difference is not strong enough for a contextual v1 rule to choose safely.")
		return
	}
	setWinner(result, winner, score,
		"Prefer the winning source for this claim scope while preserving the losing claim as conflicting evidence.", reason)
}

func setWinner(result *research.Conflict, winner Observation, score float64, resolution, reason string) {
	claimID, sourceID := winner.Claim.ID, winner.Source.ID
	result.WinningClaimID = &claimID
	result.WinningSourceID = &sourceID
	result.WinningScope = winner.Claim.Scope
	result.Resolution = resolution
	result.Reason = reason
	result.Confidence = confidence(score)
}

func setUnresolved(result *research.Conflict, reason string) {
	result.Unresolved = true
	result.Reason = reason
	result.Confidence = confidence(0.5)
}

func authorityWinner(left, right Observation) (Observation, bool, float64, string) {
	leftNormative := normativeSource(left)
	rightNormative := normativeSource(right)
	leftRank, rightRank := tierRank(left.AuthorityTier), tierRank(right.AuthorityTier)
	if leftNormative != rightNormative {
		if leftNormative && leftRank <= rightRank {
			return left, true, 0.95, "A specification or standard is normative for this claim type; the competing source is not."
		}
		if rightNormative && rightRank <= leftRank {
			return right, true, 0.95, "A specification or standard is normative for this claim type; the competing source is not."
		}
	}
	gap := leftRank - rightRank
	if gap >= 2 {
		return right, true, 0.85, "The winning source is at least two reviewed authority tiers stronger for this topic."
	}
	if gap <= -2 {
		return left, true, 0.85, "The winning source is at least two reviewed authority tiers stronger for this topic."
	}
	return Observation{}, false, 0, ""
}

func normativeSource(observation Observation) bool {
	switch observation.Claim.Type {
	case research.ClaimDefinition, research.ClaimRequirement, research.ClaimBehavior,
		research.ClaimCompatibility, research.ClaimSecurity:
		switch observation.Source.Kind {
		case research.SourceSpecification, research.SourceStandard:
			return true
		}
	}
	return false
}

func currentMismatch(left, right research.SourceTemporalScope) bool {
	return (left == research.SourceTemporalCurrent) != (right == research.SourceTemporalCurrent)
}

func versionsDiffer(left, right *research.SourceVersion) bool {
	if left == nil && right == nil {
		return false
	}
	if left == nil || right == nil {
		return true
	}
	return *left != *right
}

func tierRank(tier research.AuthorityTier) int {
	switch tier {
	case research.AuthorityTierA:
		return 0
	case research.AuthorityTierB:
		return 1
	case research.AuthorityTierC:
		return 2
	case research.AuthorityTierD:
		return 3
	default:
		return 4
	}
}

func confidence(value float64) research.ClaimConfidence {
	result, err := research.NewClaimConfidence(value)
	if err != nil {
		panic(err)
	}
	return result
}
