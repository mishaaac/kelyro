// Package portabilityfs implements portable, integrity-checked tar.gz exports
// and staged imports on the local filesystem.
package portabilityfs

import (
	"archive/tar"
	"compress/gzip"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/mishaaac/kelyro/internal/artifacts"
	"github.com/mishaaac/kelyro/internal/platform"
	"github.com/mishaaac/kelyro/internal/portability"
	"github.com/mishaaac/kelyro/internal/workspace"
)

const (
	manifestName     = "manifest.json"
	dataPrefix       = "data/"
	maxManifestBytes = 4 * 1024 * 1024
	maxFiles         = 10000
	maxArchiveBytes  = int64(2 * 1024 * 1024 * 1024)
)

var portableInternalFiles = []string{"learning.db", "config.toml", "workspace.json"}

// Service stores no global state and never reads credential providers.
type Service struct {
	appVersion string
	validator  portability.Validator
	now        func() time.Time
}

func New(appVersion string, validator portability.Validator) *Service {
	if strings.TrimSpace(appVersion) == "" {
		appVersion = "unknown"
	}
	return &Service{appVersion: appVersion, validator: validator, now: time.Now}
}

type sourceFile struct {
	path     string
	absolute string
	record   portability.File
}

func (service *Service) Export(ctx context.Context, workspaceRoot string, options portability.ExportOptions) (report portability.Report, err error) {
	if err := ctx.Err(); err != nil {
		return portability.Report{}, err
	}
	if !options.Mode.Valid() {
		return portability.Report{}, fmt.Errorf("invalid export mode %q", options.Mode)
	}
	root, err := platform.NormalizePath(workspaceRoot)
	if err != nil {
		return portability.Report{}, err
	}
	info, err := os.Lstat(root)
	if err != nil || !info.IsDir() || info.Mode()&os.ModeSymlink != 0 {
		return portability.Report{}, fmt.Errorf("export root %s is not a regular directory", root)
	}
	metadata, err := readWorkspaceMetadata(root)
	if err != nil {
		return portability.Report{}, err
	}

	output, err := service.outputPath(root, options)
	if err != nil {
		return portability.Report{}, err
	}
	sources, err := collectSources(ctx, root, options.Mode, output)
	if err != nil {
		return portability.Report{}, err
	}
	databaseSchema := 0
	if options.Mode == portability.ModeFull {
		databasePath, found := sourcePath(sources, ".kelyro/learning.db")
		if found {
			if service.validator == nil {
				return portability.Report{}, errors.New("portable database validation is unavailable")
			}
			databaseSchema, err = service.validator.Validate(ctx, databasePath)
			if err != nil {
				return portability.Report{}, fmt.Errorf("validate exported database: %w", err)
			}
		}
	}
	manifest := portability.Manifest{
		FormatVersion: portability.FormatVersion, Mode: options.Mode, CreatedAt: service.now().UTC(),
		AppVersion: service.appVersion, WorkspaceID: metadata.WorkspaceID,
		WorkspaceSchemaVersion: metadata.SchemaVersion, DatabaseSchemaVersion: databaseSchema,
		Files: make([]portability.File, len(sources)),
	}
	for index, source := range sources {
		manifest.Files[index] = source.record
		report.TotalSize += source.record.Size
	}
	if err := validateManifest(manifest); err != nil {
		return portability.Report{}, err
	}
	if err := writeArchive(ctx, output, manifest, sources); err != nil {
		return portability.Report{}, err
	}
	return portability.Report{
		ArchivePath: output, Mode: options.Mode, FileCount: len(sources), TotalSize: report.TotalSize,
	}, nil
}

