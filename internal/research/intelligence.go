package research

import "fmt"

const (
	FreshnessAlgorithmV1               = "freshness-v1"
	RefreshSchedulingAlgorithmV1       = "refresh-scheduling-v1"
	DeprecationIntelligenceAlgorithmV1 = "deprecation-intelligence-v1"
	DeprecationLegacyAlgorithm         = "deprecation-unversioned-legacy"
	MinimumFreshnessTTLDays            = 1
	MaximumFreshnessTTLDays            = 3650
	MaximumFreshnessTTLHints           = 64
)

type FreshnessState string

const (
	FreshnessFresh   FreshnessState = "fresh"
	FreshnessAging   FreshnessState = "aging"
	FreshnessStale   FreshnessState = "stale"
	FreshnessUnknown FreshnessState = "unknown"
)

func (state FreshnessState) Validate() error {
	switch state {
	case FreshnessFresh, FreshnessAging, FreshnessStale, FreshnessUnknown:
		return nil
	default:
		return fmt.Errorf("invalid freshness state %q", state)
	}
}

// VerificationReason explains why a source or claim is next scheduled for
// re-verification. TTLExpired also names a future TTL deadline before it is due.
type VerificationReason string

const (
	VerificationTTLExpired         VerificationReason = "ttl_expired"
	VerificationNewRelease         VerificationReason = "new_release_detected"
	VerificationSourceChanged      VerificationReason = "source_changed"
	VerificationConflictUnresolved VerificationReason = "conflict_unresolved"
	VerificationSecuritySensitive  VerificationReason = "security_sensitive"
	VerificationManualRequest      VerificationReason = "manual_request"
)

func (reason VerificationReason) Validate() error {
	switch reason {
	case VerificationTTLExpired, VerificationNewRelease, VerificationSourceChanged,
		VerificationConflictUnresolved, VerificationSecuritySensitive, VerificationManualRequest:
		return nil
	default:
		return fmt.Errorf("invalid verification reason %q", reason)
	}
}

type VerificationPriority string

const (
	VerificationPriorityNormal   VerificationPriority = "normal"
	VerificationPriorityHigh     VerificationPriority = "high"
	VerificationPriorityCritical VerificationPriority = "critical"
)

func (priority VerificationPriority) Validate() error {
	switch priority {
	case VerificationPriorityNormal, VerificationPriorityHigh, VerificationPriorityCritical:
		return nil
	default:
		return fmt.Errorf("invalid verification priority %q", priority)
	}
}

func (priority VerificationPriority) Rank() int {
	switch priority {
	case VerificationPriorityCritical:
		return 0
	case VerificationPriorityHigh:
		return 1
	case VerificationPriorityNormal:
		return 2
	default:
		return 3
	}
}

// FreshnessTTLHint optionally specializes a profile TTL by claim type, source
// kind, or their exact pair. Nil selectors form a profile-wide default.
type FreshnessTTLHint struct {
	ClaimType  *ClaimType
	SourceKind *SourceKind
	TTLDays    int
}

func (hint FreshnessTTLHint) Validate() error {
	if hint.ClaimType != nil {
		if err := hint.ClaimType.Validate(); err != nil {
			return fmt.Errorf("freshness TTL claim type: %w", err)
		}
	}
	if hint.SourceKind != nil {
		if err := hint.SourceKind.Validate(); err != nil {
			return fmt.Errorf("freshness TTL source kind: %w", err)
		}
	}
	if hint.TTLDays < MinimumFreshnessTTLDays || hint.TTLDays > MaximumFreshnessTTLDays {
		return fmt.Errorf("freshness TTL days must be between %d and %d", MinimumFreshnessTTLDays, MaximumFreshnessTTLDays)
	}
	return nil
}

type ReleaseChannel string

const (
	ReleaseStable         ReleaseChannel = "stable"
	ReleasePreview        ReleaseChannel = "preview"
	ReleaseBeta           ReleaseChannel = "beta"
	ReleaseRC             ReleaseChannel = "rc"
	ReleaseExperimental   ReleaseChannel = "experimental"
	ReleaseNightly        ReleaseChannel = "nightly"
	ReleaseChannelUnknown ReleaseChannel = "unknown"
)

