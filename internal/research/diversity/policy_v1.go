package diversity

import (
	"fmt"
	"sort"
	"strings"
	"unicode"

	"github.com/mishaaac/kelyro/internal/research"
)

const (
	AlgorithmVersionV1    = "source-diversity-v1"
	MaximumSourceCount    = 128
	maximumAnnotationSize = 256
)

type Perspective string

const (
	PerspectiveNormative         Perspective = "normative"
	PerspectiveMaintainer        Perspective = "maintainer"
	PerspectiveVendor            Perspective = "vendor"
	PerspectivePractitioner      Perspective = "practitioner"
	PerspectiveAcademic          Perspective = "academic"
	PerspectiveIndependentReview Perspective = "independent_review"
	PerspectiveHistorical        Perspective = "historical"
	PerspectiveUnknown           Perspective = "unknown"
)

func (perspective Perspective) Validate() error {
	switch perspective {
	case PerspectiveNormative, PerspectiveMaintainer, PerspectiveVendor,
		PerspectivePractitioner, PerspectiveAcademic, PerspectiveIndependentReview,
		PerspectiveHistorical, PerspectiveUnknown:
		return nil
	default:
		return fmt.Errorf("invalid source diversity perspective %q", perspective)
	}
}

type TechnicalRole string

const (
	TechnicalRoleReference      TechnicalRole = "reference"
	TechnicalRoleImplementation TechnicalRole = "implementation"
	TechnicalRoleCombined       TechnicalRole = "combined"
	TechnicalRoleExplanation    TechnicalRole = "explanation"
	TechnicalRoleObservation    TechnicalRole = "observation"
	TechnicalRoleUnknown        TechnicalRole = "unknown"
)

func (role TechnicalRole) Validate() error {
	switch role {
	case TechnicalRoleReference, TechnicalRoleImplementation, TechnicalRoleCombined,
		TechnicalRoleExplanation, TechnicalRoleObservation, TechnicalRoleUnknown:
		return nil
	default:
		return fmt.Errorf("invalid source diversity technical role %q", role)
	}
}

type DeferredDimension string

const (
	DeferredGeography DeferredDimension = "geography"
	DeferredLanguage  DeferredDimension = "language"
)

type State string

const (
	StateNormativeSourceSufficient State = "normative_source_sufficient"
	StateSufficient                State = "sufficient"
	StateConcentrated              State = "concentrated"
	StateUnknown                   State = "unknown"
)

func (state State) Validate() error {
	switch state {
	case StateNormativeSourceSufficient, StateSufficient, StateConcentrated, StateUnknown:
		return nil
	default:
		return fmt.Errorf("invalid source diversity state %q", state)
	}
}

type WarningCode string

const (
	WarningNoAcceptedSources        WarningCode = "support.no_accepted_sources"
	WarningSingleIndependentSource  WarningCode = "independence.single_source"
	WarningOrganizationConcentrated WarningCode = "independence.organization_concentrated"
	WarningSharedDependency         WarningCode = "independence.shared_dependency"
	WarningIndependenceUnknown      WarningCode = "independence.metadata_unknown"
	WarningSourceKindConcentrated   WarningCode = "dimension.source_kind_concentrated"
	WarningPerspectiveConcentrated  WarningCode = "dimension.perspective_concentrated"
	WarningPerspectiveUnknown       WarningCode = "dimension.perspective_unknown"
	WarningTechnicalRoleUnknown     WarningCode = "dimension.technical_role_unknown"
	WarningReferenceAbsent          WarningCode = "dimension.reference_absent"
	WarningImplementationAbsent     WarningCode = "dimension.implementation_absent"
)

func (code WarningCode) Validate() error {
	switch code {
	case WarningNoAcceptedSources, WarningSingleIndependentSource,
		WarningOrganizationConcentrated, WarningSharedDependency,
		WarningIndependenceUnknown, WarningSourceKindConcentrated,
		WarningPerspectiveConcentrated, WarningPerspectiveUnknown,
		WarningTechnicalRoleUnknown, WarningReferenceAbsent, WarningImplementationAbsent:
		return nil
	default:
		return fmt.Errorf("invalid source diversity warning %q", code)
	}
}

