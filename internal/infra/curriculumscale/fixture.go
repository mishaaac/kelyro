// Package curriculumscale provides the deterministic, offline scale fixture
// used to guard I-04 algorithms against accidental superlinear behavior.
package curriculumscale

import (
	"context"
	"crypto/sha256"
	"fmt"
	"time"

	"github.com/mishaaac/kelyro/internal/curriculum"
	curriculumapp "github.com/mishaaac/kelyro/internal/curriculum/application"
)

const (
	ConceptCount      = 10_000
	PrerequisiteCount = 20_000
	CompetencyCount   = 1_000
	SourceBundleCount = 100
)

type Fixture struct {
	Request curriculumapp.CurriculumCompileRequest
}

// PackBuildRequest wraps a compiled fixture in the portable Learning Pack v1
// metadata used by the serialization and installation scale checks.
func (fixture Fixture) PackBuildRequest(compilation curriculum.CompilationResult) curriculumapp.PackBuildRequest {
	citations := make([]curriculum.EvidenceReportCitation, 0, len(fixture.Request.EvidenceSets))
	for index, set := range fixture.Request.EvidenceSets {
		citations = append(citations, curriculum.EvidenceReportCitation{
			SourceID: set.SourceAuthority[0].SourceID,
			Title:    fmt.Sprintf("Scale source %03d", index),
			URL:      fmt.Sprintf("https://example.invalid/scale/source/%03d", index),
			License:  "CC-BY-4.0",
		})
	}
	return curriculumapp.PackBuildRequest{
		Manifest: curriculum.PackManifest{
			ID: mustID("pack.scale.large"), Name: "Large curriculum scale pack", Description: "Deterministic I-04 performance fixture.",
			Version: mustPackVersion("1.0.0"), SchemaVersion: curriculum.LearningPackSchemaVersionV1,
			Domain: fixture.Request.Input.Goal.Domain, Target: "compiler performance verification",
			Authors: []string{"Kelyro contributors"}, Maintainers: []string{"Kelyro contributors"}, License: "MIT",
			CreatedAt: compilation.BuildInfo.BuiltAt, MinimumKelyroVersion: mustPackVersion("0.2.0-alpha.3"),
			CurriculumEntry: "curriculum/curriculum.yaml", SourceEvidenceEntry: "sources/evidence-report.json",
			BuildInfoEntry: "build/build-info.json", Status: curriculum.ConceptPreview, CurriculumID: compilation.Curriculum.ID,
		},
		Compilation: compilation, EvidenceSets: fixture.Request.EvidenceSets, Citations: citations,
	}
}

// Build creates 10,000 concepts, 20,000 acyclic prerequisite edges, 1,000
// competencies, and 100 frozen Source Bundles. It performs no network or file
// access and imposes no product limit; the cardinalities are regression probes.
func Build(ctx context.Context) (Fixture, error) {
	builtAt := mustTimestamp(time.Date(2026, time.September, 15, 15, 0, 0, 0, time.UTC))
	evidenceSets, bundleRefs, references := buildEvidence(builtAt)
	outcome := curriculum.GoalOutcome{
		ID: mustID("outcome.scale"), Statement: "Analyze and apply the generated scale domain.",
		Category: curriculum.OutcomeApplication, Capability: curriculum.OutcomeCapabilityDebug,
		EvidenceRefs: []curriculum.EvidenceRef{references[0]},
	}
	goal := curriculum.LearningGoalSpec{
		ID: mustID("goal.scale"), Title: "Curriculum scale hardening", Description: "Exercise large deterministic learning graphs.",
		Domain: "scale testing", Outcomes: []curriculum.GoalOutcome{outcome}, Scope: []string{"scale"},
	}
	area := curriculum.CompetencyAreaSpec{
		ID: mustID("area.scale"), Name: "Scale area", Description: "Generated competencies for performance hardening.",
		Scopes: []string{"scale"}, OutcomeIDs: []curriculum.ID{outcome.ID},
		EvidenceRefs: []curriculum.EvidenceRef{references[0]},
	}
	profile := curriculum.DomainProfile{ID: mustID("profile.scale"), Version: "scale-profile-v1", Domain: goal.Domain, SupportedScopes: []string{"scale"}, Areas: []curriculum.CompetencyAreaSpec{area}}
	competencies := buildCompetencies(area, outcome, references)

	candidates, err := curriculumapp.NewConceptCandidateExtractorV1().Extract(ctx, curriculumapp.ConceptCandidateExtractionRequest{EvidenceSets: evidenceSets})
	if err != nil {
		return Fixture{}, fmt.Errorf("extract scale candidates: %w", err)
	}
	criteria := atomicCriteria()
	plans := make([]curriculumapp.AtomizationPlan, len(candidates.Candidates))
	for index, candidate := range candidates.Candidates {
		conceptIndex := parseCandidateIndex(candidate.Scope)
		conceptID := conceptID(conceptIndex)
		difficulty := curriculum.DifficultyAdvanced
		foundational := false
		if conceptIndex == 0 {
			difficulty = curriculum.DifficultyFoundational
			foundational = true
		}
		plans[index] = curriculumapp.AtomizationPlan{
			CandidateID: candidate.ID, Criteria: criteria,
			Hints: []curriculum.ConceptAtomizationHint{{
				CandidateID: candidate.ID, ConceptID: conceptID, Title: fmt.Sprintf("Scale concept %05d", conceptIndex),
				Definition: fmt.Sprintf("Scale concept %05d is one deterministic independently trackable unit.", conceptIndex),
				Version:    "concept-v1", Difficulty: difficulty, Foundational: foundational,
				ClaimRefs: append([]curriculum.EvidenceRef(nil), candidate.ClaimRefs...), Criteria: criteria,
			}},
		}
	}
	semantics := buildPrerequisites(references)
	curriculumID := mustCurriculumID("curriculum.scale")
	requirements, supports := buildCoverage(curriculumID, goal.ID, references)
	request := curriculumapp.CurriculumCompileRequest{
		Input:        curriculum.CompilationInput{Goal: goal, SourceBundles: bundleRefs, RequestedAt: builtAt},
		Config:       curriculum.CompilationConfig{CompilerVersion: curriculum.CurriculumCompilerVersionV1, SourcePolicy: curriculum.SourceReferencesRequired, PackSchemaVersion: curriculum.LearningPackSchemaVersionV1},
		Metadata:     curriculumapp.CurriculumBuildMetadata{ID: curriculumID, Version: mustCurriculumVersion("2026.09.15-scale.1"), Title: "Large curriculum scale fixture", Description: "Deterministic offline I-04 performance fixture.", CreatedAt: builtAt},
		EvidenceSets: evidenceSets, DomainProfile: profile, Competencies: competencies,
		AtomizationPlans: plans, PrerequisiteSemantics: semantics,
		CoverageRequirements: requirements, CoverageSupports: supports,
		AssumptionBaseline: curriculum.AssumptionBaseline{
			ID: mustID("baseline.scale"), Version: "scale-zero-v1", DomainProfileID: profile.ID, DomainProfileVersion: profile.Version,
			LearnerProfile: curriculum.LearnerProfileZero,
			Requirements: []curriculum.FoundationRequirement{{
				ID: mustID("foundation.scale.root"), ConceptID: conceptID(0), TargetCompetencyIDs: []curriculum.ID{competencies[0].ID},
				Reason: "The generated root precedes the first competency.", EvidenceRefs: []curriculum.EvidenceRef{references[0]},
			}},
		},
	}
	return Fixture{Request: request}, nil
}

