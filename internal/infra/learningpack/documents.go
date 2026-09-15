package learningpack

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"strings"
	"time"

	"github.com/mishaaac/kelyro/internal/curriculum"
	"go.yaml.in/yaml/v3"
)

type curriculumDocument struct {
	ID                   string                        `yaml:"id"`
	Version              string                        `yaml:"version"`
	Title                string                        `yaml:"title"`
	Description          string                        `yaml:"description"`
	Goal                 goalDocument                  `yaml:"goal"`
	CompetencyMatrix     competencyMatrixDocument      `yaml:"competency_matrix"`
	Concepts             []conceptDocument             `yaml:"concepts"`
	Prerequisites        []prerequisiteDocument        `yaml:"prerequisites,omitempty"`
	Vocabulary           []vocabularyDocument          `yaml:"vocabulary,omitempty"`
	Phases               []phaseDocument               `yaml:"phases"`
	Modules              []moduleDocument              `yaml:"modules"`
	Lessons              []lessonDocument              `yaml:"lessons"`
	Topics               []topicDocument               `yaml:"topics"`
	CoverageRequirements []coverageRequirementDocument `yaml:"coverage_requirements,omitempty"`
	SourcePolicy         string                        `yaml:"source_policy"`
	SourceBundles        []bundleRefDocument           `yaml:"source_bundles,omitempty"`
	CreatedAt            string                        `yaml:"created_at"`
}

type evidenceRefDocument struct {
	BundleID string `yaml:"bundle_id" json:"bundle_id"`
	ClaimID  string `yaml:"claim_id" json:"claim_id"`
}

type bundleRefDocument struct {
	ID               string `yaml:"id" json:"id"`
	ContentHash      string `yaml:"content_hash" json:"content_hash"`
	AlgorithmVersion string `yaml:"algorithm_version" json:"algorithm_version"`
	VerifiedAt       string `yaml:"verified_at" json:"verified_at"`
}

type goalDocument struct {
	ID          string            `yaml:"id"`
	Title       string            `yaml:"title"`
	Description string            `yaml:"description"`
	Domain      string            `yaml:"domain"`
	Role        *roleDocument     `yaml:"role,omitempty"`
	Outcomes    []outcomeDocument `yaml:"outcomes"`
	Scope       []string          `yaml:"scope,omitempty"`
	Exclusions  []string          `yaml:"exclusions,omitempty"`
}

type roleDocument struct {
	ID          string `yaml:"id"`
	Name        string `yaml:"name"`
	Description string `yaml:"description"`
}

type outcomeDocument struct {
	ID           string                `yaml:"id"`
	Statement    string                `yaml:"statement"`
	Category     string                `yaml:"category"`
	Capability   string                `yaml:"capability,omitempty"`
	EvidenceRefs []evidenceRefDocument `yaml:"evidence_refs,omitempty"`
}

type competencyMatrixDocument struct {
	Version      string               `yaml:"version"`
	GoalID       string               `yaml:"goal_id"`
	Competencies []competencyDocument `yaml:"competencies"`
}

type competencyDocument struct {
	ID            string                `yaml:"id"`
	AreaID        string                `yaml:"area_id"`
	Area          string                `yaml:"area"`
	OutcomeID     string                `yaml:"outcome_id"`
	ExpectedLevel string                `yaml:"expected_level"`
	Dimensions    []dimensionDocument   `yaml:"dimensions,omitempty"`
	EvidenceRefs  []evidenceRefDocument `yaml:"evidence_refs,omitempty"`
	ConceptRefs   []string              `yaml:"concept_refs,omitempty"`
	ParentID      string                `yaml:"parent_id,omitempty"`
}

type dimensionDocument struct {
	ID            string `yaml:"id"`
	ExpectedLevel string `yaml:"expected_level"`
}

type conceptDocument struct {
	ID           string                `yaml:"id"`
	Title        string                `yaml:"title"`
	Definition   string                `yaml:"definition"`
	Version      string                `yaml:"version"`
	Atomicity    string                `yaml:"atomicity"`
	Difficulty   int                   `yaml:"difficulty"`
	Status       string                `yaml:"status"`
	Foundational bool                  `yaml:"foundational"`
	EvidenceRefs []evidenceRefDocument `yaml:"evidence_refs,omitempty"`
}

type prerequisiteDocument struct {
	ConceptID         string                `yaml:"concept_id"`
	RequiredConceptID string                `yaml:"required_concept_id"`
	Kind              string                `yaml:"kind"`
	EvidenceRefs      []evidenceRefDocument `yaml:"evidence_refs,omitempty"`
}

type vocabularyDocument struct {
	Term               string   `yaml:"term"`
	CanonicalConceptID string   `yaml:"canonical_concept_id"`
	IntroducedBy       string   `yaml:"introduced_by"`
	UsedBy             []string `yaml:"used_by,omitempty"`
	Aliases            []string `yaml:"aliases,omitempty"`
	Scope              string   `yaml:"scope"`
}