type Warning struct {
	Code      WarningCode
	Detail    string
	SourceIDs []research.SourceID
}

func (warning Warning) Validate() error {
	if err := warning.Code.Validate(); err != nil {
		return err
	}
	if err := validateAnnotation("source diversity warning detail", warning.Detail, false); err != nil {
		return err
	}
	previous := ""
	for _, sourceID := range warning.SourceIDs {
		if err := sourceID.Validate(); err != nil {
			return err
		}
		if sourceID.String() <= previous {
			return fmt.Errorf("source diversity warning IDs must be unique and ordered")
		}
		previous = sourceID.String()
	}
	return nil
}

// SourceReview contains reviewed classification only. Organization and
// DependencyGroup are never inferred from page text, domains, or popularity.
type SourceReview struct {
	Source          research.Source
	TrustDecision   *research.TrustDecision
	Organization    string
	DependencyGroup string
	Perspective     Perspective
	TechnicalRole   TechnicalRole
}

func (review SourceReview) Validate() error {
	if err := review.Source.Validate(); err != nil {
		return fmt.Errorf("source diversity source: %w", err)
	}
	if review.TrustDecision != nil {
		if err := review.TrustDecision.Validate(); err != nil {
			return fmt.Errorf("source diversity trust decision: %w", err)
		}
		if review.TrustDecision.SourceID != review.Source.ID {
			return fmt.Errorf("source diversity trust decision does not match source")
		}
	}
	if err := validateAnnotation("source diversity organization", review.Organization, true); err != nil {
		return err
	}
	if err := validateAnnotation("source diversity dependency group", review.DependencyGroup, true); err != nil {
		return err
	}
	if err := review.Perspective.Validate(); err != nil {
		return err
	}
	return review.TechnicalRole.Validate()
}

type Input struct {
	Claim   research.Claim
	Sources []SourceReview
}

func (input Input) Validate() error {
	if err := input.Claim.Validate(); err != nil {
		return fmt.Errorf("source diversity claim: %w", err)
	}
	if len(input.Sources) == 0 || len(input.Sources) > MaximumSourceCount {
		return fmt.Errorf("source diversity sources must contain between 1 and %d entries", MaximumSourceCount)
	}
	if len(input.Sources) != len(input.Claim.SourceIDs) {
		return fmt.Errorf("source diversity must review every claim source")
	}
	claimSources := make(map[research.SourceID]struct{}, len(input.Claim.SourceIDs))
	for _, sourceID := range input.Claim.SourceIDs {
		claimSources[sourceID] = struct{}{}
	}
	seen := make(map[research.SourceID]struct{}, len(input.Sources))
	for index, review := range input.Sources {
		if err := review.Validate(); err != nil {
			return fmt.Errorf("source diversity review %d: %w", index, err)
		}
		if _, exists := claimSources[review.Source.ID]; !exists {
			return fmt.Errorf("source diversity source %q is not declared by claim", review.Source.ID)
		}
		if _, exists := seen[review.Source.ID]; exists {
			return fmt.Errorf("source diversity repeats source %q", review.Source.ID)
		}
		seen[review.Source.ID] = struct{}{}
	}
	return nil
}

type Assessment struct {
	State                  State
	SourceCount            int
	EligibleSourceCount    int
	IndependentSourceCount int
	OrganizationCount      int
	SourceKindCount        int
	PerspectiveCount       int
	ReferencePresent       bool
	ImplementationPresent  bool
	Warnings               []Warning
	DeferredDimensions     []DeferredDimension
	AlgorithmVersion       string
}