func (channel ReleaseChannel) Validate() error {
	switch channel {
	case ReleaseStable, ReleasePreview, ReleaseBeta, ReleaseRC,
		ReleaseExperimental, ReleaseNightly, ReleaseChannelUnknown:
		return nil
	default:
		return fmt.Errorf("invalid release channel %q", channel)
	}
}

type ReleaseStatus string

const (
	ReleaseCurrent       ReleaseStatus = "current"
	ReleaseSuperseded    ReleaseStatus = "superseded"
	ReleaseLegacy        ReleaseStatus = "legacy"
	ReleaseEOL           ReleaseStatus = "eol"
	ReleaseStatusUnknown ReleaseStatus = "unknown"
)

func (status ReleaseStatus) Validate() error {
	switch status {
	case ReleaseCurrent, ReleaseSuperseded, ReleaseLegacy, ReleaseEOL, ReleaseStatusUnknown:
		return nil
	default:
		return fmt.Errorf("invalid release status %q", status)
	}
}

type TechnologyRelease struct {
	ID           ID
	TechnologyID ID
	Version      VersionIdentifier
	Channel      ReleaseChannel
	Status       ReleaseStatus
	SourceIDs    []SourceID
	ReleasedAt   *Timestamp
	VerifiedAt   Timestamp
}

func (record TechnologyRelease) Validate() error {
	if err := record.ID.Validate(); err != nil {
		return fmt.Errorf("release record: %w", err)
	}
	if err := record.TechnologyID.Validate(); err != nil {
		return fmt.Errorf("release technology: %w", err)
	}
	if err := record.Version.Validate(); err != nil {
		return err
	}
	if err := record.Channel.Validate(); err != nil {
		return err
	}
	if err := record.Status.Validate(); err != nil {
		return err
	}
	if err := validateSourceIDs("release sources", record.SourceIDs, 1); err != nil {
		return err
	}
	if err := validateOptionalTimestamp("release released at", record.ReleasedAt); err != nil {
		return err
	}
	if err := validateTimestamp("release verified at", record.VerifiedAt); err != nil {
		return err
	}
	if record.ReleasedAt != nil && record.ReleasedAt.After(record.VerifiedAt) {
		return fmt.Errorf("release date follows verification")
	}
	return nil
}

// ReleaseRecord preserves the Step 01/application port name while callers
// migrate to the explicit TechnologyRelease entity.
type ReleaseRecord = TechnologyRelease

type DeprecationStatus string

const (
	DeprecationDeprecated     DeprecationStatus = "deprecated"
	DeprecationRemoved        DeprecationStatus = "removed"
	DeprecationLegacy         DeprecationStatus = "legacy"
	DeprecationHistoricalOnly DeprecationStatus = "historical_only"
	DeprecationSuperseded     DeprecationStatus = "superseded"
)

func (status DeprecationStatus) Validate() error {
	switch status {
	case DeprecationDeprecated, DeprecationRemoved, DeprecationLegacy,
		DeprecationHistoricalOnly, DeprecationSuperseded:
		return nil
	default:
		return fmt.Errorf("invalid deprecation status %q", status)
	}
}

// DeprecationDetermination makes the evidentiary basis of a deprecation
// conclusion visible. LegacyUnclassified is reserved for records written
// before the Step 21 algorithm existed; new assessments must never use it.
type DeprecationDetermination string

const (
	DeprecationExplicitEvidence           DeprecationDetermination = "explicit_evidence"
	DeprecationMultiSourceStrongInference DeprecationDetermination = "multi_source_strong_inference"
	DeprecationLegacyUnclassified         DeprecationDetermination = "legacy_unclassified"
)

func (determination DeprecationDetermination) Validate() error {
	switch determination {
	case DeprecationExplicitEvidence, DeprecationMultiSourceStrongInference,
		DeprecationLegacyUnclassified:
		return nil
	default:
		return fmt.Errorf("invalid deprecation determination %q", determination)
	}
}

