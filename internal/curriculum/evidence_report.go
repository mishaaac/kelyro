package curriculum

import (
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"math"
	"net/url"
	"strings"
	"unicode"
	"unicode/utf8"
)

const CurriculumEvidenceReportSchemaVersionV1 = "curriculum-evidence-report/v1"
const MaximumCitationExcerptBytes = 512
const MaximumCitationURLBytes = 8 << 10

type EvidenceReportCitation struct {
	SourceID    ID
	Title       string
	URL         string
	License     string
	Excerpt     string
	ExcerptHash string
}

func (citation EvidenceReportCitation) Validate() error {
	if err := citation.SourceID.Validate(); err != nil {
		return err
	}
	if err := requireText("evidence report citation title", citation.Title); err != nil {
		return err
	}
	if len(citation.URL) > MaximumCitationURLBytes {
		return fmt.Errorf("evidence report citation URL exceeds %d bytes", MaximumCitationURLBytes)
	}
	parsed, err := url.Parse(citation.URL)
	if err != nil || (parsed.Scheme != "https" && parsed.Scheme != "http") || parsed.Hostname() == "" || parsed.Opaque != "" {
		return errors.New("evidence report citation URL must be absolute HTTP(S)")
	}
	if strings.ContainsAny(citation.URL, "<>\\") || strings.IndexFunc(citation.URL, unicode.IsControl) >= 0 {
		return errors.New("evidence report citation URL contains unsafe characters")
	}
	if parsed.User != nil {
		return errors.New("evidence report citation URL must not contain credentials")
	}
	query, err := url.ParseQuery(parsed.RawQuery)
	if err != nil {
		return errors.New("evidence report citation URL query is invalid")
	}
	for name := range query {
		if sensitiveCitationParameter(name) {
			return errors.New("evidence report citation URL contains credential-like query parameters")
		}
	}
	if strings.Contains(parsed.Fragment, "=") {
		fragment, fragmentErr := url.ParseQuery(parsed.Fragment)
		if fragmentErr != nil {
			return errors.New("evidence report citation URL fragment is invalid")
		}
		for name := range fragment {
			if sensitiveCitationParameter(name) {
				return errors.New("evidence report citation URL contains credential-like fragment parameters")
			}
		}
	}
	if !utf8.ValidString(citation.Excerpt) || len([]byte(citation.Excerpt)) > MaximumCitationExcerptBytes {
		return fmt.Errorf("evidence report citation excerpt must be valid UTF-8 within %d bytes", MaximumCitationExcerptBytes)
	}
	if strings.IndexFunc(citation.Excerpt, unicode.IsControl) >= 0 {
		return errors.New("evidence report citation excerpt contains a control character")
	}
	if citation.Excerpt == "" && citation.ExcerptHash != "" {
		return errors.New("evidence report citation hash requires an excerpt")
	}
	if citation.Excerpt != "" && !contentHashPattern.MatchString(citation.ExcerptHash) {
		return errors.New("evidence report citation excerpt hash is not canonical sha256")
	}
	if citation.Excerpt != "" {
		digest := sha256.Sum256([]byte(citation.Excerpt))
		if citation.ExcerptHash != "sha256:"+hex.EncodeToString(digest[:]) {
			return errors.New("evidence report citation excerpt hash does not match excerpt")
		}
	}
	if citation.License != "" {
		if err := requireText("evidence report citation license", citation.License); err != nil {
			return err
		}
	}
	return nil
}

func sensitiveCitationParameter(name string) bool {
	normalized := strings.ToLower(strings.ReplaceAll(strings.ReplaceAll(strings.TrimSpace(name), "-", "_"), ".", "_"))
	for _, marker := range []string{"api_key", "apikey", "key", "access_token", "token", "auth", "secret", "password", "credential", "signature", "sig"} {
		if normalized == marker || strings.HasSuffix(normalized, "_"+marker) {
			return true
		}
	}
	return false
}

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
	Citations             []EvidenceReportCitation
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
	seenCompetencies := make(map[ID]struct{}, len(report.Competencies))
	for _, competency := range report.Competencies {
		if err := competency.Validate(); err != nil {
			return err
		}
		if _, exists := seenCompetencies[competency.ID]; exists {
			return fmt.Errorf("duplicate evidence report competency %q", competency.ID)
		}
		seenCompetencies[competency.ID] = struct{}{}
	}
	if err := validateSourceBundleRefs(report.Bundles, SourceReferencesOptionalForFixture); err != nil {
		return err
	}
	seenClaims := make(map[EvidenceRef]struct{}, len(report.Claims))
	for _, claim := range report.Claims {
		if err := claim.Validate(); err != nil {
			return err
		}
		if _, exists := seenClaims[claim]; exists {
			return errors.New("duplicate curriculum evidence report claim")
		}
		seenClaims[claim] = struct{}{}
	}
	seenCitations := make(map[ID]struct{}, len(report.Citations))
	for _, citation := range report.Citations {
		if err := citation.Validate(); err != nil {
			return err
		}
		if _, exists := seenCitations[citation.SourceID]; exists {
			return fmt.Errorf("duplicate evidence report citation %q", citation.SourceID)
		}
		seenCitations[citation.SourceID] = struct{}{}
	}
	if err := report.PrimarySourceCoverage.Validate(); err != nil {
		return err
	}
	if report.PrimarySourceCoverage.ReferencedClaims != len(report.Claims) {
		return errors.New("curriculum evidence report referenced claim count does not match claims")
	}
	seenFreshness := make(map[ID]struct{}, len(report.Freshness))
	for _, freshness := range report.Freshness {
		if err := freshness.Validate(); err != nil {
			return err
		}
		if _, exists := seenFreshness[freshness.BundleID]; exists {
			return fmt.Errorf("duplicate evidence report freshness bundle %q", freshness.BundleID)
		}
		seenFreshness[freshness.BundleID] = struct{}{}
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
	for _, content := range report.HistoricalContent {
		if err := content.Validate(); err != nil {
			return err
		}
		if content.Status != ConceptLegacy && content.Status != ConceptHistorical && content.Status != ConceptDeprecated {
			return fmt.Errorf("concept %q is not historical content", content.ConceptID)
		}
	}
	for _, content := range report.ExperimentalContent {
		if err := content.Validate(); err != nil {
			return err
		}
		if content.Status != ConceptExperimental && content.Status != ConceptPreview {
			return fmt.Errorf("concept %q is not experimental content", content.ConceptID)
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
