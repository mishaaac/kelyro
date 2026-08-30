package drift

import (
	"fmt"
	"sort"
	"strings"
	"unicode"

	"github.com/mishaaac/kelyro/internal/research"
)

type SnapshotObservation struct {
	SourceID       research.SourceID
	OldSnapshot    research.SourceSnapshot
	NewSnapshot    *research.SourceSnapshot
	AffectedClaims []research.ClaimID
}

type ReleaseObservation struct {
	Release        research.ReleaseRecord
	AffectedClaims []research.ClaimID
}

type Input struct {
	OldBundle            research.SourceBundle
	NewBundle            *research.SourceBundle
	OldClaims            []research.Claim
	NewClaims            []research.Claim
	SnapshotObservations []SnapshotObservation
	ReleaseObservations  []ReleaseObservation
	DetectedAt           research.Timestamp
}

type Finding struct {
	Type           research.DriftType
	Severity       research.Severity
	AffectedClaims []research.ClaimID
	OldEvidence    []research.ID
	NewEvidence    []research.ID
	Confidence     research.ClaimConfidence
}

type Result struct {
	Findings         []Finding
	UnresolvedClaims []research.ClaimID
}

func DetectV1(input Input) (Result, error) {
	if err := validateInput(input); err != nil {
		return Result{}, err
	}
	oldClaims := claimMap(input.OldClaims)
	newClaims := claimMap(input.NewClaims)
	unresolved := make(map[research.ClaimID]struct{})
	findings := make(map[research.DriftType]Finding)

	for _, claimID := range input.OldBundle.ClaimIDs {
		oldClaim := oldClaims[claimID]
		newClaim, exists := newClaims[claimID]
		if !exists {
			unresolved[claimID] = struct{}{}
			continue
		}
		oldEvidence, newEvidence := oldClaim.EvidenceIDs, newClaim.EvidenceIDs
		switch {
		case oldClaim.Type != research.ClaimDeprecation && newClaim.Type == research.ClaimDeprecation:
			mergeFinding(findings, newFinding(research.DriftDeprecationIntroduced, research.SeverityCritical, .95, claimID, oldEvidence, newEvidence))
		case oldClaim.Type != newClaim.Type:
			mergeFinding(findings, newFinding(research.DriftClaimInvalidated, research.SeverityImportant, .85, claimID, oldEvidence, newEvidence))
		}
		if !sameVersion(oldClaim.VersionScope, newClaim.VersionScope) {
			mergeFinding(findings, newFinding(research.DriftVersionSuperseded, research.SeverityImportant, .95, claimID, oldEvidence, newEvidence))
		}
		if semanticTextV1(oldClaim.Scope) != semanticTextV1(newClaim.Scope) || oldClaim.StatusScope != newClaim.StatusScope {
			mergeFinding(findings, newFinding(research.DriftScopeChanged, research.SeverityImportant, .9, claimID, oldEvidence, newEvidence))
		}
		if semanticTextV1(oldClaim.Statement) != semanticTextV1(newClaim.Statement) {
			driftType := research.DriftClaimInvalidated
			confidence := .8
			if oldClaim.Type == research.ClaimRecommendation || newClaim.Type == research.ClaimRecommendation {
				driftType = research.DriftRecommendationChanged
				confidence = .85
			}
			mergeFinding(findings, newFinding(driftType, research.SeverityImportant, confidence, claimID, oldEvidence, newEvidence))
		}
	}

	for _, observation := range input.SnapshotObservations {
		if observation.NewSnapshot == nil {
			for _, claimID := range observation.AffectedClaims {
				unresolved[claimID] = struct{}{}
			}
			continue
		}
		newSnapshot := *observation.NewSnapshot
		gone := newSnapshot.Fetch.StatusCode == 404 || newSnapshot.Fetch.StatusCode == 410
		changed := observation.OldSnapshot.Fetch.ContentHash != "" && newSnapshot.Fetch.ContentHash != "" &&
			observation.OldSnapshot.Fetch.ContentHash != newSnapshot.Fetch.ContentHash
		if !gone && !changed {
			continue
		}
		severity, confidence := research.SeverityMinor, .5
		if gone {
			severity, confidence = research.SeverityImportant, .75
		}
		for _, claimID := range observation.AffectedClaims {
			oldClaim := oldClaims[claimID]
			newClaim, exists := newClaims[claimID]
			var newEvidence []research.ID
			if exists {
				newEvidence = newClaim.EvidenceIDs
			}
			mergeFinding(findings, newFinding(research.DriftSourceChanged, severity, confidence, claimID, oldClaim.EvidenceIDs, newEvidence))
		}
	}

	for _, observation := range input.ReleaseObservations {
		if observation.Release.Status != research.ReleaseCurrent {
			continue
		}
		for _, claimID := range observation.AffectedClaims {
			oldClaim := oldClaims[claimID]
			if oldClaim.VersionScope == nil || oldClaim.VersionScope.String() == observation.Release.Version.String() {
				continue
			}
			newClaim, exists := newClaims[claimID]
			var newEvidence []research.ID
			if exists {
				newEvidence = newClaim.EvidenceIDs
			}
			mergeFinding(findings, newFinding(research.DriftVersionSuperseded, research.SeverityImportant, .95, claimID, oldClaim.EvidenceIDs, newEvidence))
		}
	}

	result := Result{Findings: make([]Finding, 0, len(findings)), UnresolvedClaims: mapClaimIDs(unresolved)}
	for _, finding := range findings {
		finding.AffectedClaims = sortedClaimIDs(finding.AffectedClaims)
		finding.OldEvidence = sortedIDs(finding.OldEvidence)
		finding.NewEvidence = sortedIDs(finding.NewEvidence)
		result.Findings = append(result.Findings, finding)
	}
	sort.Slice(result.Findings, func(i, j int) bool {
		return driftTypeRank(result.Findings[i].Type) < driftTypeRank(result.Findings[j].Type)
	})
	return result, nil
}

