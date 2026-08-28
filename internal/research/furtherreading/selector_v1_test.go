package furtherreading

import (
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/mishaaac/kelyro/internal/research"
	"github.com/mishaaac/kelyro/internal/research/freshness"
	"github.com/mishaaac/kelyro/internal/research/quality"
)

func TestSelectV1KeepsCommunityPaywallAndStaleTutorialVisible(t *testing.T) {
	t.Parallel()
	candidate := readingCandidate(t, "community-tutorial", research.SourceCommunityArticle, CategoryTutorial)
	candidate.Community = true
	candidate.Access = AccessPaywalled
	candidate.ReadingLevel = ReadingAdvanced
	candidate.Freshness = freshnessAssessment(t, candidate.Source.Kind, 800)

	selection, err := SelectV1(selectionInput(candidate))
	if err != nil {
		t.Fatalf("SelectV1() error = %v", err)
	}
	if len(selection.Items) != 1 {
		t.Fatalf("items = %+v", selection.Items)
	}
	item := selection.Items[0]
	for _, label := range []Label{LabelCommunity, LabelPaywalled, LabelStale} {
		if !containsLabel(item.Labels, label) {
			t.Fatalf("labels = %v, missing %q", item.Labels, label)
		}
	}
	for _, warning := range []WarningCode{WarningPaywalled, WarningStaleTutorial, WarningAboveReadingLevel} {
		if !containsWarning(item.Warnings, warning) {
			t.Fatalf("warnings = %+v, missing %q", item.Warnings, warning)
		}
	}
	if item.Access != AccessPaywalled || !item.Community {
		t.Fatalf("student-visible disclosure = %+v", item)
	}
}

func TestSelectV1DoesNotPromotePrimaryEvidenceIntoReadingMaterial(t *testing.T) {
	t.Parallel()
	denseSpecification := readingCandidate(t, "dense-spec", research.SourceSpecification, CategoryOfficialDeepDive)
	denseSpecification.Quality = qualityAssessment(t, quality.UseEvidence)
	denseSpecification.Trust.Tier = research.AuthorityTierA

	clearCommunity := readingCandidate(t, "clear-community", research.SourceCommunityArticle, CategoryCommunityExplanation)
	clearCommunity.Community = true
	clearCommunity.Trust.State = research.TrustAcceptedSupplement
	clearCommunity.Trust.Tier = research.AuthorityTierC

	rejected := readingCandidate(t, "rejected", research.SourceOfficialTutorial, CategoryTutorial)
	rejected.Trust.State = research.TrustRejected

	selection, err := SelectV1(selectionInput(denseSpecification, clearCommunity, rejected))
	if err != nil {
		t.Fatalf("SelectV1() error = %v", err)
	}
	if got := selectedIDs(selection); !reflect.DeepEqual(got, []string{"clear-community"}) {
		t.Fatalf("selected IDs = %v", got)
	}
	if exclusionFor(selection, denseSpecification.Source.ID) != ExcludedQualityUnsuitable {
		t.Fatalf("dense evidence exclusion = %+v", selection.Excluded)
	}
	if exclusionFor(selection, rejected.Source.ID) != ExcludedTrustNotAccepted {
		t.Fatalf("rejected trust exclusion = %+v", selection.Excluded)
	}
}

