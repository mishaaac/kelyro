package sqlite

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/mishaaac/kelyro/internal/research"
	"github.com/mishaaac/kelyro/internal/research/application"
)

type researchFreshnessRepository struct {
	executor executor
	timeout  time.Duration
}

func (repository *researchFreshnessRepository) Save(ctx context.Context, record application.FreshnessRecord) error {
	const operation = "save SQLite freshness state"
	if err := record.Validate(); err != nil {
		return researchInvalid(operation, err)
	}
	schedulingJSON, err := encodeRefreshScheduling(record)
	if err != nil {
		return researchInvalid(operation, err)
	}
	opCtx, cancel, err := researchOperationContext(ctx, repository.timeout, operation)
	if err != nil {
		return err
	}
	defer cancel()
	_, err = repository.executor.ExecContext(opCtx, `INSERT INTO freshness_state (subject_id,state,score,last_verified_at,next_verify_at,algorithm_version,scheduling_json) VALUES (?,?,?,?,?,?,?) ON CONFLICT(subject_id) DO UPDATE SET state=excluded.state,score=excluded.score,last_verified_at=excluded.last_verified_at,next_verify_at=excluded.next_verify_at,algorithm_version=excluded.algorithm_version,scheduling_json=excluded.scheduling_json`, record.SubjectID.String(), string(record.State), record.Score.Value(), timestampText(record.LastVerifiedAt), optionalTimestampText(record.NextVerifyAt), record.AlgorithmVersion, schedulingJSON)
	if err != nil {
		return researchPersistence(operation, err)
	}
	return nil
}
func (repository *researchFreshnessRepository) Get(ctx context.Context, id research.ID) (application.FreshnessRecord, error) {
	const operation = "get SQLite freshness state"
	if err := id.Validate(); err != nil {
		return application.FreshnessRecord{}, researchInvalid(operation, err)
	}
	opCtx, cancel, err := researchOperationContext(ctx, repository.timeout, operation)
	if err != nil {
		return application.FreshnessRecord{}, err
	}
	defer cancel()
	return scanFreshness(repository.executor.QueryRowContext(opCtx, `SELECT subject_id,state,score,last_verified_at,next_verify_at,algorithm_version,scheduling_json FROM freshness_state WHERE subject_id=?`, id.String()), operation)
}
func (repository *researchFreshnessRepository) ListDue(ctx context.Context, asOf research.Timestamp) ([]application.FreshnessRecord, error) {
	const operation = "list SQLite freshness due"
	if err := asOf.Validate(); err != nil {
		return nil, researchInvalid(operation, err)
	}
	opCtx, cancel, err := researchOperationContext(ctx, repository.timeout, operation)
	if err != nil {
		return nil, err
	}
	defer cancel()
	rows, err := repository.executor.QueryContext(opCtx, `SELECT subject_id,state,score,last_verified_at,next_verify_at,algorithm_version,scheduling_json FROM freshness_state WHERE next_verify_at IS NOT NULL AND next_verify_at<=? ORDER BY CASE json_extract(scheduling_json,'$.priority') WHEN 'critical' THEN 0 WHEN 'high' THEN 1 ELSE 2 END,next_verify_at,subject_id`, timestampText(asOf))
	if err != nil {
		return nil, researchPersistence(operation, err)
	}
	defer rows.Close()
	result := make([]application.FreshnessRecord, 0)
	for rows.Next() {
		item, scanErr := scanFreshness(rows, operation)
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
func scanFreshness(row rowScanner, operation string) (application.FreshnessRecord, error) {
	var subjectValue, state, lastVerified, algorithm string
	var score float64
	var next sql.NullString
	var schedulingJSON string
	if err := row.Scan(&subjectValue, &state, &score, &lastVerified, &next, &algorithm, &schedulingJSON); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return application.FreshnessRecord{}, researchNotFound(operation)
		}
		return application.FreshnessRecord{}, researchPersistence(operation, err)
	}
	subjectID, err := research.NewID(subjectValue)
	if err != nil {
		return application.FreshnessRecord{}, researchPersistence(operation, err)
	}
	freshnessScore, err := research.NewFreshnessScore(score)
	if err != nil {
		return application.FreshnessRecord{}, researchPersistence(operation, err)
	}
	lastVerifiedAt, err := scanTimestamp(lastVerified)
	if err != nil {
		return application.FreshnessRecord{}, researchPersistence(operation, err)
	}
	nextVerifyAt, err := scanOptionalTimestamp(next)
	if err != nil {
		return application.FreshnessRecord{}, researchPersistence(operation, err)
	}
	scheduling, err := decodeRefreshScheduling(schedulingJSON)
	if err != nil {
		return application.FreshnessRecord{}, researchPersistence(operation, err)
	}
	item := application.FreshnessRecord{
		SubjectID: subjectID, State: research.FreshnessState(state), Score: freshnessScore,
		LastVerifiedAt: lastVerifiedAt, NextVerifyAt: nextVerifyAt,
		AlgorithmVersion: algorithm,
	}
	if nextVerifyAt != nil {
		item.VerificationReason = scheduling.VerificationReason
		item.Priority = scheduling.Priority
		item.SchedulingAlgorithmVersion = scheduling.AlgorithmVersion
	}
	if err := item.Validate(); err != nil {
		return application.FreshnessRecord{}, researchPersistence(operation, err)
	}
	return item, nil
}

