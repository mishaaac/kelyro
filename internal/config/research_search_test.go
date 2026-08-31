package config

import "testing"

func TestResolveResearchSearchUsesSafeDefaultsAndLayeredOverrides(t *testing.T) {
	t.Parallel()

	defaults, err := ResolveResearchSearch(nil)
	if err != nil {
		t.Fatal(err)
	}
	if defaults.Provider != "" || defaults.MaxResultsPerQuery != 8 || defaults.MaxQueriesPerRun != 4 {
		t.Fatalf("research search defaults = %+v", defaults)
	}
	if state := defaults.State(ResearchSearchAvailability{AdapterAvailable: true, CredentialAvailable: true}); state != ResearchSearchDisabled {
		t.Fatalf("disabled state = %q", state)
	}

	configured, err := ResolveResearchSearch(Settings{
		KeyResearchSearchProvider:           StringValue("reference-api"),
		KeyResearchSearchMaxResultsPerQuery: NumberValue(12),
		KeyResearchSearchMaxQueriesPerRun:   NumberValue(3),
	})
	if err != nil {
		t.Fatal(err)
	}
	if configured.Provider != "reference-api" || configured.MaxResultsPerQuery != 12 || configured.MaxQueriesPerRun != 3 {
		t.Fatalf("research search override = %+v", configured)
	}
}

func TestResearchSearchProviderReadinessStatesContainNoCredentials(t *testing.T) {
	t.Parallel()

	search := ResearchSearchConfig{Provider: "reference-api", MaxResultsPerQuery: 8, MaxQueriesPerRun: 4}
	for _, test := range []struct {
		name         string
		availability ResearchSearchAvailability
		want         ResearchSearchProviderState
	}{
		{name: "unavailable adapter", want: ResearchSearchUnavailable},
		{name: "missing credential", availability: ResearchSearchAvailability{AdapterAvailable: true, CredentialRequired: true}, want: ResearchSearchMissingCredential},
		{name: "configured authenticated", availability: ResearchSearchAvailability{AdapterAvailable: true, CredentialRequired: true, CredentialAvailable: true}, want: ResearchSearchConfigured},
		{name: "configured unauthenticated", availability: ResearchSearchAvailability{AdapterAvailable: true}, want: ResearchSearchConfigured},
	} {
		t.Run(test.name, func(t *testing.T) {
			state := search.State(test.availability)
			if state != test.want {
				t.Fatalf("State() = %q, want %q", state, test.want)
			}
			if err := state.Validate(); err != nil {
				t.Fatal(err)
			}
		})
	}
	if err := ResearchSearchProviderState("invented").Validate(); err == nil {
		t.Fatal("invented provider state passed validation")
	}
}

func TestResearchSearchConfigurationRejectsUnsafeOrUnboundedValues(t *testing.T) {
	t.Parallel()

	for _, test := range []struct {
		name  string
		key   string
		value string
	}{
		{name: "uppercase provider", key: KeyResearchSearchProvider, value: "Vendor"},
		{name: "provider path", key: KeyResearchSearchProvider, value: "vendor/api"},
		{name: "provider trailing separator", key: KeyResearchSearchProvider, value: "vendor-"},
		{name: "zero results", key: KeyResearchSearchMaxResultsPerQuery, value: "0"},
		{name: "fractional results", key: KeyResearchSearchMaxResultsPerQuery, value: "1.5"},
		{name: "excess results", key: KeyResearchSearchMaxResultsPerQuery, value: "101"},
		{name: "zero queries", key: KeyResearchSearchMaxQueriesPerRun, value: "0"},
		{name: "excess queries", key: KeyResearchSearchMaxQueriesPerRun, value: "9"},
		{name: "credential in config", key: "research.search.api_key", value: "secret"},
	} {
		t.Run(test.name, func(t *testing.T) {
			if _, err := ParseValue(test.key, test.value); err == nil {
				t.Fatal("unsafe research search configuration passed validation")
			}
		})
	}
}