func validateInput(input Input) error {
	if err := input.OldBundle.Validate(); err != nil {
		return fmt.Errorf("old source bundle: %w", err)
	}
	if input.NewBundle != nil {
		if err := input.NewBundle.Validate(); err != nil {
			return fmt.Errorf("new source bundle: %w", err)
		}
		if input.NewBundle.Topic != input.OldBundle.Topic || input.NewBundle.Purpose != input.OldBundle.Purpose {
			return fmt.Errorf("source bundles describe different research topics or purposes")
		}
	}
	if err := input.DetectedAt.Validate(); err != nil {
		return fmt.Errorf("drift detection time: %w", err)
	}
	if input.DetectedAt.Before(input.OldBundle.VerifiedAt) ||
		(input.NewBundle != nil && input.DetectedAt.Before(input.NewBundle.VerifiedAt)) {
		return fmt.Errorf("drift detection precedes a compared bundle")
	}
	oldClaims, err := validateClaims("old", input.OldClaims)
	if err != nil {
		return err
	}
	newClaims, err := validateClaims("new", input.NewClaims)
	if err != nil {
		return err
	}
	for _, claimID := range input.OldBundle.ClaimIDs {
		if _, exists := oldClaims[claimID]; !exists {
			return fmt.Errorf("old source bundle claim %q is missing", claimID)
		}
	}
	if input.NewBundle != nil {
		for _, claimID := range input.NewBundle.ClaimIDs {
			if _, exists := newClaims[claimID]; !exists {
				return fmt.Errorf("new source bundle claim %q is missing", claimID)
			}
		}
	}
	oldSources := make(map[research.SourceID]struct{}, len(input.OldBundle.Sources))
	for _, source := range input.OldBundle.Sources {
		oldSources[source.SourceID] = struct{}{}
	}
	for index, observation := range input.SnapshotObservations {
		if err := observation.SourceID.Validate(); err != nil {
			return fmt.Errorf("snapshot observation %d: %w", index, err)
		}
		if _, exists := oldSources[observation.SourceID]; !exists {
			return fmt.Errorf("snapshot observation %d source is absent from old bundle", index)
		}
		if err := observation.OldSnapshot.Validate(); err != nil || observation.OldSnapshot.SourceID != observation.SourceID {
			return fmt.Errorf("snapshot observation %d has invalid old snapshot", index)
		}
		if observation.NewSnapshot != nil {
			if err := observation.NewSnapshot.Validate(); err != nil || observation.NewSnapshot.SourceID != observation.SourceID {
				return fmt.Errorf("snapshot observation %d has invalid new snapshot", index)
			}
			if observation.NewSnapshot.FetchedAt.Before(observation.OldSnapshot.FetchedAt) {
				return fmt.Errorf("snapshot observation %d moves backwards in time", index)
			}
		}
		if err := validateAffectedClaims(observation.AffectedClaims, oldClaims); err != nil {
			return fmt.Errorf("snapshot observation %d: %w", index, err)
		}
	}
	for index, observation := range input.ReleaseObservations {
		if err := observation.Release.Validate(); err != nil {
			return fmt.Errorf("release observation %d: %w", index, err)
		}
		if err := validateAffectedClaims(observation.AffectedClaims, oldClaims); err != nil {
			return fmt.Errorf("release observation %d: %w", index, err)
		}
	}
	return nil
}

func validateClaims(label string, claims []research.Claim) (map[research.ClaimID]research.Claim, error) {
	result := make(map[research.ClaimID]research.Claim, len(claims))
	for index, claim := range claims {
		if err := claim.Validate(); err != nil {
			return nil, fmt.Errorf("%s claim %d: %w", label, index, err)
		}
		if _, exists := result[claim.ID]; exists {
			return nil, fmt.Errorf("%s claims repeat %q", label, claim.ID)
		}
		result[claim.ID] = claim
	}
	return result, nil
}