type refreshSchedulingJSON struct {
	VerificationReason research.VerificationReason   `json:"verification_reason"`
	Priority           research.VerificationPriority `json:"priority"`
	AlgorithmVersion   string                        `json:"algorithm_version"`
}

func encodeRefreshScheduling(record application.FreshnessRecord) (string, error) {
	metadata := refreshSchedulingJSON{
		VerificationReason: research.VerificationTTLExpired,
		Priority:           research.VerificationPriorityNormal,
		AlgorithmVersion:   research.RefreshSchedulingAlgorithmV1,
	}
	if record.NextVerifyAt != nil {
		metadata.VerificationReason = record.VerificationReason
		metadata.Priority = record.Priority
		metadata.AlgorithmVersion = record.SchedulingAlgorithmVersion
	}
	encoded, err := json.Marshal(metadata)
	if err != nil {
		return "", fmt.Errorf("encode refresh schedule: %w", err)
	}
	return string(encoded), nil
}

func decodeRefreshScheduling(encoded string) (refreshSchedulingJSON, error) {
	var metadata refreshSchedulingJSON
	if err := json.Unmarshal([]byte(encoded), &metadata); err != nil {
		return refreshSchedulingJSON{}, fmt.Errorf("decode refresh schedule: %w", err)
	}
	if err := metadata.VerificationReason.Validate(); err != nil {
		return refreshSchedulingJSON{}, err
	}
	if err := metadata.Priority.Validate(); err != nil {
		return refreshSchedulingJSON{}, err
	}
	if metadata.AlgorithmVersion != research.RefreshSchedulingAlgorithmV1 {
		return refreshSchedulingJSON{}, fmt.Errorf("invalid refresh scheduling algorithm %q", metadata.AlgorithmVersion)
	}
	return metadata, nil
}

type researchVerificationRepository struct {
	executor executor
	timeout  time.Duration
}

