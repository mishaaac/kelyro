package sqlite

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"

	"github.com/mishaaac/kelyro/internal/research"
)

type researchSourceBundleRepository struct {
	executor executor
	timeout  time.Duration
}

func (repository *researchSourceBundleRepository) Append(ctx context.Context, bundle research.SourceBundle) error {
	const operation = "append SQLite source bundle"
	if err := bundle.Validate(); err != nil {
		return researchInvalid(operation, err)
	}
	if bundle.AlgorithmVersion != research.SourceBundleAlgorithmV1 {
		return researchInvalid(operation, fmt.Errorf("new source bundles require %q", research.SourceBundleAlgorithmV1))
	}
	encoded, err := bundle.ExportJSON()
	if err != nil {
		return researchInvalid(operation, err)
	}
	opCtx, cancel, err := researchOperationContext(ctx, repository.timeout, operation)
	if err != nil {
		return err
	}
	defer cancel()
	target := repository.executor
	var transaction *sql.Tx
	if starter, ok := repository.executor.(researchTransactionStarter); ok {
		transaction, err = starter.BeginTx(opCtx, nil)
		if err != nil {
			return researchPersistence(operation, err)
		}
		target = transaction
	}
	rollback := func(cause error) error {
		if transaction == nil {
			return cause
		}
		if rollbackErr := transaction.Rollback(); rollbackErr != nil {
			return errors.Join(cause, researchPersistence(operation, fmt.Errorf("rollback source bundle append: %w", rollbackErr)))
		}
		return cause
	}
	exists, err := recordExists(opCtx, target, "source_bundles", "id", bundle.ID.String())
	if err != nil {
		return rollback(researchPersistence(operation, err))
	}
	if exists {
		return rollback(researchConflict(operation))
	}
	if exists, err = recordExists(opCtx, target, "research_runs", "id", bundle.RunID.String()); err != nil {
		return rollback(researchPersistence(operation, err))
	} else if !exists {
		return rollback(researchNotFound(operation))
	}
	var requestSubject, requestDomain, requestTechnology, requestPurpose, runStatus string
	var requestVersion, runCompleted sql.NullString
	if err := target.QueryRowContext(opCtx, `SELECT t.subject,t.domain,t.technology,t.purpose,t.target_version,r.status,r.completed_at FROM research_runs r JOIN research_topics t ON t.request_id=r.request_id WHERE r.id=?`, bundle.RunID.String()).Scan(
		&requestSubject, &requestDomain, &requestTechnology, &requestPurpose, &requestVersion, &runStatus, &runCompleted,
	); err != nil {
		return rollback(researchPersistence(operation, err))
	}
	completedAt, err := scanOptionalTimestamp(runCompleted)
	if err != nil {
		return rollback(researchPersistence(operation, err))
	}
	if runStatus != string(research.ResearchRunCompleted) || completedAt == nil || bundle.VerifiedAt.Before(*completedAt) ||
		bundle.Topic != (research.ResearchTopic{Subject: requestSubject, Domain: requestDomain, Technology: requestTechnology}) ||
		string(bundle.Purpose) != requestPurpose || !sameStoredVersion(bundle.TargetVersion, requestVersion) {
		return rollback(researchInvalid(operation, errors.New("bundle research run/request relationship does not match")))
	}
	claimSet := make(map[research.ClaimID]struct{}, len(bundle.ClaimIDs))
	declaredSources := make(map[string]struct{})
	for _, claimID := range bundle.ClaimIDs {
		if exists, err = recordExists(opCtx, target, "claims", "id", claimID.String()); err != nil {
			return rollback(researchPersistence(operation, err))
		} else if !exists {
			return rollback(researchNotFound(operation))
		}
		claimSet[claimID] = struct{}{}
		rows, queryErr := target.QueryContext(opCtx, `SELECT source_id FROM claim_sources WHERE claim_id=?`, claimID.String())
		if queryErr != nil {
			return rollback(researchPersistence(operation, queryErr))
		}
		for rows.Next() {
			var sourceID string
			if scanErr := rows.Scan(&sourceID); scanErr != nil {
				_ = rows.Close()
				return rollback(researchPersistence(operation, scanErr))
			}
			declaredSources[sourceID] = struct{}{}
		}
		if rowsErr := rows.Err(); rowsErr != nil {
			_ = rows.Close()
			return rollback(researchPersistence(operation, rowsErr))
		}
		if closeErr := rows.Close(); closeErr != nil {
			return rollback(researchPersistence(operation, closeErr))
		}
	}
	if len(declaredSources) != len(bundle.Sources) {
		return rollback(researchInvalid(operation, errors.New("bundle sources do not match bundle claim source union")))
	}
	for _, source := range bundle.Sources {
		if exists, err = recordExists(opCtx, target, "sources", "id", source.SourceID.String()); err != nil {
			return rollback(researchPersistence(operation, err))
		} else if !exists {
			return rollback(researchNotFound(operation))
		}
		if _, declared := declaredSources[source.SourceID.String()]; !declared {
			return rollback(researchInvalid(operation, errors.New("bundle source is not declared by a bundle claim")))
		}
	}
	for _, conflictID := range bundle.ConflictIDs {
		if exists, err = recordExists(opCtx, target, "source_conflicts", "id", conflictID.String()); err != nil {
			return rollback(researchPersistence(operation, err))
		} else if !exists {
			return rollback(researchNotFound(operation))
		}
		var claimValuesJSON string
		if err := target.QueryRowContext(opCtx, `SELECT claim_ids_json FROM source_conflicts WHERE id=?`, conflictID.String()).Scan(&claimValuesJSON); err != nil {
			return rollback(researchPersistence(operation, err))
		}
		var claimValues []string
		if err := decodeJSON(claimValuesJSON, &claimValues); err != nil {
			return rollback(researchPersistence(operation, err))
		}
		relevant := false
		for _, value := range claimValues {
			claimID, parseErr := research.NewClaimID(value)
			if parseErr != nil {
				return rollback(researchPersistence(operation, parseErr))
			}
			if _, exists := claimSet[claimID]; exists {
				relevant = true
				break
			}
		}
		if !relevant {
			return rollback(researchInvalid(operation, errors.New("bundle conflict is unrelated to bundle claims")))
		}
	}
	_, err = target.ExecContext(opCtx, `INSERT INTO source_bundles
(id,run_id,topic_subject,topic_domain,topic_technology,purpose,target_version,state,verified_at,summary,bundle_json,content_hash,algorithm_version)
VALUES (?,?,?,?,?,?,?,?,?,?,?,?,?)`, bundle.ID.String(), bundle.RunID.String(), bundle.Topic.Subject,
		bundle.Topic.Domain, bundle.Topic.Technology, string(bundle.Purpose), optionalVersionText(bundle.TargetVersion),
		string(bundle.State), timestampText(bundle.VerifiedAt), bundle.Summary, string(encoded), bundle.ContentHash, bundle.AlgorithmVersion)
	if err != nil {
		return rollback(researchPersistence(operation, err))
	}
	for position, claimID := range bundle.ClaimIDs {
		_, err = target.ExecContext(opCtx, `INSERT INTO source_bundle_items (bundle_id,item_type,item_id,position,temporal_scope,source_role,version_scope,warning) VALUES (?,?,?,?,NULL,NULL,NULL,'')`, bundle.ID.String(), "claim", claimID.String(), position)
		if err != nil {
			return rollback(researchPersistence(operation, err))
		}
	}
	for position, source := range bundle.Sources {
		_, err = target.ExecContext(opCtx, `INSERT INTO source_bundle_items (bundle_id,item_type,item_id,position,temporal_scope,source_role,version_scope,warning) VALUES (?,?,?,?,?,?,?,?)`, bundle.ID.String(), "source", source.SourceID.String(), position, string(source.TemporalScope), string(source.Role), optionalVersionText(source.VersionScope), source.Warning)
		if err != nil {
			return rollback(researchPersistence(operation, err))
		}
	}
	if transaction != nil {
		if err := transaction.Commit(); err != nil {
			return researchPersistence(operation, err)
		}
	}
	return nil
}

