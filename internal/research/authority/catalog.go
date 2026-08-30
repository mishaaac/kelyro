package authority

import (
	"fmt"
	"sort"
	"strings"

	"github.com/mishaaac/kelyro/internal/research"
)

const ContractVersion = "authority-profiles/v1"

// Catalog is an immutable, deterministic set of authority profiles.
type Catalog struct {
	profiles []research.AuthorityProfile
}

// NewCatalog validates collection-level invariants that a single profile
// cannot observe, including duplicate identities and contradictory selectors.
func NewCatalog(profiles []research.AuthorityProfile) (Catalog, error) {
	if len(profiles) == 0 {
		return Catalog{}, fmt.Errorf("authority profile catalog is empty")
	}
	ids := make(map[research.ID]struct{}, len(profiles))
	selectors := make(map[string]research.ID, len(profiles))
	clones := make([]research.AuthorityProfile, len(profiles))
	for index, profile := range profiles {
		if err := profile.Validate(); err != nil {
			return Catalog{}, fmt.Errorf("authority profile %d: %w", index, err)
		}
		if _, exists := ids[profile.ID]; exists {
			return Catalog{}, fmt.Errorf("duplicate authority profile ID %q", profile.ID)
		}
		ids[profile.ID] = struct{}{}
		selector := normalizedSelector(profile)
		if other, exists := selectors[selector]; exists {
			return Catalog{}, fmt.Errorf("contradictory authority profiles %q and %q use selector %q", other, profile.ID, selector)
		}
		selectors[selector] = profile.ID
		clones[index] = cloneProfile(profile)
	}
	sort.Slice(clones, func(i, j int) bool { return clones[i].ID.String() < clones[j].ID.String() })
	return Catalog{profiles: clones}, nil
}

// Match selects one profile using domain specificity first, then topic-pattern
// specificity. Equal matches are resolved by stable profile ID order.
func (catalog Catalog) Match(topic research.ResearchTopic) (research.AuthorityProfile, bool, error) {
	if err := topic.Validate(); err != nil {
		return research.AuthorityProfile{}, false, fmt.Errorf("match authority profile: %w", err)
	}
	key := topicKey(topic)
	domain := strings.ToLower(topic.Domain)
	best := -1
	bestDomainExact := false
	bestSpecificity := -1
	for index, profile := range catalog.profiles {
		profileDomain := strings.ToLower(profile.Domain)
		domainExact := profileDomain != "*" && profileDomain == domain
		if !domainExact && profileDomain != "*" {
			continue
		}
		pattern := normalizedPattern(profile.TopicPattern)
		if !wildcardMatch(pattern, key) {
			continue
		}
		specificity := len([]rune(strings.ReplaceAll(pattern, "*", "")))
		if best == -1 || (domainExact && !bestDomainExact) ||
			(domainExact == bestDomainExact && specificity > bestSpecificity) {
			best = index
			bestDomainExact = domainExact
			bestSpecificity = specificity
		}
	}
	if best == -1 {
		return research.AuthorityProfile{}, false, nil
	}
	return cloneProfile(catalog.profiles[best]), true, nil
}

// Profiles returns a stable, defensive copy ordered by profile ID.
func (catalog Catalog) Profiles() []research.AuthorityProfile {
	result := make([]research.AuthorityProfile, len(catalog.profiles))
	for index, profile := range catalog.profiles {
		result[index] = cloneProfile(profile)
	}
	return result
}

func normalizedSelector(profile research.AuthorityProfile) string {
	return strings.ToLower(profile.Domain) + "\x00" + normalizedPattern(profile.TopicPattern)
}

func normalizedPattern(pattern string) string {
	if pattern == "" {
		return "*"
	}
	return strings.ToLower(strings.Join(strings.Fields(pattern), " "))
}

func topicKey(topic research.ResearchTopic) string {
	subject := strings.ToLower(strings.Join(strings.Fields(topic.Subject), " "))
	if topic.Technology == "" {
		return subject
	}
	technology := strings.ToLower(strings.Join(strings.Fields(topic.Technology), " "))
	return technology + "/" + subject
}

func wildcardMatch(pattern, value string) bool {
	parts := strings.Split(pattern, "*")
	if len(parts) == 1 {
		return pattern == value
	}
	position := 0
	for index, part := range parts {
		if part == "" {
			continue
		}
		found := strings.Index(value[position:], part)
		if found < 0 || (index == 0 && !strings.HasPrefix(pattern, "*") && found != 0) {
			return false
		}
		position += found + len(part)
	}
	last := parts[len(parts)-1]
	return last == "" || strings.HasSuffix(value, last)
}

func cloneProfile(profile research.AuthorityProfile) research.AuthorityProfile {
	clone := profile
	clone.PreferredKinds = append([]research.SourceKind(nil), profile.PreferredKinds...)
	clone.PreferredDomains = append([]string(nil), profile.PreferredDomains...)
	clone.PreferredOrganizations = append([]string(nil), profile.PreferredOrganizations...)
	clone.AllowedSupplementaryKinds = append([]research.SourceKind(nil), profile.AllowedSupplementaryKinds...)
	clone.FreshnessTTLHints = cloneFreshnessTTLHints(profile.FreshnessTTLHints)
	return clone
}

func cloneFreshnessTTLHints(hints []research.FreshnessTTLHint) []research.FreshnessTTLHint {
	result := make([]research.FreshnessTTLHint, len(hints))
	for index, hint := range hints {
		result[index] = hint
		if hint.ClaimType != nil {
			value := *hint.ClaimType
			result[index].ClaimType = &value
		}
		if hint.SourceKind != nil {
			value := *hint.SourceKind
			result[index].SourceKind = &value
		}
	}
	return result
}
