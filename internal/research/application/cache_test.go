package application_test

import (
	"context"
	"errors"
	"fmt"
	"sort"
	"testing"
	"time"

	"github.com/mishaaac/kelyro/internal/research"
	"github.com/mishaaac/kelyro/internal/research/application"
)

func TestResearchCachePolicyV1DefinesEveryRequiredLayer(t *testing.T) {
	t.Parallel()
	want := map[application.CacheLayer]struct {
		ttl time.Duration
		max int
	}{
		application.CacheLayerDiscovery:        {24 * time.Hour, 256 << 10},
		application.CacheLayerFetchMetadata:    {24 * time.Hour, 32 << 10},
		application.CacheLayerBoundedSource:    {7 * 24 * time.Hour, application.MaximumCachedSourceBodyBytes},
		application.CacheLayerNormalizedSource: {7 * 24 * time.Hour, application.MaximumCachedSourceBodyBytes},
		application.CacheLayerSourceBundle:     {30 * 24 * time.Hour, research.MaximumSourceBundleJSONBytes},
	}
	for _, layer := range application.ResearchCacheLayers() {
		policy, err := application.CachePolicyV1(layer)
		if err != nil {
			t.Fatal(err)
		}
		expected, ok := want[layer]
		if !ok || policy.TTL != expected.ttl || policy.MaximumBytes != expected.max {
			t.Fatalf("policy %s = %+v, want %+v", layer, policy, expected)
		}
		delete(want, layer)
	}
	if len(want) != 0 {
		t.Fatalf("missing cache layers = %+v", want)
	}
}

func TestResearchCacheServiceReportsExplicitFreshAndOfflineStaleHits(t *testing.T) {
	t.Parallel()
	store := newMemoryCacheStore()
	clock := &cacheClock{now: cacheTimestamp(t, time.Date(2026, 8, 28, 10, 0, 0, 0, time.UTC))}
	service := application.NewResearchCacheService(store, clock)
	ctx := context.Background()
	if err := service.Put(ctx, application.CacheLayerDiscovery, "query:portable", []byte(`[{"title":"Portable"}]`)); err != nil {
		t.Fatal(err)
	}
	fresh, err := service.Get(ctx, application.CacheLayerDiscovery, "query:portable", application.CacheReadFreshOnly)
	if err != nil || !fresh.Hit || fresh.Stale || fresh.Warning != "" {
		t.Fatalf("fresh lookup = (%+v,%v)", fresh, err)
	}
	fresh.Record.Payload[0] = 'X'
	reloaded, err := service.Get(ctx, application.CacheLayerDiscovery, "query:portable", application.CacheReadFreshOnly)
	if err != nil || reloaded.Record.Payload[0] == 'X' {
		t.Fatalf("cache lookup leaked payload = (%q,%v)", reloaded.Record.Payload, err)
	}

	clock.now = cacheTimestamp(t, clock.now.Time().Add(25*time.Hour))
	miss, err := service.Get(ctx, application.CacheLayerDiscovery, "query:portable", application.CacheReadFreshOnly)
	if err != nil || miss.Hit || !miss.Stale {
		t.Fatalf("fresh-only stale lookup = (%+v,%v)", miss, err)
	}
	offline, err := service.Get(ctx, application.CacheLayerDiscovery, "query:portable", application.CacheReadOffline)
	if err != nil || !offline.Hit || !offline.Stale || offline.Warning != application.CacheWarningStaleOffline {
		t.Fatalf("offline stale lookup = (%+v,%v)", offline, err)
	}
	missing, err := service.Get(ctx, application.CacheLayerDiscovery, "query:missing", application.CacheReadOffline)
	if err != nil || missing.Hit {
		t.Fatalf("missing lookup = (%+v,%v)", missing, err)
	}
}