func validateAffectedClaims(claimIDs []research.ClaimID, claims map[research.ClaimID]research.Claim) error {
	if len(claimIDs) == 0 {
		return fmt.Errorf("affected claims are empty")
	}
	seen := make(map[research.ClaimID]struct{}, len(claimIDs))
	for _, claimID := range claimIDs {
		if err := claimID.Validate(); err != nil {
			return err
		}
		if _, exists := claims[claimID]; !exists {
			return fmt.Errorf("affected claim %q is absent from old claims", claimID)
		}
		if _, exists := seen[claimID]; exists {
			return fmt.Errorf("affected claim %q is duplicated", claimID)
		}
		seen[claimID] = struct{}{}
	}
	return nil
}

func newFinding(driftType research.DriftType, severity research.Severity, confidence float64, claimID research.ClaimID, oldEvidence, newEvidence []research.ID) Finding {
	score, err := research.NewClaimConfidence(confidence)
	if err != nil {
		panic(err)
	}
	return Finding{
		Type: driftType, Severity: severity, AffectedClaims: []research.ClaimID{claimID},
		OldEvidence: append([]research.ID(nil), oldEvidence...), NewEvidence: append([]research.ID(nil), newEvidence...),
		Confidence: score,
	}
}

func mergeFinding(findings map[research.DriftType]Finding, next Finding) {
	current, exists := findings[next.Type]
	if !exists {
		findings[next.Type] = next
		return
	}
	current.AffectedClaims = appendUniqueClaimIDs(current.AffectedClaims, next.AffectedClaims...)
	current.OldEvidence = appendUniqueIDs(current.OldEvidence, next.OldEvidence...)
	current.NewEvidence = appendUniqueIDs(current.NewEvidence, next.NewEvidence...)
	if severityRank(next.Severity) > severityRank(current.Severity) {
		current.Severity = next.Severity
	}
	if next.Confidence.Value() < current.Confidence.Value() {
		current.Confidence = next.Confidence
	}
	findings[next.Type] = current
}

func semanticTextV1(value string) string {
	var result strings.Builder
	space := false
	for _, character := range strings.ToLower(value) {
		if unicode.IsLetter(character) || unicode.IsDigit(character) {
			if space && result.Len() > 0 {
				result.WriteByte(' ')
			}
			space = false
			result.WriteRune(character)
		} else {
			space = true
		}
	}
	return result.String()
}

func sameVersion(left, right *research.SourceVersion) bool {
	if left == nil || right == nil {
		return left == nil && right == nil
	}
	return *left == *right
}

func claimMap(claims []research.Claim) map[research.ClaimID]research.Claim {
	result := make(map[research.ClaimID]research.Claim, len(claims))
	for _, claim := range claims {
		result[claim.ID] = claim
	}
	return result
}

func appendUniqueClaimIDs(values []research.ClaimID, candidates ...research.ClaimID) []research.ClaimID {
	seen := make(map[research.ClaimID]struct{}, len(values)+len(candidates))
	for _, value := range values {
		seen[value] = struct{}{}
	}
	for _, candidate := range candidates {
		if _, exists := seen[candidate]; !exists {
			values = append(values, candidate)
			seen[candidate] = struct{}{}
		}
	}
	return values
}

func appendUniqueIDs(values []research.ID, candidates ...research.ID) []research.ID {
	seen := make(map[research.ID]struct{}, len(values)+len(candidates))
	for _, value := range values {
		seen[value] = struct{}{}
	}
	for _, candidate := range candidates {
		if _, exists := seen[candidate]; !exists {
			values = append(values, candidate)
			seen[candidate] = struct{}{}
		}
	}
	return values
}

func sortedClaimIDs(values []research.ClaimID) []research.ClaimID {
	result := append([]research.ClaimID(nil), values...)
	sort.Slice(result, func(i, j int) bool { return result[i].String() < result[j].String() })
	return result
}

func sortedIDs(values []research.ID) []research.ID {
	result := append([]research.ID(nil), values...)
	sort.Slice(result, func(i, j int) bool { return result[i].String() < result[j].String() })
	return result
}

func mapClaimIDs(values map[research.ClaimID]struct{}) []research.ClaimID {
	result := make([]research.ClaimID, 0, len(values))
	for claimID := range values {
		result = append(result, claimID)
	}
	return sortedClaimIDs(result)
}

func severityRank(severity research.Severity) int {
	switch severity {
	case research.SeverityCritical:
		return 3
	case research.SeverityImportant:
		return 2
	case research.SeverityMinor:
		return 1
	default:
		return 0
	}
}

func driftTypeRank(driftType research.DriftType) int {
	switch driftType {
	case research.DriftSourceChanged:
		return 0
	case research.DriftClaimInvalidated:
		return 1
	case research.DriftVersionSuperseded:
		return 2
	case research.DriftRecommendationChanged:
		return 3
	case research.DriftDeprecationIntroduced:
		return 4
	case research.DriftScopeChanged:
		return 5
	default:
		return 6
	}
}
