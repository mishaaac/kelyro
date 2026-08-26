package sqlite

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"

	"github.com/mishaaac/kelyro/internal/research"
)

type researchClaimRepository struct {
	executor executor
	timeout  time.Duration
}

func (repository *researchClaimRepository) Append(ctx context.Context, claim research.Claim) error {
	const operation = "append SQLite research claim"
	if err := claim.Validate(); err != nil {
		return researchInvalid(operation, err)
	}
	evidenceJSON, err := encodeJSON(operation, idStrings(claim.EvidenceIDs))
	if err != nil {
		return err
	}
	opCtx, cancel, err := researchOperationContext(ctx, repository.timeout, operation)
	if err != nil {
		return err
	}
	defer cancel()
	exists, err := recordExists(opCtx, repository.executor, "claims", "id", claim.ID.String())
	if err != nil {
		return researchPersistence(operation, err)
	}
	if exists {
		return researchConflict(operation)
	}
	declared := make(map[string]struct{}, len(claim.SourceIDs))
	for _, sourceID := range claim.SourceIDs {
		exists, err = recordExists(opCtx, repository.executor, "sources", "id", sourceID.String())
		if err != nil {
			return researchPersistence(operation, err)
		}
		if !exists {
			return researchNotFound(operation)
		}
		declared[sourceID.String()] = struct{}{}
	}
	used := make(map[string]struct{}, len(declared))
	for _, evidenceID := range claim.EvidenceIDs {
		var sourceID string
		err = repository.executor.QueryRowContext(opCtx, `SELECT source_id FROM evidence WHERE id=?`, evidenceID.String()).Scan(&sourceID)
		if errors.Is(err, sql.ErrNoRows) {
			return researchNotFound(operation)
		}
		if err != nil {
			return researchPersistence(operation, err)
		}
		if _, exists := declared[sourceID]; !exists {
			return researchInvalid(operation, errors.New("claim evidence source is not declared"))
		}
		used[sourceID] = struct{}{}
	}
	if len(used) != len(declared) {
		return researchInvalid(operation, errors.New("claim source has no supporting evidence"))
	}
	_, err = repository.executor.ExecContext(opCtx, `INSERT INTO claims
(id,topic_subject,topic_domain,topic_technology,statement,claim_type,version_scope,confidence,evidence_ids_json,created_at,scope,status_scope)
VALUES (?,?,?,?,?,?,?,?,?,?,?,?)`, claim.ID.String(), claim.Topic.Subject, claim.Topic.Domain, claim.Topic.Technology,
		claim.Statement, string(claim.Type), optionalVersionText(claim.VersionScope), claim.Confidence.Value(), evidenceJSON,
		timestampText(claim.CreatedAt), claim.Scope, string(claim.StatusScope))
	if err != nil {
		return researchPersistence(operation, err)
	}
	for position, sourceID := range claim.SourceIDs {
		if _, err := repository.executor.ExecContext(opCtx, `INSERT INTO claim_sources (claim_id,source_id,position) VALUES (?,?,?)`, claim.ID.String(), sourceID.String(), position); err != nil {
			return researchPersistence(operation, err)
		}
	}
	return nil
}

func (repository *researchClaimRepository) Get(ctx context.Context, id research.ClaimID) (research.Claim, error) {
	const operation = "get SQLite research claim"
	if err := id.Validate(); err != nil {
		return research.Claim{}, researchInvalid(operation, err)
	}
	opCtx, cancel, err := researchOperationContext(ctx, repository.timeout, operation)
	if err != nil {
		return research.Claim{}, err
	}
	defer cancel()
	var idValue, subject, domain, technology, statement, claimType, evidenceJSON, created, scope, statusScope string
	var version sql.NullString
	var confidenceValue float64
	err = repository.executor.QueryRowContext(opCtx, `SELECT id,topic_subject,topic_domain,topic_technology,statement,claim_type,version_scope,confidence,evidence_ids_json,created_at,scope,status_scope FROM claims WHERE id=?`, id.String()).
		Scan(&idValue, &subject, &domain, &technology, &statement, &claimType, &version, &confidenceValue, &evidenceJSON, &created, &scope, &statusScope)
	if errors.Is(err, sql.ErrNoRows) {
		return research.Claim{}, researchNotFound(operation)
	}
	if err != nil {
		return research.Claim{}, researchPersistence(operation, err)
	}
	claimID, err := research.NewClaimID(idValue)
	if err != nil {
		return research.Claim{}, researchPersistence(operation, err)
	}
	topic, err := research.NewResearchTopic(subject, domain, technology)
	if err != nil {
		return research.Claim{}, researchPersistence(operation, err)
	}
	versionScope, err := scanOptionalVersion(version)
	if err != nil {
		return research.Claim{}, researchPersistence(operation, err)
	}
	confidence, err := research.NewClaimConfidence(confidenceValue)
	if err != nil {
		return research.Claim{}, researchPersistence(operation, err)
	}
	createdAt, err := scanTimestamp(created)
	if err != nil {
		return research.Claim{}, researchPersistence(operation, err)
	}
	var evidenceValues []string
	if err := decodeJSON(evidenceJSON, &evidenceValues); err != nil {
		return research.Claim{}, researchPersistence(operation, err)
	}
	evidenceIDs, err := parseIDs(evidenceValues)
	if err != nil {
		return research.Claim{}, researchPersistence(operation, err)
	}
	rows, err := repository.executor.QueryContext(opCtx, `SELECT source_id FROM claim_sources WHERE claim_id=? ORDER BY position`, id.String())
	if err != nil {
		return research.Claim{}, researchPersistence(operation, err)
	}
	defer rows.Close()
	var sourceValues []string
	for rows.Next() {
		var value string
		if err := rows.Scan(&value); err != nil {
			return research.Claim{}, researchPersistence(operation, err)
		}
		sourceValues = append(sourceValues, value)
	}
	if err := rows.Err(); err != nil {
		return research.Claim{}, researchPersistence(operation, err)
	}
	sourceIDs, err := parseSourceIDs(sourceValues)
	if err != nil {
		return research.Claim{}, researchPersistence(operation, err)
	}
	claim := research.Claim{
		ID: claimID, Topic: topic, Statement: statement, Type: research.ClaimType(claimType), Scope: scope,
		VersionScope: versionScope, StatusScope: research.ClaimStatusScope(statusScope), Confidence: confidence,
		SourceIDs: sourceIDs, EvidenceIDs: evidenceIDs, CreatedAt: createdAt,
	}
	if err := claim.Validate(); err != nil {
		return research.Claim{}, researchPersistence(operation, fmt.Errorf("invalid stored claim: %w", err))
	}
	return claim, nil
}
