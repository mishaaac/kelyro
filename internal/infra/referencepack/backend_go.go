// Package referencepack contains deterministic, development-only Learning Pack
// sources used to exercise the complete I-04 compiler and pack builder.
package referencepack

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"time"

	"github.com/mishaaac/kelyro/internal/curriculum"
	curriculumapp "github.com/mishaaac/kelyro/internal/curriculum/application"
	"github.com/mishaaac/kelyro/internal/infra/learningpack"
)

const BackendGoArchiveName = "backend-go-reference.zip"

type conceptSpec struct {
	key, scope, title, definition, competency, context string
	kind                                               curriculum.EvidenceClaimKind
	status                                             curriculum.ConceptStatus
	difficulty                                         curriculum.Difficulty
	foundational                                       bool
}

var backendGoConcepts = []conceptSpec{
	{key: "terminal", scope: "terminal process execution", title: "Terminal process execution", definition: "A terminal starts a command as a process with arguments, working directory, standard streams, and an exit status.", competency: "foundations", context: "Local development foundations", kind: curriculum.EvidenceClaimDefinition, status: curriculum.ConceptCurrent, difficulty: curriculum.DifficultyIntroductory, foundational: true},
	{key: "modules", scope: "Go module identity", title: "Go module identity", definition: "A go.mod file declares a module path and the language/toolchain requirements used to resolve packages.", competency: "foundations", context: "Go workspace foundations", kind: curriculum.EvidenceClaimDefinition, status: curriculum.ConceptCurrent, difficulty: curriculum.DifficultyFoundational},
	{key: "gopath", scope: "GOPATH workspace mode", title: "GOPATH workspace mode", definition: "GOPATH workspace mode is historical context for maintaining code that predates module-aware workflows.", competency: "foundations", context: "Historical Go workspace context", kind: curriculum.EvidenceClaimHistorical, status: curriculum.ConceptLegacy, difficulty: curriculum.DifficultyIntermediate},
	{key: "handlers", scope: "HTTP handler boundary", title: "HTTP handler boundary", definition: "An HTTP handler translates a request into validated application input and writes one protocol response.", competency: "build", context: "HTTP service boundaries", kind: curriculum.EvidenceClaimBehavior, status: curriculum.ConceptCurrent, difficulty: curriculum.DifficultyIntermediate},
	{key: "transactions", scope: "database transaction boundary", title: "Database transaction boundary", definition: "A transaction groups database changes into one atomic commit or rollback boundary.", competency: "build", context: "Persistence boundaries", kind: curriculum.EvidenceClaimBehavior, status: curriculum.ConceptCurrent, difficulty: curriculum.DifficultyIntermediate},
	{key: "tests", scope: "deterministic service test", title: "Deterministic service test", definition: "A deterministic service test controls external inputs and asserts observable behavior without depending on the public Internet.", competency: "debug", context: "Verification and debugging", kind: curriculum.EvidenceClaimRecommendation, status: curriculum.ConceptCurrent, difficulty: curriculum.DifficultyAdvanced},
	{key: "threats", scope: "service trust boundary", title: "Service trust boundary", definition: "A trust boundary marks where input or authority changes and therefore requires explicit validation and authorization decisions.", competency: "security", context: "Security boundaries", kind: curriculum.EvidenceClaimSecurity, status: curriculum.ConceptCurrent, difficulty: curriculum.DifficultyAdvanced},
	{key: "observability", scope: "service observability signal", title: "Service observability signal", definition: "Logs, metrics, and traces expose different evidence about a running service and its failure modes.", competency: "operate", context: "Production observability", kind: curriculum.EvidenceClaimRequirement, status: curriculum.ConceptCurrent, difficulty: curriculum.DifficultyAdvanced},
	{key: "deployment", scope: "repeatable service deployment", title: "Repeatable service deployment", definition: "A repeatable deployment promotes an identified build with explicit configuration and observable health checks.", competency: "operate", context: "Deployment and operations", kind: curriculum.EvidenceClaimRecommendation, status: curriculum.ConceptCurrent, difficulty: curriculum.DifficultyAdvanced},
}

