package research

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/url"
	"path"
	"regexp"
	"strings"
	"unicode"
)

const (
	SourceCodeEvidenceV1              = "source-code-evidence-v1"
	MaximumSourceCodeLocatorJSONBytes = 8192
	MaximumSourceCodeLineSpan         = 200
	maximumSourceCodeTextBytes        = 512
)

var sourceCodeCommitPattern = regexp.MustCompile(`^[0-9a-fA-F]{7,64}$`)

// SourceCodeLicense records reviewed license metadata when it is available.
// Identifier is intentionally host-neutral and need not be an SPDX value.
type SourceCodeLicense struct {
	Identifier string
	Name       string
	Locator    *SourceLocator
}

func (license SourceCodeLicense) Validate() error {
	if err := validateSourceCodeText("source code license identifier", license.Identifier, true); err != nil {
		return err
	}
	if err := validateSourceCodeText("source code license name", license.Name, false); err != nil {
		return err
	}
	if license.Locator != nil {
		if err := license.Locator.Validate(); err != nil {
			return fmt.Errorf("source code license locator: %w", err)
		}
	}
	return nil
}

// SourceCodeLocator identifies the exact reviewed code behind an Evidence
// excerpt. Permalink is supplied by a host adapter; the domain never builds a
// GitHub-, GitLab-, or forge-specific URL.
type SourceCodeLocator struct {
	Repository       SourceLocator
	Permalink        SourceLocator
	Commit           string
	Path             string
	StartLine        int
	EndLine          int
	Symbol           string
	VersionScope     SourceVersion
	License          *SourceCodeLicense
	AlgorithmVersion string
}

func (locator SourceCodeLocator) Validate() error {
	if err := locator.Repository.Validate(); err != nil {
		return fmt.Errorf("source code repository: %w", err)
	}
	if err := locator.Permalink.Validate(); err != nil {
		return fmt.Errorf("source code permalink: %w", err)
	}
	if !sourceCodeCommitPattern.MatchString(locator.Commit) {
		return fmt.Errorf("source code commit is not a 7-64 character hexadecimal revision")
	}
	if err := validateSourceCodePath(locator.Path); err != nil {
		return err
	}
	if locator.StartLine < 1 || locator.EndLine < locator.StartLine {
		return fmt.Errorf("source code line range is invalid")
	}
	if locator.EndLine-locator.StartLine+1 > MaximumSourceCodeLineSpan {
		return fmt.Errorf("source code line range exceeds %d lines", MaximumSourceCodeLineSpan)
	}
	if err := validateSourceCodeText("source code symbol", locator.Symbol, false); err != nil {
		return err
	}
	if err := locator.VersionScope.Validate(); err != nil {
		return fmt.Errorf("source code version scope: %w", err)
	}
	if locator.License != nil {
		if err := locator.License.Validate(); err != nil {
			return err
		}
	}
	if locator.AlgorithmVersion != SourceCodeEvidenceV1 {
		return fmt.Errorf("invalid source code evidence algorithm %q", locator.AlgorithmVersion)
	}
	if err := validateSourceCodePermalink(locator); err != nil {
		return err
	}
	return nil
}

func (locator SourceCodeLocator) Clone() *SourceCodeLocator {
	clone := locator
	if locator.License != nil {
		license := *locator.License
		if locator.License.Locator != nil {
			licenseLocator := *locator.License.Locator
			license.Locator = &licenseLocator
		}
		clone.License = &license
	}
	return &clone
}

func validateSourceCodePermalink(locator SourceCodeLocator) error {
	repositoryURL, err := url.Parse(locator.Repository.String())
	if err != nil {
		return fmt.Errorf("parse source code repository: %w", err)
	}
	permalinkURL, err := url.Parse(locator.Permalink.String())
	if err != nil {
		return fmt.Errorf("parse source code permalink: %w", err)
	}
	if !strings.EqualFold(repositoryURL.Host, permalinkURL.Host) {
		return fmt.Errorf("source code permalink host does not match repository")
	}
	if locator.Repository.String() == locator.Permalink.String() {
		return fmt.Errorf("source code permalink must differ from repository locator")
	}
	if !strings.Contains(strings.ToLower(permalinkURL.EscapedPath()), strings.ToLower(locator.Commit)) {
		return fmt.Errorf("source code permalink does not contain the immutable commit")
	}
	return nil
}

