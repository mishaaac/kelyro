package updatecache

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"

	"github.com/mishaaac/kelyro/internal/update"
)

func TestStoreSavesLoadsChannelsAndUsesRestrictivePermissions(t *testing.T) {
	t.Parallel()
	store, path := testStore(t)
	now := time.Date(2026, 8, 12, 18, 0, 0, 0, time.UTC)
	stable := update.CachedCheck{
		Channel: update.Stable, CheckedAt: now, Found: true,
		Release: update.Release{Version: "1.2.3", URL: "https://example.invalid/stable", PublishedAt: now.Add(-time.Hour)},
	}
	prerelease := update.CachedCheck{Channel: update.Prerelease, CheckedAt: now, Found: false}
	if err := store.Save(context.Background(), stable); err != nil {
		t.Fatalf("Save(stable): %v", err)
	}
	if err := store.Save(context.Background(), prerelease); err != nil {
		t.Fatalf("Save(prerelease): %v", err)
	}
	for channel, want := range map[update.Channel]update.CachedCheck{update.Stable: stable, update.Prerelease: prerelease} {
		got, found, err := store.Load(context.Background(), channel)
		if err != nil || !found || got.Channel != want.Channel || got.Found != want.Found || got.Release.Version != want.Release.Version || !got.CheckedAt.Equal(want.CheckedAt) {
			t.Fatalf("Load(%s) = %+v, %v, %v; want %+v", channel, got, found, err, want)
		}
	}
	info, err := os.Stat(path)
	if err != nil {
		t.Fatalf("Stat(cache): %v", err)
	}
	if runtime.GOOS != "windows" && info.Mode().Perm() != 0o600 {
		t.Errorf("cache permissions = %o, want 600", info.Mode().Perm())
	}
}

func TestStoreMissingAndMalformedCache(t *testing.T) {
	t.Parallel()
	store, path := testStore(t)
	if _, found, err := store.Load(context.Background(), update.Stable); err != nil || found {
		t.Fatalf("Load(missing) found=%v error=%v", found, err)
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		t.Fatalf("MkdirAll(): %v", err)
	}
	if err := os.WriteFile(path, []byte(`{"schema_version":1,"checks":{"stable":{"channel":"stable"}}}`), 0o600); err != nil {
		t.Fatalf("WriteFile(): %v", err)
	}
	if _, _, err := store.Load(context.Background(), update.Stable); err == nil || !strings.Contains(err.Error(), "timestamp is empty") {
		t.Fatalf("Load(malformed) error = %v", err)
	}
}

func TestStoreAtomicFailurePreservesPreviousCache(t *testing.T) {
	t.Parallel()
	store, path := testStore(t)
	initial := update.CachedCheck{Channel: update.Stable, CheckedAt: time.Now().UTC(), Found: true, Release: update.Release{Version: "1.0.0"}}
	if err := store.Save(context.Background(), initial); err != nil {
		t.Fatalf("Save(initial): %v", err)
	}
	want, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("ReadFile(initial): %v", err)
	}
	wantErr := errors.New("rename failed")
	store.rename = func(string, string) error { return wantErr }
	err = store.Save(context.Background(), update.CachedCheck{Channel: update.Stable, CheckedAt: time.Now().UTC(), Found: false})
	if !errors.Is(err, wantErr) {
		t.Fatalf("Save(failure) error = %v", err)
	}
	got, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("ReadFile(after failure): %v", err)
	}
	if string(got) != string(want) {
		t.Fatal("failed atomic save changed existing cache")
	}
	temporary, err := filepath.Glob(filepath.Join(filepath.Dir(path), ".updates-*.tmp"))
	if err != nil || len(temporary) != 0 {
		t.Fatalf("staging files = %v, error = %v", temporary, err)
	}
}

func testStore(t *testing.T) (*Store, string) {
	t.Helper()
	path := filepath.Join(t.TempDir(), "cache with spaces", "updates.json")
	store := New()
	store.path = func() (string, error) { return path, nil }
	return store, path
}
