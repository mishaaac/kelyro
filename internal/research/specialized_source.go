package research

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"strings"
	"unicode"
)

const (
	SpecializedSourceMetadataV1       = "specialized-source-metadata-v1"
	MaximumSpecializedSourceJSONBytes = 8192
	maximumSpecializedTextBytes       = 512
)

type SourceAffiliation string

const (
	SourceAffiliationOfficial  SourceAffiliation = "official"
	SourceAffiliationCommunity SourceAffiliation = "community"
)

func (affiliation SourceAffiliation) Validate() error {
	switch affiliation {
	case SourceAffiliationOfficial, SourceAffiliationCommunity:
		return nil
	default:
		return fmt.Errorf("invalid specialized source affiliation %q", affiliation)
	}
}

type StandardStatus string

const (
	StandardDraft      StandardStatus = "draft"
	StandardActive     StandardStatus = "active"
	StandardSuperseded StandardStatus = "superseded"
	StandardWithdrawn  StandardStatus = "withdrawn"
	StandardDeprecated StandardStatus = "deprecated"
	StandardUnknown    StandardStatus = "unknown"
)

func (status StandardStatus) Validate() error {
	switch status {
	case StandardDraft, StandardActive, StandardSuperseded,
		StandardWithdrawn, StandardDeprecated, StandardUnknown:
		return nil
	default:
		return fmt.Errorf("invalid standard status %q", status)
	}
}

type PlaygroundDetails struct {
	Interactive      bool
	LanguageRuntime  string
	Version          *SourceVersion
	Affiliation      SourceAffiliation
	ShareableLocator SourceLocator
}

func (details PlaygroundDetails) Validate() error {
	if !details.Interactive {
		return fmt.Errorf("playground must be interactive")
	}
	if err := validateSpecializedText("playground language/runtime", details.LanguageRuntime, false); err != nil {
		return err
	}
	if details.Version != nil {
		if err := details.Version.Validate(); err != nil {
			return fmt.Errorf("playground version: %w", err)
		}
	}
	if err := details.Affiliation.Validate(); err != nil {
		return err
	}
	if err := details.ShareableLocator.Validate(); err != nil {
		return fmt.Errorf("playground shareable locator: %w", err)
	}
	return nil
}

type PackageReferenceDetails struct {
	PackageModule        string
	Symbol               string
	Version              *SourceVersion
	CanonicalDocsLocator SourceLocator
	SourceCodeLocator    *SourceLocator
}

func (details PackageReferenceDetails) Validate() error {
	if err := validateSpecializedText("package/module", details.PackageModule, false); err != nil {
		return err
	}
	if err := validateSpecializedText("package symbol", details.Symbol, true); err != nil {
		return err
	}
	if details.Version != nil {
		if err := details.Version.Validate(); err != nil {
			return fmt.Errorf("package reference version: %w", err)
		}
	}
	if err := details.CanonicalDocsLocator.Validate(); err != nil {
		return fmt.Errorf("package reference canonical docs locator: %w", err)
	}
	if details.SourceCodeLocator != nil {
		if err := details.SourceCodeLocator.Validate(); err != nil {
			return fmt.Errorf("package reference source code locator: %w", err)
		}
	}
	return nil
}

type StandardDetails struct {
	StandardsBody   string
	StandardID      string
	Revision        *SourceVersion
	Status          StandardStatus
	OfficialLocator SourceLocator
}

func (details StandardDetails) Validate() error {
	if err := validateSpecializedText("standards body", details.StandardsBody, false); err != nil {
		return err
	}
	if err := validateSpecializedText("standard ID", details.StandardID, false); err != nil {
		return err
	}
	if details.Revision != nil {
		if err := details.Revision.Validate(); err != nil {
			return fmt.Errorf("standard revision: %w", err)
		}
	}
	if err := details.Status.Validate(); err != nil {
		return err
	}
	if err := details.OfficialLocator.Validate(); err != nil {
		return fmt.Errorf("standard official locator: %w", err)
	}
	return nil
}

