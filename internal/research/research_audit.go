package research

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"sort"
	"time"
	"unicode/utf8"
)

const (
	ResearchAuditAlgorithmV1       = "research-audit-v1"
	MaximumResearchAuditJSONBytes  = 256 << 10
	MaximumResearchAuditQueries    = 64
	MaximumResearchAuditSources    = 512
	MaximumResearchAuditProviders  = 32
	MaximumResearchAuditAlgorithms = 64
	MaximumResearchAuditTextBytes  = 4 << 10
)

type ResearchAuditNetworkMode string

const (
	ResearchAuditNetworkOffline ResearchAuditNetworkMode = "offline"
	ResearchAuditNetworkOnline  ResearchAuditNetworkMode = "online"
	ResearchAuditNetworkAuto    ResearchAuditNetworkMode = "auto"
)

func (mode ResearchAuditNetworkMode) Validate() error {
	switch mode {
	case ResearchAuditNetworkOffline, ResearchAuditNetworkOnline, ResearchAuditNetworkAuto:
		return nil
	default:
		return fmt.Errorf("invalid research audit network mode %q", mode)
	}
}

// ResearchAuditAlgorithm records an additional algorithm selected for a run.
// The four required pipeline stages remain first-class fields on the audit.
type ResearchAuditAlgorithm struct {
	Stage   string
	Version string
}

func (algorithm ResearchAuditAlgorithm) Validate() error {
	if err := validateResearchAuditText("research audit algorithm stage", algorithm.Stage, true); err != nil {
		return err
	}
	return validateResearchAuditText("research audit algorithm version", algorithm.Version, true)
}

// ResearchAuditSource binds the exact locator read during a run to the durable
// snapshot and canonical decoded-content hash that were observed.
type ResearchAuditSource struct {
	SourceID     SourceID
	Locator      SourceLocator
	SnapshotID   ID
	SnapshotHash string
}

func (source ResearchAuditSource) Validate() error {
	if err := source.SourceID.Validate(); err != nil {
		return fmt.Errorf("research audit source: %w", err)
	}
	if err := source.Locator.Validate(); err != nil {
		return fmt.Errorf("research audit source locator: %w", err)
	}
	if err := source.SnapshotID.Validate(); err != nil {
		return fmt.Errorf("research audit snapshot: %w", err)
	}
	if err := ValidateCanonicalContentHashV1(source.SnapshotHash); err != nil {
		return fmt.Errorf("research audit snapshot hash: %w", err)
	}
	return nil
}

// ResearchRunAudit is one immutable lifecycle checkpoint for a ResearchRun.
// Multiple records form its append-only audit trail.
type ResearchRunAudit struct {
	ID                      ID
	RunID                   ID
	RecordedAt              Timestamp
	StartedAt               Timestamp
	CompletedAt             *Timestamp
	Outcome                 ResearchRunStatus
	QueryPlannerVersion     string
	TrustPolicyVersion      string
	FreshnessVersion        string
	ConflictResolverVersion string
	ProvidersUsed           []string
	NetworkMode             ResearchAuditNetworkMode
	NetworkAllowed          bool
	CacheHits               int
	SourceCount             int
	BytesFetched            int64
	Queries                 []string
	Sources                 []ResearchAuditSource
	TargetTechnology        string
	TargetVersion           *SourceVersion
	AdditionalAlgorithms    []ResearchAuditAlgorithm
	AlgorithmVersion        string
	ContentHash             string
}

func (audit ResearchRunAudit) Validate() error {
	if err := audit.validateCore(); err != nil {
		return err
	}
	if err := ValidateCanonicalContentHashV1(audit.ContentHash); err != nil {
		return fmt.Errorf("research audit content hash: %w", err)
	}
	payload, err := audit.canonicalJSON(false)
	if err != nil {
		return err
	}
	if want := CanonicalContentHashV1(payload); audit.ContentHash != want {
		return fmt.Errorf("research audit content hash does not match canonical representation")
	}
	return nil
}