func (service *Service) Import(ctx context.Context, options portability.ImportOptions) (portability.Report, error) {
	if err := ctx.Err(); err != nil {
		return portability.Report{}, err
	}
	if strings.TrimSpace(options.ArchivePath) == "" || strings.TrimSpace(options.Destination) == "" {
		return portability.Report{}, errors.New("archive path and destination are required")
	}
	if !options.Conflicts.Valid() {
		return portability.Report{}, fmt.Errorf("invalid conflict strategy %q", options.Conflicts)
	}
	archivePath, err := platform.NormalizePath(options.ArchivePath)
	if err != nil {
		return portability.Report{}, err
	}
	destination, err := platform.NormalizePath(options.Destination)
	if err != nil {
		return portability.Report{}, err
	}
	staging, err := os.MkdirTemp("", "kelyro-import-*")
	if err != nil {
		return portability.Report{}, fmt.Errorf("create import staging directory: %w", err)
	}
	defer os.RemoveAll(staging)

	manifest, err := readArchive(ctx, archivePath, staging)
	if err != nil {
		return portability.Report{}, err
	}
	if err := service.validateStaged(ctx, staging, manifest); err != nil {
		return portability.Report{}, err
	}
	report, err := planImport(ctx, archivePath, destination, staging, manifest, options)
	if err != nil {
		return portability.Report{}, err
	}
	if options.DryRun {
		return report, nil
	}
	if len(report.Conflicts) > 0 {
		return report, fmt.Errorf("%w: %s", portability.ErrConflict, strings.Join(report.Conflicts, ", "))
	}
	if err := commitImport(ctx, destination, staging, report); err != nil {
		return portability.Report{}, err
	}
	return report, nil
}

func (service *Service) outputPath(root string, options portability.ExportOptions) (string, error) {
	if options.OutputPath != "" {
		return platform.NormalizePath(options.OutputPath)
	}
	name := fmt.Sprintf("%s-%s-%s.kelyro.tar.gz", filepath.Base(root), options.Mode, service.now().UTC().Format("20060102T150405.000000000Z"))
	return platform.NormalizePath(filepath.Join(filepath.Dir(root), name))
}

func collectSources(ctx context.Context, root string, mode portability.Mode, excluded string) ([]sourceFile, error) {
	var sources []sourceFile
	seen := make(map[string]string)
	add := func(absolute, portable string, ownership artifacts.Ownership) error {
		if samePath(absolute, excluded) {
			return nil
		}
		if !validPortablePath(portable) {
			return fmt.Errorf("workspace path %q is not portable", portable)
		}
		folded := strings.ToLower(portable)
		if previous, ok := seen[folded]; ok {
			return fmt.Errorf("workspace paths %q and %q collide on case-insensitive filesystems", previous, portable)
		}
		info, err := os.Lstat(absolute)
		if err != nil {
			return err
		}
		if !info.Mode().IsRegular() {
			return fmt.Errorf("export source %s is not a regular file", absolute)
		}
		record, err := hashFile(ctx, absolute, portable, ownership)
		if err != nil {
			return err
		}
		seen[folded] = portable
		sources = append(sources, sourceFile{path: portable, absolute: absolute, record: record})
		return nil
	}

	err := filepath.WalkDir(root, func(current string, entry fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if err := ctx.Err(); err != nil {
			return err
		}
		if current == root {
			return nil
		}
		relative, err := filepath.Rel(root, current)
		if err != nil {
			return err
		}
		components := strings.Split(filepath.ToSlash(relative), "/")
		hidden := false
		for _, component := range components {
			hidden = hidden || strings.HasPrefix(component, ".")
		}
		if entry.IsDir() && hidden {
			return filepath.SkipDir
		}
		if hidden {
			return nil
		}
		if entry.Type()&os.ModeSymlink != 0 || entry.IsDir() || !entry.Type().IsRegular() {
			return nil
		}
		if !strings.EqualFold(filepath.Ext(entry.Name()), ".md") {
			return nil
		}
		return add(current, filepath.ToSlash(relative), artifacts.Classify(relative))
	})
	if err != nil {
		return nil, fmt.Errorf("select readable workspace files: %w", err)
	}

	if mode == portability.ModeFull {
		internal, err := platform.WorkspaceInternalDir(root)
		if err != nil {
			return nil, err
		}
		for _, name := range portableInternalFiles {
			absolute := filepath.Join(internal, name)
			_, err := os.Lstat(absolute)
			if errors.Is(err, fs.ErrNotExist) && name != "workspace.json" {
				continue
			}
			if err != nil {
				return nil, fmt.Errorf("inspect portable workspace state %s: %w", absolute, err)
			}
			if err := add(absolute, path.Join(".kelyro", name), artifacts.MachineOwned); err != nil {
				return nil, err
			}
		}
		stateRoot := filepath.Join(internal, "state")
		err = filepath.WalkDir(stateRoot, func(current string, entry fs.DirEntry, walkErr error) error {
			if errors.Is(walkErr, fs.ErrNotExist) && current == stateRoot {
				return nil
			}
			if walkErr != nil {
				return walkErr
			}
			if current == stateRoot || entry.IsDir() {
				return nil
			}
			if entry.Type()&os.ModeSymlink != 0 || !entry.Type().IsRegular() {
				return fmt.Errorf("portable state source %s is not a regular file", current)
			}
			relative, err := filepath.Rel(internal, current)
			if err != nil {
				return err
			}
			return add(current, path.Join(".kelyro", filepath.ToSlash(relative)), artifacts.MachineOwned)
		})
		if err != nil {
			return nil, fmt.Errorf("select portable workspace state: %w", err)
		}
	}
	sort.Slice(sources, func(i, j int) bool { return sources[i].path < sources[j].path })
	return sources, nil
}

