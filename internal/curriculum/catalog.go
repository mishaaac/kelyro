package curriculum

import (
	"fmt"
	"net/url"
	"sort"
	"strings"
)

const PackCatalogSchemaVersionV1 = "pack-catalog/v1"

type PackCatalogTrust string

const (
	PackCatalogOfficial   PackCatalogTrust = "official"
	PackCatalogVerified   PackCatalogTrust = "verified"
	PackCatalogCommunity  PackCatalogTrust = "community"
	PackCatalogUnverified PackCatalogTrust = "unverified"
)

func (trust PackCatalogTrust) Validate() error {
	switch trust {
	case PackCatalogOfficial, PackCatalogVerified, PackCatalogCommunity, PackCatalogUnverified:
		return nil
	default:
		return fmt.Errorf("invalid pack catalog trust %q", trust)
	}
}

type PackCompatibility string

const (
	PackCompatible           PackCompatibility = "compatible"
	PackIncompatible         PackCompatibility = "incompatible"
	PackCompatibilityUnknown PackCompatibility = "unknown"
)

func (compatibility PackCompatibility) Validate() error {
	switch compatibility {
	case PackCompatible, PackIncompatible, PackCompatibilityUnknown:
		return nil
	default:
		return fmt.Errorf("invalid pack compatibility %q", compatibility)
	}
}

type PackCatalogSourceMetadata struct {
	ID      ID
	Name    string
	Locator string
	Trust   PackCatalogTrust
}

func (source PackCatalogSourceMetadata) Validate() error {
	if err := source.ID.Validate(); err != nil {
		return fmt.Errorf("pack catalog source: %w", err)
	}
	if err := requireText("pack catalog source name", source.Name); err != nil {
		return err
	}
	if err := validateCatalogLocator("pack catalog source locator", source.Locator, true); err != nil {
		return err
	}
	return source.Trust.Validate()
}

type PackCatalogVersion struct {
	Version              PackVersion
	Status               PackStatus
	MinimumKelyroVersion PackVersion
	Compatibility        PackCompatibility
	Artifact             string
}

func (version PackCatalogVersion) Validate() error {
	if err := version.Version.Validate(); err != nil {
		return err
	}
	if err := version.Status.Validate(); err != nil {
		return err
	}
	if err := version.MinimumKelyroVersion.Validate(); err != nil {
		return fmt.Errorf("catalog minimum Kelyro version: %w", err)
	}
	if err := version.Compatibility.Validate(); err != nil {
		return err
	}
	return validateCatalogLocator("pack catalog artifact", version.Artifact, false)
}

func validateCatalogLocator(name, value string, allowLocal bool) error {
	if err := requireText(name, value); err != nil {
		return err
	}
	parsed, err := url.Parse(value)
	if err != nil || parsed.User != nil || parsed.RawQuery != "" || parsed.Fragment != "" {
		return fmt.Errorf("%s must not contain credentials, query, or fragment", name)
	}
	if parsed.Scheme == "https" && parsed.Host != "" {
		return nil
	}
	if allowLocal && parsed.Scheme == "local" && strings.TrimSpace(parsed.Opaque) != "" {
		return nil
	}
	return fmt.Errorf("%s must be a query-free HTTPS locator", name)
}

type PackCatalogEntry struct {
	PackID      ID
	Name        string
	Description string
	Domain      string
	Target      string
	Maintainer  string
	Versions    []PackCatalogVersion
	Source      PackCatalogSourceMetadata
}

func (entry PackCatalogEntry) Validate() error {
	if err := entry.PackID.Validate(); err != nil {
		return fmt.Errorf("pack catalog entry: %w", err)
	}
	for _, field := range []struct{ name, value string }{
		{"pack catalog name", entry.Name}, {"pack catalog description", entry.Description},
		{"pack catalog domain", entry.Domain}, {"pack catalog target", entry.Target},
		{"pack catalog maintainer", entry.Maintainer},
	} {
		if err := requireText(field.name, field.value); err != nil {
			return err
		}
	}
	if len(entry.Versions) == 0 {
		return fmt.Errorf("pack catalog entry %q has no versions", entry.PackID)
	}
	seen := make(map[string]struct{}, len(entry.Versions))
	previous := ""
	for _, version := range entry.Versions {
		if err := version.Validate(); err != nil {
			return err
		}
		if _, exists := seen[version.Version.String()]; exists {
			return fmt.Errorf("pack catalog entry %q contains duplicate version %q", entry.PackID, version.Version.String())
		}
		seen[version.Version.String()] = struct{}{}
		if previous != "" && previous >= version.Version.String() {
			return fmt.Errorf("pack catalog entry %q versions are not canonically ordered", entry.PackID)
		}
		previous = version.Version.String()
	}
	return entry.Source.Validate()
}

type PackCatalogSnapshot struct {
	SchemaVersion string
	GeneratedAt   Timestamp
	Entries       []PackCatalogEntry
}

func (snapshot PackCatalogSnapshot) Validate() error {
	if snapshot.SchemaVersion != PackCatalogSchemaVersionV1 {
		return fmt.Errorf("unsupported pack catalog schema %q", snapshot.SchemaVersion)
	}
	if err := snapshot.GeneratedAt.Validate(); err != nil {
		return fmt.Errorf("pack catalog generated at: %w", err)
	}
	seen := make(map[ID]struct{}, len(snapshot.Entries))
	previous := ""
	for _, entry := range snapshot.Entries {
		if err := entry.Validate(); err != nil {
			return err
		}
		if _, exists := seen[entry.PackID]; exists {
			return fmt.Errorf("pack catalog contains duplicate pack %q", entry.PackID)
		}
		seen[entry.PackID] = struct{}{}
		if previous != "" && previous >= entry.PackID.String() {
			return fmt.Errorf("pack catalog entries are not canonically ordered")
		}
		previous = entry.PackID.String()
	}
	return nil
}

func SortPackCatalog(snapshot PackCatalogSnapshot) PackCatalogSnapshot {
	result := snapshot
	result.Entries = append([]PackCatalogEntry(nil), snapshot.Entries...)
	for index := range result.Entries {
		result.Entries[index].Versions = append([]PackCatalogVersion(nil), result.Entries[index].Versions...)
		sort.Slice(result.Entries[index].Versions, func(i, j int) bool {
			return result.Entries[index].Versions[i].Version.String() < result.Entries[index].Versions[j].Version.String()
		})
	}
	sort.Slice(result.Entries, func(i, j int) bool { return result.Entries[i].PackID.String() < result.Entries[j].PackID.String() })
	return result
}
