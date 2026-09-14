package application

import (
	"context"
	"fmt"

	"github.com/mishaaac/kelyro/internal/curriculum"
	"github.com/mishaaac/kelyro/internal/research"
)

type CurriculumCompilerService interface {
	Compile(context.Context, CurriculumCompileRequest) (curriculum.CompilationResult, error)
}

type CurriculumBuildMetadata struct {
	ID          curriculum.CurriculumID
	Version     curriculum.CurriculumVersion
	Title       string
	Description string
	CreatedAt   curriculum.Timestamp
}

type AtomizationPlan struct {
	CandidateID curriculum.ID
	Criteria    curriculum.AtomicConceptCriteria
	Hints       []curriculum.ConceptAtomizationHint
}

// CurriculumCompileRequest contains only frozen evidence and pack-authored
// policy inputs. The compiler has no discovery/fetch port and performs no live
// research.
type CurriculumCompileRequest struct {
	Input                 curriculum.CompilationInput
	Config                curriculum.CompilationConfig
	Metadata              CurriculumBuildMetadata
	EvidenceSets          []curriculum.CurriculumEvidenceSet
	DomainProfile         curriculum.DomainProfile
	Competencies          []curriculum.Competency
	AtomizationPlans      []AtomizationPlan
	PrerequisiteSemantics []curriculum.ConceptPrerequisiteSemantic
	AvailableConcepts     []curriculum.Concept
	VocabularyDefinitions []curriculum.VocabularyDefinition
	VocabularyUses        []curriculum.VocabularyUse
	VocabularyBaseline    []curriculum.DomainVocabularyBaselineTerm
	CoverageRequirements  []curriculum.CoverageRequirement
	CoverageSupports      []curriculum.CoverageSupport
	AssumptionBaseline    curriculum.AssumptionBaseline
	PracticeContext       []curriculum.PracticeContextAssignment
	ContextualConceptIDs  []curriculum.ConceptID
	BeginnerTools         []curriculum.ToolRequirement
	BeginnerToolUses      []curriculum.BeginnerToolUse
	BeginnerAssumptions   []curriculum.BeginnerAssumption
}

type EvidenceIngestionRequest struct {
	BundleID         research.ID
	CriticalClaimIDs []research.ClaimID
}

type EvidenceIngestionResult struct {
	Evidence curriculum.CurriculumEvidenceSet
	Accepted bool
	Reasons  []string
}

type SourceBundleIngestionService interface {
	Ingest(context.Context, EvidenceIngestionRequest) (EvidenceIngestionResult, error)
}

type GoalDecompositionRequest struct {
	Goal         curriculum.LearningGoalSpec
	EvidenceSets []curriculum.CurriculumEvidenceSet
	Profile      curriculum.DomainProfile
}

type GoalDecomposerService interface {
	Decompose(context.Context, GoalDecompositionRequest) (curriculum.GoalDecomposition, error)
}

type CompetencyMatrixRequest struct {
	Decomposition curriculum.GoalDecomposition
	Competencies  []curriculum.Competency
	EvidenceSets  []curriculum.CurriculumEvidenceSet
}

type CompetencyMatrixBuilderService interface {
	Build(context.Context, CompetencyMatrixRequest) (curriculum.CompetencyMatrix, error)
}

type ConceptCandidateExtractionRequest struct {
	EvidenceSets []curriculum.CurriculumEvidenceSet
}

type ConceptCandidateExtractorService interface {
	Extract(context.Context, ConceptCandidateExtractionRequest) (curriculum.ConceptCandidateSet, error)
}

type AtomicityPolicy interface {
	Version() string
	Assess(curriculum.AtomicConceptCriteria) (curriculum.AtomicityAssessment, error)
}

type ConceptAtomizationRequest struct {
	Candidate         curriculum.ConceptCandidate
	EvidenceSets      []curriculum.CurriculumEvidenceSet
	CandidateCriteria curriculum.AtomicConceptCriteria
	DomainHints       []curriculum.ConceptAtomizationHint
}

type ConceptAtomizerService interface {
	Atomize(context.Context, ConceptAtomizationRequest) (curriculum.AtomicConceptSet, error)
}

type GranularityReviewRequest struct {
	Concepts       []curriculum.Concept
	MergeProposals []curriculum.ConceptMergeProposal
	VisualGroups   []curriculum.VisualConceptGroup
}

type GranularityGuardService interface {
	Review(context.Context, GranularityReviewRequest) (curriculum.GranularityResult, error)
}

type PrerequisiteExtractionRequest struct {
	Concepts     []curriculum.Concept
	EvidenceSets []curriculum.CurriculumEvidenceSet
	Semantics    []curriculum.ConceptPrerequisiteSemantic
}