func (repository *researchSourceBundleRepository) Get(ctx context.Context, id research.ID) (research.SourceBundle, error) {
	const operation = "get SQLite source bundle"
	if err := id.Validate(); err != nil {
		return research.SourceBundle{}, researchInvalid(operation, err)
	}
	opCtx, cancel, err := researchOperationContext(ctx, repository.timeout, operation)
	if err != nil {
		return research.SourceBundle{}, err
	}
	defer cancel()
	return repository.get(opCtx, operation, `WHERE id=?`, id.String())
}

func (repository *researchSourceBundleRepository) ListByRun(ctx context.Context, runID research.ID) ([]research.SourceBundle, error) {
	const operation = "list SQLite source bundles by run"
	if err := runID.Validate(); err != nil {
		return nil, researchInvalid(operation, err)
	}
	opCtx, cancel, err := researchOperationContext(ctx, repository.timeout, operation)
	if err != nil {
		return nil, err
	}
	defer cancel()
	rows, err := repository.executor.QueryContext(opCtx, `SELECT id FROM source_bundles WHERE run_id=? ORDER BY verified_at,id`, runID.String())
	if err != nil {
		return nil, researchPersistence(operation, err)
	}
	defer rows.Close()
	ids := make([]string, 0)
	for rows.Next() {
		var id string
		if scanErr := rows.Scan(&id); scanErr != nil {
			return nil, researchPersistence(operation, scanErr)
		}
		ids = append(ids, id)
	}
	if err := rows.Err(); err != nil {
		return nil, researchPersistence(operation, err)
	}
	if err := rows.Close(); err != nil {
		return nil, researchPersistence(operation, err)
	}
	result := make([]research.SourceBundle, 0, len(ids))
	for _, id := range ids {
		bundle, getErr := repository.get(opCtx, operation, `WHERE id=?`, id)
		if getErr != nil {
			return nil, getErr
		}
		result = append(result, bundle)
	}
	return result, nil
}

