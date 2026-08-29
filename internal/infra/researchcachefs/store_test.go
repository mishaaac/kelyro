package researchcachefs

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"

	"github.com/mishaaac/kelyro/internal/platform"
	"github.com/mishaaac/kelyro/internal/research"
	"github.com/mishaaac/kelyro/internal/research/application"
	"github.com/mishaaac/kelyro/internal/storage/sqlite"
)

func TestFactoryUsesFoundationCacheDirectoryAndRoundTripsLayers(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	clock := &testClock{now: fsTimestamp(t, time.Date(2026, 8, 28, 10, 0, 0, 0, time.UTC))}
	service, err := NewFactory().WithClock(clock).Open(context.Background(), root)
	if err != nil {
		t.Fatal(err)
	}
	for _, item := range []struct {
		layer   application.CacheLayer
		key     string
		payload string
	}{
		{application.CacheLayerDiscovery, "query:docs", `[{"title":"Docs"}]`},
		{application.CacheLayerFetchMetadata, "fetch:metadata", `{"etag":"v1"}`},
		{application.CacheLayerBoundedSource, "fetch:body", "bounded body"},
		{application.CacheLayerNormalizedSource, "normalized:docs", "normalized body"},
		{application.CacheLayerSourceBundle, "bundle:topic", `{"bundle":"topic"}`},
	} {
		if err := service.Put(context.Background(), item.layer, item.key, []byte(item.payload)); err != nil {
			t.Fatal(err)
		}
		lookup, err := service.Get(context.Background(), item.layer, item.key, application.CacheReadOffline)
		if err != nil || !lookup.Hit || string(lookup.Record.Payload) != item.payload {
			t.Fatalf("lookup %s = (%+v,%v)", item.layer, lookup, err)
		}
	}
	status, err := service.Status(context.Background())
	if err != nil || status.TotalEntries != 5 || len(status.Layers) != 5 || status.CorruptEntries != 0 {
		t.Fatalf("status = (%+v,%v)", status, err)
	}
	directory, _ := platform.WorkspaceResearchCacheDir(root)
	info, err := os.Stat(filepath.Join(directory, string(application.CacheLayerDiscovery)))
	if err != nil || !info.IsDir() {
		t.Fatalf("Foundation research cache directory = (%+v,%v)", info, err)
	}
	entries, _ := os.ReadDir(filepath.Join(directory, string(application.CacheLayerDiscovery)))
	fileInfo, err := entries[0].Info()
	if err != nil {
		t.Fatal(err)
	}
	if runtime.GOOS != "windows" && fileInfo.Mode().Perm() != 0o600 {
		t.Fatalf("cache permissions = %o, want 600", fileInfo.Mode().Perm())
	}
}