func hashFile(ctx context.Context, absolute, portable string, ownership artifacts.Ownership) (portability.File, error) {
	input, err := os.Open(absolute)
	if err != nil {
		return portability.File{}, fmt.Errorf("open export source %s: %w", absolute, err)
	}
	defer input.Close()
	digest := sha256.New()
	size, err := copyWithContext(ctx, digest, input)
	if err != nil {
		return portability.File{}, fmt.Errorf("hash export source %s: %w", absolute, err)
	}
	return portability.File{Path: portable, Size: size, SHA256: hex.EncodeToString(digest.Sum(nil)), Ownership: ownership}, nil
}

func writeArchive(ctx context.Context, destination string, manifest portability.Manifest, sources []sourceFile) (err error) {
	if _, err := os.Lstat(destination); err == nil {
		return fmt.Errorf("export destination already exists: %s", destination)
	} else if !errors.Is(err, fs.ErrNotExist) {
		return fmt.Errorf("inspect export destination: %w", err)
	}
	parent := filepath.Dir(destination)
	info, err := os.Stat(parent)
	if err != nil || !info.IsDir() {
		return fmt.Errorf("export destination directory %s is unavailable", parent)
	}
	temporary, err := os.CreateTemp(parent, ".kelyro-export-*")
	if err != nil {
		return fmt.Errorf("create export staging file: %w", err)
	}
	temporaryPath := temporary.Name()
	defer func() {
		_ = temporary.Close()
		if err != nil {
			_ = os.Remove(temporaryPath)
		}
	}()
	if err = temporary.Chmod(0o600); err != nil {
		return err
	}
	compressed := gzip.NewWriter(temporary)
	compressed.Name = "kelyro-portable-export"
	compressed.ModTime = manifest.CreatedAt
	archive := tar.NewWriter(compressed)
	encoded, err := json.MarshalIndent(manifest, "", "  ")
	if err != nil {
		return fmt.Errorf("encode portable manifest: %w", err)
	}
	encoded = append(encoded, '\n')
	if err = archive.WriteHeader(&tar.Header{Name: manifestName, Mode: 0o600, Size: int64(len(encoded)), ModTime: manifest.CreatedAt}); err != nil {
		return err
	}
	if _, err = archive.Write(encoded); err != nil {
		return err
	}
	buffer := make([]byte, 64*1024)
	for _, source := range sources {
		if err = ctx.Err(); err != nil {
			return err
		}
		mode := int64(0o644)
		if source.record.Ownership == artifacts.MachineOwned {
			mode = 0o600
		}
		if err = archive.WriteHeader(&tar.Header{Name: dataPrefix + source.path, Mode: mode, Size: source.record.Size, ModTime: manifest.CreatedAt}); err != nil {
			return err
		}
		input, openErr := os.Open(source.absolute)
		if openErr != nil {
			return openErr
		}
		digest := sha256.New()
		written, copyErr := copyBufferWithContext(ctx, io.MultiWriter(archive, digest), input, buffer)
		closeErr := input.Close()
		if copyErr != nil {
			return copyErr
		}
		if closeErr != nil {
			return closeErr
		}
		if written != source.record.Size || hex.EncodeToString(digest.Sum(nil)) != source.record.SHA256 {
			return fmt.Errorf("export source changed while archiving: %s", source.path)
		}
	}
	if err = archive.Close(); err != nil {
		return fmt.Errorf("close tar archive: %w", err)
	}
	if err = compressed.Close(); err != nil {
		return fmt.Errorf("close compressed archive: %w", err)
	}
	if err = temporary.Sync(); err != nil {
		return fmt.Errorf("sync portable archive: %w", err)
	}
	if err = temporary.Close(); err != nil {
		return fmt.Errorf("close portable archive: %w", err)
	}
	if _, err = os.Lstat(destination); err == nil {
		return fmt.Errorf("export destination appeared during creation: %s", destination)
	} else if !errors.Is(err, fs.ErrNotExist) {
		return err
	}
	if err = os.Rename(temporaryPath, destination); err != nil {
		return fmt.Errorf("publish portable archive: %w", err)
	}
	return nil
}

