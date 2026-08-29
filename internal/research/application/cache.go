package application

import (
	"context"
	"errors"
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/mishaaac/kelyro/internal/research"
)

const (
	ResearchCacheAlgorithmV1     = "research-cache-v1"
	MaximumResearchCacheBytes    = 64 << 20
	MaximumResearchCacheItems    = 512
	MaximumResearchCacheKeyBytes = 1024
)

// ResearchCacheLimits configures eviction below the immutable hard ceilings.
// Lower limits are useful for storage-constrained or stricter workspaces.
type ResearchCacheLimits struct {
	MaximumBytes int
	MaximumItems int
}

func DefaultResearchCacheLimits() ResearchCacheLimits {
	return ResearchCacheLimits{MaximumBytes: MaximumResearchCacheBytes, MaximumItems: MaximumResearchCacheItems}
}

func (limits ResearchCacheLimits) Validate() error {
	if limits.MaximumBytes < 1 || limits.MaximumBytes > MaximumResearchCacheBytes {
		return fmt.Errorf("research cache byte limit must be between 1 and %d", MaximumResearchCacheBytes)
	}
	if limits.MaximumItems < 1 || limits.MaximumItems > MaximumResearchCacheItems {
		return fmt.Errorf("research cache item limit must be between 1 and %d", MaximumResearchCacheItems)
	}
	return nil
}

type CacheLayer string

const (
	CacheLayerDiscovery        CacheLayer = "discovery"
	CacheLayerFetchMetadata    CacheLayer = "fetch_metadata"
	CacheLayerBoundedSource    CacheLayer = "bounded_source"
	CacheLayerNormalizedSource CacheLayer = "normalized_source"
	CacheLayerSourceBundle     CacheLayer = "source_bundle"
)

var researchCacheLayers = []CacheLayer{
	CacheLayerDiscovery,
	CacheLayerFetchMetadata,
	CacheLayerBoundedSource,
	CacheLayerNormalizedSource,
	CacheLayerSourceBundle,
}

func ResearchCacheLayers() []CacheLayer {
	return append([]CacheLayer(nil), researchCacheLayers...)
}

func (layer CacheLayer) Validate() error {
	for _, candidate := range researchCacheLayers {
		if layer == candidate {
			return nil
		}
	}
	return fmt.Errorf("invalid research cache layer %q", layer)
}

type CacheLayerPolicy struct {
	Layer        CacheLayer
	TTL          time.Duration
	MaximumBytes int
}

func CachePolicyV1(layer CacheLayer) (CacheLayerPolicy, error) {
	if err := layer.Validate(); err != nil {
		return CacheLayerPolicy{}, err
	}
	policy := CacheLayerPolicy{Layer: layer}
	switch layer {
	case CacheLayerDiscovery:
		policy.TTL, policy.MaximumBytes = 24*time.Hour, 256<<10
	case CacheLayerFetchMetadata:
		policy.TTL, policy.MaximumBytes = 24*time.Hour, 32<<10
	case CacheLayerBoundedSource:
		policy.TTL, policy.MaximumBytes = 7*24*time.Hour, MaximumCachedSourceBodyBytes
	case CacheLayerNormalizedSource:
		policy.TTL, policy.MaximumBytes = 7*24*time.Hour, MaximumCachedSourceBodyBytes
	case CacheLayerSourceBundle:
		policy.TTL, policy.MaximumBytes = 30*24*time.Hour, research.MaximumSourceBundleJSONBytes
	default:
		panic("validated research cache layer is unreachable")
	}
	return policy, nil
}

type CacheRecord struct {
	Layer            CacheLayer
	Key              string
	Payload          []byte
	ContentHash      string
	StoredAt         research.Timestamp
	ExpiresAt        research.Timestamp
	AlgorithmVersion string
}

func (record CacheRecord) Validate() error {
	policy, err := CachePolicyV1(record.Layer)
	if err != nil {
		return err
	}
	if strings.TrimSpace(record.Key) == "" || record.Key != strings.TrimSpace(record.Key) {
		return fmt.Errorf("research cache key is invalid")
	}
	if len(record.Key) > MaximumResearchCacheKeyBytes {
		return fmt.Errorf("research cache key exceeds %d bytes", MaximumResearchCacheKeyBytes)
	}
	if len(record.Payload) == 0 || len(record.Payload) > policy.MaximumBytes {
		return fmt.Errorf("research cache payload must contain between 1 and %d bytes", policy.MaximumBytes)
	}
	if err := research.ValidateCanonicalContentHashV1(record.ContentHash); err != nil {
		return fmt.Errorf("research cache content hash: %w", err)
	}
	if record.ContentHash != research.CanonicalContentHashV1(record.Payload) {
		return fmt.Errorf("research cache content hash does not match payload")
	}
	if err := record.StoredAt.Validate(); err != nil {
		return fmt.Errorf("research cache stored at: %w", err)
	}
	if err := record.ExpiresAt.Validate(); err != nil {
		return fmt.Errorf("research cache expires at: %w", err)
	}
	if !record.ExpiresAt.After(record.StoredAt) {
		return fmt.Errorf("research cache expiry must follow storage")
	}
	if record.ExpiresAt.Time().Sub(record.StoredAt.Time()) != policy.TTL {
		return fmt.Errorf("research cache expiry does not match %s TTL", record.Layer)
	}
	if record.AlgorithmVersion != ResearchCacheAlgorithmV1 {
		return fmt.Errorf("invalid research cache algorithm %q", record.AlgorithmVersion)
	}
	return nil
}

