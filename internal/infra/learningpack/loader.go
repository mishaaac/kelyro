package learningpack

import (
	"archive/zip"
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/mishaaac/kelyro/internal/curriculum"
	curriculumapp "github.com/mishaaac/kelyro/internal/curriculum/application"
)

const (
	MaximumPackEntries      = 1024
	MaximumPackFileBytes    = 4 << 20
	MaximumPackTotalBytes   = 32 << 20
	MaximumCompressionRatio = 100
)

var forbiddenExtensions = map[string]struct{}{
	".bat": {}, ".bash": {}, ".cmd": {}, ".com": {}, ".dll": {}, ".dylib": {},
	".exe": {}, ".js": {}, ".msi": {}, ".ps1": {}, ".py": {}, ".sh": {}, ".so": {}, ".zsh": {},
}

// Validator loads and validates untrusted v1 directories and ZIP files. It is
// read-only and never follows links, executes content, installs packs, or uses
// the network.
type Validator struct{}

func NewValidator() *Validator { return &Validator{} }

func (validator *Validator) Validate(ctx context.Context, source curriculumapp.PackSource) (curriculumapp.PackValidationResult, error) {
	if err := ctx.Err(); err != nil {
		return curriculumapp.PackValidationResult{}, err
	}
	if strings.TrimSpace(source.Path) == "" {
		return invalidResult("source", "pack path is empty"), nil
	}
	entries, err := loadPackEntries(ctx, source.Path)
	if err != nil {
		return invalidResult("source", err.Error()), nil
	}
	pack, warnings, err := validateEntries(entries)
	if err != nil {
		return invalidResult("pack", err.Error()), nil
	}
	archive, err := canonicalArchive(entries)
	if err != nil {
		return curriculumapp.PackValidationResult{}, fmt.Errorf("snapshot validated pack: %w", err)
	}
	digest := sha256.Sum256(entries[ChecksumsName])
	return curriculumapp.PackValidationResult{
		Pack: &pack, ContentHash: "sha256:" + hex.EncodeToString(digest[:]),
		PortableArchive: archive, Warnings: warnings,
	}, nil
}

func invalidResult(path, message string) curriculumapp.PackValidationResult {
	return curriculumapp.PackValidationResult{Errors: []curriculumapp.PackValidationIssue{{Code: "invalid_pack", Path: path, Message: message}}}
}

func loadPackEntries(ctx context.Context, source string) (map[string][]byte, error) {
	info, err := os.Lstat(source)
	if err != nil {
		return nil, fmt.Errorf("open pack: %w", err)
	}
	if info.Mode()&os.ModeSymlink != 0 {
		return nil, fmt.Errorf("pack root must not be a symlink")
	}
	if info.IsDir() {
		return loadDirectory(ctx, source)
	}
	if !info.Mode().IsRegular() {
		return nil, fmt.Errorf("pack source is neither a directory nor a regular ZIP file")
	}
	return loadZIP(ctx, source)
}

func loadDirectory(ctx context.Context, root string) (map[string][]byte, error) {
	rootAbs, err := filepath.Abs(root)
	if err != nil {
		return nil, err
	}
	entries := make(map[string][]byte)
	total := int64(0)
	err = filepath.WalkDir(rootAbs, func(name string, item fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if err := ctx.Err(); err != nil {
			return err
		}
		if name == rootAbs {
			return nil
		}
		if item.Type()&os.ModeSymlink != 0 {
			return fmt.Errorf("symlink entry %q is not allowed", name)
		}
		if item.IsDir() {
			return nil
		}
		info, err := item.Info()
		if err != nil {
			return err
		}
		if !info.Mode().IsRegular() {
			return fmt.Errorf("non-regular entry %q is not allowed", name)
		}
		relative, err := filepath.Rel(rootAbs, name)
		if err != nil {
			return err
		}
		portable := filepath.ToSlash(relative)
		if err := validateEntryName(portable); err != nil {
			return err
		}
		if info.Mode().Perm()&0111 != 0 {
			return fmt.Errorf("executable entry %q is not allowed", portable)
		}
		if info.Size() > MaximumPackFileBytes {
			return fmt.Errorf("entry %q exceeds %d bytes", portable, MaximumPackFileBytes)
		}
		total += info.Size()
		if total > MaximumPackTotalBytes {
			return fmt.Errorf("pack exceeds %d uncompressed bytes", MaximumPackTotalBytes)
		}
		if len(entries) >= MaximumPackEntries {
			return fmt.Errorf("pack exceeds %d file entries", MaximumPackEntries)
		}
		resolved, err := filepath.EvalSymlinks(name)
		if err != nil {
			return err
		}
		resolvedAbs, err := filepath.Abs(resolved)
		if err != nil {
			return err
		}
		inside, err := filepath.Rel(rootAbs, resolvedAbs)
		if err != nil || inside == ".." || strings.HasPrefix(inside, ".."+string(filepath.Separator)) {
			return fmt.Errorf("entry %q escapes pack root", portable)
		}
		encoded, err := readBoundedFile(resolvedAbs)
		if err != nil {
			return fmt.Errorf("read entry %q: %w", portable, err)
		}
		entries[portable] = encoded
		return nil
	})
	if err != nil {
		return nil, fmt.Errorf("load pack directory: %w", err)
	}
	return entries, nil
}

