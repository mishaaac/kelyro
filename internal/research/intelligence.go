package research

import "fmt"

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

type ReleaseRecord struct {
	ID           ID
	TechnologyID ID
	Version      SourceVersion
	Channel      ReleaseChannel
	Status       ReleaseStatus
	SourceIDs    []SourceID
	ReleasedAt   *Timestamp
	VerifiedAt   Timestamp
}

func (record ReleaseRecord) Validate() error {
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
	return nil
}

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

type DeprecationRecord struct {
	ID           ID
	Subject      string
	Status       DeprecationStatus
	IntroducedIn *SourceVersion
	DeprecatedIn *SourceVersion
	RemovedIn    *SourceVersion
	Replacement  string
	SourceIDs    []SourceID
	EvidenceIDs  []ID
	VerifiedAt   Timestamp
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
	return validateTimestamp("deprecation verified at", record.VerifiedAt)
}

type ConflictType string

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
	ID         ID
	Type       ConflictType
	ClaimIDs   []ClaimID
	Resolution string
	Unresolved bool
	DetectedAt Timestamp
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
	if conflict.Unresolved && conflict.Resolution != "" {
		return fmt.Errorf("unresolved conflict has a resolution")
	}
	if !conflict.Unresolved {
		if err := requireText("conflict resolution", conflict.Resolution); err != nil {
			return err
		}
	}
	return validateTimestamp("conflict detected at", conflict.DetectedAt)
}

type VerificationStatus string

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

type VerificationResult struct {
	ID         ID
	ClaimID    ClaimID
	Status     VerificationStatus
	SourceIDs  []SourceID
	Confidence ClaimConfidence
	VerifiedAt Timestamp
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
	return validateTimestamp("claim verified at", result.VerifiedAt)
}

type DriftType string

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
	ID             ID
	OldBundleID    ID
	NewBundleID    *ID
	Type           DriftType
	Severity       Severity
	AffectedClaims []ClaimID
	OldEvidence    []ID
	NewEvidence    []ID
	DetectedAt     Timestamp
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
	return validateTimestamp("drift detected at", report.DetectedAt)
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
