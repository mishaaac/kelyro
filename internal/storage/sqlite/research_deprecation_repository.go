package sqlite

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/mishaaac/kelyro/internal/research"
)

type researchDeprecationRepository struct {
	executor executor
	timeout  time.Duration
}

func (repository *researchDeprecationRepository) Append(ctx context.Context, record research.DeprecationRecord) error {
	const operation = "append SQLite deprecation"
	if err := record.Validate(); err != nil {
		return researchInvalid(operation, err)
	}
	if record.AlgorithmVersion != research.DeprecationIntelligenceAlgorithmV1 {
		return researchInvalid(operation, errors.New("new deprecation records must use deprecation-intelligence-v1"))
	}
	sourcesJSON, err := encodeJSON(operation, sourceIDStrings(record.SourceIDs))
	if err != nil {
		return err
	}
	evidenceJSON, err := encodeJSON(operation, idStrings(record.EvidenceIDs))
	if err != nil {
		return err
	}
	opCtx, cancel, err := researchOperationContext(ctx, repository.timeout, operation)
	if err != nil {
		return err
	}
	defer cancel()
	if exists, existsErr := recordExists(opCtx, repository.executor, "deprecation_records", "id", record.ID.String()); existsErr != nil {
		return researchPersistence(operation, existsErr)
	} else if exists {
		return researchConflict(operation)
	}
	declared := make(map[string]struct{}, len(record.SourceIDs))
	for _, sourceID := range record.SourceIDs {
		exists, existsErr := recordExists(opCtx, repository.executor, "sources", "id", sourceID.String())
		if existsErr != nil {
			return researchPersistence(operation, existsErr)
		}
		if !exists {
			return researchNotFound(operation)
		}
		declared[sourceID.String()] = struct{}{}
	}
	used := make(map[string]struct{}, len(declared))
	for _, evidenceID := range record.EvidenceIDs {
		var sourceID string
		err = repository.executor.QueryRowContext(opCtx, `SELECT source_id FROM evidence WHERE id=?`, evidenceID.String()).Scan(&sourceID)
		if errors.Is(err, sql.ErrNoRows) {
			return researchNotFound(operation)
		}
		if err != nil {
			return researchPersistence(operation, err)
		}
		if _, exists := declared[sourceID]; !exists {
			return researchInvalid(operation, errors.New("deprecation evidence source is not declared"))
		}
		used[sourceID] = struct{}{}
	}
	if len(used) != len(declared) {
		return researchInvalid(operation, errors.New("deprecation source has no supporting evidence"))
	}
	_, err = repository.executor.ExecContext(opCtx, `INSERT INTO deprecation_records
(id,subject,status,introduced_in,deprecated_in,removed_in,replacement,source_ids_json,evidence_ids_json,verified_at,determination,algorithm_version)
VALUES (?,?,?,?,?,?,?,?,?,?,?,?)`, record.ID.String(), record.Subject, string(record.Status),
		optionalVersionText(record.IntroducedIn), optionalVersionText(record.DeprecatedIn), optionalVersionText(record.RemovedIn),
		record.Replacement, sourcesJSON, evidenceJSON, timestampText(record.VerifiedAt), string(record.Determination), record.AlgorithmVersion)
	if err != nil {
		return researchPersistence(operation, err)
	}
	return nil
}

func (repository *researchDeprecationRepository) Get(ctx context.Context, id research.ID) (research.DeprecationRecord, error) {
	const operation = "get SQLite deprecation"
	if err := id.Validate(); err != nil {
		return research.DeprecationRecord{}, researchInvalid(operation, err)
	}
	opCtx, cancel, err := researchOperationContext(ctx, repository.timeout, operation)
	if err != nil {
		return research.DeprecationRecord{}, err
	}
	defer cancel()
	return scanDeprecation(repository.executor.QueryRowContext(opCtx, deprecationSelect+` WHERE id=?`, id.String()), operation)
}

