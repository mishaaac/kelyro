package queryplanner

import (
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/mishaaac/kelyro/internal/research"
)

func TestPlannerV1PlansDefinitionQueriesUsingAuthorityPreferences(t *testing.T) {
	t.Parallel()
	input := plannerInput(t, "Interfaces", "software", "Go", research.PurposeConceptDefinition, nil,
		[]research.SourceKind{research.SourceSpecification, research.SourceOfficialDocumentation}, research.AuthorityTierB)

	plan, err := (PlannerV1{}).Plan(input)
	if err != nil {
		t.Fatal(err)
	}
	assertPlan(t, plan, []ResearchQuery{
		{Query: "Go Interfaces specification", DesiredSourceKind: research.SourceSpecification, RequiredAuthority: research.AuthorityTierB, Priority: 1},
		{Query: "Go Interfaces official documentation", DesiredSourceKind: research.SourceOfficialDocumentation, RequiredAuthority: research.AuthorityTierB, Priority: 2},
		{Query: "Go Interfaces tutorial", DesiredSourceKind: research.SourceOfficialTutorial, RequiredAuthority: research.AuthorityTierB, Priority: 3},
	})
}

func TestPlannerV1PlansVersionBoundReleaseQueries(t *testing.T) {
	t.Parallel()
	version := sourceVersion(t, "v1.24.3")
	input := plannerInput(t, "runtime", "software", "Go", research.PurposeReleaseStatus, &version,
		[]research.SourceKind{research.SourceReleaseNotes}, research.AuthorityTierA)

	plan, err := (PlannerV1{}).Plan(input)
	if err != nil {
		t.Fatal(err)
	}
	assertPlan(t, plan, []ResearchQuery{
		{Query: "Go runtime v1.24.3 release notes", DesiredSourceKind: research.SourceReleaseNotes, RequiredAuthority: research.AuthorityTierA, Priority: 1},
		{Query: "Go runtime v1.24.3 official release status", DesiredSourceKind: research.SourceOfficialDocumentation, RequiredAuthority: research.AuthorityTierA, Priority: 2},
		{Query: "Go runtime v1.24.3 release announcement", DesiredSourceKind: research.SourceOfficialBlog, RequiredAuthority: research.AuthorityTierA, Priority: 3},
	})
}

func TestPlannerV1PlansSecurityQueries(t *testing.T) {
	t.Parallel()
	input := plannerInput(t, "TLS configuration", "software", "Caddy", research.PurposeSecurityGuidance, nil,
		[]research.SourceKind{research.SourceStandard, research.SourceOfficialDocumentation}, research.AuthorityTierA)

	plan, err := (PlannerV1{}).Plan(input)
	if err != nil {
		t.Fatal(err)
	}
	if len(plan.Queries) != 4 || plan.Queries[0].Query != "Caddy TLS configuration security standard" ||
		plan.Queries[0].DesiredSourceKind != research.SourceStandard ||
		plan.Queries[1].Query != "Caddy TLS configuration official security guidance" {
		t.Fatalf("security plan = %+v", plan)
	}
	for _, query := range plan.Queries {
		if !strings.Contains(strings.ToLower(query.Query), "security") {
			t.Fatalf("security query lacks explicit intent: %+v", query)
		}
	}
}

func TestPlannerV1KeepsGenericTopicsTechnologyAgnostic(t *testing.T) {
	t.Parallel()
	input := plannerInput(t, "Bayesian inference", "statistics", "", research.PurposeConceptDefinition, nil,
		[]research.SourceKind{research.SourcePaper, research.SourceBookReference}, research.AuthorityTierC)

	plan, err := (PlannerV1{}).Plan(input)
	if err != nil {
		t.Fatal(err)
	}
	if len(plan.Queries) != 5 ||
		plan.Queries[3].Query != "statistics Bayesian inference definition paper" ||
		plan.Queries[3].DesiredSourceKind != research.SourcePaper ||
		plan.Queries[4].Query != "statistics Bayesian inference definition book reference" {
		t.Fatalf("generic plan = %+v", plan)
	}
	for _, query := range plan.Queries {
		if strings.Contains(query.Query, "  ") || strings.Contains(strings.ToLower(query.Query), "api reference") ||
			strings.Contains(strings.ToLower(query.Query), "source code") {
			t.Fatalf("generic definition query contains a technology assumption: %+v", query)
		}
	}
}

