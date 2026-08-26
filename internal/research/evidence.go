package research

import (
	"fmt"
	"unicode/utf8"
)

const (
	MaximumEvidenceExcerptBytes = 8 << 10
	MaximumEvidenceContextBytes = 2 << 10
	MaximumClaimScopeBytes      = 1 << 10
	MaximumCitationSectionBytes = 2 << 10
	MaximumCitationLabelBytes   = 2 << 10
	CitationAlgorithmV1         = "citation-v1"
)

// CanonicalEvidenceExcerptHashV1 hashes the exact validated UTF-8 excerpt.
// Context is deliberately excluded because ExcerptHash identifies only the
// minimal quoted evidence carried by Evidence.Excerpt.
func CanonicalEvidenceExcerptHashV1(excerpt string) string {
	return CanonicalContentHashV1([]byte(excerpt))
}

// Evidence is a bounded observation from one immutable SourceSnapshot.
type Evidence struct {
	ID               ID
	SourceID         SourceID
	SnapshotID       ID
	Location         string
	Excerpt          string
	ExcerptHash      string
	ContextBefore    string
	ContextAfter     string
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
	if err := validateBoundedEvidenceText("evidence excerpt", evidence.Excerpt, MaximumEvidenceExcerptBytes, true); err != nil {
		return err
	}
	if err := validateBoundedEvidenceText("evidence context before", evidence.ContextBefore, MaximumEvidenceContextBytes, false); err != nil {
		return err
	}
	if err := validateBoundedEvidenceText("evidence context after", evidence.ContextAfter, MaximumEvidenceContextBytes, false); err != nil {
		return err
	}
	if err := ValidateCanonicalContentHashV1(evidence.ExcerptHash); err != nil {
		return fmt.Errorf("evidence excerpt hash: %w", err)
	}
	if evidence.ExcerptHash != CanonicalEvidenceExcerptHashV1(evidence.Excerpt) {
		return fmt.Errorf("evidence excerpt hash does not match excerpt")
	}
	if err := validateTimestamp("evidence extracted at", evidence.ExtractedAt); err != nil {
		return err
	}
	return requireText("evidence extractor version", evidence.ExtractorVersion)
}

