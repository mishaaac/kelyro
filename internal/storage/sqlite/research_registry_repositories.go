package sqlite

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"

	"github.com/mishaaac/kelyro/internal/research"
	"github.com/mishaaac/kelyro/internal/research/application"
)

func newResearchRepositories(target executor, timeout time.Duration) application.Repositories {
	return application.Repositories{
		Sources:        &researchSourceRepository{target, timeout},
		Snapshots:      &researchSnapshotRepository{target, timeout},
		Evidence:       &researchEvidenceRepository{target, timeout},
		Citations:      &researchCitationRepository{target, timeout},
		Provenance:     &researchProvenanceRepository{target, timeout},
		Runs:           &researchRunRepository{target, timeout},
		TrustRegistry:  &researchTrustRegistryRepository{target, timeout},
		SourceRegistry: &researchSourceRegistryRepository{target, timeout},
		Releases:       &researchReleaseRepository{target, timeout},
		Freshness:      &researchFreshnessRepository{target, timeout},
		Verification:   &researchVerificationRepository{target, timeout},
		Drift:          &researchDriftRepository{target, timeout},
		Impact:         &researchImpactRepository{target, timeout},
		Cache:          &researchCacheRepository{target, timeout},
	}
}

var (
	_ application.SourceRepository         = (*researchSourceRepository)(nil)
	_ application.SnapshotRepository       = (*researchSnapshotRepository)(nil)
	_ application.EvidenceRepository       = (*researchEvidenceRepository)(nil)
	_ application.CitationRepository       = (*researchCitationRepository)(nil)
	_ application.ProvenanceRepository     = (*researchProvenanceRepository)(nil)
	_ application.ResearchRunRepository    = (*researchRunRepository)(nil)
	_ application.TrustRegistryRepository  = (*researchTrustRegistryRepository)(nil)
	_ application.SourceRegistryRepository = (*researchSourceRegistryRepository)(nil)
	_ application.ReleaseRepository        = (*researchReleaseRepository)(nil)
	_ application.FreshnessRepository      = (*researchFreshnessRepository)(nil)
	_ application.VerificationRepository   = (*researchVerificationRepository)(nil)
	_ application.DriftRepository          = (*researchDriftRepository)(nil)
	_ application.ImpactRepository         = (*researchImpactRepository)(nil)
	_ application.ResearchCacheRepository  = (*researchCacheRepository)(nil)
)

type researchRunRepository struct {
	executor executor
	timeout  time.Duration
}

func (repository *researchRunRepository) Create(ctx context.Context, request research.ResearchRequest, run research.ResearchRun) error {
	const operation = "create SQLite research run"
	if err := request.Validate(); err != nil {
		return researchInvalid(operation, err)
	}
	if err := run.Validate(); err != nil {
		return researchInvalid(operation, err)
	}
	if run.RequestID != request.ID {
		return researchInvalid(operation, errors.New("run request does not match"))
	}
	opCtx, cancel, err := researchOperationContext(ctx, repository.timeout, operation)
	if err != nil {
		return err
	}
	defer cancel()
	stored, getErr := scanResearchRequest(repository.executor.QueryRowContext(opCtx, `SELECT request_id,subject,domain,technology,purpose,target_version,requested_at FROM research_topics WHERE request_id=?`, request.ID.String()), operation)
	requestExists := getErr == nil
	if getErr != nil && !errors.Is(getErr, application.ErrNotFound) {
		return getErr
	}
	if requestExists && !sameResearchRequest(stored, request) {
		return researchConflict(operation)
	}
	runExists, err := recordExists(opCtx, repository.executor, "research_runs", "id", run.ID.String())
	if err != nil {
		return researchPersistence(operation, err)
	}
	if runExists {
		return researchConflict(operation)
	}
	createdRequest := false
	if !requestExists {
		_, err = repository.executor.ExecContext(opCtx, `INSERT INTO research_topics (request_id,subject,domain,technology,purpose,target_version,requested_at) VALUES (?,?,?,?,?,?,?)`, request.ID.String(), request.Topic.Subject, request.Topic.Domain, request.Topic.Technology, string(request.Purpose), optionalVersionText(request.TargetVersion), timestampText(request.RequestedAt))
		if err != nil {
			return researchPersistence(operation, err)
		}
		createdRequest = true
	}
	_, err = repository.executor.ExecContext(opCtx, `INSERT INTO research_runs (id,request_id,status,started_at,completed_at) VALUES (?,?,?,?,?)`, run.ID.String(), run.RequestID.String(), string(run.Status), timestampText(run.StartedAt), optionalTimestampText(run.CompletedAt))
	if err != nil {
		if createdRequest {
			_, _ = repository.executor.ExecContext(opCtx, `DELETE FROM research_topics WHERE request_id=? AND NOT EXISTS (SELECT 1 FROM research_runs WHERE request_id=?)`, request.ID.String(), request.ID.String())
		}
		return researchPersistence(operation, err)
	}
	return nil
}