type phaseDocument struct {
	ID          string `yaml:"id"`
	Title       string `yaml:"title"`
	Description string `yaml:"description"`
	Order       int    `yaml:"order"`
}
type moduleDocument struct {
	ID          string `yaml:"id"`
	PhaseID     string `yaml:"phase_id"`
	Title       string `yaml:"title"`
	Description string `yaml:"description"`
	Order       int    `yaml:"order"`
}
type lessonDocument struct {
	ID          string `yaml:"id"`
	ModuleID    string `yaml:"module_id"`
	Title       string `yaml:"title"`
	Description string `yaml:"description"`
	Order       int    `yaml:"order"`
}
type topicDocument struct {
	ID          string   `yaml:"id"`
	LessonID    string   `yaml:"lesson_id"`
	Title       string   `yaml:"title"`
	Description string   `yaml:"description"`
	Order       int      `yaml:"order"`
	ConceptIDs  []string `yaml:"concept_ids"`
}

type coverageRequirementDocument struct {
	ID           string                `yaml:"id"`
	Dimension    string                `yaml:"dimension"`
	TargetKind   string                `yaml:"target_kind"`
	TargetID     string                `yaml:"target_id"`
	Description  string                `yaml:"description"`
	EvidenceRefs []evidenceRefDocument `yaml:"evidence_refs,omitempty"`
}

type evidenceReportDocument struct {
	SchemaVersion         string                             `json:"schema_version"`
	Goal                  evidenceReportGoalDocument         `json:"goal"`
	Competencies          []evidenceReportCompetencyDocument `json:"competencies"`
	ConceptCount          int                                `json:"concept_count"`
	SourceBundleCount     int                                `json:"source_bundle_count"`
	Bundles               []bundleRefDocument                `json:"bundles"`
	Claims                []evidenceRefDocument              `json:"claims"`
	Citations             []evidenceReportCitationDocument   `json:"citations"`
	PrimarySourceCoverage evidenceReportCoverageDocument     `json:"primary_source_coverage"`
	Freshness             []evidenceReportFreshnessDocument  `json:"freshness"`
	Conflicts             []evidenceReportConflictDocument   `json:"conflicts"`
	Caveats               []evidenceReportCaveatDocument     `json:"caveats"`
	HistoricalContent     []temporalContentDocument          `json:"historical_content"`
	ExperimentalContent   []temporalContentDocument          `json:"experimental_content"`
	Gaps                  []gapDocument                      `json:"gaps"`
	CompilerVersion       string                             `json:"compiler_version"`
	PassVersions          []compilationPassDocument          `json:"pass_versions"`
}

type evidenceReportGoalDocument struct {
	ID     string `json:"id"`
	Title  string `json:"title"`
	Domain string `json:"domain"`
}

type evidenceReportCompetencyDocument struct {
	ID            string `json:"id"`
	Area          string `json:"area"`
	OutcomeID     string `json:"outcome_id"`
	ExpectedLevel string `json:"expected_level"`
	ConceptCount  int    `json:"concept_count"`
}

type evidenceReportCoverageDocument struct {
	ReferencedClaims int     `json:"referenced_claims"`
	PrimaryClaims    int     `json:"primary_claims"`
	Ratio            float64 `json:"ratio"`
	PolicyVersion    string  `json:"policy_version"`
}

type evidenceReportCitationDocument struct {
	SourceID    string `json:"source_id"`
	Title       string `json:"title"`
	URL         string `json:"url"`
	License     string `json:"license,omitempty"`
	Excerpt     string `json:"excerpt,omitempty"`
	ExcerptHash string `json:"excerpt_hash,omitempty"`
}

type evidenceReportFreshnessDocument struct {
	BundleID       string  `json:"bundle_id"`
	State          string  `json:"state"`
	Score          float64 `json:"score"`
	LastVerifiedAt string  `json:"last_verified_at,omitempty"`
	Algorithm      string  `json:"algorithm"`
}

type evidenceReportConflictDocument struct {
	BundleID         string   `json:"bundle_id"`
	ConflictID       string   `json:"conflict_id"`
	ClaimIDs         []string `json:"claim_ids"`
	Unresolved       bool     `json:"unresolved"`
	Reason           string   `json:"reason"`
	AlgorithmVersion string   `json:"algorithm_version"`
}

type evidenceReportCaveatDocument struct {
	BundleID string `json:"bundle_id"`
	Text     string `json:"text"`
}
type temporalContentDocument struct {
	ConceptID string `json:"concept_id"`
	Status    string `json:"status"`
}
type gapDocument struct {
	ID           string                `json:"id"`
	Kind         string                `json:"kind"`
	Severity     string                `json:"severity"`
	TargetID     string                `json:"target_id"`
	Reason       string                `json:"reason"`
	EvidenceRefs []evidenceRefDocument `json:"evidence_refs"`
}

