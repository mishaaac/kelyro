// Package researchcachefs stores disposable Research cache records below the
// Foundation workspace cache directory.
package researchcachefs

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/mishaaac/kelyro/internal/platform"
	"github.com/mishaaac/kelyro/internal/research"
	"github.com/mishaaac/kelyro/internal/research/application"
)

const (
	cacheSchemaVersion   = 1
	maximumEnvelopeBytes = (application.MaximumCachedSourceBodyBytes * 4 / 3) + (64 << 10)
)

type Factory struct {
	clock  application.Clock
	limits application.ResearchCacheLimits
}

func NewFactory() *Factory {
	return &Factory{clock: systemClock{}, limits: application.DefaultResearchCacheLimits()}
}

func (factory *Factory) WithClock(clock application.Clock) *Factory {
	factory.clock = clock
	return factory
}

func (factory *Factory) WithLimits(limits application.ResearchCacheLimits) *Factory {
	factory.limits = limits
	return factory
}

func (factory *Factory) Open(ctx context.Context, root string) (application.ResearchCacheService, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	cacheRoot, err := platform.WorkspaceResearchCacheDir(root)
	if err != nil {
		return nil, fmt.Errorf("resolve research cache directory: %w", err)
	}
	service, err := application.NewResearchCacheServiceWithLimits(newStore(cacheRoot), factory.clock, factory.limits)
	if err != nil {
		return nil, fmt.Errorf("configure research cache retention: %w", err)
	}
	return service, nil
}

type systemClock struct{}

func (systemClock) Now() research.Timestamp {
	timestamp, err := research.NewTimestamp(time.Now().UTC())
	if err != nil {
		panic(err)
	}
	return timestamp
}

type Store struct {
	root string
	mu   sync.Mutex
}

func newStore(root string) *Store { return &Store{root: root} }

type envelope struct {
	SchemaVersion    int                    `json:"schema_version"`
	AlgorithmVersion string                 `json:"algorithm_version"`
	Layer            application.CacheLayer `json:"layer"`
	Key              string                 `json:"key"`
	Payload          []byte                 `json:"payload"`
	ContentHash      string                 `json:"content_hash"`
	StoredAt         string                 `json:"stored_at"`
	ExpiresAt        string                 `json:"expires_at"`
}

func (store *Store) Put(ctx context.Context, record application.CacheRecord) error {
	const operation = "write filesystem research cache"
	if err := contextError(operation, ctx); err != nil {
		return err
	}
	if err := record.Validate(); err != nil {
		return invalid(operation, err)
	}
	store.mu.Lock()
	defer store.mu.Unlock()
	path, err := store.recordPath(record.Layer, record.Key)
	if err != nil {
		return invalid(operation, err)
	}
	encoded, err := encodeRecord(record)
	if err != nil {
		return invalid(operation, err)
	}
	if err := writeAtomic(path, encoded); err != nil {
		return persistence(operation, err)
	}
	return nil
}

func (store *Store) Get(ctx context.Context, layer application.CacheLayer, key string) (application.CacheRecord, error) {
	const operation = "read filesystem research cache"
	if err := contextError(operation, ctx); err != nil {
		return application.CacheRecord{}, err
	}
	store.mu.Lock()
	defer store.mu.Unlock()
	path, err := store.recordPath(layer, key)
	if err != nil {
		return application.CacheRecord{}, invalid(operation, err)
	}
	record, err := readRecord(path)
	if errors.Is(err, fs.ErrNotExist) {
		return application.CacheRecord{}, notFound(operation)
	}
	if err != nil {
		return application.CacheRecord{}, persistence(operation, err)
	}
	if record.Layer != layer || record.Key != key {
		return application.CacheRecord{}, persistence(operation, errors.New("cache identity does not match requested record"))
	}
	return cloneRecord(record), nil
}

func (store *Store) Delete(ctx context.Context, layer application.CacheLayer, key string) error {
	const operation = "delete filesystem research cache"
	if err := contextError(operation, ctx); err != nil {
		return err
	}
	store.mu.Lock()
	defer store.mu.Unlock()
	path, err := store.recordPath(layer, key)
	if err != nil {
		return invalid(operation, err)
	}
	if err := os.Remove(path); errors.Is(err, fs.ErrNotExist) {
		return notFound(operation)
	} else if err != nil {
		return persistence(operation, err)
	}
	return nil
}

