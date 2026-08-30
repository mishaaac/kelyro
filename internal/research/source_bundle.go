package research

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"sort"
	"strings"
	"time"
	"unicode/utf8"
)

const (
	SourceBundleAlgorithmV1         = "source-bundle-v1"
	SourceBundleLegacyAlgorithm     = "source-bundle-unversioned-legacy"
	SourceBundleFreshnessV1         = "source-bundle-freshness-v1"
	SourceBundleFreshnessLegacy     = "source-bundle-freshness-legacy-unknown"
	MaximumSourceBundleSummaryBytes = 8 << 10
	MaximumSourceBundleWarningBytes = 2 << 10
	MaximumSourceBundleJSONBytes    = 256 << 10
	MaximumSourceBundleItems        = 512
)

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

type SourceBundleSourceRole string

const (
	BundleSourcePrimary      SourceBundleSourceRole = "primary"
	BundleSourceSupporting   SourceBundleSourceRole = "supporting"
	BundleSourceHistorical   SourceBundleSourceRole = "historical"
	BundleSourceUnclassified SourceBundleSourceRole = "unclassified"
)

func (role SourceBundleSourceRole) Validate() error {
	switch role {
	case BundleSourcePrimary, BundleSourceSupporting, BundleSourceHistorical, BundleSourceUnclassified:
		return nil
	default:
		return fmt.Errorf("invalid source bundle source role %q", role)
	}
}

// SourceBundleSource freezes both the source role and temporal annotation used
// during assembly. Later trust or temporal reclassification cannot rewrite it.
type SourceBundleSource struct {
	SourceID      SourceID
	Role          SourceBundleSourceRole
	TemporalScope SourceTemporalScope
	VersionScope  *SourceVersion
	Warning       string
}