type DeprecationRecord struct {
	ID               ID
	Subject          string
	Status           DeprecationStatus
	Determination    DeprecationDetermination
	IntroducedIn     *SourceVersion
	DeprecatedIn     *SourceVersion
	RemovedIn        *SourceVersion
	Replacement      string
	SourceIDs        []SourceID
	EvidenceIDs      []ID
	VerifiedAt       Timestamp
	AlgorithmVersion string
}

func (record DeprecationRecord) Validate() error {
	if err := record.ID.Validate(); err != nil {
		return fmt.Errorf("deprecation record: %w", err)
	}
	if err := requireText("deprecation subject", record.Subject); err != nil {
		return err
	}
	if err := record.Status.Validate(); err != nil {
		return err
	}
	if err := record.Determination.Validate(); err != nil {
		return err
	}
	for _, item := range []struct {
		name    string
		version *SourceVersion
	}{
		{"introduced version", record.IntroducedIn},
		{"deprecated version", record.DeprecatedIn},
		{"removed version", record.RemovedIn},
	} {
		if item.version != nil {
			if err := item.version.Validate(); err != nil {
				return fmt.Errorf("%s: %w", item.name, err)
			}
		}
	}
	if err := validateOptionalText("deprecation replacement", record.Replacement); err != nil {
		return err
	}
	if err := validateSourceIDs("deprecation sources", record.SourceIDs, 1); err != nil {
		return err
	}
	if err := validateIDs("deprecation evidence", record.EvidenceIDs, 1); err != nil {
		return err
	}
	if err := validateTimestamp("deprecation verified at", record.VerifiedAt); err != nil {
		return err
	}
	switch record.AlgorithmVersion {
	case DeprecationIntelligenceAlgorithmV1:
		if record.Determination == DeprecationLegacyUnclassified {
			return fmt.Errorf("deprecation-intelligence-v1 cannot be legacy unclassified")
		}
		if record.Determination == DeprecationMultiSourceStrongInference && len(record.SourceIDs) < 2 {
			return fmt.Errorf("multi-source strong inference requires at least 2 sources")
		}
		if record.Determination == DeprecationMultiSourceStrongInference && len(record.EvidenceIDs) < 2 {
			return fmt.Errorf("multi-source strong inference requires at least 2 evidence records")
		}
	case DeprecationLegacyAlgorithm:
		if record.Determination != DeprecationLegacyUnclassified {
			return fmt.Errorf("legacy deprecation record must be legacy unclassified")
		}
	default:
		return fmt.Errorf("invalid deprecation algorithm version %q", record.AlgorithmVersion)
	}
	return nil
}

type ConflictType string

const (
	ConflictResolverAlgorithmV1 = "conflict-resolver-v1"
	ConflictLegacyAlgorithm     = "conflict-unversioned-legacy"
)

const (
	ConflictDirectContradiction        ConflictType = "direct_contradiction"
	ConflictVersionMismatch            ConflictType = "version_mismatch"
	ConflictTemporalMismatch           ConflictType = "temporal_mismatch"
	ConflictScopeMismatch              ConflictType = "scope_mismatch"
	ConflictRecommendationDisagreement ConflictType = "recommendation_disagreement"
	ConflictAuthorityMismatch          ConflictType = "authority_mismatch"
)

func (conflictType ConflictType) Validate() error {
	switch conflictType {
	case ConflictDirectContradiction, ConflictVersionMismatch,
		ConflictTemporalMismatch, ConflictScopeMismatch,
		ConflictRecommendationDisagreement, ConflictAuthorityMismatch:
		return nil
	default:
		return fmt.Errorf("invalid conflict type %q", conflictType)
	}
}

type Conflict struct {
	ID               ID
	Type             ConflictType
	ClaimIDs         []ClaimID
	Resolution       string
	Confidence       ClaimConfidence
	Reason           string
	WinningClaimID   *ClaimID
	WinningSourceID  *SourceID
	WinningScope     string
	Unresolved       bool
	DetectedAt       Timestamp
	AlgorithmVersion string
}