func loadZIP(ctx context.Context, source string) (map[string][]byte, error) {
	archive, err := zip.OpenReader(source)
	if err != nil {
		return nil, fmt.Errorf("open pack ZIP: %w", err)
	}
	defer archive.Close()
	if len(archive.File) > MaximumPackEntries {
		return nil, fmt.Errorf("pack exceeds %d archive entries", MaximumPackEntries)
	}
	entries := make(map[string][]byte)
	seen := make(map[string]struct{}, len(archive.File))
	total := uint64(0)
	for _, item := range archive.File {
		if err := ctx.Err(); err != nil {
			return nil, err
		}
		name := strings.TrimSuffix(item.Name, "/")
		if name == "" {
			continue
		}
		if err := validateEntryName(name); err != nil {
			return nil, err
		}
		if _, exists := seen[name]; exists {
			return nil, fmt.Errorf("duplicate archive entry %q", name)
		}
		seen[name] = struct{}{}
		mode := item.Mode()
		if mode&os.ModeSymlink != 0 {
			return nil, fmt.Errorf("symlink archive entry %q is not allowed", name)
		}
		if item.FileInfo().IsDir() {
			continue
		}
		if !mode.IsRegular() {
			return nil, fmt.Errorf("non-regular archive entry %q is not allowed", name)
		}
		if mode.Perm()&0111 != 0 {
			return nil, fmt.Errorf("executable archive entry %q is not allowed", name)
		}
		if item.UncompressedSize64 > MaximumPackFileBytes {
			return nil, fmt.Errorf("entry %q exceeds %d bytes", name, MaximumPackFileBytes)
		}
		total += item.UncompressedSize64
		if total > MaximumPackTotalBytes {
			return nil, fmt.Errorf("pack exceeds %d uncompressed bytes", MaximumPackTotalBytes)
		}
		if item.UncompressedSize64 > 0 && (item.CompressedSize64 == 0 || item.UncompressedSize64/item.CompressedSize64 > MaximumCompressionRatio) {
			return nil, fmt.Errorf("entry %q exceeds compression ratio limit", name)
		}
		reader, err := item.Open()
		if err != nil {
			return nil, fmt.Errorf("open archive entry %q: %w", name, err)
		}
		encoded, readErr := io.ReadAll(io.LimitReader(reader, MaximumPackFileBytes+1))
		closeErr := reader.Close()
		if readErr != nil {
			return nil, fmt.Errorf("read archive entry %q: %w", name, readErr)
		}
		if closeErr != nil {
			return nil, fmt.Errorf("close archive entry %q: %w", name, closeErr)
		}
		if len(encoded) > MaximumPackFileBytes {
			return nil, fmt.Errorf("entry %q exceeds %d bytes", name, MaximumPackFileBytes)
		}
		entries[name] = encoded
	}
	return entries, nil
}

func readBoundedFile(name string) ([]byte, error) {
	file, err := os.Open(name)
	if err != nil {
		return nil, err
	}
	defer file.Close()
	encoded, err := io.ReadAll(io.LimitReader(file, MaximumPackFileBytes+1))
	if err != nil {
		return nil, err
	}
	if len(encoded) > MaximumPackFileBytes {
		return nil, fmt.Errorf("file exceeds %d bytes", MaximumPackFileBytes)
	}
	return encoded, nil
}

