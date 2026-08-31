package cli

import (
	"strings"
	"testing"

	"github.com/mishaaac/kelyro/internal/doctor"
)

func TestFormatDiagnosticsRendersResearchSearchReadiness(t *testing.T) {
	t.Parallel()

	output := formatDiagnostics(doctor.Report{Checks: []doctor.Check{
		{ID: "research.search.network", Section: doctor.SectionResearchSearch, DisplayName: "Network policy enabled", Requirement: doctor.Optional, State: doctor.Pass, Detail: "privacy.allow_network=true"},
		{ID: "research.search.provider", Section: doctor.SectionResearchSearch, DisplayName: "Provider configured", Requirement: doctor.Optional, State: doctor.Pass, Detail: "configured"},
		{ID: "research.search.credential", Section: doctor.SectionResearchSearch, DisplayName: "Credential available", Requirement: doctor.Optional, State: doctor.Pass, Detail: "available"},
	}})
	for _, want := range []string{"Research Search", "✓ Network policy enabled", "✓ Provider configured", "✓ Credential available"} {
		if !strings.Contains(output, want) {
			t.Fatalf("formatted readiness = %q, want %q", output, want)
		}
	}
}