func buildEvidence(builtAt curriculum.Timestamp) ([]curriculum.CurriculumEvidenceSet, []curriculum.SourceBundleRef, []curriculum.EvidenceRef) {
	sets := make([]curriculum.CurriculumEvidenceSet, SourceBundleCount)
	refs := make([]curriculum.SourceBundleRef, SourceBundleCount)
	claimRefs := make([]curriculum.EvidenceRef, ConceptCount)
	for bundleIndex := 0; bundleIndex < SourceBundleCount; bundleIndex++ {
		digest := sha256.Sum256([]byte(fmt.Sprintf("curriculum-scale-bundle-%03d", bundleIndex)))
		bundle := curriculum.SourceBundleRef{
			ID: mustID(fmt.Sprintf("bundle.scale.%03d", bundleIndex)), ContentHash: fmt.Sprintf("sha256:%x", digest),
			AlgorithmVersion: "source-bundle-scale-v1", VerifiedAt: builtAt,
		}
		sourceID := mustID(fmt.Sprintf("source.scale.%03d", bundleIndex))
		set := curriculum.CurriculumEvidenceSet{
			Bundle: bundle, Eligibility: curriculum.EvidenceReadyForCompile,
			SourceAuthority:  []curriculum.EvidenceSourceAuthority{{SourceID: sourceID, Role: "primary", TemporalScope: "current"}},
			Freshness:        curriculum.EvidenceFreshness{State: "fresh", Score: 1, LastVerifiedAt: &builtAt, Algorithm: "scale-freshness-v1"},
			AlgorithmVersion: curriculum.EvidenceIngestionAlgorithmV1,
		}
		first := bundleIndex * (ConceptCount / SourceBundleCount)
		last := first + ConceptCount/SourceBundleCount
		for conceptIndex := first; conceptIndex < last; conceptIndex++ {
			claimID := mustID(fmt.Sprintf("claim.scale.%05d", conceptIndex))
			set.Claims = append(set.Claims, curriculum.CurriculumEvidenceClaim{
				ID: claimID, Statement: fmt.Sprintf("Scale concept %05d is independently trackable.", conceptIndex),
				Kind: curriculum.EvidenceClaimDefinition, Scope: fmt.Sprintf("scale concept %05d", conceptIndex),
				Status: curriculum.ConceptCurrent, Confidence: 1, SourceIDs: []curriculum.ID{sourceID},
			})
			claimRefs[conceptIndex] = curriculum.EvidenceRef{BundleID: bundle.ID, ClaimID: claimID}
		}
		sets[bundleIndex], refs[bundleIndex] = set, bundle
	}
	return sets, refs, claimRefs
}

