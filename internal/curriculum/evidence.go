package curriculum

import "fmt"

const EvidenceIngestionAlgorithmV1 = "source-bundle-ingestion-v1"

type EvidenceEligibility string

const (
	EvidenceReadyForCompile  EvidenceEligibility = "ready_for_compile"
	EvidenceReadyWithCaveats EvidenceEligibility = "ready_with_caveats"
	EvidenceNotReady         EvidenceEligibility = "not_ready"
)

func (eligibility EvidenceEligibility) Validate() error {
	switch eligibility {
	case EvidenceReadyForCompile, EvidenceReadyWithCaveats, EvidenceNotReady:
		return nil
	default:
		return fmt.Errorf("invalid curriculum evidence eligibility %q", eligibility)
	}
}

type EvidenceFreshness struct {
	State          string
	Score          float64
	LastVerifiedAt *Timestamp
	Algorithm      string
}

func (freshness EvidenceFreshness) Validate() error {
	switch freshness.State {
	case "fresh", "aging", "stale", "unknown":
	default:
		return fmt.Errorf("invalid evidence freshness state %q", freshness.State)
	}
	if freshness.Score < 0 || freshness.Score > 1 {
		return fmt.Errorf("evidence freshness score is outside 0..1")
	}
	if freshness.LastVerifiedAt != nil {
		if err := freshness.LastVerifiedAt.Validate(); err != nil {
			return fmt.Errorf("evidence freshness verification: %w", err)
		}
	}
	return requireText("evidence freshness algorithm", freshness.Algorithm)
}

// EvidenceSourceAuthority is the authority/temporal role frozen by I-03 in the
// exact Source Bundle. It intentionally does not re-evaluate current trust.
type EvidenceSourceAuthority struct {
	SourceID      ID
	Role          string
	TemporalScope string
	VersionScope  string
}

func (authority EvidenceSourceAuthority) Validate() error {
	if err := authority.SourceID.Validate(); err != nil {
		return fmt.Errorf("evidence authority source: %w", err)
	}
	switch authority.Role {
	case "primary", "supporting", "historical":
	default:
		return fmt.Errorf("invalid evidence authority role %q", authority.Role)
	}
	switch authority.TemporalScope {
	case "current", "historical", "version_bound", "archived":
	default:
		return fmt.Errorf("invalid evidence temporal scope %q", authority.TemporalScope)
	}
	if authority.TemporalScope == "version_bound" && authority.VersionScope == "" {
		return fmt.Errorf("version-bound evidence authority requires version scope")
	}
	return nil
}

type CurriculumEvidenceClaim struct {
	ID           ID
	Statement    string
	Scope        string
	VersionScope string
	Status       ConceptStatus
	Confidence   float64
	SourceIDs    []ID
}

func (claim CurriculumEvidenceClaim) Validate() error {
	if err := claim.ID.Validate(); err != nil {
		return fmt.Errorf("curriculum evidence claim: %w", err)
	}
	if err := requireText("curriculum evidence statement", claim.Statement); err != nil {
		return err
	}
	if err := requireText("curriculum evidence scope", claim.Scope); err != nil {
		return err
	}
	if err := claim.Status.Validate(); err != nil {
		return err
	}
	if claim.Confidence < 0 || claim.Confidence > 1 {
		return fmt.Errorf("curriculum evidence confidence is outside 0..1")
	}
	return validateIDs("curriculum evidence claim sources", claim.SourceIDs)
}

type CurriculumEvidenceConflict struct {
	ID               ID
	ClaimIDs         []ID
	Unresolved       bool
	Reason           string
	AlgorithmVersion string
}

func (conflict CurriculumEvidenceConflict) Validate() error {
	if err := conflict.ID.Validate(); err != nil {
		return fmt.Errorf("curriculum evidence conflict: %w", err)
	}
	if err := validateIDs("curriculum evidence conflict claims", conflict.ClaimIDs); err != nil {
		return err
	}
	if len(conflict.ClaimIDs) < 2 {
		return fmt.Errorf("curriculum evidence conflict requires at least two claims")
	}
	if err := requireText("curriculum evidence conflict reason", conflict.Reason); err != nil {
		return err
	}
	return requireText("curriculum evidence conflict algorithm", conflict.AlgorithmVersion)
}

