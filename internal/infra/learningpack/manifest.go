// Package learningpack implements the portable Learning Pack v1 adapters.
// It parses untrusted files into curriculum domain values; it never executes
// pack content or performs network access.
package learningpack

import (
	"errors"
	"fmt"
	"io"
	"path"
	"regexp"
	"strings"
	"time"
	"unicode"
	"unicode/utf8"

	"github.com/mishaaac/kelyro/internal/curriculum"
	"go.yaml.in/yaml/v3"
)

const (
	SchemaVersionV1      = curriculum.LearningPackSchemaVersionV1
	ManifestName         = "pack.yaml"
	ChecksumsName        = "checksums.txt"
	MaximumManifestBytes = 64 << 10
	MaximumPackPathBytes = 1024
)

var portableIDPattern = regexp.MustCompile(`^[a-z0-9]+(?:[._-][a-z0-9]+)*$`)

type manifestDocument struct {
	ID                   string               `yaml:"id"`
	Name                 string               `yaml:"name"`
	Description          string               `yaml:"description"`
	Version              string               `yaml:"version"`
	SchemaVersion        string               `yaml:"schema_version"`
	Domain               string               `yaml:"domain"`
	Target               string               `yaml:"target"`
	Authors              []string             `yaml:"authors"`
	Maintainers          []string             `yaml:"maintainers"`
	License              string               `yaml:"license"`
	CreatedAt            string               `yaml:"created_at"`
	MinimumKelyroVersion string               `yaml:"minimum_kelyro_version"`
	Dependencies         []dependencyDocument `yaml:"dependencies,omitempty"`
	EnvironmentPack      string               `yaml:"environment_pack,omitempty"`
	CurriculumEntry      string               `yaml:"curriculum_entry"`
	SourceEvidenceEntry  string               `yaml:"source_evidence_entry"`
	BuildInfoEntry       string               `yaml:"build_info_entry"`
	Status               string               `yaml:"status"`
	CurriculumID         string               `yaml:"curriculum_id"`
}

type dependencyDocument struct {
	ID         string `yaml:"id"`
	Constraint string `yaml:"constraint"`
}

// ParseManifest strictly decodes exactly one bounded UTF-8 YAML document.
func ParseManifest(reader io.Reader) (curriculum.PackManifest, error) {
	if reader == nil {
		return curriculum.PackManifest{}, fmt.Errorf("parse pack manifest: reader is nil")
	}
	limited := &io.LimitedReader{R: reader, N: MaximumManifestBytes + 1}
	encoded, err := io.ReadAll(limited)
	if err != nil {
		return curriculum.PackManifest{}, fmt.Errorf("parse pack manifest: %w", err)
	}
	if len(encoded) > MaximumManifestBytes {
		return curriculum.PackManifest{}, fmt.Errorf("parse pack manifest: exceeds %d bytes", MaximumManifestBytes)
	}
	if !utf8.Valid(encoded) {
		return curriculum.PackManifest{}, fmt.Errorf("parse pack manifest: invalid UTF-8")
	}
	decoder := yaml.NewDecoder(strings.NewReader(string(encoded)))
	decoder.KnownFields(true)
	var source manifestDocument
	if err := decoder.Decode(&source); err != nil {
		if errors.Is(err, io.EOF) {
			return curriculum.PackManifest{}, fmt.Errorf("parse pack manifest: document is empty")
		}
		return curriculum.PackManifest{}, fmt.Errorf("parse pack manifest: %w", err)
	}
	var extra yaml.Node
	if err := decoder.Decode(&extra); !errors.Is(err, io.EOF) {
		if err != nil {
			return curriculum.PackManifest{}, fmt.Errorf("parse pack manifest trailing document: %w", err)
		}
		return curriculum.PackManifest{}, fmt.Errorf("parse pack manifest: multiple documents are not allowed")
	}
	return decodeManifest(source)
}