func (store *Store) Inspect(ctx context.Context) (application.CacheInventory, error) {
	const operation = "inspect filesystem research cache"
	if err := contextError(operation, ctx); err != nil {
		return application.CacheInventory{}, err
	}
	store.mu.Lock()
	defer store.mu.Unlock()
	inventory := application.CacheInventory{Records: make([]application.CacheRecord, 0)}
	for _, layer := range application.ResearchCacheLayers() {
		if err := ctx.Err(); err != nil {
			return application.CacheInventory{}, application.Classify(application.ErrorUnavailable, operation, err)
		}
		directory := filepath.Join(store.root, string(layer))
		entries, err := os.ReadDir(directory)
		if errors.Is(err, fs.ErrNotExist) {
			continue
		}
		if err != nil {
			return application.CacheInventory{}, persistence(operation, err)
		}
		for _, entry := range entries {
			if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".json") {
				continue
			}
			path := filepath.Join(directory, entry.Name())
			info, infoErr := entry.Info()
			if infoErr != nil {
				return application.CacheInventory{}, persistence(operation, infoErr)
			}
			record, readErr := readRecord(path)
			if readErr != nil || filepath.Base(store.pathFor(record.Layer, record.Key)) != entry.Name() || record.Layer != layer {
				inventory.CorruptEntries++
				inventory.CorruptBytes += info.Size()
				continue
			}
			inventory.Records = append(inventory.Records, cloneRecord(record))
		}
	}
	return inventory, nil
}

func (store *Store) Clear(ctx context.Context) (application.ResearchCacheClearResult, error) {
	const operation = "clear filesystem research cache"
	if err := contextError(operation, ctx); err != nil {
		return application.ResearchCacheClearResult{}, err
	}
	store.mu.Lock()
	defer store.mu.Unlock()
	result := application.ResearchCacheClearResult{}
	for _, layer := range application.ResearchCacheLayers() {
		directory := filepath.Join(store.root, string(layer))
		entries, err := os.ReadDir(directory)
		if errors.Is(err, fs.ErrNotExist) {
			continue
		}
		if err != nil {
			return application.ResearchCacheClearResult{}, persistence(operation, err)
		}
		for _, entry := range entries {
			if entry.IsDir() || (!strings.HasSuffix(entry.Name(), ".json") && !strings.Contains(entry.Name(), ".tmp")) {
				continue
			}
			if err := ctx.Err(); err != nil {
				return application.ResearchCacheClearResult{}, application.Classify(application.ErrorUnavailable, operation, err)
			}
			info, infoErr := entry.Info()
			if infoErr != nil {
				return application.ResearchCacheClearResult{}, persistence(operation, infoErr)
			}
			if err := os.Remove(filepath.Join(directory, entry.Name())); err != nil && !errors.Is(err, fs.ErrNotExist) {
				return application.ResearchCacheClearResult{}, persistence(operation, err)
			}
			result.RemovedEntries++
			result.RemovedBytes += info.Size()
		}
		_ = os.Remove(directory)
	}
	_ = os.Remove(store.root)
	return result, nil
}

func (store *Store) recordPath(layer application.CacheLayer, key string) (string, error) {
	if err := layer.Validate(); err != nil {
		return "", err
	}
	if strings.TrimSpace(key) == "" || key != strings.TrimSpace(key) || len(key) > application.MaximumResearchCacheKeyBytes {
		return "", errors.New("research cache key is invalid")
	}
	return store.pathFor(layer, key), nil
}

func (store *Store) pathFor(layer application.CacheLayer, key string) string {
	digest := sha256.Sum256([]byte(string(layer) + "\x00" + key))
	return filepath.Join(store.root, string(layer), hex.EncodeToString(digest[:])+".json")
}

func encodeRecord(record application.CacheRecord) ([]byte, error) {
	doc := envelope{
		SchemaVersion: cacheSchemaVersion, AlgorithmVersion: record.AlgorithmVersion,
		Layer: record.Layer, Key: record.Key, Payload: record.Payload, ContentHash: record.ContentHash,
		StoredAt: record.StoredAt.Time().Format(time.RFC3339Nano), ExpiresAt: record.ExpiresAt.Time().Format(time.RFC3339Nano),
	}
	encoded, err := json.Marshal(doc)
	if err != nil {
		return nil, fmt.Errorf("encode research cache: %w", err)
	}
	if len(encoded) > maximumEnvelopeBytes {
		return nil, fmt.Errorf("research cache envelope exceeds %d bytes", maximumEnvelopeBytes)
	}
	return append(encoded, '\n'), nil
}