type buildInfoDocument struct {
	SchemaVersion     string                    `json:"schema_version"`
	CompilerVersion   string                    `json:"compiler_version"`
	Passes            []compilationPassDocument `json:"passes"`
	SourceBundles     []bundleRefDocument       `json:"source_bundles"`
	CompilationConfig compilationConfigDocument `json:"compilation_config"`
	PackSchemaVersion string                    `json:"pack_schema_version"`
	InputHash         string                    `json:"input_hash"`
	OutputHash        string                    `json:"output_hash"`
	BuiltAt           string                    `json:"built_at"`
}

type compilationPassDocument struct {
	Name    string `json:"name"`
	Version string `json:"version"`
}

type compilationConfigDocument struct {
	CompilerVersion   string `json:"compiler_version"`
	SourcePolicy      string `json:"source_policy"`
	PackSchemaVersion string `json:"pack_schema_version"`
}

type environmentDocument struct {
	SchemaVersion   string                    `yaml:"schema_version"`
	ID              string                    `yaml:"id"`
	Version         string                    `yaml:"version"`
	PlatformSupport []string                  `yaml:"platform_support"`
	Tools           []toolDocument            `yaml:"tools"`
	InstallGuidance []installGuidanceDocument `yaml:"install_guidance"`
}

type toolDocument struct {
	ID                  string                `yaml:"id"`
	DisplayName         string                `yaml:"display_name"`
	Purpose             string                `yaml:"purpose"`
	MinimumVersion      string                `yaml:"minimum_version"`
	Level               string                `yaml:"level"`
	IntroducedAt        string                `yaml:"introduced_at"`
	WhenNeeded          string                `yaml:"when_needed"`
	Platforms           []string              `yaml:"platforms"`
	InstallGuidanceRefs []string              `yaml:"install_guidance_refs"`
	EvidenceRefs        []evidenceRefDocument `yaml:"evidence_refs"`
}

type installGuidanceDocument struct {
	ID           string                `yaml:"id"`
	Platform     string                `yaml:"platform"`
	SourceName   string                `yaml:"source_name"`
	OfficialURL  string                `yaml:"official_url"`
	Instructions string                `yaml:"instructions"`
	EvidenceRefs []evidenceRefDocument `yaml:"evidence_refs"`
}

func decodeStrictYAML[T any](name string, encoded []byte) (T, error) {
	var zero T
	decoder := yaml.NewDecoder(strings.NewReader(string(encoded)))
	decoder.KnownFields(true)
	var result T
	if err := decoder.Decode(&result); err != nil {
		return zero, fmt.Errorf("decode %s: %w", name, err)
	}
	var extra yaml.Node
	if err := decoder.Decode(&extra); !errors.Is(err, io.EOF) {
		if err != nil {
			return zero, fmt.Errorf("decode %s trailing document: %w", name, err)
		}
		return zero, fmt.Errorf("decode %s: multiple documents are not allowed", name)
	}
	return result, nil
}

func decodeCurriculum(encoded []byte) (curriculum.CurriculumDefinition, error) {
	source, err := decodeStrictYAML[curriculumDocument]("curriculum", encoded)
	if err != nil {
		return curriculum.CurriculumDefinition{}, err
	}
	id, err := curriculum.NewCurriculumID(source.ID)
	if err != nil {
		return curriculum.CurriculumDefinition{}, err
	}
	version, err := curriculum.NewCurriculumVersion(source.Version)
	if err != nil {
		return curriculum.CurriculumDefinition{}, err
	}
	createdAt, err := parseTimestamp(source.CreatedAt)
	if err != nil {
		return curriculum.CurriculumDefinition{}, fmt.Errorf("curriculum created_at: %w", err)
	}
	goal, err := decodeGoal(source.Goal)
	if err != nil {
		return curriculum.CurriculumDefinition{}, err
	}
	matrix, err := decodeMatrix(source.CompetencyMatrix)
	if err != nil {
		return curriculum.CurriculumDefinition{}, err
	}
	result := curriculum.CurriculumDefinition{ID: id, Version: version, Title: source.Title, Description: source.Description, Goal: goal, Competencies: matrix, SourcePolicy: curriculum.SourceReferencePolicy(source.SourcePolicy), CreatedAt: createdAt}
	for _, raw := range source.SourceBundles {
		value, err := decodeBundleRef(raw)
		if err != nil {
			return curriculum.CurriculumDefinition{}, err
		}
		result.SourceBundles = append(result.SourceBundles, value)
	}
	for _, raw := range source.Concepts {
		value, err := decodeConcept(raw)
		if err != nil {
			return curriculum.CurriculumDefinition{}, err
		}
		result.Concepts = append(result.Concepts, value)
	}
	for _, raw := range source.Prerequisites {
		value, err := decodePrerequisite(raw)
		if err != nil {
			return curriculum.CurriculumDefinition{}, err
		}
		result.Prerequisites = append(result.Prerequisites, value)
	}
	for _, raw := range source.Vocabulary {
		value, err := decodeVocabulary(raw)
		if err != nil {
			return curriculum.CurriculumDefinition{}, err
		}
		result.Vocabulary.Terms = append(result.Vocabulary.Terms, value)
	}
	for _, raw := range source.Phases {
		value, err := curriculum.NewID(raw.ID)
		if err != nil {
			return curriculum.CurriculumDefinition{}, err
		}
		result.Phases = append(result.Phases, curriculum.Phase{ID: value, Title: raw.Title, Description: raw.Description, Order: raw.Order})
	}
	for _, raw := range source.Modules {
		id, err := curriculum.NewID(raw.ID)
		if err != nil {
			return curriculum.CurriculumDefinition{}, err
		}
		parent, err := curriculum.NewID(raw.PhaseID)
		if err != nil {
			return curriculum.CurriculumDefinition{}, err
		}
		result.Modules = append(result.Modules, curriculum.Module{ID: id, PhaseID: parent, Title: raw.Title, Description: raw.Description, Order: raw.Order})
	}
	for _, raw := range source.Lessons {
		id, err := curriculum.NewID(raw.ID)
		if err != nil {
			return curriculum.CurriculumDefinition{}, err
		}
		parent, err := curriculum.NewID(raw.ModuleID)
		if err != nil {
			return curriculum.CurriculumDefinition{}, err
		}
		result.Lessons = append(result.Lessons, curriculum.LessonSpec{ID: id, ModuleID: parent, Title: raw.Title, Description: raw.Description, Order: raw.Order})
	}
	for _, raw := range source.Topics {
		id, err := curriculum.NewID(raw.ID)
		if err != nil {
			return curriculum.CurriculumDefinition{}, err
		}
		parent, err := curriculum.NewID(raw.LessonID)
		if err != nil {
			return curriculum.CurriculumDefinition{}, err
		}
		concepts, err := decodeConceptIDs(raw.ConceptIDs)
		if err != nil {
			return curriculum.CurriculumDefinition{}, err
		}
		result.Topics = append(result.Topics, curriculum.TopicSpec{ID: id, LessonID: parent, Title: raw.Title, Description: raw.Description, Order: raw.Order, ConceptIDs: concepts})
	}
	for _, raw := range source.CoverageRequirements {
		value, err := decodeCoverage(raw)
		if err != nil {
			return curriculum.CurriculumDefinition{}, err
		}
		result.CoverageRequirements = append(result.CoverageRequirements, value)
	}
	if err := result.Validate(); err != nil {
		return curriculum.CurriculumDefinition{}, fmt.Errorf("curriculum: %w", err)
	}
	return result, nil
}

