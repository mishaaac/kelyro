package learning

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"math"
	"strings"
)

const (
	DiagnosticContractVersion      = "diagnostic/v1"
	DiagnosticScoringPolicyVersion = "diagnostic-scoring/v1"
	diagnosticObjectiveWeight      = 1.0
	diagnosticSelfReportWeight     = 0.25
	diagnosticConfidenceTarget     = 2.0
)

type DiagnosticRef struct {
	ID      ID
	Version string
}

func (reference DiagnosticRef) Validate() error {
	if err := reference.ID.Validate(); err != nil {
		return fmt.Errorf("diagnostic: %w", err)
	}
	return requireText("diagnostic version", reference.Version)
}

type DiagnosticItemKind string

const (
	DiagnosticSingleChoice   DiagnosticItemKind = "single_choice"
	DiagnosticMultipleChoice DiagnosticItemKind = "multiple_choice"
	DiagnosticShortAnswer    DiagnosticItemKind = "short_answer"
	DiagnosticSelfReport     DiagnosticItemKind = "self_report"
)

func (kind DiagnosticItemKind) Valid() bool {
	switch kind {
	case DiagnosticSingleChoice, DiagnosticMultipleChoice, DiagnosticShortAnswer, DiagnosticSelfReport:
		return true
	default:
		return false
	}
}

type DiagnosticOption struct {
	Value string
	Label string
	Score MasteryScore
}

type DiagnosticBranchRequirement struct {
	ItemID       ID
	MinimumScore MasteryScore
}

type DiagnosticItem struct {
	ID              ID
	ConceptID       ID
	Kind            DiagnosticItemKind
	Prompt          string
	Options         []DiagnosticOption
	AcceptedAnswers []string
	Requirements    []DiagnosticBranchRequirement
}

func (item DiagnosticItem) Validate() error {
	if err := item.ID.Validate(); err != nil {
		return fmt.Errorf("diagnostic item: %w", err)
	}
	if err := item.ConceptID.Validate(); err != nil {
		return fmt.Errorf("diagnostic item concept: %w", err)
	}
	if !item.Kind.Valid() {
		return fmt.Errorf("diagnostic item kind %q is invalid", item.Kind)
	}
	if err := requireText("diagnostic item prompt", item.Prompt); err != nil {
		return err
	}
	optionValues := make(map[string]struct{}, len(item.Options))
	for _, option := range item.Options {
		if err := requireText("diagnostic option value", option.Value); err != nil {
			return err
		}
		if err := requireText("diagnostic option label", option.Label); err != nil {
			return err
		}
		if _, exists := optionValues[option.Value]; exists {
			return fmt.Errorf("diagnostic item %q has duplicate option %q", item.ID, option.Value)
		}
		optionValues[option.Value] = struct{}{}
		if item.Kind == DiagnosticSelfReport {
			if err := option.Score.Validate(); err != nil {
				return fmt.Errorf("diagnostic self-report option %q: %w", option.Value, err)
			}
		}
	}
	switch item.Kind {
	case DiagnosticSingleChoice:
		if len(item.Options) < 2 || len(item.AcceptedAnswers) != 1 {
			return fmt.Errorf("single-choice item %q requires options and exactly one accepted answer", item.ID)
		}
	case DiagnosticMultipleChoice:
		if len(item.Options) < 2 || len(item.AcceptedAnswers) == 0 {
			return fmt.Errorf("multiple-choice item %q requires options and accepted answers", item.ID)
		}
	case DiagnosticShortAnswer:
		if len(item.Options) != 0 || len(item.AcceptedAnswers) == 0 {
			return fmt.Errorf("short-answer item %q requires accepted answers and no options", item.ID)
		}
	case DiagnosticSelfReport:
		if len(item.Options) < 2 || len(item.AcceptedAnswers) != 0 {
			return fmt.Errorf("self-report item %q requires scored options and no accepted answers", item.ID)
		}
	}
	accepted := make(map[string]struct{}, len(item.AcceptedAnswers))
	for _, answer := range item.AcceptedAnswers {
		normalized := normalizeDiagnosticAnswer(answer)
		if normalized == "" {
			return fmt.Errorf("diagnostic item %q has an empty accepted answer", item.ID)
		}
		if _, exists := accepted[normalized]; exists {
			return fmt.Errorf("diagnostic item %q has duplicate accepted answer %q", item.ID, answer)
		}
		accepted[normalized] = struct{}{}
		if item.Kind != DiagnosticShortAnswer {
			if _, exists := optionValues[answer]; !exists {
				return fmt.Errorf("diagnostic item %q accepts unknown option %q", item.ID, answer)
			}
		}
	}
	for _, requirement := range item.Requirements {
		if err := requirement.ItemID.Validate(); err != nil {
			return fmt.Errorf("diagnostic item requirement: %w", err)
		}
		if err := requirement.MinimumScore.Validate(); err != nil {
			return fmt.Errorf("diagnostic item requirement score: %w", err)
		}
	}
	return nil
}