// BuildBackendGo compiles and packages a deliberately bounded preview. Its
// evidence is a deterministic development fixture, not a production Source
// Bundle and not a claim that the professional domain is complete.
func BuildBackendGo(ctx context.Context) (curriculumapp.PackBuildResult, error) {
	request, environment, err := backendGoCompileRequest(ctx)
	if err != nil {
		return curriculumapp.PackBuildResult{}, fmt.Errorf("prepare backend Go reference: %w", err)
	}
	compiled, err := curriculumapp.NewCurriculumCompilerV1().Compile(ctx, request)
	if err != nil {
		return curriculumapp.PackBuildResult{}, fmt.Errorf("compile backend Go reference: %w", err)
	}
	if compiled.Diagnostics == nil || compiled.Diagnostics.Review == nil || compiled.Diagnostics.Review.Decision == curriculum.ReviewRejected {
		return curriculumapp.PackBuildResult{}, fmt.Errorf("compile backend Go reference: deterministic curriculum review rejected the development pack")
	}
	manifest := backendGoManifest(compiled)
	result, err := learningpack.NewBuilder().Build(ctx, curriculumapp.PackBuildRequest{
		Manifest: manifest, Compilation: compiled, EvidenceSets: request.EvidenceSets, Environment: &environment,
		Citations: []curriculum.EvidenceReportCitation{{SourceID: mustID("source.backend-go-reference"), Title: "Official Go documentation", URL: "https://go.dev/doc/"}},
		Assets: []curriculumapp.PackBuildAsset{{
			Path: "assets/DEVELOPMENT-NOTICE.md", Content: []byte("# Development reference\n\nThis preview uses deterministic fixture evidence and is not production-complete.\n"),
			License: "CC-BY-4.0", CopyrightHolder: "Kelyro contributors", KelyroAuthored: true,
		}},
	})
	if err != nil {
		return curriculumapp.PackBuildResult{}, fmt.Errorf("build backend Go reference: %w", err)
	}
	return result, nil
}