func decodeGoal(raw goalDocument) (curriculum.LearningGoalSpec, error) {
	id, err := curriculum.NewID(raw.ID)
	if err != nil {
		return curriculum.LearningGoalSpec{}, err
	}
	result := curriculum.LearningGoalSpec{ID: id, Title: raw.Title, Description: raw.Description, Domain: raw.Domain, Scope: append([]string(nil), raw.Scope...), Exclusions: append([]string(nil), raw.Exclusions...)}
	if raw.Role != nil {
		roleID, err := curriculum.NewID(raw.Role.ID)
		if err != nil {
			return curriculum.LearningGoalSpec{}, err
		}
		result.Role = &curriculum.ProfessionalRole{ID: roleID, Name: raw.Role.Name, Description: raw.Role.Description}
	}
	for _, item := range raw.Outcomes {
		outcomeID, err := curriculum.NewID(item.ID)
		if err != nil {
			return curriculum.LearningGoalSpec{}, err
		}
		refs, err := decodeEvidenceRefs(item.EvidenceRefs)
		if err != nil {
			return curriculum.LearningGoalSpec{}, err
		}
		result.Outcomes = append(result.Outcomes, curriculum.GoalOutcome{
			ID: outcomeID, Statement: item.Statement, Category: curriculum.OutcomeCategory(item.Category),
			Capability: curriculum.OutcomeCapability(item.Capability), EvidenceRefs: refs,
		})
	}
	return result, nil
}

func decodeMatrix(raw competencyMatrixDocument) (curriculum.CompetencyMatrix, error) {
	goalID, err := curriculum.NewID(raw.GoalID)
	if err != nil {
		return curriculum.CompetencyMatrix{}, err
	}
	result := curriculum.CompetencyMatrix{Version: raw.Version, GoalID: goalID}
	for _, item := range raw.Competencies {
		id, err := curriculum.NewID(item.ID)
		if err != nil {
			return curriculum.CompetencyMatrix{}, err
		}
		outcome, err := curriculum.NewID(item.OutcomeID)
		if err != nil {
			return curriculum.CompetencyMatrix{}, err
		}
		refs, err := decodeEvidenceRefs(item.EvidenceRefs)
		if err != nil {
			return curriculum.CompetencyMatrix{}, err
		}
		concepts, err := decodeConceptIDs(item.ConceptRefs)
		if err != nil {
			return curriculum.CompetencyMatrix{}, err
		}
		areaID, err := curriculum.NewID(item.AreaID)
		if err != nil {
			return curriculum.CompetencyMatrix{}, err
		}
		dimensions := make([]curriculum.CompetencyDimension, 0, len(item.Dimensions))
		for _, rawDimension := range item.Dimensions {
			dimensionID, err := curriculum.NewID(rawDimension.ID)
			if err != nil {
				return curriculum.CompetencyMatrix{}, err
			}
			dimensions = append(dimensions, curriculum.CompetencyDimension{ID: dimensionID, ExpectedLevel: curriculum.CompetencyLevel(rawDimension.ExpectedLevel)})
		}
		competency := curriculum.Competency{ID: id, AreaID: areaID, Area: item.Area, OutcomeID: outcome, ExpectedLevel: curriculum.CompetencyLevel(item.ExpectedLevel), Dimensions: dimensions, EvidenceRefs: refs, ConceptRefs: concepts}
		if item.ParentID != "" {
			parentID, err := curriculum.NewID(item.ParentID)
			if err != nil {
				return curriculum.CompetencyMatrix{}, err
			}
			competency.ParentID = &parentID
		}
		result.Competencies = append(result.Competencies, competency)
	}
	return result, nil
}

