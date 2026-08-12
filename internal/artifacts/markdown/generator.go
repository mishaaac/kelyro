// Package markdown renders Kelyro's human-readable workspace documents without
// reading or writing persistence.
package markdown

import (
	"bytes"
	"fmt"
	"path/filepath"
	"strings"
	"text/template"
)

const (
	Creator                 = "foundation-markdown"
	LearningTemplateVersion = "foundation-learning/v1"
	RoadmapTemplateVersion  = "foundation-roadmap/v1"
)

var (
	learningTemplate = template.Must(template.New("learning").Parse(`# Kelyro

Workspace: {{.Workspace}}
Status: initialized

No learning path has been configured yet.
`))
	roadmapTemplate = template.Must(template.New("roadmap").Parse(`# Roadmap

No learning path has been generated yet.
`))
)

// Model contains only the stable, human-facing data needed by Foundation
// documents. Internal workspace state may evolve independently.
type Model struct {
	Workspace string
}

// Document is generated Markdown plus the metadata required for safe
// regeneration.
type Document struct {
	Path            string
	Content         []byte
	TemplateVersion string
}

// Generate renders the complete set of Foundation workspace documents as
// UTF-8 text with LF line endings.
func Generate(model Model) ([]Document, error) {
	model.Workspace = singleLine(model.Workspace)
	if model.Workspace == "" {
		return nil, fmt.Errorf("workspace display name is empty")
	}

	learning, err := execute(learningTemplate, model)
	if err != nil {
		return nil, fmt.Errorf("render LEARNING.md: %w", err)
	}
	roadmap, err := execute(roadmapTemplate, struct{}{})
	if err != nil {
		return nil, fmt.Errorf("render ROADMAP.md: %w", err)
	}

	return []Document{
		{Path: "LEARNING.md", Content: learning, TemplateVersion: LearningTemplateVersion},
		{Path: filepath.Join("00-roadmap", "ROADMAP.md"), Content: roadmap, TemplateVersion: RoadmapTemplateVersion},
	}, nil
}

func execute(parsed *template.Template, data any) ([]byte, error) {
	var rendered bytes.Buffer
	if err := parsed.Execute(&rendered, data); err != nil {
		return nil, err
	}
	return rendered.Bytes(), nil
}

func singleLine(value string) string {
	return strings.Join(strings.Fields(value), " ")
}