func (conflict Conflict) Validate() error {
	if err := conflict.ID.Validate(); err != nil {
		return fmt.Errorf("conflict: %w", err)
	}
	if err := conflict.Type.Validate(); err != nil {
		return err
	}
	if err := validateClaimIDs("conflict claims", conflict.ClaimIDs, 2); err != nil {
		return err
	}
	if err := conflict.Confidence.Validate(); err != nil {
		return fmt.Errorf("conflict confidence: %w", err)
	}
	if err := requireText("conflict reason", conflict.Reason); err != nil {
		return err
	}
	if err := validateOptionalText("conflict winning scope", conflict.WinningScope); err != nil {
		return err
	}
	if (conflict.WinningClaimID == nil) != (conflict.WinningSourceID == nil) {
		return fmt.Errorf("conflict winning claim and source must be present together")
	}
	if conflict.WinningClaimID == nil && conflict.WinningScope != "" {
		return fmt.Errorf("conflict winning scope requires a winner")
	}
	if conflict.WinningClaimID != nil {
		if err := requireText("conflict winning scope", conflict.WinningScope); err != nil {
			return err
		}
		if err := conflict.WinningClaimID.Validate(); err != nil {
			return err
		}
		if err := conflict.WinningSourceID.Validate(); err != nil {
			return err
		}
		found := false
		for _, claimID := range conflict.ClaimIDs {
			if claimID == *conflict.WinningClaimID {
				found = true
				break
			}
		}
		if !found {
			return fmt.Errorf("conflict winning claim is not a conflicting claim")
		}
	}
	if conflict.Unresolved && conflict.Resolution != "" {
		return fmt.Errorf("unresolved conflict has a resolution")
	}
	if conflict.Unresolved && conflict.WinningClaimID != nil {
		return fmt.Errorf("unresolved conflict cannot have a winner")
	}
	if !conflict.Unresolved {
		if err := requireText("conflict resolution", conflict.Resolution); err != nil {
			return err
		}
	}
	if err := validateTimestamp("conflict detected at", conflict.DetectedAt); err != nil {
		return err
	}
	switch conflict.AlgorithmVersion {
	case ConflictResolverAlgorithmV1:
		if len(conflict.ClaimIDs) != 2 {
			return fmt.Errorf("conflict-resolver-v1 requires exactly 2 claims")
		}
		return nil
	case ConflictLegacyAlgorithm:
		if conflict.WinningClaimID != nil || conflict.WinningSourceID != nil || conflict.WinningScope != "" {
			return fmt.Errorf("legacy conflict cannot contain resolver winner metadata")
		}
		return nil
	default:
		return fmt.Errorf("invalid conflict algorithm version %q", conflict.AlgorithmVersion)
	}
}

type VerificationStatus string

const (
	MultiSourceVerificationAlgorithmV1 = "multi-source-verification-v1"
	VerificationLegacyAlgorithm        = "verification-unversioned-legacy"
)

const (
	VerificationVerified       VerificationStatus = "verified"
	VerificationVerifiedCaveat VerificationStatus = "verified_with_caveat"
	VerificationInsufficient   VerificationStatus = "insufficient_evidence"
	VerificationConflicted     VerificationStatus = "conflicted"
	VerificationRejected       VerificationStatus = "rejected"
)

func (status VerificationStatus) Validate() error {
	switch status {
	case VerificationVerified, VerificationVerifiedCaveat,
		VerificationInsufficient, VerificationConflicted, VerificationRejected:
		return nil
	default:
		return fmt.Errorf("invalid verification status %q", status)
	}
}

type ClaimVerificationRequirement string

