package researchcachefs

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"strings"
	"time"

	"github.com/mishaaac/kelyro/internal/research"
	"github.com/mishaaac/kelyro/internal/research/application"
)

// OfflineAdapter encodes the live-port outputs that Step 07 may read without
// network access. It also exposes normalized-source and Source Bundle layers.
type OfflineAdapter struct {
	cache application.ResearchCacheService
}

func NewOfflineAdapter(cache application.ResearchCacheService) *OfflineAdapter {
	return &OfflineAdapter{cache: cache}
}

type searchResultJSON struct {
	Title         string `json:"title"`
	Locator       string `json:"locator"`
	Snippet       string `json:"snippet,omitempty"`
	Provider      string `json:"provider"`
	Rank          int    `json:"rank"`
	PublishedHint string `json:"published_hint,omitempty"`
}

func (adapter *OfflineAdapter) CacheSearch(ctx context.Context, query application.SearchQuery, options application.SearchOptions, results []application.SearchResult) error {
	if adapter == nil || adapter.cache == nil {
		return errors.New("research cache service is unavailable")
	}
	wire := make([]searchResultJSON, 0, len(results))
	for index, result := range results {
		result.CacheHit, result.CacheStale, result.CacheWarning = false, false, ""
		if err := result.Validate(); err != nil {
			return fmt.Errorf("cache search result %d: %w", index, err)
		}
		item := searchResultJSON{Title: result.Title, Locator: result.Locator.String(), Snippet: result.Snippet, Provider: result.Provider, Rank: result.Rank}
		if result.PublishedHint != nil {
			item.PublishedHint = formatCacheTimestamp(*result.PublishedHint)
		}
		wire = append(wire, item)
	}
	encoded, err := json.Marshal(wire)
	if err != nil {
		return err
	}
	return adapter.cache.Put(ctx, application.CacheLayerDiscovery, searchCacheKey(query, options), encoded)
}

func (adapter *OfflineAdapter) SearchCached(ctx context.Context, query application.SearchQuery, options application.SearchOptions) ([]application.SearchResult, error) {
	lookup, err := adapter.lookup(ctx, application.CacheLayerDiscovery, searchCacheKey(query, options))
	if err != nil {
		return nil, err
	}
	var wire []searchResultJSON
	if err := decodeStrict(lookup.Record.Payload, &wire); err != nil {
		return nil, cachePersistence("decode discovery cache", err)
	}
	results := make([]application.SearchResult, 0, len(wire))
	for index, item := range wire {
		locator, err := research.NewSourceLocator(item.Locator)
		if err != nil {
			return nil, cachePersistence("decode discovery cache", err)
		}
		result := application.SearchResult{
			Title: item.Title, Locator: locator, Snippet: item.Snippet, Provider: item.Provider, Rank: item.Rank,
			CacheHit: true, CacheStale: lookup.Stale, CacheWarning: lookup.Warning,
		}
		if item.PublishedHint != "" {
			published, parseErr := parseCacheTimestamp(item.PublishedHint)
			if parseErr != nil {
				return nil, cachePersistence("decode discovery cache", parseErr)
			}
			result.PublishedHint = &published
		}
		if err := result.Validate(); err != nil {
			return nil, cachePersistence("decode discovery cache", fmt.Errorf("result %d: %w", index, err))
		}
		results = append(results, result)
	}
	return results, nil
}

type fetchedMetadataJSON struct {
	SourceID      string `json:"source_id"`
	Locator       string `json:"locator"`
	FetchedAt     string `json:"fetched_at"`
	StatusCode    int    `json:"status_code"`
	ContentType   string `json:"content_type"`
	ETag          string `json:"etag,omitempty"`
	LastModified  string `json:"last_modified,omitempty"`
	ContentHash   string `json:"content_hash,omitempty"`
	ContentLength int64  `json:"content_length"`
	FetchVersion  string `json:"fetch_version"`
}

type fetchedBodyJSON struct {
	Body []byte `json:"body"`
}

