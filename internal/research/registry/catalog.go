package registry

import (
	"fmt"
	"net/url"
	"sort"
	"strings"

	"github.com/mishaaac/kelyro/internal/research"
)

type Catalog struct {
	entries []research.SourceRegistryEntry
}

// AppliesTo reports whether an entry declares the source kind and research
// domain/topic context. It does not evaluate trust.
func AppliesTo(entry research.SourceRegistryEntry, topic research.ResearchTopic, kind research.SourceKind) bool {
	if entry.Validate() != nil || topic.Validate() != nil || kind.Validate() != nil {
		return false
	}
	kindFound := false
	for _, candidate := range entry.SourceKinds {
		if candidate == kind {
			kindFound = true
			break
		}
	}
	if !kindFound {
		return false
	}
	domainFound := false
	for _, domain := range entry.ResearchDomains {
		if domain == "*" || strings.EqualFold(domain, topic.Domain) {
			domainFound = true
			break
		}
	}
	if !domainFound {
		return false
	}
	key := strings.ToLower(strings.Join(strings.Fields(topic.Subject), " "))
	if topic.Technology != "" {
		key = strings.ToLower(strings.Join(strings.Fields(topic.Technology), " ")) + "/" + key
	}
	for _, pattern := range entry.TopicPatterns {
		if wildcardMatch(strings.ToLower(strings.Join(strings.Fields(pattern), " ")), key) {
			return true
		}
	}
	return false
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

func NewCatalog(entries []research.SourceRegistryEntry) (Catalog, error) {
	ids := make(map[research.ID]struct{}, len(entries))
	domains := make(map[string]research.ID)
	clones := make([]research.SourceRegistryEntry, len(entries))
	for index, entry := range entries {
		if err := entry.Validate(); err != nil {
			return Catalog{}, fmt.Errorf("source registry entry %d: %w", index, err)
		}
		if _, exists := ids[entry.ID]; exists {
			return Catalog{}, fmt.Errorf("duplicate source registry ID %q", entry.ID)
		}
		ids[entry.ID] = struct{}{}
		for _, domain := range entry.CanonicalDomains {
			if other, exists := domains[domain.String()]; exists {
				return Catalog{}, fmt.Errorf("duplicate canonical domain %q in registry entries %q and %q", domain, other, entry.ID)
			}
			domains[domain.String()] = entry.ID
		}
		clones[index] = cloneEntry(entry)
	}
	sort.Slice(clones, func(i, j int) bool { return clones[i].ID.String() < clones[j].ID.String() })
	return Catalog{entries: clones}, nil
}

// MatchLocator returns the most specific domain match while preserving the
// registry status, including blocked and historical entries.
func (catalog Catalog) MatchLocator(locator research.SourceLocator) (research.SourceRegistryEntry, bool, error) {
	if err := locator.Validate(); err != nil {
		return research.SourceRegistryEntry{}, false, fmt.Errorf("match source registry locator: %w", err)
	}
	parsed, err := url.Parse(locator.String())
	if err != nil {
		return research.SourceRegistryEntry{}, false, fmt.Errorf("match source registry locator: %w", err)
	}
	bestEntry := -1
	bestExact := false
	bestLength := -1
	for entryIndex, entry := range catalog.entries {
		for _, domain := range entry.CanonicalDomains {
			if !domain.MatchesHost(parsed.Hostname()) {
				continue
			}
			exact := !domain.IncludesSubdomains()
			length := len(domain.Host())
			if bestEntry == -1 || (exact && !bestExact) || (exact == bestExact && length > bestLength) {
				bestEntry, bestExact, bestLength = entryIndex, exact, length
			}
		}
	}
	if bestEntry == -1 {
		return research.SourceRegistryEntry{}, false, nil
	}
	return cloneEntry(catalog.entries[bestEntry]), true, nil
}

func (catalog Catalog) Entries() []research.SourceRegistryEntry {
	result := make([]research.SourceRegistryEntry, len(catalog.entries))
	for index, entry := range catalog.entries {
		result[index] = cloneEntry(entry)
	}
	return result
}

func cloneEntry(entry research.SourceRegistryEntry) research.SourceRegistryEntry {
	clone := entry
	clone.CanonicalDomains = append([]research.CanonicalDomain(nil), entry.CanonicalDomains...)
	clone.SourceKinds = append([]research.SourceKind(nil), entry.SourceKinds...)
	clone.AuthorityHints = append([]research.RegistryAuthorityHint(nil), entry.AuthorityHints...)
	clone.ResearchDomains = append([]string(nil), entry.ResearchDomains...)
	clone.TopicPatterns = append([]string(nil), entry.TopicPatterns...)
	return clone
}