func (audit ResearchRunAudit) validateCore() error {
	if err := audit.ID.Validate(); err != nil {
		return fmt.Errorf("research audit: %w", err)
	}
	if err := audit.RunID.Validate(); err != nil {
		return fmt.Errorf("research audit run: %w", err)
	}
	if err := validateTimestamp("research audit recorded at", audit.RecordedAt); err != nil {
		return err
	}
	if err := validateTimestamp("research audit started at", audit.StartedAt); err != nil {
		return err
	}
	if err := validateOptionalTimestamp("research audit completed at", audit.CompletedAt); err != nil {
		return err
	}
	if audit.RecordedAt.Before(audit.StartedAt) {
		return fmt.Errorf("research audit was recorded before the run started")
	}
	if audit.CompletedAt != nil {
		if audit.CompletedAt.Before(audit.StartedAt) {
			return fmt.Errorf("research audit completion precedes start")
		}
		if audit.RecordedAt.Before(*audit.CompletedAt) {
			return fmt.Errorf("research audit was recorded before run completion")
		}
	}
	if err := audit.Outcome.Validate(); err != nil {
		return err
	}
	switch audit.Outcome {
	case ResearchRunCompleted, ResearchRunFailed, ResearchRunCancelled:
		if audit.CompletedAt == nil {
			return fmt.Errorf("terminal research audit outcome has no completion timestamp")
		}
	case ResearchRunPlanned, ResearchRunRunning:
		if audit.CompletedAt != nil {
			return fmt.Errorf("non-terminal research audit outcome has a completion timestamp")
		}
	}
	versions := []struct{ name, value string }{
		{"query planner", audit.QueryPlannerVersion},
		{"trust policy", audit.TrustPolicyVersion},
		{"freshness", audit.FreshnessVersion},
		{"conflict resolver", audit.ConflictResolverVersion},
	}
	for _, version := range versions {
		if err := validateResearchAuditText("research audit "+version.name+" version", version.value, true); err != nil {
			return err
		}
	}
	if len(audit.ProvidersUsed) > MaximumResearchAuditProviders {
		return fmt.Errorf("research audit providers exceed %d", MaximumResearchAuditProviders)
	}
	seenProviders := make(map[string]struct{}, len(audit.ProvidersUsed))
	for _, provider := range audit.ProvidersUsed {
		if err := validateResearchAuditText("research audit provider", provider, true); err != nil {
			return err
		}
		if _, exists := seenProviders[provider]; exists {
			return fmt.Errorf("research audit contains duplicate provider %q", provider)
		}
		seenProviders[provider] = struct{}{}
	}
	if err := audit.NetworkMode.Validate(); err != nil {
		return err
	}
	if audit.NetworkMode == ResearchAuditNetworkOffline && audit.NetworkAllowed {
		return fmt.Errorf("offline research audit cannot allow network")
	}
	if audit.CacheHits < 0 || audit.SourceCount < 0 || audit.BytesFetched < 0 {
		return fmt.Errorf("research audit counters cannot be negative")
	}
	if len(audit.Queries) == 0 || len(audit.Queries) > MaximumResearchAuditQueries {
		return fmt.Errorf("research audit query count must be between 1 and %d", MaximumResearchAuditQueries)
	}
	seenQueries := make(map[string]struct{}, len(audit.Queries))
	for _, query := range audit.Queries {
		if err := validateResearchAuditText("research audit query", query, true); err != nil {
			return err
		}
		if _, exists := seenQueries[query]; exists {
			return fmt.Errorf("research audit contains duplicate query %q", query)
		}
		seenQueries[query] = struct{}{}
	}
	if len(audit.Sources) > MaximumResearchAuditSources || audit.SourceCount != len(audit.Sources) {
		return fmt.Errorf("research audit source count does not match its bounded source records")
	}
	seenSources := make(map[ID]struct{}, len(audit.Sources))
	for index, source := range audit.Sources {
		if err := source.Validate(); err != nil {
			return fmt.Errorf("research audit source %d: %w", index, err)
		}
		if _, exists := seenSources[source.SnapshotID]; exists {
			return fmt.Errorf("research audit contains duplicate snapshot %q", source.SnapshotID)
		}
		seenSources[source.SnapshotID] = struct{}{}
	}
	if err := validateResearchAuditText("research audit target technology", audit.TargetTechnology, false); err != nil {
		return err
	}
	if audit.TargetVersion != nil {
		if err := audit.TargetVersion.Validate(); err != nil {
			return err
		}
	}
	if len(audit.AdditionalAlgorithms) > MaximumResearchAuditAlgorithms {
		return fmt.Errorf("research audit additional algorithms exceed %d", MaximumResearchAuditAlgorithms)
	}
	seenStages := make(map[string]struct{}, len(audit.AdditionalAlgorithms))
	for _, algorithm := range audit.AdditionalAlgorithms {
		if err := algorithm.Validate(); err != nil {
			return err
		}
		if _, exists := seenStages[algorithm.Stage]; exists {
			return fmt.Errorf("research audit contains duplicate algorithm stage %q", algorithm.Stage)
		}
		seenStages[algorithm.Stage] = struct{}{}
	}
	if audit.AlgorithmVersion != ResearchAuditAlgorithmV1 {
		return fmt.Errorf("research audit algorithm version must be %q", ResearchAuditAlgorithmV1)
	}
	return nil
}

