// Package diagnosticjson decodes deterministic diagnostic content supplied
// alongside a curriculum fixture. It evaluates no answers and compiles no
// curriculum content.
package diagnosticjson

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"

	"github.com/mishaaac/kelyro/internal/learning"
)

type document struct {
	ContractVersion   string            `json:"contract_version"`
	ScoringVersion    string            `json:"scoring_version"`
	ID                string            `json:"id"`
	Version           string            `json:"version"`
	CurriculumID      string            `json:"curriculum_id"`
	CurriculumVersion string            `json:"curriculum_version"`
	Title             string            `json:"title"`
	Sections          []sectionDocument `json:"sections"`
}

type sectionDocument struct {
	ID    string         `json:"id"`
	Title string         `json:"title"`
	Items []itemDocument `json:"items"`
}

type itemDocument struct {
	ID              string                `json:"id"`
	ConceptID       string                `json:"concept_id"`
	Kind            string                `json:"kind"`
	Prompt          string                `json:"prompt"`
	Options         []optionDocument      `json:"options"`
	AcceptedAnswers []string              `json:"accepted_answers"`
	Requirements    []requirementDocument `json:"requirements"`
}

type optionDocument struct {
	Value string   `json:"value"`
	Label string   `json:"label"`
	Score *float64 `json:"score"`
}

type requirementDocument struct {
	ItemID       string  `json:"item_id"`
	MinimumScore float64 `json:"minimum_score"`
}

func Load(reader io.Reader) (learning.Diagnostic, error) {
	if reader == nil {
		return learning.Diagnostic{}, fmt.Errorf("load diagnostic JSON: reader is nil")
	}
	decoder := json.NewDecoder(reader)
	decoder.DisallowUnknownFields()
	var source document
	if err := decoder.Decode(&source); err != nil {
		if errors.Is(err, io.EOF) {
			return learning.Diagnostic{}, fmt.Errorf("load diagnostic JSON: document is empty")
		}
		return learning.Diagnostic{}, fmt.Errorf("load diagnostic JSON: %w", err)
	}
	var trailing json.RawMessage
	if err := decoder.Decode(&trailing); !errors.Is(err, io.EOF) {
		if err != nil {
			return learning.Diagnostic{}, fmt.Errorf("load diagnostic JSON trailing value: %w", err)
		}
		return learning.Diagnostic{}, fmt.Errorf("load diagnostic JSON: multiple values are not allowed")
	}
	diagnosticID, err := learning.NewID(source.ID)
	if err != nil {
		return learning.Diagnostic{}, fmt.Errorf("load diagnostic JSON id: %w", err)
	}
	curriculumID, err := learning.NewID(source.CurriculumID)
	if err != nil {
		return learning.Diagnostic{}, fmt.Errorf("load diagnostic JSON curriculum id: %w", err)
	}
	sections := make([]learning.DiagnosticSection, 0, len(source.Sections))
	for sectionIndex, rawSection := range source.Sections {
		sectionID, err := learning.NewID(rawSection.ID)
		if err != nil {
			return learning.Diagnostic{}, fmt.Errorf("load diagnostic JSON section %d: %w", sectionIndex, err)
		}
		section := learning.DiagnosticSection{ID: sectionID, Title: rawSection.Title}
		for itemIndex, rawItem := range rawSection.Items {
			item, err := decodeItem(rawItem)
			if err != nil {
				return learning.Diagnostic{}, fmt.Errorf("load diagnostic JSON section %d item %d: %w", sectionIndex, itemIndex, err)
			}
			section.Items = append(section.Items, item)
		}
		sections = append(sections, section)
	}
	diagnostic, err := learning.NewDiagnostic(source.ContractVersion, source.ScoringVersion,
		learning.DiagnosticRef{ID: diagnosticID, Version: source.Version},
		learning.CurriculumRef{ID: curriculumID, Version: source.CurriculumVersion}, source.Title, sections)
	if err != nil {
		return learning.Diagnostic{}, fmt.Errorf("load diagnostic JSON: %w", err)
	}
	return diagnostic, nil
}

func decodeItem(source itemDocument) (learning.DiagnosticItem, error) {
	id, err := learning.NewID(source.ID)
	if err != nil {
		return learning.DiagnosticItem{}, err
	}
	conceptID, err := learning.NewID(source.ConceptID)
	if err != nil {
		return learning.DiagnosticItem{}, err
	}
	item := learning.DiagnosticItem{ID: id, ConceptID: conceptID, Kind: learning.DiagnosticItemKind(source.Kind), Prompt: source.Prompt, AcceptedAnswers: append([]string(nil), source.AcceptedAnswers...)}
	for _, raw := range source.Options {
		score := 0.0
		if raw.Score != nil {
			score = *raw.Score
		}
		decodedScore, err := learning.NewMasteryScore(score)
		if err != nil {
			return learning.DiagnosticItem{}, fmt.Errorf("option %q score: %w", raw.Value, err)
		}
		item.Options = append(item.Options, learning.DiagnosticOption{Value: raw.Value, Label: raw.Label, Score: decodedScore})
	}
	for _, raw := range source.Requirements {
		requiredID, err := learning.NewID(raw.ItemID)
		if err != nil {
			return learning.DiagnosticItem{}, fmt.Errorf("requirement: %w", err)
		}
		minimum, err := learning.NewMasteryScore(raw.MinimumScore)
		if err != nil {
			return learning.DiagnosticItem{}, fmt.Errorf("requirement %q score: %w", raw.ItemID, err)
		}
		item.Requirements = append(item.Requirements, learning.DiagnosticBranchRequirement{ItemID: requiredID, MinimumScore: minimum})
	}
	return item, nil
}