func decodeConcept(raw conceptDocument) (curriculum.Concept, error) {
	id, err := curriculum.NewConceptID(raw.ID)
	if err != nil {
		return curriculum.Concept{}, err
	}
	refs, err := decodeEvidenceRefs(raw.EvidenceRefs)
	if err != nil {
		return curriculum.Concept{}, err
	}
	return curriculum.Concept{ID: id, Title: raw.Title, Definition: raw.Definition, Version: raw.Version, Atomicity: curriculum.Atomicity(raw.Atomicity), Difficulty: curriculum.Difficulty(raw.Difficulty), Status: curriculum.ConceptStatus(raw.Status), Foundational: raw.Foundational, EvidenceRefs: refs}, nil
}
func decodePrerequisite(raw prerequisiteDocument) (curriculum.Prerequisite, error) {
	id, err := curriculum.NewConceptID(raw.ConceptID)
	if err != nil {
		return curriculum.Prerequisite{}, err
	}
	required, err := curriculum.NewConceptID(raw.RequiredConceptID)
	if err != nil {
		return curriculum.Prerequisite{}, err
	}
	refs, err := decodeEvidenceRefs(raw.EvidenceRefs)
	if err != nil {
		return curriculum.Prerequisite{}, err
	}
	return curriculum.Prerequisite{ConceptID: id, RequiredConceptID: required, Kind: curriculum.PrerequisiteKind(raw.Kind), EvidenceRefs: refs}, nil
}
func decodeVocabulary(raw vocabularyDocument) (curriculum.VocabularyTerm, error) {
	canonical, err := curriculum.NewConceptID(raw.CanonicalConceptID)
	if err != nil {
		return curriculum.VocabularyTerm{}, err
	}
	introduced, err := curriculum.NewConceptID(raw.IntroducedBy)
	if err != nil {
		return curriculum.VocabularyTerm{}, err
	}
	used, err := decodeConceptIDs(raw.UsedBy)
	if err != nil {
		return curriculum.VocabularyTerm{}, err
	}
	return curriculum.VocabularyTerm{Term: raw.Term, CanonicalConceptID: canonical, IntroducedBy: introduced, UsedBy: used, Aliases: append([]string(nil), raw.Aliases...), Scope: raw.Scope}, nil
}
func decodeCoverage(raw coverageRequirementDocument) (curriculum.CoverageRequirement, error) {
	id, err := curriculum.NewID(raw.ID)
	if err != nil {
		return curriculum.CoverageRequirement{}, err
	}
	target, err := curriculum.NewID(raw.TargetID)
	if err != nil {
		return curriculum.CoverageRequirement{}, err
	}
	refs, err := decodeEvidenceRefs(raw.EvidenceRefs)
	if err != nil {
		return curriculum.CoverageRequirement{}, err
	}
	return curriculum.CoverageRequirement{ID: id, Dimension: curriculum.CoverageDimension(raw.Dimension), TargetKind: curriculum.CoverageTargetKind(raw.TargetKind), TargetID: target, Description: raw.Description, EvidenceRefs: refs}, nil
}

func decodeEvidenceRefs(raw []evidenceRefDocument) ([]curriculum.EvidenceRef, error) {
	result := make([]curriculum.EvidenceRef, 0, len(raw))
	for _, item := range raw {
		bundle, err := curriculum.NewID(item.BundleID)
		if err != nil {
			return nil, err
		}
		claim, err := curriculum.NewID(item.ClaimID)
		if err != nil {
			return nil, err
		}
		result = append(result, curriculum.EvidenceRef{BundleID: bundle, ClaimID: claim})
	}
	return result, nil
}
func decodeConceptIDs(raw []string) ([]curriculum.ConceptID, error) {
	result := make([]curriculum.ConceptID, 0, len(raw))
	for _, item := range raw {
		id, err := curriculum.NewConceptID(item)
		if err != nil {
			return nil, err
		}
		result = append(result, id)
	}
	return result, nil
}
func decodeBundleRef(raw bundleRefDocument) (curriculum.SourceBundleRef, error) {
	id, err := curriculum.NewID(raw.ID)
	if err != nil {
		return curriculum.SourceBundleRef{}, err
	}
	verified, err := parseTimestamp(raw.VerifiedAt)
	if err != nil {
		return curriculum.SourceBundleRef{}, err
	}
	return curriculum.SourceBundleRef{ID: id, ContentHash: raw.ContentHash, AlgorithmVersion: raw.AlgorithmVersion, VerifiedAt: verified}, nil
}
func parseTimestamp(value string) (curriculum.Timestamp, error) {
	parsed, err := time.Parse(time.RFC3339, value)
	if err != nil || !strings.HasSuffix(value, "Z") {
		return curriculum.Timestamp{}, fmt.Errorf("must be RFC3339 UTC Z")
	}
	return curriculum.NewTimestamp(parsed)
}

