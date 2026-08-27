package furtherreading

import (
	"fmt"
	"math"
	"sort"
	"strings"

	"github.com/mishaaac/kelyro/internal/research"
	"github.com/mishaaac/kelyro/internal/research/freshness"
	"github.com/mishaaac/kelyro/internal/research/quality"
	"github.com/mishaaac/kelyro/internal/research/temporal"
)

const (
	AlgorithmVersionV1         = "further-reading-selection-v1"
	MaximumCandidateCount      = 128
	MaximumSelectionCount      = 7
	maximumAnnotationLength    = 256
	categoryDiversityBonus     = 0.04
	organizationDiversityBonus = 0.03
)

type Category string

const (
	CategoryOfficialDeepDive     Category = "official_deep_dive"
	CategoryTutorial             Category = "tutorial"
	CategoryInteractiveResource  Category = "interactive_resource"
	CategoryReference            Category = "reference"
	CategoryCommunityExplanation Category = "community_explanation"
	CategoryVideoSupplement      Category = "video_supplement"
	CategorySourceCode           Category = "source_code"
)

func (category Category) Validate() error {
	switch category {
	case CategoryOfficialDeepDive, CategoryTutorial, CategoryInteractiveResource,
		CategoryReference, CategoryCommunityExplanation, CategoryVideoSupplement,
		CategorySourceCode:
		return nil
	default:
		return fmt.Errorf("invalid further reading category %q", category)
	}
}

type ReadingLevel string

const (
	ReadingIntroductory ReadingLevel = "introductory"
	ReadingIntermediate ReadingLevel = "intermediate"
	ReadingAdvanced     ReadingLevel = "advanced"
)

func (level ReadingLevel) Validate() error {
	switch level {
	case ReadingIntroductory, ReadingIntermediate, ReadingAdvanced:
		return nil
	default:
		return fmt.Errorf("invalid further reading level %q", level)
	}
}

type Access string

const (
	AccessOpen                 Access = "open"
	AccessRegistrationRequired Access = "registration_required"
	AccessPaywalled            Access = "paywalled"
	AccessUnknown              Access = "unknown"
)

func (access Access) Validate() error {
	switch access {
	case AccessOpen, AccessRegistrationRequired, AccessPaywalled, AccessUnknown:
		return nil
	default:
		return fmt.Errorf("invalid further reading access %q", access)
	}
}

type Label string

const (
	LabelCommunity            Label = "community"
	LabelRegistrationRequired Label = "registration_required"
	LabelPaywalled            Label = "paywalled"
	LabelAccessUnknown        Label = "access_unknown"
	LabelStale                Label = "stale"
	LabelHistorical           Label = "historical"
)

type WarningCode string

const (
	WarningRegistrationRequired WarningCode = "access.registration_required"
	WarningPaywalled            WarningCode = "access.paywalled"
	WarningAccessUnknown        WarningCode = "access.unknown"
	WarningStaleTutorial        WarningCode = "freshness.stale_tutorial"
	WarningStaleResource        WarningCode = "freshness.stale_resource"
	WarningFreshnessUnknown     WarningCode = "freshness.unknown"
	WarningNonCurrent           WarningCode = "temporal.non_current"
	WarningAboveReadingLevel    WarningCode = "reading_level.above_target"
	WarningOrganizationUnknown  WarningCode = "diversity.organization_unknown"
)

type Warning struct {
	Code   WarningCode
	Detail string
}

type ExclusionReason string

const (
	ExcludedTrustNotAccepted  ExclusionReason = "trust_not_accepted"
	ExcludedQualityUnsuitable ExclusionReason = "quality_not_suitable"
	ExcludedNotApplicable     ExclusionReason = "version_not_applicable"
	ExcludedDuplicate         ExclusionReason = "duplicate"
	ExcludedLimitReached      ExclusionReason = "limit_reached"
)

type Candidate struct {
	Source       research.Source
	Category     Category
	ReadingLevel ReadingLevel
	Access       Access
	Community    bool
	Organization string
	DuplicateKey string
	Quality      quality.Assessment
	Trust        research.TrustDecision
	Freshness    freshness.Assessment
}