func validateResearchAuditText(name, value string, required bool) error {
	if value == "" && !required {
		return nil
	}
	if err := requireText(name, value); err != nil {
		return err
	}
	if !utf8.ValidString(value) || len(value) > MaximumResearchAuditTextBytes {
		return fmt.Errorf("%s must be valid UTF-8 within %d bytes", name, MaximumResearchAuditTextBytes)
	}
	return nil
}

// SealResearchRunAuditV1 canonicalizes set-like fields and binds the record to
// a stable content hash. Query order remains meaningful and is preserved.
func SealResearchRunAuditV1(audit ResearchRunAudit) (ResearchRunAudit, error) {
	audit.AlgorithmVersion = ResearchAuditAlgorithmV1
	audit.ProvidersUsed = canonicalStrings(audit.ProvidersUsed)
	audit.Sources = append([]ResearchAuditSource(nil), audit.Sources...)
	sort.Slice(audit.Sources, func(i, j int) bool {
		if audit.Sources[i].SourceID != audit.Sources[j].SourceID {
			return audit.Sources[i].SourceID.String() < audit.Sources[j].SourceID.String()
		}
		return audit.Sources[i].SnapshotID.String() < audit.Sources[j].SnapshotID.String()
	})
	audit.AdditionalAlgorithms = append([]ResearchAuditAlgorithm(nil), audit.AdditionalAlgorithms...)
	sort.Slice(audit.AdditionalAlgorithms, func(i, j int) bool {
		if audit.AdditionalAlgorithms[i].Stage != audit.AdditionalAlgorithms[j].Stage {
			return audit.AdditionalAlgorithms[i].Stage < audit.AdditionalAlgorithms[j].Stage
		}
		return audit.AdditionalAlgorithms[i].Version < audit.AdditionalAlgorithms[j].Version
	})
	audit.Queries = append([]string(nil), audit.Queries...)
	audit.SourceCount = len(audit.Sources)
	audit.ContentHash = ""
	if err := audit.validateCore(); err != nil {
		return ResearchRunAudit{}, err
	}
	payload, err := audit.canonicalJSON(false)
	if err != nil {
		return ResearchRunAudit{}, err
	}
	audit.ContentHash = CanonicalContentHashV1(payload)
	if err := audit.Validate(); err != nil {
		return ResearchRunAudit{}, err
	}
	return audit, nil
}

type researchAuditAlgorithmJSON struct {
	Stage   string `json:"stage"`
	Version string `json:"version"`
}

type researchAuditSourceJSON struct {
	SourceID     string `json:"source_id"`
	Locator      string `json:"locator"`
	SnapshotID   string `json:"snapshot_id"`
	SnapshotHash string `json:"snapshot_hash"`
}

type researchRunAuditJSON struct {
	AuditID                 string                       `json:"audit_id"`
	RunID                   string                       `json:"run_id"`
	RecordedAt              string                       `json:"recorded_at"`
	StartedAt               string                       `json:"started_at"`
	CompletedAt             string                       `json:"completed_at,omitempty"`
	Outcome                 string                       `json:"outcome"`
	QueryPlannerVersion     string                       `json:"query_planner_version"`
	TrustPolicyVersion      string                       `json:"trust_policy_version"`
	FreshnessVersion        string                       `json:"freshness_version"`
	ConflictResolverVersion string                       `json:"conflict_resolver_version"`
	ProvidersUsed           []string                     `json:"providers_used"`
	NetworkMode             string                       `json:"network_mode"`
	NetworkAllowed          bool                         `json:"network_allowed"`
	CacheHits               int                          `json:"cache_hits"`
	SourceCount             int                          `json:"source_count"`
	BytesFetched            int64                        `json:"bytes_fetched"`
	Queries                 []string                     `json:"queries"`
	Sources                 []researchAuditSourceJSON    `json:"sources"`
	TargetTechnology        string                       `json:"target_technology,omitempty"`
	TargetVersion           string                       `json:"target_version,omitempty"`
	AdditionalAlgorithms    []researchAuditAlgorithmJSON `json:"additional_algorithms"`
	AlgorithmVersion        string                       `json:"algorithm_version"`
	ContentHash             string                       `json:"content_hash,omitempty"`
}