func TestStoreDetectsCorruptionAndClearPreservesDurableWorkspaceData(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	internal, _ := platform.WorkspaceInternalDir(root)
	if err := os.MkdirAll(internal, 0o700); err != nil {
		t.Fatal(err)
	}
	database, err := sqlite.Open(context.Background(), root, sqlite.WithAppVersion("test"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = database.Close() })
	repositories := database.Repositories().Research
	observed := fsTimestamp(t, time.Date(2026, 8, 28, 9, 0, 0, 0, time.UTC))
	sourceID, _ := research.NewSourceID("source.cache-preservation")
	snapshotID, _ := research.NewID("snapshot.cache-preservation")
	evidenceID, _ := research.NewID("evidence.cache-preservation")
	locator, _ := research.NewSourceLocator("https://example.test/cache-preservation")
	source := research.Source{
		ID: sourceID, Kind: research.SourceOfficialDocumentation, Locator: locator,
		TemporalScope: research.SourceTemporalCurrent, Metadata: research.SourceMetadata{Title: "Durable source"}, CreatedAt: observed,
	}
	if err := repositories.Sources.Create(context.Background(), source); err != nil {
		t.Fatal(err)
	}
	snapshot := research.SourceSnapshot{
		ID: snapshotID, SourceID: sourceID, Locator: locator, FetchedAt: observed,
		Fetch: research.FetchMetadata{StatusCode: 200, ContentType: "text/plain", ContentHash: "sha256:durable", ContentLength: 7, FetchVersion: "fetch/v1"},
	}
	if err := repositories.Snapshots.Append(context.Background(), snapshot); err != nil {
		t.Fatal(err)
	}
	excerpt := "durable"
	evidence := research.Evidence{
		ID: evidenceID, SourceID: sourceID, SnapshotID: snapshotID, Location: "line 1",
		Excerpt: excerpt, ExcerptHash: research.CanonicalEvidenceExcerptHashV1(excerpt), ExtractedAt: observed, ExtractorVersion: "extract/v1",
	}
	if err := repositories.Evidence.Append(context.Background(), evidence); err != nil {
		t.Fatal(err)
	}
	cacheRoot, _ := platform.WorkspaceCacheDir(root)
	otherCache := filepath.Join(cacheRoot, "other-component.cache")
	if err := os.MkdirAll(cacheRoot, 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(otherCache, []byte("other cache"), 0o600); err != nil {
		t.Fatal(err)
	}
	clock := &testClock{now: fsTimestamp(t, time.Date(2026, 8, 28, 10, 0, 0, 0, time.UTC))}
	service, err := NewFactory().WithClock(clock).Open(context.Background(), root)
	if err != nil {
		t.Fatal(err)
	}
	if err := service.Put(context.Background(), application.CacheLayerDiscovery, "query:corrupt", []byte("valid first")); err != nil {
		t.Fatal(err)
	}
	researchRoot, _ := platform.WorkspaceResearchCacheDir(root)
	layerDirectory := filepath.Join(researchRoot, string(application.CacheLayerDiscovery))
	entries, err := os.ReadDir(layerDirectory)
	if err != nil || len(entries) != 1 {
		t.Fatalf("cache entries = (%v,%v)", entries, err)
	}
	path := filepath.Join(layerDirectory, entries[0].Name())
	if err := os.WriteFile(path, []byte(`{"schema_version":1,"payload":"tampered"}`), 0o600); err != nil {
		t.Fatal(err)
	}
	status, err := service.Status(context.Background())
	if err != nil || status.CorruptEntries != 1 || status.TotalEntries != 0 {
		t.Fatalf("corrupt status = (%+v,%v)", status, err)
	}
	if _, err := service.Get(context.Background(), application.CacheLayerDiscovery, "query:corrupt", application.CacheReadOffline); !errors.Is(err, application.ErrPersistenceFailure) {
		t.Fatalf("corrupt Get() error = %v, want persistence_failure", err)
	}
	cleared, err := service.Clear(context.Background())
	if err != nil || cleared.RemovedEntries != 1 {
		t.Fatalf("Clear() = (%+v,%v)", cleared, err)
	}
	if got, getErr := repositories.Snapshots.Get(context.Background(), snapshotID); getErr != nil || got.ID != snapshotID {
		t.Fatalf("snapshot after cache clear = (%+v,%v)", got, getErr)
	}
	if got, getErr := repositories.Evidence.Get(context.Background(), evidenceID); getErr != nil || got.ID != evidenceID || got.Excerpt != excerpt {
		t.Fatalf("evidence after cache clear = (%+v,%v)", got, getErr)
	}
	content, readErr := os.ReadFile(otherCache)
	if readErr != nil || string(content) != "other cache" {
		t.Fatalf("other component cache = (%q,%v)", content, readErr)
	}
	if _, err := os.Stat(researchRoot); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("research cache root still exists or stat failed: %v", err)
	}
}

func TestStoreRejectsTamperedContentHash(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	clock := &testClock{now: fsTimestamp(t, time.Date(2026, 8, 28, 10, 0, 0, 0, time.UTC))}
	service, _ := NewFactory().WithClock(clock).Open(context.Background(), root)
	if err := service.Put(context.Background(), application.CacheLayerBoundedSource, "source:one", []byte("bounded source")); err != nil {
		t.Fatal(err)
	}
	researchRoot, _ := platform.WorkspaceResearchCacheDir(root)
	directory := filepath.Join(researchRoot, string(application.CacheLayerBoundedSource))
	entries, _ := os.ReadDir(directory)
	path := filepath.Join(directory, entries[0].Name())
	encoded, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	tampered := strings.Replace(string(encoded), "sha256:", "sha256:0", 1)
	if err := os.WriteFile(path, []byte(tampered), 0o600); err != nil {
		t.Fatal(err)
	}
	status, err := service.Status(context.Background())
	if err != nil || status.CorruptEntries != 1 {
		t.Fatalf("tampered hash status = (%+v,%v)", status, err)
	}
}