func (repository *researchRunRepository) GetRequest(ctx context.Context, id research.ID) (research.ResearchRequest, error) {
	const operation = "get SQLite research request"
	if err := id.Validate(); err != nil {
		return research.ResearchRequest{}, researchInvalid(operation, err)
	}
	opCtx, cancel, err := researchOperationContext(ctx, repository.timeout, operation)
	if err != nil {
		return research.ResearchRequest{}, err
	}
	defer cancel()
	return scanResearchRequest(repository.executor.QueryRowContext(opCtx, `SELECT request_id,subject,domain,technology,purpose,target_version,requested_at FROM research_topics WHERE request_id=?`, id.String()), operation)
}
func scanResearchRequest(row rowScanner, operation string) (research.ResearchRequest, error) {
	var idValue, subject, domain, technology, purpose, requested string
	var target sql.NullString
	if err := row.Scan(&idValue, &subject, &domain, &technology, &purpose, &target, &requested); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return research.ResearchRequest{}, researchNotFound(operation)
		}
		return research.ResearchRequest{}, researchPersistence(operation, err)
	}
	id, err := research.NewID(idValue)
	if err != nil {
		return research.ResearchRequest{}, researchPersistence(operation, err)
	}
	topic, err := research.NewResearchTopic(subject, domain, technology)
	if err != nil {
		return research.ResearchRequest{}, researchPersistence(operation, err)
	}
	targetVersion, err := scanOptionalVersion(target)
	if err != nil {
		return research.ResearchRequest{}, researchPersistence(operation, err)
	}
	requestedAt, err := scanTimestamp(requested)
	if err != nil {
		return research.ResearchRequest{}, researchPersistence(operation, err)
	}
	item := research.ResearchRequest{ID: id, Topic: topic, Purpose: research.ResearchPurpose(purpose), TargetVersion: targetVersion, RequestedAt: requestedAt}
	if err := item.Validate(); err != nil {
		return research.ResearchRequest{}, researchPersistence(operation, err)
	}
	return item, nil
}