func backendGoCompileRequest(ctx context.Context) (curriculumapp.CurriculumCompileRequest, curriculum.EnvironmentPack, error) {
	builtAt := mustTimestamp(time.Date(2026, time.September, 15, 12, 0, 0, 0, time.UTC))
	bundleHash := sha256.Sum256([]byte("backend-go-reference-development-evidence-v1"))
	bundle := curriculum.SourceBundleRef{ID: mustID("bundle.backend-go-reference.dev-v1"), ContentHash: "sha256:" + hex.EncodeToString(bundleHash[:]), AlgorithmVersion: "source-bundle-fixture-v1", VerifiedAt: builtAt}
	sourceID := mustID("source.backend-go-reference")
	evidence := curriculum.CurriculumEvidenceSet{
		Bundle: bundle, Eligibility: curriculum.EvidenceReadyWithCaveats,
		SourceAuthority:  []curriculum.EvidenceSourceAuthority{{SourceID: sourceID, Role: "primary", TemporalScope: "current"}},
		Freshness:        curriculum.EvidenceFreshness{State: "fresh", Score: 1, LastVerifiedAt: &builtAt, Algorithm: "fixture-freshness-v1"},
		Caveats:          []string{"Development fixture only; replace with production I-03 Source Bundles before claiming professional completeness."},
		AlgorithmVersion: curriculum.EvidenceIngestionAlgorithmV1,
	}
	references := make(map[string]curriculum.EvidenceRef, len(backendGoConcepts))
	for _, spec := range backendGoConcepts {
		claimID := mustID("claim.backend-go." + spec.key)
		evidence.Claims = append(evidence.Claims, curriculum.CurriculumEvidenceClaim{
			ID: claimID, Statement: spec.definition, Kind: spec.kind, Scope: spec.scope,
			Status: spec.status, Confidence: .8, SourceIDs: []curriculum.ID{sourceID},
		})
		references[spec.key] = curriculum.EvidenceRef{BundleID: bundle.ID, ClaimID: claimID}
	}

	outcomes := []curriculum.GoalOutcome{
		{ID: mustID("outcome.backend-go.explain"), Statement: "Explain the foundations of a module-aware Go service.", Category: curriculum.OutcomeKnowledge, Capability: curriculum.OutcomeCapabilityExplain, EvidenceRefs: refs(references, "terminal", "modules")},
		{ID: mustID("outcome.backend-go.build"), Statement: "Build an HTTP service with an explicit persistence boundary.", Category: curriculum.OutcomeApplication, Capability: curriculum.OutcomeCapabilityBuild, EvidenceRefs: refs(references, "handlers", "transactions")},
		{ID: mustID("outcome.backend-go.debug"), Statement: "Debug service behavior with deterministic tests.", Category: curriculum.OutcomeDebugging, Capability: curriculum.OutcomeCapabilityDebug, EvidenceRefs: refs(references, "tests")},
		{ID: mustID("outcome.backend-go.operate"), Statement: "Operate a service with observable, repeatable deployments.", Category: curriculum.OutcomeProduction, Capability: curriculum.OutcomeCapabilityOperate, EvidenceRefs: refs(references, "observability", "deployment")},
		{ID: mustID("outcome.backend-go.maintain"), Statement: "Maintain service trust boundaries and historical workspace context.", Category: curriculum.OutcomeMaintenance, Capability: curriculum.OutcomeCapabilityMaintain, EvidenceRefs: refs(references, "threats", "gopath")},
	}
	goal := curriculum.LearningGoalSpec{
		ID: mustID("goal.backend-go-reference"), Title: "Backend engineering with Go (development reference)",
		Description: "Exercise the I-04 compiler across foundations, service design, verification, security, operations, and explicitly historical context.", Domain: "backend engineering",
		Role:     &curriculum.ProfessionalRole{ID: mustID("role.backend-go-engineer"), Name: "Backend Go engineer", Description: "Builds, debugs, operates, and maintains Go services."},
		Outcomes: outcomes, Scope: []string{"foundations", "services", "security", "operations"}, Exclusions: []string{"production completeness", "I-05 lesson and exercise runtime"},
	}
	areas := []curriculum.CompetencyAreaSpec{
		{ID: mustID("area.backend-go.foundations"), Name: "Foundations and Go workspace", Description: "Zero-assumption command and module foundations with historical workspace context.", Scopes: []string{"foundations"}, OutcomeIDs: []curriculum.ID{outcomes[0].ID}, RequiredForProfessional: true, EvidenceRefs: refs(references, "terminal", "modules", "gopath")},
		{ID: mustID("area.backend-go.services"), Name: "Service construction and verification", Description: "HTTP, persistence, deterministic testing, and debugging boundaries.", Scopes: []string{"services"}, OutcomeIDs: []curriculum.ID{outcomes[1].ID, outcomes[2].ID}, RequiredForProfessional: true, EvidenceRefs: refs(references, "handlers", "transactions", "tests")},
		{ID: mustID("area.backend-go.security"), Name: "Service security", Description: "Trust boundaries and maintenance decisions.", Scopes: []string{"security"}, OutcomeIDs: []curriculum.ID{outcomes[4].ID}, RequiredForProfessional: true, EvidenceRefs: refs(references, "threats")},
		{ID: mustID("area.backend-go.operations"), Name: "Production operations", Description: "Observability and repeatable deployment boundaries.", Scopes: []string{"operations"}, OutcomeIDs: []curriculum.ID{outcomes[3].ID}, RequiredForProfessional: true, EvidenceRefs: refs(references, "observability", "deployment")},
	}
	profile := curriculum.DomainProfile{ID: mustID("profile.backend-go-reference"), Version: "backend-go-profile-v1", Domain: goal.Domain, SupportedScopes: append([]string(nil), goal.Scope...), Areas: areas}

	competencies := []curriculum.Competency{
		competency("foundations", areas[0], outcomes[0], curriculum.CompetencyUnderstand, references, "terminal", "modules", "gopath"),
		competency("build", areas[1], outcomes[1], curriculum.CompetencyApply, references, "handlers", "transactions"),
		competency("debug", areas[1], outcomes[2], curriculum.CompetencyAnalyze, references, "tests"),
		competency("security", areas[2], outcomes[4], curriculum.CompetencyAnalyze, references, "threats"),
		competency("operate", areas[3], outcomes[3], curriculum.CompetencyOperate, references, "observability", "deployment"),
	}
	candidates, err := curriculumapp.NewConceptCandidateExtractorV1().Extract(ctx, curriculumapp.ConceptCandidateExtractionRequest{EvidenceSets: []curriculum.CurriculumEvidenceSet{evidence}})
	if err != nil {
		return curriculumapp.CurriculumCompileRequest{}, curriculum.EnvironmentPack{}, err
	}
	byScope := make(map[string]curriculum.ConceptCandidate, len(candidates.Candidates))
	for _, candidate := range candidates.Candidates {
		byScope[candidate.Scope] = candidate
	}
	criteria := atomicCriteria()
	var plans []curriculumapp.AtomizationPlan
	var practice []curriculum.PracticeContextAssignment
	for _, spec := range backendGoConcepts {
		candidate := byScope[spec.scope]
		conceptID := mustConceptID("concept.backend-go." + spec.key)
		plans = append(plans, curriculumapp.AtomizationPlan{CandidateID: candidate.ID, Criteria: criteria, Hints: []curriculum.ConceptAtomizationHint{{
			CandidateID: candidate.ID, ConceptID: conceptID, Title: spec.title, Definition: spec.definition,
			Version: "concept-v1", Difficulty: spec.difficulty, Foundational: spec.foundational,
			ClaimRefs: append([]curriculum.EvidenceRef(nil), candidate.ClaimRefs...), Criteria: criteria,
		}}})
		practice = append(practice, curriculum.PracticeContextAssignment{ConceptID: conceptID, Context: spec.context})
	}
	semantics := []curriculum.ConceptPrerequisiteSemantic{
		prerequisite("modules", "terminal", curriculum.PrerequisiteHard, references, "Terminal execution precedes module-aware Go commands."),
		prerequisite("gopath", "terminal", curriculum.PrerequisiteExposureOnly, references, "Historical workspace commands require terminal context."),
		prerequisite("handlers", "modules", curriculum.PrerequisiteHard, references, "A service handler is built inside a module-aware project."),
		prerequisite("transactions", "modules", curriculum.PrerequisiteHard, references, "Database integration is introduced in a module-aware service."),
		prerequisite("tests", "handlers", curriculum.PrerequisiteHard, references, "Service tests observe handler behavior."),
		prerequisite("threats", "handlers", curriculum.PrerequisiteHard, references, "Trust boundaries are analyzed at service inputs."),
		prerequisite("observability", "handlers", curriculum.PrerequisiteHard, references, "Operational signals observe a running service boundary."),
		prerequisite("deployment", "observability", curriculum.PrerequisiteHard, references, "Deployment health requires observable signals."),
	}

	curriculumID := mustCurriculumID("curriculum.backend-go-reference")
	requirements, supports := coverage(curriculumID, goal.ID, references)
	goTool := curriculum.ToolRequirement{
		ID: mustID("tool.go"), DisplayName: "Go toolchain", Purpose: "Compile, test, and manage the reference Go module.", MinimumVersion: "1.24.0", Level: curriculum.ToolRequired,
		IntroducedAt: conceptPtr("modules"), WhenNeeded: conceptPtr("modules"), Platforms: []string{curriculum.EnvironmentPlatformDarwin, curriculum.EnvironmentPlatformLinux, curriculum.EnvironmentPlatformWindows},
		InstallGuidanceRefs: []curriculum.ID{mustID("install.go.darwin"), mustID("install.go.linux"), mustID("install.go.windows")}, EvidenceRefs: refs(references, "modules"),
	}
	environment := curriculum.EnvironmentPack{
		ID: mustID("environment.backend-go-reference"), Version: mustPackVersion("0.1.0"), SchemaVersion: curriculum.EnvironmentPackSchemaVersionV1,
		SupportedPlatforms: []string{curriculum.EnvironmentPlatformDarwin, curriculum.EnvironmentPlatformLinux, curriculum.EnvironmentPlatformWindows}, Tools: []curriculum.ToolRequirement{goTool},
		InstallGuidance: []curriculum.ToolInstallGuidance{
			installGuidance("darwin", curriculum.EnvironmentPlatformDarwin, references),
			installGuidance("linux", curriculum.EnvironmentPlatformLinux, references),
			installGuidance("windows", curriculum.EnvironmentPlatformWindows, references),
		},
	}
	request := curriculumapp.CurriculumCompileRequest{
		Input:        curriculum.CompilationInput{Goal: goal, SourceBundles: []curriculum.SourceBundleRef{bundle}, RequestedAt: builtAt},
		Config:       curriculum.CompilationConfig{CompilerVersion: curriculum.CurriculumCompilerVersionV1, SourcePolicy: curriculum.SourceReferencesOptionalForFixture, PackSchemaVersion: curriculum.LearningPackSchemaVersionV1},
		Metadata:     curriculumapp.CurriculumBuildMetadata{ID: curriculumID, Version: mustCurriculumVersion("2026.09.15-dev.1"), Title: "Backend engineering with Go reference", Description: "A bounded development curriculum used to verify the I-04 compiler and Learning Pack lifecycle.", CreatedAt: builtAt},
		EvidenceSets: []curriculum.CurriculumEvidenceSet{evidence}, DomainProfile: profile, Competencies: competencies,
		AtomizationPlans: plans, PrerequisiteSemantics: semantics,
		VocabularyDefinitions: []curriculum.VocabularyDefinition{
			{Term: "module", CanonicalConceptID: conceptID("modules"), IntroducedBy: conceptID("modules"), Scope: "Go workspace"},
			{Term: "handler", CanonicalConceptID: conceptID("handlers"), IntroducedBy: conceptID("handlers"), Scope: "HTTP service"},
		},
		VocabularyUses:       []curriculum.VocabularyUse{{Term: "module", UsedAt: conceptID("handlers")}, {Term: "module", UsedAt: conceptID("tests")}, {Term: "handler", UsedAt: conceptID("tests")}, {Term: "handler", UsedAt: conceptID("threats")}, {Term: "handler", UsedAt: conceptID("observability")}},
		CoverageRequirements: requirements, CoverageSupports: supports,
		AssumptionBaseline: curriculum.AssumptionBaseline{
			ID: mustID("baseline.backend-go.zero"), Version: "backend-go-zero-v1", DomainProfileID: profile.ID, DomainProfileVersion: profile.Version, LearnerProfile: curriculum.LearnerProfileZero,
			Requirements: []curriculum.FoundationRequirement{{ID: mustID("foundation.backend-go.terminal"), ConceptID: conceptID("terminal"), TargetCompetencyIDs: competencyIDs(competencies), Reason: "A zero-assumption learner needs an explicit command execution boundary before using the Go toolchain.", EvidenceRefs: refs(references, "terminal")}},
		},
		PracticeContext: practice, ContextualConceptIDs: []curriculum.ConceptID{conceptID("gopath")},
		BeginnerTools: []curriculum.ToolRequirement{goTool}, BeginnerToolUses: []curriculum.BeginnerToolUse{{ToolID: goTool.ID, UsedAt: conceptID("handlers")}},
	}
	return request, environment, nil
}