func TestSelectV1IsDeterministicAndRewardsCategoryAndOrganizationDiversity(t *testing.T) {
	t.Parallel()
	first := readingCandidate(t, "a-first", research.SourceOfficialTutorial, CategoryTutorial)
	first.Organization = "Publisher One"
	second := readingCandidate(t, "b-same", research.SourceOfficialTutorial, CategoryTutorial)
	second.Organization = "Publisher One"
	diverse := readingCandidate(t, "c-diverse", research.SourcePackageReference, CategoryReference)
	diverse.Organization = "Publisher Two"

	input := selectionInput(first, second, diverse)
	input.Limit = 2
	forward, err := SelectV1(input)
	if err != nil {
		t.Fatal(err)
	}
	input.Candidates = []Candidate{diverse, second, first}
	reverse, err := SelectV1(input)
	if err != nil {
		t.Fatal(err)
	}
	want := []string{"a-first", "c-diverse"}
	if got := selectedIDs(forward); !reflect.DeepEqual(got, want) {
		t.Fatalf("forward selection = %v, want %v", got, want)
	}
	if got := selectedIDs(reverse); !reflect.DeepEqual(got, want) {
		t.Fatalf("reverse selection = %v, want %v", got, want)
	}
	if reverse.AlgorithmVersion != AlgorithmVersionV1 || exclusionFor(reverse, second.Source.ID) != ExcludedLimitReached {
		t.Fatalf("selection metadata = %+v", reverse)
	}
}

func TestSelectV1DeduplicatesByReviewedKeyAndConsidersReadingLevel(t *testing.T) {
	t.Parallel()
	advanced := readingCandidate(t, "advanced-copy", research.SourceOfficialTutorial, CategoryTutorial)
	advanced.ReadingLevel = ReadingAdvanced
	advanced.DuplicateKey = "same lesson"
	exact := readingCandidate(t, "exact-copy", research.SourceOfficialTutorial, CategoryTutorial)
	exact.DuplicateKey = "same lesson"

	selection, err := SelectV1(selectionInput(advanced, exact))
	if err != nil {
		t.Fatal(err)
	}
	if got := selectedIDs(selection); !reflect.DeepEqual(got, []string{"exact-copy"}) {
		t.Fatalf("selected IDs = %v", got)
	}
	if exclusionFor(selection, advanced.Source.ID) != ExcludedDuplicate {
		t.Fatalf("duplicate exclusions = %+v", selection.Excluded)
	}
}

func TestSelectV1ExcludesVersionMismatchButKeepsHistoricalContextWithWarning(t *testing.T) {
	t.Parallel()
	versionOne, _ := research.NewSourceVersion("1")
	versionTwo, _ := research.NewSourceVersion("2")
	mismatch := readingCandidate(t, "mismatch", research.SourceOfficialTutorial, CategoryTutorial)
	mismatch.Source.Version = &versionOne
	mismatch.Source.TemporalScope = research.SourceTemporalVersionBound
	historical := readingCandidate(t, "historical", research.SourceOfficialTutorial, CategoryTutorial)
	historical.Source.TemporalScope = research.SourceTemporalHistorical

	input := selectionInput(mismatch, historical)
	input.Purpose = research.PurposeVersionBehavior
	input.TargetVersion = &versionTwo
	selection, err := SelectV1(input)
	if err != nil {
		t.Fatal(err)
	}
	if got := selectedIDs(selection); !reflect.DeepEqual(got, []string{"historical"}) {
		t.Fatalf("selected IDs = %v", got)
	}
	if exclusionFor(selection, mismatch.Source.ID) != ExcludedNotApplicable {
		t.Fatalf("version exclusions = %+v", selection.Excluded)
	}
	if !containsLabel(selection.Items[0].Labels, LabelHistorical) || !containsWarning(selection.Items[0].Warnings, WarningNonCurrent) {
		t.Fatalf("historical annotations = %+v", selection.Items[0])
	}
}

func TestSelectV1ValidatesBoundsAndExplicitCommunityLabel(t *testing.T) {
	t.Parallel()
	candidate := readingCandidate(t, "community", research.SourceCommunityForum, CategoryTutorial)
	input := selectionInput(candidate)
	if _, err := SelectV1(input); err == nil || !strings.Contains(err.Error(), "explicitly labeled") {
		t.Fatalf("unlabeled community error = %v", err)
	}
	candidate.Community = true
	input = selectionInput(candidate)
	input.Limit = MaximumSelectionCount + 1
	if _, err := SelectV1(input); err == nil || !strings.Contains(err.Error(), "limit") {
		t.Fatalf("oversize limit error = %v", err)
	}
}