func readArchive(ctx context.Context, archivePath, staging string) (portability.Manifest, error) {
	input, err := os.Open(archivePath)
	if err != nil {
		return portability.Manifest{}, fmt.Errorf("open portable archive: %w", err)
	}
	defer input.Close()
	compressed, err := gzip.NewReader(input)
	if err != nil {
		return portability.Manifest{}, fmt.Errorf("%w: gzip header: %v", portability.ErrMalformed, err)
	}
	defer compressed.Close()
	reader := tar.NewReader(compressed)
	header, err := reader.Next()
	if err != nil || header.Name != manifestName || !regularHeader(header) || header.Size < 0 || header.Size > maxManifestBytes {
		return portability.Manifest{}, fmt.Errorf("%w: manifest must be the first regular archive entry", portability.ErrMalformed)
	}
	encoded, err := io.ReadAll(io.LimitReader(reader, maxManifestBytes+1))
	if err != nil || int64(len(encoded)) != header.Size {
		return portability.Manifest{}, fmt.Errorf("%w: read manifest", portability.ErrMalformed)
	}
	var manifest portability.Manifest
	decoder := json.NewDecoder(strings.NewReader(string(encoded)))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&manifest); err != nil {
		return portability.Manifest{}, fmt.Errorf("%w: decode manifest: %v", portability.ErrMalformed, err)
	}
	if err := decoder.Decode(&struct{}{}); !errors.Is(err, io.EOF) {
		return portability.Manifest{}, fmt.Errorf("%w: manifest contains trailing data", portability.ErrMalformed)
	}
	if err := validateManifest(manifest); err != nil {
		return portability.Manifest{}, err
	}
	expected := make(map[string]portability.File, len(manifest.Files))
	for _, file := range manifest.Files {
		expected[file.Path] = file
	}
	seen := make(map[string]bool, len(expected))
	for {
		if err := ctx.Err(); err != nil {
			return portability.Manifest{}, err
		}
		header, err = reader.Next()
		if errors.Is(err, io.EOF) {
			break
		}
		if err != nil {
			return portability.Manifest{}, fmt.Errorf("%w: read tar entry: %v", portability.ErrMalformed, err)
		}
		if !regularHeader(header) || !strings.HasPrefix(header.Name, dataPrefix) {
			return portability.Manifest{}, fmt.Errorf("%w: invalid archive entry %q", portability.ErrMalformed, header.Name)
		}
		portable := strings.TrimPrefix(header.Name, dataPrefix)
		record, ok := expected[portable]
		if !ok || seen[portable] || header.Size != record.Size {
			return portability.Manifest{}, fmt.Errorf("%w: unexpected or duplicate archive entry %q", portability.ErrMalformed, header.Name)
		}
		destination := filepath.Join(staging, "data", filepath.FromSlash(portable))
		if err := os.MkdirAll(filepath.Dir(destination), 0o700); err != nil {
			return portability.Manifest{}, err
		}
		output, err := os.OpenFile(destination, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0o600)
		if err != nil {
			return portability.Manifest{}, err
		}
		digest := sha256.New()
		written, copyErr := copyWithContext(ctx, io.MultiWriter(output, digest), io.LimitReader(reader, record.Size+1))
		closeErr := output.Close()
		if copyErr != nil || closeErr != nil || written != record.Size || hex.EncodeToString(digest.Sum(nil)) != record.SHA256 {
			return portability.Manifest{}, fmt.Errorf("%w: checksum mismatch for %s", portability.ErrMalformed, portable)
		}
		seen[portable] = true
	}
	if len(seen) != len(expected) {
		return portability.Manifest{}, fmt.Errorf("%w: archive is missing declared files", portability.ErrMalformed)
	}
	return manifest, nil
}