func decodeEvidenceReport(encoded []byte) (curriculum.CurriculumEvidenceReport, error) {
	if err := rejectDuplicateJSONKeys(encoded); err != nil {
		return curriculum.CurriculumEvidenceReport{}, fmt.Errorf("decode evidence report: %w", err)
	}
	decoder := json.NewDecoder(strings.NewReader(string(encoded)))
	decoder.DisallowUnknownFields()
	var report evidenceReportDocument
	if err := decoder.Decode(&report); err != nil {
		return curriculum.CurriculumEvidenceReport{}, fmt.Errorf("decode evidence report: %w", err)
	}
	if err := ensureJSONEOF(decoder); err != nil {
		return curriculum.CurriculumEvidenceReport{}, err
	}
	goalID, err := curriculum.NewID(report.Goal.ID)
	if err != nil {
		return curriculum.CurriculumEvidenceReport{}, err
	}
	result := curriculum.CurriculumEvidenceReport{
		SchemaVersion: report.SchemaVersion,
		Goal:          curriculum.EvidenceReportGoal{ID: goalID, Title: report.Goal.Title, Domain: report.Goal.Domain},
		ConceptCount:  report.ConceptCount, SourceBundleCount: report.SourceBundleCount,
		PrimarySourceCoverage: curriculum.EvidenceReportCoverage{
			ReferencedClaims: report.PrimarySourceCoverage.ReferencedClaims, PrimaryClaims: report.PrimarySourceCoverage.PrimaryClaims,
			Ratio: report.PrimarySourceCoverage.Ratio, PolicyVersion: report.PrimarySourceCoverage.PolicyVersion,
		},
		CompilerVersion: report.CompilerVersion,
	}
	for index, raw := range report.Bundles {
		bundle, err := decodeBundleRef(raw)
		if err != nil {
			return curriculum.CurriculumEvidenceReport{}, fmt.Errorf("evidence report bundle %d: %w", index, err)
		}
		result.Bundles = append(result.Bundles, bundle)
	}
	result.Claims, err = decodeEvidenceRefs(report.Claims)
	if err != nil {
		return curriculum.CurriculumEvidenceReport{}, err
	}
	for _, raw := range report.Citations {
		sourceID, idErr := curriculum.NewID(raw.SourceID)
		if idErr != nil {
			return curriculum.CurriculumEvidenceReport{}, idErr
		}
		result.Citations = append(result.Citations, curriculum.EvidenceReportCitation{SourceID: sourceID, Title: raw.Title, URL: raw.URL, License: raw.License, Excerpt: raw.Excerpt, ExcerptHash: raw.ExcerptHash})
	}
	for _, raw := range report.Competencies {
		id, idErr := curriculum.NewID(raw.ID)
		if idErr != nil {
			return curriculum.CurriculumEvidenceReport{}, idErr
		}
		outcome, idErr := curriculum.NewID(raw.OutcomeID)
		if idErr != nil {
			return curriculum.CurriculumEvidenceReport{}, idErr
		}
		result.Competencies = append(result.Competencies, curriculum.EvidenceReportCompetency{ID: id, Area: raw.Area, OutcomeID: outcome, ExpectedLevel: curriculum.CompetencyLevel(raw.ExpectedLevel), ConceptCount: raw.ConceptCount})
	}
	for _, raw := range report.Freshness {
		bundleID, idErr := curriculum.NewID(raw.BundleID)
		if idErr != nil {
			return curriculum.CurriculumEvidenceReport{}, idErr
		}
		freshness := curriculum.EvidenceFreshness{State: raw.State, Score: raw.Score, Algorithm: raw.Algorithm}
		if raw.LastVerifiedAt != "" {
			value, timeErr := parseTimestamp(raw.LastVerifiedAt)
			if timeErr != nil {
				return curriculum.CurriculumEvidenceReport{}, timeErr
			}
			freshness.LastVerifiedAt = &value
		}
		result.Freshness = append(result.Freshness, curriculum.EvidenceReportFreshness{BundleID: bundleID, Freshness: freshness})
	}
	for _, raw := range report.Conflicts {
		bundleID, idErr := curriculum.NewID(raw.BundleID)
		if idErr != nil {
			return curriculum.CurriculumEvidenceReport{}, idErr
		}
		conflictID, idErr := curriculum.NewID(raw.ConflictID)
		if idErr != nil {
			return curriculum.CurriculumEvidenceReport{}, idErr
		}
		claimIDs := make([]curriculum.ID, len(raw.ClaimIDs))
		for index, value := range raw.ClaimIDs {
			claimIDs[index], idErr = curriculum.NewID(value)
			if idErr != nil {
				return curriculum.CurriculumEvidenceReport{}, idErr
			}
		}
		result.Conflicts = append(result.Conflicts, curriculum.EvidenceReportConflict{BundleID: bundleID, ConflictID: conflictID, ClaimIDs: claimIDs, Unresolved: raw.Unresolved, Reason: raw.Reason, AlgorithmVersion: raw.AlgorithmVersion})
	}
	for _, raw := range report.Caveats {
		bundleID, idErr := curriculum.NewID(raw.BundleID)
		if idErr != nil {
			return curriculum.CurriculumEvidenceReport{}, idErr
		}
		result.Caveats = append(result.Caveats, curriculum.EvidenceReportCaveat{BundleID: bundleID, Text: raw.Text})
	}
	decodeTemporal := func(values []temporalContentDocument) ([]curriculum.EvidenceReportTemporalContent, error) {
		output := make([]curriculum.EvidenceReportTemporalContent, len(values))
		for index, raw := range values {
			id, idErr := curriculum.NewConceptID(raw.ConceptID)
			if idErr != nil {
				return nil, idErr
			}
			output[index] = curriculum.EvidenceReportTemporalContent{ConceptID: id, Status: curriculum.ConceptStatus(raw.Status)}
		}
		return output, nil
	}
	result.HistoricalContent, err = decodeTemporal(report.HistoricalContent)
	if err != nil {
		return curriculum.CurriculumEvidenceReport{}, err
	}
	result.ExperimentalContent, err = decodeTemporal(report.ExperimentalContent)
	if err != nil {
		return curriculum.CurriculumEvidenceReport{}, err
	}
	for _, raw := range report.Gaps {
		id, idErr := curriculum.NewID(raw.ID)
		if idErr != nil {
			return curriculum.CurriculumEvidenceReport{}, idErr
		}
		target, idErr := curriculum.NewID(raw.TargetID)
		if idErr != nil {
			return curriculum.CurriculumEvidenceReport{}, idErr
		}
		refs, refErr := decodeEvidenceRefs(raw.EvidenceRefs)
		if refErr != nil {
			return curriculum.CurriculumEvidenceReport{}, refErr
		}
		result.Gaps = append(result.Gaps, curriculum.Gap{ID: id, Kind: curriculum.GapKind(raw.Kind), Severity: curriculum.GapSeverity(raw.Severity), TargetID: target, Reason: raw.Reason, EvidenceRefs: refs})
	}
	for _, raw := range report.PassVersions {
		result.PassVersions = append(result.PassVersions, curriculum.CompilationPassVersion{Name: raw.Name, Version: raw.Version})
	}
	if err := result.Validate(); err != nil {
		return curriculum.CurriculumEvidenceReport{}, fmt.Errorf("evidence report: %w", err)
	}
	return result, nil
}