func (repository *researchVerificationRepository) Append(ctx context.Context, result research.VerificationResult) error {
	const operation = "append SQLite verification result"
	if err := result.Validate(); err != nil {
		return researchInvalid(operation, err)
	}
	encoded, err := encodeJSON(operation, sourceIDStrings(result.SourceIDs))
	if err != nil {
		return err
	}
	opCtx, cancel, err := researchOperationContext(ctx, repository.timeout, operation)
	if err != nil {
		return err
	}
	defer cancel()
	exists, err := recordExists(opCtx, repository.executor, "verification_results", "id", result.ID.String())
	if err != nil {
		return researchPersistence(operation, err)
	}
	if exists {
		return researchConflict(operation)
	}
	for _, sourceID := range result.SourceIDs {
		exists, err = recordExists(opCtx, repository.executor, "sources", "id", sourceID.String())
		if err != nil {
			return researchPersistence(operation, err)
		}
		if !exists {
			return researchNotFound(operation)
		}
	}
	_, err = repository.executor.ExecContext(opCtx, `INSERT INTO verification_results (id,claim_id,status,source_ids_json,confidence,verified_at) VALUES (?,?,?,?,?,?)`, result.ID.String(), result.ClaimID.String(), string(result.Status), encoded, result.Confidence.Value(), timestampText(result.VerifiedAt))
	if err != nil {
		return researchPersistence(operation, err)
	}
	return nil
}
func (repository *researchVerificationRepository) Get(ctx context.Context, id research.ID) (research.VerificationResult, error) {
	return repository.get(ctx, "get SQLite verification result", `WHERE id=?`, id.String(), id.Validate())
}
func (repository *researchVerificationRepository) LatestByClaim(ctx context.Context, id research.ClaimID) (research.VerificationResult, error) {
	return repository.get(ctx, "get latest SQLite verification result", `WHERE claim_id=? ORDER BY verified_at DESC,id DESC LIMIT 1`, id.String(), id.Validate())
}
func (repository *researchVerificationRepository) get(ctx context.Context, operation, suffix, value string, validation error) (research.VerificationResult, error) {
	if validation != nil {
		return research.VerificationResult{}, researchInvalid(operation, validation)
	}
	opCtx, cancel, err := researchOperationContext(ctx, repository.timeout, operation)
	if err != nil {
		return research.VerificationResult{}, err
	}
	defer cancel()
	return scanVerification(repository.executor.QueryRowContext(opCtx, `SELECT id,claim_id,status,source_ids_json,confidence,verified_at FROM verification_results `+suffix, value), operation)
}
func scanVerification(row rowScanner, operation string) (research.VerificationResult, error) {
	var idValue, claimValue, status, sourcesJSON, verified string
	var confidence float64
	if err := row.Scan(&idValue, &claimValue, &status, &sourcesJSON, &confidence, &verified); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return research.VerificationResult{}, researchNotFound(operation)
		}
		return research.VerificationResult{}, researchPersistence(operation, err)
	}
	id, err := research.NewID(idValue)
	if err != nil {
		return research.VerificationResult{}, researchPersistence(operation, err)
	}
	claimID, err := research.NewClaimID(claimValue)
	if err != nil {
		return research.VerificationResult{}, researchPersistence(operation, err)
	}
	var sourceValues []string
	if err := decodeJSON(sourcesJSON, &sourceValues); err != nil {
		return research.VerificationResult{}, researchPersistence(operation, err)
	}
	sourceIDs, err := parseSourceIDs(sourceValues)
	if err != nil {
		return research.VerificationResult{}, researchPersistence(operation, err)
	}
	score, err := research.NewClaimConfidence(confidence)
	if err != nil {
		return research.VerificationResult{}, researchPersistence(operation, err)
	}
	verifiedAt, err := scanTimestamp(verified)
	if err != nil {
		return research.VerificationResult{}, researchPersistence(operation, err)
	}
	item := research.VerificationResult{ID: id, ClaimID: claimID, Status: research.VerificationStatus(status), SourceIDs: sourceIDs, Confidence: score, VerifiedAt: verifiedAt}
	if err := item.Validate(); err != nil {
		return research.VerificationResult{}, researchPersistence(operation, err)
	}
	return item, nil
}

type researchDriftRepository struct {
	executor executor
	timeout  time.Duration
}