func (service *Service) validateStaged(ctx context.Context, staging string, manifest portability.Manifest) error {
	if manifest.Mode != portability.ModeFull {
		return nil
	}
	metadata, err := readMetadataFile(filepath.Join(staging, "data", ".kelyro", "workspace.json"))
	if err != nil || metadata.WorkspaceID != manifest.WorkspaceID || metadata.SchemaVersion != manifest.WorkspaceSchemaVersion {
		return fmt.Errorf("%w: workspace metadata does not match manifest", portability.ErrMalformed)
	}
	database := filepath.Join(staging, "data", ".kelyro", "learning.db")
	if manifest.DatabaseSchemaVersion == 0 {
		return nil
	}
	if service.validator == nil {
		return errors.New("portable database validation is unavailable")
	}
	version, err := service.validator.Validate(ctx, database)
	if err != nil || version != manifest.DatabaseSchemaVersion {
		return fmt.Errorf("%w: imported database validation failed", portability.ErrMalformed)
	}
	return nil
}

func planImport(ctx context.Context, archivePath, destination, staging string, manifest portability.Manifest, options portability.ImportOptions) (portability.Report, error) {
	report := portability.Report{
		ArchivePath: archivePath, Destination: destination, Mode: manifest.Mode, DryRun: options.DryRun,
		FileCount: len(manifest.Files),
	}
	if info, err := os.Lstat(destination); err == nil {
		if !info.IsDir() || info.Mode()&os.ModeSymlink != 0 {
			return portability.Report{}, fmt.Errorf("import destination %s is not a regular directory", destination)
		}
	} else if !errors.Is(err, fs.ErrNotExist) {
		return portability.Report{}, fmt.Errorf("inspect import destination: %w", err)
	}
	for _, file := range manifest.Files {
		if err := ctx.Err(); err != nil {
			return portability.Report{}, err
		}
		report.TotalSize += file.Size
		target, exists, err := inspectTarget(destination, file.Path)
		if err != nil {
			return portability.Report{}, err
		}
		if !exists {
			report.Creates = append(report.Creates, file.Path)
			continue
		}
		same, err := fileMatches(ctx, target, file)
		if err != nil {
			return portability.Report{}, err
		}
		if same {
			report.Skips = append(report.Skips, file.Path)
			continue
		}
		switch options.Conflicts {
		case portability.ConflictKeep:
			report.Skips = append(report.Skips, file.Path)
		case portability.ConflictOverwrite:
			report.Replaces = append(report.Replaces, file.Path)
		default:
			report.Conflicts = append(report.Conflicts, file.Path)
		}
	}
	return report, nil
}

type committedFile struct {
	target   string
	original string
	replaced bool
}