func (repository *researchDeprecationRepository) ListBySubject(ctx context.Context, subject string) ([]research.DeprecationRecord, error) {
	const operation = "list SQLite deprecation history"
	if strings.TrimSpace(subject) == "" || subject != strings.TrimSpace(subject) {
		return nil, researchInvalid(operation, errors.New("deprecation subject is invalid"))
	}
	opCtx, cancel, err := researchOperationContext(ctx, repository.timeout, operation)
	if err != nil {
		return nil, err
	}
	defer cancel()
	rows, err := repository.executor.QueryContext(opCtx, deprecationSelect+` WHERE subject=? ORDER BY verified_at,id`, subject)
	if err != nil {
		return nil, researchPersistence(operation, err)
	}
	defer rows.Close()
	result := make([]research.DeprecationRecord, 0)
	for rows.Next() {
		record, scanErr := scanDeprecation(rows, operation)
		if scanErr != nil {
			return nil, scanErr
		}
		result = append(result, record)
	}
	if err := rows.Err(); err != nil {
		return nil, researchPersistence(operation, err)
	}
	return result, nil
}

const deprecationSelect = `SELECT id,subject,status,introduced_in,deprecated_in,removed_in,replacement,source_ids_json,evidence_ids_json,verified_at,determination,algorithm_version FROM deprecation_records`

func scanDeprecation(row rowScanner, operation string) (research.DeprecationRecord, error) {
	var idValue, subject, status, replacement, sourcesJSON, evidenceJSON, verified, determination, algorithm string
	var introduced, deprecated, removed sql.NullString
	if err := row.Scan(&idValue, &subject, &status, &introduced, &deprecated, &removed, &replacement,
		&sourcesJSON, &evidenceJSON, &verified, &determination, &algorithm); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return research.DeprecationRecord{}, researchNotFound(operation)
		}
		return research.DeprecationRecord{}, researchPersistence(operation, err)
	}
	id, err := research.NewID(idValue)
	if err != nil {
		return research.DeprecationRecord{}, researchPersistence(operation, err)
	}
	introducedIn, err := scanOptionalVersion(introduced)
	if err != nil {
		return research.DeprecationRecord{}, researchPersistence(operation, err)
	}
	deprecatedIn, err := scanOptionalVersion(deprecated)
	if err != nil {
		return research.DeprecationRecord{}, researchPersistence(operation, err)
	}
	removedIn, err := scanOptionalVersion(removed)
	if err != nil {
		return research.DeprecationRecord{}, researchPersistence(operation, err)
	}
	var sourceValues, evidenceValues []string
	if err := decodeJSON(sourcesJSON, &sourceValues); err != nil {
		return research.DeprecationRecord{}, researchPersistence(operation, err)
	}
	if err := decodeJSON(evidenceJSON, &evidenceValues); err != nil {
		return research.DeprecationRecord{}, researchPersistence(operation, err)
	}
	sourceIDs, err := parseSourceIDs(sourceValues)
	if err != nil {
		return research.DeprecationRecord{}, researchPersistence(operation, err)
	}
	evidenceIDs, err := parseIDs(evidenceValues)
	if err != nil {
		return research.DeprecationRecord{}, researchPersistence(operation, err)
	}
	verifiedAt, err := scanTimestamp(verified)
	if err != nil {
		return research.DeprecationRecord{}, researchPersistence(operation, err)
	}
	record := research.DeprecationRecord{
		ID: id, Subject: subject, Status: research.DeprecationStatus(status),
		Determination: research.DeprecationDetermination(determination),
		IntroducedIn:  introducedIn, DeprecatedIn: deprecatedIn, RemovedIn: removedIn,
		Replacement: replacement, SourceIDs: sourceIDs, EvidenceIDs: evidenceIDs,
		VerifiedAt: verifiedAt, AlgorithmVersion: algorithm,
	}
	if err := record.Validate(); err != nil {
		return research.DeprecationRecord{}, researchPersistence(operation, fmt.Errorf("invalid stored deprecation: %w", err))
	}
	return record, nil
}