func (repository *researchSourceBundleRepository) get(ctx context.Context, operation, suffix string, value ...any) (research.SourceBundle, error) {
	return repository.scan(ctx, repository.executor.QueryRowContext(ctx, sourceBundleSelect+` `+suffix, value...), operation)
}

const sourceBundleSelect = `SELECT id,run_id,topic_subject,topic_domain,topic_technology,purpose,target_version,state,verified_at,summary,bundle_json,content_hash,algorithm_version FROM source_bundles`

func (repository *researchSourceBundleRepository) scan(ctx context.Context, row rowScanner, operation string) (research.SourceBundle, error) {
	var idValue, runValue, subject, domain, technology, purpose, state, verified, summary, encoded, contentHash, algorithm string
	var targetVersion sql.NullString
	if err := row.Scan(&idValue, &runValue, &subject, &domain, &technology, &purpose, &targetVersion, &state, &verified, &summary, &encoded, &contentHash, &algorithm); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return research.SourceBundle{}, researchNotFound(operation)
		}
		return research.SourceBundle{}, researchPersistence(operation, err)
	}
	if algorithm == research.SourceBundleAlgorithmV1 {
		bundle, err := research.ParseSourceBundleJSON([]byte(encoded))
		if err != nil {
			return research.SourceBundle{}, researchPersistence(operation, fmt.Errorf("invalid stored source bundle: %w", err))
		}
		verifiedAt, err := scanTimestamp(verified)
		if err != nil {
			return research.SourceBundle{}, researchPersistence(operation, err)
		}
		if bundle.ID.String() != idValue || bundle.RunID.String() != runValue || bundle.Topic != (research.ResearchTopic{Subject: subject, Domain: domain, Technology: technology}) ||
			string(bundle.Purpose) != purpose || string(bundle.State) != state || bundle.Summary != summary || bundle.ContentHash != contentHash ||
			bundle.AlgorithmVersion != algorithm || !bundle.VerifiedAt.Time().Equal(verifiedAt.Time()) || !sameStoredVersion(bundle.TargetVersion, targetVersion) {
			return research.SourceBundle{}, researchPersistence(operation, errors.New("stored source bundle metadata does not match canonical representation"))
		}
		return bundle, nil
	}
	return repository.scanLegacy(ctx, operation, idValue, runValue, subject, domain, technology, purpose, targetVersion, state, verified, summary, algorithm)
}

