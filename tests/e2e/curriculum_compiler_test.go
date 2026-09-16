//go:build e2e

package e2e_test

import (
	"archive/zip"
	"bytes"
	"context"
	"errors"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"testing"
	"time"

	"github.com/mishaaac/kelyro/internal/curriculum"
	curriculumapp "github.com/mishaaac/kelyro/internal/curriculum/application"
	"github.com/mishaaac/kelyro/internal/infra/learningmigration"
	"github.com/mishaaac/kelyro/internal/infra/learningpack"
	"github.com/mishaaac/kelyro/internal/infra/referencepack"
	"github.com/mishaaac/kelyro/internal/learning"
	learningapp "github.com/mishaaac/kelyro/internal/learning/application"
	learningmemory "github.com/mishaaac/kelyro/internal/learning/application/memory"
	"github.com/mishaaac/kelyro/internal/research"
)

func TestCurriculumCompilerAndPackLifecycleEndToEnd(t *testing.T) {
	ctx := context.Background()
	fixture := newCurriculumCompilerE2EFixture(t, ctx)

	t.Run("01 fresh valid bundle reaches roadmap", func(t *testing.T) {
		if fixture.ingestion.Evidence.Eligibility != curriculum.EvidenceReadyForCompile ||
			len(fixture.compiled.Curriculum.Concepts) != 9 {
			t.Fatalf("ingestion/compilation = %+v / %d concepts", fixture.ingestion, len(fixture.compiled.Curriculum.Concepts))
		}
		if fixture.validation.Pack == nil || len(fixture.validation.Errors) != 0 {
			t.Fatalf("pack validation = %+v", fixture.validation)
		}
		if fixture.active.Pack.Manifest.ID != fixture.manifest.ID {
			t.Fatalf("active pack = %+v", fixture.active)
		}

		projected, err := learningmigration.ProjectCurriculum(fixture.compiled.Curriculum)
		if err != nil {
			t.Fatal(err)
		}
		studentStore := learningmemory.New()
		createdAt := time.Date(2026, 9, 15, 13, 0, 0, 0, time.UTC)
		profiles := learningapp.NewProfileService(learningapp.NewStudentService(studentStore.Repositories().Students),
			learningapp.WithProfileClock(func() time.Time { return createdAt }))
		goalID := mustLearningID(t, "goal.backend-go-e2e")
		goals := learningapp.NewGoalLifecycleService(profiles, studentStore,
			learningapp.WithGoalClock(func() time.Time { return createdAt.Add(time.Minute) }),
			learningapp.WithGoalIDGenerator(func() (learning.ID, error) { return goalID, nil }))
		threshold, _ := learning.NewMasteryThreshold(.8)
		goal, err := goals.Set(ctx, learningapp.SetGoalInput{
			Title: "Backend Go", Domain: "backend engineering", TargetOutcome: "Build and operate Go services",
			StartingLevel: learning.ExperienceBeginner, MasteryThreshold: threshold,
		})
		if err != nil {
			t.Fatal(err)
		}
		instances := learningapp.NewCurriculumInstanceService(profiles, studentStore,
			learningapp.WithCurriculumInstanceClock(func() time.Time { return createdAt.Add(2 * time.Minute) }),
			learningapp.WithCurriculumInstanceIDGenerator(func() (learning.ID, error) { return mustLearningID(t, "instance.backend-go-e2e"), nil }))
		instance, err := instances.Create(ctx, goal.ID, projected, learning.CurriculumSourcePack)
		if err != nil {
			t.Fatal(err)
		}
		mastery := learningapp.NewMasteryPolicyService(profiles, studentStore.Repositories().Mastery,
			learningapp.WithMasteryPolicyClock(func() time.Time { return createdAt.Add(3 * time.Minute) }))
		plans := learningapp.NewAdaptiveDailyPlanService(profiles, mastery, studentStore,
			learningapp.WithAdaptiveDailyPlanClock(func() time.Time { return createdAt.Add(4 * time.Minute) }))
		dashboard := learningapp.NewProgressDashboardService(profiles, mastery, plans, studentStore,
			learningapp.WithProgressDashboardClock(func() time.Time { return createdAt.Add(4 * time.Minute) }))
		view, err := dashboard.Show(ctx)
		if err != nil {
			t.Fatal(err)
		}
		if view.Curriculum == nil || view.Curriculum.Instance.ID != instance.ID ||
			view.Curriculum.Instance.Source != learning.CurriculumSourcePack ||
			len(view.Roadmap) != len(projected.Nodes) {
			t.Fatalf("I-02 curriculum instance/roadmap = %+v / %d nodes", view.Curriculum, len(view.Roadmap))
		}
	})

	t.Run("02 missing evidence blocks compile", func(t *testing.T) {
		request := fixture.freshRequest(t, ctx)
		request.EvidenceSets = nil
		_, err := curriculumapp.NewCurriculumCompilerV1().Compile(ctx, request)
		if !errors.Is(err, curriculumapp.ErrInvalidState) || !strings.Contains(err.Error(), "evidence sets do not match") {
			t.Fatalf("missing-evidence compile error = %v", err)
		}
	})

	t.Run("03 unresolved critical conflict blocks compile", func(t *testing.T) {
		provider, bundle, critical := researchBundleFixture(t, fixture.syntheticEvidence, sourceBundleOptions{conflicted: true})
		ingested, err := curriculumapp.NewSourceBundleIngester(provider).Ingest(ctx, curriculumapp.EvidenceIngestionRequest{
			BundleID: bundle.ID, CriticalClaimIDs: []research.ClaimID{critical},
		})
		if err != nil || ingested.Accepted || ingested.Evidence.Eligibility != curriculum.EvidenceNotReady {
			t.Fatalf("conflicted ingestion = %+v / %v", ingested, err)
		}
		request := fixture.freshRequest(t, ctx)
		request.Input.SourceBundles = []curriculum.SourceBundleRef{ingested.Evidence.Bundle}
		request.EvidenceSets = []curriculum.CurriculumEvidenceSet{ingested.Evidence}
		if _, err := curriculumapp.NewCurriculumCompilerV1().Compile(ctx, request); !errors.Is(err, curriculumapp.ErrInvalidState) {
			t.Fatalf("conflicted compile error = %v", err)
		}
	})

	t.Run("04 historical source remains contextual", func(t *testing.T) {
		provider, bundle, _ := researchBundleFixture(t, fixture.syntheticEvidence, sourceBundleOptions{historicalSource: true})
		ingested, err := curriculumapp.NewSourceBundleIngester(provider).Ingest(ctx, curriculumapp.EvidenceIngestionRequest{BundleID: bundle.ID})
		if err != nil || !ingested.Accepted || ingested.Evidence.Eligibility != curriculum.EvidenceReadyWithCaveats ||
			ingested.Evidence.SourceAuthority[0].TemporalScope != "historical" {
			t.Fatalf("historical ingestion = %+v / %v", ingested, err)
		}
		request := fixture.freshRequest(t, ctx)
		request.Input.SourceBundles = []curriculum.SourceBundleRef{ingested.Evidence.Bundle}
		request.EvidenceSets = []curriculum.CurriculumEvidenceSet{ingested.Evidence}
		compiled, err := curriculumapp.NewCurriculumCompilerV1().Compile(ctx, request)
		if err != nil {
			t.Fatal(err)
		}
		concept := curriculumConcept(t, compiled.Curriculum, "concept.backend-go.gopath")
		if concept.Status != curriculum.ConceptLegacy {
			t.Fatalf("historical concept status = %q", concept.Status)
		}
		found := false
		for _, classification := range compiled.Diagnostics.Temporal.Classifications {
			found = found || classification.Status == curriculum.ConceptLegacy || classification.Status == curriculum.ConceptHistorical
		}
		if !found {
			t.Fatalf("temporal classification omitted historical context: %+v", compiled.Diagnostics.Temporal)
		}
	})

	t.Run("05 experimental claim remains experimental", func(t *testing.T) {
		provider, bundle, _ := researchBundleFixture(t, fixture.syntheticEvidence, sourceBundleOptions{experimentalClaim: "claim.backend-go.tests"})
		ingested, err := curriculumapp.NewSourceBundleIngester(provider).Ingest(ctx, curriculumapp.EvidenceIngestionRequest{BundleID: bundle.ID})
		if err != nil || !ingested.Accepted {
			t.Fatalf("experimental ingestion = %+v / %v", ingested, err)
		}
		request := fixture.freshRequest(t, ctx)
		originalEvidence := request.EvidenceSets
		request.Input.SourceBundles = []curriculum.SourceBundleRef{ingested.Evidence.Bundle}
		request.EvidenceSets = []curriculum.CurriculumEvidenceSet{ingested.Evidence}
		retargetAtomizationPlans(t, ctx, &request, originalEvidence, request.EvidenceSets)
		compiled, err := curriculumapp.NewCurriculumCompilerV1().Compile(ctx, request)
		if err != nil {
			t.Fatal(err)
		}
		if got := curriculumConcept(t, compiled.Curriculum, "concept.backend-go.tests").Status; got != curriculum.ConceptExperimental {
			t.Fatalf("experimental concept status = %q", got)
		}
	})

	t.Run("06 missing prerequisite is expanded", func(t *testing.T) {
		request := fixture.freshRequest(t, ctx)
		reference := request.EvidenceSets[0].Claims[0]
		availableID := mustCurriculumConceptID(t, "concept.backend-go.process")
		available := curriculum.Concept{
			ID: availableID, Title: "Operating-system process", Definition: "A process is one executing program instance.",
			Version: "concept-v1", Atomicity: curriculum.AtomicityAtomic, Difficulty: curriculum.DifficultyIntroductory,
			Status: curriculum.ConceptCurrent, Foundational: true,
			EvidenceRefs: []curriculum.EvidenceRef{{BundleID: request.EvidenceSets[0].Bundle.ID, ClaimID: reference.ID}},
		}
		terminalID := mustCurriculumConceptID(t, "concept.backend-go.terminal")
		request.AvailableConcepts = []curriculum.Concept{available}
		request.PrerequisiteSemantics = append(request.PrerequisiteSemantics, curriculum.ConceptPrerequisiteSemantic{
			ConceptID: terminalID, RequiredConceptID: availableID, Kind: curriculum.PrerequisiteHard,
			EvidenceRefs: available.EvidenceRefs, Reason: "A process precedes terminal process execution.",
		})
		request.Competencies[0].ConceptRefs = append(request.Competencies[0].ConceptRefs, availableID)
		compiled, err := curriculumapp.NewCurriculumCompilerV1().Compile(ctx, request)
		if err != nil {
			t.Fatal(err)
		}
		_ = curriculumConcept(t, compiled.Curriculum, availableID.String())
	})

	t.Run("07 definition before use failure is retained", func(t *testing.T) {
		request := fixture.freshRequest(t, ctx)
		request.VocabularyDefinitions[0].IntroducedBy = mustCurriculumConceptID(t, "concept.backend-go.deployment")
		compiled, err := curriculumapp.NewCurriculumCompilerV1().Compile(ctx, request)
		if err != nil {
			t.Fatal(err)
		}
		if compiled.Diagnostics.DefinitionBeforeUse.Passed || len(compiled.Diagnostics.DefinitionBeforeUse.Violations) == 0 {
			t.Fatalf("definition-before-use audit = %+v", compiled.Diagnostics.DefinitionBeforeUse)
		}
	})

	t.Run("08 zero assumption failure is retained", func(t *testing.T) {
		request := fixture.freshRequest(t, ctx)
		request.AssumptionBaseline.Requirements[0].ConceptID = mustCurriculumConceptID(t, "concept.backend-go.handlers")
		compiled, err := curriculumapp.NewCurriculumCompilerV1().Compile(ctx, request)
		if err != nil {
			t.Fatal(err)
		}
		if compiled.Diagnostics.ZeroAssumption.Passed || len(compiled.Diagnostics.ZeroAssumption.Violations) == 0 {
			t.Fatalf("zero-assumption audit = %+v", compiled.Diagnostics.ZeroAssumption)
		}
	})

	for _, scenario := range []struct {
		name      string
		dimension curriculum.CoverageDimension
	}{
		{"09 production gap", curriculum.CoverageProduction},
		{"10 security gap", curriculum.CoverageSecurity},
	} {
		scenario := scenario
		t.Run(scenario.name, func(t *testing.T) {
			request := fixture.freshRequest(t, ctx)
			request.CoverageSupports = removeCoverageSupport(request.CoverageSupports, scenario.dimension)
			compiled, err := curriculumapp.NewCurriculumCompilerV1().Compile(ctx, request)
			if err != nil {
				t.Fatal(err)
			}
			if !hasCoverageGap(compiled, scenario.dimension) {
				t.Fatalf("%s gap absent: %+v", scenario.dimension, compiled.Gaps)
			}
		})
	}

	t.Run("11 pack dependency is resolved explicitly", func(t *testing.T) {
		root := fixture.manifest
		dependencyID := mustCurriculumID(t, "pack.backend-go-foundations")
		root.Dependencies = []curriculum.PackDependency{{PackID: dependencyID, Constraint: ">=1.0.0 <2.0.0"}}
		resolver := curriculumapp.NewPackDependencyResolverV1()
		missing, err := resolver.Resolve(ctx, curriculumapp.PackDependencyResolutionRequest{Root: root})
		if err != nil || missing.Status != curriculum.PackDependenciesFailed || missing.Issues[0].Kind != curriculum.PackDependencyMissing {
			t.Fatalf("missing dependency = %+v / %v", missing, err)
		}
		dependency := fixture.manifest
		dependency.ID = dependencyID
		dependency.Name = "Backend Go Foundations"
		dependency.Dependencies = nil
		dependency.Version = mustPackVersion(t, "1.1.0")
		resolved, err := resolver.Resolve(ctx, curriculumapp.PackDependencyResolutionRequest{Root: root, Available: []curriculum.PackManifest{dependency}})
		if err != nil || resolved.Status != curriculum.PackDependenciesResolved || len(resolved.InstallOrder) != 2 || resolved.InstallOrder[1].PackID != root.ID {
			t.Fatalf("resolved dependency = %+v / %v", resolved, err)
		}
	})

	t.Run("12 environment pack survives validation", func(t *testing.T) {
		pack := fixture.validation.Pack
		if pack == nil || pack.Environment == nil || len(pack.Environment.Tools) != 1 || len(pack.Environment.InstallGuidance) != 3 {
			t.Fatalf("validated environment pack = %+v", pack)
		}
		platforms := append([]string(nil), pack.Environment.SupportedPlatforms...)
		sort.Strings(platforms)
		if strings.Join(platforms, ",") != "darwin,linux,windows" {
			t.Fatalf("environment platforms = %v", platforms)
		}
	})

	t.Run("13 upgrade adds concept", func(t *testing.T) {
		oldDefinition := fixture.compiled.Curriculum
		newDefinition, addedID := curriculumWithAddedConcept(t, oldDefinition)
		classification, err := curriculumapp.NewCurriculumChangeClassifierV1().Classify(ctx, curriculumapp.CurriculumChangeClassificationRequest{Old: oldDefinition, New: newDefinition})
		if err != nil || !classificationHas(classification, curriculum.ChangeConceptAdded, addedID) {
			t.Fatalf("add-concept classification = %+v / %v", classification, err)
		}
		plan, err := curriculumapp.NewCurriculumMigrationPlannerV1().Plan(ctx, curriculumapp.CurriculumMigrationPlanningRequest{Old: oldDefinition, New: newDefinition, Classification: classification})
		if err != nil || !migrationHas(plan, curriculum.MigrationInitializeUnknown, addedID) {
			t.Fatalf("add-concept migration = %+v / %v", plan, err)
		}
	})

	t.Run("14 upgrade splits concept without mastery transfer", func(t *testing.T) {
		oldDefinition := fixture.compiled.Curriculum
		newDefinition, oldID, firstID, secondID := curriculumWithSplitConcept(t, oldDefinition)
		mapping := curriculum.ConceptIdentityMapping{
			OldConceptIDs: []curriculum.ConceptID{oldID}, NewConceptIDs: []curriculum.ConceptID{firstID, secondID},
			Rationale: "Historical GOPATH context is separated into configuration and workspace behavior.",
		}
		classification, err := curriculumapp.NewCurriculumChangeClassifierV1().Classify(ctx, curriculumapp.CurriculumChangeClassificationRequest{Old: oldDefinition, New: newDefinition, IdentityMappings: []curriculum.ConceptIdentityMapping{mapping}})
		if err != nil || !classificationHas(classification, curriculum.ChangeConceptSplit, oldID) {
			t.Fatalf("split classification = %+v / %v", classification, err)
		}
		plan, err := curriculumapp.NewCurriculumMigrationPlannerV1().Plan(ctx, curriculumapp.CurriculumMigrationPlanningRequest{Old: oldDefinition, New: newDefinition, Classification: classification, IdentityMappings: []curriculum.ConceptIdentityMapping{mapping}})
		if err != nil || !migrationHas(plan, curriculum.MigrationSplitNoTransfer, firstID) || !migrationHas(plan, curriculum.MigrationSplitNoTransfer, secondID) {
			t.Fatalf("split migration = %+v / %v", plan, err)
		}
	})

	t.Run("15 migration preserves mastery", func(t *testing.T) {
		assertMigrationPreservesMastery(t, ctx, fixture.compiled.Curriculum)
	})

	t.Run("16 export is copyright safe", func(t *testing.T) {
		assertCopyrightSafePack(t, fixture.build.PortableArchive, fixture.ingestion.Evidence.Claims)
	})
}