// CurriculumEvidenceSet is immutable compiler input derived only from one
// exact, validated I-03 Source Bundle and its durable records.
type CurriculumEvidenceSet struct {
	Bundle           SourceBundleRef
	Eligibility      EvidenceEligibility
	Claims           []CurriculumEvidenceClaim
	VersionScopes    []string
	SourceAuthority  []EvidenceSourceAuthority
	Freshness        EvidenceFreshness
	Conflicts        []CurriculumEvidenceConflict
	Caveats          []string
	AlgorithmVersion string
}

func (set CurriculumEvidenceSet) Validate() error {
	if err := set.Bundle.Validate(); err != nil {
		return err
	}
	if err := set.Eligibility.Validate(); err != nil {
		return err
	}
	if len(set.Claims) == 0 {
		return fmt.Errorf("curriculum evidence set has no claims")
	}
	seenClaims := make(map[ID]struct{}, len(set.Claims))
	for _, claim := range set.Claims {
		if err := claim.Validate(); err != nil {
			return err
		}
		if _, exists := seenClaims[claim.ID]; exists {
			return fmt.Errorf("curriculum evidence set contains duplicate claim %q", claim.ID)
		}
		seenClaims[claim.ID] = struct{}{}
	}
	if err := validateTexts("curriculum evidence version scopes", set.VersionScopes); err != nil {
		return err
	}
	seenVersions := make(map[string]struct{}, len(set.VersionScopes))
	for _, version := range set.VersionScopes {
		if _, exists := seenVersions[version]; exists {
			return fmt.Errorf("curriculum evidence set contains duplicate version scope %q", version)
		}
		seenVersions[version] = struct{}{}
	}
	if err := set.Freshness.Validate(); err != nil {
		return err
	}
	seenSources := make(map[ID]struct{}, len(set.SourceAuthority))
	for _, authority := range set.SourceAuthority {
		if err := authority.Validate(); err != nil {
			return err
		}
		if _, exists := seenSources[authority.SourceID]; exists {
			return fmt.Errorf("curriculum evidence set contains duplicate source %q", authority.SourceID)
		}
		seenSources[authority.SourceID] = struct{}{}
	}
	for _, claim := range set.Claims {
		for _, sourceID := range claim.SourceIDs {
			if _, exists := seenSources[sourceID]; !exists {
				return fmt.Errorf("curriculum evidence claim %q references source outside bundle", claim.ID)
			}
		}
	}
	seenConflicts := make(map[ID]struct{}, len(set.Conflicts))
	for _, conflict := range set.Conflicts {
		if err := conflict.Validate(); err != nil {
			return err
		}
		if _, exists := seenConflicts[conflict.ID]; exists {
			return fmt.Errorf("curriculum evidence set contains duplicate conflict %q", conflict.ID)
		}
		seenConflicts[conflict.ID] = struct{}{}
		for _, claimID := range conflict.ClaimIDs {
			if _, exists := seenClaims[claimID]; !exists {
				return fmt.Errorf("curriculum evidence conflict references claim outside bundle")
			}
		}
	}
	if err := validateTexts("curriculum evidence caveats", set.Caveats); err != nil {
		return err
	}
	if set.Eligibility == EvidenceReadyForCompile && len(set.Caveats) != 0 {
		return fmt.Errorf("ready curriculum evidence cannot contain caveats")
	}
	if set.Eligibility == EvidenceReadyWithCaveats && len(set.Caveats) == 0 {
		return fmt.Errorf("caveated curriculum evidence requires caveats")
	}
	if set.AlgorithmVersion != EvidenceIngestionAlgorithmV1 {
		return fmt.Errorf("unsupported curriculum evidence ingestion algorithm %q", set.AlgorithmVersion)
	}
	return nil
}
