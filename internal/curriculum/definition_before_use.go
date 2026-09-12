package curriculum

import "fmt"

const DefinitionBeforeUseAuditVersionV1 = "definition-before-use-v1"

type DefinitionBeforeUseViolationCode string

const (
	DefinitionBeforeUseMissingPrerequisite DefinitionBeforeUseViolationCode = "missing_vocabulary_prerequisite"
	DefinitionBeforeUseIntroducedAfterUse  DefinitionBeforeUseViolationCode = "introduced_after_use"
	DefinitionBeforeUseSameLessonOrder     DefinitionBeforeUseViolationCode = "same_lesson_out_of_order"
)

func (code DefinitionBeforeUseViolationCode) Validate() error {
	switch code {
	case DefinitionBeforeUseMissingPrerequisite, DefinitionBeforeUseIntroducedAfterUse, DefinitionBeforeUseSameLessonOrder:
		return nil
	default:
		return fmt.Errorf("invalid definition-before-use violation code %q", code)
	}
}

type CurriculumAuditSeverity string

const (
	CurriculumAuditWarning CurriculumAuditSeverity = "warning"
	CurriculumAuditError   CurriculumAuditSeverity = "error"
)

func (severity CurriculumAuditSeverity) Validate() error {
	switch severity {
	case CurriculumAuditWarning, CurriculumAuditError:
		return nil
	default:
		return fmt.Errorf("invalid curriculum audit severity %q", severity)
	}
}

type DefinitionBeforeUseViolation struct {
	Code                  DefinitionBeforeUseViolationCode
	Term                  string
	ObservedTerm          string
	UsedAt                ConceptID
	ExpectedIntroduction  ConceptID
	SuggestedPrerequisite *ConceptID
	Severity              CurriculumAuditSeverity
	Reason                string
}

func (violation DefinitionBeforeUseViolation) Validate() error {
	if err := violation.Code.Validate(); err != nil {
		return err
	}
	for _, field := range []struct{ name, value string }{
		{name: "definition-before-use term", value: violation.Term},
		{name: "definition-before-use observed term", value: violation.ObservedTerm},
		{name: "definition-before-use reason", value: violation.Reason},
	} {
		if err := requireText(field.name, field.value); err != nil {
			return err
		}
	}
	if err := violation.UsedAt.Validate(); err != nil {
		return fmt.Errorf("definition-before-use use concept: %w", err)
	}
	if err := violation.ExpectedIntroduction.Validate(); err != nil {
		return fmt.Errorf("definition-before-use expected introduction: %w", err)
	}
	if violation.UsedAt == violation.ExpectedIntroduction {
		return fmt.Errorf("definition-before-use violation cannot introduce and use at the same concept")
	}
	if violation.SuggestedPrerequisite != nil {
		if err := violation.SuggestedPrerequisite.Validate(); err != nil {
			return fmt.Errorf("definition-before-use suggested prerequisite: %w", err)
		}
		if *violation.SuggestedPrerequisite != violation.ExpectedIntroduction {
			return fmt.Errorf("definition-before-use suggestion must identify the expected introduction")
		}
	}
	return violation.Severity.Validate()
}

type DefinitionBeforeUseAuditResult struct {
	Passed           bool
	AuditedUseCount  int
	BaselineUseCount int
	Violations       []DefinitionBeforeUseViolation
	AlgorithmVersion string
}

func (result DefinitionBeforeUseAuditResult) Validate() error {
	if result.AlgorithmVersion != DefinitionBeforeUseAuditVersionV1 {
		return fmt.Errorf("unsupported definition-before-use audit version %q", result.AlgorithmVersion)
	}
	if result.AuditedUseCount < 0 || result.BaselineUseCount < 0 {
		return fmt.Errorf("definition-before-use audit counts cannot be negative")
	}
	if result.Passed != (len(result.Violations) == 0) {
		return fmt.Errorf("definition-before-use passed state does not match violations")
	}
	seen := make(map[string]struct{}, len(result.Violations))
	for _, violation := range result.Violations {
		if err := violation.Validate(); err != nil {
			return err
		}
		key := normalizeVocabularyToken(violation.Term) + "\x00" + violation.UsedAt.String()
		if _, exists := seen[key]; exists {
			return fmt.Errorf("duplicate definition-before-use violation for %q at %q", violation.Term, violation.UsedAt)
		}
		seen[key] = struct{}{}
	}
	return nil
}
