package queryplanner

import (
	"fmt"
	"strings"

	"github.com/mishaaac/kelyro/internal/research"
)

const (
	AlgorithmVersion = "query-planner-v1"
	MaximumQueries   = 8
)

// Input is the complete, provider-neutral intent consumed by PlannerV1.
// AuthorityProfile must already have been selected for Topic by the authority
// catalog; matching profiles is deliberately outside this algorithm.
type Input struct {
	Topic            research.ResearchTopic
	TargetVersion    *research.SourceVersion
	Purpose          research.ResearchPurpose
	AuthorityProfile research.AuthorityProfile
}

func (input Input) Validate() error {
	if err := input.Topic.Validate(); err != nil {
		return fmt.Errorf("query planner topic: %w", err)
	}
	if input.TargetVersion != nil {
		if err := input.TargetVersion.Validate(); err != nil {
			return fmt.Errorf("query planner target version: %w", err)
		}
	}
	if err := input.Purpose.Validate(); err != nil {
		return fmt.Errorf("query planner purpose: %w", err)
	}
	if err := input.AuthorityProfile.Validate(); err != nil {
		return fmt.Errorf("query planner authority profile: %w", err)
	}
	return nil
}

// ResearchQuery is one ordered discovery intention. RequiredAuthority is a
// threshold for later classification, not a trust decision about any result.
type ResearchQuery struct {
	Query             string
	DesiredSourceKind research.SourceKind
	RequiredAuthority research.AuthorityTier
	Priority          int
}

func (query ResearchQuery) Validate() error {
	if strings.TrimSpace(query.Query) == "" {
		return fmt.Errorf("planned research query is empty")
	}
	if query.Query != strings.Join(strings.Fields(query.Query), " ") {
		return fmt.Errorf("planned research query is not normalized")
	}
	if err := query.DesiredSourceKind.Validate(); err != nil {
		return err
	}
	if err := query.RequiredAuthority.Validate(); err != nil {
		return err
	}
	if query.Priority <= 0 {
		return fmt.Errorf("planned research query priority must be positive")
	}
	return nil
}

type ResearchQueryPlan struct {
	AlgorithmVersion string
	Queries          []ResearchQuery
}

func (plan ResearchQueryPlan) Validate() error {
	if plan.AlgorithmVersion != AlgorithmVersion {
		return fmt.Errorf("invalid query planner version %q", plan.AlgorithmVersion)
	}
	if len(plan.Queries) == 0 {
		return fmt.Errorf("research query plan is empty")
	}
	if len(plan.Queries) > MaximumQueries {
		return fmt.Errorf("research query plan exceeds %d queries", MaximumQueries)
	}
	seenKinds := make(map[research.SourceKind]struct{}, len(plan.Queries))
	seenQueries := make(map[string]struct{}, len(plan.Queries))
	for index, query := range plan.Queries {
		if err := query.Validate(); err != nil {
			return fmt.Errorf("research query plan item %d: %w", index, err)
		}
		if query.Priority != index+1 {
			return fmt.Errorf("research query plan item %d has non-sequential priority %d", index, query.Priority)
		}
		if _, exists := seenKinds[query.DesiredSourceKind]; exists {
			return fmt.Errorf("research query plan repeats source kind %q", query.DesiredSourceKind)
		}
		seenKinds[query.DesiredSourceKind] = struct{}{}
		key := strings.ToLower(query.Query)
		if _, exists := seenQueries[key]; exists {
			return fmt.Errorf("research query plan repeats query %q", query.Query)
		}
		seenQueries[key] = struct{}{}
	}
	return nil
}

// PlannerV1 is stateless. Its zero value is ready for deterministic use.
type PlannerV1 struct{}

func (PlannerV1) Plan(input Input) (ResearchQueryPlan, error) {
	if err := input.Validate(); err != nil {
		return ResearchQueryPlan{}, err
	}

	templates := orderedTemplates(input.Purpose, input.AuthorityProfile.PreferredKinds)
	base := queryBase(input.Topic, input.TargetVersion)
	queries := make([]ResearchQuery, 0, min(len(templates), MaximumQueries))
	for _, template := range templates {
		if len(queries) == MaximumQueries {
			break
		}
		queries = append(queries, ResearchQuery{
			Query:             strings.Join([]string{base, template.qualifier}, " "),
			DesiredSourceKind: template.kind,
			RequiredAuthority: input.AuthorityProfile.MinimumTier,
			Priority:          len(queries) + 1,
		})
	}

	plan := ResearchQueryPlan{AlgorithmVersion: AlgorithmVersion, Queries: queries}
	if err := plan.Validate(); err != nil {
		return ResearchQueryPlan{}, fmt.Errorf("query planner v1 output: %w", err)
	}
	return plan, nil
}

type queryTemplate struct {
	kind      research.SourceKind
	qualifier string
}