type PrerequisiteExtractorService interface {
	Extract(context.Context, PrerequisiteExtractionRequest) (curriculum.PrerequisiteExtraction, error)
}

type PrerequisiteExpansionRequest struct {
	Concepts          []curriculum.Concept
	Prerequisites     []curriculum.Prerequisite
	Semantics         []curriculum.ConceptPrerequisiteSemantic
	AvailableConcepts []curriculum.Concept
	EvidenceSets      []curriculum.CurriculumEvidenceSet
}

type PrerequisiteExpansionService interface {
	Expand(context.Context, PrerequisiteExpansionRequest) (curriculum.PrerequisiteExpansionResult, error)
}

type KnowledgeGraphCompilationRequest struct {
	Concepts      []curriculum.Concept
	Prerequisites []curriculum.Prerequisite
}

type KnowledgeGraphCompilerService interface {
	Compile(context.Context, KnowledgeGraphCompilationRequest) (curriculum.KnowledgeGraphCompilation, error)
}

type VocabularyGraphRequest struct {
	Concepts       []curriculum.Concept
	Definitions    []curriculum.VocabularyDefinition
	Uses           []curriculum.VocabularyUse
	DomainBaseline []curriculum.DomainVocabularyBaselineTerm
}

type VocabularyGraphBuilderService interface {
	Build(context.Context, VocabularyGraphRequest) (curriculum.VocabularyGraphCompilation, error)
}

type CurriculumHierarchyBuildRequest struct {
	Competencies    curriculum.CompetencyMatrix
	Concepts        []curriculum.Concept
	Graph           curriculum.KnowledgeGraphCompilation
	PracticeContext []curriculum.PracticeContextAssignment
}

type CurriculumHierarchyBuilderService interface {
	Build(context.Context, CurriculumHierarchyBuildRequest) (curriculum.CurriculumHierarchy, error)
}

type BeginnerSimulationRequest struct {
	Concepts    []curriculum.Concept
	Graph       curriculum.KnowledgeGraphCompilation
	Hierarchy   curriculum.CurriculumHierarchy
	Vocabulary  curriculum.VocabularyGraphCompilation
	Tools       []curriculum.ToolRequirement
	ToolUses    []curriculum.BeginnerToolUse
	Assumptions []curriculum.BeginnerAssumption
}

type BeginnerSimulationService interface {
	Simulate(context.Context, BeginnerSimulationRequest) (curriculum.BeginnerSimulationResult, error)
}

type ExpertCoverageReviewRequest struct {
	Goal         curriculum.LearningGoalSpec
	Competencies curriculum.CompetencyMatrix
	Concepts     []curriculum.Concept
	Coverage     curriculum.CoverageReport
}

type ExpertCoverageReviewService interface {
	Review(context.Context, ExpertCoverageReviewRequest) (curriculum.ExpertCoverageReviewResult, error)
}

// ExpertCoverageAdvisor is an optional, non-authoritative future AI or human
// adapter. It may add notes but cannot change deterministic findings.
type ExpertCoverageAdvisor interface {
	Advise(context.Context, ExpertCoverageReviewRequest, curriculum.ExpertCoverageReviewResult) ([]string, error)
}

type DefinitionBeforeUseAuditRequest struct {
	Graph      curriculum.KnowledgeGraphCompilation
	Vocabulary curriculum.VocabularyGraphCompilation
	Topics     []curriculum.TopicSpec
}

type DefinitionBeforeUseAuditService interface {
	Audit(context.Context, DefinitionBeforeUseAuditRequest) (curriculum.DefinitionBeforeUseAuditResult, error)
}

type ZeroAssumptionAuditRequest struct {
	DomainProfile curriculum.DomainProfile
	Baseline      curriculum.AssumptionBaseline
	Competencies  curriculum.CompetencyMatrix
	Concepts      []curriculum.Concept
	Graph         curriculum.KnowledgeGraphCompilation
	EvidenceSets  []curriculum.CurriculumEvidenceSet
}

type ZeroAssumptionAuditService interface {
	Audit(context.Context, ZeroAssumptionAuditRequest) (curriculum.ZeroAssumptionAuditResult, error)
}

type FirstPrinciplesExpansionRequest struct {
	AuditResult   curriculum.ZeroAssumptionAuditResult
	Concepts      []curriculum.Concept
	Prerequisites []curriculum.Prerequisite
	Candidates    []curriculum.FirstPrinciplesCandidate
	EvidenceSets  []curriculum.CurriculumEvidenceSet
}

type FirstPrinciplesExpansionService interface {
	Expand(context.Context, FirstPrinciplesExpansionRequest) (curriculum.FirstPrinciplesExpansionResult, error)
}