// Evaluate scores one answer deterministically and never retains its raw text.
func (item DiagnosticItem) Evaluate(answers []string) (MasteryScore, error) {
	if err := item.Validate(); err != nil {
		return MasteryScore{}, err
	}
	if len(answers) == 0 {
		return MasteryScore{}, fmt.Errorf("diagnostic answer is empty")
	}
	seen := make(map[string]struct{}, len(answers))
	for _, answer := range answers {
		if strings.TrimSpace(answer) == "" {
			return MasteryScore{}, fmt.Errorf("diagnostic answer contains an empty value")
		}
		if _, exists := seen[answer]; exists {
			return MasteryScore{}, fmt.Errorf("diagnostic answer contains a duplicate value")
		}
		seen[answer] = struct{}{}
	}
	if item.Kind != DiagnosticShortAnswer {
		allowed := make(map[string]DiagnosticOption, len(item.Options))
		for _, option := range item.Options {
			allowed[option.Value] = option
		}
		for _, answer := range answers {
			if _, exists := allowed[answer]; !exists {
				return MasteryScore{}, fmt.Errorf("diagnostic answer is not an item option")
			}
		}
		if item.Kind != DiagnosticMultipleChoice && len(answers) != 1 {
			return MasteryScore{}, fmt.Errorf("diagnostic item %q accepts exactly one answer", item.ID)
		}
		if item.Kind == DiagnosticSelfReport {
			return allowed[answers[0]].Score, nil
		}
	}
	correct := make(map[string]struct{}, len(item.AcceptedAnswers))
	for _, answer := range item.AcceptedAnswers {
		correct[normalizeDiagnosticAnswer(answer)] = struct{}{}
	}
	if len(answers) != len(correct) {
		return NewMasteryScore(0)
	}
	for _, answer := range answers {
		if _, exists := correct[normalizeDiagnosticAnswer(answer)]; !exists {
			return NewMasteryScore(0)
		}
	}
	return NewMasteryScore(1)
}

func (item DiagnosticItem) EvidenceWeight() float64 {
	if item.Kind == DiagnosticSelfReport {
		return diagnosticSelfReportWeight
	}
	return diagnosticObjectiveWeight
}

type DiagnosticSection struct {
	ID    ID
	Title string
	Items []DiagnosticItem
}

type Diagnostic struct {
	ContractVersion string
	ScoringVersion  string
	Reference       DiagnosticRef
	Curriculum      CurriculumRef
	Title           string
	Sections        []DiagnosticSection
}

func NewDiagnostic(contractVersion, scoringVersion string, reference DiagnosticRef, curriculum CurriculumRef, title string, sections []DiagnosticSection) (Diagnostic, error) {
	diagnostic := Diagnostic{ContractVersion: contractVersion, ScoringVersion: scoringVersion, Reference: reference, Curriculum: curriculum, Title: title, Sections: cloneDiagnosticSections(sections)}
	if err := diagnostic.Validate(); err != nil {
		return Diagnostic{}, err
	}
	return diagnostic, nil
}

