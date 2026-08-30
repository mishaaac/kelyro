package tui

import (
	"errors"
	"fmt"
	"strings"
	"testing"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/mishaaac/kelyro/internal/app"
	"github.com/mishaaac/kelyro/internal/research"
	researchapp "github.com/mishaaac/kelyro/internal/research/application"
)

func TestResearchTransparencyNavigationLoadsOnlyOnCommands(t *testing.T) {
	result, source, locator := researchTUIResult(t)
	service := &fakeService{result: result}
	native := &fakeResearchPlatform{}
	model := readyModel(service)
	model.platform = native

	researchScreen, command := model.Update(keyMsg("R"))
	if command == nil || researchScreen.(Model).screen != screenResearch {
		t.Fatal("R did not open Research")
	}
	loadedResearch, _ := researchScreen.(Model).Update(command())
	for _, expected := range []string{"Research", "Runs: 3", "research-cost-control-v1", "[s] Sources", "[c] Conflicts", "[f] Freshness"} {
		if !strings.Contains(loadedResearch.(Model).View(), expected) {
			t.Errorf("research view missing %q:\n%s", expected, loadedResearch.(Model).View())
		}
	}

	sourcesScreen, command := loadedResearch.(Model).Update(keyMsg("s"))
	loadedSources, _ := sourcesScreen.(Model).Update(command())
	for _, expected := range []string{"Sources", "Go 1.22 specification", "official documentation", "tier A", "stale 40%", "Historical source"} {
		if !strings.Contains(loadedSources.(Model).View(), expected) {
			t.Errorf("sources view missing %q:\n%s", expected, loadedSources.(Model).View())
		}
	}

	detailScreen, command := loadedSources.(Model).Update(tea.KeyMsg{Type: tea.KeyEnter})
	loadedDetail, _ := detailScreen.(Model).Update(command())
	detail := loadedDetail.(Model)
	for _, expected := range []string{"Source detail", "Authority: tier A", "Freshness: stale", "Version scope: historical · go1.22", "go.dev/ref/spec", "Historical source for version", "Latest snapshot"} {
		if !strings.Contains(detail.View(), expected) {
			t.Errorf("source detail missing %q:\n%s", expected, detail.View())
		}
	}
	if strings.Contains(detail.View(), "utm_campaign") {
		t.Fatalf("source detail exposed query string:\n%s", detail.View())
	}

	opening, openCommand := detail.Update(keyMsg("o"))
	if openCommand == nil || !opening.(Model).opening {
		t.Fatal("o did not start URL opening")
	}
	opened, _ := opening.(Model).Update(openCommand())
	if native.opened != locator.String() || !strings.Contains(opened.(Model).notice, "default browser") {
		t.Fatalf("opened URL=%q notice=%q", native.opened, opened.(Model).notice)
	}
	if source.ID != detail.sourceDetail.Source.ID {
		t.Fatalf("source detail ID = %s", detail.sourceDetail.Source.ID)
	}

	calls := len(service.executed)
	_ = opened.(Model).View()
	_ = opened.(Model).View()
	if len(service.executed) != calls {
		t.Fatal("render triggered application I/O")
	}
}

func TestConflictsOpenClaimProvenanceAndFreshness(t *testing.T) {
	result, _, _ := researchTUIResult(t)
	service := &fakeService{result: result}
	model := readyModel(service)
	model.screen = screenResearch

	conflictsScreen, command := model.Update(keyMsg("c"))
	loadedConflicts, _ := conflictsScreen.(Model).Update(command())
	if view := loadedConflicts.(Model).View(); !strings.Contains(view, "version mismatch") || !strings.Contains(view, "[claim.go-range]") {
		t.Fatalf("conflicts view:\n%s", view)
	}

	claimScreen, command := loadedConflicts.(Model).Update(tea.KeyMsg{Type: tea.KeyEnter})
	loadedClaim, _ := claimScreen.(Model).Update(command())
	for _, expected := range []string{"Claim detail", "claim.go-range", "Provenance", "Go permits range over integer values", "extractor-v1"} {
		if !strings.Contains(loadedClaim.(Model).View(), expected) {
			t.Errorf("claim view missing %q:\n%s", expected, loadedClaim.(Model).View())
		}
	}

	back, _ := loadedClaim.(Model).Update(tea.KeyMsg{Type: tea.KeyEsc})
	if back.(Model).screen != screenConflicts {
		t.Fatalf("claim Esc screen = %d", back.(Model).screen)
	}

	model.screen = screenResearch
	freshnessScreen, command := model.Update(keyMsg("f"))
	loadedFreshness, _ := freshnessScreen.(Model).Update(command())
	for _, expected := range []string{"Freshness", "source.go-spec", "State: stale", "Last verified:", "manual request"} {
		if !strings.Contains(loadedFreshness.(Model).View(), expected) {
			t.Errorf("freshness view missing %q:\n%s", expected, loadedFreshness.(Model).View())
		}
	}
}