func NewSourceBundleSource(source Source) (SourceBundleSource, error) {
	if err := source.Validate(); err != nil {
		return SourceBundleSource{}, err
	}
	role := BundleSourceSupporting
	if source.TemporalScope == SourceTemporalHistorical || source.TemporalScope == SourceTemporalArchived {
		role = BundleSourceHistorical
	}
	warning, err := source.TemporalScope.Warning(source.Version)
	if err != nil {
		return SourceBundleSource{}, err
	}
	result := SourceBundleSource{
		SourceID: source.ID, Role: role, TemporalScope: source.TemporalScope,
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
	if err := source.Role.Validate(); err != nil {
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
	if !utf8.ValidString(source.Warning) || len(source.Warning) > MaximumSourceBundleWarningBytes {
		return fmt.Errorf("source bundle warning must be valid UTF-8 within %d bytes", MaximumSourceBundleWarningBytes)
	}
	if source.Role == BundleSourceHistorical && source.TemporalScope == SourceTemporalCurrent {
		return fmt.Errorf("current source cannot have historical bundle role")
	}
	if source.Role != BundleSourceHistorical && (source.TemporalScope == SourceTemporalHistorical || source.TemporalScope == SourceTemporalArchived) {
		return fmt.Errorf("non-current historical source requires historical bundle role")
	}
	return nil
}

type SourceBundleIssue string

const (
	BundleIssueMissingEvidence      SourceBundleIssue = "missing_required_evidence"
	BundleIssueMissingVerification  SourceBundleIssue = "missing_verification"
	BundleIssueInsufficientEvidence SourceBundleIssue = "insufficient_evidence"
	BundleIssueRejectedClaim        SourceBundleIssue = "rejected_claim"
	BundleIssueLegacyVerification   SourceBundleIssue = "legacy_verification"
	BundleIssueUnresolvedConflict   SourceBundleIssue = "unresolved_conflict"
	BundleIssueResolvedConflict     SourceBundleIssue = "resolved_conflict"
	BundleIssueVerificationCaveat   SourceBundleIssue = "verification_caveat"
	BundleIssueNonCurrentSource     SourceBundleIssue = "non_current_source"
	BundleIssueMissingFreshness     SourceBundleIssue = "missing_freshness"
	BundleIssueAgingFreshness       SourceBundleIssue = "aging_freshness"
	BundleIssueStaleFreshness       SourceBundleIssue = "stale_freshness"
)

func (issue SourceBundleIssue) Validate() error {
	switch issue {
	case BundleIssueMissingEvidence, BundleIssueMissingVerification,
		BundleIssueInsufficientEvidence, BundleIssueRejectedClaim,
		BundleIssueLegacyVerification, BundleIssueUnresolvedConflict,
		BundleIssueResolvedConflict, BundleIssueVerificationCaveat,
		BundleIssueNonCurrentSource, BundleIssueMissingFreshness,
		BundleIssueAgingFreshness, BundleIssueStaleFreshness:
		return nil
	default:
		return fmt.Errorf("invalid source bundle issue %q", issue)
	}
}

// SourceBundleFreshness conservatively summarizes claim freshness. Score is
// the minimum known score and LastVerifiedAt is the oldest known verification.
type SourceBundleFreshness struct {
	State            FreshnessState
	Score            FreshnessScore
	LastVerifiedAt   *Timestamp
	MissingClaimIDs  []ClaimID
	SourceAlgorithms []string
	AlgorithmVersion string
}

func (freshness SourceBundleFreshness) Validate() error {
	if err := freshness.State.Validate(); err != nil {
		return err
	}
	if err := freshness.Score.Validate(); err != nil {
		return err
	}
	if err := validateOptionalTimestamp("source bundle freshness last verified", freshness.LastVerifiedAt); err != nil {
		return err
	}
	if err := validateClaimIDs("source bundle missing freshness claims", freshness.MissingClaimIDs, 0); err != nil {
		return err
	}
	seenAlgorithms := make(map[string]struct{}, len(freshness.SourceAlgorithms))
	for _, algorithm := range freshness.SourceAlgorithms {
		if err := requireText("source bundle freshness source algorithm", algorithm); err != nil {
			return err
		}
		if _, exists := seenAlgorithms[algorithm]; exists {
			return fmt.Errorf("source bundle freshness contains duplicate source algorithm %q", algorithm)
		}
		seenAlgorithms[algorithm] = struct{}{}
	}
	switch freshness.AlgorithmVersion {
	case SourceBundleFreshnessV1:
		if len(freshness.MissingClaimIDs) > 0 {
			if freshness.State != FreshnessUnknown {
				return fmt.Errorf("missing claim freshness requires unknown bundle freshness")
			}
		} else if freshness.State == FreshnessUnknown || freshness.LastVerifiedAt == nil || len(freshness.SourceAlgorithms) == 0 {
			return fmt.Errorf("known bundle freshness requires verification metadata")
		}
	case SourceBundleFreshnessLegacy:
		if freshness.State != FreshnessUnknown || freshness.Score.Value() != 0 || freshness.LastVerifiedAt != nil ||
			len(freshness.MissingClaimIDs) != 0 || len(freshness.SourceAlgorithms) != 0 {
			return fmt.Errorf("legacy bundle freshness must remain unknown")
		}
	default:
		return fmt.Errorf("invalid source bundle freshness algorithm %q", freshness.AlgorithmVersion)
	}
	return nil
}

// SourceBundle is the immutable hand-off from I-03. It contains identities and
// bounded annotations, never raw fetched pages or full evidence excerpts.
type SourceBundle struct {
	ID               ID
	RunID            ID
	Topic            ResearchTopic
	Purpose          ResearchPurpose
	TargetVersion    *SourceVersion
	ClaimIDs         []ClaimID
	Sources          []SourceBundleSource
	ConflictIDs      []ID
	Freshness        SourceBundleFreshness
	Issues           []SourceBundleIssue
	State            SourceBundleState
	Summary          string
	VerifiedAt       Timestamp
	AlgorithmVersion string
	ContentHash      string
}

func (bundle SourceBundle) Validate() error {
	if err := bundle.validateCore(); err != nil {
		return err
	}
	if bundle.AlgorithmVersion == SourceBundleLegacyAlgorithm {
		if bundle.ContentHash != "" {
			return fmt.Errorf("legacy source bundle cannot contain an invented hash")
		}
		return nil
	}
	if err := ValidateCanonicalContentHashV1(bundle.ContentHash); err != nil {
		return fmt.Errorf("source bundle content hash: %w", err)
	}
	payload, err := bundle.canonicalJSON(false)
	if err != nil {
		return err
	}
	if want := CanonicalContentHashV1(payload); bundle.ContentHash != want {
		return fmt.Errorf("source bundle content hash does not match canonical representation")
	}
	return nil
}

func (bundle SourceBundle) validateCore() error {
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
	if len(bundle.ClaimIDs) > MaximumSourceBundleItems {
		return fmt.Errorf("source bundle claims exceed %d", MaximumSourceBundleItems)
	}
	if len(bundle.Sources) == 0 || len(bundle.Sources) > MaximumSourceBundleItems {
		return fmt.Errorf("source bundle source count must be between 1 and %d", MaximumSourceBundleItems)
	}
	seenSources := make(map[SourceID]struct{}, len(bundle.Sources))
	for index, source := range bundle.Sources {
		if err := source.Validate(); err != nil {
			return fmt.Errorf("source bundle source %d: %w", index, err)
		}
		if _, exists := seenSources[source.SourceID]; exists {
			return fmt.Errorf("source bundle sources contains duplicate source id %q", source.SourceID)
		}
		seenSources[source.SourceID] = struct{}{}
		if source.TemporalScope == SourceTemporalVersionBound && source.Role != BundleSourceHistorical &&
			(bundle.TargetVersion == nil || *source.VersionScope != *bundle.TargetVersion) {
			return fmt.Errorf("version-bound source does not match source bundle target version")
		}
	}
	if err := validateIDs("source bundle conflicts", bundle.ConflictIDs, 0); err != nil {
		return err
	}
	if len(bundle.ConflictIDs) > MaximumSourceBundleItems {
		return fmt.Errorf("source bundle conflicts exceed %d", MaximumSourceBundleItems)
	}
	if err := bundle.Freshness.Validate(); err != nil {
		return fmt.Errorf("source bundle freshness: %w", err)
	}
	claimSet := make(map[ClaimID]struct{}, len(bundle.ClaimIDs))
	for _, claimID := range bundle.ClaimIDs {
		claimSet[claimID] = struct{}{}
	}
	for _, claimID := range bundle.Freshness.MissingClaimIDs {
		if _, exists := claimSet[claimID]; !exists {
			return fmt.Errorf("source bundle freshness references claim %q outside bundle", claimID)
		}
	}
	if bundle.Freshness.LastVerifiedAt != nil && bundle.Freshness.LastVerifiedAt.After(bundle.VerifiedAt) {
		return fmt.Errorf("source bundle freshness verification is after bundle")
	}
	seenIssues := make(map[SourceBundleIssue]struct{}, len(bundle.Issues))
	for _, issue := range bundle.Issues {
		if err := issue.Validate(); err != nil {
			return err
		}
		if _, exists := seenIssues[issue]; exists {
			return fmt.Errorf("source bundle contains duplicate issue %q", issue)
		}
		seenIssues[issue] = struct{}{}
	}
	if err := bundle.State.Validate(); err != nil {
		return err
	}
	if err := requireText("source bundle summary", bundle.Summary); err != nil {
		return err
	}
	if !utf8.ValidString(bundle.Summary) || len(bundle.Summary) > MaximumSourceBundleSummaryBytes {
		return fmt.Errorf("source bundle summary must be valid UTF-8 within %d bytes", MaximumSourceBundleSummaryBytes)
	}
	if err := validateTimestamp("source bundle verified at", bundle.VerifiedAt); err != nil {
		return err
	}
	switch bundle.AlgorithmVersion {
	case SourceBundleAlgorithmV1:
		for _, source := range bundle.Sources {
			if source.Role == BundleSourceUnclassified {
				return fmt.Errorf("source-bundle-v1 cannot contain an unclassified source")
			}
		}
		if bundle.Freshness.AlgorithmVersion != SourceBundleFreshnessV1 {
			return fmt.Errorf("source-bundle-v1 requires source-bundle-freshness-v1")
		}
		if len(bundle.Freshness.MissingClaimIDs) > 0 && !containsSourceBundleIssue(bundle.Issues, BundleIssueMissingFreshness) {
			return fmt.Errorf("missing freshness requires a source bundle issue")
		}
		if bundle.Freshness.State == FreshnessAging && !containsSourceBundleIssue(bundle.Issues, BundleIssueAgingFreshness) {
			return fmt.Errorf("aging freshness requires a source bundle issue")
		}
		if bundle.Freshness.State == FreshnessStale && !containsSourceBundleIssue(bundle.Issues, BundleIssueStaleFreshness) {
			return fmt.Errorf("stale freshness requires a source bundle issue")
		}
		for _, source := range bundle.Sources {
			if source.Role == BundleSourceHistorical && !containsSourceBundleIssue(bundle.Issues, BundleIssueNonCurrentSource) {
				return fmt.Errorf("historical source requires a source bundle issue")
			}
		}
		if want := sourceBundleStateForIssues(bundle.Issues); bundle.State != want {
			return fmt.Errorf("source bundle state %q does not match issues; want %q", bundle.State, want)
		}
	case SourceBundleLegacyAlgorithm:
		if bundle.Freshness.AlgorithmVersion != SourceBundleFreshnessLegacy || len(bundle.Issues) != 0 {
			return fmt.Errorf("legacy source bundle cannot contain invented v1 assessment metadata")
		}
	default:
		return fmt.Errorf("invalid source bundle algorithm version %q", bundle.AlgorithmVersion)
	}
	return nil
}

func containsSourceBundleIssue(issues []SourceBundleIssue, target SourceBundleIssue) bool {
	for _, issue := range issues {
		if issue == target {
			return true
		}
	}
	return false
}

func sourceBundleStateForIssues(issues []SourceBundleIssue) SourceBundleState {
	for _, issue := range issues {
		switch issue {
		case BundleIssueMissingEvidence, BundleIssueMissingVerification,
			BundleIssueInsufficientEvidence, BundleIssueRejectedClaim,
			BundleIssueLegacyVerification, BundleIssueMissingFreshness:
			return BundleIncomplete
		}
	}
	for _, issue := range issues {
		if issue == BundleIssueUnresolvedConflict {
			return BundleConflicted
		}
	}
	if len(issues) > 0 {
		return BundleReadyWithCaveats
	}
	return BundleReady
}

// SealSourceBundleV1 canonicalizes collection order, derives the state and
// summary, and binds the result to its reproducible content hash.
func SealSourceBundleV1(bundle SourceBundle) (SourceBundle, error) {
	bundle.AlgorithmVersion = SourceBundleAlgorithmV1
	bundle.ClaimIDs = canonicalClaimIDs(bundle.ClaimIDs)
	bundle.Sources = canonicalBundleSources(bundle.Sources)
	bundle.ConflictIDs = canonicalIDs(bundle.ConflictIDs)
	bundle.Freshness.MissingClaimIDs = canonicalClaimIDs(bundle.Freshness.MissingClaimIDs)
	bundle.Freshness.SourceAlgorithms = canonicalStrings(bundle.Freshness.SourceAlgorithms)
	bundle.Issues = canonicalBundleIssues(bundle.Issues)
	bundle.State = sourceBundleStateForIssues(bundle.Issues)
	bundle.Summary = sourceBundleSummary(bundle)
	bundle.ContentHash = ""
	if err := bundle.validateCore(); err != nil {
		return SourceBundle{}, err
	}
	payload, err := bundle.canonicalJSON(false)
	if err != nil {
		return SourceBundle{}, err
	}
	bundle.ContentHash = CanonicalContentHashV1(payload)
	if err := bundle.Validate(); err != nil {
		return SourceBundle{}, err
	}
	return bundle, nil
}

func sourceBundleSummary(bundle SourceBundle) string {
	counts := map[SourceBundleSourceRole]int{}
	for _, source := range bundle.Sources {
		counts[source.Role]++
	}
	summary := fmt.Sprintf(
		"Source bundle for %q is %s: %d claims, %d primary sources, %d supporting sources, %d historical sources, %d conflicts; freshness %s.",
		bundle.Topic.Subject, bundle.State, len(bundle.ClaimIDs), counts[BundleSourcePrimary],
		counts[BundleSourceSupporting], counts[BundleSourceHistorical], len(bundle.ConflictIDs), bundle.Freshness.State,
	)
	if len(bundle.Issues) > 0 {
		values := make([]string, len(bundle.Issues))
		for index, issue := range bundle.Issues {
			values[index] = string(issue)
		}
		summary += " Issues: " + strings.Join(values, ", ") + "."
	}
	return summary
}

type sourceBundleJSON struct {
	BundleID            string                    `json:"bundle_id"`
	ResearchTopic       researchTopicJSON         `json:"research_topic"`
	Purpose             string                    `json:"purpose"`
	TargetVersion       string                    `json:"target_version,omitempty"`
	Claims              []string                  `json:"claims"`
	PrimarySources      []sourceBundleSourceJSON  `json:"primary_sources"`
	SupportingSources   []sourceBundleSourceJSON  `json:"supporting_sources"`
	HistoricalSources   []sourceBundleSourceJSON  `json:"historical_sources"`
	UnclassifiedSources []sourceBundleSourceJSON  `json:"unclassified_sources,omitempty"`
	Conflicts           []string                  `json:"conflicts"`
	Freshness           sourceBundleFreshnessJSON `json:"freshness"`
	Issues              []string                  `json:"issues"`
	State               string                    `json:"state"`
	Summary             string                    `json:"summary"`
	VerifiedAt          string                    `json:"verified_at"`
	ResearchRun         string                    `json:"research_run"`
	AlgorithmVersion    string                    `json:"algorithm_version"`
	ContentHash         string                    `json:"content_hash,omitempty"`
}

type researchTopicJSON struct {
	Subject    string `json:"subject"`
	Domain     string `json:"domain,omitempty"`
	Technology string `json:"technology,omitempty"`
}

type sourceBundleSourceJSON struct {
	SourceID      string `json:"source_id"`
	TemporalScope string `json:"temporal_scope"`
	VersionScope  string `json:"version_scope,omitempty"`
	Warning       string `json:"warning,omitempty"`
}

type sourceBundleFreshnessJSON struct {
	State            string   `json:"state"`
	Score            float64  `json:"score"`
	LastVerifiedAt   string   `json:"last_verified_at,omitempty"`
	MissingClaims    []string `json:"missing_claims"`
	SourceAlgorithms []string `json:"source_algorithms"`
	AlgorithmVersion string   `json:"algorithm_version"`
}

// ExportJSON returns the bounded canonical machine representation.
func (bundle SourceBundle) ExportJSON() ([]byte, error) {
	if err := bundle.Validate(); err != nil {
		return nil, err
	}
	return bundle.canonicalJSON(true)
}

func (bundle SourceBundle) canonicalJSON(includeHash bool) ([]byte, error) {
	claims := canonicalClaimIDs(bundle.ClaimIDs)
	sources := canonicalBundleSources(bundle.Sources)
	conflicts := canonicalIDs(bundle.ConflictIDs)
	issues := canonicalBundleIssues(bundle.Issues)
	payload := sourceBundleJSON{
		BundleID: bundle.ID.String(), ResearchTopic: researchTopicJSON{
			Subject: bundle.Topic.Subject, Domain: bundle.Topic.Domain, Technology: bundle.Topic.Technology,
		},
		Purpose: string(bundle.Purpose), Claims: make([]string, len(claims)),
		PrimarySources: make([]sourceBundleSourceJSON, 0), SupportingSources: make([]sourceBundleSourceJSON, 0),
		HistoricalSources: make([]sourceBundleSourceJSON, 0), UnclassifiedSources: make([]sourceBundleSourceJSON, 0),
		Conflicts: make([]string, len(conflicts)), Issues: make([]string, len(issues)),
		State: string(bundle.State), Summary: bundle.Summary,
		VerifiedAt: bundle.VerifiedAt.Time().Format(time.RFC3339Nano), ResearchRun: bundle.RunID.String(),
		AlgorithmVersion: bundle.AlgorithmVersion,
		Freshness: sourceBundleFreshnessJSON{
			State: string(bundle.Freshness.State), Score: bundle.Freshness.Score.Value(),
			MissingClaims:    make([]string, len(bundle.Freshness.MissingClaimIDs)),
			SourceAlgorithms: canonicalStrings(bundle.Freshness.SourceAlgorithms),
			AlgorithmVersion: bundle.Freshness.AlgorithmVersion,
		},
	}
	if bundle.TargetVersion != nil {
		payload.TargetVersion = bundle.TargetVersion.String()
	}
	if bundle.Freshness.LastVerifiedAt != nil {
		payload.Freshness.LastVerifiedAt = bundle.Freshness.LastVerifiedAt.Time().Format(time.RFC3339Nano)
	}
	if includeHash {
		payload.ContentHash = bundle.ContentHash
	}
	for index, id := range claims {
		payload.Claims[index] = id.String()
	}
	for _, source := range sources {
		item := sourceBundleSourceJSON{SourceID: source.SourceID.String(), TemporalScope: string(source.TemporalScope), Warning: source.Warning}
		if source.VersionScope != nil {
			item.VersionScope = source.VersionScope.String()
		}
		switch source.Role {
		case BundleSourcePrimary:
			payload.PrimarySources = append(payload.PrimarySources, item)
		case BundleSourceSupporting:
			payload.SupportingSources = append(payload.SupportingSources, item)
		case BundleSourceHistorical:
			payload.HistoricalSources = append(payload.HistoricalSources, item)
		case BundleSourceUnclassified:
			payload.UnclassifiedSources = append(payload.UnclassifiedSources, item)
		}
	}
	for index, id := range conflicts {
		payload.Conflicts[index] = id.String()
	}
	for index, id := range canonicalClaimIDs(bundle.Freshness.MissingClaimIDs) {
		payload.Freshness.MissingClaims[index] = id.String()
	}
	for index, issue := range issues {
		payload.Issues[index] = string(issue)
	}
	encoded, err := json.MarshalIndent(payload, "", "  ")
	if err != nil {
		return nil, fmt.Errorf("encode source bundle: %w", err)
	}
	if len(encoded) > MaximumSourceBundleJSONBytes {
		return nil, fmt.Errorf("source bundle export exceeds %d bytes", MaximumSourceBundleJSONBytes)
	}
	return encoded, nil
}

// ParseSourceBundleJSON accepts only the bounded machine representation and
// revalidates its hash and all closed vocabulary before returning it.
func ParseSourceBundleJSON(encoded []byte) (SourceBundle, error) {
	if len(encoded) == 0 || len(encoded) > MaximumSourceBundleJSONBytes {
		return SourceBundle{}, fmt.Errorf("source bundle JSON size must be between 1 and %d bytes", MaximumSourceBundleJSONBytes)
	}
	decoder := json.NewDecoder(bytes.NewReader(encoded))
	decoder.DisallowUnknownFields()
	var payload sourceBundleJSON
	if err := decoder.Decode(&payload); err != nil {
		return SourceBundle{}, fmt.Errorf("decode source bundle: %w", err)
	}
	var extra any
	if err := decoder.Decode(&extra); err != io.EOF {
		return SourceBundle{}, fmt.Errorf("decode source bundle: trailing data")
	}
	bundle, err := sourceBundleFromJSON(payload)
	if err != nil {
		return SourceBundle{}, err
	}
	if err := bundle.Validate(); err != nil {
		return SourceBundle{}, err
	}
	canonical, err := bundle.canonicalJSON(true)
	if err != nil {
		return SourceBundle{}, err
	}
	if !bytes.Equal(encoded, canonical) {
		return SourceBundle{}, fmt.Errorf("source bundle JSON is not canonical")
	}
	return bundle, nil
}

func sourceBundleFromJSON(payload sourceBundleJSON) (SourceBundle, error) {
	id, err := NewID(payload.BundleID)
	if err != nil {
		return SourceBundle{}, err
	}
	runID, err := NewID(payload.ResearchRun)
	if err != nil {
		return SourceBundle{}, err
	}
	topic, err := NewResearchTopic(payload.ResearchTopic.Subject, payload.ResearchTopic.Domain, payload.ResearchTopic.Technology)
	if err != nil {
		return SourceBundle{}, err
	}
	verifiedAt, err := parseSourceBundleTimestamp(payload.VerifiedAt)
	if err != nil {
		return SourceBundle{}, err
	}
	bundle := SourceBundle{
		ID: id, RunID: runID, Topic: topic, Purpose: ResearchPurpose(payload.Purpose),
		State: SourceBundleState(payload.State), Summary: payload.Summary, VerifiedAt: verifiedAt,
		AlgorithmVersion: payload.AlgorithmVersion, ContentHash: payload.ContentHash,
	}
	if payload.TargetVersion != "" {
		version, versionErr := NewSourceVersion(payload.TargetVersion)
		if versionErr != nil {
			return SourceBundle{}, versionErr
		}
		bundle.TargetVersion = &version
	}
	bundle.ClaimIDs, err = parseSourceBundleClaimIDs(payload.Claims)
	if err != nil {
		return SourceBundle{}, err
	}
	for _, group := range []struct {
		role  SourceBundleSourceRole
		items []sourceBundleSourceJSON
	}{
		{BundleSourcePrimary, payload.PrimarySources}, {BundleSourceSupporting, payload.SupportingSources},
		{BundleSourceHistorical, payload.HistoricalSources}, {BundleSourceUnclassified, payload.UnclassifiedSources},
	} {
		for _, item := range group.items {
			sourceID, sourceErr := NewSourceID(item.SourceID)
			if sourceErr != nil {
				return SourceBundle{}, sourceErr
			}
			source := SourceBundleSource{SourceID: sourceID, Role: group.role, TemporalScope: SourceTemporalScope(item.TemporalScope), Warning: item.Warning}
			if item.VersionScope != "" {
				version, versionErr := NewSourceVersion(item.VersionScope)
				if versionErr != nil {
					return SourceBundle{}, versionErr
				}
				source.VersionScope = &version
			}
			bundle.Sources = append(bundle.Sources, source)
		}
	}
	bundle.ConflictIDs, err = parseSourceBundleIDs(payload.Conflicts)
	if err != nil {
		return SourceBundle{}, err
	}
	missing, err := parseSourceBundleClaimIDs(payload.Freshness.MissingClaims)
	if err != nil {
		return SourceBundle{}, err
	}
	score, err := NewFreshnessScore(payload.Freshness.Score)
	if err != nil {
		return SourceBundle{}, err
	}
	var lastVerified *Timestamp
	if payload.Freshness.LastVerifiedAt != "" {
		value, timeErr := parseSourceBundleTimestamp(payload.Freshness.LastVerifiedAt)
		if timeErr != nil {
			return SourceBundle{}, timeErr
		}
		lastVerified = &value
	}
	bundle.Freshness = SourceBundleFreshness{
		State: FreshnessState(payload.Freshness.State), Score: score, LastVerifiedAt: lastVerified,
		MissingClaimIDs: missing, SourceAlgorithms: append([]string(nil), payload.Freshness.SourceAlgorithms...),
		AlgorithmVersion: payload.Freshness.AlgorithmVersion,
	}
	bundle.Issues = make([]SourceBundleIssue, len(payload.Issues))
	for index, issue := range payload.Issues {
		bundle.Issues[index] = SourceBundleIssue(issue)
	}
	return bundle, nil
}

func parseSourceBundleClaimIDs(values []string) ([]ClaimID, error) {
	result := make([]ClaimID, len(values))
	for index, value := range values {
		id, err := NewClaimID(value)
		if err != nil {
			return nil, err
		}
		result[index] = id
	}
	return result, nil
}

func parseSourceBundleIDs(values []string) ([]ID, error) {
	result := make([]ID, len(values))
	for index, value := range values {
		id, err := NewID(value)
		if err != nil {
			return nil, err
		}
		result[index] = id
	}
	return result, nil
}

func parseSourceBundleTimestamp(value string) (Timestamp, error) {
	parsed, err := time.Parse(time.RFC3339Nano, value)
	if err != nil {
		return Timestamp{}, err
	}
	return NewTimestamp(parsed)
}

func canonicalClaimIDs(values []ClaimID) []ClaimID {
	result := append([]ClaimID(nil), values...)
	sort.Slice(result, func(i, j int) bool { return result[i].String() < result[j].String() })
	return result
}

func canonicalIDs(values []ID) []ID {
	result := append([]ID(nil), values...)
	sort.Slice(result, func(i, j int) bool { return result[i].String() < result[j].String() })
	return result
}

func canonicalStrings(values []string) []string {
	result := append([]string(nil), values...)
	sort.Strings(result)
	return result
}

func canonicalBundleSources(values []SourceBundleSource) []SourceBundleSource {
	result := append([]SourceBundleSource(nil), values...)
	sort.Slice(result, func(i, j int) bool {
		if result[i].Role != result[j].Role {
			return sourceBundleRoleOrder(result[i].Role) < sourceBundleRoleOrder(result[j].Role)
		}
		return result[i].SourceID.String() < result[j].SourceID.String()
	})
	return result
}

func sourceBundleRoleOrder(role SourceBundleSourceRole) int {
	switch role {
	case BundleSourcePrimary:
		return 0
	case BundleSourceSupporting:
		return 1
	case BundleSourceHistorical:
		return 2
	default:
		return 3
	}
}

func canonicalBundleIssues(values []SourceBundleIssue) []SourceBundleIssue {
	result := append([]SourceBundleIssue(nil), values...)
	sort.Slice(result, func(i, j int) bool { return result[i] < result[j] })
	return result
}