func (repository *researchRunRepository) GetRun(ctx context.Context, id research.ID) (research.ResearchRun, error) {
	const operation = "get SQLite research run"
	if err := id.Validate(); err != nil {
		return research.ResearchRun{}, researchInvalid(operation, err)
	}
	opCtx, cancel, err := researchOperationContext(ctx, repository.timeout, operation)
	if err != nil {
		return research.ResearchRun{}, err
	}
	defer cancel()
	return scanResearchRun(repository.executor.QueryRowContext(opCtx, `SELECT id,request_id,status,started_at,completed_at FROM research_runs WHERE id=?`, id.String()), operation)
}
func scanResearchRun(row rowScanner, operation string) (research.ResearchRun, error) {
	var idValue, requestValue, status, started string
	var completed sql.NullString
	if err := row.Scan(&idValue, &requestValue, &status, &started, &completed); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return research.ResearchRun{}, researchNotFound(operation)
		}
		return research.ResearchRun{}, researchPersistence(operation, err)
	}
	id, err := research.NewID(idValue)
	if err != nil {
		return research.ResearchRun{}, researchPersistence(operation, err)
	}
	requestID, err := research.NewID(requestValue)
	if err != nil {
		return research.ResearchRun{}, researchPersistence(operation, err)
	}
	startedAt, err := scanTimestamp(started)
	if err != nil {
		return research.ResearchRun{}, researchPersistence(operation, err)
	}
	completedAt, err := scanOptionalTimestamp(completed)
	if err != nil {
		return research.ResearchRun{}, researchPersistence(operation, err)
	}
	item := research.ResearchRun{ID: id, RequestID: requestID, Status: research.ResearchRunStatus(status), StartedAt: startedAt, CompletedAt: completedAt}
	if err := item.Validate(); err != nil {
		return research.ResearchRun{}, researchPersistence(operation, err)
	}
	return item, nil
}
func (repository *researchRunRepository) UpdateRun(ctx context.Context, run research.ResearchRun) error {
	const operation = "update SQLite research run"
	if err := run.Validate(); err != nil {
		return researchInvalid(operation, err)
	}
	opCtx, cancel, err := researchOperationContext(ctx, repository.timeout, operation)
	if err != nil {
		return err
	}
	defer cancel()
	var requestID string
	err = repository.executor.QueryRowContext(opCtx, `SELECT request_id FROM research_runs WHERE id=?`, run.ID.String()).Scan(&requestID)
	if errors.Is(err, sql.ErrNoRows) {
		return researchNotFound(operation)
	}
	if err != nil {
		return researchPersistence(operation, err)
	}
	if requestID != run.RequestID.String() {
		return researchInvalid(operation, errors.New("run request identity cannot change"))
	}
	_, err = repository.executor.ExecContext(opCtx, `UPDATE research_runs SET status=?,started_at=?,completed_at=? WHERE id=?`, string(run.Status), timestampText(run.StartedAt), optionalTimestampText(run.CompletedAt), run.ID.String())
	if err != nil {
		return researchPersistence(operation, err)
	}
	return nil
}

func sameResearchRequest(left, right research.ResearchRequest) bool {
	if left.ID != right.ID || left.Topic != right.Topic || left.Purpose != right.Purpose || !left.RequestedAt.Time().Equal(right.RequestedAt.Time()) {
		return false
	}
	if left.TargetVersion == nil || right.TargetVersion == nil {
		return left.TargetVersion == nil && right.TargetVersion == nil
	}
	return *left.TargetVersion == *right.TargetVersion
}

type researchTrustRegistryRepository struct {
	executor executor
	timeout  time.Duration
}

type freshnessTTLHintJSON struct {
	ClaimType  string `json:"claim_type,omitempty"`
	SourceKind string `json:"source_kind,omitempty"`
	TTLDays    int    `json:"ttl_days"`
}