type curriculumCompilerE2EFixture struct {
	syntheticEvidence curriculum.CurriculumEvidenceSet
	ingestion         curriculumapp.EvidenceIngestionResult
	compiled          curriculum.CompilationResult
	manifest          curriculum.PackManifest
	build             curriculumapp.PackBuildResult
	validation        curriculumapp.PackValidationResult
	active            curriculumapp.InstalledPack
}

func (fixture curriculumCompilerE2EFixture) freshRequest(t *testing.T, ctx context.Context) curriculumapp.CurriculumCompileRequest {
	t.Helper()
	request, _, err := referencepack.BackendGoDevelopmentFixture(ctx)
	if err != nil {
		t.Fatal(err)
	}
	request.Input.SourceBundles = []curriculum.SourceBundleRef{fixture.ingestion.Evidence.Bundle}
	request.EvidenceSets = []curriculum.CurriculumEvidenceSet{fixture.ingestion.Evidence}
	return request
}

func retargetAtomizationPlans(t *testing.T, ctx context.Context, request *curriculumapp.CurriculumCompileRequest, oldEvidence, newEvidence []curriculum.CurriculumEvidenceSet) {
	t.Helper()
	extractor := curriculumapp.NewConceptCandidateExtractorV1()
	oldCandidates, err := extractor.Extract(ctx, curriculumapp.ConceptCandidateExtractionRequest{EvidenceSets: oldEvidence})
	if err != nil {
		t.Fatal(err)
	}
	newCandidates, err := extractor.Extract(ctx, curriculumapp.ConceptCandidateExtractionRequest{EvidenceSets: newEvidence})
	if err != nil {
		t.Fatal(err)
	}
	oldScope := make(map[curriculum.ID]string, len(oldCandidates.Candidates))
	for _, candidate := range oldCandidates.Candidates {
		oldScope[candidate.ID] = candidate.Scope
	}
	newByScope := make(map[string]curriculum.ID, len(newCandidates.Candidates))
	for _, candidate := range newCandidates.Candidates {
		newByScope[candidate.Scope] = candidate.ID
	}
	for planIndex := range request.AtomizationPlans {
		scope, exists := oldScope[request.AtomizationPlans[planIndex].CandidateID]
		if !exists {
			t.Fatalf("old atomization candidate %s has no scope", request.AtomizationPlans[planIndex].CandidateID)
		}
		candidateID, exists := newByScope[scope]
		if !exists {
			t.Fatalf("new evidence has no candidate for scope %q", scope)
		}
		request.AtomizationPlans[planIndex].CandidateID = candidateID
		for hintIndex := range request.AtomizationPlans[planIndex].Hints {
			request.AtomizationPlans[planIndex].Hints[hintIndex].CandidateID = candidateID
		}
	}
}