// SourceSpecialization is a closed union. Exactly one details record must be
// present and its kind must match the owning Source.
type SourceSpecialization struct {
	Kind             SourceKind
	Playground       *PlaygroundDetails
	PackageReference *PackageReferenceDetails
	Standard         *StandardDetails
	AlgorithmVersion string
}

func (specialization SourceSpecialization) Validate() error {
	if specialization.AlgorithmVersion != SpecializedSourceMetadataV1 {
		return fmt.Errorf("specialized source algorithm version must be %q", SpecializedSourceMetadataV1)
	}
	count := 0
	if specialization.Playground != nil {
		count++
		if err := specialization.Playground.Validate(); err != nil {
			return err
		}
	}
	if specialization.PackageReference != nil {
		count++
		if err := specialization.PackageReference.Validate(); err != nil {
			return err
		}
	}
	if specialization.Standard != nil {
		count++
		if err := specialization.Standard.Validate(); err != nil {
			return err
		}
	}
	if count != 1 {
		return fmt.Errorf("specialized source must contain exactly one details record")
	}
	switch specialization.Kind {
	case SourcePlayground:
		if specialization.Playground == nil {
			return fmt.Errorf("playground specialization requires playground details")
		}
	case SourcePackageReference:
		if specialization.PackageReference == nil {
			return fmt.Errorf("package reference specialization requires package details")
		}
	case SourceStandard:
		if specialization.Standard == nil {
			return fmt.Errorf("standard specialization requires standard details")
		}
	default:
		return fmt.Errorf("source kind %q does not support specialized metadata", specialization.Kind)
	}
	return nil
}

func (specialization SourceSpecialization) Clone() *SourceSpecialization {
	clone := specialization
	if specialization.Playground != nil {
		details := *specialization.Playground
		details.Version = cloneSpecializedVersion(details.Version)
		clone.Playground = &details
	}
	if specialization.PackageReference != nil {
		details := *specialization.PackageReference
		details.Version = cloneSpecializedVersion(details.Version)
		if details.SourceCodeLocator != nil {
			locator := *details.SourceCodeLocator
			details.SourceCodeLocator = &locator
		}
		clone.PackageReference = &details
	}
	if specialization.Standard != nil {
		details := *specialization.Standard
		details.Revision = cloneSpecializedVersion(details.Revision)
		clone.Standard = &details
	}
	return &clone
}

func validateSourceSpecialization(source Source) error {
	if source.Specialization == nil {
		if source.Kind == SourcePlayground {
			return fmt.Errorf("playground source requires specialized metadata")
		}
		return nil
	}
	if err := source.Specialization.Validate(); err != nil {
		return err
	}
	if source.Specialization.Kind != source.Kind {
		return fmt.Errorf("specialized source kind does not match source kind")
	}
	switch source.Kind {
	case SourcePlayground:
		return validateSpecializedVersion("playground", source.Version, source.Specialization.Playground.Version)
	case SourcePackageReference:
		if source.Locator != source.Specialization.PackageReference.CanonicalDocsLocator {
			return fmt.Errorf("package reference locator must be its canonical docs locator")
		}
		return validateSpecializedVersion("package reference", source.Version, source.Specialization.PackageReference.Version)
	case SourceStandard:
		if source.Locator != source.Specialization.Standard.OfficialLocator {
			return fmt.Errorf("standard locator must be its official locator")
		}
		return validateSpecializedVersion("standard", source.Version, source.Specialization.Standard.Revision)
	default:
		return fmt.Errorf("source kind %q cannot carry specialized metadata", source.Kind)
	}
}

func validateSpecializedVersion(name string, sourceVersion, detailsVersion *SourceVersion) error {
	if (sourceVersion == nil) != (detailsVersion == nil) {
		return fmt.Errorf("%s source and specialized versions must both be known or unknown", name)
	}
	if sourceVersion != nil && *sourceVersion != *detailsVersion {
		return fmt.Errorf("%s source and specialized versions do not match", name)
	}
	return nil
}