func (assessment Assessment) Validate() error {
	if err := assessment.State.Validate(); err != nil {
		return err
	}
	if assessment.SourceCount < 1 || assessment.SourceCount > MaximumSourceCount ||
		assessment.EligibleSourceCount < 0 || assessment.EligibleSourceCount > assessment.SourceCount ||
		assessment.IndependentSourceCount < 0 || assessment.IndependentSourceCount > assessment.EligibleSourceCount ||
		assessment.OrganizationCount < 0 || assessment.OrganizationCount > assessment.EligibleSourceCount ||
		assessment.SourceKindCount < 0 || assessment.SourceKindCount > assessment.EligibleSourceCount ||
		assessment.PerspectiveCount < 0 || assessment.PerspectiveCount > assessment.EligibleSourceCount {
		return fmt.Errorf("source diversity counts are inconsistent")
	}
	switch assessment.State {
	case StateNormativeSourceSufficient:
		if assessment.EligibleSourceCount != 1 || len(assessment.Warnings) != 0 {
			return fmt.Errorf("normative single-source sufficiency must contain one eligible source and no diversity warnings")
		}
	case StateSufficient:
		if assessment.IndependentSourceCount < 2 {
			return fmt.Errorf("sufficient diversity requires at least two independent sources")
		}
	case StateConcentrated:
		if assessment.EligibleSourceCount == 0 || assessment.IndependentSourceCount >= 2 {
			return fmt.Errorf("concentrated diversity requires support without two independent sources")
		}
	case StateUnknown:
		if assessment.EligibleSourceCount != 0 || assessment.IndependentSourceCount != 0 {
			return fmt.Errorf("unknown diversity cannot contain eligible sources")
		}
	}
	if assessment.AlgorithmVersion != AlgorithmVersionV1 {
		return fmt.Errorf("source diversity algorithm version must be %q", AlgorithmVersionV1)
	}
	if len(assessment.DeferredDimensions) != 2 || assessment.DeferredDimensions[0] != DeferredGeography || assessment.DeferredDimensions[1] != DeferredLanguage {
		return fmt.Errorf("source diversity must defer geography and language in v1")
	}
	seenWarnings := make(map[WarningCode]struct{}, len(assessment.Warnings))
	for _, warning := range assessment.Warnings {
		if err := warning.Validate(); err != nil {
			return err
		}
		if _, exists := seenWarnings[warning.Code]; exists {
			return fmt.Errorf("source diversity contains duplicate warning %q", warning.Code)
		}
		seenWarnings[warning.Code] = struct{}{}
	}
	return nil
}

type PolicyV1 struct{}

func (PolicyV1) Assess(input Input) (Assessment, error) {
	if err := input.Validate(); err != nil {
		return Assessment{}, fmt.Errorf("assess %s: %w", AlgorithmVersionV1, err)
	}
	reviews := append([]SourceReview(nil), input.Sources...)
	sort.Slice(reviews, func(i, j int) bool { return reviews[i].Source.ID.String() < reviews[j].Source.ID.String() })
	eligible := make([]SourceReview, 0, len(reviews))
	for _, review := range reviews {
		if accepted(review.TrustDecision) {
			eligible = append(eligible, review)
		}
	}
	assessment := summarize(input.Claim, reviews, eligible)
	if err := assessment.Validate(); err != nil {
		return Assessment{}, fmt.Errorf("assess %s output: %w", AlgorithmVersionV1, err)
	}
	assessment.Warnings = cloneWarnings(assessment.Warnings)
	assessment.DeferredDimensions = append([]DeferredDimension(nil), assessment.DeferredDimensions...)
	return assessment, nil
}