func newCurriculumCompilerE2EFixture(t *testing.T, ctx context.Context) curriculumCompilerE2EFixture {
	t.Helper()
	request, environment, err := referencepack.BackendGoDevelopmentFixture(ctx)
	if err != nil {
		t.Fatal(err)
	}
	synthetic := request.EvidenceSets[0]
	provider, bundle, _ := researchBundleFixture(t, synthetic, sourceBundleOptions{})
	ingestion, err := curriculumapp.NewSourceBundleIngester(provider).Ingest(ctx, curriculumapp.EvidenceIngestionRequest{BundleID: bundle.ID})
	if err != nil || !ingestion.Accepted {
		t.Fatalf("ingest I-03 fixture = %+v / %v", ingestion, err)
	}
	request.Input.SourceBundles = []curriculum.SourceBundleRef{ingestion.Evidence.Bundle}
	request.EvidenceSets = []curriculum.CurriculumEvidenceSet{ingestion.Evidence}
	compiled, err := curriculumapp.NewCurriculumCompilerV1().Compile(ctx, request)
	if err != nil {
		t.Fatal(err)
	}
	manifest := curriculum.PackManifest{
		ID: mustCurriculumID(t, "backend-go-e2e"), Name: "Backend Go E2E", Description: "Offline compiler lifecycle fixture.",
		Version: mustPackVersion(t, "1.0.0"), SchemaVersion: curriculum.LearningPackSchemaVersionV1,
		Domain: "backend engineering", Target: "Backend Go engineer", Authors: []string{"Kelyro contributors"},
		Maintainers: []string{"Kelyro contributors"}, License: "CC-BY-4.0", CreatedAt: compiled.BuildInfo.BuiltAt,
		MinimumKelyroVersion: mustPackVersion(t, "0.2.0-alpha.3"), EnvironmentEntry: "environment/environment.yaml",
		CurriculumEntry: "curriculum/curriculum.yaml", SourceEvidenceEntry: "sources/evidence-report.json",
		BuildInfoEntry: "build/build-info.json", Status: curriculum.ConceptPreview, CurriculumID: compiled.Curriculum.ID,
	}
	sourceID := ingestion.Evidence.SourceAuthority[0].SourceID
	built, err := learningpack.NewBuilder().Build(ctx, curriculumapp.PackBuildRequest{
		Manifest: manifest, Compilation: compiled, EvidenceSets: request.EvidenceSets, Environment: &environment,
		Citations: []curriculum.EvidenceReportCitation{{SourceID: sourceID, Title: "Official fixture documentation", URL: "https://example.invalid/backend-go"}},
		Assets:    []curriculumapp.PackBuildAsset{{Path: "assets/E2E-NOTICE.md", Content: []byte("# E2E fixture\n\nKelyro-authored deterministic test content.\n"), License: "CC-BY-4.0", CopyrightHolder: "Kelyro contributors", KelyroAuthored: true}},
	})
	if err != nil {
		t.Fatal(err)
	}
	archivePath := filepath.Join(t.TempDir(), "backend-go-e2e.zip")
	if err := os.WriteFile(archivePath, built.PortableArchive, 0o600); err != nil {
		t.Fatal(err)
	}
	validator := learningpack.NewValidator()
	validation, err := validator.Validate(ctx, curriculumapp.PackSource{Path: archivePath})
	if err != nil || len(validation.Errors) != 0 {
		t.Fatalf("validate pack = %+v / %v", validation, err)
	}
	repository := newE2EPackRepository()
	installer := curriculumapp.NewPackInstallerV1(validator, curriculumapp.NewPackDependencyResolverV1(), repository,
		e2eCurriculumClock{at: mustCurriculumTimestamp(t, time.Date(2026, 9, 15, 12, 30, 0, 0, time.UTC))})
	installed, err := installer.Install(ctx, curriculumapp.PackInstallRequest{Source: curriculumapp.PackSource{Path: archivePath}})
	if err != nil || !installed.Installed {
		t.Fatalf("install pack = %+v / %v", installed, err)
	}
	if _, err := installer.Activate(ctx, curriculumapp.PackActivateRequest{WorkspaceRoot: t.TempDir(), PackID: manifest.ID, Version: manifest.Version}); err != nil {
		t.Fatal(err)
	}
	active, err := installer.Active(ctx, repository.workspace)
	if err != nil {
		t.Fatal(err)
	}
	return curriculumCompilerE2EFixture{syntheticEvidence: synthetic, ingestion: ingestion, compiled: compiled, manifest: manifest, build: built, validation: validation, active: active}
}