func commitImport(ctx context.Context, destination, staging string, report portability.Report) (err error) {
	if err := os.MkdirAll(destination, 0o755); err != nil {
		return fmt.Errorf("create import destination: %w", err)
	}
	if report.Mode == portability.ModeFull {
		for _, relative := range []string{
			filepath.Join(".kelyro", "state"), filepath.Join(".kelyro", "cache"),
			filepath.Join(".kelyro", "backups"), filepath.Join(".kelyro", "logs"),
		} {
			if err := os.MkdirAll(filepath.Join(destination, relative), 0o700); err != nil {
				return fmt.Errorf("create imported workspace directory %s: %w", relative, err)
			}
		}
	}
	recovery, err := os.MkdirTemp(destination, ".kelyro-import-recovery-*")
	if err != nil {
		return fmt.Errorf("create import recovery directory: %w", err)
	}
	defer func() {
		if removeErr := os.RemoveAll(recovery); err == nil && removeErr != nil {
			err = removeErr
		}
	}()
	actions := make(map[string]bool, len(report.Creates)+len(report.Replaces))
	for _, item := range report.Creates {
		actions[item] = false
	}
	for _, item := range report.Replaces {
		actions[item] = true
	}
	paths := append(append([]string(nil), report.Creates...), report.Replaces...)
	sort.Strings(paths)
	committed := make([]committedFile, 0, len(paths))
	rollback := func(cause error) error {
		var rollbackErr error
		for index := len(committed) - 1; index >= 0; index-- {
			item := committed[index]
			if removeErr := os.Remove(item.target); removeErr != nil && !errors.Is(removeErr, fs.ErrNotExist) {
				rollbackErr = errors.Join(rollbackErr, removeErr)
			}
			if item.replaced {
				if restoreErr := os.Rename(item.original, item.target); restoreErr != nil {
					rollbackErr = errors.Join(rollbackErr, restoreErr)
				}
			}
		}
		return errors.Join(cause, rollbackErr)
	}
	for _, portable := range paths {
		if err := ctx.Err(); err != nil {
			return rollback(err)
		}
		target, exists, inspectErr := inspectTarget(destination, portable)
		if inspectErr != nil {
			return rollback(inspectErr)
		}
		replace := actions[portable]
		if replace != exists {
			return rollback(fmt.Errorf("%w: destination changed during import: %s", portability.ErrConflict, portable))
		}
		if err := os.MkdirAll(filepath.Dir(target), directoryMode(portable)); err != nil {
			return rollback(err)
		}
		temporary, err := os.CreateTemp(filepath.Dir(target), ".kelyro-import-*")
		if err != nil {
			return rollback(err)
		}
		temporaryPath := temporary.Name()
		source, err := os.Open(filepath.Join(staging, "data", filepath.FromSlash(portable)))
		if err == nil {
			_, err = copyWithContext(ctx, temporary, source)
			_ = source.Close()
		}
		if syncErr := temporary.Sync(); err == nil {
			err = syncErr
		}
		if closeErr := temporary.Close(); err == nil {
			err = closeErr
		}
		if err == nil {
			err = os.Chmod(temporaryPath, fileMode(portable))
		}
		if err != nil {
			_ = os.Remove(temporaryPath)
			return rollback(err)
		}
		item := committedFile{target: target, replaced: replace}
		if replace {
			item.original = filepath.Join(recovery, filepath.FromSlash(portable))
			if err := os.MkdirAll(filepath.Dir(item.original), 0o700); err != nil {
				_ = os.Remove(temporaryPath)
				return rollback(err)
			}
			if err := os.Rename(target, item.original); err != nil {
				_ = os.Remove(temporaryPath)
				return rollback(err)
			}
		}
		if err := os.Rename(temporaryPath, target); err != nil {
			if replace {
				_ = os.Rename(item.original, target)
			}
			_ = os.Remove(temporaryPath)
			return rollback(err)
		}
		committed = append(committed, item)
	}
	return nil
}