func (diagnostic Diagnostic) Validate() error {
	if diagnostic.ContractVersion != DiagnosticContractVersion {
		return fmt.Errorf("unsupported diagnostic contract version %q", diagnostic.ContractVersion)
	}
	if diagnostic.ScoringVersion != DiagnosticScoringPolicyVersion {
		return fmt.Errorf("unsupported diagnostic scoring version %q", diagnostic.ScoringVersion)
	}
	if err := diagnostic.Reference.Validate(); err != nil {
		return err
	}
	if err := diagnostic.Curriculum.Validate(); err != nil {
		return err
	}
	if err := requireText("diagnostic title", diagnostic.Title); err != nil {
		return err
	}
	if len(diagnostic.Sections) == 0 {
		return fmt.Errorf("diagnostic has no sections")
	}
	sectionIDs := make(map[ID]struct{}, len(diagnostic.Sections))
	itemOrder := make(map[ID]int)
	position := 0
	for _, section := range diagnostic.Sections {
		if err := section.ID.Validate(); err != nil {
			return fmt.Errorf("diagnostic section: %w", err)
		}
		if err := requireText("diagnostic section title", section.Title); err != nil {
			return err
		}
		if _, exists := sectionIDs[section.ID]; exists {
			return fmt.Errorf("duplicate diagnostic section %q", section.ID)
		}
		sectionIDs[section.ID] = struct{}{}
		if len(section.Items) == 0 {
			return fmt.Errorf("diagnostic section %q has no items", section.ID)
		}
		for _, item := range section.Items {
			if err := item.Validate(); err != nil {
				return err
			}
			if _, exists := itemOrder[item.ID]; exists {
				return fmt.Errorf("duplicate diagnostic item %q", item.ID)
			}
			itemOrder[item.ID] = position
			position++
		}
	}
	for _, item := range diagnostic.Items() {
		for _, requirement := range item.Requirements {
			requiredPosition, exists := itemOrder[requirement.ItemID]
			if !exists {
				return fmt.Errorf("diagnostic item %q requires unknown item %q", item.ID, requirement.ItemID)
			}
			if requiredPosition >= itemOrder[item.ID] {
				return fmt.Errorf("diagnostic item %q requirement %q must appear earlier", item.ID, requirement.ItemID)
			}
		}
	}
	return nil
}

func (diagnostic Diagnostic) Items() []DiagnosticItem {
	items := make([]DiagnosticItem, 0)
	for _, section := range diagnostic.Sections {
		items = append(items, section.Items...)
	}
	return items
}

func (diagnostic Diagnostic) Item(id ID) (DiagnosticItem, bool) {
	for _, item := range diagnostic.Items() {
		if item.ID == id {
			return item, true
		}
	}
	return DiagnosticItem{}, false
}

type DiagnosticAttemptStatus string

const (
	DiagnosticInProgress DiagnosticAttemptStatus = "in_progress"
	DiagnosticCompleted  DiagnosticAttemptStatus = "completed"
	DiagnosticSkipped    DiagnosticAttemptStatus = "skipped"
)

func (status DiagnosticAttemptStatus) Valid() bool {
	switch status {
	case DiagnosticInProgress, DiagnosticCompleted, DiagnosticSkipped:
		return true
	default:
		return false
	}
}

type DiagnosticObservation struct {
	ItemID     ID
	ConceptID  ID
	Score      MasteryScore
	EvidenceID ID
	AnsweredAt Timestamp
}

type DiagnosticAttempt struct {
	ID                    ID
	StudentID             ID
	CurriculumInstanceID  ID
	Diagnostic            DiagnosticRef
	DefinitionFingerprint string
	Status                DiagnosticAttemptStatus
	Observations          []DiagnosticObservation
	StartedAt             Timestamp
	UpdatedAt             Timestamp
	CompletedAt           *Timestamp
	SkippedAt             *Timestamp
}

func NewDiagnosticAttempt(id, studentID, instanceID ID, diagnostic Diagnostic, startedAt Timestamp) (DiagnosticAttempt, error) {
	fingerprint, err := DiagnosticFingerprint(diagnostic)
	if err != nil {
		return DiagnosticAttempt{}, err
	}
	attempt := DiagnosticAttempt{ID: id, StudentID: studentID, CurriculumInstanceID: instanceID, Diagnostic: diagnostic.Reference, DefinitionFingerprint: fingerprint, Status: DiagnosticInProgress, StartedAt: startedAt, UpdatedAt: startedAt}
	return attempt, attempt.Validate()
}