type sourceBundleOptions struct {
	conflicted        bool
	experimentalClaim string
	historicalSource  bool
}

type e2eResearchProvider struct {
	bundle    research.SourceBundle
	claims    map[research.ClaimID]research.Claim
	conflicts map[research.ID]research.Conflict
}

func (provider *e2eResearchProvider) GetBundle(_ context.Context, id research.ID) (research.SourceBundle, error) {
	if provider.bundle.ID != id {
		return research.SourceBundle{}, curriculumapp.ErrNotFound
	}
	return provider.bundle, nil
}

func (provider *e2eResearchProvider) GetClaim(_ context.Context, id research.ClaimID) (research.Claim, error) {
	claim, exists := provider.claims[id]
	if !exists {
		return research.Claim{}, curriculumapp.ErrNotFound
	}
	return claim, nil
}

func (provider *e2eResearchProvider) GetConflict(_ context.Context, id research.ID) (research.Conflict, error) {
	conflict, exists := provider.conflicts[id]
	if !exists {
		return research.Conflict{}, curriculumapp.ErrNotFound
	}
	return conflict, nil
}

func researchBundleFixture(t *testing.T, evidence curriculum.CurriculumEvidenceSet, options sourceBundleOptions) (*e2eResearchProvider, research.SourceBundle, research.ClaimID) {
	t.Helper()
	topic, err := research.NewResearchTopic("Backend Go E2E", "software", "Go")
	if err != nil {
		t.Fatal(err)
	}
	verifiedAt := mustResearchTimestamp(t, time.Date(2026, 9, 15, 11, 0, 0, 0, time.UTC))
	sourceID := mustResearchSourceID(t, evidence.SourceAuthority[0].SourceID.String())
	claimIDs := make([]research.ClaimID, 0, len(evidence.Claims))
	claims := make(map[research.ClaimID]research.Claim, len(evidence.Claims))
	for index, item := range evidence.Claims {
		claimID := mustResearchClaimID(t, item.ID.String())
		claimIDs = append(claimIDs, claimID)
		confidence, _ := research.NewClaimConfidence(item.Confidence)
		status := research.ClaimStatusStable
		switch item.Status {
		case curriculum.ConceptPreview:
			status = research.ClaimStatusPreview
		case curriculum.ConceptExperimental:
			status = research.ClaimStatusExperimental
		case curriculum.ConceptLegacy:
			status = research.ClaimStatusLegacy
		}
		if item.ID.String() == options.experimentalClaim {
			status = research.ClaimStatusExperimental
		}
		claims[claimID] = research.Claim{
			ID: claimID, Topic: topic, Statement: item.Statement, Type: research.ClaimType(item.Kind), Scope: item.Scope,
			StatusScope: status, Confidence: confidence, SourceIDs: []research.SourceID{sourceID},
			EvidenceIDs: []research.ID{mustResearchID(t, "evidence.e2e."+string(rune('a'+index)))}, CreatedAt: verifiedAt,
		}
	}
	score, _ := research.NewFreshnessScore(1)
	bundleSource := research.SourceBundleSource{SourceID: sourceID, Role: research.BundleSourcePrimary, TemporalScope: research.SourceTemporalCurrent}
	var issues []research.SourceBundleIssue
	if options.historicalSource {
		warning, warningErr := research.SourceTemporalHistorical.Warning(nil)
		if warningErr != nil {
			t.Fatal(warningErr)
		}
		bundleSource = research.SourceBundleSource{SourceID: sourceID, Role: research.BundleSourceHistorical, TemporalScope: research.SourceTemporalHistorical, Warning: warning}
		issues = []research.SourceBundleIssue{research.BundleIssueNonCurrentSource}
	}
	bundleInput := research.SourceBundle{
		ID: mustResearchID(t, evidence.Bundle.ID.String()), RunID: mustResearchID(t, "run.backend-go-e2e"),
		Topic: topic, Purpose: research.PurposeCurrentUsage, ClaimIDs: claimIDs,
		Sources:   []research.SourceBundleSource{bundleSource},
		Freshness: research.SourceBundleFreshness{State: research.FreshnessFresh, Score: score, LastVerifiedAt: &verifiedAt, SourceAlgorithms: []string{research.FreshnessAlgorithmV1}, AlgorithmVersion: research.SourceBundleFreshnessV1},
		Issues:    issues, VerifiedAt: verifiedAt,
	}
	provider := &e2eResearchProvider{claims: claims, conflicts: make(map[research.ID]research.Conflict)}
	critical := claimIDs[0]
	if options.conflicted {
		conflictID := mustResearchID(t, "conflict.backend-go-e2e")
		confidence, _ := research.NewClaimConfidence(.9)
		provider.conflicts[conflictID] = research.Conflict{
			ID: conflictID, Type: research.ConflictDirectContradiction, ClaimIDs: []research.ClaimID{claimIDs[0], claimIDs[1]},
			Confidence: confidence, Reason: "Critical fixture claims conflict.", Unresolved: true,
			DetectedAt: verifiedAt, AlgorithmVersion: research.ConflictResolverAlgorithmV1,
		}
		bundleInput.ConflictIDs = []research.ID{conflictID}
		bundleInput.Issues = append(bundleInput.Issues, research.BundleIssueUnresolvedConflict)
	}
	bundle, err := research.SealSourceBundleV1(bundleInput)
	if err != nil {
		t.Fatal(err)
	}
	provider.bundle = bundle
	return provider, bundle, critical
}

