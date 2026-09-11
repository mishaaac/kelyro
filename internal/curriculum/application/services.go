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

type CoverageService interface {
	Analyze(context.Context, curriculum.CurriculumDefinition) ([]curriculum.CoverageResult, []curriculum.Gap, error)
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