func TestInteractiveReadingCategoryRequiresSpecializedPlayground(t *testing.T) {
	t.Parallel()
	candidate := readingCandidate(t, "interactive", research.SourceOther, CategoryInteractiveResource)
	input := selectionInput(candidate)
	if _, err := SelectV1(input); err == nil || !strings.Contains(err.Error(), "playground") {
		t.Fatalf("generic interactive resource error = %v", err)
	}
	candidate.Source.Kind = research.SourcePlayground
	shareable, err := research.NewSourceLocator("https://example.com/interactive/share/abc")
	if err != nil {
		t.Fatal(err)
	}
	candidate.Source.Specialization = &research.SourceSpecialization{
		Kind: research.SourcePlayground, AlgorithmVersion: research.SpecializedSourceMetadataV1,
		Playground: &research.PlaygroundDetails{
			Interactive: true, LanguageRuntime: "Portable runtime",
			Affiliation: research.SourceAffiliationOfficial, ShareableLocator: shareable,
		},
	}
	candidate.Freshness = freshnessAssessment(t, research.SourcePlayground, 5)
	selection, err := SelectV1(selectionInput(candidate))
	if err != nil || len(selection.Items) != 1 || selection.Items[0].Category != CategoryInteractiveResource {
		t.Fatalf("playground selection = (%+v, %v)", selection, err)
	}
	candidate.Source.Specialization.Playground.Affiliation = research.SourceAffiliationCommunity
	if _, err := SelectV1(selectionInput(candidate)); err == nil || !strings.Contains(err.Error(), "community label") {
		t.Fatalf("unlabeled community playground error = %v", err)
	}
	candidate.Community = true
	selection, err = SelectV1(selectionInput(candidate))
	if err != nil || !containsLabel(selection.Items[0].Labels, LabelCommunity) {
		t.Fatalf("community playground selection = (%+v, %v)", selection, err)
	}
}

func TestVideoSupplementCommunityLabelMatchesReviewedAffiliation(t *testing.T) {
	t.Parallel()
	candidate := readingCandidate(t, "community-video", research.SourceVideo, CategoryVideoSupplement)
	published := candidate.Source.CreatedAt
	candidate.Source.Metadata.PublishedAt = &published
	candidate.Source.Video = &research.VideoSupplementMetadata{
		VideoLocator: candidate.Source.Locator, Channel: "Community Sessions", DurationSeconds: 600,
		Affiliation: research.SourceAffiliationCommunity, TranscriptAvailability: research.TranscriptPartial,
		AlgorithmVersion: research.VideoSupplementMetadataV1,
	}
	input := selectionInput(candidate)
	if _, err := SelectV1(input); err == nil || !strings.Contains(err.Error(), "video community label") {
		t.Fatalf("unlabeled community video error = %v", err)
	}
	candidate.Community = true
	selection, err := SelectV1(selectionInput(candidate))
	if err != nil || len(selection.Items) != 1 || !containsLabel(selection.Items[0].Labels, LabelCommunity) {
		t.Fatalf("community video selection = (%+v, %v)", selection, err)
	}
}

func selectionInput(candidates ...Candidate) Input {
	return Input{
		Purpose: research.PurposeCurrentUsage, TargetReadingLevel: ReadingIntermediate,
		Limit: MaximumSelectionCount, Candidates: candidates,
	}
}

