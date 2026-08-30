package temporal_test

import (
	"strings"
	"testing"
	"time"

	"github.com/mishaaac/kelyro/internal/research"
	"github.com/mishaaac/kelyro/internal/research/temporal"
)

func TestArchivedDocumentationIsHistoricalContextForCurrentGuidance(t *testing.T) {
	t.Parallel()
	source := temporalSource(t, "archived-docs", research.SourceOfficialDocumentation, research.SourceTemporalArchived, "1.0")
	assessment, err := temporal.AssessV1(temporal.Input{Source: source, Purpose: research.PurposeCurrentUsage})
	if err != nil {
		t.Fatal(err)
	}
	if assessment.Role != temporal.RoleHistoricalContext || assessment.Warning == "" ||
		!strings.Contains(assessment.Warning, "Archived source") {
		t.Fatalf("archived assessment = %+v", assessment)
	}
}

func TestOldReleaseNotesCanBePrimaryAuthorityForMatchingVersion(t *testing.T) {
	t.Parallel()
	source := temporalSource(t, "old-release", research.SourceReleaseNotes, research.SourceTemporalVersionBound, "1.8")
	target := temporalVersion(t, "1.8")
	assessment, err := temporal.AssessV1(temporal.Input{
		Source: source, Purpose: research.PurposeVersionBehavior, TargetVersion: &target,
	})
	if err != nil {
		t.Fatal(err)
	}
	if assessment.Role != temporal.RoleVersionAuthority || assessment.Warning == "" ||
		assessment.AlgorithmVersion != research.SourceTemporalPolicyV1 {
		t.Fatalf("old release assessment = %+v", assessment)
	}
	other := temporalVersion(t, "2.0")
	assessment, err = temporal.AssessV1(temporal.Input{
		Source: source, Purpose: research.PurposeVersionBehavior, TargetVersion: &other,
	})
	if err != nil || assessment.Role != temporal.RoleNotApplicable {
		t.Fatalf("mismatched release assessment = (%+v, %v)", assessment, err)
	}
}

func TestCurrentAndHistoricalGuidanceRemainDistinct(t *testing.T) {
	t.Parallel()
	current := temporalSource(t, "current", research.SourceOfficialDocumentation, research.SourceTemporalCurrent, "2.0")
	historical := temporalSource(t, "historical", research.SourceOfficialDocumentation, research.SourceTemporalHistorical, "1.0")
	currentAssessment, err := temporal.AssessV1(temporal.Input{Source: current, Purpose: research.PurposeCurrentUsage})
	if err != nil {
		t.Fatal(err)
	}
	historicalAssessment, err := temporal.AssessV1(temporal.Input{Source: historical, Purpose: research.PurposeCurrentUsage})
	if err != nil {
		t.Fatal(err)
	}
	if currentAssessment.Role != temporal.RoleCurrentGuidance || currentAssessment.Warning != "" ||
		historicalAssessment.Role != temporal.RoleHistoricalContext || historicalAssessment.Warning == "" {
		t.Fatalf("current/historical assessments = (%+v, %+v)", currentAssessment, historicalAssessment)
	}
}

func temporalSource(t *testing.T, suffix string, kind research.SourceKind, scope research.SourceTemporalScope, versionText string) research.Source {
	t.Helper()
	id, _ := research.NewSourceID("source." + suffix)
	locator, _ := research.NewSourceLocator("https://example.test/" + suffix)
	created, _ := research.NewTimestamp(time.Date(2026, time.August, 26, 12, 0, 0, 0, time.UTC))
	version := temporalVersion(t, versionText)
	return research.Source{
		ID: id, Kind: kind, Locator: locator, Version: &version, TemporalScope: scope,
		Metadata: research.SourceMetadata{Title: "Fixture " + suffix}, CreatedAt: created,
	}
}

func temporalVersion(t *testing.T, value string) research.SourceVersion {
	t.Helper()
	version, err := research.NewSourceVersion(value)
	if err != nil {
		t.Fatal(err)
	}
	return version
}
