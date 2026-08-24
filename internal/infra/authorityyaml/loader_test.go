package authorityyaml

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/mishaaac/kelyro/internal/research"
)

func TestLoadTechnologySoftwareFixture(t *testing.T) {
	t.Parallel()
	encoded := readTechnologyFixture(t)
	catalog, err := Load(bytes.NewReader(encoded))
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}
	if got := len(catalog.Profiles()); got != 2 {
		t.Fatalf("profile count = %d, want 2", got)
	}
	topic, err := research.NewResearchTopic("Interfaces", "software", "Go")
	if err != nil {
		t.Fatal(err)
	}
	profile, found, err := catalog.Match(topic)
	if err != nil || !found || profile.ID.String() != "authority.technology-software.go" {
		t.Fatalf("Go fixture Match() = (%+v, %v, %v)", profile, found, err)
	}
	if len(profile.PreferredDomains) != 3 || profile.PreferredDomains[0] != "go.dev" {
		t.Fatalf("Go preferred domains = %v", profile.PreferredDomains)
	}
}

func TestLoadRejectsStrictYAMLAndProfileViolations(t *testing.T) {
	t.Parallel()
	base := `contract_version: authority-profiles/v1
profiles:
  - id: authority.test
    version: test/v1
    domain: software
    topic_pattern: "*"
    preferred_source_kinds: [specification]
    minimum_corroboration: 1
    minimum_tier: C
    created_at: 2026-08-24T00:00:00Z
`
	tests := []struct {
		name    string
		encoded string
		want    string
	}{
		{name: "unknown field", encoded: base + "unknown: true\n", want: "field unknown not found"},
		{name: "multiple documents", encoded: base + "---\nprofiles: []\n", want: "multiple documents are not allowed"},
		{name: "unknown source kind", encoded: strings.Replace(base, "specification", "social_post", 1), want: "invalid source kind"},
		{name: "invalid preferred domain", encoded: strings.Replace(base, "minimum_corroboration", "preferred_domains: [go.dev/doc]\n    minimum_corroboration", 1), want: "invalid preferred domain pattern"},
		{name: "contradictory kinds", encoded: strings.Replace(base, "minimum_corroboration", "allowed_supplementary_kinds: [specification]\n    minimum_corroboration", 1), want: "both preferred and supplementary"},
		{name: "duplicate ID", encoded: base + strings.TrimPrefix(base, "contract_version: authority-profiles/v1\nprofiles:\n"), want: "duplicate authority profile ID"},
		{name: "empty", encoded: "", want: "document is empty"},
	}
	for _, test := range tests {
		test := test
		t.Run(test.name, func(t *testing.T) {
			_, err := Load(strings.NewReader(test.encoded))
			if err == nil || !strings.Contains(err.Error(), test.want) {
				t.Fatalf("Load() error = %v, want containing %q", err, test.want)
			}
		})
	}
}

func readTechnologyFixture(t *testing.T) []byte {
	t.Helper()
	path := filepath.Join("..", "..", "..", "assets", "research", "authority-profiles", "technology-software.yaml")
	encoded, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("ReadFile(%q) error = %v", path, err)
	}
	return encoded
}