func validateBoundedEvidenceText(name, value string, maximumBytes int, required bool) error {
	if required {
		if err := requireText(name, value); err != nil {
			return err
		}
	} else if err := validateOptionalText(name, value); err != nil {
		return err
	}
	if !utf8.ValidString(value) {
		return fmt.Errorf("%s is not valid UTF-8", name)
	}
	if len(value) > maximumBytes {
		return fmt.Errorf("%s exceeds %d bytes", name, maximumBytes)
	}
	return nil
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

// ClaimStatusScope records which maturity/status family a claim applies to.
// It is separate from release detection: this value scopes the assertion but
// does not establish the actual status of a release.
type ClaimStatusScope string

const (
	ClaimStatusAll          ClaimStatusScope = "all"
	ClaimStatusStable       ClaimStatusScope = "stable"
	ClaimStatusPreview      ClaimStatusScope = "preview"
	ClaimStatusExperimental ClaimStatusScope = "experimental"
	ClaimStatusLegacy       ClaimStatusScope = "legacy"
)

func (scope ClaimStatusScope) Validate() error {
	switch scope {
	case ClaimStatusAll, ClaimStatusStable, ClaimStatusPreview,
		ClaimStatusExperimental, ClaimStatusLegacy:
		return nil
	default:
		return fmt.Errorf("invalid claim status scope %q", scope)
	}
}

// Claim is an assertion explicitly backed by both sources and evidence.
type Claim struct {
	ID           ClaimID
	Topic        ResearchTopic
	Statement    string
	Type         ClaimType
	Scope        string
	VersionScope *SourceVersion
	StatusScope  ClaimStatusScope
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
	if err := requireText("claim scope", claim.Scope); err != nil {
		return err
	}
	if !utf8.ValidString(claim.Scope) {
		return fmt.Errorf("claim scope is not valid UTF-8")
	}
	if len(claim.Scope) > MaximumClaimScopeBytes {
		return fmt.Errorf("claim scope exceeds %d bytes", MaximumClaimScopeBytes)
	}
	if claim.VersionScope != nil {
		if err := claim.VersionScope.Validate(); err != nil {
			return err
		}
	}
	if err := claim.StatusScope.Validate(); err != nil {
		return err
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

// CitationLinkStrategy records how the most specific stable locator was
// selected. The canonical fallback intentionally has no DeepLink.
type CitationLinkStrategy string

const (
	CitationURLAnchor         CitationLinkStrategy = "url_anchor"
	CitationPackageSymbol     CitationLinkStrategy = "package_symbol"
	CitationSpecification     CitationLinkStrategy = "spec_section"
	CitationReleaseHeading    CitationLinkStrategy = "release_heading"
	CitationSourcePermalink   CitationLinkStrategy = "source_permalink"
	CitationCanonicalFallback CitationLinkStrategy = "canonical_fallback"
)

func (strategy CitationLinkStrategy) Validate() error {
	switch strategy {
	case CitationURLAnchor, CitationPackageSymbol, CitationSpecification,
		CitationReleaseHeading, CitationSourcePermalink, CitationCanonicalFallback:
		return nil
	default:
		return fmt.Errorf("invalid citation link strategy %q", strategy)
	}
}

type Citation struct {
	ID                       ID
	SourceID                 SourceID
	SnapshotID               ID
	EvidenceID               ID
	Title                    string
	Locator                  SourceLocator
	DeepLink                 *DeepLink
	LinkStrategy             CitationLinkStrategy
	Section                  string
	SnapshotDate             Timestamp
	VersionScope             *SourceVersion
	TemporalScope            SourceTemporalScope
	TemporalWarning          string
	LastVerified             Timestamp
	AlgorithmVersion         string
	TemporalAlgorithmVersion string
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
		if err := requireText("citation deep link label", citation.DeepLink.Label); err != nil {
			return err
		}
		if !utf8.ValidString(citation.DeepLink.Label) {
			return fmt.Errorf("citation deep link label is not valid UTF-8")
		}
		if len(citation.DeepLink.Label) > MaximumCitationLabelBytes {
			return fmt.Errorf("citation deep link label exceeds %d bytes", MaximumCitationLabelBytes)
		}
	}
	if err := citation.LinkStrategy.Validate(); err != nil {
		return err
	}
	if citation.LinkStrategy == CitationCanonicalFallback && citation.DeepLink != nil {
		return fmt.Errorf("canonical fallback must not contain a deep link")
	}
	if citation.LinkStrategy != CitationCanonicalFallback && citation.DeepLink == nil {
		return fmt.Errorf("citation strategy %q requires a deep link", citation.LinkStrategy)
	}
	if err := requireText("citation section", citation.Section); err != nil {
		return err
	}
	if !utf8.ValidString(citation.Section) {
		return fmt.Errorf("citation section is not valid UTF-8")
	}
	if len(citation.Section) > MaximumCitationSectionBytes {
		return fmt.Errorf("citation section exceeds %d bytes", MaximumCitationSectionBytes)
	}
	if err := validateTimestamp("citation snapshot date", citation.SnapshotDate); err != nil {
		return err
	}
	if citation.VersionScope != nil {
		if err := citation.VersionScope.Validate(); err != nil {
			return fmt.Errorf("citation version scope: %w", err)
		}
	}
	if err := citation.TemporalScope.Validate(); err != nil {
		return err
	}
	wantTemporalWarning, err := citation.TemporalScope.Warning(citation.VersionScope)
	if err != nil {
		return err
	}
	switch citation.TemporalAlgorithmVersion {
	case SourceTemporalPolicyV1:
		if citation.TemporalWarning != wantTemporalWarning {
			return fmt.Errorf("citation temporal warning does not match scope")
		}
	case SourceTemporalLegacyCurrent:
		if citation.TemporalScope != SourceTemporalCurrent || citation.TemporalWarning != "" {
			return fmt.Errorf("legacy citation temporal metadata must be current without warning")
		}
	default:
		return fmt.Errorf("invalid citation temporal algorithm version %q", citation.TemporalAlgorithmVersion)
	}
	if err := validateTimestamp("citation last verified", citation.LastVerified); err != nil {
		return err
	}
	if citation.LastVerified.Before(citation.SnapshotDate) {
		return fmt.Errorf("citation verification precedes snapshot")
	}
	if citation.AlgorithmVersion != CitationAlgorithmV1 {
		return fmt.Errorf("citation algorithm version must be %q", CitationAlgorithmV1)
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
	if citation.LastVerified.Before(evidence.ExtractedAt) {
		return fmt.Errorf("citation verification precedes evidence extraction")
	}
	if citation.Locator != source.Locator {
		return fmt.Errorf("citation locator does not match canonical source")
	}
	if !citation.SnapshotDate.Time().Equal(snapshot.FetchedAt.Time()) {
		return fmt.Errorf("citation snapshot date does not match snapshot")
	}
	if citation.Title != source.Metadata.Title {
		return fmt.Errorf("citation title does not match source")
	}
	if !sameOptionalSourceVersion(citation.VersionScope, source.Version) {
		return fmt.Errorf("citation version scope does not match source")
	}
	if citation.TemporalScope != source.TemporalScope {
		return fmt.Errorf("citation temporal scope does not match source")
	}
	if citation.TemporalAlgorithmVersion == SourceTemporalPolicyV1 {
		warning, err := source.TemporalScope.Warning(source.Version)
		if err != nil {
			return err
		}
		if citation.TemporalWarning != warning {
			return fmt.Errorf("citation temporal warning does not match source")
		}
	}
	return nil
}

func sameOptionalSourceVersion(left, right *SourceVersion) bool {
	if left == nil || right == nil {
		return left == nil && right == nil
	}
	return *left == *right
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

// SourceBundleSource preserves the temporal meaning of a source at bundle
// assembly time. Later reclassification of Source does not rewrite old bundles.
type SourceBundleSource struct {
	SourceID      SourceID
	TemporalScope SourceTemporalScope
	VersionScope  *SourceVersion
	Warning       string
}

func NewSourceBundleSource(source Source) (SourceBundleSource, error) {
	if err := source.Validate(); err != nil {
		return SourceBundleSource{}, err
	}
	warning, err := source.TemporalScope.Warning(source.Version)
	if err != nil {
		return SourceBundleSource{}, err
	}
	result := SourceBundleSource{
		SourceID: source.ID, TemporalScope: source.TemporalScope,
		VersionScope: cloneOptionalSourceVersion(source.Version), Warning: warning,
	}
	if err := result.Validate(); err != nil {
		return SourceBundleSource{}, err
	}
	return result, nil
}

func (source SourceBundleSource) Validate() error {
	if err := source.SourceID.Validate(); err != nil {
		return err
	}
	if err := source.TemporalScope.Validate(); err != nil {
		return err
	}
	if source.VersionScope != nil {
		if err := source.VersionScope.Validate(); err != nil {
			return err
		}
	}
	warning, err := source.TemporalScope.Warning(source.VersionScope)
	if err != nil {
		return err
	}
	if source.Warning != warning {
		return fmt.Errorf("source bundle temporal warning does not match scope")
	}
	return nil
}

// SourceBundle groups traceable identities and preserves source temporal
// scopes. Serialization, hashing, and compiler eligibility remain Step 25.
type SourceBundle struct {
	ID            ID
	RunID         ID
	Topic         ResearchTopic
	Purpose       ResearchPurpose
	TargetVersion *SourceVersion
	ClaimIDs      []ClaimID
	Sources       []SourceBundleSource
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
	if len(bundle.Sources) == 0 {
		return fmt.Errorf("source bundle sources requires at least 1 source")
	}
	seenSources := make(map[SourceID]struct{}, len(bundle.Sources))
	requiresCurrentCaveat := false
	for index, source := range bundle.Sources {
		if err := source.Validate(); err != nil {
			return fmt.Errorf("source bundle source %d: %w", index, err)
		}
		if _, exists := seenSources[source.SourceID]; exists {
			return fmt.Errorf("source bundle sources contains duplicate source id %q", source.SourceID)
		}
		seenSources[source.SourceID] = struct{}{}
		if source.TemporalScope == SourceTemporalVersionBound {
			if bundle.TargetVersion == nil || *source.VersionScope != *bundle.TargetVersion {
				return fmt.Errorf("version-bound source does not match source bundle target version")
			}
		}
		if bundle.Purpose == PurposeCurrentUsage && source.TemporalScope != SourceTemporalCurrent {
			requiresCurrentCaveat = true
		}
	}
	if err := bundle.State.Validate(); err != nil {
		return err
	}
	if requiresCurrentCaveat && bundle.State == BundleReady {
		return fmt.Errorf("current guidance bundle with non-current sources requires caveats")
	}
	return validateTimestamp("source bundle verified at", bundle.VerifiedAt)
}

func cloneOptionalSourceVersion(version *SourceVersion) *SourceVersion {
	if version == nil {
		return nil
	}
	clone := *version
	return &clone
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