func (attempt DiagnosticAttempt) Validate() error {
	for name, id := range map[string]ID{"attempt": attempt.ID, "student": attempt.StudentID, "curriculum instance": attempt.CurriculumInstanceID} {
		if err := id.Validate(); err != nil {
			return fmt.Errorf("diagnostic %s: %w", name, err)
		}
	}
	if err := attempt.Diagnostic.Validate(); err != nil {
		return err
	}
	if !strings.HasPrefix(attempt.DefinitionFingerprint, "sha256:") || len(attempt.DefinitionFingerprint) != 71 {
		return fmt.Errorf("diagnostic definition fingerprint is invalid")
	}
	if !attempt.Status.Valid() {
		return fmt.Errorf("diagnostic attempt status %q is invalid", attempt.Status)
	}
	if err := attempt.StartedAt.Validate(); err != nil {
		return fmt.Errorf("diagnostic started at: %w", err)
	}
	if err := attempt.UpdatedAt.Validate(); err != nil {
		return fmt.Errorf("diagnostic updated at: %w", err)
	}
	if attempt.UpdatedAt.Before(attempt.StartedAt) {
		return fmt.Errorf("diagnostic update precedes start")
	}
	seenItems := make(map[ID]struct{}, len(attempt.Observations))
	seenEvidence := make(map[ID]struct{}, len(attempt.Observations))
	for _, observation := range attempt.Observations {
		if err := observation.ItemID.Validate(); err != nil {
			return fmt.Errorf("diagnostic observation item: %w", err)
		}
		if err := observation.ConceptID.Validate(); err != nil {
			return fmt.Errorf("diagnostic observation concept: %w", err)
		}
		if err := observation.Score.Validate(); err != nil {
			return fmt.Errorf("diagnostic observation: %w", err)
		}
		if err := observation.EvidenceID.Validate(); err != nil {
			return fmt.Errorf("diagnostic observation evidence: %w", err)
		}
		if err := observation.AnsweredAt.Validate(); err != nil {
			return fmt.Errorf("diagnostic observation answered at: %w", err)
		}
		if observation.AnsweredAt.Before(attempt.StartedAt) || attempt.UpdatedAt.Before(observation.AnsweredAt) {
			return fmt.Errorf("diagnostic observation timestamp is outside attempt")
		}
		if _, exists := seenItems[observation.ItemID]; exists {
			return fmt.Errorf("duplicate diagnostic observation item %q", observation.ItemID)
		}
		if _, exists := seenEvidence[observation.EvidenceID]; exists {
			return fmt.Errorf("duplicate diagnostic observation evidence %q", observation.EvidenceID)
		}
		seenItems[observation.ItemID] = struct{}{}
		seenEvidence[observation.EvidenceID] = struct{}{}
	}
	if err := validateOptionalTimestamp("diagnostic completed at", attempt.CompletedAt); err != nil {
		return err
	}
	if err := validateOptionalTimestamp("diagnostic skipped at", attempt.SkippedAt); err != nil {
		return err
	}
	switch attempt.Status {
	case DiagnosticInProgress:
		if attempt.CompletedAt != nil || attempt.SkippedAt != nil {
			return fmt.Errorf("in-progress diagnostic cannot have terminal timestamps")
		}
	case DiagnosticCompleted:
		if attempt.CompletedAt == nil || attempt.SkippedAt != nil {
			return fmt.Errorf("completed diagnostic requires only completed timestamp")
		}
	case DiagnosticSkipped:
		if attempt.SkippedAt == nil || attempt.CompletedAt != nil || len(attempt.Observations) != 0 {
			return fmt.Errorf("skipped diagnostic must have no observations and only skipped timestamp")
		}
	}
	return nil
}

func (attempt DiagnosticAttempt) Record(observation DiagnosticObservation) (DiagnosticAttempt, error) {
	if attempt.Status != DiagnosticInProgress {
		return DiagnosticAttempt{}, fmt.Errorf("diagnostic attempt is not in progress")
	}
	attempt.Observations = append(append([]DiagnosticObservation(nil), attempt.Observations...), observation)
	attempt.UpdatedAt = observation.AnsweredAt
	return attempt, attempt.Validate()
}

func (attempt DiagnosticAttempt) Complete(at Timestamp) (DiagnosticAttempt, error) {
	if attempt.Status != DiagnosticInProgress {
		return DiagnosticAttempt{}, fmt.Errorf("diagnostic attempt is not in progress")
	}
	attempt.Status, attempt.UpdatedAt, attempt.CompletedAt = DiagnosticCompleted, at, &at
	return attempt, attempt.Validate()
}

