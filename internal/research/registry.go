package research

import (
	"fmt"
	"strings"
)

// RegistryStatus is descriptive registry metadata, not a trust decision.
type RegistryStatus string

const (
	RegistryTrusted     RegistryStatus = "trusted"
	RegistryConditional RegistryStatus = "conditional"
	RegistryHistorical  RegistryStatus = "historical"
	RegistryDeprecated  RegistryStatus = "deprecated"
	RegistryBlocked     RegistryStatus = "blocked"
)

func (status RegistryStatus) Validate() error {
	switch status {
	case RegistryTrusted, RegistryConditional, RegistryHistorical, RegistryDeprecated, RegistryBlocked:
		return nil
	default:
		return fmt.Errorf("invalid source registry status %q", status)
	}
}

// CanonicalDomain is a normalized DNS host rule. A leading wildcard matches
// subdomains only; callers must add the apex explicitly when both are valid.
type CanonicalDomain struct {
	host              string
	includeSubdomains bool
}

func NewCanonicalDomain(value string) (CanonicalDomain, error) {
	if value == "" || value != strings.TrimSpace(value) {
		return CanonicalDomain{}, fmt.Errorf("canonical domain %q is invalid", value)
	}
	wildcard := strings.HasPrefix(value, "*.")
	host := value
	if wildcard {
		host = strings.TrimPrefix(host, "*.")
	}
	host = strings.TrimSuffix(strings.ToLower(host), ".")
	if err := validateDNSHost(host); err != nil {
		return CanonicalDomain{}, fmt.Errorf("canonical domain %q: %w", value, err)
	}
	return CanonicalDomain{host: host, includeSubdomains: wildcard}, nil
}

func (domain CanonicalDomain) String() string {
	if domain.includeSubdomains {
		return "*." + domain.host
	}
	return domain.host
}

func (domain CanonicalDomain) Host() string { return domain.host }
func (domain CanonicalDomain) IncludesSubdomains() bool {
	return domain.includeSubdomains
}

func (domain CanonicalDomain) Validate() error {
	if err := validateDNSHost(domain.host); err != nil {
		return fmt.Errorf("canonical domain: %w", err)
	}
	return nil
}

func (domain CanonicalDomain) MatchesHost(value string) bool {
	host := strings.TrimSuffix(strings.ToLower(value), ".")
	if validateDNSHost(host) != nil {
		return false
	}
	if domain.includeSubdomains {
		return host != domain.host && strings.HasSuffix(host, "."+domain.host)
	}
	return host == domain.host
}

func validateDNSHost(host string) error {
	if host == "" || len(host) > 253 || strings.ContainsAny(host, "*/:@ ") {
		return fmt.Errorf("DNS host %q is invalid", host)
	}
	labels := strings.Split(host, ".")
	if len(labels) < 2 {
		return fmt.Errorf("DNS host %q must have at least two labels", host)
	}
	for _, label := range labels {
		if label == "" || len(label) > 63 || strings.HasPrefix(label, "-") || strings.HasSuffix(label, "-") {
			return fmt.Errorf("DNS host %q has an invalid label", host)
		}
		for _, character := range label {
			if (character >= 'a' && character <= 'z') || (character >= '0' && character <= '9') || character == '-' {
				continue
			}
			return fmt.Errorf("DNS host %q contains an invalid character", host)
		}
	}
	return nil
}

// RegistryAuthorityHint explains why a source kind may carry contextual
// authority. Trust Policy still evaluates the actual source and evidence.
type RegistryAuthorityHint struct {
	SourceKind SourceKind
	Tier       AuthorityTier
	Reason     string
}

func (hint RegistryAuthorityHint) Validate() error {
	if err := hint.SourceKind.Validate(); err != nil {
		return err
	}
	if err := hint.Tier.Validate(); err != nil {
		return err
	}
	return requireText("registry authority hint reason", hint.Reason)
}

// SourceRegistryEntry describes a known organization/source family. Its status
// and hints are inputs to later policy evaluation, never evidence by themselves.
type SourceRegistryEntry struct {
	ID               ID
	Organization     string
	CanonicalDomains []CanonicalDomain
	SourceKinds      []SourceKind
	AuthorityHints   []RegistryAuthorityHint
	ResearchDomains  []string
	TopicPatterns    []string
	Notes            string
	Status           RegistryStatus
	AddedAt          Timestamp
	LastReviewedAt   Timestamp
}