func (candidate Candidate) Validate() error {
	if err := candidate.Source.Validate(); err != nil {
		return fmt.Errorf("source: %w", err)
	}
	if err := candidate.Category.Validate(); err != nil {
		return err
	}
	if err := candidate.ReadingLevel.Validate(); err != nil {
		return err
	}
	if err := candidate.Access.Validate(); err != nil {
		return err
	}
	if err := validateOptionalAnnotation("organization", candidate.Organization); err != nil {
		return err
	}
	if err := validateOptionalAnnotation("duplicate key", candidate.DuplicateKey); err != nil {
		return err
	}
	if err := candidate.Quality.Validate(); err != nil {
		return fmt.Errorf("quality assessment: %w", err)
	}
	if err := candidate.Trust.Validate(); err != nil {
		return fmt.Errorf("trust decision: %w", err)
	}
	if candidate.Trust.SourceID != candidate.Source.ID {
		return fmt.Errorf("trust decision source does not match candidate")
	}
	if err := candidate.Freshness.Validate(); err != nil {
		return fmt.Errorf("freshness assessment: %w", err)
	}
	if isCommunityKind(candidate.Source.Kind) && !candidate.Community {
		return fmt.Errorf("community source kind must be explicitly labeled")
	}
	if candidate.Category == CategoryCommunityExplanation && !candidate.Community {
		return fmt.Errorf("community explanation must be explicitly labeled")
	}
	if candidate.Category == CategoryVideoSupplement && candidate.Source.Kind != research.SourceVideo {
		return fmt.Errorf("video supplement requires a video source")
	}
	if candidate.Category == CategorySourceCode && candidate.Source.Kind != research.SourceCode {
		return fmt.Errorf("source code reading requires a source code source")
	}
	if candidate.Category == CategoryOfficialDeepDive && (candidate.Community || !isOfficialDeepDiveKind(candidate.Source.Kind)) {
		return fmt.Errorf("official deep dive requires a non-community official source")
	}
	return nil
}

type Input struct {
	Purpose            research.ResearchPurpose
	TargetVersion      *research.SourceVersion
	TargetReadingLevel ReadingLevel
	Limit              int
	Candidates         []Candidate
}

type Item struct {
	SourceID                  research.SourceID
	Title                     string
	Locator                   research.SourceLocator
	Category                  Category
	ReadingLevel              ReadingLevel
	Access                    Access
	Community                 bool
	Organization              string
	QualityScore              research.QualityScore
	AuthorityTier             research.AuthorityTier
	FreshnessState            research.FreshnessState
	RankScore                 float64
	Labels                    []Label
	Warnings                  []Warning
	QualityAlgorithmVersion   string
	TrustPolicyVersion        string
	FreshnessAlgorithmVersion string
}

type Exclusion struct {
	SourceID research.SourceID
	Reason   ExclusionReason
	Detail   string
}

type Selection struct {
	TargetReadingLevel ReadingLevel
	Limit              int
	Items              []Item
	Excluded           []Exclusion
	AlgorithmVersion   string
}

type rankedCandidate struct {
	candidate Candidate
	temporal  temporal.Assessment
	baseScore float64
}

