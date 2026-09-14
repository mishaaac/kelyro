package application

import (
	"context"
	"errors"
	"reflect"
	"testing"
	"time"

	"github.com/mishaaac/kelyro/internal/curriculum"
)

func TestPackCatalogV1RefreshesCacheAndComputesCompatibility(t *testing.T) {
	t.Parallel()
	snapshot := catalogFixture(t)
	cache := &catalogCacheFake{}
	service := NewPackCatalogV1(catalogSourceFake{snapshot: snapshot}, cache, "v1.5.0")
	view, err := service.Catalog(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if view.Offline || view.AlgorithmVersion != PackCatalogAlgorithmVersionV1 || cache.replaced == nil {
		t.Fatalf("view=%+v cache=%+v", view, cache)
	}
	versions := view.Snapshot.Entries[0].Versions
	if versions[0].Compatibility != curriculum.PackCompatible || versions[1].Compatibility != curriculum.PackIncompatible {
		t.Fatalf("compatibility = %+v", versions)
	}

	repeated, err := service.Catalog(context.Background())
	if err != nil || !reflect.DeepEqual(view, repeated) {
		t.Fatalf("repeated catalog differs: %+v / %+v / %v", view, repeated, err)
	}
}

func TestPackCatalogV1FallsBackOfflineAndSearchesDeterministically(t *testing.T) {
	t.Parallel()
	snapshot := catalogFixture(t)
	service := NewPackCatalogV1(catalogSourceFake{err: errors.New("source offline")}, &catalogCacheFake{snapshot: snapshot}, "dev")
	view, err := service.Search(context.Background(), "backend go")
	if err != nil {
		t.Fatal(err)
	}
	if !view.Offline || view.SourceWarning != "source offline" || len(view.Snapshot.Entries) != 1 || view.Snapshot.Entries[0].PackID.String() != "go.backend" {
		t.Fatalf("search view = %+v", view)
	}
	for _, version := range view.Snapshot.Entries[0].Versions {
		if version.Compatibility != curriculum.PackCompatibilityUnknown {
			t.Fatalf("development compatibility = %q", version.Compatibility)
		}
	}
	empty, err := service.Search(context.Background(), "does-not-exist")
	if err != nil || len(empty.Snapshot.Entries) != 0 {
		t.Fatalf("empty search = %+v, %v", empty, err)
	}
}

type catalogSourceFake struct {
	snapshot curriculum.PackCatalogSnapshot
	err      error
}

func (fake catalogSourceFake) Load(context.Context) (curriculum.PackCatalogSnapshot, error) {
	return fake.snapshot, fake.err
}

type catalogCacheFake struct {
	snapshot curriculum.PackCatalogSnapshot
	replaced *curriculum.PackCatalogSnapshot
}

func (fake *catalogCacheFake) Load(context.Context) (curriculum.PackCatalogSnapshot, error) {
	return fake.snapshot, nil
}
func (fake *catalogCacheFake) Replace(_ context.Context, snapshot curriculum.PackCatalogSnapshot) error {
	fake.snapshot = snapshot
	fake.replaced = &snapshot
	return nil
}

func catalogFixture(t *testing.T) curriculum.PackCatalogSnapshot {
	t.Helper()
	generated, _ := curriculum.NewTimestamp(time.Date(2026, 9, 14, 13, 0, 0, 0, time.UTC))
	source := curriculum.PackCatalogSourceMetadata{ID: curriculumID(t, "catalog.official"), Name: "Official catalog", Locator: "https://packs.kelyro.example/catalog.json", Trust: curriculum.PackCatalogOfficial}
	return curriculum.PackCatalogSnapshot{
		SchemaVersion: curriculum.PackCatalogSchemaVersionV1, GeneratedAt: generated,
		Entries: []curriculum.PackCatalogEntry{
			{PackID: curriculumID(t, "go.backend"), Name: "Go Backend", Description: "Backend engineering with Go.", Domain: "software-engineering", Target: "Backend engineer", Maintainer: "Kelyro", Source: source, Versions: []curriculum.PackCatalogVersion{
				{Version: packVersion(t, "1.0.0"), Status: curriculum.ConceptCurrent, MinimumKelyroVersion: packVersion(t, "1.0.0"), Compatibility: curriculum.PackCompatibilityUnknown, Artifact: "https://packs.kelyro.example/go.backend/1.0.0.zip"},
				{Version: packVersion(t, "2.0.0"), Status: curriculum.ConceptPreview, MinimumKelyroVersion: packVersion(t, "2.0.0"), Compatibility: curriculum.PackCompatibilityUnknown, Artifact: "https://packs.kelyro.example/go.backend/2.0.0.zip"},
			}},
			{PackID: curriculumID(t, "python.data"), Name: "Python Data", Description: "Data engineering with Python.", Domain: "data", Target: "Data engineer", Maintainer: "Community", Source: curriculum.PackCatalogSourceMetadata{ID: curriculumID(t, "catalog.community"), Name: "Community catalog", Locator: "local:community", Trust: curriculum.PackCatalogCommunity}, Versions: []curriculum.PackCatalogVersion{
				{Version: packVersion(t, "1.0.0"), Status: curriculum.ConceptCurrent, MinimumKelyroVersion: packVersion(t, "1.0.0"), Compatibility: curriculum.PackCompatibilityUnknown, Artifact: "https://packs.example/python.data/1.0.0.zip"},
			}},
		},
	}
}
