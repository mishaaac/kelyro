package packcatalogfs

import (
	"context"
	"os"
	"path/filepath"
	"reflect"
	"runtime"
	"testing"
	"time"

	"github.com/mishaaac/kelyro/internal/curriculum"
)

func TestCacheRoundTripsValidatedCatalogAndSupportsEmptyOfflineState(t *testing.T) {
	t.Parallel()
	path := filepath.Join(t.TempDir(), "cache", "catalog.json")
	cache := &Cache{path: func() (string, error) { return path, nil }}
	empty, err := cache.Load(context.Background())
	if err != nil || empty.SchemaVersion != curriculum.PackCatalogSchemaVersionV1 || len(empty.Entries) != 0 {
		t.Fatalf("empty Load() = %+v, %v", empty, err)
	}
	snapshot := filesystemCatalogFixture(t)
	if err := cache.Replace(context.Background(), snapshot); err != nil {
		t.Fatal(err)
	}
	loaded, err := cache.Load(context.Background())
	if err != nil || !reflect.DeepEqual(curriculum.SortPackCatalog(snapshot), loaded) {
		t.Fatalf("Load() = %+v, %v", loaded, err)
	}
	info, err := os.Stat(path)
	if err != nil || runtime.GOOS != "windows" && info.Mode().Perm()&0o077 != 0 {
		t.Fatalf("cache permissions = %v, %v", info.Mode(), err)
	}
}

func TestDocumentSourceRejectsUnknownFieldsAndTrailingJSON(t *testing.T) {
	t.Parallel()
	for name, encoded := range map[string]string{
		"unknown":  `{"schema_version":"pack-catalog/v1","generated_at":"2026-09-14T13:00:00Z","entries":[],"install":true}`,
		"trailing": `{"schema_version":"pack-catalog/v1","generated_at":"2026-09-14T13:00:00Z","entries":[]} {}`,
	} {
		t.Run(name, func(t *testing.T) {
			path := filepath.Join(t.TempDir(), "catalog.json")
			if err := os.WriteFile(path, []byte(encoded), 0o600); err != nil {
				t.Fatal(err)
			}
			if _, err := (DocumentSource{Path: path}).Load(context.Background()); err == nil {
				t.Fatal("invalid catalog accepted")
			}
		})
	}
}

func filesystemCatalogFixture(t *testing.T) curriculum.PackCatalogSnapshot {
	t.Helper()
	id := func(value string) curriculum.ID { result, _ := curriculum.NewID(value); return result }
	version := func(value string) curriculum.PackVersion {
		result, _ := curriculum.NewPackVersion(value)
		return result
	}
	generated, _ := curriculum.NewTimestamp(time.Date(2026, 9, 14, 13, 0, 0, 0, time.UTC))
	return curriculum.PackCatalogSnapshot{
		SchemaVersion: curriculum.PackCatalogSchemaVersionV1, GeneratedAt: generated,
		Entries: []curriculum.PackCatalogEntry{{
			PackID: id("go.backend"), Name: "Go Backend", Description: "Backend engineering.", Domain: "software", Target: "Backend engineer", Maintainer: "Kelyro",
			Versions: []curriculum.PackCatalogVersion{{Version: version("1.0.0"), Status: curriculum.ConceptCurrent, MinimumKelyroVersion: version("0.2.0"), Compatibility: curriculum.PackCompatible, Artifact: "https://packs.example/go.backend.zip"}},
			Source:   curriculum.PackCatalogSourceMetadata{ID: id("catalog.official"), Name: "Official", Locator: "https://packs.example/catalog.json", Trust: curriculum.PackCatalogOfficial},
		}},
	}
}