const (
	VerificationRequirementNormativePrimary ClaimVerificationRequirement = "normative_primary"
	VerificationRequirementProduction       ClaimVerificationRequirement = "production_recommendation"
	VerificationRequirementSecurity         ClaimVerificationRequirement = "security_authority"
	VerificationRequirementCommunity        ClaimVerificationRequirement = "community_corroboration"
	VerificationRequirementGeneral          ClaimVerificationRequirement = "general_support"
	VerificationRequirementLegacy           ClaimVerificationRequirement = "legacy_unclassified"
)

func (requirement ClaimVerificationRequirement) Validate() error {
	switch requirement {
	case VerificationRequirementNormativePrimary, VerificationRequirementProduction,
		VerificationRequirementSecurity, VerificationRequirementCommunity,
		VerificationRequirementGeneral, VerificationRequirementLegacy:
		return nil
	default:
		return fmt.Errorf("invalid claim verification requirement %q", requirement)
	}
}

type ClaimVerificationReason string

const (
	VerificationReasonPrimarySourceSufficient ClaimVerificationReason = "primary_source_sufficient"
	VerificationReasonIndependentSupport      ClaimVerificationReason = "independent_support"
	VerificationReasonSingleStrongSource      ClaimVerificationReason = "single_strong_source"
	VerificationReasonSecurityAuthority       ClaimVerificationReason = "security_authority_present"
	VerificationReasonSecurityAuthorityAbsent ClaimVerificationReason = "security_authority_absent"
	VerificationReasonCorroborationMissing    ClaimVerificationReason = "corroboration_missing"
	VerificationReasonSameOrganization        ClaimVerificationReason = "same_organization"
	VerificationReasonOrganizationUnknown     ClaimVerificationReason = "organization_unknown"
	VerificationReasonScopeInconsistent       ClaimVerificationReason = "scope_inconsistent"
	VerificationReasonUnresolvedConflict      ClaimVerificationReason = "unresolved_conflict"
	VerificationReasonLosesResolvedConflict   ClaimVerificationReason = "loses_resolved_conflict"
	VerificationReasonSourcesRejected         ClaimVerificationReason = "sources_rejected"
	VerificationReasonWeakSupport             ClaimVerificationReason = "weak_support"
	VerificationReasonLegacyUnclassified      ClaimVerificationReason = "legacy_unclassified"
)

func (reason ClaimVerificationReason) Validate() error {
	switch reason {
	case VerificationReasonPrimarySourceSufficient, VerificationReasonIndependentSupport,
		VerificationReasonSingleStrongSource, VerificationReasonSecurityAuthority,
		VerificationReasonSecurityAuthorityAbsent, VerificationReasonCorroborationMissing,
		VerificationReasonSameOrganization, VerificationReasonOrganizationUnknown,
		VerificationReasonScopeInconsistent, VerificationReasonUnresolvedConflict,
		VerificationReasonLosesResolvedConflict, VerificationReasonSourcesRejected,
		VerificationReasonWeakSupport, VerificationReasonLegacyUnclassified:
		return nil
	default:
		return fmt.Errorf("invalid claim verification reason %q", reason)
	}
}

// VerificationAuthorityDistribution preserves reviewed TrustDecision tiers and
// explicitly counts sources without a decision instead of inventing a tier.
type VerificationAuthorityDistribution struct {
	TierA   int
	TierB   int
	TierC   int
	TierD   int
	TierE   int
	Unknown int
}

func (distribution VerificationAuthorityDistribution) Validate(sourceCount int) error {
	counts := []int{
		distribution.TierA, distribution.TierB, distribution.TierC,
		distribution.TierD, distribution.TierE, distribution.Unknown,
	}
	total := 0
	for _, count := range counts {
		if count < 0 {
			return fmt.Errorf("verification authority distribution contains a negative count")
		}
		total += count
	}
	if total != sourceCount {
		return fmt.Errorf("verification authority distribution totals %d, want %d", total, sourceCount)
	}
	return nil
}

type VerificationMetrics struct {
	SourceCount                  int
	IndependentOrganizationCount int
	AuthorityDistribution        VerificationAuthorityDistribution
	ScopeConsistent              bool
}

