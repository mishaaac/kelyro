package sqlite

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"

	"github.com/mishaaac/kelyro/internal/research"
)

type researchSourceRepository struct {
	executor executor
	timeout  time.Duration
}

func (repository *researchSourceRepository) Create(ctx context.Context, source research.Source) error {
	const operation = "create SQLite research source"
	if err := source.Validate(); err != nil {
		return researchInvalid(operation, err)
	}
	opCtx, cancel, err := researchOperationContext(ctx, repository.timeout, operation)
	if err != nil {
		return err
	}
	defer cancel()
	for column, value := range map[string]string{"id": source.ID.String(), "locator": source.Locator.String()} {
		exists, checkErr := recordExists(opCtx, repository.executor, "sources", column, value)
		if checkErr != nil {
			return researchPersistence(operation, checkErr)
		}
		if exists {
			return researchConflict(operation)
		}
	}
	_, err = repository.executor.ExecContext(opCtx, `INSERT INTO sources
(id,kind,locator,version,title,publisher,language,published_at,updated_at,created_at)
VALUES (?,?,?,?,?,?,?,?,?,?)`, source.ID.String(), string(source.Kind), source.Locator.String(), optionalVersionText(source.Version),
		source.Metadata.Title, source.Metadata.Publisher, source.Metadata.Language, optionalTimestampText(source.Metadata.PublishedAt),
		optionalTimestampText(source.Metadata.UpdatedAt), timestampText(source.CreatedAt))
	if err != nil {
		return researchPersistence(operation, err)
	}
	return nil
}

func (repository *researchSourceRepository) Get(ctx context.Context, id research.SourceID) (research.Source, error) {
	const operation = "get SQLite research source"
	if err := id.Validate(); err != nil {
		return research.Source{}, researchInvalid(operation, err)
	}
	opCtx, cancel, err := researchOperationContext(ctx, repository.timeout, operation)
	if err != nil {
		return research.Source{}, err
	}
	defer cancel()
	return scanResearchSource(repository.executor.QueryRowContext(opCtx, `SELECT id,kind,locator,version,title,publisher,language,published_at,updated_at,created_at FROM sources WHERE id=?`, id.String()), operation)
}

func (repository *researchSourceRepository) FindByLocator(ctx context.Context, locator research.SourceLocator) (research.Source, error) {
	const operation = "find SQLite research source by locator"
	if err := locator.Validate(); err != nil {
		return research.Source{}, researchInvalid(operation, err)
	}
	opCtx, cancel, err := researchOperationContext(ctx, repository.timeout, operation)
	if err != nil {
		return research.Source{}, err
	}
	defer cancel()
	return scanResearchSource(repository.executor.QueryRowContext(opCtx, `SELECT id,kind,locator,version,title,publisher,language,published_at,updated_at,created_at FROM sources WHERE locator=?`, locator.String()), operation)
}

func (repository *researchSourceRepository) List(ctx context.Context) ([]research.Source, error) {
	const operation = "list SQLite research sources"
	opCtx, cancel, err := researchOperationContext(ctx, repository.timeout, operation)
	if err != nil {
		return nil, err
	}
	defer cancel()
	rows, err := repository.executor.QueryContext(opCtx, `SELECT id,kind,locator,version,title,publisher,language,published_at,updated_at,created_at FROM sources ORDER BY id`)
	if err != nil {
		return nil, researchPersistence(operation, err)
	}
	defer rows.Close()
	result := make([]research.Source, 0)
	for rows.Next() {
		source, scanErr := scanResearchSource(rows, operation)
		if scanErr != nil {
			return nil, scanErr
		}
		result = append(result, source)
	}
	if err := rows.Err(); err != nil {
		return nil, researchPersistence(operation, err)
	}
	return result, nil
}