func (repository *researchDriftRepository) Append(ctx context.Context, report research.DriftReport) error {
	const operation = "append SQLite drift report"
	if err := report.Validate(); err != nil {
		return researchInvalid(operation, err)
	}
	claims, err := encodeJSON(operation, claimIDStrings(report.AffectedClaims))
	if err != nil {
		return err
	}
	oldEvidence, err := encodeJSON(operation, idStrings(report.OldEvidence))
	if err != nil {
		return err
	}
	newEvidence, err := encodeJSON(operation, idStrings(report.NewEvidence))
	if err != nil {
		return err
	}
	opCtx, cancel, err := researchOperationContext(ctx, repository.timeout, operation)
	if err != nil {
		return err
	}
	defer cancel()
	exists, err := recordExists(opCtx, repository.executor, "drift_reports", "id", report.ID.String())
	if err != nil {
		return researchPersistence(operation, err)
	}
	if exists {
		return researchConflict(operation)
	}
	var newBundle any
	if report.NewBundleID != nil {
		newBundle = report.NewBundleID.String()
	}
	_, err = repository.executor.ExecContext(opCtx, `INSERT INTO drift_reports (id,old_bundle_id,new_bundle_id,drift_type,severity,affected_claim_ids_json,old_evidence_ids_json,new_evidence_ids_json,detected_at) VALUES (?,?,?,?,?,?,?,?,?)`, report.ID.String(), report.OldBundleID.String(), newBundle, string(report.Type), string(report.Severity), claims, oldEvidence, newEvidence, timestampText(report.DetectedAt))
	if err != nil {
		return researchPersistence(operation, err)
	}
	return nil
}
func (repository *researchDriftRepository) Get(ctx context.Context, id research.ID) (research.DriftReport, error) {
	const operation = "get SQLite drift report"
	if err := id.Validate(); err != nil {
		return research.DriftReport{}, researchInvalid(operation, err)
	}
	opCtx, cancel, err := researchOperationContext(ctx, repository.timeout, operation)
	if err != nil {
		return research.DriftReport{}, err
	}
	defer cancel()
	var idValue, oldBundleValue, driftType, severity, claimsJSON, oldEvidenceJSON, newEvidenceJSON, detected string
	var newBundleValue sql.NullString
	err = repository.executor.QueryRowContext(opCtx, `SELECT id,old_bundle_id,new_bundle_id,drift_type,severity,affected_claim_ids_json,old_evidence_ids_json,new_evidence_ids_json,detected_at FROM drift_reports WHERE id=?`, id.String()).Scan(&idValue, &oldBundleValue, &newBundleValue, &driftType, &severity, &claimsJSON, &oldEvidenceJSON, &newEvidenceJSON, &detected)
	if errors.Is(err, sql.ErrNoRows) {
		return research.DriftReport{}, researchNotFound(operation)
	}
	if err != nil {
		return research.DriftReport{}, researchPersistence(operation, err)
	}
	reportID, err := research.NewID(idValue)
	if err != nil {
		return research.DriftReport{}, researchPersistence(operation, err)
	}
	oldBundleID, err := research.NewID(oldBundleValue)
	if err != nil {
		return research.DriftReport{}, researchPersistence(operation, err)
	}
	var newBundleID *research.ID
	if newBundleValue.Valid {
		parsed, parseErr := research.NewID(newBundleValue.String)
		if parseErr != nil {
			return research.DriftReport{}, researchPersistence(operation, parseErr)
		}
		newBundleID = &parsed
	}
	var claimValues, oldEvidenceValues, newEvidenceValues []string
	for value, target := range map[string]*[]string{claimsJSON: &claimValues, oldEvidenceJSON: &oldEvidenceValues, newEvidenceJSON: &newEvidenceValues} {
		if decodeErr := decodeJSON(value, target); decodeErr != nil {
			return research.DriftReport{}, researchPersistence(operation, decodeErr)
		}
	}
	claims, err := parseClaimIDs(claimValues)
	if err != nil {
		return research.DriftReport{}, researchPersistence(operation, err)
	}
	oldEvidence, err := parseIDs(oldEvidenceValues)
	if err != nil {
		return research.DriftReport{}, researchPersistence(operation, err)
	}
	newEvidence, err := parseIDs(newEvidenceValues)
	if err != nil {
		return research.DriftReport{}, researchPersistence(operation, err)
	}
	detectedAt, err := scanTimestamp(detected)
	if err != nil {
		return research.DriftReport{}, researchPersistence(operation, err)
	}
	item := research.DriftReport{ID: reportID, OldBundleID: oldBundleID, NewBundleID: newBundleID, Type: research.DriftType(driftType), Severity: research.Severity(severity), AffectedClaims: claims, OldEvidence: oldEvidence, NewEvidence: newEvidence, DetectedAt: detectedAt}
	if err := item.Validate(); err != nil {
		return research.DriftReport{}, researchPersistence(operation, fmt.Errorf("invalid stored drift: %w", err))
	}
	return item, nil
}

