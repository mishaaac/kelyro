package sqlite

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"

	"github.com/mishaaac/kelyro/internal/research"
)

type researchCitationRepository struct {
	executor executor
	timeout  time.Duration
}

func (repository *researchCitationRepository) Append(ctx context.Context, citation research.Citation) error {
	const operation = "append SQLite citation"
	if err := citation.Validate(); err != nil {
		return researchInvalid(operation, err)
	}
	if citation.TemporalAlgorithmVersion != research.SourceTemporalPolicyV1 {
		return researchInvalid(operation, errors.New("new citations must use source-temporal-policy-v1"))
	}
	source, err := (&researchSourceRepository{repository.executor, repository.timeout}).Get(ctx, citation.SourceID)
	if err != nil {
		return err
	}
	snapshot, err := (&researchSnapshotRepository{repository.executor, repository.timeout}).Get(ctx, citation.SnapshotID)
	if err != nil {
		return err
	}
	evidence, err := (&researchEvidenceRepository{repository.executor, repository.timeout}).Get(ctx, citation.EvidenceID)
	if err != nil {
		return err
	}
	if err := research.ValidateCitationRelationships(citation, source, snapshot, evidence); err != nil {
		return researchInvalid(operation, err)
	}
	opCtx, cancel, err := researchOperationContext(ctx, repository.timeout, operation)
	if err != nil {
		return err
	}
	defer cancel()
	exists, err := recordExists(opCtx, repository.executor, "citations", "id", citation.ID.String())
	if err != nil {
		return researchPersistence(operation, err)
	}
	if exists {
		return researchConflict(operation)
	}
	var deepLocator any
	deepLabel := any("")
	if citation.DeepLink != nil {
		deepLocator = citation.DeepLink.Locator.String()
		deepLabel = citation.DeepLink.Label
	}
	_, err = repository.executor.ExecContext(opCtx, `INSERT INTO citations
(id,source_id,snapshot_id,evidence_id,title,locator,deep_link_locator,deep_link_label,snapshot_date,last_verified,link_strategy,section,version_scope,algorithm_version,temporal_scope,temporal_warning,temporal_algorithm_version)
VALUES (?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?)`, citation.ID.String(), citation.SourceID.String(), citation.SnapshotID.String(),
		citation.EvidenceID.String(), citation.Title, citation.Locator.String(), deepLocator, deepLabel,
		timestampText(citation.SnapshotDate), timestampText(citation.LastVerified), string(citation.LinkStrategy),
		citation.Section, optionalVersionText(citation.VersionScope), citation.AlgorithmVersion,
		string(citation.TemporalScope), citation.TemporalWarning, citation.TemporalAlgorithmVersion)
	if err != nil {
		return researchPersistence(operation, err)
	}
	return nil
}

func (repository *researchCitationRepository) Get(ctx context.Context, id research.ID) (research.Citation, error) {
	const operation = "get SQLite citation"
	if err := id.Validate(); err != nil {
		return research.Citation{}, researchInvalid(operation, err)
	}
	opCtx, cancel, err := researchOperationContext(ctx, repository.timeout, operation)
	if err != nil {
		return research.Citation{}, err
	}
	defer cancel()
	return scanResearchCitation(repository.executor.QueryRowContext(opCtx, citationSelect+` WHERE id=?`, id.String()), operation)
}

func (repository *researchCitationRepository) ListByEvidence(ctx context.Context, evidenceID research.ID) ([]research.Citation, error) {
	const operation = "list SQLite citations by evidence"
	if err := evidenceID.Validate(); err != nil {
		return nil, researchInvalid(operation, err)
	}
	opCtx, cancel, err := researchOperationContext(ctx, repository.timeout, operation)
	if err != nil {
		return nil, err
	}
	defer cancel()
	rows, err := repository.executor.QueryContext(opCtx, citationSelect+` WHERE evidence_id=? ORDER BY id`, evidenceID.String())
	if err != nil {
		return nil, researchPersistence(operation, err)
	}
	defer rows.Close()
	result := make([]research.Citation, 0)
	for rows.Next() {
		item, scanErr := scanResearchCitation(rows, operation)
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

const citationSelect = `SELECT id,source_id,snapshot_id,evidence_id,title,locator,deep_link_locator,deep_link_label,snapshot_date,last_verified,link_strategy,section,version_scope,algorithm_version,temporal_scope,temporal_warning,temporal_algorithm_version FROM citations`

func scanResearchCitation(row rowScanner, operation string) (research.Citation, error) {
	var idValue, sourceValue, snapshotValue, evidenceValue, title, locatorValue string
	var deepLocator, deepLabel, version sql.NullString
	var snapshotDateValue, lastVerifiedValue, strategy, section, algorithm string
	var temporalScope, temporalWarning, temporalAlgorithm string
	if err := row.Scan(&idValue, &sourceValue, &snapshotValue, &evidenceValue, &title, &locatorValue,
		&deepLocator, &deepLabel, &snapshotDateValue, &lastVerifiedValue, &strategy, &section, &version, &algorithm,
		&temporalScope, &temporalWarning, &temporalAlgorithm); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return research.Citation{}, researchNotFound(operation)
		}
		return research.Citation{}, researchPersistence(operation, err)
	}
	id, err := research.NewID(idValue)
	if err != nil {
		return research.Citation{}, researchPersistence(operation, err)
	}
	sourceID, err := research.NewSourceID(sourceValue)
	if err != nil {
		return research.Citation{}, researchPersistence(operation, err)
	}
	snapshotID, err := research.NewID(snapshotValue)
	if err != nil {
		return research.Citation{}, researchPersistence(operation, err)
	}
	evidenceID, err := research.NewID(evidenceValue)
	if err != nil {
		return research.Citation{}, researchPersistence(operation, err)
	}
	locator, err := research.NewSourceLocator(locatorValue)
	if err != nil {
		return research.Citation{}, researchPersistence(operation, err)
	}
	var deepLink *research.DeepLink
	if deepLocator.Valid {
		locator, locatorErr := research.NewSourceLocator(deepLocator.String)
		if locatorErr != nil {
			return research.Citation{}, researchPersistence(operation, locatorErr)
		}
		deepLink = &research.DeepLink{Locator: locator, Label: deepLabel.String}
	}
	snapshotDate, err := scanTimestamp(snapshotDateValue)
	if err != nil {
		return research.Citation{}, researchPersistence(operation, err)
	}
	lastVerified, err := scanTimestamp(lastVerifiedValue)
	if err != nil {
		return research.Citation{}, researchPersistence(operation, err)
	}
	versionScope, err := scanOptionalVersion(version)
	if err != nil {
		return research.Citation{}, researchPersistence(operation, err)
	}
	result := research.Citation{
		ID: id, SourceID: sourceID, SnapshotID: snapshotID, EvidenceID: evidenceID,
		Title: title, Locator: locator, DeepLink: deepLink,
		LinkStrategy: research.CitationLinkStrategy(strategy), Section: section,
		SnapshotDate: snapshotDate, VersionScope: versionScope, LastVerified: lastVerified,
		AlgorithmVersion: algorithm, TemporalScope: research.SourceTemporalScope(temporalScope),
		TemporalWarning: temporalWarning, TemporalAlgorithmVersion: temporalAlgorithm,
	}
	if err := result.Validate(); err != nil {
		return research.Citation{}, researchPersistence(operation, fmt.Errorf("invalid stored citation: %w", err))
	}
	return result, nil
}
