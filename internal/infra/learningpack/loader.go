package learningpack

import (
	"archive/zip"
	"bufio"
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"time"
	"unicode"
	"unicode/utf8"

	"github.com/mishaaac/kelyro/internal/curriculum"
	curriculumapp "github.com/mishaaac/kelyro/internal/curriculum/application"
)

const (
	MaximumPackEntries      = 1024
	MaximumPackFileBytes    = 16 << 20
	MaximumPackTotalBytes   = 32 << 20
	MaximumCompressionRatio = 100
)

var forbiddenExtensions = map[string]struct{}{
	".apk": {}, ".app": {}, ".bat": {}, ".bash": {}, ".class": {}, ".cmd": {}, ".com": {},
	".deb": {}, ".dll": {}, ".dmg": {}, ".dylib": {}, ".exe": {}, ".hta": {}, ".jar": {},
	".js": {}, ".msi": {}, ".pkg": {}, ".ps1": {}, ".py": {}, ".rpm": {}, ".scr": {},
	".sh": {}, ".so": {}, ".svg": {}, ".vbs": {}, ".wasm": {}, ".zsh": {},
}

var forbiddenRetentionExtensions = map[string]struct{}{".doc": {}, ".docx": {}, ".epub": {}, ".htm": {}, ".html": {}, ".mhtml": {}, ".pdf": {}}
var forbiddenRetentionField = regexp.MustCompile(`(?i)["']?(raw_body|raw_content|cached_body|cached_content|response_body|web_body|page_body|full_article|full_text|transcript)["']?[ \t]*:`)
var rawHTMLTag = regexp.MustCompile(`(?i)<\s*/?\s*[a-z][a-z0-9-]*(?:\s[^>]*)?/?>`)
var unsafeMarkdownDestination = regexp.MustCompile(`(?i)(?:!?\[[^\]\r\n]*\]\(\s*<?|\[[^\]\r\n]+\]:\s*<?|<\s*)(?:javascript|vbscript|data|file)(?::|&colon;|&#0*58;|&#x0*3a;)`)
var markdownImage = regexp.MustCompile(`!\[`)

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
		return loadDirectory(ctx, source, info)
	}
	if !info.Mode().IsRegular() {
		return nil, fmt.Errorf("pack source is neither a directory nor a regular ZIP file")
	}
	return loadZIP(ctx, source)
}