func (repository *researchTrustRegistryRepository) SaveProfile(ctx context.Context, profile research.AuthorityProfile) error {
	const operation = "save SQLite authority profile"
	if err := profile.Validate(); err != nil {
		return researchInvalid(operation, err)
	}
	preferredKinds, err := encodeJSON(operation, sourceKindStrings(profile.PreferredKinds))
	if err != nil {
		return err
	}
	preferredDomains, err := encodeJSON(operation, append([]string{}, profile.PreferredDomains...))
	if err != nil {
		return err
	}
	preferredOrganizations, err := encodeJSON(operation, append([]string{}, profile.PreferredOrganizations...))
	if err != nil {
		return err
	}
	supplementaryKinds, err := encodeJSON(operation, sourceKindStrings(profile.AllowedSupplementaryKinds))
	if err != nil {
		return err
	}
	freshnessTTLHints, err := encodeJSON(operation, encodeFreshnessTTLHints(profile.FreshnessTTLHints))
	if err != nil {
		return err
	}
	opCtx, cancel, err := researchOperationContext(ctx, repository.timeout, operation)
	if err != nil {
		return err
	}
	defer cancel()
	_, err = repository.executor.ExecContext(opCtx, `INSERT INTO authority_profiles (id,version,domain,topic_pattern,preferred_kinds_json,preferred_domains_json,preferred_organizations_json,minimum_corroboration,supplementary_kinds_json,freshness_ttl_hints_json,minimum_tier,created_at) VALUES (?,?,?,?,?,?,?,?,?,?,?,?) ON CONFLICT(id) DO UPDATE SET version=excluded.version,domain=excluded.domain,topic_pattern=excluded.topic_pattern,preferred_kinds_json=excluded.preferred_kinds_json,preferred_domains_json=excluded.preferred_domains_json,preferred_organizations_json=excluded.preferred_organizations_json,minimum_corroboration=excluded.minimum_corroboration,supplementary_kinds_json=excluded.supplementary_kinds_json,freshness_ttl_hints_json=excluded.freshness_ttl_hints_json,minimum_tier=excluded.minimum_tier,created_at=excluded.created_at`, profile.ID.String(), profile.Version, profile.Domain, profile.TopicPattern, preferredKinds, preferredDomains, preferredOrganizations, profile.MinimumCorroboration, supplementaryKinds, freshnessTTLHints, string(profile.MinimumTier), timestampText(profile.CreatedAt))
	if err != nil {
		return researchPersistence(operation, err)
	}
	return nil
}
func (repository *researchTrustRegistryRepository) GetProfile(ctx context.Context, id research.ID) (research.AuthorityProfile, error) {
	const operation = "get SQLite authority profile"
	if err := id.Validate(); err != nil {
		return research.AuthorityProfile{}, researchInvalid(operation, err)
	}
	opCtx, cancel, err := researchOperationContext(ctx, repository.timeout, operation)
	if err != nil {
		return research.AuthorityProfile{}, err
	}
	defer cancel()
	return scanAuthorityProfile(repository.executor.QueryRowContext(opCtx, authorityProfileSelect+` WHERE id=?`, id.String()), operation)
}
func (repository *researchTrustRegistryRepository) ListProfiles(ctx context.Context) ([]research.AuthorityProfile, error) {
	const operation = "list SQLite authority profiles"
	opCtx, cancel, err := researchOperationContext(ctx, repository.timeout, operation)
	if err != nil {
		return nil, err
	}
	defer cancel()
	rows, err := repository.executor.QueryContext(opCtx, authorityProfileSelect+` ORDER BY id`)
	if err != nil {
		return nil, researchPersistence(operation, err)
	}
	defer rows.Close()
	result := make([]research.AuthorityProfile, 0)
	for rows.Next() {
		item, scanErr := scanAuthorityProfile(rows, operation)
		if scanErr != nil {
			return nil, scanErr
		}
		result = append(result, item)
	}
	if err := rows.Err(); err != nil {
		return nil, researchPersistence(operation, err)
	}
	return result, nil
}
func scanAuthorityProfile(row rowScanner, operation string) (research.AuthorityProfile, error) {
	var idValue, version, domain, pattern, kindsJSON, domainsJSON, organizationsJSON, supplementaryJSON, freshnessTTLJSON, tier, created string
	var minimumCorroboration int
	if err := row.Scan(&idValue, &version, &domain, &pattern, &kindsJSON, &domainsJSON, &organizationsJSON, &minimumCorroboration, &supplementaryJSON, &freshnessTTLJSON, &tier, &created); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return research.AuthorityProfile{}, researchNotFound(operation)
		}
		return research.AuthorityProfile{}, researchPersistence(operation, err)
	}
	id, err := research.NewID(idValue)
	if err != nil {
		return research.AuthorityProfile{}, researchPersistence(operation, err)
	}
	var kindValues []string
	if err := decodeJSON(kindsJSON, &kindValues); err != nil {
		return research.AuthorityProfile{}, researchPersistence(operation, err)
	}
	kinds := make([]research.SourceKind, len(kindValues))
	for i, value := range kindValues {
		kinds[i] = research.SourceKind(value)
	}
	var preferredDomains, preferredOrganizations, supplementaryValues []string
	if err := decodeJSON(domainsJSON, &preferredDomains); err != nil {
		return research.AuthorityProfile{}, researchPersistence(operation, err)
	}
	if err := decodeJSON(organizationsJSON, &preferredOrganizations); err != nil {
		return research.AuthorityProfile{}, researchPersistence(operation, err)
	}
	if err := decodeJSON(supplementaryJSON, &supplementaryValues); err != nil {
		return research.AuthorityProfile{}, researchPersistence(operation, err)
	}
	supplementaryKinds := make([]research.SourceKind, len(supplementaryValues))
	for i, value := range supplementaryValues {
		supplementaryKinds[i] = research.SourceKind(value)
	}
	var freshnessTTLValues []freshnessTTLHintJSON
	if err := decodeJSON(freshnessTTLJSON, &freshnessTTLValues); err != nil {
		return research.AuthorityProfile{}, researchPersistence(operation, err)
	}
	freshnessTTLHints := decodeFreshnessTTLHints(freshnessTTLValues)
	createdAt, err := scanTimestamp(created)
	if err != nil {
		return research.AuthorityProfile{}, researchPersistence(operation, err)
	}
	item := research.AuthorityProfile{ID: id, Version: version, Domain: domain, TopicPattern: pattern, PreferredKinds: kinds, PreferredDomains: preferredDomains, PreferredOrganizations: preferredOrganizations, MinimumCorroboration: minimumCorroboration, AllowedSupplementaryKinds: supplementaryKinds, FreshnessTTLHints: freshnessTTLHints, MinimumTier: research.AuthorityTier(tier), CreatedAt: createdAt}
	if err := item.Validate(); err != nil {
		return research.AuthorityProfile{}, researchPersistence(operation, err)
	}
	return item, nil
}