func orderedTemplates(purpose research.ResearchPurpose, preferred []research.SourceKind) []queryTemplate {
	defaults := purposeTemplates(purpose)
	byKind := make(map[research.SourceKind]queryTemplate, len(defaults))
	for _, template := range defaults {
		byKind[template.kind] = template
	}

	ordered := make([]queryTemplate, 0, len(preferred)+len(defaults))
	seen := make(map[research.SourceKind]struct{}, len(preferred)+len(defaults))
	for _, kind := range preferred {
		template, relevant := byKind[kind]
		if !relevant {
			continue
		}
		ordered = append(ordered, template)
		seen[kind] = struct{}{}
	}
	for _, template := range defaults {
		if _, exists := seen[template.kind]; exists {
			continue
		}
		ordered = append(ordered, template)
		seen[template.kind] = struct{}{}
	}
	for _, kind := range preferred {
		if _, exists := seen[kind]; exists {
			continue
		}
		ordered = append(ordered, queryTemplate{
			kind: kind, qualifier: purposeQualifier(purpose) + " " + sourceKindQualifier(kind),
		})
		seen[kind] = struct{}{}
	}
	return ordered
}

func purposeTemplates(purpose research.ResearchPurpose) []queryTemplate {
	switch purpose {
	case research.PurposeConceptDefinition:
		return []queryTemplate{
			{kind: research.SourceOfficialDocumentation, qualifier: "official documentation"},
			{kind: research.SourceSpecification, qualifier: "specification"},
			{kind: research.SourceOfficialTutorial, qualifier: "tutorial"},
		}
	case research.PurposeCurrentUsage:
		return []queryTemplate{
			{kind: research.SourceOfficialDocumentation, qualifier: "official documentation current usage"},
			{kind: research.SourcePackageReference, qualifier: "API reference"},
			{kind: research.SourceOfficialTutorial, qualifier: "tutorial"},
			{kind: research.SourceCode, qualifier: "source code"},
		}
	case research.PurposeVersionBehavior:
		return []queryTemplate{
			{kind: research.SourceOfficialDocumentation, qualifier: "official documentation version behavior"},
			{kind: research.SourcePackageReference, qualifier: "API reference"},
			{kind: research.SourceReleaseNotes, qualifier: "release notes"},
			{kind: research.SourceCode, qualifier: "source code"},
		}
	case research.PurposeReleaseStatus:
		return []queryTemplate{
			{kind: research.SourceReleaseNotes, qualifier: "release notes"},
			{kind: research.SourceOfficialDocumentation, qualifier: "official release status"},
			{kind: research.SourceOfficialBlog, qualifier: "release announcement"},
		}
	case research.PurposeDeprecationCheck:
		return []queryTemplate{
			{kind: research.SourceOfficialDocumentation, qualifier: "official documentation deprecation"},
			{kind: research.SourceReleaseNotes, qualifier: "release notes deprecation"},
			{kind: research.SourceSpecification, qualifier: "specification deprecation"},
			{kind: research.SourceCode, qualifier: "source code deprecation"},
		}
	case research.PurposePrerequisiteResearch:
		return []queryTemplate{
			{kind: research.SourceOfficialTutorial, qualifier: "tutorial prerequisites"},
			{kind: research.SourceOfficialDocumentation, qualifier: "official documentation prerequisites"},
			{kind: research.SourceSpecification, qualifier: "specification prerequisites"},
		}
	case research.PurposeProductionPractice:
		return []queryTemplate{
			{kind: research.SourceOfficialDocumentation, qualifier: "official documentation production practices"},
			{kind: research.SourcePackageReference, qualifier: "API reference production practices"},
			{kind: research.SourceCode, qualifier: "source code production examples"},
			{kind: research.SourceOfficialTutorial, qualifier: "tutorial production practices"},
		}
	case research.PurposeSecurityGuidance:
		return []queryTemplate{
			{kind: research.SourceOfficialDocumentation, qualifier: "official security guidance"},
			{kind: research.SourceStandard, qualifier: "security standard"},
			{kind: research.SourceReleaseNotes, qualifier: "security advisory release notes"},
			{kind: research.SourceCode, qualifier: "security source code"},
		}
	default:
		return nil
	}
}

func purposeQualifier(purpose research.ResearchPurpose) string {
	switch purpose {
	case research.PurposeConceptDefinition:
		return "definition"
	case research.PurposeCurrentUsage:
		return "current usage"
	case research.PurposeVersionBehavior:
		return "version behavior"
	case research.PurposeReleaseStatus:
		return "release status"
	case research.PurposeDeprecationCheck:
		return "deprecation"
	case research.PurposePrerequisiteResearch:
		return "prerequisites"
	case research.PurposeProductionPractice:
		return "production practices"
	case research.PurposeSecurityGuidance:
		return "security guidance"
	default:
		return "research"
	}
}

func sourceKindQualifier(kind research.SourceKind) string {
	switch kind {
	case research.SourceOfficialDocumentation:
		return "official documentation"
	case research.SourcePackageReference:
		return "API reference"
	case research.SourceOfficialTutorial:
		return "tutorial"
	default:
		return strings.ReplaceAll(string(kind), "_", " ")
	}
}

func queryBase(topic research.ResearchTopic, version *research.SourceVersion) string {
	parts := make([]string, 0, 4)
	if topic.Technology != "" {
		parts = append(parts, topic.Technology)
	} else if topic.Domain != "" {
		parts = append(parts, topic.Domain)
	}
	parts = append(parts, topic.Subject)
	if version != nil {
		parts = append(parts, version.String())
	}
	return strings.Join(strings.Fields(strings.Join(parts, " ")), " ")
}
