package registry

import (
	"strings"
	"testing"
	"time"

	"github.com/mishaaac/kelyro/internal/research"
)

func TestCanonicalDomainNormalizesAndAppliesExplicitSubdomainRules(t *testing.T) {
	t.Parallel()
	exact, err := research.NewCanonicalDomain("Go.Dev.")
	if err != nil || exact.String() != "go.dev" {
		t.Fatalf("NewCanonicalDomain(exact) = (%q, %v)", exact, err)
	}
	wildcard, err := research.NewCanonicalDomain("*.Docs.Go.Dev.")
	if err != nil || wildcard.String() != "*.docs.go.dev" {
		t.Fatalf("NewCanonicalDomain(wildcard) = (%q, %v)", wildcard, err)
	}
	if !exact.MatchesHost("GO.DEV.") || exact.MatchesHost("doc.go.dev") {
		t.Fatal("exact domain did not remain apex-only")
	}
	if !wildcard.MatchesHost("api.docs.go.dev") || wildcard.MatchesHost("docs.go.dev") || wildcard.MatchesHost("other.go.dev") {
		t.Fatal("wildcard domain did not remain subdomain-only")
	}
	for _, invalid := range []string{"go", "https://go.dev", "*.go", "go.dev/path", " go.dev"} {
		if _, err := research.NewCanonicalDomain(invalid); err == nil {
			t.Fatalf("NewCanonicalDomain(%q) accepted invalid domain", invalid)
		}
	}
}

func TestCatalogPreservesBlockedAndHistoricalStatus(t *testing.T) {
	t.Parallel()
	blocked := registryTestEntry(t, "registry.blocked", "blocked.example", research.RegistryBlocked)
	historical := registryTestEntry(t, "registry.historical", "archive.example", research.RegistryHistorical)
	catalog, err := NewCatalog([]research.SourceRegistryEntry{historical, blocked})
	if err != nil {
		t.Fatal(err)
	}
	for _, test := range []struct {
		locator string
		status  research.RegistryStatus
	}{
		{"https://blocked.example/unsafe", research.RegistryBlocked},
		{"https://archive.example/v1/docs", research.RegistryHistorical},
	} {
		locator, _ := research.NewSourceLocator(test.locator)
		entry, found, err := catalog.MatchLocator(locator)
		if err != nil || !found || entry.Status != test.status {
			t.Fatalf("MatchLocator(%q) = (%+v, %v, %v)", test.locator, entry, found, err)
		}
	}
}

func TestCatalogUsesExactHostBeforeWildcardAndRejectsDuplicateDomain(t *testing.T) {
	t.Parallel()
	wildcard := registryTestEntry(t, "registry.wildcard", "*.example.com", research.RegistryConditional)
	exact := registryTestEntry(t, "registry.exact", "docs.example.com", research.RegistryTrusted)
	catalog, err := NewCatalog([]research.SourceRegistryEntry{wildcard, exact})
	if err != nil {
		t.Fatal(err)
	}
	locator, _ := research.NewSourceLocator("https://docs.example.com/api")
	entry, found, err := catalog.MatchLocator(locator)
	if err != nil || !found || entry.ID != exact.ID {
		t.Fatalf("exact MatchLocator() = (%+v, %v, %v)", entry, found, err)
	}
	duplicate := registryTestEntry(t, "registry.duplicate", "DOCS.EXAMPLE.COM.", research.RegistryTrusted)
	if _, err := NewCatalog([]research.SourceRegistryEntry{exact, duplicate}); err == nil || !strings.Contains(err.Error(), "duplicate canonical domain") {
		t.Fatalf("NewCatalog(duplicate) error = %v", err)
	}
}

func registryTestEntry(t *testing.T, idValue, domainValue string, status research.RegistryStatus) research.SourceRegistryEntry {
	t.Helper()
	id, err := research.NewID(idValue)
	if err != nil {
		t.Fatal(err)
	}
	domain, err := research.NewCanonicalDomain(domainValue)
	if err != nil {
		t.Fatal(err)
	}
	added, _ := research.NewTimestamp(time.Date(2026, 8, 20, 10, 0, 0, 0, time.UTC))
	reviewed, _ := research.NewTimestamp(time.Date(2026, 8, 21, 10, 0, 0, 0, time.UTC))
	tier := research.AuthorityTierB
	if status == research.RegistryBlocked {
		tier = research.AuthorityTierE
	}
	return research.SourceRegistryEntry{
		ID: id, Organization: "Example", CanonicalDomains: []research.CanonicalDomain{domain},
		SourceKinds:     []research.SourceKind{research.SourceOfficialDocumentation},
		AuthorityHints:  []research.RegistryAuthorityHint{{SourceKind: research.SourceOfficialDocumentation, Tier: tier, Reason: "Deterministic fixture reason."}},
		ResearchDomains: []string{"software"}, TopicPatterns: []string{"*"}, Notes: "Fixture entry.",
		Status: status, AddedAt: added, LastReviewedAt: reviewed,
	}
}