func loadDirectory(ctx context.Context, root string, rootInfo fs.FileInfo) (map[string][]byte, error) {
	rootAbs, err := filepath.Abs(root)
	if err != nil {
		return nil, err
	}
	entries := make(map[string][]byte)
	declaredTotal := int64(0)
	readTotal := int64(0)
	entryCount := 0
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
		entryCount++
		if entryCount > MaximumPackEntries {
			return fmt.Errorf("pack exceeds %d file system entries", MaximumPackEntries)
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
		declaredTotal += info.Size()
		if declaredTotal > MaximumPackTotalBytes {
			return fmt.Errorf("pack exceeds %d uncompressed bytes", MaximumPackTotalBytes)
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
		encoded, err := readBoundedDirectoryFile(resolvedAbs, info)
		if err != nil {
			return fmt.Errorf("read entry %q: %w", portable, err)
		}
		readTotal += int64(len(encoded))
		if readTotal > MaximumPackTotalBytes {
			return fmt.Errorf("pack exceeds %d uncompressed bytes", MaximumPackTotalBytes)
		}
		entries[portable] = encoded
		return nil
	})
	if err != nil {
		return nil, fmt.Errorf("load pack directory: %w", err)
	}
	currentRoot, err := os.Lstat(rootAbs)
	if err != nil || currentRoot.Mode()&os.ModeSymlink != 0 || !os.SameFile(rootInfo, currentRoot) {
		return nil, fmt.Errorf("load pack directory: pack root changed during validation")
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
		minimumCompressedSize := (item.UncompressedSize64 + MaximumCompressionRatio - 1) / MaximumCompressionRatio
		if item.UncompressedSize64 > 0 && item.CompressedSize64 < minimumCompressedSize {
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

func readBoundedDirectoryFile(name string, expected fs.FileInfo) ([]byte, error) {
	file, err := os.Open(name)
	if err != nil {
		return nil, err
	}
	defer file.Close()
	opened, err := file.Stat()
	if err != nil {
		return nil, err
	}
	current, err := os.Lstat(name)
	if err != nil {
		return nil, err
	}
	if current.Mode()&os.ModeSymlink != 0 || !opened.Mode().IsRegular() || !current.Mode().IsRegular() ||
		!os.SameFile(expected, opened) || !os.SameFile(opened, current) {
		return nil, fmt.Errorf("file changed or became unsafe during validation")
	}
	if opened.Size() > MaximumPackFileBytes {
		return nil, fmt.Errorf("file exceeds %d bytes", MaximumPackFileBytes)
	}
	encoded, err := io.ReadAll(io.LimitReader(file, MaximumPackFileBytes+1))
	if err != nil {
		return nil, err
	}
	if len(encoded) > MaximumPackFileBytes {
		return nil, fmt.Errorf("file exceeds %d bytes", MaximumPackFileBytes)
	}
	after, err := file.Stat()
	if err != nil || after.Size() != opened.Size() || int64(len(encoded)) != opened.Size() {
		return nil, fmt.Errorf("file changed size during validation")
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
	if _, forbidden := forbiddenRetentionExtensions[strings.ToLower(filepath.Ext(name))]; forbidden {
		return fmt.Errorf("retained external document entry %q is not allowed", name)
	}
	lower := strings.ToLower(name)
	for _, segment := range strings.Split(lower, "/") {
		switch segment {
		case "cache", "cached", "raw", "bodies", "snapshots", "transcripts":
			return fmt.Errorf("forbidden retained-source path %q", name)
		}
	}
	if strings.Contains(filepath.Base(lower), "transcript") || strings.Contains(filepath.Base(lower), "full-article") || strings.Contains(filepath.Base(lower), "full_article") {
		return fmt.Errorf("forbidden retained-source path %q", name)
	}
	return nil
}

func validateEntries(entries map[string][]byte) (curriculum.LearningPack, []curriculumapp.PackValidationIssue, error) {
	if err := validateInMemoryEntryBounds(entries); err != nil {
		return curriculum.LearningPack{}, nil, err
	}
	for _, required := range []string{ManifestName, ChecksumsName, EvidenceMarkdownName, AssetLicensesName} {
		if _, exists := entries[required]; !exists {
			return curriculum.LearningPack{}, nil, fmt.Errorf("required entry %q is missing", required)
		}
	}
	for name, encoded := range entries {
		if !utf8.Valid(encoded) {
			return curriculum.LearningPack{}, nil, fmt.Errorf("entry %q is not valid UTF-8", name)
		}
		if err := validateTextControls(name, encoded); err != nil {
			return curriculum.LearningPack{}, nil, err
		}
		if strings.EqualFold(filepath.Ext(name), ".md") {
			if err := validateMarkdown(name, encoded); err != nil {
				return curriculum.LearningPack{}, nil, err
			}
		}
	}
	if err := validateChecksums(entries); err != nil {
		return curriculum.LearningPack{}, nil, err
	}
	manifest, err := ParseManifest(bytes.NewReader(entries[ManifestName]))
	if err != nil {
		return curriculum.LearningPack{}, nil, err
	}
	for _, required := range []string{manifest.CurriculumEntry, manifest.SourceEvidenceEntry, manifest.BuildInfoEntry} {
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
	buildInfo, err := decodeBuildInfo(entries[manifest.BuildInfoEntry])
	if err != nil {
		return curriculum.LearningPack{}, nil, err
	}
	pack := curriculum.LearningPack{Manifest: manifest, Curriculum: curriculumDefinition, BuildInfo: &buildInfo, EvidenceReport: &report}
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
	if err := validateCopyrightAwareEntries(entries, report); err != nil {
		return curriculum.LearningPack{}, nil, err
	}
	warnings := make([]curriculumapp.PackValidationIssue, 0)
	if manifest.Status != curriculum.ConceptCurrent {
		warnings = append(warnings, curriculumapp.PackValidationIssue{Code: "non_current_pack", Path: ManifestName, Message: "pack status is " + string(manifest.Status)})
	}
	return pack, warnings, nil
}

func validateTextControls(name string, encoded []byte) error {
	for _, character := range string(encoded) {
		if unicode.IsControl(character) && character != '\n' && character != '\t' {
			return fmt.Errorf("entry %q contains a control character", name)
		}
	}
	return nil
}

func validateMarkdown(name string, encoded []byte) error {
	inFence := false
	fence := ""
	for _, line := range strings.Split(string(encoded), "\n") {
		trimmed := strings.TrimSpace(line)
		if strings.HasPrefix(trimmed, "```") || strings.HasPrefix(trimmed, "~~~") {
			marker := trimmed[:3]
			if !inFence {
				inFence, fence = true, marker
			} else if marker == fence {
				inFence, fence = false, ""
			}
			continue
		}
		if inFence {
			continue
		}
		if rawHTMLTag.MatchString(line) {
			return fmt.Errorf("Markdown entry %q contains unsafe raw HTML", name)
		}
		if unsafeMarkdownDestination.MatchString(line) {
			return fmt.Errorf("Markdown entry %q contains an unsafe link scheme", name)
		}
		if markdownImage.MatchString(line) {
			return fmt.Errorf("Markdown entry %q contains an embedded image", name)
		}
	}
	return nil
}

// validateInMemoryEntryBounds keeps builder validation aligned with the
// directory and ZIP loaders. Without it, Build could return an archive that
// the same package would later reject during installation.
func validateInMemoryEntryBounds(entries map[string][]byte) error {
	if len(entries) > MaximumPackEntries {
		return fmt.Errorf("pack exceeds %d file entries", MaximumPackEntries)
	}
	total := uint64(0)
	for name, encoded := range entries {
		if len(encoded) > MaximumPackFileBytes {
			return fmt.Errorf("entry %q exceeds %d bytes", name, MaximumPackFileBytes)
		}
		total += uint64(len(encoded))
		if total > MaximumPackTotalBytes {
			return fmt.Errorf("pack exceeds %d uncompressed bytes", MaximumPackTotalBytes)
		}
	}
	return nil
}

func validateCopyrightAwareEntries(entries map[string][]byte, report curriculum.CurriculumEvidenceReport) error {
	names := make([]string, 0, len(entries))
	for name := range entries {
		names = append(names, name)
	}
	sort.Strings(names)
	for _, name := range names {
		encoded := entries[name]
		extension := strings.ToLower(filepath.Ext(name))
		if (extension == ".json" || extension == ".yaml" || extension == ".yml") && forbiddenRetentionField.Match(encoded) {
			return fmt.Errorf("entry %q contains a forbidden source-retention field", name)
		}
	}
	if report.SourceBundleCount > 0 && len(report.Citations) == 0 {
		return fmt.Errorf("evidence report has no source citation URLs")
	}
	expectedMarkdown := curriculumapp.RenderCurriculumEvidenceMarkdown(report)
	if string(entries[EvidenceMarkdownName]) != expectedMarkdown {
		return fmt.Errorf("%s does not match the canonical evidence report projection", EvidenceMarkdownName)
	}
	encoded := entries[AssetLicensesName]
	if err := rejectDuplicateJSONKeys(encoded); err != nil {
		return fmt.Errorf("decode asset licenses: %w", err)
	}
	decoder := json.NewDecoder(bytes.NewReader(encoded))
	decoder.DisallowUnknownFields()
	var ledger assetLicenseDocument
	if err := decoder.Decode(&ledger); err != nil {
		return fmt.Errorf("decode asset licenses: %w", err)
	}
	if err := ensureJSONEOF(decoder); err != nil {
		return err
	}
	if ledger.SchemaVersion != AssetLicenseSchemaV1 {
		return fmt.Errorf("unsupported asset license schema %q", ledger.SchemaVersion)
	}
	recorded := make(map[string]assetLicenseEntryDocument, len(ledger.Assets))
	previous := ""
	for _, asset := range ledger.Assets {
		if err := validatePortablePath("licensed asset", asset.Path); err != nil {
			return err
		}
		if !strings.HasPrefix(asset.Path, "assets/") || asset.Path == AssetLicensesName {
			return fmt.Errorf("licensed asset %q is outside assets/", asset.Path)
		}
		if strings.TrimSpace(asset.License) == "" || strings.TrimSpace(asset.CopyrightHolder) == "" {
			return fmt.Errorf("licensed asset %q has no license or copyright holder", asset.Path)
		}
		if strings.IndexFunc(asset.License, unicode.IsControl) >= 0 || strings.IndexFunc(asset.CopyrightHolder, unicode.IsControl) >= 0 {
			return fmt.Errorf("licensed asset %q metadata contains a control character", asset.Path)
		}
		if asset.Authorship != AssetAuthorshipKelyro {
			return fmt.Errorf("licensed asset %q is not declared Kelyro-authored", asset.Path)
		}
		if previous != "" && asset.Path <= previous {
			return fmt.Errorf("asset license entries are duplicate or not sorted")
		}
		previous = asset.Path
		recorded[asset.Path] = asset
	}
	for _, name := range names {
		content := entries[name]
		if !strings.HasPrefix(name, "assets/") || name == AssetLicensesName {
			continue
		}
		asset, exists := recorded[name]
		if !exists {
			return fmt.Errorf("asset %q has no license record", name)
		}
		digest := sha256.Sum256(content)
		if asset.ContentHash != "sha256:"+hex.EncodeToString(digest[:]) {
			return fmt.Errorf("asset %q license record hash does not match", name)
		}
		delete(recorded, name)
	}
	if len(recorded) != 0 {
		return fmt.Errorf("asset license ledger references a missing asset")
	}
	return nil
}

func validateChecksums(entries map[string][]byte) error {
	expectedCount := len(entries) - 1
	checksums := make(map[string]string, expectedCount)
	previous := ""
	scanner := bufio.NewScanner(bytes.NewReader(entries[ChecksumsName]))
	scanner.Buffer(make([]byte, 4096), 64+2+MaximumPackPathBytes+1)
	index := 0
	for scanner.Scan() {
		index++
		if index > expectedCount {
			return fmt.Errorf("checksums do not cover every pack file")
		}
		line := scanner.Text()
		parts := strings.SplitN(line, "  ", 2)
		if len(parts) != 2 || len(parts[0]) != 64 {
			return fmt.Errorf("checksums line %d is malformed", index)
		}
		if _, err := hex.DecodeString(parts[0]); err != nil || strings.ToLower(parts[0]) != parts[0] {
			return fmt.Errorf("checksums line %d has invalid SHA-256", index)
		}
		if err := validateEntryName(parts[1]); err != nil || parts[1] == ChecksumsName {
			return fmt.Errorf("checksums line %d has invalid path", index)
		}
		if previous != "" && parts[1] <= previous {
			return fmt.Errorf("checksums entries are duplicate or not sorted")
		}
		previous = parts[1]
		checksums[parts[1]] = parts[0]
	}
	if err := scanner.Err(); err != nil {
		return fmt.Errorf("read checksums: %w", err)
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

func validateEvidenceReport(report curriculum.CurriculumEvidenceReport, definition curriculum.CurriculumDefinition, environment *curriculum.EnvironmentPack) error {
	if len(report.Bundles) != len(definition.SourceBundles) {
		return fmt.Errorf("evidence report bundle set does not match curriculum")
	}
	for index, value := range report.Bundles {
		if value != definition.SourceBundles[index] {
			return fmt.Errorf("evidence report bundle %d does not match curriculum", index)
		}
	}
	reported := make(map[curriculum.EvidenceRef]struct{}, len(report.Claims))
	for _, reference := range report.Claims {
		if _, exists := reported[reference]; exists {
			return fmt.Errorf("evidence report contains duplicate claim reference")
		}
		reported[reference] = struct{}{}
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