// SelectV1 returns a bounded selection plus explicit reasons for every valid
// candidate that was not selected. Candidate annotations are reviewed inputs;
// the selector never infers reading level, paywall state, duplicates, or
// organizational independence from external content.
func SelectV1(input Input) (Selection, error) {
	if err := validateInput(input); err != nil {
		return Selection{}, fmt.Errorf("select %s: %w", AlgorithmVersionV1, err)
	}
	selection := Selection{
		TargetReadingLevel: input.TargetReadingLevel,
		Limit:              input.Limit,
		AlgorithmVersion:   AlgorithmVersionV1,
	}
	eligible := make([]rankedCandidate, 0, len(input.Candidates))
	for _, candidate := range input.Candidates {
		temporalAssessment, err := temporal.AssessV1(temporal.Input{
			Source: candidate.Source, Purpose: input.Purpose, TargetVersion: input.TargetVersion,
		})
		if err != nil {
			return Selection{}, fmt.Errorf("select %s source %q: %w", AlgorithmVersionV1, candidate.Source.ID, err)
		}
		switch {
		case candidate.Trust.State != research.TrustAccepted && candidate.Trust.State != research.TrustAcceptedSupplement:
			selection.Excluded = append(selection.Excluded, exclusion(candidate, ExcludedTrustNotAccepted, "The latest reviewed trust decision does not accept this resource for use."))
		case candidate.Quality.RecommendedUse != quality.UseFurtherReading && candidate.Quality.RecommendedUse != quality.UseExample:
			selection.Excluded = append(selection.Excluded, exclusion(candidate, ExcludedQualityUnsuitable, "The reviewed resource quality assessment does not recommend a student-facing reading use."))
		case temporalAssessment.Role == temporal.RoleNotApplicable:
			selection.Excluded = append(selection.Excluded, exclusion(candidate, ExcludedNotApplicable, "The resource does not apply to the requested version."))
		default:
			eligible = append(eligible, rankedCandidate{
				candidate: candidate,
				temporal:  temporalAssessment,
				baseScore: baseScore(candidate, temporalAssessment, input.TargetReadingLevel),
			})
		}
	}

	eligible, duplicateExclusions := removeDuplicates(eligible)
	selection.Excluded = append(selection.Excluded, duplicateExclusions...)
	selectedCategories := make(map[Category]struct{})
	selectedOrganizations := make(map[string]struct{})
	for len(selection.Items) < input.Limit && len(eligible) > 0 {
		bestIndex := 0
		bestScore := selectionScore(eligible[0], selectedCategories, selectedOrganizations)
		for index := 1; index < len(eligible); index++ {
			score := selectionScore(eligible[index], selectedCategories, selectedOrganizations)
			if betterSelection(eligible[index], score, eligible[bestIndex], bestScore) {
				bestIndex, bestScore = index, score
			}
		}
		selected := eligible[bestIndex]
		selection.Items = append(selection.Items, makeItem(selected, bestScore, input.TargetReadingLevel))
		selectedCategories[selected.candidate.Category] = struct{}{}
		if organization := normalizedOrganization(selected.candidate.Organization); organization != "" {
			selectedOrganizations[organization] = struct{}{}
		}
		eligible = append(eligible[:bestIndex], eligible[bestIndex+1:]...)
	}
	for _, candidate := range eligible {
		selection.Excluded = append(selection.Excluded, exclusion(candidate.candidate, ExcludedLimitReached, "The bounded selection limit was reached."))
	}
	sort.Slice(selection.Excluded, func(i, j int) bool {
		if selection.Excluded[i].SourceID != selection.Excluded[j].SourceID {
			return selection.Excluded[i].SourceID.String() < selection.Excluded[j].SourceID.String()
		}
		return selection.Excluded[i].Reason < selection.Excluded[j].Reason
	})
	return selection, nil
}

func validateInput(input Input) error {
	if err := input.Purpose.Validate(); err != nil {
		return err
	}
	if input.TargetVersion != nil {
		if err := input.TargetVersion.Validate(); err != nil {
			return fmt.Errorf("target version: %w", err)
		}
	}
	if err := input.TargetReadingLevel.Validate(); err != nil {
		return err
	}
	if input.Limit < 1 || input.Limit > MaximumSelectionCount {
		return fmt.Errorf("further reading limit must be between 1 and %d", MaximumSelectionCount)
	}
	if len(input.Candidates) == 0 || len(input.Candidates) > MaximumCandidateCount {
		return fmt.Errorf("further reading candidate count must be between 1 and %d", MaximumCandidateCount)
	}
	seen := make(map[research.SourceID]struct{}, len(input.Candidates))
	for index, candidate := range input.Candidates {
		if err := candidate.Validate(); err != nil {
			return fmt.Errorf("candidate %d: %w", index, err)
		}
		if _, exists := seen[candidate.Source.ID]; exists {
			return fmt.Errorf("duplicate further reading source %q", candidate.Source.ID)
		}
		seen[candidate.Source.ID] = struct{}{}
	}
	return nil
}