type e2eCurriculumClock struct{ at curriculum.Timestamp }

func (clock e2eCurriculumClock) Now() curriculum.Timestamp { return clock.at }

type e2ePackRepository struct {
	values      map[string]curriculumapp.InstalledPack
	activations map[string]curriculumapp.PackActivation
	workspace   string
}

func newE2EPackRepository() *e2ePackRepository {
	return &e2ePackRepository{values: make(map[string]curriculumapp.InstalledPack), activations: make(map[string]curriculumapp.PackActivation)}
}

func (repository *e2ePackRepository) Add(_ context.Context, artifact curriculumapp.PackInstallationArtifact) error {
	key := artifact.Pack.Manifest.ID.String() + "@" + artifact.Pack.Manifest.Version.String()
	if _, exists := repository.values[key]; exists {
		return curriculumapp.Classify(curriculumapp.ErrorConflict, "e2e add pack", nil)
	}
	repository.values[key] = artifact.InstalledPack
	return nil
}

func (repository *e2ePackRepository) Get(_ context.Context, id curriculum.ID, version curriculum.PackVersion) (curriculumapp.InstalledPack, error) {
	value, exists := repository.values[id.String()+"@"+version.String()]
	if !exists {
		return curriculumapp.InstalledPack{}, curriculumapp.Classify(curriculumapp.ErrorNotFound, "e2e get pack", nil)
	}
	return value, nil
}