func decodeBuildInfo(encoded []byte) (curriculum.ReproducibilityMetadata, error) {
	if err := rejectDuplicateJSONKeys(encoded); err != nil {
		return curriculum.ReproducibilityMetadata{}, fmt.Errorf("decode build info: %w", err)
	}
	decoder := json.NewDecoder(bytes.NewReader(encoded))
	decoder.DisallowUnknownFields()
	var document buildInfoDocument
	if err := decoder.Decode(&document); err != nil {
		return curriculum.ReproducibilityMetadata{}, fmt.Errorf("decode build info: %w", err)
	}
	if err := ensureJSONEOF(decoder); err != nil {
		return curriculum.ReproducibilityMetadata{}, err
	}
	builtAt, err := parseTimestamp(document.BuiltAt)
	if err != nil {
		return curriculum.ReproducibilityMetadata{}, fmt.Errorf("build info built_at: %w", err)
	}
	passes := make([]curriculum.CompilationPassVersion, len(document.Passes))
	for index, pass := range document.Passes {
		passes[index] = curriculum.CompilationPassVersion{Name: pass.Name, Version: pass.Version}
	}
	bundles := make([]curriculum.SourceBundleRef, len(document.SourceBundles))
	for index, bundle := range document.SourceBundles {
		bundles[index], err = decodeBundleRef(bundle)
		if err != nil {
			return curriculum.ReproducibilityMetadata{}, fmt.Errorf("build info source bundle %d: %w", index, err)
		}
	}
	metadata := curriculum.ReproducibilityMetadata{
		SchemaVersion: document.SchemaVersion, CompilerVersion: document.CompilerVersion,
		Passes: passes, SourceBundles: bundles,
		CompilationConfig: curriculum.CompilationConfig{
			CompilerVersion:   document.CompilationConfig.CompilerVersion,
			SourcePolicy:      curriculum.SourceReferencePolicy(document.CompilationConfig.SourcePolicy),
			PackSchemaVersion: document.CompilationConfig.PackSchemaVersion,
		},
		PackSchemaVersion: document.PackSchemaVersion, InputHash: document.InputHash,
		OutputHash: document.OutputHash, BuiltAt: builtAt,
	}
	if err := metadata.Validate(); err != nil {
		return curriculum.ReproducibilityMetadata{}, fmt.Errorf("build info: %w", err)
	}
	return metadata, nil
}

