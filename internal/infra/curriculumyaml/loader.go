// Package curriculumyaml decodes the fixture/import representation of the
// curriculum consumption contract. It does not compile or enrich curricula.
package curriculumyaml

import (
	"errors"
	"fmt"
	"io"

	"github.com/mishaaac/kelyro/internal/learning"
	"go.yaml.in/yaml/v3"
)

type document struct {
	ContractVersion string         `yaml:"contract_version"`
	ID              string         `yaml:"id"`
	Version         string         `yaml:"version"`
	Title           string         `yaml:"title"`
	Description     string         `yaml:"description"`
	Nodes           []nodeDocument `yaml:"nodes"`
}

type nodeDocument struct {
	ID          string           `yaml:"id"`
	Type        string           `yaml:"type"`
	Parent      string           `yaml:"parent,omitempty"`
	Title       string           `yaml:"title"`
	Description string           `yaml:"description"`
	Order       int              `yaml:"order"`
	Display     displayDocument  `yaml:"display"`
	Status      statusDocument   `yaml:"status"`
	Version     string           `yaml:"version"`
	Concept     *conceptDocument `yaml:"concept,omitempty"`
}

type displayDocument struct {
	ShortTitle string `yaml:"short_title,omitempty"`
	Hidden     bool   `yaml:"hidden,omitempty"`
}

type statusDocument struct {
	State string `yaml:"state"`
	Note  string `yaml:"note,omitempty"`
}

type conceptDocument struct {
	Objectives             []string               `yaml:"objectives"`
	Prerequisites          []prerequisiteDocument `yaml:"prerequisites,omitempty"`
	Difficulty             int                    `yaml:"difficulty"`
	EstimatedEffortMinutes int                    `yaml:"estimated_effort_minutes"`
	TheoryRequired         bool                   `yaml:"theory_required"`
	AssessmentExpectations []string               `yaml:"assessment_expectations"`
}

type prerequisiteDocument struct {
	ConceptID   string `yaml:"concept_id"`
	Requirement string `yaml:"requirement"`
}

// Load strictly decodes exactly one YAML document and returns a validated,
// canonical curriculum definition.
func Load(reader io.Reader) (learning.Curriculum, error) {
	if reader == nil {
		return learning.Curriculum{}, fmt.Errorf("load curriculum YAML: reader is nil")
	}
	decoder := yaml.NewDecoder(reader)
	decoder.KnownFields(true)

	var source document
	if err := decoder.Decode(&source); err != nil {
		if errors.Is(err, io.EOF) {
			return learning.Curriculum{}, fmt.Errorf("load curriculum YAML: document is empty")
		}
		return learning.Curriculum{}, fmt.Errorf("load curriculum YAML: %w", err)
	}
	var extra yaml.Node
	if err := decoder.Decode(&extra); !errors.Is(err, io.EOF) {
		if err != nil {
			return learning.Curriculum{}, fmt.Errorf("load curriculum YAML trailing document: %w", err)
		}
		return learning.Curriculum{}, fmt.Errorf("load curriculum YAML: multiple documents are not allowed")
	}

	referenceID, err := learning.NewID(source.ID)
	if err != nil {
		return learning.Curriculum{}, fmt.Errorf("load curriculum YAML id: %w", err)
	}
	nodes := make([]learning.CurriculumNode, 0, len(source.Nodes))
	for index, raw := range source.Nodes {
		node, err := decodeNode(raw)
		if err != nil {
			return learning.Curriculum{}, fmt.Errorf("load curriculum YAML node %d: %w", index, err)
		}
		nodes = append(nodes, node)
	}
	curriculum, err := learning.NewCurriculum(
		source.ContractVersion,
		learning.CurriculumRef{ID: referenceID, Version: source.Version},
		source.Title,
		source.Description,
		nodes,
	)
	if err != nil {
		return learning.Curriculum{}, fmt.Errorf("load curriculum YAML: %w", err)
	}
	return curriculum, nil
}

func decodeNode(source nodeDocument) (learning.CurriculumNode, error) {
	id, err := learning.NewID(source.ID)
	if err != nil {
		return learning.CurriculumNode{}, fmt.Errorf("id: %w", err)
	}
	var parentID *learning.ID
	if source.Parent != "" {
		parent, err := learning.NewID(source.Parent)
		if err != nil {
			return learning.CurriculumNode{}, fmt.Errorf("parent: %w", err)
		}
		parentID = &parent
	}
	node := learning.CurriculumNode{
		ID:          id,
		Type:        learning.CurriculumNodeType(source.Type),
		ParentID:    parentID,
		Title:       source.Title,
		Description: source.Description,
		Order:       source.Order,
		Display: learning.CurriculumDisplayHints{
			ShortTitle: source.Display.ShortTitle,
			Hidden:     source.Display.Hidden,
		},
		Status: learning.CurriculumStatusMetadata{
			State: learning.CurriculumNodeStatus(source.Status.State),
			Note:  source.Status.Note,
		},
		Version: source.Version,
	}
	if source.Concept != nil {
		definition, err := decodeConcept(*source.Concept)
		if err != nil {
			return learning.CurriculumNode{}, err
		}
		node.Concept = &definition
	}
	return node, nil
}

func decodeConcept(source conceptDocument) (learning.ConceptDefinition, error) {
	prerequisites := make([]learning.ConceptPrerequisite, 0, len(source.Prerequisites))
	for index, raw := range source.Prerequisites {
		conceptID, err := learning.NewID(raw.ConceptID)
		if err != nil {
			return learning.ConceptDefinition{}, fmt.Errorf("prerequisite %d: %w", index, err)
		}
		prerequisites = append(prerequisites, learning.ConceptPrerequisite{
			ConceptID:   conceptID,
			Requirement: learning.PrerequisiteRequirement(raw.Requirement),
		})
	}
	return learning.ConceptDefinition{
		Objectives:             append([]string(nil), source.Objectives...),
		Prerequisites:          prerequisites,
		Difficulty:             learning.ConceptDifficulty(source.Difficulty),
		EstimatedEffortMinutes: source.EstimatedEffortMinutes,
		TheoryRequired:         source.TheoryRequired,
		AssessmentExpectations: append([]string(nil), source.AssessmentExpectations...),
	}, nil
}
