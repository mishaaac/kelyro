package researchdb

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/mishaaac/kelyro/internal/research"
)

func TestFactoryPersistsSourceRegistryAcrossStoreLifetimes(t *testing.T) {
	ctx := context.Background()
	root := t.TempDir()
	if err := os.Mkdir(filepath.Join(root, ".kelyro"), 0o700); err != nil {
		t.Fatal(err)
	}
	factory := NewFactory("test")
	store, err := factory.Open(ctx, root)
	if err != nil {
		t.Fatal(err)
	}
	entry := researchDBEntry(t)
	if err := store.Registry().Save(ctx, entry); err != nil {
		t.Fatal(err)
	}
	if err := store.Close(); err != nil {
		t.Fatal(err)
	}
	reopened, err := factory.Open(ctx, root)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = reopened.Close() })
	loaded, err := reopened.Registry().Get(ctx, entry.ID)
	if err != nil || loaded.Status != research.RegistryHistorical || loaded.CanonicalDomains[0].String() != "archive.example" {
		t.Fatalf("reopened registry entry = (%+v, %v)", loaded, err)
	}
}

func researchDBEntry(t *testing.T) research.SourceRegistryEntry {
	t.Helper()
	id, _ := research.NewID("registry.archive")
	domain, _ := research.NewCanonicalDomain("archive.example")
	at, _ := research.NewTimestamp(time.Date(2026, 8, 24, 10, 0, 0, 0, time.UTC))
	return research.SourceRegistryEntry{
		ID: id, Organization: "Archive", CanonicalDomains: []research.CanonicalDomain{domain},
		SourceKinds:     []research.SourceKind{research.SourceOfficialDocumentation},
		AuthorityHints:  []research.RegistryAuthorityHint{{SourceKind: research.SourceOfficialDocumentation, Tier: research.AuthorityTierB, Reason: "Historical official documentation."}},
		ResearchDomains: []string{"software"}, TopicPatterns: []string{"*"}, Status: research.RegistryHistorical,
		AddedAt: at, LastReviewedAt: at,
	}
}