func backendGoManifest(compiled curriculum.CompilationResult) curriculum.PackManifest {
	return curriculum.PackManifest{
		ID: mustID("backend-go-reference"), Name: "Backend Go Reference", Description: "Development-only preview pack that exercises I-04; deterministic fixture evidence does not establish production completeness.",
		Version: mustPackVersion("0.1.0"), SchemaVersion: curriculum.LearningPackSchemaVersionV1, Domain: "backend engineering", Target: "Backend Go engineer development reference",
		Authors: []string{"Kelyro contributors"}, Maintainers: []string{"Kelyro contributors"}, License: "CC-BY-4.0", CreatedAt: compiled.BuildInfo.BuiltAt,
		MinimumKelyroVersion: mustPackVersion("0.2.0-alpha.3"), EnvironmentEntry: "environment/environment.yaml", CurriculumEntry: "curriculum/curriculum.yaml",
		SourceEvidenceEntry: "sources/evidence-report.json", BuildInfoEntry: "build/build-info.json", Status: curriculum.ConceptPreview, CurriculumID: compiled.Curriculum.ID,
	}
}

func competency(key string, area curriculum.CompetencyAreaSpec, outcome curriculum.GoalOutcome, level curriculum.CompetencyLevel, references map[string]curriculum.EvidenceRef, concepts ...string) curriculum.Competency {
	ids := make([]curriculum.ConceptID, 0, len(concepts))
	for _, concept := range concepts {
		ids = append(ids, conceptID(concept))
	}
	return curriculum.Competency{ID: mustID("competency.backend-go." + key), AreaID: area.ID, Area: area.Name, OutcomeID: outcome.ID, ExpectedLevel: level, EvidenceRefs: refs(references, concepts...), ConceptRefs: ids}
}

