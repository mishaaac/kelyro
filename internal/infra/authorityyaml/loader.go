package authorityyaml

import (
	"errors"
	"fmt"
	"io"
	"time"

	"github.com/mishaaac/kelyro/internal/research"
	"github.com/mishaaac/kelyro/internal/research/authority"
	"go.yaml.in/yaml/v3"
)

type document struct {
	ContractVersion string            `yaml:"contract_version"`
	Profiles        []profileDocument `yaml:"profiles"`
}

type profileDocument struct {
	ID                        string                     `yaml:"id"`
	Version                   string                     `yaml:"version"`
	Domain                    string                     `yaml:"domain"`
	TopicPattern              string                     `yaml:"topic_pattern"`
	PreferredKinds            []string                   `yaml:"preferred_source_kinds"`
	PreferredDomains          []string                   `yaml:"preferred_domains,omitempty"`
	PreferredOrganizations    []string                   `yaml:"preferred_organizations,omitempty"`
	MinimumCorroboration      int                        `yaml:"minimum_corroboration"`
	AllowedSupplementaryKinds []string                   `yaml:"allowed_supplementary_kinds,omitempty"`
	FreshnessTTLHints         []freshnessTTLHintDocument `yaml:"freshness_ttl_hints,omitempty"`
	MinimumTier               string                     `yaml:"minimum_tier"`
	CreatedAt                 string                     `yaml:"created_at"`
}

type freshnessTTLHintDocument struct {
	ClaimType  string `yaml:"claim_type,omitempty"`
	SourceKind string `yaml:"source_kind,omitempty"`
	TTLDays    int    `yaml:"ttl_days"`
}

// Load strictly decodes exactly one YAML document and validates the complete
// authority profile catalog.
func Load(reader io.Reader) (authority.Catalog, error) {
	if reader == nil {
		return authority.Catalog{}, fmt.Errorf("load authority profiles YAML: reader is nil")
	}
	decoder := yaml.NewDecoder(reader)
	decoder.KnownFields(true)
	var source document
	if err := decoder.Decode(&source); err != nil {
		if errors.Is(err, io.EOF) {
			return authority.Catalog{}, fmt.Errorf("load authority profiles YAML: document is empty")
		}
		return authority.Catalog{}, fmt.Errorf("load authority profiles YAML: %w", err)
	}
	var extra yaml.Node
	if err := decoder.Decode(&extra); !errors.Is(err, io.EOF) {
		if err != nil {
			return authority.Catalog{}, fmt.Errorf("load authority profiles YAML trailing document: %w", err)
		}
		return authority.Catalog{}, fmt.Errorf("load authority profiles YAML: multiple documents are not allowed")
	}
	if source.ContractVersion != authority.ContractVersion {
		return authority.Catalog{}, fmt.Errorf("load authority profiles YAML: unsupported contract version %q", source.ContractVersion)
	}
	profiles := make([]research.AuthorityProfile, 0, len(source.Profiles))
	for index, raw := range source.Profiles {
		profile, err := decodeProfile(raw)
		if err != nil {
			return authority.Catalog{}, fmt.Errorf("load authority profiles YAML profile %d: %w", index, err)
		}
		profiles = append(profiles, profile)
	}
	catalog, err := authority.NewCatalog(profiles)
	if err != nil {
		return authority.Catalog{}, fmt.Errorf("load authority profiles YAML: %w", err)
	}
	return catalog, nil
}

func decodeProfile(source profileDocument) (research.AuthorityProfile, error) {
	id, err := research.NewID(source.ID)
	if err != nil {
		return research.AuthorityProfile{}, fmt.Errorf("id: %w", err)
	}
	parsed, err := time.Parse(time.RFC3339Nano, source.CreatedAt)
	if err != nil {
		return research.AuthorityProfile{}, fmt.Errorf("created_at: %w", err)
	}
	createdAt, err := research.NewTimestamp(parsed)
	if err != nil {
		return research.AuthorityProfile{}, fmt.Errorf("created_at: %w", err)
	}
	profile := research.AuthorityProfile{
		ID: id, Version: source.Version, Domain: source.Domain, TopicPattern: source.TopicPattern,
		PreferredKinds: decodeKinds(source.PreferredKinds), PreferredDomains: append([]string(nil), source.PreferredDomains...),
		PreferredOrganizations:    append([]string(nil), source.PreferredOrganizations...),
		MinimumCorroboration:      source.MinimumCorroboration,
		AllowedSupplementaryKinds: decodeKinds(source.AllowedSupplementaryKinds),
		FreshnessTTLHints:         decodeFreshnessTTLHints(source.FreshnessTTLHints),
		MinimumTier:               research.AuthorityTier(source.MinimumTier), CreatedAt: createdAt,
	}
	if err := profile.Validate(); err != nil {
		return research.AuthorityProfile{}, err
	}
	return profile, nil
}

func decodeFreshnessTTLHints(values []freshnessTTLHintDocument) []research.FreshnessTTLHint {
	if len(values) == 0 {
		return nil
	}
	result := make([]research.FreshnessTTLHint, len(values))
	for index, value := range values {
		result[index].TTLDays = value.TTLDays
		if value.ClaimType != "" {
			claimType := research.ClaimType(value.ClaimType)
			result[index].ClaimType = &claimType
		}
		if value.SourceKind != "" {
			sourceKind := research.SourceKind(value.SourceKind)
			result[index].SourceKind = &sourceKind
		}
	}
	return result
}

func decodeKinds(values []string) []research.SourceKind {
	result := make([]research.SourceKind, len(values))
	for index, value := range values {
		result[index] = research.SourceKind(value)
	}
	return result
}