const authorityProfileSelect = `SELECT id,version,domain,topic_pattern,preferred_kinds_json,preferred_domains_json,preferred_organizations_json,minimum_corroboration,supplementary_kinds_json,freshness_ttl_hints_json,minimum_tier,created_at FROM authority_profiles`

func encodeFreshnessTTLHints(hints []research.FreshnessTTLHint) []freshnessTTLHintJSON {
	result := make([]freshnessTTLHintJSON, len(hints))
	for index, hint := range hints {
		result[index].TTLDays = hint.TTLDays
		if hint.ClaimType != nil {
			result[index].ClaimType = string(*hint.ClaimType)
		}
		if hint.SourceKind != nil {
			result[index].SourceKind = string(*hint.SourceKind)
		}
	}
	return result
}

func decodeFreshnessTTLHints(values []freshnessTTLHintJSON) []research.FreshnessTTLHint {
	if len(values) == 0 {
		return nil
	}
	result := make([]research.FreshnessTTLHint, len(values))
	for index, value := range values {
		result[index].TTLDays = value.TTLDays
		if value.ClaimType != "" {
			claimType := research.ClaimType(value.ClaimType)
			result[index].ClaimType = &claimType
		}
		if value.SourceKind != "" {
			sourceKind := research.SourceKind(value.SourceKind)
			result[index].SourceKind = &sourceKind
		}
	}
	return result
}

func sourceKindStrings(kinds []research.SourceKind) []string {
	values := make([]string, len(kinds))
	for index, kind := range kinds {
		values[index] = string(kind)
	}
	return values
}