type sourceSpecializationJSON struct {
	Kind             string                       `json:"kind"`
	Playground       *playgroundDetailsJSON       `json:"playground,omitempty"`
	PackageReference *packageReferenceDetailsJSON `json:"package_reference,omitempty"`
	Standard         *standardDetailsJSON         `json:"standard,omitempty"`
	AlgorithmVersion string                       `json:"algorithm_version"`
}

type playgroundDetailsJSON struct {
	Interactive      bool   `json:"interactive"`
	LanguageRuntime  string `json:"language_runtime"`
	Version          string `json:"version,omitempty"`
	Affiliation      string `json:"affiliation"`
	ShareableLocator string `json:"shareable_locator"`
}

type packageReferenceDetailsJSON struct {
	PackageModule        string `json:"package_module"`
	Symbol               string `json:"symbol,omitempty"`
	Version              string `json:"version,omitempty"`
	CanonicalDocsLocator string `json:"canonical_docs_locator"`
	SourceCodeLocator    string `json:"source_code_locator,omitempty"`
}

type standardDetailsJSON struct {
	StandardsBody   string `json:"standards_body"`
	StandardID      string `json:"standard_id"`
	Revision        string `json:"revision,omitempty"`
	Status          string `json:"status"`
	OfficialLocator string `json:"official_locator"`
}

func EncodeSourceSpecialization(specialization SourceSpecialization) ([]byte, error) {
	if err := specialization.Validate(); err != nil {
		return nil, err
	}
	payload := sourceSpecializationJSON{Kind: string(specialization.Kind), AlgorithmVersion: specialization.AlgorithmVersion}
	if details := specialization.Playground; details != nil {
		payload.Playground = &playgroundDetailsJSON{
			Interactive: details.Interactive, LanguageRuntime: details.LanguageRuntime,
			Version: optionalSpecializedVersion(details.Version), Affiliation: string(details.Affiliation),
			ShareableLocator: details.ShareableLocator.String(),
		}
	}
	if details := specialization.PackageReference; details != nil {
		payload.PackageReference = &packageReferenceDetailsJSON{
			PackageModule: details.PackageModule, Symbol: details.Symbol,
			Version: optionalSpecializedVersion(details.Version), CanonicalDocsLocator: details.CanonicalDocsLocator.String(),
		}
		if details.SourceCodeLocator != nil {
			payload.PackageReference.SourceCodeLocator = details.SourceCodeLocator.String()
		}
	}
	if details := specialization.Standard; details != nil {
		payload.Standard = &standardDetailsJSON{
			StandardsBody: details.StandardsBody, StandardID: details.StandardID,
			Revision: optionalSpecializedVersion(details.Revision), Status: string(details.Status),
			OfficialLocator: details.OfficialLocator.String(),
		}
	}
	encoded, err := json.Marshal(payload)
	if err != nil {
		return nil, fmt.Errorf("encode specialized source metadata: %w", err)
	}
	if len(encoded) > MaximumSpecializedSourceJSONBytes {
		return nil, fmt.Errorf("specialized source metadata exceeds %d bytes", MaximumSpecializedSourceJSONBytes)
	}
	return encoded, nil
}

func ParseSourceSpecialization(encoded []byte) (SourceSpecialization, error) {
	if len(encoded) == 0 || len(encoded) > MaximumSpecializedSourceJSONBytes {
		return SourceSpecialization{}, fmt.Errorf("specialized source metadata size must be between 1 and %d bytes", MaximumSpecializedSourceJSONBytes)
	}
	decoder := json.NewDecoder(bytes.NewReader(encoded))
	decoder.DisallowUnknownFields()
	var payload sourceSpecializationJSON
	if err := decoder.Decode(&payload); err != nil {
		return SourceSpecialization{}, fmt.Errorf("decode specialized source metadata: %w", err)
	}
	var trailing any
	if err := decoder.Decode(&trailing); err != io.EOF {
		return SourceSpecialization{}, fmt.Errorf("decode specialized source metadata: trailing data")
	}
	specialization, err := sourceSpecializationFromJSON(payload)
	if err != nil {
		return SourceSpecialization{}, err
	}
	canonical, err := EncodeSourceSpecialization(specialization)
	if err != nil {
		return SourceSpecialization{}, err
	}
	if !bytes.Equal(encoded, canonical) {
		return SourceSpecialization{}, fmt.Errorf("specialized source metadata is not canonical")
	}
	return specialization, nil
}