func removeDuplicates(candidates []rankedCandidate) ([]rankedCandidate, []Exclusion) {
	byKey := make(map[string]rankedCandidate, len(candidates))
	excluded := make([]Exclusion, 0)
	for _, candidate := range candidates {
		key := candidate.candidate.DuplicateKey
		if key == "" {
			key = "source:" + candidate.candidate.Source.ID.String()
		} else {
			key = "reviewed:" + strings.ToLower(key)
		}
		current, exists := byKey[key]
		if !exists {
			byKey[key] = candidate
			continue
		}
		if betterBase(candidate, current) {
			excluded = append(excluded, exclusion(current.candidate, ExcludedDuplicate, fmt.Sprintf("A stronger reviewed resource in duplicate group %q was retained.", candidate.candidate.DuplicateKey)))
			byKey[key] = candidate
		} else {
			excluded = append(excluded, exclusion(candidate.candidate, ExcludedDuplicate, fmt.Sprintf("A stronger reviewed resource in duplicate group %q was retained.", candidate.candidate.DuplicateKey)))
		}
	}
	result := make([]rankedCandidate, 0, len(byKey))
	for _, candidate := range byKey {
		result = append(result, candidate)
	}
	sort.Slice(result, func(i, j int) bool {
		return result[i].candidate.Source.ID.String() < result[j].candidate.Source.ID.String()
	})
	return result, excluded
}

func baseScore(candidate Candidate, temporalAssessment temporal.Assessment, target ReadingLevel) float64 {
	return candidate.Quality.Score.Value()*0.43 +
		readingFit(candidate.ReadingLevel, target)*0.18 +
		candidate.Freshness.Score.Value()*0.14 +
		authorityValue(candidate.Trust.Tier)*0.08 +
		accessValue(candidate.Access)*0.05 +
		temporalValue(temporalAssessment.Role)*0.05
}

func selectionScore(candidate rankedCandidate, categories map[Category]struct{}, organizations map[string]struct{}) float64 {
	score := candidate.baseScore
	if _, exists := categories[candidate.candidate.Category]; !exists {
		score += categoryDiversityBonus
	}
	if organization := normalizedOrganization(candidate.candidate.Organization); organization != "" {
		if _, exists := organizations[organization]; !exists {
			score += organizationDiversityBonus
		}
	}
	return math.Min(1, score)
}

func betterSelection(candidate rankedCandidate, candidateScore float64, current rankedCandidate, currentScore float64) bool {
	if candidateScore != currentScore {
		return candidateScore > currentScore
	}
	return betterBase(candidate, current)
}

func betterBase(candidate, current rankedCandidate) bool {
	if candidate.baseScore != current.baseScore {
		return candidate.baseScore > current.baseScore
	}
	if candidate.candidate.Quality.Score.Value() != current.candidate.Quality.Score.Value() {
		return candidate.candidate.Quality.Score.Value() > current.candidate.Quality.Score.Value()
	}
	if authorityValue(candidate.candidate.Trust.Tier) != authorityValue(current.candidate.Trust.Tier) {
		return authorityValue(candidate.candidate.Trust.Tier) > authorityValue(current.candidate.Trust.Tier)
	}
	return candidate.candidate.Source.ID.String() < current.candidate.Source.ID.String()
}

func makeItem(candidate rankedCandidate, rankScore float64, target ReadingLevel) Item {
	labels, warnings := annotations(candidate, target)
	return Item{
		SourceID: candidate.candidate.Source.ID, Title: candidate.candidate.Source.Metadata.Title,
		Locator: candidate.candidate.Source.Locator, Category: candidate.candidate.Category,
		ReadingLevel: candidate.candidate.ReadingLevel, Access: candidate.candidate.Access,
		Community: candidate.candidate.Community, Organization: candidate.candidate.Organization,
		QualityScore: candidate.candidate.Quality.Score, AuthorityTier: candidate.candidate.Trust.Tier,
		FreshnessState: candidate.candidate.Freshness.State, RankScore: rankScore,
		Labels: labels, Warnings: warnings,
		QualityAlgorithmVersion:   candidate.candidate.Quality.AlgorithmVersion,
		TrustPolicyVersion:        candidate.candidate.Trust.Policy,
		FreshnessAlgorithmVersion: candidate.candidate.Freshness.AlgorithmVersion,
	}
}