func validateManifest(manifest portability.Manifest) error {
	if manifest.FormatVersion != portability.FormatVersion || !manifest.Mode.Valid() || manifest.CreatedAt.IsZero() ||
		strings.TrimSpace(manifest.AppVersion) == "" || strings.TrimSpace(manifest.WorkspaceID) == "" ||
		manifest.WorkspaceSchemaVersion < 1 || manifest.DatabaseSchemaVersion < 0 || len(manifest.Files) > maxFiles {
		return fmt.Errorf("%w: invalid manifest metadata", portability.ErrMalformed)
	}
	seen := make(map[string]bool, len(manifest.Files))
	total := int64(0)
	hasMetadata := false
	hasDatabase := false
	for _, file := range manifest.Files {
		folded := strings.ToLower(file.Path)
		if !validPortablePath(file.Path) || seen[folded] || file.Size < 0 || len(file.SHA256) != sha256.Size*2 || !file.Ownership.Valid() {
			return fmt.Errorf("%w: invalid file record %q", portability.ErrMalformed, file.Path)
		}
		if _, err := hex.DecodeString(file.SHA256); err != nil {
			return fmt.Errorf("%w: invalid checksum for %s", portability.ErrMalformed, file.Path)
		}
		if artifacts.Classify(filepath.FromSlash(file.Path)) != file.Ownership {
			return fmt.Errorf("%w: invalid ownership for %s", portability.ErrMalformed, file.Path)
		}
		if manifest.Mode == portability.ModeHuman && (file.Ownership == artifacts.MachineOwned || !strings.EqualFold(path.Ext(file.Path), ".md")) {
			return fmt.Errorf("%w: human export contains non-readable state %s", portability.ErrMalformed, file.Path)
		}
		if file.Ownership == artifacts.MachineOwned && !allowedInternalPath(file.Path) {
			return fmt.Errorf("%w: internal path is not allowlisted: %s", portability.ErrMalformed, file.Path)
		}
		seen[folded] = true
		total += file.Size
		if total < 0 || total > maxArchiveBytes {
			return fmt.Errorf("%w: declared archive size exceeds limit", portability.ErrMalformed)
		}
		hasMetadata = hasMetadata || file.Path == ".kelyro/workspace.json"
		hasDatabase = hasDatabase || file.Path == ".kelyro/learning.db"
	}
	if manifest.Mode == portability.ModeHuman && manifest.DatabaseSchemaVersion != 0 {
		return fmt.Errorf("%w: human export declares database state", portability.ErrMalformed)
	}
	if manifest.Mode == portability.ModeFull && (!hasMetadata || (manifest.DatabaseSchemaVersion > 0) != hasDatabase) {
		return fmt.Errorf("%w: full export is incomplete", portability.ErrMalformed)
	}
	return nil
}

func allowedInternalPath(portable string) bool {
	for _, name := range portableInternalFiles {
		if portable == path.Join(".kelyro", name) {
			return true
		}
	}
	return strings.HasPrefix(portable, ".kelyro/state/")
}

func validPortablePath(portable string) bool {
	if portable == "" || portable == "." || !fs.ValidPath(portable) || strings.ContainsAny(portable, "\\:\x00") {
		return false
	}
	for _, component := range strings.Split(portable, "/") {
		if strings.HasSuffix(component, " ") || strings.HasSuffix(component, ".") || windowsReserved(component) {
			return false
		}
	}
	return true
}

func windowsReserved(component string) bool {
	base := strings.ToUpper(strings.TrimSuffix(component, path.Ext(component)))
	if base == "CON" || base == "PRN" || base == "AUX" || base == "NUL" {
		return true
	}
	if len(base) == 4 && (strings.HasPrefix(base, "COM") || strings.HasPrefix(base, "LPT")) && base[3] >= '1' && base[3] <= '9' {
		return true
	}
	return false
}