func TestResearchCacheServiceEnforcesLayerLimitsEvictsAndDetectsCorruption(t *testing.T) {
	t.Parallel()
	store := newMemoryCacheStore()
	clock := &cacheClock{now: cacheTimestamp(t, time.Date(2026, 8, 28, 10, 0, 0, 0, time.UTC))}
	service := application.NewResearchCacheService(store, clock)
	ctx := context.Background()
	metadataPolicy, _ := application.CachePolicyV1(application.CacheLayerFetchMetadata)
	if err := service.Put(ctx, application.CacheLayerFetchMetadata, "too-large", make([]byte, metadataPolicy.MaximumBytes+1)); !errors.Is(err, application.ErrInvalidState) {
		t.Fatalf("oversize Put() error = %v, want invalid_state", err)
	}
	for index := 0; index < application.MaximumResearchCacheItems+1; index++ {
		key := fmt.Sprintf("entry:%04d", index)
		if err := service.Put(ctx, application.CacheLayerSourceBundle, key, []byte(key)); err != nil {
			t.Fatal(err)
		}
		clock.now = cacheTimestamp(t, clock.now.Time().Add(time.Second))
	}
	status, err := service.Status(ctx)
	if err != nil || status.TotalEntries != application.MaximumResearchCacheItems {
		t.Fatalf("post-eviction status = (%+v,%v)", status, err)
	}
	if _, exists := store.records[cacheStoreKey(application.CacheLayerSourceBundle, "entry:0000")]; exists {
		t.Fatal("eviction retained the deterministic oldest entry")
	}

	key := cacheStoreKey(application.CacheLayerSourceBundle, "entry:0001")
	corrupt := store.records[key]
	corrupt.ContentHash = research.CanonicalContentHashV1([]byte("different"))
	store.records[key] = corrupt
	if _, err := service.Get(ctx, application.CacheLayerSourceBundle, "entry:0001", application.CacheReadOffline); !errors.Is(err, application.ErrPersistenceFailure) {
		t.Fatalf("corrupt Get() error = %v, want persistence_failure", err)
	}
}

type cacheClock struct{ now research.Timestamp }

func (clock *cacheClock) Now() research.Timestamp { return clock.now }

type memoryCacheStore struct {
	records map[string]application.CacheRecord
	corrupt int
}

func newMemoryCacheStore() *memoryCacheStore {
	return &memoryCacheStore{records: make(map[string]application.CacheRecord)}
}

func (store *memoryCacheStore) Put(_ context.Context, record application.CacheRecord) error {
	store.records[cacheStoreKey(record.Layer, record.Key)] = cloneCacheTestRecord(record)
	return nil
}

func (store *memoryCacheStore) Get(_ context.Context, layer application.CacheLayer, key string) (application.CacheRecord, error) {
	record, exists := store.records[cacheStoreKey(layer, key)]
	if !exists {
		return application.CacheRecord{}, application.Classify(application.ErrorNotFound, "memory cache get", errors.New("missing"))
	}
	return cloneCacheTestRecord(record), nil
}

func (store *memoryCacheStore) Delete(_ context.Context, layer application.CacheLayer, key string) error {
	delete(store.records, cacheStoreKey(layer, key))
	return nil
}

func (store *memoryCacheStore) Inspect(context.Context) (application.CacheInventory, error) {
	keys := make([]string, 0, len(store.records))
	for key := range store.records {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	records := make([]application.CacheRecord, 0, len(keys))
	for _, key := range keys {
		records = append(records, cloneCacheTestRecord(store.records[key]))
	}
	return application.CacheInventory{Records: records, CorruptEntries: store.corrupt}, nil
}

func (store *memoryCacheStore) Clear(context.Context) (application.ResearchCacheClearResult, error) {
	result := application.ResearchCacheClearResult{RemovedEntries: len(store.records)}
	for key, record := range store.records {
		result.RemovedBytes += int64(len(record.Payload))
		delete(store.records, key)
	}
	return result, nil
}

func cacheStoreKey(layer application.CacheLayer, key string) string {
	return string(layer) + "\x00" + key
}

func cloneCacheTestRecord(record application.CacheRecord) application.CacheRecord {
	record.Payload = append([]byte(nil), record.Payload...)
	return record
}

func cacheTimestamp(t *testing.T, value time.Time) research.Timestamp {
	t.Helper()
	timestamp, err := research.NewTimestamp(value.UTC())
	if err != nil {
		t.Fatal(err)
	}
	return timestamp
}

var _ application.ResearchCacheStore = (*memoryCacheStore)(nil)
