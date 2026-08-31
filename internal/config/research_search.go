package config

import (
	"fmt"
	"strings"
)

const (
	DefaultResearchSearchMaxResultsPerQuery = 8
	DefaultResearchSearchMaxQueriesPerRun   = 4
	MaximumResearchSearchResultsPerQuery    = 100
	MaximumResearchSearchQueriesPerRun      = 8
)

// ResearchSearchProviderState is the safe, credential-free readiness state
// exposed to future wiring and Doctor checks.
type ResearchSearchProviderState string

const (
	ResearchSearchConfigured        ResearchSearchProviderState = "configured"
	ResearchSearchMissingCredential ResearchSearchProviderState = "missing_credentials"
	ResearchSearchDisabled          ResearchSearchProviderState = "disabled"
	ResearchSearchUnavailable       ResearchSearchProviderState = "unavailable"
)

func (state ResearchSearchProviderState) Validate() error {
	switch state {
	case ResearchSearchConfigured, ResearchSearchMissingCredential,
		ResearchSearchDisabled, ResearchSearchUnavailable:
		return nil
	default:
		return fmt.Errorf("invalid research search provider state %q", state)
	}
}

// ResearchSearchConfig is the resolved, provider-neutral search configuration.
// Provider is empty when live search is disabled. Credentials are deliberately
// absent and must be resolved through Foundation Secrets by later wiring.
type ResearchSearchConfig struct {
	Provider           string
	MaxResultsPerQuery int
	MaxQueriesPerRun   int
}

func (search ResearchSearchConfig) Validate() error {
	if err := validateResearchSearchProvider(search.Provider); err != nil {
		return err
	}
	if err := validateBoundedInteger(float64(search.MaxResultsPerQuery), 1, MaximumResearchSearchResultsPerQuery); err != nil {
		return fmt.Errorf("research search max results per query must be an integer from 1 to %d", MaximumResearchSearchResultsPerQuery)
	}
	if err := validateBoundedInteger(float64(search.MaxQueriesPerRun), 1, MaximumResearchSearchQueriesPerRun); err != nil {
		return fmt.Errorf("research search max queries per run must be an integer from 1 to %d", MaximumResearchSearchQueriesPerRun)
	}
	return nil
}

// ResearchSearchAvailability is supplied by future infra wiring. It contains
// capability booleans only and never a credential value.
type ResearchSearchAvailability struct {
	AdapterAvailable    bool
	CredentialRequired  bool
	CredentialAvailable bool
}

func (search ResearchSearchConfig) State(availability ResearchSearchAvailability) ResearchSearchProviderState {
	if search.Provider == "" {
		return ResearchSearchDisabled
	}
	if !availability.AdapterAvailable {
		return ResearchSearchUnavailable
	}
	if availability.CredentialRequired && !availability.CredentialAvailable {
		return ResearchSearchMissingCredential
	}
	return ResearchSearchConfigured
}

// ResolveResearchSearch applies the ordinary layered configuration rules and
// extracts the bounded live-search settings.
func ResolveResearchSearch(settings Settings) (ResearchSearchConfig, error) {
	resolved, err := Resolve(settings)
	if err != nil {
		return ResearchSearchConfig{}, err
	}
	return ResearchSearchFromResolved(resolved)
}

// ResearchSearchFromResolved extracts live-search settings from a complete
// resolved configuration without treating default empty scalar values as a
// new explicit configuration layer.
func ResearchSearchFromResolved(resolved Settings) (ResearchSearchConfig, error) {
	provider, providerOK := resolved[KeyResearchSearchProvider].StringField()
	results, resultsOK := resolved[KeyResearchSearchMaxResultsPerQuery].NumberField()
	queries, queriesOK := resolved[KeyResearchSearchMaxQueriesPerRun].NumberField()
	if !providerOK || !resultsOK || !queriesOK {
		return ResearchSearchConfig{}, fmt.Errorf("resolved research search configuration has invalid scalar types")
	}
	search := ResearchSearchConfig{
		Provider: provider, MaxResultsPerQuery: int(results), MaxQueriesPerRun: int(queries),
	}
	if err := search.Validate(); err != nil {
		return ResearchSearchConfig{}, err
	}
	return search, nil
}

func validateResearchSearchProvider(provider string) error {
	if provider == "" {
		return nil
	}
	if provider != strings.TrimSpace(provider) || len(provider) > 64 {
		return fmt.Errorf("research search provider must be an empty value or a lowercase identifier up to 64 characters")
	}
	for index, character := range provider {
		if (character >= 'a' && character <= 'z') || (character >= '0' && character <= '9') ||
			(index > 0 && (character == '-' || character == '_')) {
			continue
		}
		return fmt.Errorf("research search provider must be an empty value or a lowercase identifier up to 64 characters")
	}
	last := provider[len(provider)-1]
	if last == '-' || last == '_' {
		return fmt.Errorf("research search provider must be an empty value or a lowercase identifier up to 64 characters")
	}
	return nil
}