func scanResearchSource(row rowScanner, operation string) (research.Source, error) {
	var idValue, kind, locatorValue, title, publisher, language, created string
	var version, published, updated sql.NullString
	if err := row.Scan(&idValue, &kind, &locatorValue, &version, &title, &publisher, &language, &published, &updated, &created); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return research.Source{}, researchNotFound(operation)
		}
		return research.Source{}, researchPersistence(operation, err)
	}
	id, err := research.NewSourceID(idValue)
	if err != nil {
		return research.Source{}, researchPersistence(operation, err)
	}
	locator, err := research.NewSourceLocator(locatorValue)
	if err != nil {
		return research.Source{}, researchPersistence(operation, err)
	}
	versionValue, err := scanOptionalVersion(version)
	if err != nil {
		return research.Source{}, researchPersistence(operation, err)
	}
	publishedAt, err := scanOptionalTimestamp(published)
	if err != nil {
		return research.Source{}, researchPersistence(operation, err)
	}
	updatedAt, err := scanOptionalTimestamp(updated)
	if err != nil {
		return research.Source{}, researchPersistence(operation, err)
	}
	createdAt, err := scanTimestamp(created)
	if err != nil {
		return research.Source{}, researchPersistence(operation, err)
	}
	source := research.Source{ID: id, Kind: research.SourceKind(kind), Locator: locator, Version: versionValue, Metadata: research.SourceMetadata{Title: title, Publisher: publisher, Language: language, PublishedAt: publishedAt, UpdatedAt: updatedAt}, CreatedAt: createdAt}
	if err := source.Validate(); err != nil {
		return research.Source{}, researchPersistence(operation, fmt.Errorf("invalid stored source: %w", err))
	}
	return source, nil
}

type researchSnapshotRepository struct {
	executor executor
	timeout  time.Duration
}

func (repository *researchSnapshotRepository) Append(ctx context.Context, snapshot research.SourceSnapshot) error {
	const operation = "append SQLite source snapshot"
	if err := snapshot.Validate(); err != nil {
		return researchInvalid(operation, err)
	}
	opCtx, cancel, err := researchOperationContext(ctx, repository.timeout, operation)
	if err != nil {
		return err
	}
	defer cancel()
	exists, err := recordExists(opCtx, repository.executor, "source_snapshots", "id", snapshot.ID.String())
	if err != nil {
		return researchPersistence(operation, err)
	}
	if exists {
		return researchConflict(operation)
	}
	sourceExists, err := recordExists(opCtx, repository.executor, "sources", "id", snapshot.SourceID.String())
	if err != nil {
		return researchPersistence(operation, err)
	}
	if !sourceExists {
		return researchNotFound(operation)
	}
	_, err = repository.executor.ExecContext(opCtx, `INSERT INTO source_snapshots (id,source_id,locator,fetched_at,status_code,content_type,etag,last_modified,content_hash,content_length,fetch_version) VALUES (?,?,?,?,?,?,?,?,?,?,?)`, snapshot.ID.String(), snapshot.SourceID.String(), snapshot.Locator.String(), timestampText(snapshot.FetchedAt), snapshot.Fetch.StatusCode, snapshot.Fetch.ContentType, snapshot.Fetch.ETag, snapshot.Fetch.LastModified, snapshot.Fetch.ContentHash, snapshot.Fetch.ContentLength, snapshot.Fetch.FetchVersion)
	if err != nil {
		return researchPersistence(operation, err)
	}
	return nil
}