func coverage(curriculumID curriculum.CurriculumID, goalID curriculum.ID, references map[string]curriculum.EvidenceRef) ([]curriculum.CoverageRequirement, []curriculum.CoverageSupport) {
	var requirements []curriculum.CoverageRequirement
	var supports []curriculum.CoverageSupport
	allConcepts := make([]curriculum.ConceptID, 0, len(backendGoConcepts))
	for _, spec := range backendGoConcepts {
		allConcepts = append(allConcepts, conceptID(spec.key))
	}
	for _, dimension := range curriculum.AllCoverageDimensions() {
		requirementID := mustID("coverage.backend-go." + string(dimension))
		targetID := goalID
		targetKind := curriculum.CoverageTargetGoal
		if dimension == curriculum.CoverageProduction || dimension == curriculum.CoverageSecurity || dimension == curriculum.CoverageToolchain {
			targetID = mustID(curriculumID.String())
			targetKind = curriculum.CoverageTargetCurriculum
		}
		requirements = append(requirements, curriculum.CoverageRequirement{ID: requirementID, Dimension: dimension, TargetKind: targetKind, TargetID: targetID, Description: "Development reference coverage for " + string(dimension) + ".", EvidenceRefs: refs(references, "terminal")})
		supports = append(supports, curriculum.CoverageSupport{ID: mustID("support.backend-go." + string(dimension)), RequirementID: requirementID, ConceptIDs: append([]curriculum.ConceptID(nil), allConcepts...), EvidenceRefs: refs(references, "terminal"), Reason: "The reference concepts exercise this compiler coverage dimension."})
	}
	return requirements, supports
}