func TestSourceTransparencyShowsUnknownInsteadOfInventingAuthority(t *testing.T) {
	_, source, _ := researchTUIResult(t)
	model := readyModel(&fakeService{})
	model.screen = screenSourceDetail
	model.sourceDetail = &app.SourceCLIView{Source: source}
	view := model.View()
	if !strings.Contains(view, "Authority: unknown") || !strings.Contains(view, "Freshness: unknown") {
		t.Fatalf("unknown transparency view:\n%s", view)
	}
}

func TestSourceOpenFailureIsVisibleAndRetryable(t *testing.T) {
	_, source, _ := researchTUIResult(t)
	model := readyModel(&fakeService{})
	model.screen = screenSourceDetail
	model.sourceDetail = &app.SourceCLIView{Source: source}
	model.platform = &fakeResearchPlatform{err: errors.New("no opener")}
	opening, command := model.Update(keyMsg("o"))
	failed, _ := opening.(Model).Update(command())
	if failed.(Model).opening || !strings.Contains(failed.(Model).notice, "no opener") {
		t.Fatalf("failed open state = %#v", failed)
	}
}

func TestLongSourceListKeepsSelectionVisible(t *testing.T) {
	_, source, _ := researchTUIResult(t)
	model := readyModel(&fakeService{})
	model.screen = screenSources
	model.height = 10
	model.sources = make([]app.SourceCLIView, 20)
	for index := range model.sources {
		current := source
		current.Metadata.Title = fmt.Sprintf("Stored source %02d", index)
		model.sources[index] = app.SourceCLIView{Source: current}
	}
	for range 19 {
		updated, _ := model.Update(tea.KeyMsg{Type: tea.KeyDown})
		model = updated.(Model)
	}
	if view := model.View(); !strings.Contains(view, "> Stored source 19") || model.scrollOffset == 0 {
		t.Fatalf("selected source was not revealed:\n%s", view)
	}
}