func (repository *researchSnapshotRepository) Get(ctx context.Context, id research.ID) (research.SourceSnapshot, error) {
	return repository.get(ctx, "get SQLite source snapshot", `WHERE id=?`, id.String(), id.Validate())
}
func (repository *researchSnapshotRepository) LatestBySource(ctx context.Context, id research.SourceID) (research.SourceSnapshot, error) {
	return repository.get(ctx, "get latest SQLite source snapshot", `WHERE source_id=? ORDER BY fetched_at DESC,id DESC LIMIT 1`, id.String(), id.Validate())
}
func (repository *researchSnapshotRepository) get(ctx context.Context, operation, suffix, value string, validation error) (research.SourceSnapshot, error) {
	if validation != nil {
		return research.SourceSnapshot{}, researchInvalid(operation, validation)
	}
	opCtx, cancel, err := researchOperationContext(ctx, repository.timeout, operation)
	if err != nil {
		return research.SourceSnapshot{}, err
	}
	defer cancel()
	return scanSnapshot(repository.executor.QueryRowContext(opCtx, `SELECT id,source_id,locator,fetched_at,status_code,content_type,etag,last_modified,content_hash,content_length,fetch_version FROM source_snapshots `+suffix, value), operation)
}
func (repository *researchSnapshotRepository) ListBySource(ctx context.Context, id research.SourceID) ([]research.SourceSnapshot, error) {
	const operation = "list SQLite source snapshots"
	if err := id.Validate(); err != nil {
		return nil, researchInvalid(operation, err)
	}
	opCtx, cancel, err := researchOperationContext(ctx, repository.timeout, operation)
	if err != nil {
		return nil, err
	}
	defer cancel()
	rows, err := repository.executor.QueryContext(opCtx, `SELECT id,source_id,locator,fetched_at,status_code,content_type,etag,last_modified,content_hash,content_length,fetch_version FROM source_snapshots WHERE source_id=? ORDER BY fetched_at,id`, id.String())
	if err != nil {
		return nil, researchPersistence(operation, err)
	}
	defer rows.Close()
	result := make([]research.SourceSnapshot, 0)
	for rows.Next() {
		item, scanErr := scanSnapshot(rows, operation)
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
func scanSnapshot(row rowScanner, operation string) (research.SourceSnapshot, error) {
	var idValue, sourceValue, locatorValue, fetched, contentType, etag, lastModified, contentHash, fetchVersion string
	var status int
	var length int64
	if err := row.Scan(&idValue, &sourceValue, &locatorValue, &fetched, &status, &contentType, &etag, &lastModified, &contentHash, &length, &fetchVersion); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return research.SourceSnapshot{}, researchNotFound(operation)
		}
		return research.SourceSnapshot{}, researchPersistence(operation, err)
	}
	id, err := research.NewID(idValue)
	if err != nil {
		return research.SourceSnapshot{}, researchPersistence(operation, err)
	}
	sourceID, err := research.NewSourceID(sourceValue)
	if err != nil {
		return research.SourceSnapshot{}, researchPersistence(operation, err)
	}
	locator, err := research.NewSourceLocator(locatorValue)
	if err != nil {
		return research.SourceSnapshot{}, researchPersistence(operation, err)
	}
	fetchedAt, err := scanTimestamp(fetched)
	if err != nil {
		return research.SourceSnapshot{}, researchPersistence(operation, err)
	}
	item := research.SourceSnapshot{ID: id, SourceID: sourceID, Locator: locator, FetchedAt: fetchedAt, Fetch: research.FetchMetadata{StatusCode: status, ContentType: contentType, ETag: etag, LastModified: lastModified, ContentHash: contentHash, ContentLength: length, FetchVersion: fetchVersion}}
	if err := item.Validate(); err != nil {
		return research.SourceSnapshot{}, researchPersistence(operation, err)
	}
	return item, nil
}

type researchEvidenceRepository struct {
	executor executor
	timeout  time.Duration
}