func (repository *e2ePackRepository) List(context.Context) ([]curriculumapp.InstalledPack, error) {
	values := make([]curriculumapp.InstalledPack, 0, len(repository.values))
	for _, value := range repository.values {
		values = append(values, value)
	}
	return values, nil
}

func (repository *e2ePackRepository) Activate(_ context.Context, workspace string, activation curriculumapp.PackActivation) error {
	repository.workspace = workspace
	repository.activations[workspace] = activation
	return nil
}

func (repository *e2ePackRepository) Active(ctx context.Context, workspace string) (curriculumapp.InstalledPack, error) {
	activation, exists := repository.activations[workspace]
	if !exists {
		return curriculumapp.InstalledPack{}, curriculumapp.Classify(curriculumapp.ErrorNotFound, "e2e active pack", nil)
	}
	return repository.Get(ctx, activation.PackID, activation.Version)
}

func removeCoverageSupport(values []curriculum.CoverageSupport, dimension curriculum.CoverageDimension) []curriculum.CoverageSupport {
	target := "coverage.backend-go." + string(dimension)
	result := make([]curriculum.CoverageSupport, 0, len(values))
	for _, value := range values {
		if value.RequirementID.String() != target {
			result = append(result, value)
		}
	}
	return result
}

func hasCoverageGap(compiled curriculum.CompilationResult, dimension curriculum.CoverageDimension) bool {
	want := map[curriculum.CoverageDimension]curriculum.GapKind{
		curriculum.CoverageProduction: curriculum.GapMissingProduction,
		curriculum.CoverageSecurity:   curriculum.GapMissingSecurity,
	}[dimension]
	for _, gap := range compiled.Gaps {
		if gap.Kind == want {
			return true
		}
	}
	return false
}

func curriculumWithAddedConcept(t *testing.T, old curriculum.CurriculumDefinition) (curriculum.CurriculumDefinition, curriculum.ConceptID) {
	t.Helper()
	result := old
	result.Version = mustCurriculumVersion(t, "2026.09.15-e2e.2")
	id := mustCurriculumConceptID(t, "concept.backend-go.resilience")
	reference := old.Concepts[0].EvidenceRefs[0]
	result.Concepts = append(append([]curriculum.Concept(nil), old.Concepts...), curriculum.Concept{
		ID: id, Title: "Service resilience boundary", Definition: "A resilience boundary contains and reports one class of service failure.",
		Version: "concept-v1", Atomicity: curriculum.AtomicityAtomic, Difficulty: curriculum.DifficultyAdvanced,
		Status: curriculum.ConceptCurrent, EvidenceRefs: []curriculum.EvidenceRef{reference},
	})
	result.Competencies.Competencies = append([]curriculum.Competency(nil), old.Competencies.Competencies...)
	last := len(result.Competencies.Competencies) - 1
	conceptRefs := append([]curriculum.ConceptID(nil), result.Competencies.Competencies[last].ConceptRefs...)
	result.Competencies.Competencies[last].ConceptRefs = append(conceptRefs, id)
	result.Topics = cloneTopics(old.Topics)
	result.Topics[len(result.Topics)-1].ConceptIDs = append(result.Topics[len(result.Topics)-1].ConceptIDs, id)
	if err := result.Validate(); err != nil {
		t.Fatalf("added-concept curriculum: %v", err)
	}
	return result, id
}