func cloneCacheRecord(record CacheRecord) CacheRecord {
	clone := record
	clone.Payload = append([]byte(nil), record.Payload...)
	return clone
}

type CacheReadMode string

const (
	CacheReadFreshOnly CacheReadMode = "fresh_only"
	CacheReadOffline   CacheReadMode = "offline_allow_stale"
)

func (mode CacheReadMode) Validate() error {
	switch mode {
	case CacheReadFreshOnly, CacheReadOffline:
		return nil
	default:
		return fmt.Errorf("invalid research cache read mode %q", mode)
	}
}

type CacheWarning string

const CacheWarningStaleOffline CacheWarning = "stale_cache_used_offline"

type CacheLookup struct {
	Hit     bool
	Stale   bool
	Warning CacheWarning
	Record  CacheRecord
}

type CacheInventory struct {
	Records        []CacheRecord
	CorruptEntries int
	CorruptBytes   int64
}

type CacheLayerStatus struct {
	Layer        CacheLayer
	Entries      int
	PayloadBytes int64
	StaleEntries int
}

type ResearchCacheStatus struct {
	AlgorithmVersion  string
	Layers            []CacheLayerStatus
	TotalEntries      int
	TotalPayloadBytes int64
	StaleEntries      int
	CorruptEntries    int
	CorruptBytes      int64
}

type ResearchCacheClearResult struct {
	RemovedEntries int
	RemovedBytes   int64
}

// ResearchCacheStore owns only disposable cache records. Implementations must
// never expose persisted SourceSnapshots or Evidence through Clear.
type ResearchCacheStore interface {
	Put(context.Context, CacheRecord) error
	Get(context.Context, CacheLayer, string) (CacheRecord, error)
	Delete(context.Context, CacheLayer, string) error
	Inspect(context.Context) (CacheInventory, error)
	Clear(context.Context) (ResearchCacheClearResult, error)
}

type ResearchCacheService interface {
	Put(context.Context, CacheLayer, string, []byte) error
	Get(context.Context, CacheLayer, string, CacheReadMode) (CacheLookup, error)
	Status(context.Context) (ResearchCacheStatus, error)
	Clear(context.Context) (ResearchCacheClearResult, error)
}

type ResearchCacheServiceFactory interface {
	Open(context.Context, string) (ResearchCacheService, error)
}

type researchCacheService struct {
	store  ResearchCacheStore
	clock  Clock
	limits ResearchCacheLimits
}

func NewResearchCacheService(store ResearchCacheStore, clock Clock) ResearchCacheService {
	return &researchCacheService{store: store, clock: clock, limits: DefaultResearchCacheLimits()}
}

func NewResearchCacheServiceWithLimits(store ResearchCacheStore, clock Clock, limits ResearchCacheLimits) (ResearchCacheService, error) {
	if err := limits.Validate(); err != nil {
		return nil, err
	}
	return &researchCacheService{store: store, clock: clock, limits: limits}, nil
}

func (service *researchCacheService) Put(ctx context.Context, layer CacheLayer, key string, payload []byte) error {
	const operation = "put research cache"
	if err := requireDependency(operation, "research cache store", service.store); err != nil {
		return err
	}
	if err := requireDependency(operation, "research cache clock", service.clock); err != nil {
		return err
	}
	policy, err := CachePolicyV1(layer)
	if err != nil {
		return invalid(operation, err)
	}
	now := service.clock.Now()
	expires, err := research.NewTimestamp(now.Time().Add(policy.TTL))
	if err != nil {
		return invalid(operation, err)
	}
	record := CacheRecord{
		Layer: layer, Key: key, Payload: append([]byte(nil), payload...),
		ContentHash: research.CanonicalContentHashV1(payload), StoredAt: now,
		ExpiresAt: expires, AlgorithmVersion: ResearchCacheAlgorithmV1,
	}
	if err := record.Validate(); err != nil {
		return invalid(operation, err)
	}
	if err := service.store.Put(ctx, record); err != nil {
		return repositoryError(operation, err)
	}
	if err := service.evict(ctx, now); err != nil {
		return err
	}
	return nil
}