func (repository *researchSourceBundleRepository) scanLegacy(ctx context.Context, operation, idValue, runValue, subject, domain, technology, purpose string, targetVersion sql.NullString, state, verified, summary, algorithm string) (research.SourceBundle, error) {
	id, err := research.NewID(idValue)
	if err != nil {
		return research.SourceBundle{}, researchPersistence(operation, err)
	}
	runID, err := research.NewID(runValue)
	if err != nil {
		return research.SourceBundle{}, researchPersistence(operation, err)
	}
	topic, err := research.NewResearchTopic(subject, domain, technology)
	if err != nil {
		return research.SourceBundle{}, researchPersistence(operation, err)
	}
	version, err := scanOptionalVersion(targetVersion)
	if err != nil {
		return research.SourceBundle{}, researchPersistence(operation, err)
	}
	verifiedAt, err := scanTimestamp(verified)
	if err != nil {
		return research.SourceBundle{}, researchPersistence(operation, err)
	}
	claims, sources, err := repository.loadLegacyItems(ctx, id, operation)
	if err != nil {
		return research.SourceBundle{}, err
	}
	zero, _ := research.NewFreshnessScore(0)
	bundle := research.SourceBundle{
		ID: id, RunID: runID, Topic: topic, Purpose: research.ResearchPurpose(purpose), TargetVersion: version,
		ClaimIDs: claims, Sources: sources, State: research.SourceBundleState(state), Summary: summary,
		Freshness:  research.SourceBundleFreshness{State: research.FreshnessUnknown, Score: zero, AlgorithmVersion: research.SourceBundleFreshnessLegacy},
		VerifiedAt: verifiedAt, AlgorithmVersion: algorithm,
	}
	if err := bundle.Validate(); err != nil {
		return research.SourceBundle{}, researchPersistence(operation, fmt.Errorf("invalid legacy source bundle: %w", err))
	}
	return bundle, nil
}

func (repository *researchSourceBundleRepository) loadLegacyItems(ctx context.Context, bundleID research.ID, operation string) ([]research.ClaimID, []research.SourceBundleSource, error) {
	rows, err := repository.executor.QueryContext(ctx, `SELECT item_type,item_id,temporal_scope,source_role,version_scope,warning FROM source_bundle_items WHERE bundle_id=? ORDER BY item_type,position`, bundleID.String())
	if err != nil {
		return nil, nil, researchPersistence(operation, err)
	}
	defer rows.Close()
	claims := make([]research.ClaimID, 0)
	sources := make([]research.SourceBundleSource, 0)
	for rows.Next() {
		var itemType, itemID, warning string
		var temporalScope, role, version sql.NullString
		if err := rows.Scan(&itemType, &itemID, &temporalScope, &role, &version, &warning); err != nil {
			return nil, nil, researchPersistence(operation, err)
		}
		if itemType == "claim" {
			id, parseErr := research.NewClaimID(itemID)
			if parseErr != nil {
				return nil, nil, researchPersistence(operation, parseErr)
			}
			claims = append(claims, id)
			continue
		}
		id, parseErr := research.NewSourceID(itemID)
		if parseErr != nil {
			return nil, nil, researchPersistence(operation, parseErr)
		}
		versionScope, parseErr := scanOptionalVersion(version)
		if parseErr != nil {
			return nil, nil, researchPersistence(operation, parseErr)
		}
		sources = append(sources, research.SourceBundleSource{SourceID: id, Role: research.SourceBundleSourceRole(role.String), TemporalScope: research.SourceTemporalScope(temporalScope.String), VersionScope: versionScope, Warning: warning})
	}
	if err := rows.Err(); err != nil {
		return nil, nil, researchPersistence(operation, err)
	}
	return claims, sources, nil
}

func sameStoredVersion(version *research.SourceVersion, stored sql.NullString) bool {
	if version == nil {
		return !stored.Valid
	}
	return stored.Valid && version.String() == stored.String
}