func curriculumWithSplitConcept(t *testing.T, old curriculum.CurriculumDefinition) (curriculum.CurriculumDefinition, curriculum.ConceptID, curriculum.ConceptID, curriculum.ConceptID) {
	t.Helper()
	result := old
	result.Version = mustCurriculumVersion(t, "2026.09.15-e2e.3")
	oldID := mustCurriculumConceptID(t, "concept.backend-go.gopath")
	firstID := mustCurriculumConceptID(t, "concept.backend-go.gopath-config")
	secondID := mustCurriculumConceptID(t, "concept.backend-go.gopath-workspace")
	original := curriculumConcept(t, old, oldID.String())
	first, second := original, original
	first.ID, first.Title, first.Definition = firstID, "GOPATH configuration", "GOPATH configuration is historical environment context for pre-module workflows."
	second.ID, second.Title, second.Definition = secondID, "GOPATH workspace", "A GOPATH workspace is historical source-layout context for pre-module workflows."
	result.Concepts = replaceSplitConcept(old.Concepts, oldID, first, second)
	result.Competencies.Competencies = append([]curriculum.Competency(nil), old.Competencies.Competencies...)
	for index := range result.Competencies.Competencies {
		result.Competencies.Competencies[index].ConceptRefs = replaceSplitID(result.Competencies.Competencies[index].ConceptRefs, oldID, firstID, secondID)
	}
	result.Topics = cloneTopics(old.Topics)
	for index := range result.Topics {
		result.Topics[index].ConceptIDs = replaceSplitID(result.Topics[index].ConceptIDs, oldID, firstID, secondID)
	}
	result.Prerequisites = make([]curriculum.Prerequisite, 0, len(old.Prerequisites)+1)
	for _, prerequisite := range old.Prerequisites {
		if prerequisite.ConceptID != oldID && prerequisite.RequiredConceptID != oldID {
			result.Prerequisites = append(result.Prerequisites, prerequisite)
			continue
		}
		if prerequisite.ConceptID == oldID {
			left, right := prerequisite, prerequisite
			left.ConceptID, right.ConceptID = firstID, secondID
			result.Prerequisites = append(result.Prerequisites, left, right)
		}
	}
	if err := result.Validate(); err != nil {
		t.Fatalf("split curriculum: %v", err)
	}
	return result, oldID, firstID, secondID
}

func cloneTopics(values []curriculum.TopicSpec) []curriculum.TopicSpec {
	result := append([]curriculum.TopicSpec(nil), values...)
	for index := range result {
		result[index].ConceptIDs = append([]curriculum.ConceptID(nil), result[index].ConceptIDs...)
	}
	return result
}

func replaceSplitConcept(values []curriculum.Concept, oldID curriculum.ConceptID, replacements ...curriculum.Concept) []curriculum.Concept {
	result := make([]curriculum.Concept, 0, len(values)+len(replacements)-1)
	for _, value := range values {
		if value.ID == oldID {
			result = append(result, replacements...)
		} else {
			result = append(result, value)
		}
	}
	return result
}

func replaceSplitID(values []curriculum.ConceptID, oldID curriculum.ConceptID, replacements ...curriculum.ConceptID) []curriculum.ConceptID {
	result := make([]curriculum.ConceptID, 0, len(values)+len(replacements)-1)
	for _, value := range values {
		if value == oldID {
			result = append(result, replacements...)
		} else {
			result = append(result, value)
		}
	}
	return result
}

func classificationHas(classification curriculum.CurriculumChangeClassification, kind curriculum.CurriculumChangeKind, id curriculum.ConceptID) bool {
	for _, change := range classification.Changes {
		if change.Kind != kind {
			continue
		}
		for _, affected := range change.AffectedConcepts {
			if affected == id {
				return true
			}
		}
	}
	return false
}

func migrationHas(plan curriculum.CurriculumMigrationPlan, kind curriculum.CurriculumMigrationActionKind, id curriculum.ConceptID) bool {
	for _, action := range plan.Actions {
		if action.Kind != kind {
			continue
		}
		for _, target := range action.ToConceptIDs {
			if target == id {
				return true
			}
		}
	}
	return false
}

func assertMigrationPreservesMastery(t *testing.T, ctx context.Context, oldDefinition curriculum.CurriculumDefinition) {
	t.Helper()
	newDefinition := oldDefinition
	newDefinition.Version = mustCurriculumVersion(t, "2026.09.15-e2e.4")
	newDefinition.Description += " Metadata-only migration fixture."
	classification, err := curriculumapp.NewCurriculumChangeClassifierV1().Classify(ctx, curriculumapp.CurriculumChangeClassificationRequest{Old: oldDefinition, New: newDefinition})
	if err != nil {
		t.Fatal(err)
	}
	plan, err := curriculumapp.NewCurriculumMigrationPlannerV1().Plan(ctx, curriculumapp.CurriculumMigrationPlanningRequest{Old: oldDefinition, New: newDefinition, Classification: classification})
	if err != nil {
		t.Fatal(err)
	}
	oldProjected, err := learningmigration.ProjectCurriculum(oldDefinition)
	if err != nil {
		t.Fatal(err)
	}
	newProjected, err := learningmigration.ProjectCurriculum(newDefinition)
	if err != nil {
		t.Fatal(err)
	}
	store := learningmemory.New()
	createdAt := time.Date(2026, 9, 15, 14, 0, 0, 0, time.UTC)
	profiles := learningapp.NewProfileService(learningapp.NewStudentService(store.Repositories().Students),
		learningapp.WithProfileClock(func() time.Time { return createdAt }))
	goalID := mustLearningID(t, "goal.migration-e2e")
	goals := learningapp.NewGoalLifecycleService(profiles, store,
		learningapp.WithGoalClock(func() time.Time { return createdAt.Add(time.Minute) }),
		learningapp.WithGoalIDGenerator(func() (learning.ID, error) { return goalID, nil }))
	threshold, _ := learning.NewMasteryThreshold(.8)
	goal, err := goals.Set(ctx, learningapp.SetGoalInput{Title: "Migration E2E", Domain: "backend engineering", TargetOutcome: "Preserve mastery", StartingLevel: learning.ExperienceBeginner, MasteryThreshold: threshold})
	if err != nil {
		t.Fatal(err)
	}
	generatedIDs := []learning.ID{mustLearningID(t, "instance.migration-old"), mustLearningID(t, "instance.migration-new")}
	instanceClock := createdAt.Add(2 * time.Minute)
	instances := learningapp.NewCurriculumInstanceService(profiles, store,
		learningapp.WithCurriculumInstanceClock(func() time.Time { return instanceClock }),
		learningapp.WithCurriculumInstanceIDGenerator(func() (learning.ID, error) {
			id := generatedIDs[0]
			generatedIDs = generatedIDs[1:]
			return id, nil
		}))
	source, err := instances.Create(ctx, goal.ID, oldProjected, learning.CurriculumSourcePack)
	if err != nil {
		t.Fatal(err)
	}
	conceptID := mustLearningID(t, "concept.backend-go.terminal")
	state, err := instances.State(ctx, source.ID, conceptID)
	if err != nil {
		t.Fatal(err)
	}
	observed := mustLearningTimestamp(t, createdAt.Add(3*time.Minute))
	mastery, _ := learning.NewMasteryScore(.91)
	state.Exposure, state.Mastery = learning.ExposureMastered, mastery
	state.FirstSeenAt, state.LastSeenAt, state.MasteredAt = &observed, &observed, &observed
	state.UpdatedAt = observed
	if err := instances.SaveState(ctx, state); err != nil {
		t.Fatal(err)
	}
	instanceClock = createdAt.Add(4 * time.Minute)
	preserve := make([]learning.ID, 0)
	for _, action := range plan.Actions {
		if action.Kind == curriculum.MigrationPreserveState {
			preserve = append(preserve, mustLearningID(t, action.ToConceptIDs[0].String()))
		}
	}
	sort.Slice(preserve, func(i, j int) bool { return preserve[i].String() < preserve[j].String() })
	impact, err := instances.Migrate(ctx, learningapp.CurriculumInstanceMigrationRequest{
		PlanID: plan.ID.String(), BackupID: "backup.e2e", OldCurriculum: oldProjected.Reference,
		NewCurriculum: newProjected, PreserveConceptIDs: preserve,
	})
	if err != nil {
		t.Fatal(err)
	}
	listed, err := instances.List(ctx)
	if err != nil {
		t.Fatal(err)
	}
	var target learning.CurriculumInstance
	for _, instance := range listed {
		if instance.Curriculum == newProjected.Reference {
			target = instance
		}
	}
	preserved, err := instances.State(ctx, target.ID, conceptID)
	if err != nil || preserved.Mastery.Value() != .91 || preserved.Exposure != learning.ExposureMastered ||
		impact.ConceptStatesPreserved != 1 || impact.SourceInstancesArchived != 1 || impact.TargetInstancesCreated != 1 {
		t.Fatalf("preserved migration = %+v / %+v / %v", impact, preserved, err)
	}
}