func (repository *researchTrustRegistryRepository) SaveDecision(ctx context.Context, decision research.TrustDecision) error {
	const operation = "save SQLite trust decision"
	if err := decision.Validate(); err != nil {
		return researchInvalid(operation, err)
	}
	encoded, err := encodeJSON(operation, decision.Reasons)
	if err != nil {
		return err
	}
	opCtx, cancel, err := researchOperationContext(ctx, repository.timeout, operation)
	if err != nil {
		return err
	}
	defer cancel()
	exists, err := recordExists(opCtx, repository.executor, "sources", "id", decision.SourceID.String())
	if err != nil {
		return researchPersistence(operation, err)
	}
	if !exists {
		return researchNotFound(operation)
	}
	_, err = repository.executor.ExecContext(opCtx, `INSERT INTO trust_registry (source_id,state,tier,reasons_json,policy_version,evaluated_at) VALUES (?,?,?,?,?,?)`, decision.SourceID.String(), string(decision.State), string(decision.Tier), encoded, decision.Policy, timestampText(decision.EvaluatedAt))
	if err != nil {
		return researchPersistence(operation, err)
	}
	return nil
}
func (repository *researchTrustRegistryRepository) LatestDecision(ctx context.Context, id research.SourceID) (research.TrustDecision, error) {
	const operation = "get latest SQLite trust decision"
	if err := id.Validate(); err != nil {
		return research.TrustDecision{}, researchInvalid(operation, err)
	}
	opCtx, cancel, err := researchOperationContext(ctx, repository.timeout, operation)
	if err != nil {
		return research.TrustDecision{}, err
	}
	defer cancel()
	var sourceValue, state, tier, reasonsJSON, policy, evaluated string
	err = repository.executor.QueryRowContext(opCtx, `SELECT source_id,state,tier,reasons_json,policy_version,evaluated_at FROM trust_registry WHERE source_id=? ORDER BY evaluated_at DESC,id DESC LIMIT 1`, id.String()).Scan(&sourceValue, &state, &tier, &reasonsJSON, &policy, &evaluated)
	if errors.Is(err, sql.ErrNoRows) {
		return research.TrustDecision{}, researchNotFound(operation)
	}
	if err != nil {
		return research.TrustDecision{}, researchPersistence(operation, err)
	}
	sourceID, err := research.NewSourceID(sourceValue)
	if err != nil {
		return research.TrustDecision{}, researchPersistence(operation, err)
	}
	var reasons []research.TrustReason
	if err := decodeJSON(reasonsJSON, &reasons); err != nil {
		return research.TrustDecision{}, researchPersistence(operation, err)
	}
	evaluatedAt, err := scanTimestamp(evaluated)
	if err != nil {
		return research.TrustDecision{}, researchPersistence(operation, err)
	}
	item := research.TrustDecision{SourceID: sourceID, State: research.TrustDecisionState(state), Tier: research.AuthorityTier(tier), Reasons: reasons, Policy: policy, EvaluatedAt: evaluatedAt}
	if err := item.Validate(); err != nil {
		return research.TrustDecision{}, researchPersistence(operation, err)
	}
	return item, nil
}

type researchReleaseRepository struct {
	executor executor
	timeout  time.Duration
}