func (metrics VerificationMetrics) Validate(expectedSourceCount int) error {
	if metrics.SourceCount != expectedSourceCount {
		return fmt.Errorf("verification source count is %d, want %d", metrics.SourceCount, expectedSourceCount)
	}
	if metrics.IndependentOrganizationCount < 0 || metrics.IndependentOrganizationCount > metrics.SourceCount {
		return fmt.Errorf("verification independent organization count is invalid")
	}
	return metrics.AuthorityDistribution.Validate(metrics.SourceCount)
}

type VerificationResult struct {
	ID               ID
	ClaimID          ClaimID
	Status           VerificationStatus
	Requirement      ClaimVerificationRequirement
	SourceIDs        []SourceID
	Metrics          VerificationMetrics
	ReasonCodes      []ClaimVerificationReason
	Confidence       ClaimConfidence
	VerifiedAt       Timestamp
	AlgorithmVersion string
}

func (result VerificationResult) Validate() error {
	if err := result.ID.Validate(); err != nil {
		return fmt.Errorf("verification result: %w", err)
	}
	if err := result.ClaimID.Validate(); err != nil {
		return err
	}
	if err := result.Status.Validate(); err != nil {
		return err
	}
	if err := validateSourceIDs("verification sources", result.SourceIDs, 1); err != nil {
		return err
	}
	if err := result.Confidence.Validate(); err != nil {
		return fmt.Errorf("verification confidence: %w", err)
	}
	if err := validateTimestamp("claim verified at", result.VerifiedAt); err != nil {
		return err
	}
	if err := result.Requirement.Validate(); err != nil {
		return err
	}
	if len(result.ReasonCodes) == 0 {
		return fmt.Errorf("claim verification reasons are empty")
	}
	seenReasons := make(map[ClaimVerificationReason]struct{}, len(result.ReasonCodes))
	for _, reason := range result.ReasonCodes {
		if err := reason.Validate(); err != nil {
			return err
		}
		if _, exists := seenReasons[reason]; exists {
			return fmt.Errorf("claim verification contains duplicate reason %q", reason)
		}
		seenReasons[reason] = struct{}{}
	}
	switch result.AlgorithmVersion {
	case MultiSourceVerificationAlgorithmV1:
		if result.Requirement == VerificationRequirementLegacy {
			return fmt.Errorf("multi-source-verification-v1 cannot be legacy unclassified")
		}
		if _, exists := seenReasons[VerificationReasonLegacyUnclassified]; exists {
			return fmt.Errorf("multi-source-verification-v1 cannot contain a legacy reason")
		}
		return result.Metrics.Validate(len(result.SourceIDs))
	case VerificationLegacyAlgorithm:
		if result.Requirement != VerificationRequirementLegacy || len(result.ReasonCodes) != 1 ||
			result.ReasonCodes[0] != VerificationReasonLegacyUnclassified {
			return fmt.Errorf("legacy verification result must remain unclassified")
		}
		if result.Metrics != (VerificationMetrics{}) {
			return fmt.Errorf("legacy verification result cannot contain invented metrics")
		}
		return nil
	default:
		return fmt.Errorf("invalid verification algorithm version %q", result.AlgorithmVersion)
	}
}

type DriftType string

const (
	DriftAlgorithmV1     = "drift-v1"
	DriftLegacyAlgorithm = "drift-unversioned-legacy"
)

const (
	DriftSourceChanged         DriftType = "source_changed"
	DriftClaimInvalidated      DriftType = "claim_invalidated"
	DriftVersionSuperseded     DriftType = "version_superseded"
	DriftRecommendationChanged DriftType = "recommendation_changed"
	DriftDeprecationIntroduced DriftType = "deprecation_introduced"
	DriftScopeChanged          DriftType = "scope_changed"
)

func (driftType DriftType) Validate() error {
	switch driftType {
	case DriftSourceChanged, DriftClaimInvalidated, DriftVersionSuperseded,
		DriftRecommendationChanged, DriftDeprecationIntroduced, DriftScopeChanged:
		return nil
	default:
		return fmt.Errorf("invalid drift type %q", driftType)
	}
}