func (adapter *OfflineAdapter) CacheFetched(ctx context.Context, request application.FetchRequest, source application.FetchedSource) error {
	if adapter == nil || adapter.cache == nil {
		return errors.New("research cache service is unavailable")
	}
	source.Origin = application.FetchOriginLive
	source.CacheHit, source.CacheStale, source.CacheWarning = false, false, ""
	if err := source.Validate(); err != nil {
		return err
	}
	if source.SourceID != request.SourceID {
		return errors.New("fetched source does not match cache request")
	}
	metadata := fetchedMetadataJSON{
		SourceID: source.SourceID.String(), Locator: source.Locator.String(), FetchedAt: formatCacheTimestamp(source.FetchedAt),
		StatusCode: source.Metadata.StatusCode, ContentType: source.Metadata.ContentType, ETag: source.Metadata.ETag,
		LastModified: source.Metadata.LastModified, ContentHash: source.Metadata.ContentHash,
		ContentLength: source.Metadata.ContentLength, FetchVersion: source.Metadata.FetchVersion,
	}
	metadataJSON, err := json.Marshal(metadata)
	if err != nil {
		return err
	}
	bodyJSON, err := json.Marshal(fetchedBodyJSON{Body: source.Body})
	if err != nil {
		return err
	}
	key := fetchCacheKey(request)
	if err := adapter.cache.Put(ctx, application.CacheLayerFetchMetadata, key, metadataJSON); err != nil {
		return err
	}
	return adapter.cache.Put(ctx, application.CacheLayerBoundedSource, key, bodyJSON)
}

func (adapter *OfflineAdapter) FetchCached(ctx context.Context, request application.FetchRequest) (application.FetchedSource, error) {
	key := fetchCacheKey(request)
	metadataLookup, err := adapter.lookup(ctx, application.CacheLayerFetchMetadata, key)
	if err != nil {
		return application.FetchedSource{}, err
	}
	bodyLookup, err := adapter.lookup(ctx, application.CacheLayerBoundedSource, key)
	if err != nil {
		return application.FetchedSource{}, err
	}
	var metadata fetchedMetadataJSON
	if err := decodeStrict(metadataLookup.Record.Payload, &metadata); err != nil {
		return application.FetchedSource{}, cachePersistence("decode fetch metadata cache", err)
	}
	var body fetchedBodyJSON
	if err := decodeStrict(bodyLookup.Record.Payload, &body); err != nil {
		return application.FetchedSource{}, cachePersistence("decode bounded source cache", err)
	}
	sourceID, err := research.NewSourceID(metadata.SourceID)
	if err != nil {
		return application.FetchedSource{}, cachePersistence("decode fetch metadata cache", err)
	}
	locator, err := research.NewSourceLocator(metadata.Locator)
	if err != nil {
		return application.FetchedSource{}, cachePersistence("decode fetch metadata cache", err)
	}
	fetchedAt, err := parseCacheTimestamp(metadata.FetchedAt)
	if err != nil {
		return application.FetchedSource{}, cachePersistence("decode fetch metadata cache", err)
	}
	stale := metadataLookup.Stale || bodyLookup.Stale
	warning := application.CacheWarning("")
	if stale {
		warning = application.CacheWarningStaleOffline
	}
	result := application.FetchedSource{
		SourceID: sourceID, Locator: locator, FetchedAt: fetchedAt, Body: append([]byte(nil), body.Body...),
		Origin: application.FetchOriginCache, CacheHit: true, CacheStale: stale, CacheWarning: warning,
		Metadata: research.FetchMetadata{
			StatusCode: metadata.StatusCode, ContentType: metadata.ContentType, ETag: metadata.ETag,
			LastModified: metadata.LastModified, ContentHash: metadata.ContentHash,
			ContentLength: metadata.ContentLength, FetchVersion: metadata.FetchVersion,
		},
	}
	if err := result.Validate(); err != nil {
		return application.FetchedSource{}, cachePersistence("decode fetched source cache", err)
	}
	return result, nil
}

type releaseRecordJSON struct {
	ID           string   `json:"id"`
	TechnologyID string   `json:"technology_id"`
	Version      string   `json:"version"`
	Channel      string   `json:"channel"`
	Status       string   `json:"status"`
	SourceIDs    []string `json:"source_ids"`
	ReleasedAt   string   `json:"released_at,omitempty"`
	VerifiedAt   string   `json:"verified_at"`
}