func buildCompetencies(area curriculum.CompetencyAreaSpec, outcome curriculum.GoalOutcome, references []curriculum.EvidenceRef) []curriculum.Competency {
	result := make([]curriculum.Competency, CompetencyCount)
	conceptsPerCompetency := ConceptCount / CompetencyCount
	for index := range result {
		conceptRefs := make([]curriculum.ConceptID, conceptsPerCompetency)
		for offset := range conceptRefs {
			conceptRefs[offset] = conceptID(index*conceptsPerCompetency + offset)
		}
		result[index] = curriculum.Competency{
			ID: mustID(fmt.Sprintf("competency.scale.%04d", index)), AreaID: area.ID, Area: area.Name, OutcomeID: outcome.ID,
			ExpectedLevel: curriculum.CompetencyAnalyze, EvidenceRefs: []curriculum.EvidenceRef{references[index*conceptsPerCompetency]}, ConceptRefs: conceptRefs,
		}
	}
	return result
}

func buildPrerequisites(references []curriculum.EvidenceRef) []curriculum.ConceptPrerequisiteSemantic {
	result := make([]curriculum.ConceptPrerequisiteSemantic, 0, PrerequisiteCount)
	add := func(concept, required int) {
		result = append(result, curriculum.ConceptPrerequisiteSemantic{
			ConceptID: conceptID(concept), RequiredConceptID: conceptID(required), Kind: curriculum.PrerequisiteHard,
			EvidenceRefs: []curriculum.EvidenceRef{references[concept]}, Reason: "Generated acyclic scale dependency.",
		})
	}
	for index := 1; index < ConceptCount; index++ {
		add(index, index-1)
	}
	for index := 2; index < ConceptCount; index++ {
		add(index, index-2)
	}
	add(ConceptCount-1, 0)
	add(ConceptCount-2, 0)
	add(ConceptCount-3, 0)
	if len(result) != PrerequisiteCount {
		panic(fmt.Sprintf("scale prerequisite count = %d", len(result)))
	}
	return result
}

func buildCoverage(curriculumID curriculum.CurriculumID, goalID curriculum.ID, references []curriculum.EvidenceRef) ([]curriculum.CoverageRequirement, []curriculum.CoverageSupport) {
	conceptIDs := make([]curriculum.ConceptID, ConceptCount)
	for index := range conceptIDs {
		conceptIDs[index] = conceptID(index)
	}
	requirements := make([]curriculum.CoverageRequirement, 0, len(curriculum.AllCoverageDimensions()))
	supports := make([]curriculum.CoverageSupport, 0, len(curriculum.AllCoverageDimensions()))
	for _, dimension := range curriculum.AllCoverageDimensions() {
		targetID, targetKind := goalID, curriculum.CoverageTargetGoal
		if dimension == curriculum.CoverageProduction || dimension == curriculum.CoverageSecurity || dimension == curriculum.CoverageToolchain {
			targetID, targetKind = mustID(curriculumID.String()), curriculum.CoverageTargetCurriculum
		}
		requirementID := mustID("coverage.scale." + string(dimension))
		requirements = append(requirements, curriculum.CoverageRequirement{ID: requirementID, Dimension: dimension, TargetKind: targetKind, TargetID: targetID, Description: "Generated scale coverage.", EvidenceRefs: []curriculum.EvidenceRef{references[0]}})
		supports = append(supports, curriculum.CoverageSupport{ID: mustID("support.scale." + string(dimension)), RequirementID: requirementID, ConceptIDs: append([]curriculum.ConceptID(nil), conceptIDs...), EvidenceRefs: []curriculum.EvidenceRef{references[0]}, Reason: "The scale graph exercises the dimension."})
	}
	return requirements, supports
}

func parseCandidateIndex(scope string) int {
	var index int
	if _, err := fmt.Sscanf(scope, "scale concept %05d", &index); err != nil {
		panic(err)
	}
	return index
}

func atomicCriteria() curriculum.AtomicConceptCriteria {
	return curriculum.AtomicConceptCriteria{
		Named: curriculum.AtomicityCriterionSatisfied, Defined: curriculum.AtomicityCriterionSatisfied,
		PrerequisiteBoundary: curriculum.AtomicityCriterionSatisfied, Explainable: curriculum.AtomicityCriterionSatisfied,
		Practicable: curriculum.AtomicityCriterionSatisfied, Assessable: curriculum.AtomicityCriterionSatisfied,
		Evidenced: curriculum.AtomicityCriterionSatisfied, MeaningfulStandalone: curriculum.AtomicityCriterionSatisfied,
	}
}

func conceptID(index int) curriculum.ConceptID {
	return mustConceptID(fmt.Sprintf("concept.scale.%05d", index))
}

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

func mustCurriculumVersion(value string) curriculum.CurriculumVersion {
	version, err := curriculum.NewCurriculumVersion(value)
	if err != nil {
		panic(err)
	}
	return version
}

func mustPackVersion(value string) curriculum.PackVersion {
	version, err := curriculum.NewPackVersion(value)
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