type Severity string

const (
	SeverityInformational Severity = "informational"
	SeverityMinor         Severity = "minor"
	SeverityImportant     Severity = "important"
	SeverityCritical      Severity = "critical"
)

func (severity Severity) Validate() error {
	switch severity {
	case SeverityInformational, SeverityMinor, SeverityImportant, SeverityCritical:
		return nil
	default:
		return fmt.Errorf("invalid severity %q", severity)
	}
}

type DriftReport struct {
	ID               ID
	OldBundleID      ID
	NewBundleID      *ID
	Type             DriftType
	Severity         Severity
	AffectedClaims   []ClaimID
	OldEvidence      []ID
	NewEvidence      []ID
	Confidence       ClaimConfidence
	DetectedAt       Timestamp
	AlgorithmVersion string
}

func (report DriftReport) Validate() error {
	if err := report.ID.Validate(); err != nil {
		return fmt.Errorf("drift report: %w", err)
	}
	if err := report.OldBundleID.Validate(); err != nil {
		return fmt.Errorf("drift old bundle: %w", err)
	}
	if report.NewBundleID != nil {
		if err := report.NewBundleID.Validate(); err != nil {
			return fmt.Errorf("drift new bundle: %w", err)
		}
	}
	if err := report.Type.Validate(); err != nil {
		return err
	}
	if err := report.Severity.Validate(); err != nil {
		return err
	}
	if err := validateClaimIDs("drift affected claims", report.AffectedClaims, 1); err != nil {
		return err
	}
	if err := validateIDs("drift old evidence", report.OldEvidence, 1); err != nil {
		return err
	}
	if err := validateIDs("drift new evidence", report.NewEvidence, 0); err != nil {
		return err
	}
	if err := validateTimestamp("drift detected at", report.DetectedAt); err != nil {
		return err
	}
	switch report.AlgorithmVersion {
	case DriftAlgorithmV1:
		if err := report.Confidence.Validate(); err != nil {
			return fmt.Errorf("drift confidence: %w", err)
		}
	case DriftLegacyAlgorithm:
		if report.Confidence.Value() != 0 {
			return fmt.Errorf("legacy drift report cannot contain invented confidence")
		}
	default:
		return fmt.Errorf("invalid drift algorithm version %q", report.AlgorithmVersion)
	}
	return nil
}

type RecommendedAction string

const (
	ActionNoAction         RecommendedAction = "no_action"
	ActionReverify         RecommendedAction = "reverify"
	ActionReviewCurriculum RecommendedAction = "review_curriculum"
	ActionRecompileFuture  RecommendedAction = "recompile_future"
	ActionManualReview     RecommendedAction = "manual_review"
)

func (action RecommendedAction) Validate() error {
	switch action {
	case ActionNoAction, ActionReverify, ActionReviewCurriculum,
		ActionRecompileFuture, ActionManualReview:
		return nil
	default:
		return fmt.Errorf("invalid recommended action %q", action)
	}
}

type ImpactReport struct {
	ID                ID
	DriftReportID     ID
	AffectedBundleIDs []ID
	AffectedClaimIDs  []ClaimID
	Severity          Severity
	RecommendedAction RecommendedAction
	AssessedAt        Timestamp
}

func (report ImpactReport) Validate() error {
	if err := report.ID.Validate(); err != nil {
		return fmt.Errorf("impact report: %w", err)
	}
	if err := report.DriftReportID.Validate(); err != nil {
		return fmt.Errorf("impact drift report: %w", err)
	}
	if err := validateIDs("impact affected bundles", report.AffectedBundleIDs, 1); err != nil {
		return err
	}
	if err := validateClaimIDs("impact affected claims", report.AffectedClaimIDs, 1); err != nil {
		return err
	}
	if err := report.Severity.Validate(); err != nil {
		return err
	}
	if err := report.RecommendedAction.Validate(); err != nil {
		return err
	}
	return validateTimestamp("impact assessed at", report.AssessedAt)
}