func decodeManifest(source manifestDocument) (curriculum.PackManifest, error) {
	if source.SchemaVersion != SchemaVersionV1 {
		return curriculum.PackManifest{}, fmt.Errorf("unsupported pack schema version %q", source.SchemaVersion)
	}
	if !portableIDPattern.MatchString(source.ID) {
		return curriculum.PackManifest{}, fmt.Errorf("pack id %q is not a stable portable id", source.ID)
	}
	id, err := curriculum.NewID(source.ID)
	if err != nil {
		return curriculum.PackManifest{}, err
	}
	version, err := curriculum.NewPackVersion(source.Version)
	if err != nil {
		return curriculum.PackManifest{}, err
	}
	minimumVersion, err := curriculum.NewPackVersion(source.MinimumKelyroVersion)
	if err != nil {
		return curriculum.PackManifest{}, fmt.Errorf("minimum Kelyro version: %w", err)
	}
	curriculumID, err := curriculum.NewCurriculumID(source.CurriculumID)
	if err != nil {
		return curriculum.PackManifest{}, err
	}
	created, err := time.Parse(time.RFC3339, source.CreatedAt)
	if err != nil {
		return curriculum.PackManifest{}, fmt.Errorf("pack created_at must be RFC3339: %w", err)
	}
	if created.Location() != time.UTC || !strings.HasSuffix(source.CreatedAt, "Z") {
		return curriculum.PackManifest{}, fmt.Errorf("pack created_at must use UTC Z notation")
	}
	createdAt, err := curriculum.NewTimestamp(created)
	if err != nil {
		return curriculum.PackManifest{}, err
	}
	entries := []struct{ name, value string }{
		{"curriculum_entry", source.CurriculumEntry},
		{"source_evidence_entry", source.SourceEvidenceEntry},
		{"build_info_entry", source.BuildInfoEntry},
	}
	if source.EnvironmentPack != "" {
		entries = append(entries, struct{ name, value string }{"environment_pack", source.EnvironmentPack})
	}
	seenEntries := make(map[string]string, len(entries))
	for _, entry := range entries {
		if err := validatePortablePath(entry.name, entry.value); err != nil {
			return curriculum.PackManifest{}, err
		}
		if previous, exists := seenEntries[entry.value]; exists {
			return curriculum.PackManifest{}, fmt.Errorf("%s and %s reference the same entry %q", previous, entry.name, entry.value)
		}
		seenEntries[entry.value] = entry.name
	}
	dependencies := make([]curriculum.PackDependency, 0, len(source.Dependencies))
	for index, raw := range source.Dependencies {
		if !portableIDPattern.MatchString(raw.ID) {
			return curriculum.PackManifest{}, fmt.Errorf("dependency %d id %q is not a stable portable id", index, raw.ID)
		}
		dependencyID, err := curriculum.NewID(raw.ID)
		if err != nil {
			return curriculum.PackManifest{}, err
		}
		if err := validateVersionConstraint(raw.Constraint); err != nil {
			return curriculum.PackManifest{}, fmt.Errorf("dependency %q: %w", raw.ID, err)
		}
		dependencies = append(dependencies, curriculum.PackDependency{PackID: dependencyID, Constraint: raw.Constraint})
	}
	manifest := curriculum.PackManifest{
		ID: id, Name: source.Name, Description: source.Description, Version: version,
		SchemaVersion: source.SchemaVersion, Domain: source.Domain, Target: source.Target,
		Authors: append([]string(nil), source.Authors...), Maintainers: append([]string(nil), source.Maintainers...),
		License: source.License, CreatedAt: createdAt, MinimumKelyroVersion: minimumVersion,
		Dependencies: dependencies, EnvironmentEntry: source.EnvironmentPack,
		CurriculumEntry: source.CurriculumEntry, SourceEvidenceEntry: source.SourceEvidenceEntry,
		BuildInfoEntry: source.BuildInfoEntry,
		Status:         curriculum.PackStatus(source.Status), CurriculumID: curriculumID,
	}
	if err := manifest.Validate(); err != nil {
		return curriculum.PackManifest{}, fmt.Errorf("pack manifest: %w", err)
	}
	return manifest, nil
}

func validatePortablePath(name, value string) error {
	if value == "" || len(value) > MaximumPackPathBytes || !utf8.ValidString(value) || strings.ContainsAny(value, `\:<>"|?*`) {
		return fmt.Errorf("%s %q is not a valid portable relative path", name, value)
	}
	if strings.IndexFunc(value, unicode.IsControl) >= 0 {
		return fmt.Errorf("%s contains a control character", name)
	}
	if path.IsAbs(value) || path.Clean(value) != value || value == "." || strings.HasPrefix(value, "../") {
		return fmt.Errorf("%s %q is not a canonical relative path", name, value)
	}
	for _, part := range strings.Split(value, "/") {
		if part == "" || part == "." || part == ".." || strings.HasSuffix(part, " ") || strings.HasSuffix(part, ".") || windowsReservedPathSegment(part) {
			return fmt.Errorf("%s %q contains an unsafe path segment", name, value)
		}
	}
	return nil
}

func windowsReservedPathSegment(value string) bool {
	base := strings.ToUpper(strings.TrimSuffix(value, path.Ext(value)))
	if base == "CON" || base == "PRN" || base == "AUX" || base == "NUL" {
		return true
	}
	return len(base) == 4 && (strings.HasPrefix(base, "COM") || strings.HasPrefix(base, "LPT")) && base[3] >= '1' && base[3] <= '9'
}

func validateVersionConstraint(value string) error {
	if value == "" || value != strings.TrimSpace(value) {
		return fmt.Errorf("version constraint is empty or has surrounding whitespace")
	}
	for _, clause := range strings.Fields(value) {
		versionText := clause
		for _, prefix := range []string{"<=", ">=", "<", ">", "="} {
			if strings.HasPrefix(versionText, prefix) {
				versionText = strings.TrimPrefix(versionText, prefix)
				break
			}
		}
		if _, err := curriculum.NewPackVersion(versionText); err != nil {
			return fmt.Errorf("invalid version constraint %q", value)
		}
	}
	return nil
}