func (audit ResearchRunAudit) jsonPayload(includeHash bool) researchRunAuditJSON {
	payload := researchRunAuditJSON{
		AuditID: audit.ID.String(), RunID: audit.RunID.String(),
		RecordedAt: audit.RecordedAt.Time().Format(time.RFC3339Nano), StartedAt: audit.StartedAt.Time().Format(time.RFC3339Nano),
		Outcome: string(audit.Outcome), QueryPlannerVersion: audit.QueryPlannerVersion,
		TrustPolicyVersion: audit.TrustPolicyVersion, FreshnessVersion: audit.FreshnessVersion,
		ConflictResolverVersion: audit.ConflictResolverVersion,
		ProvidersUsed:           append([]string{}, audit.ProvidersUsed...), NetworkMode: string(audit.NetworkMode),
		NetworkAllowed: audit.NetworkAllowed, CacheHits: audit.CacheHits, SourceCount: audit.SourceCount,
		BytesFetched: audit.BytesFetched, Queries: append([]string(nil), audit.Queries...),
		TargetTechnology: audit.TargetTechnology, AlgorithmVersion: audit.AlgorithmVersion,
		Sources:              make([]researchAuditSourceJSON, len(audit.Sources)),
		AdditionalAlgorithms: make([]researchAuditAlgorithmJSON, len(audit.AdditionalAlgorithms)),
	}
	if audit.CompletedAt != nil {
		payload.CompletedAt = audit.CompletedAt.Time().Format(time.RFC3339Nano)
	}
	if audit.TargetVersion != nil {
		payload.TargetVersion = audit.TargetVersion.String()
	}
	for index, source := range audit.Sources {
		payload.Sources[index] = researchAuditSourceJSON{SourceID: source.SourceID.String(), Locator: source.Locator.String(), SnapshotID: source.SnapshotID.String(), SnapshotHash: source.SnapshotHash}
	}
	for index, algorithm := range audit.AdditionalAlgorithms {
		payload.AdditionalAlgorithms[index] = researchAuditAlgorithmJSON{Stage: algorithm.Stage, Version: algorithm.Version}
	}
	if includeHash {
		payload.ContentHash = audit.ContentHash
	}
	return payload
}

func (audit ResearchRunAudit) canonicalJSON(includeHash bool) ([]byte, error) {
	encoded, err := json.Marshal(audit.jsonPayload(includeHash))
	if err != nil {
		return nil, fmt.Errorf("encode research audit: %w", err)
	}
	if len(encoded) > MaximumResearchAuditJSONBytes {
		return nil, fmt.Errorf("research audit JSON exceeds %d bytes", MaximumResearchAuditJSONBytes)
	}
	return encoded, nil
}

func (audit ResearchRunAudit) ExportJSON() ([]byte, error) {
	if err := audit.Validate(); err != nil {
		return nil, err
	}
	encoded, err := json.Marshal(audit.jsonPayload(true))
	if err != nil {
		return nil, fmt.Errorf("encode research audit: %w", err)
	}
	if len(encoded) > MaximumResearchAuditJSONBytes {
		return nil, fmt.Errorf("research audit JSON exceeds %d bytes", MaximumResearchAuditJSONBytes)
	}
	return encoded, nil
}

func ParseResearchRunAuditJSON(data []byte) (ResearchRunAudit, error) {
	if len(data) == 0 || len(data) > MaximumResearchAuditJSONBytes {
		return ResearchRunAudit{}, fmt.Errorf("research audit JSON size must be between 1 and %d bytes", MaximumResearchAuditJSONBytes)
	}
	decoder := json.NewDecoder(bytes.NewReader(data))
	decoder.DisallowUnknownFields()
	var payload researchRunAuditJSON
	if err := decoder.Decode(&payload); err != nil {
		return ResearchRunAudit{}, fmt.Errorf("decode research audit: %w", err)
	}
	if err := ensureResearchAuditJSONEOF(decoder); err != nil {
		return ResearchRunAudit{}, err
	}
	audit, err := researchRunAuditFromJSON(payload)
	if err != nil {
		return ResearchRunAudit{}, err
	}
	if err := audit.Validate(); err != nil {
		return ResearchRunAudit{}, err
	}
	return audit, nil
}