func TestStoreRejectsSymlinkedCacheComponentsAndRecords(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("symlink creation may require elevated Windows privileges")
	}
	ctx := context.Background()
	t.Run("parent component", func(t *testing.T) {
		root := t.TempDir()
		outside := t.TempDir()
		internal, _ := platform.WorkspaceInternalDir(root)
		if err := os.MkdirAll(internal, 0o700); err != nil {
			t.Fatal(err)
		}
		cacheRoot, _ := platform.WorkspaceCacheDir(root)
		if err := os.Symlink(outside, cacheRoot); err != nil {
			t.Fatal(err)
		}
		service, err := NewFactory().Open(ctx, root)
		if err != nil {
			t.Fatal(err)
		}
		if err := service.Put(ctx, application.CacheLayerDiscovery, "query:escape", []byte("must stay inside")); err == nil {
			t.Fatal("cache write followed a symlinked parent")
		}
		entries, err := os.ReadDir(outside)
		if err != nil || len(entries) != 0 {
			t.Fatalf("outside directory was modified: entries=%v err=%v", entries, err)
		}
	})

	t.Run("record", func(t *testing.T) {
		root := t.TempDir()
		clock := &testClock{now: fsTimestamp(t, time.Date(2026, 8, 28, 10, 0, 0, 0, time.UTC))}
		service, err := NewFactory().WithClock(clock).Open(ctx, root)
		if err != nil {
			t.Fatal(err)
		}
		if err := service.Put(ctx, application.CacheLayerDiscovery, "query:symlink", []byte("safe")); err != nil {
			t.Fatal(err)
		}
		researchRoot, _ := platform.WorkspaceResearchCacheDir(root)
		directory := filepath.Join(researchRoot, string(application.CacheLayerDiscovery))
		entries, _ := os.ReadDir(directory)
		path := filepath.Join(directory, entries[0].Name())
		outside := filepath.Join(t.TempDir(), "outside.json")
		if err := os.WriteFile(outside, []byte(`{"payload":"secret"}`), 0o600); err != nil {
			t.Fatal(err)
		}
		if err := os.Remove(path); err != nil {
			t.Fatal(err)
		}
		if err := os.Symlink(outside, path); err != nil {
			t.Fatal(err)
		}
		if _, err := service.Get(ctx, application.CacheLayerDiscovery, "query:symlink", application.CacheReadOffline); err == nil {
			t.Fatal("cache read followed a symlinked record")
		}
		status, err := service.Status(ctx)
		if err != nil || status.CorruptEntries != 1 || status.TotalEntries != 0 {
			t.Fatalf("symlink status = (%+v, %v)", status, err)
		}
	})
}

func FuzzStoreRecordPathStaysWithinWorkspace(f *testing.F) {
	for _, seed := range []string{"normal", "../escape", `..\\escape`, "/absolute", "nul\x00name", strings.Repeat("x", 2048)} {
		f.Add(seed)
	}
	f.Fuzz(func(t *testing.T, key string) {
		boundary := t.TempDir()
		root := filepath.Join(boundary, ".kelyro", "cache", "research")
		store := newStore(boundary, root)
		path, err := store.recordPath(application.CacheLayerDiscovery, key)
		if err != nil {
			return
		}
		relative, err := filepath.Rel(boundary, path)
		if err != nil || relative == ".." || strings.HasPrefix(relative, ".."+string(filepath.Separator)) {
			t.Fatalf("record path escaped workspace: key=%q path=%q", key, path)
		}
		name := strings.TrimSuffix(filepath.Base(path), ".json")
		if len(name) != 64 {
			t.Fatalf("record filename is not a SHA-256 digest: %q", name)
		}
		for _, char := range name {
			if !strings.ContainsRune("0123456789abcdef", char) {
				t.Fatalf("record filename contains unsafe character %q", char)
			}
		}
	})
}

type testClock struct{ now research.Timestamp }

func (clock *testClock) Now() research.Timestamp { return clock.now }

func fsTimestamp(t *testing.T, value time.Time) research.Timestamp {
	t.Helper()
	timestamp, err := research.NewTimestamp(value.UTC())
	if err != nil {
		t.Fatal(err)
	}
	return timestamp
}
