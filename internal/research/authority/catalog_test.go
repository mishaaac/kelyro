package authority

import (
	"strings"
	"testing"
	"time"

	"github.com/mishaaac/kelyro/internal/research"
)

func TestCatalogMatchesTopicWithDeterministicPrecedenceAndFallback(t *testing.T) {
	t.Parallel()
	catalog, err := NewCatalog([]research.AuthorityProfile{
		authorityTestProfile(t, "authority.global", "*", "*"),
		authorityTestProfile(t, "authority.software", "software", "*"),
		authorityTestProfile(t, "authority.software.go", "software", "go/*"),
		authorityTestProfile(t, "authority.software.go.http", "software", "go/http *"),
	})
	if err != nil {
		t.Fatal(err)
	}
	tests := []struct {
		name      string
		topic     research.ResearchTopic
		want      string
		wantFound bool
	}{
		{name: "most specific technology topic", topic: authorityTestTopic(t, "HTTP caching", "software", "Go"), want: "authority.software.go.http", wantFound: true},
		{name: "technology profile", topic: authorityTestTopic(t, "Interfaces", "software", "GO"), want: "authority.software.go", wantFound: true},
		{name: "domain fallback", topic: authorityTestTopic(t, "Ownership", "software", "Rust"), want: "authority.software", wantFound: true},
		{name: "global fallback", topic: authorityTestTopic(t, "Bayesian inference", "statistics", ""), want: "authority.global", wantFound: true},
	}
	for _, test := range tests {
		test := test
		t.Run(test.name, func(t *testing.T) {
			got, found, err := catalog.Match(test.topic)
			if err != nil || found != test.wantFound || got.ID.String() != test.want {
				t.Fatalf("Match() = (%+v, %v, %v), want ID %q", got, found, err, test.want)
			}
		})
	}
}

func TestCatalogSupportsCustomFutureDomainWithoutCoreChanges(t *testing.T) {
	t.Parallel()
	profile := authorityTestProfile(t, "authority.medicine.cardiology", "medicine", "cardiology*")
	profile.PreferredKinds = []research.SourceKind{research.SourcePaper, research.SourceStandard}
	profile.PreferredDomains = []string{"who.int"}
	profile.PreferredOrganizations = []string{"World Health Organization"}
	profile.MinimumCorroboration = 2
	catalog, err := NewCatalog([]research.AuthorityProfile{profile})
	if err != nil {
		t.Fatal(err)
	}
	got, found, err := catalog.Match(authorityTestTopic(t, "Cardiology guidelines", "medicine", ""))
	if err != nil || !found || got.ID != profile.ID || got.MinimumCorroboration != 2 {
		t.Fatalf("custom Match() = (%+v, %v, %v)", got, found, err)
	}
	if _, found, err := catalog.Match(authorityTestTopic(t, "Cardiology", "software", "")); err != nil || found {
		t.Fatalf("cross-domain Match() found=%v error=%v", found, err)
	}
}

func TestCatalogRejectsDuplicateAndContradictoryProfiles(t *testing.T) {
	t.Parallel()
	base := authorityTestProfile(t, "authority.base", "software", "go/*")
	tests := []struct {
		name     string
		profiles []research.AuthorityProfile
		want     string
	}{
		{name: "duplicate ID", profiles: []research.AuthorityProfile{base, base}, want: "duplicate authority profile ID"},
		{name: "same selector", profiles: []research.AuthorityProfile{base, authorityTestProfile(t, "authority.other", "software", "GO/*")}, want: "contradictory authority profiles"},
	}
	for _, test := range tests {
		_, err := NewCatalog(test.profiles)
		if err == nil || !strings.Contains(err.Error(), test.want) {
			t.Fatalf("NewCatalog(%s) error = %v, want %q", test.name, err, test.want)
		}
	}
}

func TestAuthorityProfileRejectsInvalidAndContradictoryRules(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name   string
		mutate func(*research.AuthorityProfile)
		want   string
	}{
		{name: "invalid preferred domain", mutate: func(profile *research.AuthorityProfile) { profile.PreferredDomains = []string{"https://go.dev/doc"} }, want: "invalid preferred domain pattern"},
		{name: "unknown preferred kind", mutate: func(profile *research.AuthorityProfile) {
			profile.PreferredKinds = []research.SourceKind{"social_post"}
		}, want: "invalid source kind"},
		{name: "unknown supplementary kind", mutate: func(profile *research.AuthorityProfile) {
			profile.AllowedSupplementaryKinds = []research.SourceKind{"social_post"}
		}, want: "invalid source kind"},
		{name: "contradictory kind", mutate: func(profile *research.AuthorityProfile) {
			profile.AllowedSupplementaryKinds = []research.SourceKind{research.SourceSpecification}
		}, want: "both preferred and supplementary"},
		{name: "invalid topic glob", mutate: func(profile *research.AuthorityProfile) { profile.TopicPattern = "go/[api]" }, want: "invalid authority profile topic pattern"},
		{name: "zero corroboration", mutate: func(profile *research.AuthorityProfile) { profile.MinimumCorroboration = 0 }, want: "minimum corroboration"},
	}
	for _, test := range tests {
		profile := authorityTestProfile(t, "authority.valid", "software", "*")
		test.mutate(&profile)
		if err := profile.Validate(); err == nil || !strings.Contains(err.Error(), test.want) {
			t.Fatalf("Validate(%s) error = %v, want %q", test.name, err, test.want)
		}
	}
}

func authorityTestProfile(t *testing.T, idValue, domain, pattern string) research.AuthorityProfile {
	t.Helper()
	id, err := research.NewID(idValue)
	if err != nil {
		t.Fatal(err)
	}
	createdAt, err := research.NewTimestamp(time.Date(2026, time.August, 24, 0, 0, 0, 0, time.UTC))
	if err != nil {
		t.Fatal(err)
	}
	return research.AuthorityProfile{ID: id, Version: "authority-profile/v1", Domain: domain, TopicPattern: pattern, PreferredKinds: []research.SourceKind{research.SourceSpecification}, MinimumCorroboration: 1, MinimumTier: research.AuthorityTierC, CreatedAt: createdAt}
}

func authorityTestTopic(t *testing.T, subject, domain, technology string) research.ResearchTopic {
	t.Helper()
	topic, err := research.NewResearchTopic(subject, domain, technology)
	if err != nil {
		t.Fatal(err)
	}
	return topic
}