func annotations(candidate rankedCandidate, target ReadingLevel) ([]Label, []Warning) {
	labels := make([]Label, 0, 4)
	warnings := make([]Warning, 0, 5)
	if candidate.candidate.Community {
		labels = append(labels, LabelCommunity)
	}
	switch candidate.candidate.Access {
	case AccessRegistrationRequired:
		labels = append(labels, LabelRegistrationRequired)
		warnings = append(warnings, Warning{Code: WarningRegistrationRequired, Detail: "Access requires account registration."})
	case AccessPaywalled:
		labels = append(labels, LabelPaywalled)
		warnings = append(warnings, Warning{Code: WarningPaywalled, Detail: "The resource is behind a paywall."})
	case AccessUnknown:
		labels = append(labels, LabelAccessUnknown)
		warnings = append(warnings, Warning{Code: WarningAccessUnknown, Detail: "The resource access conditions have not been verified."})
	}
	switch candidate.candidate.Freshness.State {
	case research.FreshnessStale:
		labels = append(labels, LabelStale)
		if candidate.candidate.Category == CategoryTutorial {
			warnings = append(warnings, Warning{Code: WarningStaleTutorial, Detail: "This tutorial is stale; verify its instructions before use."})
		} else {
			warnings = append(warnings, Warning{Code: WarningStaleResource, Detail: "This resource is stale; verify current applicability before use."})
		}
	case research.FreshnessUnknown:
		warnings = append(warnings, Warning{Code: WarningFreshnessUnknown, Detail: "Freshness has not been established."})
	}
	if candidate.temporal.Role == temporal.RoleHistoricalContext {
		labels = append(labels, LabelHistorical)
		detail := candidate.temporal.Warning
		if detail == "" {
			detail = "This resource is historical context, not current guidance."
		}
		warnings = append(warnings, Warning{Code: WarningNonCurrent, Detail: detail})
	}
	if readingRank(candidate.candidate.ReadingLevel) > readingRank(target) {
		warnings = append(warnings, Warning{Code: WarningAboveReadingLevel, Detail: "The resource is above the requested reading level."})
	}
	if candidate.candidate.Organization == "" {
		warnings = append(warnings, Warning{Code: WarningOrganizationUnknown, Detail: "The publishing organization is unknown; organizational diversity is not confirmed."})
	}
	return labels, warnings
}

func exclusion(candidate Candidate, reason ExclusionReason, detail string) Exclusion {
	return Exclusion{SourceID: candidate.Source.ID, Reason: reason, Detail: detail}
}

func readingFit(candidate, target ReadingLevel) float64 {
	distance := readingRank(candidate) - readingRank(target)
	if distance < 0 {
		distance = -distance
	}
	switch distance {
	case 0:
		return 1
	case 1:
		return 0.65
	default:
		return 0.30
	}
}

func readingRank(level ReadingLevel) int {
	switch level {
	case ReadingIntroductory:
		return 0
	case ReadingIntermediate:
		return 1
	case ReadingAdvanced:
		return 2
	default:
		return 3
	}
}

func authorityValue(tier research.AuthorityTier) float64 {
	switch tier {
	case research.AuthorityTierA:
		return 1
	case research.AuthorityTierB:
		return 0.80
	case research.AuthorityTierC:
		return 0.60
	case research.AuthorityTierD:
		return 0.35
	case research.AuthorityTierE:
		return 0.10
	default:
		return 0
	}
}

func accessValue(access Access) float64 {
	switch access {
	case AccessOpen:
		return 1
	case AccessRegistrationRequired:
		return 0.65
	case AccessPaywalled:
		return 0.35
	case AccessUnknown:
		return 0.25
	default:
		return 0
	}
}

func temporalValue(role temporal.GuidanceRole) float64 {
	switch role {
	case temporal.RoleCurrentGuidance, temporal.RoleVersionAuthority:
		return 1
	case temporal.RoleHistoricalContext:
		return 0.35
	default:
		return 0
	}
}

func normalizedOrganization(value string) string { return strings.ToLower(strings.TrimSpace(value)) }

func validateOptionalAnnotation(name, value string) error {
	if value == "" {
		return nil
	}
	if value != strings.TrimSpace(value) || len(value) > maximumAnnotationLength || strings.ContainsAny(value, "\r\n") {
		return fmt.Errorf("further reading %s is invalid", name)
	}
	return nil
}

func isCommunityKind(kind research.SourceKind) bool {
	return kind == research.SourceCommunityArticle || kind == research.SourceCommunityForum
}

func isOfficialDeepDiveKind(kind research.SourceKind) bool {
	switch kind {
	case research.SourceOfficialDocumentation, research.SourceSpecification,
		research.SourceStandard, research.SourceOfficialBlog,
		research.SourcePackageReference, research.SourceOfficialTutorial:
		return true
	default:
		return false
	}
}
