package research

import "fmt"

// Evidence is a bounded observation from one immutable SourceSnapshot.
type Evidence struct {
	ID               ID
	SourceID         SourceID
	SnapshotID       ID
	Location         string
	Excerpt          string
	ExcerptHash      string
	ExtractedAt      Timestamp
	ExtractorVersion string
}

func (evidence Evidence) Validate() error {
	if err := evidence.ID.Validate(); err != nil {
		return fmt.Errorf("evidence: %w", err)
	}
	if err := evidence.SourceID.Validate(); err != nil {
		return err
	}
	if err := evidence.SnapshotID.Validate(); err != nil {
		return fmt.Errorf("evidence snapshot: %w", err)
	}
	if err := requireText("evidence location", evidence.Location); err != nil {
		return err
	}
	if err := requireText("evidence excerpt", evidence.Excerpt); err != nil {
		return err
	}
	if err := requireText("evidence excerpt hash", evidence.ExcerptHash); err != nil {
		return err
	}
	if err := validateTimestamp("evidence extracted at", evidence.ExtractedAt); err != nil {
		return err
	}
	return requireText("evidence extractor version", evidence.ExtractorVersion)
}

type ClaimType string

const (
	ClaimDefinition     ClaimType = "definition"
	ClaimRequirement    ClaimType = "requirement"
	ClaimBehavior       ClaimType = "behavior"
	ClaimVersionChange  ClaimType = "version_change"
	ClaimDeprecation    ClaimType = "deprecation"
	ClaimRecommendation ClaimType = "recommendation"
	ClaimWarning        ClaimType = "warning"
	ClaimExample        ClaimType = "example"
	ClaimCompatibility  ClaimType = "compatibility"
	ClaimSecurity       ClaimType = "security"
	ClaimHistorical     ClaimType = "historical"
)

func (claimType ClaimType) Validate() error {
	switch claimType {
	case ClaimDefinition, ClaimRequirement, ClaimBehavior, ClaimVersionChange,
		ClaimDeprecation, ClaimRecommendation, ClaimWarning, ClaimExample,
		ClaimCompatibility, ClaimSecurity, ClaimHistorical:
		return nil
	default:
		return fmt.Errorf("invalid claim type %q", claimType)
	}
}

// Claim is an assertion explicitly backed by both sources and evidence.
type Claim struct {
	ID           ClaimID
	Topic        ResearchTopic
	Statement    string
	Type         ClaimType
	VersionScope *SourceVersion
	Confidence   ClaimConfidence
	SourceIDs    []SourceID
	EvidenceIDs  []ID
	CreatedAt    Timestamp
}

func (claim Claim) Validate() error {
	if err := claim.ID.Validate(); err != nil {
		return err
	}
	if err := claim.Topic.Validate(); err != nil {
		return err
	}
	if err := requireText("claim statement", claim.Statement); err != nil {
		return err
	}
	if err := claim.Type.Validate(); err != nil {
		return err
	}
	if claim.VersionScope != nil {
		if err := claim.VersionScope.Validate(); err != nil {
			return err
		}
	}
	if err := claim.Confidence.Validate(); err != nil {
		return fmt.Errorf("claim confidence: %w", err)
	}
	if err := validateSourceIDs("claim sources", claim.SourceIDs, 1); err != nil {
		return err
	}
	if err := validateIDs("claim evidence", claim.EvidenceIDs, 1); err != nil {
		return err
	}
	return validateTimestamp("claim created at", claim.CreatedAt)
}

// Provenance captures the required claim-to-source chain. DiscoveryID is
// optional because manually registered sources need not originate in search.
type Provenance struct {
	RequestID   ID
	RunID       ID
	DiscoveryID *ID
	SourceID    SourceID
	SnapshotID  ID
	EvidenceID  ID
	ClaimID     ClaimID
	RecordedAt  Timestamp
	ToolVersion string
}

func (provenance Provenance) Validate() error {
	if err := provenance.RequestID.Validate(); err != nil {
		return fmt.Errorf("provenance request: %w", err)
	}
	if err := provenance.RunID.Validate(); err != nil {
		return fmt.Errorf("provenance run: %w", err)
	}
	if provenance.DiscoveryID != nil {
		if err := provenance.DiscoveryID.Validate(); err != nil {
			return fmt.Errorf("provenance discovery: %w", err)
		}
	}
	if err := provenance.SourceID.Validate(); err != nil {
		return err
	}
	if err := provenance.SnapshotID.Validate(); err != nil {
		return fmt.Errorf("provenance snapshot: %w", err)
	}
	if err := provenance.EvidenceID.Validate(); err != nil {
		return fmt.Errorf("provenance evidence: %w", err)
	}
	if err := provenance.ClaimID.Validate(); err != nil {
		return err
	}
	if err := validateTimestamp("provenance recorded at", provenance.RecordedAt); err != nil {
		return err
	}
	return requireText("provenance tool version", provenance.ToolVersion)
}