func inspectTarget(root, portable string) (string, bool, error) {
	if !validPortablePath(portable) {
		return "", false, fmt.Errorf("%w: invalid destination path %q", portability.ErrMalformed, portable)
	}
	target := filepath.Join(root, filepath.FromSlash(portable))
	relative, err := filepath.Rel(root, target)
	if err != nil || relative == ".." || strings.HasPrefix(relative, ".."+string(filepath.Separator)) {
		return "", false, fmt.Errorf("%w: path traversal %q", portability.ErrMalformed, portable)
	}
	current := root
	components := strings.Split(filepath.FromSlash(portable), string(filepath.Separator))
	for index, component := range components {
		current = filepath.Join(current, component)
		info, statErr := os.Lstat(current)
		if errors.Is(statErr, fs.ErrNotExist) {
			return target, false, nil
		}
		if statErr != nil {
			return "", false, statErr
		}
		if info.Mode()&os.ModeSymlink != 0 {
			return "", false, fmt.Errorf("%w: destination path uses a symbolic link: %s", portability.ErrMalformed, portable)
		}
		if index < len(components)-1 && !info.IsDir() {
			return "", false, fmt.Errorf("%w: destination ancestor is not a directory: %s", portability.ErrConflict, portable)
		}
		if index == len(components)-1 {
			if !info.Mode().IsRegular() {
				return "", false, fmt.Errorf("%w: destination is not a regular file: %s", portability.ErrConflict, portable)
			}
			return target, true, nil
		}
	}
	return target, false, nil
}

func fileMatches(ctx context.Context, target string, expected portability.File) (bool, error) {
	record, err := hashFile(ctx, target, expected.Path, expected.Ownership)
	if err != nil {
		return false, err
	}
	return record.Size == expected.Size && record.SHA256 == expected.SHA256, nil
}

func readWorkspaceMetadata(root string) (workspace.Metadata, error) {
	path, err := platform.WorkspaceMetadataPath(root)
	if err != nil {
		return workspace.Metadata{}, err
	}
	metadata, err := readMetadataFile(path)
	if err != nil {
		return workspace.Metadata{}, fmt.Errorf("read workspace metadata for export: %w", err)
	}
	return metadata, nil
}

func readMetadataFile(path string) (workspace.Metadata, error) {
	encoded, err := os.ReadFile(path)
	if err != nil {
		return workspace.Metadata{}, err
	}
	var metadata workspace.Metadata
	if err := json.Unmarshal(encoded, &metadata); err != nil {
		return workspace.Metadata{}, err
	}
	if metadata.WorkspaceID == "" || metadata.SchemaVersion < 1 || metadata.CreatedAt.IsZero() || strings.TrimSpace(metadata.AppVersion) == "" {
		return workspace.Metadata{}, errors.New("invalid workspace metadata")
	}
	return metadata, nil
}

func sourcePath(sources []sourceFile, portable string) (string, bool) {
	for _, source := range sources {
		if source.path == portable {
			return source.absolute, true
		}
	}
	return "", false
}

func regularHeader(header *tar.Header) bool {
	return header.Typeflag == tar.TypeReg || header.Typeflag == tar.TypeRegA
}

func samePath(first, second string) bool {
	if first == "" || second == "" {
		return false
	}
	return filepath.Clean(first) == filepath.Clean(second)
}

func fileMode(portable string) fs.FileMode {
	if strings.HasPrefix(portable, ".kelyro/") {
		return 0o600
	}
	return 0o644
}

func directoryMode(portable string) fs.FileMode {
	if strings.HasPrefix(portable, ".kelyro/") {
		return 0o700
	}
	return 0o755
}

func copyWithContext(ctx context.Context, destination io.Writer, source io.Reader) (int64, error) {
	return copyBufferWithContext(ctx, destination, source, make([]byte, 64*1024))
}

func copyBufferWithContext(ctx context.Context, destination io.Writer, source io.Reader, buffer []byte) (int64, error) {
	var total int64
	for {
		if err := ctx.Err(); err != nil {
			return total, err
		}
		read, readErr := source.Read(buffer)
		if read > 0 {
			written, writeErr := destination.Write(buffer[:read])
			total += int64(written)
			if writeErr != nil {
				return total, writeErr
			}
			if written != read {
				return total, io.ErrShortWrite
			}
		}
		if errors.Is(readErr, io.EOF) {
			return total, nil
		}
		if readErr != nil {
			return total, readErr
		}
	}
}

var _ portability.Service = (*Service)(nil)