func rejectDuplicateJSONKeys(encoded []byte) error {
	decoder := json.NewDecoder(bytes.NewReader(encoded))
	var walk func() error
	walk = func() error {
		token, err := decoder.Token()
		if err != nil {
			return err
		}
		delimiter, structured := token.(json.Delim)
		if !structured {
			return nil
		}
		switch delimiter {
		case '{':
			seen := make(map[string]struct{})
			for decoder.More() {
				keyToken, err := decoder.Token()
				if err != nil {
					return err
				}
				key, ok := keyToken.(string)
				if !ok {
					return fmt.Errorf("object key is not a string")
				}
				if _, exists := seen[key]; exists {
					return fmt.Errorf("duplicate JSON key %q", key)
				}
				seen[key] = struct{}{}
				if err := walk(); err != nil {
					return err
				}
			}
			_, err = decoder.Token()
			return err
		case '[':
			for decoder.More() {
				if err := walk(); err != nil {
					return err
				}
			}
			_, err = decoder.Token()
			return err
		default:
			return fmt.Errorf("unexpected JSON delimiter %q", delimiter)
		}
	}
	return walk()
}

func ensureJSONEOF(decoder *json.Decoder) error {
	var extra any
	if err := decoder.Decode(&extra); !errors.Is(err, io.EOF) {
		if err != nil {
			return fmt.Errorf("decode trailing JSON: %w", err)
		}
		return fmt.Errorf("decode evidence report: trailing JSON value")
	}
	return nil
}

func decodeEnvironment(encoded []byte) (curriculum.EnvironmentPack, error) {
	source, err := decodeStrictYAML[environmentDocument]("environment", encoded)
	if err != nil {
		return curriculum.EnvironmentPack{}, err
	}
	id, err := curriculum.NewID(source.ID)
	if err != nil {
		return curriculum.EnvironmentPack{}, err
	}
	version, err := curriculum.NewPackVersion(source.Version)
	if err != nil {
		return curriculum.EnvironmentPack{}, err
	}
	result := curriculum.EnvironmentPack{ID: id, Version: version, SchemaVersion: source.SchemaVersion, SupportedPlatforms: append([]string(nil), source.PlatformSupport...)}
	for _, raw := range source.InstallGuidance {
		guidanceID, err := curriculum.NewID(raw.ID)
		if err != nil {
			return curriculum.EnvironmentPack{}, err
		}
		refs, err := decodeEvidenceRefs(raw.EvidenceRefs)
		if err != nil {
			return curriculum.EnvironmentPack{}, err
		}
		result.InstallGuidance = append(result.InstallGuidance, curriculum.ToolInstallGuidance{
			ID: guidanceID, Platform: raw.Platform, SourceName: raw.SourceName,
			OfficialURL: raw.OfficialURL, Instructions: raw.Instructions, EvidenceRefs: refs,
		})
	}
	for _, raw := range source.Tools {
		toolID, err := curriculum.NewID(raw.ID)
		if err != nil {
			return curriculum.EnvironmentPack{}, err
		}
		refs, err := decodeEvidenceRefs(raw.EvidenceRefs)
		if err != nil {
			return curriculum.EnvironmentPack{}, err
		}
		guidanceRefs := make([]curriculum.ID, 0, len(raw.InstallGuidanceRefs))
		for _, value := range raw.InstallGuidanceRefs {
			guidanceID, err := curriculum.NewID(value)
			if err != nil {
				return curriculum.EnvironmentPack{}, err
			}
			guidanceRefs = append(guidanceRefs, guidanceID)
		}
		tool := curriculum.ToolRequirement{
			ID: toolID, DisplayName: raw.DisplayName, Purpose: raw.Purpose,
			MinimumVersion: raw.MinimumVersion, Level: curriculum.ToolRequirementLevel(raw.Level),
			Platforms: append([]string(nil), raw.Platforms...), InstallGuidanceRefs: guidanceRefs, EvidenceRefs: refs,
		}
		if raw.IntroducedAt != "" {
			conceptID, err := curriculum.NewConceptID(raw.IntroducedAt)
			if err != nil {
				return curriculum.EnvironmentPack{}, err
			}
			tool.IntroducedAt = &conceptID
		}
		if raw.WhenNeeded != "" {
			conceptID, err := curriculum.NewConceptID(raw.WhenNeeded)
			if err != nil {
				return curriculum.EnvironmentPack{}, err
			}
			tool.WhenNeeded = &conceptID
		}
		result.Tools = append(result.Tools, tool)
	}
	if err := result.ValidatePortableV1(); err != nil {
		return curriculum.EnvironmentPack{}, fmt.Errorf("environment: %w", err)
	}
	return result, nil
}