func sourceSpecializationFromJSON(payload sourceSpecializationJSON) (SourceSpecialization, error) {
	specialization := SourceSpecialization{Kind: SourceKind(payload.Kind), AlgorithmVersion: payload.AlgorithmVersion}
	if payload.Playground != nil {
		version, err := parseOptionalSpecializedVersion(payload.Playground.Version)
		if err != nil {
			return SourceSpecialization{}, err
		}
		locator, err := NewSourceLocator(payload.Playground.ShareableLocator)
		if err != nil {
			return SourceSpecialization{}, err
		}
		specialization.Playground = &PlaygroundDetails{
			Interactive: payload.Playground.Interactive, LanguageRuntime: payload.Playground.LanguageRuntime,
			Version: version, Affiliation: SourceAffiliation(payload.Playground.Affiliation), ShareableLocator: locator,
		}
	}
	if payload.PackageReference != nil {
		version, err := parseOptionalSpecializedVersion(payload.PackageReference.Version)
		if err != nil {
			return SourceSpecialization{}, err
		}
		docs, err := NewSourceLocator(payload.PackageReference.CanonicalDocsLocator)
		if err != nil {
			return SourceSpecialization{}, err
		}
		var sourceCode *SourceLocator
		if payload.PackageReference.SourceCodeLocator != "" {
			locator, locatorErr := NewSourceLocator(payload.PackageReference.SourceCodeLocator)
			if locatorErr != nil {
				return SourceSpecialization{}, locatorErr
			}
			sourceCode = &locator
		}
		specialization.PackageReference = &PackageReferenceDetails{
			PackageModule: payload.PackageReference.PackageModule, Symbol: payload.PackageReference.Symbol,
			Version: version, CanonicalDocsLocator: docs, SourceCodeLocator: sourceCode,
		}
	}
	if payload.Standard != nil {
		revision, err := parseOptionalSpecializedVersion(payload.Standard.Revision)
		if err != nil {
			return SourceSpecialization{}, err
		}
		locator, err := NewSourceLocator(payload.Standard.OfficialLocator)
		if err != nil {
			return SourceSpecialization{}, err
		}
		specialization.Standard = &StandardDetails{
			StandardsBody: payload.Standard.StandardsBody, StandardID: payload.Standard.StandardID,
			Revision: revision, Status: StandardStatus(payload.Standard.Status), OfficialLocator: locator,
		}
	}
	if err := specialization.Validate(); err != nil {
		return SourceSpecialization{}, err
	}
	return specialization, nil
}

func validateSpecializedText(name, value string, optional bool) error {
	if optional && value == "" {
		return nil
	}
	if err := requireText(name, value); err != nil {
		return err
	}
	if len(value) > maximumSpecializedTextBytes || strings.IndexFunc(value, unicode.IsControl) >= 0 {
		return fmt.Errorf("%s exceeds its bounded text contract or contains control characters", name)
	}
	return nil
}

func optionalSpecializedVersion(version *SourceVersion) string {
	if version == nil {
		return ""
	}
	return version.String()
}

func parseOptionalSpecializedVersion(value string) (*SourceVersion, error) {
	if value == "" {
		return nil, nil
	}
	version, err := NewSourceVersion(value)
	if err != nil {
		return nil, err
	}
	return &version, nil
}

func cloneSpecializedVersion(version *SourceVersion) *SourceVersion {
	if version == nil {
		return nil
	}
	clone := *version
	return &clone
}