func (adapter *OfflineAdapter) CacheReleases(ctx context.Context, query application.ReleaseLookupQuery, records []research.ReleaseRecord) error {
	wire := make([]releaseRecordJSON, 0, len(records))
	for index, record := range records {
		if err := record.Validate(); err != nil {
			return fmt.Errorf("cache release %d: %w", index, err)
		}
		if record.TechnologyID != query.TechnologyID || record.Channel != query.Channel {
			return fmt.Errorf("cache release %d does not match lookup query", index)
		}
		item := releaseRecordJSON{
			ID: record.ID.String(), TechnologyID: record.TechnologyID.String(), Version: record.Version.String(),
			Channel: string(record.Channel), Status: string(record.Status), VerifiedAt: formatCacheTimestamp(record.VerifiedAt),
			SourceIDs: make([]string, 0, len(record.SourceIDs)),
		}
		for _, sourceID := range record.SourceIDs {
			item.SourceIDs = append(item.SourceIDs, sourceID.String())
		}
		if record.ReleasedAt != nil {
			item.ReleasedAt = formatCacheTimestamp(*record.ReleasedAt)
		}
		wire = append(wire, item)
	}
	encoded, err := json.Marshal(wire)
	if err != nil {
		return err
	}
	return adapter.cache.Put(ctx, application.CacheLayerFetchMetadata, releaseCacheKey(query), encoded)
}

func (adapter *OfflineAdapter) LookupCachedReleases(ctx context.Context, query application.ReleaseLookupQuery) ([]research.ReleaseRecord, error) {
	lookup, err := adapter.lookup(ctx, application.CacheLayerFetchMetadata, releaseCacheKey(query))
	if err != nil {
		return nil, err
	}
	var wire []releaseRecordJSON
	if err := decodeStrict(lookup.Record.Payload, &wire); err != nil {
		return nil, cachePersistence("decode release lookup cache", err)
	}
	records := make([]research.ReleaseRecord, 0, len(wire))
	for index, item := range wire {
		id, err := research.NewID(item.ID)
		if err != nil {
			return nil, cachePersistence("decode release lookup cache", err)
		}
		technologyID, err := research.NewID(item.TechnologyID)
		if err != nil {
			return nil, cachePersistence("decode release lookup cache", err)
		}
		version, err := research.NewVersionIdentifier(item.Version)
		if err != nil {
			return nil, cachePersistence("decode release lookup cache", err)
		}
		verifiedAt, err := parseCacheTimestamp(item.VerifiedAt)
		if err != nil {
			return nil, cachePersistence("decode release lookup cache", err)
		}
		sourceIDs := make([]research.SourceID, 0, len(item.SourceIDs))
		for _, sourceValue := range item.SourceIDs {
			sourceID, sourceErr := research.NewSourceID(sourceValue)
			if sourceErr != nil {
				return nil, cachePersistence("decode release lookup cache", sourceErr)
			}
			sourceIDs = append(sourceIDs, sourceID)
		}
		record := research.ReleaseRecord{
			ID: id, TechnologyID: technologyID, Version: version,
			Channel: research.ReleaseChannel(item.Channel), Status: research.ReleaseStatus(item.Status),
			SourceIDs: sourceIDs, VerifiedAt: verifiedAt,
		}
		if item.ReleasedAt != "" {
			releasedAt, parseErr := parseCacheTimestamp(item.ReleasedAt)
			if parseErr != nil {
				return nil, cachePersistence("decode release lookup cache", parseErr)
			}
			record.ReleasedAt = &releasedAt
		}
		if err := record.Validate(); err != nil || record.TechnologyID != query.TechnologyID || record.Channel != query.Channel {
			if err == nil {
				err = errors.New("release does not match lookup query")
			}
			return nil, cachePersistence("decode release lookup cache", fmt.Errorf("record %d: %w", index, err))
		}
		records = append(records, record)
	}
	return records, nil
}

func (adapter *OfflineAdapter) CacheNormalized(ctx context.Context, key string, canonicalJSON []byte) error {
	return adapter.cache.Put(ctx, application.CacheLayerNormalizedSource, normalizedCacheKey(key), canonicalJSON)
}