func (repository *researchEvidenceRepository) Append(ctx context.Context, item research.Evidence) error {
	const operation = "append SQLite research evidence"
	if err := item.Validate(); err != nil {
		return researchInvalid(operation, err)
	}
	opCtx, cancel, err := researchOperationContext(ctx, repository.timeout, operation)
	if err != nil {
		return err
	}
	defer cancel()
	exists, err := recordExists(opCtx, repository.executor, "evidence", "id", item.ID.String())
	if err != nil {
		return researchPersistence(operation, err)
	}
	if exists {
		return researchConflict(operation)
	}
	var snapshotSource string
	err = repository.executor.QueryRowContext(opCtx, `SELECT source_id FROM source_snapshots WHERE id=?`, item.SnapshotID.String()).Scan(&snapshotSource)
	if errors.Is(err, sql.ErrNoRows) {
		return researchNotFound(operation)
	}
	if err != nil {
		return researchPersistence(operation, err)
	}
	if snapshotSource != item.SourceID.String() {
		return researchInvalid(operation, errors.New("evidence source does not match snapshot"))
	}
	_, err = repository.executor.ExecContext(opCtx, `INSERT INTO evidence (id,source_id,snapshot_id,location,excerpt,excerpt_hash,context_before,context_after,extracted_at,extractor_version) VALUES (?,?,?,?,?,?,?,?,?,?)`, item.ID.String(), item.SourceID.String(), item.SnapshotID.String(), item.Location, item.Excerpt, item.ExcerptHash, item.ContextBefore, item.ContextAfter, timestampText(item.ExtractedAt), item.ExtractorVersion)
	if err != nil {
		return researchPersistence(operation, err)
	}
	return nil
}
func (repository *researchEvidenceRepository) Get(ctx context.Context, id research.ID) (research.Evidence, error) {
	return repository.getOne(ctx, "get SQLite research evidence", `WHERE id=?`, id.String(), id.Validate())
}
func (repository *researchEvidenceRepository) getOne(ctx context.Context, operation, suffix, value string, validation error) (research.Evidence, error) {
	if validation != nil {
		return research.Evidence{}, researchInvalid(operation, validation)
	}
	opCtx, cancel, err := researchOperationContext(ctx, repository.timeout, operation)
	if err != nil {
		return research.Evidence{}, err
	}
	defer cancel()
	return scanResearchEvidence(repository.executor.QueryRowContext(opCtx, `SELECT id,source_id,snapshot_id,location,excerpt,excerpt_hash,context_before,context_after,extracted_at,extractor_version FROM evidence `+suffix, value), operation)
}
func (repository *researchEvidenceRepository) ListBySource(ctx context.Context, id research.SourceID) ([]research.Evidence, error) {
	return repository.list(ctx, "list SQLite evidence by source", "source_id", id.String(), id.Validate())
}
func (repository *researchEvidenceRepository) ListBySnapshot(ctx context.Context, id research.ID) ([]research.Evidence, error) {
	return repository.list(ctx, "list SQLite evidence by snapshot", "snapshot_id", id.String(), id.Validate())
}
func (repository *researchEvidenceRepository) list(ctx context.Context, operation, column, value string, validation error) ([]research.Evidence, error) {
	if validation != nil {
		return nil, researchInvalid(operation, validation)
	}
	opCtx, cancel, err := researchOperationContext(ctx, repository.timeout, operation)
	if err != nil {
		return nil, err
	}
	defer cancel()
	rows, err := repository.executor.QueryContext(opCtx, `SELECT id,source_id,snapshot_id,location,excerpt,excerpt_hash,context_before,context_after,extracted_at,extractor_version FROM evidence WHERE `+column+`=? ORDER BY id`, value)
	if err != nil {
		return nil, researchPersistence(operation, err)
	}
	defer rows.Close()
	result := make([]research.Evidence, 0)
	for rows.Next() {
		item, scanErr := scanResearchEvidence(rows, operation)
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
func scanResearchEvidence(row rowScanner, operation string) (research.Evidence, error) {
	var idValue, sourceValue, snapshotValue, location, excerpt, hash, contextBefore, contextAfter, extracted, version string
	if err := row.Scan(&idValue, &sourceValue, &snapshotValue, &location, &excerpt, &hash, &contextBefore, &contextAfter, &extracted, &version); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return research.Evidence{}, researchNotFound(operation)
		}
		return research.Evidence{}, researchPersistence(operation, err)
	}
	id, err := research.NewID(idValue)
	if err != nil {
		return research.Evidence{}, researchPersistence(operation, err)
	}
	sourceID, err := research.NewSourceID(sourceValue)
	if err != nil {
		return research.Evidence{}, researchPersistence(operation, err)
	}
	snapshotID, err := research.NewID(snapshotValue)
	if err != nil {
		return research.Evidence{}, researchPersistence(operation, err)
	}
	extractedAt, err := scanTimestamp(extracted)
	if err != nil {
		return research.Evidence{}, researchPersistence(operation, err)
	}
	item := research.Evidence{
		ID: id, SourceID: sourceID, SnapshotID: snapshotID, Location: location,
		Excerpt: excerpt, ExcerptHash: hash, ContextBefore: contextBefore, ContextAfter: contextAfter,
		ExtractedAt: extractedAt, ExtractorVersion: version,
	}
	if err := item.Validate(); err != nil {
		return research.Evidence{}, researchPersistence(operation, fmt.Errorf("invalid stored evidence: %w", err))
	}
	return item, nil
}
