package sqlite

import (
	"context"
	"database/sql"
	"errors"
	"strings"
	"time"

	"github.com/mishaaac/kelyro/internal/research"
)

type researchSourceDiscoveryRepository struct {
	executor executor
	timeout  time.Duration
}

func (repository *researchSourceDiscoveryRepository) Append(ctx context.Context, discovery research.DiscoveredSource) error {
	const operation = "append SQLite source discovery"
	if err := discovery.Validate(); err != nil {
		return researchInvalid(operation, err)
	}
	opCtx, cancel, err := researchOperationContext(ctx, repository.timeout, operation)
	if err != nil {
		return err
	}
	defer cancel()
	var locator string
	if err := repository.executor.QueryRowContext(opCtx, `SELECT locator FROM sources WHERE id=?`, discovery.SourceID.String()).Scan(&locator); errors.Is(err, sql.ErrNoRows) {
		return researchNotFound(operation)
	} else if err != nil {
		return researchPersistence(operation, err)
	}
	if locator != discovery.Locator.String() {
		return researchInvalid(operation, errors.New("discovery locator does not match source"))
	}
	if exists, err := recordExists(opCtx, repository.executor, "research_topics", "request_id", discovery.RequestID.String()); err != nil {
		return researchPersistence(operation, err)
	} else if !exists {
		return researchNotFound(operation)
	}
	_, err = repository.executor.ExecContext(opCtx, `INSERT INTO source_discoveries
(id,request_id,source_id,locator,query_text,title,snippet,provider,provider_rank,discovered_at,published_hint,cache_hit,cache_stale)
VALUES (?,?,?,?,?,?,?,?,?,?,?,?,?)`, discovery.ID.String(), discovery.RequestID.String(), discovery.SourceID.String(), discovery.Locator.String(),
		discovery.Query, discovery.Title, discovery.Snippet, discovery.Provider, discovery.Rank, timestampText(discovery.DiscoveredAt),
		optionalTimestampText(discovery.PublishedHint), discovery.CacheHit, discovery.CacheStale)
	if err != nil {
		if strings.Contains(err.Error(), "UNIQUE constraint failed") {
			return researchConflict(operation)
		}
		return researchPersistence(operation, err)
	}
	return nil
}

func (repository *researchSourceDiscoveryRepository) Get(ctx context.Context, id research.ID) (research.DiscoveredSource, error) {
	const operation = "get SQLite source discovery"
	if err := id.Validate(); err != nil {
		return research.DiscoveredSource{}, researchInvalid(operation, err)
	}
	opCtx, cancel, err := researchOperationContext(ctx, repository.timeout, operation)
	if err != nil {
		return research.DiscoveredSource{}, err
	}
	defer cancel()
	return scanSourceDiscovery(repository.executor.QueryRowContext(opCtx, sourceDiscoverySelect+` WHERE id=?`, id.String()), operation)
}

func (repository *researchSourceDiscoveryRepository) ListBySource(ctx context.Context, sourceID research.SourceID) ([]research.DiscoveredSource, error) {
	const operation = "list SQLite source discoveries"
	if err := sourceID.Validate(); err != nil {
		return nil, researchInvalid(operation, err)
	}
	opCtx, cancel, err := researchOperationContext(ctx, repository.timeout, operation)
	if err != nil {
		return nil, err
	}
	defer cancel()
	if exists, err := recordExists(opCtx, repository.executor, "sources", "id", sourceID.String()); err != nil {
		return nil, researchPersistence(operation, err)
	} else if !exists {
		return nil, researchNotFound(operation)
	}
	rows, err := repository.executor.QueryContext(opCtx, sourceDiscoverySelect+` WHERE source_id=? ORDER BY discovered_at,id`, sourceID.String())
	if err != nil {
		return nil, researchPersistence(operation, err)
	}
	defer rows.Close()
	result := make([]research.DiscoveredSource, 0)
	for rows.Next() {
		discovery, scanErr := scanSourceDiscovery(rows, operation)
		if scanErr != nil {
			return nil, scanErr
		}
		result = append(result, discovery)
	}
	if err := rows.Err(); err != nil {
		return nil, researchPersistence(operation, err)
	}
	return result, nil
}

const sourceDiscoverySelect = `SELECT id,request_id,source_id,locator,query_text,title,snippet,provider,provider_rank,discovered_at,published_hint,cache_hit,cache_stale FROM source_discoveries`

func scanSourceDiscovery(row rowScanner, operation string) (research.DiscoveredSource, error) {
	var idValue, requestIDValue, sourceIDValue, locatorValue, query, title, snippet, provider, discoveredAt string
	var publishedHint sql.NullString
	var rank int
	var cacheHit, cacheStale bool
	if err := row.Scan(&idValue, &requestIDValue, &sourceIDValue, &locatorValue, &query, &title, &snippet, &provider, &rank, &discoveredAt, &publishedHint, &cacheHit, &cacheStale); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return research.DiscoveredSource{}, researchNotFound(operation)
		}
		return research.DiscoveredSource{}, researchPersistence(operation, err)
	}
	id, err := research.NewID(idValue)
	if err != nil {
		return research.DiscoveredSource{}, researchPersistence(operation, err)
	}
	requestID, err := research.NewID(requestIDValue)
	if err != nil {
		return research.DiscoveredSource{}, researchPersistence(operation, err)
	}
	sourceID, err := research.NewSourceID(sourceIDValue)
	if err != nil {
		return research.DiscoveredSource{}, researchPersistence(operation, err)
	}
	locator, err := research.NewSourceLocator(locatorValue)
	if err != nil {
		return research.DiscoveredSource{}, researchPersistence(operation, err)
	}
	discovered, err := scanTimestamp(discoveredAt)
	if err != nil {
		return research.DiscoveredSource{}, researchPersistence(operation, err)
	}
	published, err := scanOptionalTimestamp(publishedHint)
	if err != nil {
		return research.DiscoveredSource{}, researchPersistence(operation, err)
	}
	result := research.DiscoveredSource{ID: id, RequestID: requestID, SourceID: sourceID, Locator: locator, Query: query,
		Title: title, Snippet: snippet, Provider: provider, Rank: rank, DiscoveredAt: discovered, PublishedHint: published,
		CacheHit: cacheHit, CacheStale: cacheStale}
	if err := result.Validate(); err != nil {
		return research.DiscoveredSource{}, researchPersistence(operation, err)
	}
	return result, nil
}
