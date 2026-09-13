package application

import (
	"context"

	"github.com/mishaaac/kelyro/internal/curriculum"
	"github.com/mishaaac/kelyro/internal/research"
)

type CurriculumCompilerService interface {
	Compile(context.Context, curriculum.CompilationInput, curriculum.CompilationConfig) (curriculum.CompilationResult, error)
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

type PackValidationIssue struct {
	Code    string
	Path    string
	Message string
}

type PackValidationResult struct {
	Pack     *curriculum.LearningPack
	Warnings []PackValidationIssue
	Errors   []PackValidationIssue
}

type PackValidationService interface {
	Validate(context.Context, PackSource) (PackValidationResult, error)
}

type PackInstallRequest struct {
	Source PackSource
}

type PackInstallResult struct {
	Pack      curriculum.LearningPack
	Installed bool
}

type PackInstallService interface {
	Install(context.Context, PackInstallRequest) (PackInstallResult, error)
	Activate(context.Context, curriculum.ID, curriculum.PackVersion) (PackActivation, error)
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