func readRecord(path string) (application.CacheRecord, error) {
	encoded, err := os.ReadFile(path)
	if err != nil {
		return application.CacheRecord{}, err
	}
	if len(encoded) == 0 || len(encoded) > maximumEnvelopeBytes {
		return application.CacheRecord{}, fmt.Errorf("research cache envelope size is invalid")
	}
	decoder := json.NewDecoder(bytes.NewReader(encoded))
	decoder.DisallowUnknownFields()
	var doc envelope
	if err := decoder.Decode(&doc); err != nil {
		return application.CacheRecord{}, fmt.Errorf("decode research cache: %w", err)
	}
	if err := ensureEOF(decoder); err != nil {
		return application.CacheRecord{}, err
	}
	if doc.SchemaVersion != cacheSchemaVersion {
		return application.CacheRecord{}, fmt.Errorf("unsupported research cache schema %d", doc.SchemaVersion)
	}
	storedAt, err := parseTimestamp(doc.StoredAt)
	if err != nil {
		return application.CacheRecord{}, fmt.Errorf("research cache stored_at: %w", err)
	}
	expiresAt, err := parseTimestamp(doc.ExpiresAt)
	if err != nil {
		return application.CacheRecord{}, fmt.Errorf("research cache expires_at: %w", err)
	}
	record := application.CacheRecord{
		Layer: doc.Layer, Key: doc.Key, Payload: append([]byte(nil), doc.Payload...),
		ContentHash: doc.ContentHash, StoredAt: storedAt, ExpiresAt: expiresAt,
		AlgorithmVersion: doc.AlgorithmVersion,
	}
	if err := record.Validate(); err != nil {
		return application.CacheRecord{}, err
	}
	return record, nil
}

func parseTimestamp(value string) (research.Timestamp, error) {
	parsed, err := time.Parse(time.RFC3339Nano, value)
	if err != nil {
		return research.Timestamp{}, err
	}
	return research.NewTimestamp(parsed.UTC())
}

func ensureEOF(decoder *json.Decoder) error {
	var extra any
	if err := decoder.Decode(&extra); !errors.Is(err, io.EOF) {
		if err == nil {
			return errors.New("research cache contains trailing JSON")
		}
		return err
	}
	return nil
}

func writeAtomic(path string, encoded []byte) (err error) {
	directory := filepath.Dir(path)
	if err := os.MkdirAll(directory, 0o700); err != nil {
		return err
	}
	temporary, err := os.CreateTemp(directory, ".research-cache-*.tmp")
	if err != nil {
		return err
	}
	temporaryPath := temporary.Name()
	defer func() {
		_ = temporary.Close()
		_ = os.Remove(temporaryPath)
	}()
	if err := temporary.Chmod(0o600); err != nil {
		return err
	}
	if _, err := temporary.Write(encoded); err != nil {
		return err
	}
	if err := temporary.Sync(); err != nil {
		return err
	}
	if err := temporary.Close(); err != nil {
		return err
	}
	return replaceFile(temporaryPath, path)
}

func cloneRecord(record application.CacheRecord) application.CacheRecord {
	clone := record
	clone.Payload = append([]byte(nil), record.Payload...)
	return clone
}

func contextError(operation string, ctx context.Context) error {
	if ctx == nil {
		return invalid(operation, errors.New("context is nil"))
	}
	if err := ctx.Err(); err != nil {
		return application.Classify(application.ErrorUnavailable, operation, err)
	}
	return nil
}

func invalid(operation string, err error) error {
	return application.Classify(application.ErrorInvalidState, operation, err)
}

func notFound(operation string) error {
	return application.Classify(application.ErrorNotFound, operation, fs.ErrNotExist)
}

func persistence(operation string, err error) error {
	return application.Classify(application.ErrorPersistenceFailure, operation, err)
}

var (
	_ application.ResearchCacheStore          = (*Store)(nil)
	_ application.ResearchCacheServiceFactory = (*Factory)(nil)
)