func prerequisite(concept, required string, kind curriculum.PrerequisiteKind, references map[string]curriculum.EvidenceRef, reason string) curriculum.ConceptPrerequisiteSemantic {
	return curriculum.ConceptPrerequisiteSemantic{ConceptID: conceptID(concept), RequiredConceptID: conceptID(required), Kind: kind, EvidenceRefs: refs(references, concept, required), Reason: reason}
}

func installGuidance(key, platform string, references map[string]curriculum.EvidenceRef) curriculum.ToolInstallGuidance {
	return curriculum.ToolInstallGuidance{ID: mustID("install.go." + key), Platform: platform, SourceName: "Official Go installation documentation", OfficialURL: "https://go.dev/doc/install", Instructions: "Follow the official platform instructions, then verify the selected toolchain in a terminal.", EvidenceRefs: refs(references, "terminal", "modules")}
}

func atomicCriteria() curriculum.AtomicConceptCriteria {
	return curriculum.AtomicConceptCriteria{
		Named: curriculum.AtomicityCriterionSatisfied, Defined: curriculum.AtomicityCriterionSatisfied, PrerequisiteBoundary: curriculum.AtomicityCriterionSatisfied,
		Explainable: curriculum.AtomicityCriterionSatisfied, Practicable: curriculum.AtomicityCriterionSatisfied, Assessable: curriculum.AtomicityCriterionSatisfied,
		Evidenced: curriculum.AtomicityCriterionSatisfied, MeaningfulStandalone: curriculum.AtomicityCriterionSatisfied,
	}
}

func refs(values map[string]curriculum.EvidenceRef, keys ...string) []curriculum.EvidenceRef {
	result := make([]curriculum.EvidenceRef, 0, len(keys))
	for _, key := range keys {
		result = append(result, values[key])
	}
	return result
}

func competencyIDs(values []curriculum.Competency) []curriculum.ID {
	result := make([]curriculum.ID, len(values))
	for index := range values {
		result[index] = values[index].ID
	}
	return result
}

func conceptID(key string) curriculum.ConceptID   { return mustConceptID("concept.backend-go." + key) }
func conceptPtr(key string) *curriculum.ConceptID { value := conceptID(key); return &value }

func mustID(value string) curriculum.ID {
	id, err := curriculum.NewID(value)
	if err != nil {
		panic(err)
	}
	return id
}
func mustConceptID(value string) curriculum.ConceptID {
	id, err := curriculum.NewConceptID(value)
	if err != nil {
		panic(err)
	}
	return id
}
func mustCurriculumID(value string) curriculum.CurriculumID {
	id, err := curriculum.NewCurriculumID(value)
	if err != nil {
		panic(err)
	}
	return id
}
func mustPackVersion(value string) curriculum.PackVersion {
	version, err := curriculum.NewPackVersion(value)
	if err != nil {
		panic(err)
	}
	return version
}
func mustCurriculumVersion(value string) curriculum.CurriculumVersion {
	version, err := curriculum.NewCurriculumVersion(value)
	if err != nil {
		panic(err)
	}
	return version
}
func mustTimestamp(value time.Time) curriculum.Timestamp {
	timestamp, err := curriculum.NewTimestamp(value)
	if err != nil {
		panic(err)
	}
	return timestamp
}