func (adapter *OfflineAdapter) LoadNormalized(ctx context.Context, key string) (application.CacheLookup, error) {
	return adapter.lookup(ctx, application.CacheLayerNormalizedSource, normalizedCacheKey(key))
}

func (adapter *OfflineAdapter) CacheSourceBundle(ctx context.Context, bundle research.SourceBundle) error {
	encoded, err := bundle.ExportJSON()
	if err != nil {
		return err
	}
	return adapter.cache.Put(ctx, application.CacheLayerSourceBundle, bundleCacheKey(bundle.ID.String()), encoded)
}

func (adapter *OfflineAdapter) LoadSourceBundle(ctx context.Context, id research.ID) (research.SourceBundle, application.CacheLookup, error) {
	lookup, err := adapter.lookup(ctx, application.CacheLayerSourceBundle, bundleCacheKey(id.String()))
	if err != nil {
		return research.SourceBundle{}, application.CacheLookup{}, err
	}
	bundle, err := research.ParseSourceBundleJSON(lookup.Record.Payload)
	if err != nil {
		return research.SourceBundle{}, application.CacheLookup{}, cachePersistence("decode source bundle cache", err)
	}
	if bundle.ID != id {
		return research.SourceBundle{}, application.CacheLookup{}, cachePersistence("decode source bundle cache", errors.New("bundle identity does not match cache key"))
	}
	return bundle, lookup, nil
}

func (adapter *OfflineAdapter) lookup(ctx context.Context, layer application.CacheLayer, key string) (application.CacheLookup, error) {
	if adapter == nil || adapter.cache == nil {
		return application.CacheLookup{}, errors.New("research cache service is unavailable")
	}
	lookup, err := adapter.cache.Get(ctx, layer, key, application.CacheReadOffline)
	if err != nil {
		return application.CacheLookup{}, err
	}
	if !lookup.Hit {
		return application.CacheLookup{}, application.Classify(application.ErrorNotFound, "read offline research cache", errors.New("cache miss"))
	}
	return lookup, nil
}

func searchCacheKey(query application.SearchQuery, options application.SearchOptions) string {
	kind, version := "", ""
	if options.DesiredKind != nil {
		kind = string(*options.DesiredKind)
	}
	if options.TargetVersion != nil {
		version = options.TargetVersion.String()
	}
	return hashedCacheKey("discovery", query.RequestID.String(), query.Text, kind, version, fmt.Sprint(options.Limit))
}

func fetchCacheKey(request application.FetchRequest) string {
	return hashedCacheKey("fetch", request.SourceID.String(), request.Locator.String(), fmt.Sprint(request.MaximumBytes))
}

func releaseCacheKey(query application.ReleaseLookupQuery) string {
	return hashedCacheKey("release", query.TechnologyID.String(), string(query.Channel))
}

func normalizedCacheKey(key string) string { return hashedCacheKey("normalized", key) }
func bundleCacheKey(key string) string     { return hashedCacheKey("bundle", key) }

func hashedCacheKey(prefix string, parts ...string) string {
	return prefix + ":" + strings.TrimPrefix(research.CanonicalContentHashV1([]byte(strings.Join(parts, "\x00"))), "sha256:")
}

func decodeStrict(data []byte, target any) error {
	decoder := json.NewDecoder(bytes.NewReader(data))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(target); err != nil {
		return err
	}
	var extra any
	if err := decoder.Decode(&extra); !errors.Is(err, io.EOF) {
		if err == nil {
			return errors.New("cache payload contains trailing JSON")
		}
		return err
	}
	return nil
}

func formatCacheTimestamp(timestamp research.Timestamp) string {
	return timestamp.Time().Format(time.RFC3339Nano)
}

func parseCacheTimestamp(value string) (research.Timestamp, error) {
	parsed, err := time.Parse(time.RFC3339Nano, value)
	if err != nil {
		return research.Timestamp{}, err
	}
	return research.NewTimestamp(parsed.UTC())
}

func cachePersistence(operation string, err error) error {
	return application.Classify(application.ErrorPersistenceFailure, operation, err)
}

var (
	_ application.SearchCache        = (*OfflineAdapter)(nil)
	_ application.SourceFetchCache   = (*OfflineAdapter)(nil)
	_ application.ReleaseLookupCache = (*OfflineAdapter)(nil)
)