type PackService interface {
	Get(context.Context, curriculum.ID, curriculum.PackVersion) (curriculum.LearningPack, error)
	List(context.Context) ([]curriculum.LearningPack, error)
	Active(context.Context) (curriculum.LearningPack, error)
}

type PackDependencyResolutionRequest struct {
	Root      curriculum.PackManifest
	Available []curriculum.PackManifest
}

type PackDependencyResolverService interface {
	Resolve(context.Context, PackDependencyResolutionRequest) (curriculum.PackDependencyResolution, error)
}

type PackValidationIssue struct {
	Code    string
	Path    string
	Message string
}

type PackValidationResult struct {
	Pack            *curriculum.LearningPack
	ContentHash     string
	PortableArchive []byte
	Warnings        []PackValidationIssue
	Errors          []PackValidationIssue
}

type PackValidationService interface {
	Validate(context.Context, PackSource) (PackValidationResult, error)
}

type PackInstallRequest struct {
	Source PackSource
}

type InstalledPack struct {
	Pack        curriculum.LearningPack
	ContentHash string
	InstalledAt curriculum.Timestamp
}

func (installed InstalledPack) Validate() error {
	if err := installed.Pack.Validate(); err != nil {
		return err
	}
	if !canonicalSHA256(installed.ContentHash) {
		return fmt.Errorf("installed pack content hash is not canonical SHA-256")
	}
	if err := installed.InstalledAt.Validate(); err != nil {
		return fmt.Errorf("installed pack time: %w", err)
	}
	return nil
}

type PackInstallationArtifact struct {
	InstalledPack
	PortableArchive []byte
}

func (artifact PackInstallationArtifact) Validate() error {
	if err := artifact.InstalledPack.Validate(); err != nil {
		return err
	}
	if len(artifact.PortableArchive) == 0 {
		return fmt.Errorf("installed pack archive is empty")
	}
	return nil
}

type PackInstallResult struct {
	Pack        curriculum.LearningPack
	ContentHash string
	Installed   bool
}

type PackActivateRequest struct {
	WorkspaceRoot string
	PackID        curriculum.ID
	Version       curriculum.PackVersion
}

type PackInstallService interface {
	Install(context.Context, PackInstallRequest) (PackInstallResult, error)
	Activate(context.Context, PackActivateRequest) (PackActivation, error)
	List(context.Context) ([]InstalledPack, error)
	Find(context.Context, curriculum.ID) ([]InstalledPack, error)
	Active(context.Context, string) (InstalledPack, error)
}

type PackCatalogView struct {
	Snapshot         curriculum.PackCatalogSnapshot
	Offline          bool
	SourceWarning    string
	AlgorithmVersion string
}

type PackCatalogService interface {
	Catalog(context.Context) (PackCatalogView, error)
	Search(context.Context, string) (PackCatalogView, error)
}

type PackUpgradeRequest struct {
	PackID        curriculum.ID
	TargetVersion curriculum.PackVersion
	DryRun        bool
}

type PackUpgradeResult struct {
	Current   curriculum.LearningPack
	Candidate curriculum.LearningPack
	Changes   []curriculum.CurriculumChange
	Applied   bool
}

type PackUpgradeService interface {
	Upgrade(context.Context, PackUpgradeRequest) (PackUpgradeResult, error)
}

type PackVersioningRequest struct {
	CurrentVersion   curriculum.PackVersion
	CandidateVersion curriculum.PackVersion
	Changes          []curriculum.CurriculumChange
}

type PackVersioningService interface {
	Classify(context.Context, PackVersioningRequest) (curriculum.PackVersioningDecision, error)
}

type CoverageAnalysisRequest struct {
	CurriculumID curriculum.CurriculumID
	Goal         curriculum.LearningGoalSpec
	Competencies curriculum.CompetencyMatrix
	Concepts     []curriculum.Concept
	Requirements []curriculum.CoverageRequirement
	Supports     []curriculum.CoverageSupport
	EvidenceSets []curriculum.CurriculumEvidenceSet
}

type CoverageService interface {
	Analyze(context.Context, CoverageAnalysisRequest) (curriculum.CoverageReport, error)
}

type TheoryCoverageRequest struct {
	Competencies           curriculum.CompetencyMatrix
	Concepts               []curriculum.Concept
	ImportantCompetencyIDs []curriculum.ID
	Contracts              []curriculum.TheoryContract
	Supports               []curriculum.TheoryFacetSupport
	EvidenceSets           []curriculum.CurriculumEvidenceSet
}

type TheoryCoverageService interface {
	Analyze(context.Context, TheoryCoverageRequest) (curriculum.TheoryCoverageReport, error)
}