func (service *researchCacheService) Get(ctx context.Context, layer CacheLayer, key string, mode CacheReadMode) (CacheLookup, error) {
	const operation = "get research cache"
	if err := layer.Validate(); err != nil {
		return CacheLookup{}, invalid(operation, err)
	}
	if err := mode.Validate(); err != nil {
		return CacheLookup{}, invalid(operation, err)
	}
	if err := requireDependency(operation, "research cache store", service.store); err != nil {
		return CacheLookup{}, err
	}
	if err := requireDependency(operation, "research cache clock", service.clock); err != nil {
		return CacheLookup{}, err
	}
	record, err := service.store.Get(ctx, layer, key)
	if errors.Is(err, ErrNotFound) {
		return CacheLookup{Hit: false}, nil
	}
	if err != nil {
		return CacheLookup{}, repositoryError(operation, err)
	}
	if err := record.Validate(); err != nil {
		return CacheLookup{}, repositoryError(operation, err)
	}
	stale := !service.clock.Now().Before(record.ExpiresAt)
	if stale && mode == CacheReadFreshOnly {
		return CacheLookup{Hit: false, Stale: true}, nil
	}
	lookup := CacheLookup{Hit: true, Stale: stale, Record: cloneCacheRecord(record)}
	if stale {
		lookup.Warning = CacheWarningStaleOffline
	}
	return lookup, nil
}

func (service *researchCacheService) Status(ctx context.Context) (ResearchCacheStatus, error) {
	const operation = "inspect research cache"
	if err := requireDependency(operation, "research cache store", service.store); err != nil {
		return ResearchCacheStatus{}, err
	}
	if err := requireDependency(operation, "research cache clock", service.clock); err != nil {
		return ResearchCacheStatus{}, err
	}
	inventory, err := service.store.Inspect(ctx)
	if err != nil {
		return ResearchCacheStatus{}, repositoryError(operation, err)
	}
	status := ResearchCacheStatus{
		AlgorithmVersion: ResearchCacheAlgorithmV1, CorruptEntries: inventory.CorruptEntries,
		CorruptBytes: inventory.CorruptBytes, Layers: make([]CacheLayerStatus, 0, len(researchCacheLayers)),
	}
	byLayer := make(map[CacheLayer]*CacheLayerStatus, len(researchCacheLayers))
	for _, layer := range researchCacheLayers {
		status.Layers = append(status.Layers, CacheLayerStatus{Layer: layer})
		byLayer[layer] = &status.Layers[len(status.Layers)-1]
	}
	now := service.clock.Now()
	for _, record := range inventory.Records {
		if err := record.Validate(); err != nil {
			return ResearchCacheStatus{}, repositoryError(operation, err)
		}
		item := byLayer[record.Layer]
		item.Entries++
		item.PayloadBytes += int64(len(record.Payload))
		status.TotalEntries++
		status.TotalPayloadBytes += int64(len(record.Payload))
		if !now.Before(record.ExpiresAt) {
			item.StaleEntries++
			status.StaleEntries++
		}
	}
	return status, nil
}

func (service *researchCacheService) Clear(ctx context.Context) (ResearchCacheClearResult, error) {
	const operation = "clear research cache"
	if err := requireDependency(operation, "research cache store", service.store); err != nil {
		return ResearchCacheClearResult{}, err
	}
	result, err := service.store.Clear(ctx)
	if err != nil {
		return ResearchCacheClearResult{}, repositoryError(operation, err)
	}
	return result, nil
}

func (service *researchCacheService) evict(ctx context.Context, now research.Timestamp) error {
	const operation = "evict research cache"
	inventory, err := service.store.Inspect(ctx)
	if err != nil {
		return repositoryError(operation, err)
	}
	records := append([]CacheRecord(nil), inventory.Records...)
	sort.Slice(records, func(i, j int) bool {
		if records[i].StoredAt.Time().Equal(records[j].StoredAt.Time()) {
			if records[i].Layer == records[j].Layer {
				return records[i].Key < records[j].Key
			}
			return records[i].Layer < records[j].Layer
		}
		return records[i].StoredAt.Before(records[j].StoredAt)
	})
	totalBytes := 0
	for _, record := range records {
		totalBytes += len(record.Payload)
	}
	retained := records[:0]
	for _, record := range records {
		if !now.Before(record.ExpiresAt) {
			if err := service.store.Delete(ctx, record.Layer, record.Key); err != nil && !errors.Is(err, ErrNotFound) {
				return repositoryError(operation, err)
			}
			totalBytes -= len(record.Payload)
			continue
		}
		retained = append(retained, record)
	}
	records = retained
	for len(records) > service.limits.MaximumItems || totalBytes > service.limits.MaximumBytes {
		oldest := records[0]
		if err := service.store.Delete(ctx, oldest.Layer, oldest.Key); err != nil && !errors.Is(err, ErrNotFound) {
			return repositoryError(operation, err)
		}
		totalBytes -= len(oldest.Payload)
		records = records[1:]
	}
	return nil
}

var _ ResearchCacheService = (*researchCacheService)(nil)