func (repository *researchReleaseRepository) Create(ctx context.Context, record research.ReleaseRecord) error {
	const operation = "create SQLite release record"
	if err := record.Validate(); err != nil {
		return researchInvalid(operation, err)
	}
	encoded, err := encodeJSON(operation, sourceIDStrings(record.SourceIDs))
	if err != nil {
		return err
	}
	opCtx, cancel, err := researchOperationContext(ctx, repository.timeout, operation)
	if err != nil {
		return err
	}
	defer cancel()
	exists, err := recordExists(opCtx, repository.executor, "release_records", "id", record.ID.String())
	if err != nil {
		return researchPersistence(operation, err)
	}
	if exists {
		return researchConflict(operation)
	}
	for _, sourceID := range record.SourceIDs {
		exists, err = recordExists(opCtx, repository.executor, "sources", "id", sourceID.String())
		if err != nil {
			return researchPersistence(operation, err)
		}
		if !exists {
			return researchNotFound(operation)
		}
	}
	_, err = repository.executor.ExecContext(opCtx, `INSERT INTO release_records (id,technology_id,version,channel,status,source_ids_json,released_at,verified_at) VALUES (?,?,?,?,?,?,?,?)`, record.ID.String(), record.TechnologyID.String(), record.Version.String(), string(record.Channel), string(record.Status), encoded, optionalTimestampText(record.ReleasedAt), timestampText(record.VerifiedAt))
	if err != nil {
		return researchPersistence(operation, err)
	}
	return nil
}
func (repository *researchReleaseRepository) Get(ctx context.Context, id research.ID) (research.ReleaseRecord, error) {
	return repository.get(ctx, "get SQLite release record", `WHERE id=?`, id.String(), id.Validate())
}
func (repository *researchReleaseRepository) ListByTechnology(ctx context.Context, id research.ID) ([]research.ReleaseRecord, error) {
	const operation = "list SQLite release records"
	if err := id.Validate(); err != nil {
		return nil, researchInvalid(operation, err)
	}
	opCtx, cancel, err := researchOperationContext(ctx, repository.timeout, operation)
	if err != nil {
		return nil, err
	}
	defer cancel()
	rows, err := repository.executor.QueryContext(opCtx, `SELECT id,technology_id,version,channel,status,source_ids_json,released_at,verified_at FROM release_records WHERE technology_id=? ORDER BY verified_at,id`, id.String())
	if err != nil {
		return nil, researchPersistence(operation, err)
	}
	defer rows.Close()
	result := make([]research.ReleaseRecord, 0)
	for rows.Next() {
		item, scanErr := scanRelease(rows, operation)
		if scanErr != nil {
			return nil, scanErr
		}
		result = append(result, item)
	}
	if err := rows.Err(); err != nil {
		return nil, researchPersistence(operation, err)
	}
	return result, nil
}
func (repository *researchReleaseRepository) get(ctx context.Context, operation, suffix, value string, validation error) (research.ReleaseRecord, error) {
	if validation != nil {
		return research.ReleaseRecord{}, researchInvalid(operation, validation)
	}
	opCtx, cancel, err := researchOperationContext(ctx, repository.timeout, operation)
	if err != nil {
		return research.ReleaseRecord{}, err
	}
	defer cancel()
	return scanRelease(repository.executor.QueryRowContext(opCtx, `SELECT id,technology_id,version,channel,status,source_ids_json,released_at,verified_at FROM release_records `+suffix, value), operation)
}
func scanRelease(row rowScanner, operation string) (research.ReleaseRecord, error) {
	var idValue, technologyValue, versionValue, channel, status, sourcesJSON, verified string
	var released sql.NullString
	if err := row.Scan(&idValue, &technologyValue, &versionValue, &channel, &status, &sourcesJSON, &released, &verified); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return research.ReleaseRecord{}, researchNotFound(operation)
		}
		return research.ReleaseRecord{}, researchPersistence(operation, err)
	}
	id, err := research.NewID(idValue)
	if err != nil {
		return research.ReleaseRecord{}, researchPersistence(operation, err)
	}
	technologyID, err := research.NewID(technologyValue)
	if err != nil {
		return research.ReleaseRecord{}, researchPersistence(operation, err)
	}
	version, err := research.NewSourceVersion(versionValue)
	if err != nil {
		return research.ReleaseRecord{}, researchPersistence(operation, err)
	}
	var sourceValues []string
	if err := decodeJSON(sourcesJSON, &sourceValues); err != nil {
		return research.ReleaseRecord{}, researchPersistence(operation, err)
	}
	sources, err := parseSourceIDs(sourceValues)
	if err != nil {
		return research.ReleaseRecord{}, researchPersistence(operation, err)
	}
	releasedAt, err := scanOptionalTimestamp(released)
	if err != nil {
		return research.ReleaseRecord{}, researchPersistence(operation, err)
	}
	verifiedAt, err := scanTimestamp(verified)
	if err != nil {
		return research.ReleaseRecord{}, researchPersistence(operation, err)
	}
	item := research.ReleaseRecord{ID: id, TechnologyID: technologyID, Version: version, Channel: research.ReleaseChannel(channel), Status: research.ReleaseStatus(status), SourceIDs: sources, ReleasedAt: releasedAt, VerifiedAt: verifiedAt}
	if err := item.Validate(); err != nil {
		return research.ReleaseRecord{}, researchPersistence(operation, fmt.Errorf("invalid stored release: %w", err))
	}
	return item, nil
}
