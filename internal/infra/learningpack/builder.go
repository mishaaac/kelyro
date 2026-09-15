package learningpack

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"sort"
	"strings"
	"unicode/utf8"

	"github.com/mishaaac/kelyro/internal/curriculum"
	curriculumapp "github.com/mishaaac/kelyro/internal/curriculum/application"
)

const (
	EvidenceMarkdownName  = "EVIDENCE.md"
	AssetLicensesName     = "assets/licenses.json"
	AssetLicenseSchemaV1  = "pack-asset-licenses/v1"
	AssetAuthorshipKelyro = "kelyro_authored"
)

type Builder struct {
	reporter curriculumapp.CurriculumEvidenceReportService
}

func NewBuilder() *Builder {
	return &Builder{reporter: curriculumapp.NewCurriculumEvidenceReporterV1()}
}

func (builder *Builder) Build(ctx context.Context, request curriculumapp.PackBuildRequest) (curriculumapp.PackBuildResult, error) {
	const operation = "build Learning Pack"
	if err := ctx.Err(); err != nil {
		return curriculumapp.PackBuildResult{}, curriculumapp.ExternalError(operation, err)
	}
	if builder == nil || builder.reporter == nil {
		return curriculumapp.PackBuildResult{}, curriculumapp.RequireDependency(operation, "evidence reporter", nil)
	}
	if err := validateBuildRequest(request); err != nil {
		return curriculumapp.PackBuildResult{}, curriculumapp.Invalid(operation, err)
	}
	report, err := builder.reporter.Generate(ctx, curriculumapp.CurriculumEvidenceReportRequest{
		Compilation: request.Compilation, EvidenceSets: request.EvidenceSets, Citations: request.Citations,
	})
	if err != nil {
		return curriculumapp.PackBuildResult{}, err
	}

	entries, err := encodePackEntries(request, report)
	if err != nil {
		return curriculumapp.PackBuildResult{}, curriculumapp.Invalid(operation, err)
	}
	writeChecksums(entries)
	pack, _, err := validateEntries(entries)
	if err != nil {
		return curriculumapp.PackBuildResult{}, curriculumapp.Invalid(operation, fmt.Errorf("validate built pack: %w", err))
	}
	archive, err := canonicalArchive(entries)
	if err != nil {
		return curriculumapp.PackBuildResult{}, curriculumapp.ExternalError(operation, err)
	}
	digest := sha256.Sum256(entries[ChecksumsName])
	names := make([]string, 0, len(entries))
	for name := range entries {
		names = append(names, name)
	}
	sort.Strings(names)
	return curriculumapp.PackBuildResult{
		Pack: pack, ContentHash: "sha256:" + hex.EncodeToString(digest[:]), PortableArchive: archive, Entries: names,
	}, nil
}

func validateBuildRequest(request curriculumapp.PackBuildRequest) error {
	if err := request.Manifest.Validate(); err != nil {
		return err
	}
	if request.Manifest.SchemaVersion != SchemaVersionV1 {
		return fmt.Errorf("unsupported pack schema version %q", request.Manifest.SchemaVersion)
	}
	entryFields := []struct{ name, value string }{
		{"curriculum_entry", request.Manifest.CurriculumEntry},
		{"source_evidence_entry", request.Manifest.SourceEvidenceEntry},
		{"build_info_entry", request.Manifest.BuildInfoEntry},
	}
	if request.Manifest.EnvironmentEntry != "" {
		entryFields = append(entryFields, struct{ name, value string }{"environment_pack", request.Manifest.EnvironmentEntry})
	}
	reserved := map[string]struct{}{ManifestName: {}, ChecksumsName: {}, EvidenceMarkdownName: {}, AssetLicensesName: {}, "README.md": {}, "LICENSE": {}}
	for _, entry := range entryFields {
		if err := validatePortablePath(entry.name, entry.value); err != nil {
			return err
		}
		if _, exists := reserved[entry.value]; exists {
			return fmt.Errorf("%s conflicts with reserved entry %q", entry.name, entry.value)
		}
		reserved[entry.value] = struct{}{}
	}
	if err := request.Compilation.Validate(); err != nil {
		return err
	}
	if request.Compilation.BuildInfo == nil {
		return fmt.Errorf("compilation has no reproducibility metadata")
	}
	if request.Manifest.CurriculumID != request.Compilation.Curriculum.ID {
		return fmt.Errorf("manifest curriculum does not match compilation")
	}
	if request.Manifest.SchemaVersion != request.Compilation.BuildInfo.PackSchemaVersion {
		return fmt.Errorf("manifest schema does not match build info")
	}
	if request.Manifest.CreatedAt != request.Compilation.BuildInfo.BuiltAt {
		return fmt.Errorf("manifest creation time does not match build time")
	}
	if request.Environment == nil && request.Manifest.EnvironmentEntry != "" {
		return fmt.Errorf("manifest declares an environment entry without an environment pack")
	}
	if request.Environment != nil {
		if request.Manifest.EnvironmentEntry == "" {
			return fmt.Errorf("environment pack requires a manifest entry")
		}
		if err := request.Environment.ValidatePortableV1(); err != nil {
			return err
		}
	}
	if err := validateBuildCitations(request.EvidenceSets, request.Citations); err != nil {
		return err
	}
	coreEntries := 9
	if request.Environment != nil {
		coreEntries++
	}
	if len(request.Assets)+coreEntries > MaximumPackEntries {
		return fmt.Errorf("pack exceeds %d file entries", MaximumPackEntries)
	}
	seenAssets := make(map[string]struct{}, len(request.Assets))
	for _, asset := range request.Assets {
		if err := validateBuildAsset(asset); err != nil {
			return err
		}
		if _, exists := seenAssets[asset.Path]; exists {
			return fmt.Errorf("duplicate pack asset %q", asset.Path)
		}
		seenAssets[asset.Path] = struct{}{}
	}
	return nil
}