func validateSourceCodePath(value string) error {
	if value == "" || strings.Contains(value, `\`) || path.IsAbs(value) ||
		path.Clean(value) != value || strings.HasPrefix(value, "../") {
		return fmt.Errorf("source code path is not a clean relative path")
	}
	return validateSourceCodeText("source code path", value, true)
}

func validateSourceCodeText(name, value string, required bool) error {
	if required {
		if err := requireText(name, value); err != nil {
			return err
		}
	} else if err := validateOptionalText(name, value); err != nil {
		return err
	}
	if strings.IndexFunc(value, unicode.IsControl) >= 0 {
		return fmt.Errorf("%s contains control characters", name)
	}
	if len(value) > maximumSourceCodeTextBytes {
		return fmt.Errorf("%s exceeds %d bytes", name, maximumSourceCodeTextBytes)
	}
	return nil
}

// ValidateSourceCodeEvidenceRelationship applies the relational rules that
// Evidence.Validate cannot check without loading its Source.
func ValidateSourceCodeEvidenceRelationship(source Source, evidence Evidence) error {
	if err := source.Validate(); err != nil {
		return fmt.Errorf("source code evidence source: %w", err)
	}
	if err := evidence.Validate(); err != nil {
		return fmt.Errorf("source code evidence: %w", err)
	}
	if source.ID != evidence.SourceID {
		return fmt.Errorf("source code evidence source relationship does not match")
	}
	if source.Kind != SourceCode {
		if evidence.SourceCode != nil {
			return fmt.Errorf("source code locator requires a source_code source")
		}
		return nil
	}
	if evidence.SourceCode == nil {
		return fmt.Errorf("source_code evidence requires a reproducible source code locator")
	}
	if evidence.SourceCode.Repository != source.Locator {
		return fmt.Errorf("source code repository does not match source locator")
	}
	if source.Version != nil && evidence.SourceCode.VersionScope.String() != source.Version.String() {
		return fmt.Errorf("source code version scope does not match source version")
	}
	return nil
}

type sourceCodeLocatorJSON struct {
	AlgorithmVersion string                 `json:"algorithm_version"`
	Repository       string                 `json:"repository"`
	Permalink        string                 `json:"permalink"`
	Commit           string                 `json:"commit"`
	Path             string                 `json:"path"`
	StartLine        int                    `json:"start_line"`
	EndLine          int                    `json:"end_line"`
	Symbol           string                 `json:"symbol,omitempty"`
	VersionScope     string                 `json:"version_scope"`
	License          *sourceCodeLicenseJSON `json:"license,omitempty"`
}

type sourceCodeLicenseJSON struct {
	Identifier string `json:"identifier"`
	Name       string `json:"name,omitempty"`
	Locator    string `json:"locator,omitempty"`
}

func EncodeSourceCodeLocator(locator SourceCodeLocator) ([]byte, error) {
	if err := locator.Validate(); err != nil {
		return nil, err
	}
	wire := sourceCodeLocatorJSON{
		AlgorithmVersion: locator.AlgorithmVersion, Repository: locator.Repository.String(),
		Permalink: locator.Permalink.String(), Commit: locator.Commit, Path: locator.Path,
		StartLine: locator.StartLine, EndLine: locator.EndLine, Symbol: locator.Symbol,
		VersionScope: locator.VersionScope.String(),
	}
	if locator.License != nil {
		wire.License = &sourceCodeLicenseJSON{Identifier: locator.License.Identifier, Name: locator.License.Name}
		if locator.License.Locator != nil {
			wire.License.Locator = locator.License.Locator.String()
		}
	}
	encoded, err := json.Marshal(wire)
	if err != nil {
		return nil, fmt.Errorf("encode source code locator: %w", err)
	}
	if len(encoded) > MaximumSourceCodeLocatorJSONBytes {
		return nil, fmt.Errorf("source code locator JSON exceeds %d bytes", MaximumSourceCodeLocatorJSONBytes)
	}
	return encoded, nil
}

func ParseSourceCodeLocator(data []byte) (SourceCodeLocator, error) {
	if len(data) == 0 || len(data) > MaximumSourceCodeLocatorJSONBytes {
		return SourceCodeLocator{}, fmt.Errorf("source code locator JSON size is invalid")
	}
	decoder := json.NewDecoder(bytes.NewReader(data))
	decoder.DisallowUnknownFields()
	var wire sourceCodeLocatorJSON
	if err := decoder.Decode(&wire); err != nil {
		return SourceCodeLocator{}, fmt.Errorf("decode source code locator: %w", err)
	}
	if err := ensureSourceCodeJSONEOF(decoder); err != nil {
		return SourceCodeLocator{}, err
	}
	repository, err := NewSourceLocator(wire.Repository)
	if err != nil {
		return SourceCodeLocator{}, fmt.Errorf("source code repository: %w", err)
	}
	permalink, err := NewSourceLocator(wire.Permalink)
	if err != nil {
		return SourceCodeLocator{}, fmt.Errorf("source code permalink: %w", err)
	}
	version, err := NewSourceVersion(wire.VersionScope)
	if err != nil {
		return SourceCodeLocator{}, fmt.Errorf("source code version scope: %w", err)
	}
	locator := SourceCodeLocator{
		Repository: repository, Permalink: permalink, Commit: wire.Commit, Path: wire.Path,
		StartLine: wire.StartLine, EndLine: wire.EndLine, Symbol: wire.Symbol,
		VersionScope: version, AlgorithmVersion: wire.AlgorithmVersion,
	}
	if wire.License != nil {
		license := &SourceCodeLicense{Identifier: wire.License.Identifier, Name: wire.License.Name}
		if wire.License.Locator != "" {
			licenseLocator, locatorErr := NewSourceLocator(wire.License.Locator)
			if locatorErr != nil {
				return SourceCodeLocator{}, fmt.Errorf("source code license locator: %w", locatorErr)
			}
			license.Locator = &licenseLocator
		}
		locator.License = license
	}
	if err := locator.Validate(); err != nil {
		return SourceCodeLocator{}, err
	}
	return locator, nil
}

func ensureSourceCodeJSONEOF(decoder *json.Decoder) error {
	var trailing any
	if err := decoder.Decode(&trailing); err != io.EOF {
		if err == nil {
			return fmt.Errorf("source code locator JSON contains trailing data")
		}
		return fmt.Errorf("decode source code locator trailing data: %w", err)
	}
	return nil
}
