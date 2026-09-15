package curriculum

import (
	"errors"
	"fmt"
	"math"
)

const CurriculumEvidenceReportSchemaVersionV1 = "curriculum-evidence-report/v1"

type EvidenceReportGoal struct {
	ID     ID
	Title  string
	Domain string
}

func (goal EvidenceReportGoal) Validate() error {
	if err := goal.ID.Validate(); err != nil {
		return err
	}
	if err := requireText("evidence report goal title", goal.Title); err != nil {
		return err
	}
	return requireText("evidence report goal domain", goal.Domain)
}

type EvidenceReportCompetency struct {
	ID            ID
	Area          string
	OutcomeID     ID
	ExpectedLevel CompetencyLevel
	ConceptCount  int
}

func (competency EvidenceReportCompetency) Validate() error {
	if err := competency.ID.Validate(); err != nil {
		return err
	}
	if err := requireText("evidence report competency area", competency.Area); err != nil {
		return err
	}
	if err := competency.OutcomeID.Validate(); err != nil {
		return err
	}
	if err := competency.ExpectedLevel.Validate(); err != nil {
		return err
	}
	if competency.ConceptCount < 0 {
		return errors.New("evidence report competency concept count is negative")
	}
	return nil
}

type EvidenceReportCoverage struct {
	ReferencedClaims int
	PrimaryClaims    int
	Ratio            float64
	PolicyVersion    string
}

func (coverage EvidenceReportCoverage) Validate() error {
	if coverage.ReferencedClaims < 0 || coverage.PrimaryClaims < 0 || coverage.PrimaryClaims > coverage.ReferencedClaims {
		return errors.New("evidence report primary coverage counts are invalid")
	}
	if coverage.Ratio < 0 || coverage.Ratio > 1 {
		return errors.New("evidence report primary coverage ratio is outside 0..1")
	}
	expected := 0.0
	if coverage.ReferencedClaims > 0 {
		expected = float64(coverage.PrimaryClaims) / float64(coverage.ReferencedClaims)
	}
	if math.Abs(coverage.Ratio-expected) > 1e-12 {
		return errors.New("evidence report primary coverage ratio does not match counts")
	}
	return requireText("evidence report primary coverage policy", coverage.PolicyVersion)
}

type EvidenceReportFreshness struct {
	BundleID  ID
	Freshness EvidenceFreshness
}

func (freshness EvidenceReportFreshness) Validate() error {
	if err := freshness.BundleID.Validate(); err != nil {
		return err
	}
	return freshness.Freshness.Validate()
}

type EvidenceReportConflict struct {
	BundleID         ID
	ConflictID       ID
	ClaimIDs         []ID
	Unresolved       bool
	Reason           string
	AlgorithmVersion string
}

func (conflict EvidenceReportConflict) Validate() error {
	if err := conflict.BundleID.Validate(); err != nil {
		return err
	}
	value := CurriculumEvidenceConflict{ID: conflict.ConflictID, ClaimIDs: conflict.ClaimIDs, Unresolved: conflict.Unresolved, Reason: conflict.Reason, AlgorithmVersion: conflict.AlgorithmVersion}
	return value.Validate()
}

type EvidenceReportCaveat struct {
	BundleID ID
	Text     string
}

func (caveat EvidenceReportCaveat) Validate() error {
	if err := caveat.BundleID.Validate(); err != nil {
		return err
	}
	return requireText("evidence report caveat", caveat.Text)
}

type EvidenceReportTemporalContent struct {
	ConceptID ConceptID
	Status    ConceptStatus
}

func (content EvidenceReportTemporalContent) Validate() error {
	if err := content.ConceptID.Validate(); err != nil {
		return err
	}
	return content.Status.Validate()
}

// CurriculumEvidenceReport is a bounded, source-body-free summary of the
// evidence behind one compilation. Claims are represented only by immutable
// bundle/claim references.
type CurriculumEvidenceReport struct {
	SchemaVersion         string
	Goal                  EvidenceReportGoal
	Competencies          []EvidenceReportCompetency
	ConceptCount          int
	SourceBundleCount     int
	Bundles               []SourceBundleRef
	Claims                []EvidenceRef
	PrimarySourceCoverage EvidenceReportCoverage
	Freshness             []EvidenceReportFreshness
	Conflicts             []EvidenceReportConflict
	Caveats               []EvidenceReportCaveat
	HistoricalContent     []EvidenceReportTemporalContent
	ExperimentalContent   []EvidenceReportTemporalContent
	Gaps                  []Gap
	CompilerVersion       string
	PassVersions          []CompilationPassVersion
}

func (report CurriculumEvidenceReport) Validate() error {
	if report.SchemaVersion != CurriculumEvidenceReportSchemaVersionV1 {
		return fmt.Errorf("unsupported curriculum evidence report schema %q", report.SchemaVersion)
	}
	if err := report.Goal.Validate(); err != nil {
		return err
	}
	if report.ConceptCount < 0 || report.SourceBundleCount < 0 || report.SourceBundleCount != len(report.Bundles) {
		return errors.New("curriculum evidence report counts are invalid")
	}
	for _, competency := range report.Competencies {
		if err := competency.Validate(); err != nil {
			return err
		}
	}
	if err := validateSourceBundleRefs(report.Bundles, SourceReferencesOptionalForFixture); err != nil {
		return err
	}
	for _, claim := range report.Claims {
		if err := claim.Validate(); err != nil {
			return err
		}
	}
	if err := report.PrimarySourceCoverage.Validate(); err != nil {
		return err
	}
	if report.PrimarySourceCoverage.ReferencedClaims != len(report.Claims) {
		return errors.New("curriculum evidence report referenced claim count does not match claims")
	}
	for _, freshness := range report.Freshness {
		if err := freshness.Validate(); err != nil {
			return err
		}
	}
	for _, conflict := range report.Conflicts {
		if err := conflict.Validate(); err != nil {
			return err
		}
	}
	for _, caveat := range report.Caveats {
		if err := caveat.Validate(); err != nil {
			return err
		}
	}
	for _, content := range append(append([]EvidenceReportTemporalContent(nil), report.HistoricalContent...), report.ExperimentalContent...) {
		if err := content.Validate(); err != nil {
			return err
		}
	}
	for _, gap := range report.Gaps {
		if err := gap.Validate(); err != nil {
			return err
		}
	}
	if err := requireText("evidence report compiler version", report.CompilerVersion); err != nil {
		return err
	}
	if len(report.PassVersions) == 0 {
		return errors.New("curriculum evidence report has no compiler pass versions")
	}
	for _, pass := range report.PassVersions {
		if err := pass.Validate(); err != nil {
			return err
		}
	}
	return nil
}