func readingCandidate(t *testing.T, id string, kind research.SourceKind, category Category) Candidate {
	t.Helper()
	sourceID, err := research.NewSourceID(id)
	if err != nil {
		t.Fatal(err)
	}
	locator, err := research.NewSourceLocator("https://example.com/" + id)
	if err != nil {
		t.Fatal(err)
	}
	created := readingTimestamp(t, 0)
	return Candidate{
		Source: research.Source{
			ID: sourceID, Kind: kind, Locator: locator, TemporalScope: research.SourceTemporalCurrent,
			Metadata: research.SourceMetadata{Title: "Reading " + id, Publisher: "Example"}, CreatedAt: created,
		},
		Category: category, ReadingLevel: ReadingIntermediate, Access: AccessOpen,
		Organization: "Example Organization", Quality: qualityAssessment(t, quality.UseFurtherReading),
		Trust: research.TrustDecision{
			SourceID: sourceID, State: research.TrustAccepted, Tier: research.AuthorityTierB,
			Reasons: []research.TrustReason{{Code: "test.reviewed", Detail: "Reviewed fixture."}},
			Policy:  "trust-policy-v1", EvaluatedAt: readingTimestamp(t, 1),
		},
		Freshness: freshnessAssessment(t, kind, 5),
	}
}

func qualityAssessment(t *testing.T, use quality.RecommendedUse) quality.Assessment {
	t.Helper()
	values := []float64{.78, .88, .78, .70, .82, .65, .88, .15}
	switch use {
	case quality.UseEvidence:
		values = []float64{.95, .20, .90, .90, .60, .10, .20, .20}
	case quality.UseExample:
		values = []float64{.80, .75, .85, .55, .65, .90, .75, .15}
	}
	scores := make([]research.QualityScore, len(values))
	for index, value := range values {
		var err error
		scores[index], err = research.NewQualityScore(value)
		if err != nil {
			t.Fatal(err)
		}
	}
	assessment, err := (quality.ModelV1{}).Assess(quality.Input{Dimensions: quality.Dimensions{
		AccuracyConfidence: scores[0], Clarity: scores[1], Specificity: scores[2], Depth: scores[3],
		Maintainability: scores[4], Examples: scores[5], Accessibility: scores[6], Noise: scores[7],
	}})
	if err != nil {
		t.Fatal(err)
	}
	if assessment.RecommendedUse != use {
		t.Fatalf("quality fixture use = %q, want %q", assessment.RecommendedUse, use)
	}
	return assessment
}

type readingClock struct{ now research.Timestamp }

func (clock readingClock) Now() research.Timestamp { return clock.now }

func freshnessAssessment(t *testing.T, kind research.SourceKind, ageDays int) freshness.Assessment {
	t.Helper()
	now := readingTimestamp(t, 1000)
	lastVerified, err := research.NewTimestamp(now.Time().Add(-time.Duration(ageDays) * 24 * time.Hour))
	if err != nil {
		t.Fatal(err)
	}
	assessment, err := freshness.NewModelV1(readingClock{now: now}).Assess(freshness.Input{
		LastVerifiedAt: &lastVerified, ClaimType: research.ClaimRecommendation, SourceKind: kind,
	})
	if err != nil {
		t.Fatal(err)
	}
	return assessment
}

func readingTimestamp(t *testing.T, hour int) research.Timestamp {
	t.Helper()
	timestamp, err := research.NewTimestamp(time.Date(2026, 8, 1, hour, 0, 0, 0, time.UTC))
	if err != nil {
		t.Fatal(err)
	}
	return timestamp
}

func selectedIDs(selection Selection) []string {
	result := make([]string, len(selection.Items))
	for index, item := range selection.Items {
		result[index] = item.SourceID.String()
	}
	return result
}

func exclusionFor(selection Selection, sourceID research.SourceID) ExclusionReason {
	for _, exclusion := range selection.Excluded {
		if exclusion.SourceID == sourceID {
			return exclusion.Reason
		}
	}
	return ""
}

func containsLabel(labels []Label, want Label) bool {
	for _, label := range labels {
		if label == want {
			return true
		}
	}
	return false
}

func containsWarning(warnings []Warning, want WarningCode) bool {
	for _, warning := range warnings {
		if warning.Code == want {
			return true
		}
	}
	return false
}