func canonicalArchive(entries map[string][]byte) ([]byte, error) {
	var encoded bytes.Buffer
	archive := zip.NewWriter(&encoded)
	names := make([]string, 0, len(entries))
	for name := range entries {
		names = append(names, name)
	}
	sort.Strings(names)
	for _, name := range names {
		header := &zip.FileHeader{Name: name, Method: zip.Store}
		header.SetMode(0o644)
		header.Modified = time.Unix(0, 0).UTC()
		writer, err := archive.CreateHeader(header)
		if err != nil {
			_ = archive.Close()
			return nil, err
		}
		if _, err := writer.Write(entries[name]); err != nil {
			_ = archive.Close()
			return nil, err
		}
	}
	if err := archive.Close(); err != nil {
		return nil, err
	}
	return encoded.Bytes(), nil
}

func validateEntryName(name string) error {
	if err := validatePortablePath("entry", name); err != nil {
		return err
	}
	if _, forbidden := forbiddenExtensions[strings.ToLower(filepath.Ext(name))]; forbidden {
		return fmt.Errorf("script or executable entry %q is not allowed", name)
	}
	return nil
}

func validateEntries(entries map[string][]byte) (curriculum.LearningPack, []curriculumapp.PackValidationIssue, error) {
	for _, required := range []string{ManifestName, ChecksumsName} {
		if _, exists := entries[required]; !exists {
			return curriculum.LearningPack{}, nil, fmt.Errorf("required entry %q is missing", required)
		}
	}
	for name, encoded := range entries {
		if !utf8.Valid(encoded) {
			return curriculum.LearningPack{}, nil, fmt.Errorf("entry %q is not valid UTF-8", name)
		}
	}
	if err := validateChecksums(entries); err != nil {
		return curriculum.LearningPack{}, nil, err
	}
	manifest, err := ParseManifest(bytes.NewReader(entries[ManifestName]))
	if err != nil {
		return curriculum.LearningPack{}, nil, err
	}
	for _, required := range []string{manifest.CurriculumEntry, manifest.SourceEvidenceEntry} {
		if _, exists := entries[required]; !exists {
			return curriculum.LearningPack{}, nil, fmt.Errorf("manifest entry %q is missing", required)
		}
	}
	curriculumDefinition, err := decodeCurriculum(entries[manifest.CurriculumEntry])
	if err != nil {
		return curriculum.LearningPack{}, nil, err
	}
	if curriculumDefinition.ID != manifest.CurriculumID {
		return curriculum.LearningPack{}, nil, fmt.Errorf("manifest curriculum_id does not match curriculum entry")
	}
	report, err := decodeEvidenceReport(entries[manifest.SourceEvidenceEntry])
	if err != nil {
		return curriculum.LearningPack{}, nil, err
	}
	pack := curriculum.LearningPack{Manifest: manifest, Curriculum: curriculumDefinition}
	if manifest.EnvironmentEntry != "" {
		encoded, exists := entries[manifest.EnvironmentEntry]
		if !exists {
			return curriculum.LearningPack{}, nil, fmt.Errorf("manifest environment entry %q is missing", manifest.EnvironmentEntry)
		}
		environment, err := decodeEnvironment(encoded)
		if err != nil {
			return curriculum.LearningPack{}, nil, err
		}
		pack.Environment = &environment
	}
	if err := validateEvidenceReport(report, curriculumDefinition, pack.Environment); err != nil {
		return curriculum.LearningPack{}, nil, err
	}
	if err := pack.Validate(); err != nil {
		return curriculum.LearningPack{}, nil, fmt.Errorf("learning pack: %w", err)
	}
	warnings := make([]curriculumapp.PackValidationIssue, 0)
	if manifest.Status != curriculum.ConceptCurrent {
		warnings = append(warnings, curriculumapp.PackValidationIssue{Code: "non_current_pack", Path: ManifestName, Message: "pack status is " + string(manifest.Status)})
	}
	return pack, warnings, nil
}