func (attempt DiagnosticAttempt) Skip(at Timestamp) (DiagnosticAttempt, error) {
	if attempt.Status != DiagnosticInProgress || len(attempt.Observations) != 0 {
		return DiagnosticAttempt{}, fmt.Errorf("only an unanswered in-progress diagnostic can be skipped")
	}
	attempt.Status, attempt.UpdatedAt, attempt.SkippedAt = DiagnosticSkipped, at, &at
	return attempt, attempt.Validate()
}

type ConceptEstimate struct {
	ConceptID        ID
	Known            bool
	EstimatedMastery MasteryScore
	Confidence       MasteryScore
	EvidenceCount    int
	ObjectiveCount   int
}

type DiagnosticResult struct {
	AttemptID ID
	Partial   bool
	Estimates []ConceptEstimate
}

func BuildDiagnosticResult(diagnostic Diagnostic, attempt DiagnosticAttempt) (DiagnosticResult, error) {
	if err := diagnostic.Validate(); err != nil {
		return DiagnosticResult{}, err
	}
	if err := validateDiagnosticAttemptDefinition(diagnostic, attempt); err != nil {
		return DiagnosticResult{}, err
	}
	type aggregate struct {
		weightedScore, weight float64
		evidence, objective   int
	}
	aggregates := make(map[ID]aggregate)
	itemByID := make(map[ID]DiagnosticItem)
	conceptOrder := make([]ID, 0)
	seenConcepts := make(map[ID]struct{})
	for _, item := range diagnostic.Items() {
		itemByID[item.ID] = item
		if _, seen := seenConcepts[item.ConceptID]; !seen {
			seenConcepts[item.ConceptID] = struct{}{}
			conceptOrder = append(conceptOrder, item.ConceptID)
		}
	}
	for _, observation := range attempt.Observations {
		item, exists := itemByID[observation.ItemID]
		if !exists || item.ConceptID != observation.ConceptID {
			return DiagnosticResult{}, fmt.Errorf("diagnostic observation item %q does not match definition", observation.ItemID)
		}
		value := aggregates[item.ConceptID]
		weight := item.EvidenceWeight()
		value.weightedScore += observation.Score.Value() * weight
		value.weight += weight
		value.evidence++
		if item.Kind != DiagnosticSelfReport {
			value.objective++
		}
		aggregates[item.ConceptID] = value
	}
	estimates := make([]ConceptEstimate, 0, len(conceptOrder))
	for _, conceptID := range conceptOrder {
		value := aggregates[conceptID]
		estimate := ConceptEstimate{ConceptID: conceptID, EvidenceCount: value.evidence, ObjectiveCount: value.objective}
		if value.weight > 0 {
			estimate.Known = true
			estimate.EstimatedMastery, _ = NewMasteryScore(value.weightedScore / value.weight)
			estimate.Confidence, _ = NewMasteryScore(math.Min(1, value.weight/diagnosticConfidenceTarget))
		}
		estimates = append(estimates, estimate)
	}
	return DiagnosticResult{AttemptID: attempt.ID, Partial: attempt.Status == DiagnosticInProgress, Estimates: estimates}, nil
}

// NextDiagnosticItem applies deterministic v1 branching and redundancy rules.
func NextDiagnosticItem(diagnostic Diagnostic, attempt DiagnosticAttempt) (*DiagnosticItem, error) {
	if err := validateDiagnosticAttemptDefinition(diagnostic, attempt); err != nil {
		return nil, err
	}
	if attempt.Status != DiagnosticInProgress {
		return nil, nil
	}
	observations := make(map[ID]DiagnosticObservation, len(attempt.Observations))
	for _, observation := range attempt.Observations {
		observations[observation.ItemID] = observation
	}
	result, err := BuildDiagnosticResult(diagnostic, attempt)
	if err != nil {
		return nil, err
	}
	confidence := make(map[ID]float64, len(result.Estimates))
	objective := make(map[ID]int, len(result.Estimates))
	for _, estimate := range result.Estimates {
		confidence[estimate.ConceptID], objective[estimate.ConceptID] = estimate.Confidence.Value(), estimate.ObjectiveCount
	}
	skipped := make(map[ID]struct{})
	for _, item := range diagnostic.Items() {
		if _, answered := observations[item.ID]; answered {
			continue
		}
		blocked := false
		for _, requirement := range item.Requirements {
			if _, requirementSkipped := skipped[requirement.ItemID]; requirementSkipped {
				blocked = true
				break
			}
			observation, answered := observations[requirement.ItemID]
			if !answered || observation.Score.Value() < requirement.MinimumScore.Value() {
				blocked = true
				break
			}
		}
		if blocked {
			skipped[item.ID] = struct{}{}
			continue
		}
		if objective[item.ConceptID] >= 2 && confidence[item.ConceptID] >= 1 {
			skipped[item.ID] = struct{}{}
			continue
		}
		candidate := item
		return &candidate, nil
	}
	return nil, nil
}