func researchRunAuditFromJSON(payload researchRunAuditJSON) (ResearchRunAudit, error) {
	id, err := NewID(payload.AuditID)
	if err != nil {
		return ResearchRunAudit{}, err
	}
	runID, err := NewID(payload.RunID)
	if err != nil {
		return ResearchRunAudit{}, err
	}
	recordedAt, err := parseResearchAuditTimestamp(payload.RecordedAt)
	if err != nil {
		return ResearchRunAudit{}, fmt.Errorf("research audit recorded at: %w", err)
	}
	startedAt, err := parseResearchAuditTimestamp(payload.StartedAt)
	if err != nil {
		return ResearchRunAudit{}, fmt.Errorf("research audit started at: %w", err)
	}
	var completedAt *Timestamp
	if payload.CompletedAt != "" {
		value, parseErr := parseResearchAuditTimestamp(payload.CompletedAt)
		if parseErr != nil {
			return ResearchRunAudit{}, fmt.Errorf("research audit completed at: %w", parseErr)
		}
		completedAt = &value
	}
	var targetVersion *SourceVersion
	if payload.TargetVersion != "" {
		value, versionErr := NewSourceVersion(payload.TargetVersion)
		if versionErr != nil {
			return ResearchRunAudit{}, versionErr
		}
		targetVersion = &value
	}
	audit := ResearchRunAudit{
		ID: id, RunID: runID, RecordedAt: recordedAt, StartedAt: startedAt, CompletedAt: completedAt,
		Outcome: ResearchRunStatus(payload.Outcome), QueryPlannerVersion: payload.QueryPlannerVersion,
		TrustPolicyVersion: payload.TrustPolicyVersion, FreshnessVersion: payload.FreshnessVersion,
		ConflictResolverVersion: payload.ConflictResolverVersion,
		ProvidersUsed:           append([]string(nil), payload.ProvidersUsed...), NetworkMode: ResearchAuditNetworkMode(payload.NetworkMode),
		NetworkAllowed: payload.NetworkAllowed, CacheHits: payload.CacheHits, SourceCount: payload.SourceCount,
		BytesFetched: payload.BytesFetched, Queries: append([]string(nil), payload.Queries...),
		TargetTechnology: payload.TargetTechnology, TargetVersion: targetVersion,
		AlgorithmVersion: payload.AlgorithmVersion, ContentHash: payload.ContentHash,
		Sources:              make([]ResearchAuditSource, len(payload.Sources)),
		AdditionalAlgorithms: make([]ResearchAuditAlgorithm, len(payload.AdditionalAlgorithms)),
	}
	for index, item := range payload.Sources {
		sourceID, sourceErr := NewSourceID(item.SourceID)
		if sourceErr != nil {
			return ResearchRunAudit{}, sourceErr
		}
		locator, locatorErr := NewSourceLocator(item.Locator)
		if locatorErr != nil {
			return ResearchRunAudit{}, locatorErr
		}
		snapshotID, snapshotErr := NewID(item.SnapshotID)
		if snapshotErr != nil {
			return ResearchRunAudit{}, snapshotErr
		}
		audit.Sources[index] = ResearchAuditSource{SourceID: sourceID, Locator: locator, SnapshotID: snapshotID, SnapshotHash: item.SnapshotHash}
	}
	for index, item := range payload.AdditionalAlgorithms {
		audit.AdditionalAlgorithms[index] = ResearchAuditAlgorithm{Stage: item.Stage, Version: item.Version}
	}
	return audit, nil
}

func parseResearchAuditTimestamp(value string) (Timestamp, error) {
	parsed, err := time.Parse(time.RFC3339Nano, value)
	if err != nil {
		return Timestamp{}, err
	}
	return NewTimestamp(parsed)
}

func ensureResearchAuditJSONEOF(decoder *json.Decoder) error {
	var extra any
	if err := decoder.Decode(&extra); err == io.EOF {
		return nil
	} else if err != nil {
		return fmt.Errorf("decode research audit trailing data: %w", err)
	}
	return fmt.Errorf("research audit JSON contains trailing data")
}

// ResearchAuditInternetDisclaimer is shared by human-facing audit views.
const ResearchAuditInternetDisclaimer = "Stored metadata can reproduce the run inputs and decisions; it cannot guarantee that the future Internet will return the same content."