type researchImpactRepository struct {
	executor executor
	timeout  time.Duration
}

func (repository *researchImpactRepository) Append(ctx context.Context, report research.ImpactReport) error {
	const operation = "append SQLite impact report"
	if err := report.Validate(); err != nil {
		return researchInvalid(operation, err)
	}
	bundles, err := encodeJSON(operation, idStrings(report.AffectedBundleIDs))
	if err != nil {
		return err
	}
	claims, err := encodeJSON(operation, claimIDStrings(report.AffectedClaimIDs))
	if err != nil {
		return err
	}
	opCtx, cancel, err := researchOperationContext(ctx, repository.timeout, operation)
	if err != nil {
		return err
	}
	defer cancel()
	exists, err := recordExists(opCtx, repository.executor, "impact_reports", "id", report.ID.String())
	if err != nil {
		return researchPersistence(operation, err)
	}
	if exists {
		return researchConflict(operation)
	}
	exists, err = recordExists(opCtx, repository.executor, "drift_reports", "id", report.DriftReportID.String())
	if err != nil {
		return researchPersistence(operation, err)
	}
	if !exists {
		return researchNotFound(operation)
	}
	_, err = repository.executor.ExecContext(opCtx, `INSERT INTO impact_reports (id,drift_report_id,affected_bundle_ids_json,affected_claim_ids_json,severity,recommended_action,assessed_at) VALUES (?,?,?,?,?,?,?)`, report.ID.String(), report.DriftReportID.String(), bundles, claims, string(report.Severity), string(report.RecommendedAction), timestampText(report.AssessedAt))
	if err != nil {
		return researchPersistence(operation, err)
	}
	return nil
}
func (repository *researchImpactRepository) Get(ctx context.Context, id research.ID) (research.ImpactReport, error) {
	const operation = "get SQLite impact report"
	if err := id.Validate(); err != nil {
		return research.ImpactReport{}, researchInvalid(operation, err)
	}
	opCtx, cancel, err := researchOperationContext(ctx, repository.timeout, operation)
	if err != nil {
		return research.ImpactReport{}, err
	}
	defer cancel()
	var idValue, driftValue, bundlesJSON, claimsJSON, severity, action, assessed string
	err = repository.executor.QueryRowContext(opCtx, `SELECT id,drift_report_id,affected_bundle_ids_json,affected_claim_ids_json,severity,recommended_action,assessed_at FROM impact_reports WHERE id=?`, id.String()).Scan(&idValue, &driftValue, &bundlesJSON, &claimsJSON, &severity, &action, &assessed)
	if errors.Is(err, sql.ErrNoRows) {
		return research.ImpactReport{}, researchNotFound(operation)
	}
	if err != nil {
		return research.ImpactReport{}, researchPersistence(operation, err)
	}
	reportID, err := research.NewID(idValue)
	if err != nil {
		return research.ImpactReport{}, researchPersistence(operation, err)
	}
	driftID, err := research.NewID(driftValue)
	if err != nil {
		return research.ImpactReport{}, researchPersistence(operation, err)
	}
	var bundleValues, claimValues []string
	if err := decodeJSON(bundlesJSON, &bundleValues); err != nil {
		return research.ImpactReport{}, researchPersistence(operation, err)
	}
	if err := decodeJSON(claimsJSON, &claimValues); err != nil {
		return research.ImpactReport{}, researchPersistence(operation, err)
	}
	bundles, err := parseIDs(bundleValues)
	if err != nil {
		return research.ImpactReport{}, researchPersistence(operation, err)
	}
	claims, err := parseClaimIDs(claimValues)
	if err != nil {
		return research.ImpactReport{}, researchPersistence(operation, err)
	}
	assessedAt, err := scanTimestamp(assessed)
	if err != nil {
		return research.ImpactReport{}, researchPersistence(operation, err)
	}
	item := research.ImpactReport{ID: reportID, DriftReportID: driftID, AffectedBundleIDs: bundles, AffectedClaimIDs: claims, Severity: research.Severity(severity), RecommendedAction: research.RecommendedAction(action), AssessedAt: assessedAt}
	if err := item.Validate(); err != nil {
		return research.ImpactReport{}, researchPersistence(operation, err)
	}
	return item, nil
}