func validateDiagnosticAttemptDefinition(diagnostic Diagnostic, attempt DiagnosticAttempt) error {
	if err := attempt.Validate(); err != nil {
		return err
	}
	if attempt.Diagnostic != diagnostic.Reference {
		return fmt.Errorf("diagnostic attempt definition reference does not match")
	}
	fingerprint, err := DiagnosticFingerprint(diagnostic)
	if err != nil {
		return err
	}
	if fingerprint != attempt.DefinitionFingerprint {
		return fmt.Errorf("diagnostic definition fingerprint does not match attempt")
	}
	return nil
}

func DiagnosticFingerprint(diagnostic Diagnostic) (string, error) {
	if err := diagnostic.Validate(); err != nil {
		return "", err
	}
	type canonicalOption struct {
		Value, Label string
		Score        float64
	}
	type canonicalRequirement struct {
		ItemID       string
		MinimumScore float64
	}
	type canonicalItem struct {
		ID, ConceptID, Kind, Prompt string
		Options                     []canonicalOption
		Accepted                    []string
		Requirements                []canonicalRequirement
	}
	type canonicalSection struct {
		ID, Title string
		Items     []canonicalItem
	}
	type canonicalDiagnostic struct {
		Contract, Scoring, ID, Version, CurriculumID, CurriculumVersion, Title string
		Sections                                                               []canonicalSection
	}
	canonical := canonicalDiagnostic{Contract: diagnostic.ContractVersion, Scoring: diagnostic.ScoringVersion, ID: diagnostic.Reference.ID.String(), Version: diagnostic.Reference.Version, CurriculumID: diagnostic.Curriculum.ID.String(), CurriculumVersion: diagnostic.Curriculum.Version, Title: diagnostic.Title}
	for _, section := range diagnostic.Sections {
		encodedSection := canonicalSection{ID: section.ID.String(), Title: section.Title}
		for _, item := range section.Items {
			encodedItem := canonicalItem{ID: item.ID.String(), ConceptID: item.ConceptID.String(), Kind: string(item.Kind), Prompt: item.Prompt, Accepted: append([]string(nil), item.AcceptedAnswers...)}
			for _, option := range item.Options {
				encodedItem.Options = append(encodedItem.Options, canonicalOption{option.Value, option.Label, option.Score.Value()})
			}
			for _, requirement := range item.Requirements {
				encodedItem.Requirements = append(encodedItem.Requirements, canonicalRequirement{requirement.ItemID.String(), requirement.MinimumScore.Value()})
			}
			encodedSection.Items = append(encodedSection.Items, encodedItem)
		}
		canonical.Sections = append(canonical.Sections, encodedSection)
	}
	payload, err := json.Marshal(canonical)
	if err != nil {
		return "", err
	}
	digest := sha256.Sum256(payload)
	return "sha256:" + hex.EncodeToString(digest[:]), nil
}

func normalizeDiagnosticAnswer(value string) string {
	return strings.ToLower(strings.Join(strings.Fields(value), " "))
}

func cloneDiagnosticSections(sections []DiagnosticSection) []DiagnosticSection {
	cloned := make([]DiagnosticSection, len(sections))
	for index, section := range sections {
		cloned[index] = section
		cloned[index].Items = append([]DiagnosticItem(nil), section.Items...)
		for itemIndex := range cloned[index].Items {
			item := &cloned[index].Items[itemIndex]
			item.Options = append([]DiagnosticOption(nil), item.Options...)
			item.AcceptedAnswers = append([]string(nil), item.AcceptedAnswers...)
			item.Requirements = append([]DiagnosticBranchRequirement(nil), item.Requirements...)
		}
	}
	return cloned
}