type PracticeCoverageRequest struct {
	Competencies        curriculum.CompetencyMatrix
	Concepts            []curriculum.Concept
	ImportantConceptIDs []curriculum.ConceptID
	Requirements        []curriculum.PracticeRequirement
	Expectations        []curriculum.PracticeExpectation
	EvidenceSets        []curriculum.CurriculumEvidenceSet
}

type PracticeCoverageService interface {
	Analyze(context.Context, PracticeCoverageRequest) (curriculum.PracticeCoverageReport, error)
}

type ProductionCoverageRequest struct {
	CurriculumID curriculum.CurriculumID
	Goal         curriculum.LearningGoalSpec
	Competencies curriculum.CompetencyMatrix
	Concepts     []curriculum.Concept
	Requirements []curriculum.ProductionRequirement
	Supports     []curriculum.ProductionSupport
	EvidenceSets []curriculum.CurriculumEvidenceSet
}

type ProductionCoverageService interface {
	Analyze(context.Context, ProductionCoverageRequest) (curriculum.ProductionCoverageReport, error)
}

type ToolchainCoverageRequest struct {
	GoalID           curriculum.ID
	Concepts         []curriculum.Concept
	Requirements     []curriculum.ToolchainCoverageRequirement
	EnvironmentPacks []curriculum.EnvironmentPack
	EvidenceSets     []curriculum.CurriculumEvidenceSet
}

type ToolchainCoverageService interface {
	Analyze(context.Context, ToolchainCoverageRequest) (curriculum.ToolchainCoverageReport, error)
}

type EnvironmentDoctorPlanRequest struct {
	Environment      curriculum.EnvironmentPack
	Concepts         []curriculum.Concept
	Graph            curriculum.KnowledgeGraphCompilation
	Hierarchy        curriculum.CurriculumHierarchy
	CurrentConceptID curriculum.ConceptID
	Platform         string
}

type EnvironmentDoctorPlanService interface {
	Plan(context.Context, EnvironmentDoctorPlanRequest) (curriculum.EnvironmentDoctorPlan, error)
}

type SecurityCoverageRequest struct {
	CurriculumID curriculum.CurriculumID
	Goal         curriculum.LearningGoalSpec
	Competencies curriculum.CompetencyMatrix
	Concepts     []curriculum.Concept
	Requirements []curriculum.SecurityRequirement
	Supports     []curriculum.SecuritySupport
	EvidenceSets []curriculum.CurriculumEvidenceSet
}

type SecurityCoverageService interface {
	Analyze(context.Context, SecurityCoverageRequest) (curriculum.SecurityCoverageReport, error)
}

type TemporalClassificationRequest struct {
	Concepts             []curriculum.Concept
	ContextualConceptIDs []curriculum.ConceptID
	Lessons              []curriculum.LessonTemporalInput
	EvidenceSets         []curriculum.CurriculumEvidenceSet
}

type TemporalClassificationService interface {
	Classify(context.Context, TemporalClassificationRequest) (curriculum.TemporalClassificationResult, error)
}

type GuidanceClassificationRequest struct {
	TemporalResult curriculum.TemporalClassificationResult
	EvidenceSets   []curriculum.CurriculumEvidenceSet
}

type GuidanceClassificationService interface {
	Classify(context.Context, GuidanceClassificationRequest) (curriculum.GuidanceClassificationResult, error)
}

type GapScanRequest struct {
	GoalID                  curriculum.ID
	Coverage                curriculum.CoverageReport
	Requirements            []curriculum.CoverageRequirement
	PrerequisiteGaps        []curriculum.PrerequisiteExpansionGap
	CurrentGuidanceFindings []curriculum.CurrentGuidanceFinding
}

type GapScannerService interface {
	Scan(context.Context, GapScanRequest) (curriculum.GapScanReport, error)
}

type CurriculumAuditResult struct {
	Name    string
	Version string
	Passed  bool
	Reasons []string
}

type CurriculumAuditService interface {
	Audit(context.Context, curriculum.CurriculumDefinition) ([]CurriculumAuditResult, error)
}

type CurriculumReviewRequest struct {
	Compilation  curriculum.CompilationResult
	EvidenceSets []curriculum.CurriculumEvidenceSet
}

type CurriculumReviewerService interface {
	Review(context.Context, CurriculumReviewRequest) (curriculum.CurriculumReviewResult, error)
}

// CurriculumReviewAdvisor is an optional, non-authoritative hook for future AI
// or human-assistance adapters. Its notes cannot change the core decision.
type CurriculumReviewAdvisor interface {
	Advise(context.Context, CurriculumReviewRequest, curriculum.CurriculumReviewResult) ([]string, error)
}