func assertCopyrightSafePack(t *testing.T, encoded []byte, claims []curriculum.CurriculumEvidenceClaim) {
	t.Helper()
	archive, err := zip.NewReader(bytes.NewReader(encoded), int64(len(encoded)))
	if err != nil {
		t.Fatal(err)
	}
	for _, entry := range archive.File {
		reader, err := entry.Open()
		if err != nil {
			t.Fatal(err)
		}
		content, readErr := io.ReadAll(reader)
		closeErr := reader.Close()
		if readErr != nil || closeErr != nil {
			t.Fatalf("read %s: %v / %v", entry.Name, readErr, closeErr)
		}
		lower := strings.ToLower(string(content))
		for _, forbidden := range []string{"raw_body", "raw_content", "full_article", "full_text", "transcript"} {
			if strings.Contains(lower, forbidden) {
				t.Fatalf("export %s retained forbidden source field %q", entry.Name, forbidden)
			}
		}
		if entry.Name == "sources/evidence-report.json" || entry.Name == learningpack.EvidenceMarkdownName {
			for _, claim := range claims {
				if strings.Contains(string(content), claim.Statement) {
					t.Fatalf("evidence export %s retained claim statement %q", entry.Name, claim.ID)
				}
			}
		}
	}
}

func curriculumConcept(t *testing.T, definition curriculum.CurriculumDefinition, rawID string) curriculum.Concept {
	t.Helper()
	for _, concept := range definition.Concepts {
		if concept.ID.String() == rawID {
			return concept
		}
	}
	t.Fatalf("concept %s not found", rawID)
	return curriculum.Concept{}
}

func mustCurriculumID(t *testing.T, value string) curriculum.ID {
	t.Helper()
	id, err := curriculum.NewID(value)
	if err != nil {
		t.Fatal(err)
	}
	return id
}

func mustCurriculumConceptID(t *testing.T, value string) curriculum.ConceptID {
	t.Helper()
	id, err := curriculum.NewConceptID(value)
	if err != nil {
		t.Fatal(err)
	}
	return id
}

func mustCurriculumVersion(t *testing.T, value string) curriculum.CurriculumVersion {
	t.Helper()
	version, err := curriculum.NewCurriculumVersion(value)
	if err != nil {
		t.Fatal(err)
	}
	return version
}

func mustPackVersion(t *testing.T, value string) curriculum.PackVersion {
	t.Helper()
	version, err := curriculum.NewPackVersion(value)
	if err != nil {
		t.Fatal(err)
	}
	return version
}

func mustCurriculumTimestamp(t *testing.T, value time.Time) curriculum.Timestamp {
	t.Helper()
	timestamp, err := curriculum.NewTimestamp(value)
	if err != nil {
		t.Fatal(err)
	}
	return timestamp
}

func mustResearchID(t *testing.T, value string) research.ID {
	t.Helper()
	id, err := research.NewID(value)
	if err != nil {
		t.Fatal(err)
	}
	return id
}

func mustResearchClaimID(t *testing.T, value string) research.ClaimID {
	t.Helper()
	id, err := research.NewClaimID(value)
	if err != nil {
		t.Fatal(err)
	}
	return id
}

func mustResearchSourceID(t *testing.T, value string) research.SourceID {
	t.Helper()
	id, err := research.NewSourceID(value)
	if err != nil {
		t.Fatal(err)
	}
	return id
}

func mustResearchTimestamp(t *testing.T, value time.Time) research.Timestamp {
	t.Helper()
	timestamp, err := research.NewTimestamp(value)
	if err != nil {
		t.Fatal(err)
	}
	return timestamp
}

func mustLearningID(t *testing.T, value string) learning.ID {
	t.Helper()
	id, err := learning.NewID(value)
	if err != nil {
		t.Fatal(err)
	}
	return id
}

func mustLearningTimestamp(t *testing.T, value time.Time) learning.Timestamp {
	t.Helper()
	timestamp, err := learning.NewTimestamp(value)
	if err != nil {
		t.Fatal(err)
	}
	return timestamp
}