func validateBuildCitations(sets []curriculum.CurriculumEvidenceSet, citations []curriculum.EvidenceReportCitation) error {
	required := make(map[curriculum.ID]struct{})
	for _, set := range sets {
		for _, source := range set.SourceAuthority {
			required[source.SourceID] = struct{}{}
		}
	}
	seen := make(map[curriculum.ID]struct{}, len(citations))
	for _, citation := range citations {
		if err := citation.Validate(); err != nil {
			return err
		}
		if _, exists := required[citation.SourceID]; !exists {
			return fmt.Errorf("citation source %q is outside frozen evidence", citation.SourceID)
		}
		if _, exists := seen[citation.SourceID]; exists {
			return fmt.Errorf("duplicate citation source %q", citation.SourceID)
		}
		seen[citation.SourceID] = struct{}{}
	}
	for sourceID := range required {
		if _, exists := seen[sourceID]; !exists {
			return fmt.Errorf("source %q has no citation URL", sourceID)
		}
	}
	return nil
}

func validateBuildAsset(asset curriculumapp.PackBuildAsset) error {
	if err := validatePortablePath("asset", asset.Path); err != nil {
		return err
	}
	if !strings.HasPrefix(asset.Path, "assets/") || asset.Path == AssetLicensesName {
		return fmt.Errorf("asset %q must be below assets/ and not use the license ledger path", asset.Path)
	}
	if !asset.KelyroAuthored {
		return fmt.Errorf("asset %q is not declared Kelyro-authored original content", asset.Path)
	}
	if strings.TrimSpace(asset.License) == "" || strings.TrimSpace(asset.CopyrightHolder) == "" || asset.License != strings.TrimSpace(asset.License) || asset.CopyrightHolder != strings.TrimSpace(asset.CopyrightHolder) {
		return fmt.Errorf("asset %q requires license and copyright holder", asset.Path)
	}
	if !utf8.Valid(asset.Content) || len(asset.Content) > MaximumPackFileBytes {
		return fmt.Errorf("asset %q must be valid UTF-8 within %d bytes", asset.Path, MaximumPackFileBytes)
	}
	return validateEntryName(asset.Path)
}

func writeChecksums(entries map[string][]byte) {
	names := make([]string, 0, len(entries))
	for name := range entries {
		if name != ChecksumsName {
			names = append(names, name)
		}
	}
	sort.Strings(names)
	var lines strings.Builder
	for _, name := range names {
		digest := sha256.Sum256(entries[name])
		fmt.Fprintf(&lines, "%s  %s\n", hex.EncodeToString(digest[:]), name)
	}
	entries[ChecksumsName] = []byte(lines.String())
}

type assetLicenseDocument struct {
	SchemaVersion string                      `json:"schema_version"`
	Assets        []assetLicenseEntryDocument `json:"assets"`
}

type assetLicenseEntryDocument struct {
	Path            string `json:"path"`
	License         string `json:"license"`
	CopyrightHolder string `json:"copyright_holder"`
	Authorship      string `json:"authorship"`
	ContentHash     string `json:"content_hash"`
}

func encodeAssetLicenses(assets []curriculumapp.PackBuildAsset) ([]byte, error) {
	document := assetLicenseDocument{SchemaVersion: AssetLicenseSchemaV1, Assets: make([]assetLicenseEntryDocument, 0, len(assets))}
	for _, asset := range assets {
		digest := sha256.Sum256(asset.Content)
		document.Assets = append(document.Assets, assetLicenseEntryDocument{Path: asset.Path, License: asset.License, CopyrightHolder: asset.CopyrightHolder, Authorship: AssetAuthorshipKelyro, ContentHash: "sha256:" + hex.EncodeToString(digest[:])})
	}
	sort.Slice(document.Assets, func(i, j int) bool { return document.Assets[i].Path < document.Assets[j].Path })
	return json.MarshalIndent(document, "", "  ")
}

var _ curriculumapp.PackBuildService = (*Builder)(nil)