func researchTUIResult(t *testing.T) (app.Result, research.Source, research.SourceLocator) {
	t.Helper()
	at := researchTimestamp(t, time.Date(2026, 8, 26, 14, 0, 0, 0, time.UTC))
	due := researchTimestamp(t, time.Date(2026, 8, 27, 14, 0, 0, 0, time.UTC))
	version, err := research.NewSourceVersion("go1.22")
	if err != nil {
		t.Fatal(err)
	}
	locator, err := research.NewSourceLocator("https://go.dev/ref/spec?utm_campaign=a-very-long-tracking-value")
	if err != nil {
		t.Fatal(err)
	}
	sourceID, _ := research.NewSourceID("source.go-spec")
	source := research.Source{ID: sourceID, Kind: research.SourceOfficialDocumentation, Locator: locator, Version: &version,
		TemporalScope: research.SourceTemporalHistorical, Metadata: research.SourceMetadata{Title: "Go 1.22 specification", Publisher: "The Go Authors"}, CreatedAt: at}
	trust := research.TrustDecision{SourceID: sourceID, State: research.TrustAccepted, Tier: research.AuthorityTierA,
		Reasons: []research.TrustReason{{Code: "normative"}}, Policy: "trust-policy-v1", EvaluatedAt: at}
	score, _ := research.NewFreshnessScore(.4)
	subjectID, _ := research.NewID(sourceID.String())
	freshness := researchapp.FreshnessRecord{SubjectID: subjectID, State: research.FreshnessStale, Score: score, LastVerifiedAt: at,
		NextVerifyAt: &due, VerificationReason: research.VerificationManualRequest, Priority: research.VerificationPriorityHigh,
		AlgorithmVersion: research.FreshnessAlgorithmV1, SchedulingAlgorithmVersion: research.RefreshSchedulingAlgorithmV1}
	snapshotID, _ := research.NewID("snapshot.go-spec")
	snapshot := research.SourceSnapshot{ID: snapshotID, SourceID: sourceID, Locator: locator, FetchedAt: at,
		Fetch: research.FetchMetadata{StatusCode: 200, ContentType: "text/html", ContentHash: strings.Repeat("a", 64), FetchVersion: "fetch-v1"}}
	view := app.SourceCLIView{Source: source, LatestSnapshot: &snapshot, TrustDecision: &trust, Freshness: &freshness}

	claimID, _ := research.NewClaimID("claim.go-range")
	otherClaimID, _ := research.NewClaimID("claim.go-range-old")
	conflictID, _ := research.NewID("conflict.go-range")
	confidence, _ := research.NewClaimConfidence(.8)
	conflict := research.Conflict{ID: conflictID, ClaimIDs: []research.ClaimID{claimID, otherClaimID}, Type: research.ConflictVersionMismatch,
		Reason: "The stored claims apply to different Go versions.", Confidence: confidence, Unresolved: true, DetectedAt: at,
		AlgorithmVersion: research.ConflictResolverAlgorithmV1}
	graphID, _ := research.NewID("graph.go-range")
	requestID, _ := research.NewID("request.go-range")
	runID, _ := research.NewID("run.go-range")
	evidenceID, _ := research.NewID("evidence.go-range")
	claimNodeID, _ := research.NewID(claimID.String())
	graph := research.ProvenanceGraph{ID: graphID, ClaimID: claimID, RecordedAt: at, AlgorithmVersion: research.ProvenanceGraphAlgorithmV1,
		Nodes: []research.ProvenanceNode{
			{ID: requestID, Kind: research.ProvenanceRequest, Label: "Go range behavior", OccurredAt: at},
			{ID: runID, Kind: research.ProvenanceRun, Label: "Stored research run", OccurredAt: at},
			{ID: evidenceID, Kind: research.ProvenanceEvidence, Label: "Specification excerpt", OccurredAt: at, ToolVersion: "extractor-v1"},
			{ID: claimNodeID, Kind: research.ProvenanceClaim, Label: "Go permits range over integer values", OccurredAt: at},
		}}
	stats := researchapp.ResearchCostStats{Runs: 3, BudgetStoppedRuns: 1, AsOf: at, AlgorithmVersion: research.ResearchCostControlAlgorithmV1}
	return app.Result{ResearchCostStats: &stats, SourceTransparency: []app.SourceCLIView{view}, Source: &view,
		SourceConflicts: []research.Conflict{conflict}, ProvenanceGraph: &graph, StaleSources: []researchapp.FreshnessRecord{freshness}}, source, locator
}

func researchTimestamp(t *testing.T, value time.Time) research.Timestamp {
	t.Helper()
	timestamp, err := research.NewTimestamp(value)
	if err != nil {
		t.Fatal(err)
	}
	return timestamp
}

func keyMsg(value string) tea.KeyMsg {
	return tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune(value)}
}

type fakeResearchPlatform struct {
	opened string
	err    error
}

func (*fakeResearchPlatform) Name() string                      { return "test" }
func (*fakeResearchPlatform) UserHomeDir() (string, error)      { return "/home/test", nil }
func (*fakeResearchPlatform) UserConfigDir() (string, error)    { return "/config", nil }
func (*fakeResearchPlatform) UserCacheDir() (string, error)     { return "/cache", nil }
func (*fakeResearchPlatform) CommandPath(string) (string, bool) { return "", false }
func (*fakeResearchPlatform) OpenPath(string) error             { return nil }
func (platform *fakeResearchPlatform) OpenURL(locator string) error {
	platform.opened = locator
	return platform.err
}