// ValidateProvenanceRelationships verifies that a provenance record describes
// the supplied aggregate chain rather than merely containing well-formed IDs.
func ValidateProvenanceRelationships(
	provenance Provenance,
	request ResearchRequest,
	run ResearchRun,
	source Source,
	snapshot SourceSnapshot,
	evidence Evidence,
	claim Claim,
) error {
	for _, item := range []struct {
		name     string
		validate func() error
	}{
		{"provenance", provenance.Validate},
		{"request", request.Validate},
		{"run", run.Validate},
		{"source", source.Validate},
		{"snapshot", snapshot.Validate},
		{"evidence", evidence.Validate},
		{"claim", claim.Validate},
	} {
		if err := item.validate(); err != nil {
			return fmt.Errorf("%s: %w", item.name, err)
		}
	}
	if provenance.RequestID != request.ID || run.RequestID != request.ID {
		return fmt.Errorf("provenance request relationship does not match")
	}
	if provenance.RunID != run.ID {
		return fmt.Errorf("provenance run relationship does not match")
	}
	if provenance.SourceID != source.ID || snapshot.SourceID != source.ID || evidence.SourceID != source.ID {
		return fmt.Errorf("provenance source relationship does not match")
	}
	if provenance.SnapshotID != snapshot.ID || evidence.SnapshotID != snapshot.ID {
		return fmt.Errorf("provenance snapshot relationship does not match")
	}
	if provenance.EvidenceID != evidence.ID || !containsID(claim.EvidenceIDs, evidence.ID) {
		return fmt.Errorf("provenance evidence relationship does not match")
	}
	if provenance.ClaimID != claim.ID || !containsSourceID(claim.SourceIDs, source.ID) {
		return fmt.Errorf("provenance claim relationship does not match")
	}
	return nil
}

type Citation struct {
	ID           ID
	SourceID     SourceID
	SnapshotID   ID
	EvidenceID   ID
	Title        string
	Locator      SourceLocator
	DeepLink     *DeepLink
	SnapshotDate Timestamp
	LastVerified Timestamp
}

func (citation Citation) Validate() error {
	if err := citation.ID.Validate(); err != nil {
		return fmt.Errorf("citation: %w", err)
	}
	if err := citation.SourceID.Validate(); err != nil {
		return err
	}
	if err := citation.SnapshotID.Validate(); err != nil {
		return fmt.Errorf("citation snapshot: %w", err)
	}
	if err := citation.EvidenceID.Validate(); err != nil {
		return fmt.Errorf("citation evidence: %w", err)
	}
	if err := requireText("citation title", citation.Title); err != nil {
		return err
	}
	if err := citation.Locator.Validate(); err != nil {
		return err
	}
	if citation.DeepLink != nil {
		if err := citation.DeepLink.Validate(); err != nil {
			return err
		}
	}
	if err := validateTimestamp("citation snapshot date", citation.SnapshotDate); err != nil {
		return err
	}
	if err := validateTimestamp("citation last verified", citation.LastVerified); err != nil {
		return err
	}
	if citation.LastVerified.Before(citation.SnapshotDate) {
		return fmt.Errorf("citation verification precedes snapshot")
	}
	return nil
}

// ValidateCitationRelationships verifies a citation against the source,
// immutable snapshot, and bounded evidence that it names.
func ValidateCitationRelationships(
	citation Citation,
	source Source,
	snapshot SourceSnapshot,
	evidence Evidence,
) error {
	for _, item := range []struct {
		name     string
		validate func() error
	}{
		{"citation", citation.Validate},
		{"source", source.Validate},
		{"snapshot", snapshot.Validate},
		{"evidence", evidence.Validate},
	} {
		if err := item.validate(); err != nil {
			return fmt.Errorf("%s: %w", item.name, err)
		}
	}
	if citation.SourceID != source.ID || snapshot.SourceID != source.ID || evidence.SourceID != source.ID {
		return fmt.Errorf("citation source relationship does not match")
	}
	if citation.SnapshotID != snapshot.ID || evidence.SnapshotID != snapshot.ID {
		return fmt.Errorf("citation snapshot relationship does not match")
	}
	if citation.EvidenceID != evidence.ID {
		return fmt.Errorf("citation evidence relationship does not match")
	}
	if citation.Locator != snapshot.Locator {
		return fmt.Errorf("citation locator does not match snapshot")
	}
	return nil
}

type SourceBundleState string

const (
	BundleReady            SourceBundleState = "ready"
	BundleReadyWithCaveats SourceBundleState = "ready_with_caveats"
	BundleIncomplete       SourceBundleState = "incomplete"
	BundleConflicted       SourceBundleState = "conflicted"
)

func (state SourceBundleState) Validate() error {
	switch state {
	case BundleReady, BundleReadyWithCaveats, BundleIncomplete, BundleConflicted:
		return nil
	default:
		return fmt.Errorf("invalid source bundle state %q", state)
	}
}

// SourceBundle groups traceable identities only. Serialization, hashing, and
// compiler eligibility are reserved for later steps.
type SourceBundle struct {
	ID            ID
	RunID         ID
	Topic         ResearchTopic
	Purpose       ResearchPurpose
	TargetVersion *SourceVersion
	ClaimIDs      []ClaimID
	SourceIDs     []SourceID
	State         SourceBundleState
	VerifiedAt    Timestamp
}

func (bundle SourceBundle) Validate() error {
	if err := bundle.ID.Validate(); err != nil {
		return fmt.Errorf("source bundle: %w", err)
	}
	if err := bundle.RunID.Validate(); err != nil {
		return fmt.Errorf("source bundle run: %w", err)
	}
	if err := bundle.Topic.Validate(); err != nil {
		return err
	}
	if err := bundle.Purpose.Validate(); err != nil {
		return err
	}
	if bundle.TargetVersion != nil {
		if err := bundle.TargetVersion.Validate(); err != nil {
			return err
		}
	}
	if err := validateClaimIDs("source bundle claims", bundle.ClaimIDs, 1); err != nil {
		return err
	}
	if err := validateSourceIDs("source bundle sources", bundle.SourceIDs, 1); err != nil {
		return err
	}
	if err := bundle.State.Validate(); err != nil {
		return err
	}
	return validateTimestamp("source bundle verified at", bundle.VerifiedAt)
}

func containsID(ids []ID, target ID) bool {
	for _, id := range ids {
		if id == target {
			return true
		}
	}
	return false
}

func containsSourceID(ids []SourceID, target SourceID) bool {
	for _, id := range ids {
		if id == target {
			return true
		}
	}
	return false
}