type researchCacheRepository struct {
	executor executor
	timeout  time.Duration
}

const maximumResearchCachePayloadBytes = application.MaximumCachedSourceBodyBytes

func (repository *researchCacheRepository) Put(ctx context.Context, entry application.CacheEntry) error {
	const operation = "put SQLite research cache entry"
	if err := entry.Validate(); err != nil {
		return researchInvalid(operation, err)
	}
	if len(entry.Payload) > maximumResearchCachePayloadBytes {
		return researchInvalid(operation, fmt.Errorf("cache payload exceeds %d bytes", maximumResearchCachePayloadBytes))
	}
	opCtx, cancel, err := researchOperationContext(ctx, repository.timeout, operation)
	if err != nil {
		return err
	}
	defer cancel()
	_, err = repository.executor.ExecContext(opCtx, `INSERT INTO research_cache_entries (cache_key,payload,content_hash,stored_at,expires_at) VALUES (?,?,?,?,?) ON CONFLICT(cache_key) DO UPDATE SET payload=excluded.payload,content_hash=excluded.content_hash,stored_at=excluded.stored_at,expires_at=excluded.expires_at`, entry.Key, entry.Payload, entry.ContentHash, timestampText(entry.StoredAt), optionalTimestampText(entry.ExpiresAt))
	if err != nil {
		return researchPersistence(operation, err)
	}
	return nil
}
func (repository *researchCacheRepository) Get(ctx context.Context, key string) (application.CacheEntry, error) {
	const operation = "get SQLite research cache entry"
	if err := validateResearchKey(operation, key); err != nil {
		return application.CacheEntry{}, err
	}
	opCtx, cancel, err := researchOperationContext(ctx, repository.timeout, operation)
	if err != nil {
		return application.CacheEntry{}, err
	}
	defer cancel()
	var storedKey, hash, stored string
	var payload []byte
	var expires sql.NullString
	err = repository.executor.QueryRowContext(opCtx, `SELECT cache_key,payload,content_hash,stored_at,expires_at FROM research_cache_entries WHERE cache_key=?`, key).Scan(&storedKey, &payload, &hash, &stored, &expires)
	if errors.Is(err, sql.ErrNoRows) {
		return application.CacheEntry{}, researchNotFound(operation)
	}
	if err != nil {
		return application.CacheEntry{}, researchPersistence(operation, err)
	}
	storedAt, err := scanTimestamp(stored)
	if err != nil {
		return application.CacheEntry{}, researchPersistence(operation, err)
	}
	expiresAt, err := scanOptionalTimestamp(expires)
	if err != nil {
		return application.CacheEntry{}, researchPersistence(operation, err)
	}
	item := application.CacheEntry{Key: storedKey, Payload: append([]byte(nil), payload...), ContentHash: hash, StoredAt: storedAt, ExpiresAt: expiresAt}
	if err := item.Validate(); err != nil {
		return application.CacheEntry{}, researchPersistence(operation, err)
	}
	return item, nil
}
func (repository *researchCacheRepository) Delete(ctx context.Context, key string) error {
	const operation = "delete SQLite research cache entry"
	if err := validateResearchKey(operation, key); err != nil {
		return err
	}
	opCtx, cancel, err := researchOperationContext(ctx, repository.timeout, operation)
	if err != nil {
		return err
	}
	defer cancel()
	result, err := repository.executor.ExecContext(opCtx, `DELETE FROM research_cache_entries WHERE cache_key=?`, key)
	if err != nil {
		return researchPersistence(operation, err)
	}
	affected, err := result.RowsAffected()
	if err != nil {
		return researchPersistence(operation, err)
	}
	if affected == 0 {
		return researchNotFound(operation)
	}
	return nil
}