func TestPlannerV1CoversEveryResearchPurpose(t *testing.T) {
	t.Parallel()
	tests := []struct {
		purpose  research.ResearchPurpose
		wantKind research.SourceKind
	}{
		{purpose: research.PurposeConceptDefinition, wantKind: research.SourceSpecification},
		{purpose: research.PurposeCurrentUsage, wantKind: research.SourcePackageReference},
		{purpose: research.PurposeVersionBehavior, wantKind: research.SourceReleaseNotes},
		{purpose: research.PurposeReleaseStatus, wantKind: research.SourceReleaseNotes},
		{purpose: research.PurposeDeprecationCheck, wantKind: research.SourceReleaseNotes},
		{purpose: research.PurposePrerequisiteResearch, wantKind: research.SourceOfficialTutorial},
		{purpose: research.PurposeProductionPractice, wantKind: research.SourceCode},
		{purpose: research.PurposeSecurityGuidance, wantKind: research.SourceStandard},
	}
	for _, test := range tests {
		test := test
		t.Run(string(test.purpose), func(t *testing.T) {
			input := plannerInput(t, "topic", "domain", "technology", test.purpose, nil,
				[]research.SourceKind{research.SourceOfficialDocumentation}, research.AuthorityTierB)
			plan, err := (PlannerV1{}).Plan(input)
			if err != nil {
				t.Fatal(err)
			}
			found := false
			for _, query := range plan.Queries {
				found = found || query.DesiredSourceKind == test.wantKind
			}
			if !found {
				t.Fatalf("plan for %q lacks source kind %q: %+v", test.purpose, test.wantKind, plan)
			}
		})
	}
}

func TestPlannerV1IsDeterministicBoundedAndDoesNotMutateInput(t *testing.T) {
	t.Parallel()
	kinds := []research.SourceKind{
		research.SourceVideo, research.SourcePaper, research.SourceBookReference, research.SourceStandard,
		research.SourceOfficialDocumentation, research.SourceSpecification, research.SourceReleaseNotes,
		research.SourceOfficialBlog, research.SourcePackageReference, research.SourceOfficialTutorial,
	}
	input := plannerInput(t, "incident\tresponse", "security", "", research.PurposeSecurityGuidance, nil,
		kinds, research.AuthorityTierB)
	originalKinds := append([]research.SourceKind(nil), input.AuthorityProfile.PreferredKinds...)

	first, err := (PlannerV1{}).Plan(input)
	if err != nil {
		t.Fatal(err)
	}
	second, err := (PlannerV1{}).Plan(input)
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(first, second) {
		t.Fatalf("plans differ:\nfirst: %+v\nsecond: %+v", first, second)
	}
	if len(first.Queries) != MaximumQueries {
		t.Fatalf("query count = %d, want %d", len(first.Queries), MaximumQueries)
	}
	if !reflect.DeepEqual(input.AuthorityProfile.PreferredKinds, originalKinds) {
		t.Fatalf("planner mutated preferred kinds: %+v", input.AuthorityProfile.PreferredKinds)
	}
}

func TestPlannerV1RejectsInvalidInputAndPlan(t *testing.T) {
	t.Parallel()
	input := plannerInput(t, "Definitions", "software", "Go", research.PurposeConceptDefinition, nil,
		[]research.SourceKind{research.SourceOfficialDocumentation}, research.AuthorityTierB)
	input.Purpose = research.ResearchPurpose("invented")
	if _, err := (PlannerV1{}).Plan(input); err == nil || !strings.Contains(err.Error(), "invalid research purpose") {
		t.Fatalf("invalid purpose error = %v", err)
	}

	invalidPlan := ResearchQueryPlan{AlgorithmVersion: AlgorithmVersion, Queries: []ResearchQuery{
		{Query: "valid query", DesiredSourceKind: research.SourceOfficialDocumentation, RequiredAuthority: research.AuthorityTierB, Priority: 2},
	}}
	if err := invalidPlan.Validate(); err == nil || !strings.Contains(err.Error(), "non-sequential priority") {
		t.Fatalf("invalid plan error = %v", err)
	}
}

func assertPlan(t *testing.T, got ResearchQueryPlan, want []ResearchQuery) {
	t.Helper()
	if got.AlgorithmVersion != AlgorithmVersion || !reflect.DeepEqual(got.Queries, want) {
		t.Fatalf("plan = %+v, want version %q queries %+v", got, AlgorithmVersion, want)
	}
	if err := got.Validate(); err != nil {
		t.Fatalf("plan validation failed: %v", err)
	}
}

func plannerInput(
	t *testing.T,
	subject, domain, technology string,
	purpose research.ResearchPurpose,
	version *research.SourceVersion,
	preferred []research.SourceKind,
	tier research.AuthorityTier,
) Input {
	t.Helper()
	topic, err := research.NewResearchTopic(subject, domain, technology)
	if err != nil {
		t.Fatal(err)
	}
	id, err := research.NewID("authority.test")
	if err != nil {
		t.Fatal(err)
	}
	createdAt, err := research.NewTimestamp(time.Date(2026, time.August, 25, 0, 0, 0, 0, time.UTC))
	if err != nil {
		t.Fatal(err)
	}
	return Input{
		Topic:         topic,
		TargetVersion: version,
		Purpose:       purpose,
		AuthorityProfile: research.AuthorityProfile{
			ID: id, Version: "authority-profiles/v1", Domain: "*", TopicPattern: "*",
			PreferredKinds: preferred, MinimumCorroboration: 1, MinimumTier: tier, CreatedAt: createdAt,
		},
	}
}

func sourceVersion(t *testing.T, value string) research.SourceVersion {
	t.Helper()
	version, err := research.NewSourceVersion(value)
	if err != nil {
		t.Fatal(err)
	}
	return version
}