func summarize(claim research.Claim, reviews, eligible []SourceReview) Assessment {
	organizations := make(map[string]struct{})
	kinds := make(map[research.SourceKind]struct{})
	perspectives := make(map[Perspective]struct{})
	referencePresent, implementationPresent := false, false
	for _, review := range eligible {
		if organization := normalize(review.Organization); organization != "" {
			organizations[organization] = struct{}{}
		}
		kinds[review.Source.Kind] = struct{}{}
		if review.Perspective != PerspectiveUnknown {
			perspectives[review.Perspective] = struct{}{}
		}
		switch review.TechnicalRole {
		case TechnicalRoleReference:
			referencePresent = true
		case TechnicalRoleImplementation:
			implementationPresent = true
		case TechnicalRoleCombined:
			referencePresent, implementationPresent = true, true
		}
	}
	independentCount := countIndependent(eligible)
	assessment := Assessment{
		SourceCount: len(reviews), EligibleSourceCount: len(eligible),
		IndependentSourceCount: independentCount, OrganizationCount: len(organizations),
		SourceKindCount: len(kinds), PerspectiveCount: len(perspectives),
		ReferencePresent: referencePresent, ImplementationPresent: implementationPresent,
		DeferredDimensions: []DeferredDimension{DeferredGeography, DeferredLanguage},
		AlgorithmVersion:   AlgorithmVersionV1,
	}
	if normativeSingleSourceSufficient(claim, eligible) {
		assessment.State = StateNormativeSourceSufficient
		return assessment
	}
	switch {
	case len(eligible) == 0:
		assessment.State = StateUnknown
	case independentCount >= 2:
		assessment.State = StateSufficient
	default:
		assessment.State = StateConcentrated
	}
	assessment.Warnings = warningsFor(claim, eligible, independentCount, len(kinds), len(perspectives), referencePresent, implementationPresent)
	return assessment
}

func countIndependent(reviews []SourceReview) int {
	if len(reviews) == 1 {
		return 1
	}
	parents := make([]int, len(reviews))
	for index := range parents {
		parents[index] = index
	}
	find := func(index int) int { return index }
	var findRoot func(int) int
	findRoot = func(index int) int {
		if parents[index] != index {
			parents[index] = findRoot(parents[index])
		}
		return parents[index]
	}
	find = findRoot
	union := func(left, right int) {
		leftRoot, rightRoot := find(left), find(right)
		if leftRoot != rightRoot {
			parents[rightRoot] = leftRoot
		}
	}
	for left := range reviews {
		for right := left + 1; right < len(reviews); right++ {
			sameOrganization := normalize(reviews[left].Organization) != "" && normalize(reviews[left].Organization) == normalize(reviews[right].Organization)
			sameDependency := normalize(reviews[left].DependencyGroup) != "" && normalize(reviews[left].DependencyGroup) == normalize(reviews[right].DependencyGroup)
			if sameOrganization || sameDependency {
				union(left, right)
			}
		}
	}
	knownRoots := make(map[int]struct{})
	for index, review := range reviews {
		if normalize(review.Organization) != "" || normalize(review.DependencyGroup) != "" {
			knownRoots[find(index)] = struct{}{}
		}
	}
	return len(knownRoots)
}

func warningsFor(claim research.Claim, reviews []SourceReview, independentCount, kindCount, perspectiveCount int, referencePresent, implementationPresent bool) []Warning {
	warnings := make([]Warning, 0, 8)
	if len(reviews) == 0 {
		return []Warning{warning(WarningNoAcceptedSources, "No source has an accepted Trust Decision, so diversity is unknown.", nil)}
	}
	allIDs := sourceIDs(reviews)
	if independentCount < 2 {
		warnings = append(warnings, warning(WarningSingleIndependentSource, "Fewer than two demonstrably independent source origins support the claim.", allIDs))
	}
	if ids := repeatedGroupIDs(reviews, func(review SourceReview) string { return normalize(review.Organization) }); len(ids) > 0 {
		warnings = append(warnings, warning(WarningOrganizationConcentrated, "Multiple supporting sources belong to the same reviewed organization.", ids))
	}
	if ids := repeatedGroupIDs(reviews, func(review SourceReview) string { return normalize(review.DependencyGroup) }); len(ids) > 0 {
		warnings = append(warnings, warning(WarningSharedDependency, "Multiple supporting sources share the same reviewed upstream dependency.", ids))
	}
	unknownIndependence := make([]research.SourceID, 0)
	unknownPerspective := make([]research.SourceID, 0)
	unknownRole := make([]research.SourceID, 0)
	for _, review := range reviews {
		if normalize(review.Organization) == "" && normalize(review.DependencyGroup) == "" {
			unknownIndependence = append(unknownIndependence, review.Source.ID)
		}
		if review.Perspective == PerspectiveUnknown {
			unknownPerspective = append(unknownPerspective, review.Source.ID)
		}
		if review.TechnicalRole == TechnicalRoleUnknown {
			unknownRole = append(unknownRole, review.Source.ID)
		}
	}
	if len(unknownIndependence) > 0 {
		warnings = append(warnings, warning(WarningIndependenceUnknown, "Organization and upstream dependency are unknown for some sources; independence is not assumed.", unknownIndependence))
	}
	if len(reviews) > 1 && kindCount == 1 {
		warnings = append(warnings, warning(WarningSourceKindConcentrated, "All accepted supporting sources use the same source kind.", allIDs))
	}
	if len(reviews) > 1 && perspectiveCount == 1 {
		warnings = append(warnings, warning(WarningPerspectiveConcentrated, "All known supporting perspectives are the same.", allIDs))
	}
	if len(unknownPerspective) > 0 {
		warnings = append(warnings, warning(WarningPerspectiveUnknown, "Some supporting source perspectives are unknown.", unknownPerspective))
	}
	if len(unknownRole) > 0 {
		warnings = append(warnings, warning(WarningTechnicalRoleUnknown, "Some implementation/reference roles are unknown.", unknownRole))
	}
	if needsTechnicalBalance(claim.Type) {
		if !referencePresent {
			warnings = append(warnings, warning(WarningReferenceAbsent, "No accepted source is reviewed as reference material.", allIDs))
		}
		if !implementationPresent {
			warnings = append(warnings, warning(WarningImplementationAbsent, "No accepted source is reviewed as implementation material.", allIDs))
		}
	}
	return warnings
}