func (entry SourceRegistryEntry) Validate() error {
	if err := entry.ID.Validate(); err != nil {
		return fmt.Errorf("source registry entry: %w", err)
	}
	if err := requireText("source registry organization", entry.Organization); err != nil {
		return err
	}
	if len(entry.CanonicalDomains) == 0 {
		return fmt.Errorf("source registry canonical domains are empty")
	}
	domains := make(map[string]struct{}, len(entry.CanonicalDomains))
	for _, domain := range entry.CanonicalDomains {
		if err := domain.Validate(); err != nil {
			return err
		}
		if _, exists := domains[domain.String()]; exists {
			return fmt.Errorf("source registry contains duplicate canonical domain %q", domain)
		}
		domains[domain.String()] = struct{}{}
	}
	if len(entry.SourceKinds) == 0 {
		return fmt.Errorf("source registry source kinds are empty")
	}
	kinds := make(map[SourceKind]struct{}, len(entry.SourceKinds))
	for _, kind := range entry.SourceKinds {
		if err := kind.Validate(); err != nil {
			return err
		}
		if _, exists := kinds[kind]; exists {
			return fmt.Errorf("source registry contains duplicate source kind %q", kind)
		}
		kinds[kind] = struct{}{}
	}
	if len(entry.AuthorityHints) == 0 {
		return fmt.Errorf("source registry authority hints are empty")
	}
	seenHints := make(map[SourceKind]struct{}, len(entry.AuthorityHints))
	for _, hint := range entry.AuthorityHints {
		if err := hint.Validate(); err != nil {
			return err
		}
		if _, exists := kinds[hint.SourceKind]; !exists {
			return fmt.Errorf("registry authority hint kind %q is not declared by the entry", hint.SourceKind)
		}
		if _, exists := seenHints[hint.SourceKind]; exists {
			return fmt.Errorf("source registry contains duplicate authority hint for %q", hint.SourceKind)
		}
		seenHints[hint.SourceKind] = struct{}{}
	}
	if len(entry.ResearchDomains) == 0 {
		return fmt.Errorf("source registry research domains are empty")
	}
	if err := validateRegistryDomains(entry.ResearchDomains); err != nil {
		return err
	}
	if len(entry.TopicPatterns) == 0 {
		return fmt.Errorf("source registry topic patterns are empty")
	}
	if err := validateRegistryTopicPatterns(entry.TopicPatterns); err != nil {
		return err
	}
	if err := validateOptionalText("source registry notes", entry.Notes); err != nil {
		return err
	}
	if err := entry.Status.Validate(); err != nil {
		return err
	}
	if err := validateTimestamp("source registry added at", entry.AddedAt); err != nil {
		return err
	}
	if err := validateTimestamp("source registry last reviewed at", entry.LastReviewedAt); err != nil {
		return err
	}
	if entry.LastReviewedAt.Before(entry.AddedAt) {
		return fmt.Errorf("source registry review precedes addition")
	}
	return nil
}

func validateRegistryDomains(values []string) error {
	seen := make(map[string]struct{}, len(values))
	for _, value := range values {
		if err := validateAuthorityDomain(value); err != nil {
			return err
		}
		if _, exists := seen[value]; exists {
			return fmt.Errorf("source registry contains duplicate research domain %q", value)
		}
		seen[value] = struct{}{}
	}
	return nil
}

func validateRegistryTopicPatterns(values []string) error {
	seen := make(map[string]struct{}, len(values))
	for _, value := range values {
		if value == "" {
			return fmt.Errorf("source registry topic pattern is empty")
		}
		if err := validateTopicPattern(value); err != nil {
			return err
		}
		key := strings.ToLower(strings.Join(strings.Fields(value), " "))
		if _, exists := seen[key]; exists {
			return fmt.Errorf("source registry contains duplicate topic pattern %q", value)
		}
		seen[key] = struct{}{}
	}
	return nil
}