func validateChecksums(entries map[string][]byte) error {
	lines := strings.Split(strings.TrimSuffix(string(entries[ChecksumsName]), "\n"), "\n")
	checksums := make(map[string]string, len(lines))
	previous := ""
	for index, line := range lines {
		parts := strings.SplitN(line, "  ", 2)
		if len(parts) != 2 || len(parts[0]) != 64 {
			return fmt.Errorf("checksums line %d is malformed", index+1)
		}
		if _, err := hex.DecodeString(parts[0]); err != nil || strings.ToLower(parts[0]) != parts[0] {
			return fmt.Errorf("checksums line %d has invalid SHA-256", index+1)
		}
		if err := validateEntryName(parts[1]); err != nil || parts[1] == ChecksumsName {
			return fmt.Errorf("checksums line %d has invalid path", index+1)
		}
		if previous != "" && parts[1] <= previous {
			return fmt.Errorf("checksums entries are duplicate or not sorted")
		}
		previous = parts[1]
		checksums[parts[1]] = parts[0]
	}
	if len(checksums) != len(entries)-1 {
		return fmt.Errorf("checksums do not cover every pack file")
	}
	for name, encoded := range entries {
		if name == ChecksumsName {
			continue
		}
		expected, exists := checksums[name]
		if !exists {
			return fmt.Errorf("checksum for %q is missing", name)
		}
		actual := sha256.Sum256(encoded)
		if hex.EncodeToString(actual[:]) != expected {
			return fmt.Errorf("checksum mismatch for %q", name)
		}
	}
	return nil
}

func validateEvidenceReport(report evidenceReportDocument, definition curriculum.CurriculumDefinition, environment *curriculum.EnvironmentPack) error {
	if len(report.Bundles) != len(definition.SourceBundles) {
		return fmt.Errorf("evidence report bundle set does not match curriculum")
	}
	for index, raw := range report.Bundles {
		value, err := decodeBundleRef(raw)
		if err != nil {
			return fmt.Errorf("evidence report bundle %d: %w", index, err)
		}
		if value != definition.SourceBundles[index] {
			return fmt.Errorf("evidence report bundle %d does not match curriculum", index)
		}
	}
	reported := make(map[curriculum.EvidenceRef]struct{}, len(report.Claims))
	for _, raw := range report.Claims {
		refs, err := decodeEvidenceRefs([]evidenceRefDocument{raw})
		if err != nil {
			return err
		}
		if _, exists := reported[refs[0]]; exists {
			return fmt.Errorf("evidence report contains duplicate claim reference")
		}
		reported[refs[0]] = struct{}{}
	}
	for _, reference := range collectEvidenceRefs(definition, environment) {
		if _, exists := reported[reference]; !exists {
			return fmt.Errorf("curriculum claim %q in bundle %q is missing from evidence report", reference.ClaimID, reference.BundleID)
		}
	}
	return nil
}

func collectEvidenceRefs(definition curriculum.CurriculumDefinition, environment *curriculum.EnvironmentPack) []curriculum.EvidenceRef {
	seen := make(map[curriculum.EvidenceRef]struct{})
	result := make([]curriculum.EvidenceRef, 0)
	add := func(values []curriculum.EvidenceRef) {
		for _, value := range values {
			if _, exists := seen[value]; !exists {
				seen[value] = struct{}{}
				result = append(result, value)
			}
		}
	}
	for _, value := range definition.Goal.Outcomes {
		add(value.EvidenceRefs)
	}
	for _, value := range definition.Competencies.Competencies {
		add(value.EvidenceRefs)
	}
	for _, value := range definition.Concepts {
		add(value.EvidenceRefs)
	}
	for _, value := range definition.Prerequisites {
		add(value.EvidenceRefs)
	}
	for _, value := range definition.CoverageRequirements {
		add(value.EvidenceRefs)
	}
	if environment != nil {
		for _, value := range environment.Tools {
			add(value.EvidenceRefs)
		}
		for _, value := range environment.InstallGuidance {
			add(value.EvidenceRefs)
		}
	}
	sort.Slice(result, func(i, j int) bool {
		if result[i].BundleID != result[j].BundleID {
			return result[i].BundleID.String() < result[j].BundleID.String()
		}
		return result[i].ClaimID.String() < result[j].ClaimID.String()
	})
	return result
}