func normativeSingleSourceSufficient(claim research.Claim, reviews []SourceReview) bool {
	if len(reviews) != 1 || (claim.Type != research.ClaimDefinition && claim.Type != research.ClaimRequirement) {
		return false
	}
	review := reviews[0]
	return (review.Source.Kind == research.SourceSpecification || review.Source.Kind == research.SourceStandard) &&
		review.TrustDecision != nil && review.TrustDecision.State == research.TrustAccepted &&
		review.TrustDecision.Tier == research.AuthorityTierA
}

func accepted(decision *research.TrustDecision) bool {
	return decision != nil && (decision.State == research.TrustAccepted || decision.State == research.TrustAcceptedSupplement)
}

func needsTechnicalBalance(claimType research.ClaimType) bool {
	switch claimType {
	case research.ClaimRecommendation, research.ClaimSecurity, research.ClaimExample:
		return true
	default:
		return false
	}
}

func repeatedGroupIDs(reviews []SourceReview, key func(SourceReview) string) []research.SourceID {
	groups := make(map[string][]research.SourceID)
	for _, review := range reviews {
		value := key(review)
		if value != "" {
			groups[value] = append(groups[value], review.Source.ID)
		}
	}
	result := make([]research.SourceID, 0)
	for _, ids := range groups {
		if len(ids) > 1 {
			result = append(result, ids...)
		}
	}
	sort.Slice(result, func(i, j int) bool { return result[i].String() < result[j].String() })
	return result
}

func sourceIDs(reviews []SourceReview) []research.SourceID {
	result := make([]research.SourceID, len(reviews))
	for index, review := range reviews {
		result[index] = review.Source.ID
	}
	return result
}

func warning(code WarningCode, detail string, sourceIDs []research.SourceID) Warning {
	return Warning{Code: code, Detail: detail, SourceIDs: append([]research.SourceID(nil), sourceIDs...)}
}

func cloneWarnings(warnings []Warning) []Warning {
	result := make([]Warning, len(warnings))
	for index, warning := range warnings {
		result[index] = warning
		result[index].SourceIDs = append([]research.SourceID(nil), warning.SourceIDs...)
	}
	return result
}

func normalize(value string) string { return strings.ToLower(strings.Join(strings.Fields(value), " ")) }

func validateAnnotation(name, value string, optional bool) error {
	if optional && value == "" {
		return nil
	}
	if strings.TrimSpace(value) == "" || value != strings.TrimSpace(value) || len(value) > maximumAnnotationSize || strings.IndexFunc(value, unicode.IsControl) >= 0 {
		return fmt.Errorf("%s is invalid", name)
	}
	return nil
}
