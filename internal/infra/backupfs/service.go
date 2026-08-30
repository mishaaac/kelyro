// Package backupfs implements atomic, integrity-checked workspace backups on
// the local filesystem.
package backupfs

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/mishaaac/kelyro/internal/backup"
	"github.com/mishaaac/kelyro/internal/platform"
	"github.com/mishaaac/kelyro/internal/workspace"
)

const (
	manifestName  = "manifest.json"
	dataDirectory = "data"
	maxManifest   = 4 * 1024 * 1024
)

var rootFiles = []string{"learning.db", "config.toml", "workspace.json"}

// Service stores backups below .kelyro/backups and restores only an explicit
// allowlist of machine-owned workspace data.
type Service struct {
	appVersion string
	validator  backup.Validator
	reconciler backup.RestoreReconciler
	now        func() time.Time
	random     io.Reader
	rename     func(string, string) error
	removeAll  func(string) error
}

// New creates a filesystem backup service. validator is required whenever a
// workspace contains learning.db.
func New(appVersion string, validator backup.Validator) *Service {
	if strings.TrimSpace(appVersion) == "" {
		appVersion = "unknown"
	}
	service := &Service{
		appVersion: appVersion,
		validator:  validator,
		now:        time.Now, random: rand.Reader,
		rename: os.Rename, removeAll: os.RemoveAll,
	}
	service.reconciler, _ = validator.(backup.RestoreReconciler)
	return service
}

func (service *Service) Create(ctx context.Context, root string, options backup.CreateOptions) (result backup.Info, err error) {
	if err := ctx.Err(); err != nil {
		return backup.Info{}, err
	}
	if options.Retention < 1 || options.Retention > 100 {
		return backup.Info{}, fmt.Errorf("backup retention must be from 1 to 100")
	}
	_, internal, backupDir, err := workspacePaths(root)
	if err != nil {
		return backup.Info{}, err
	}
	if err := os.MkdirAll(backupDir, 0o700); err != nil {
		return backup.Info{}, fmt.Errorf("create backup directory %s: %w", backupDir, err)
	}

	createdAt := service.now().UTC()
	id, err := service.newID(createdAt)
	if err != nil {
		return backup.Info{}, err
	}
	staging, err := os.MkdirTemp(backupDir, ".backup-*")
	if err != nil {
		return backup.Info{}, fmt.Errorf("create backup staging directory: %w", err)
	}
	defer func() { _ = service.removeAll(staging) }()
	dataPath := filepath.Join(staging, dataDirectory)
	if err := os.MkdirAll(filepath.Join(dataPath, "state"), 0o700); err != nil {
		return backup.Info{}, fmt.Errorf("create backup data staging: %w", err)
	}

	files, err := service.copyAllowlisted(ctx, internal, dataPath)
	if err != nil {
		return backup.Info{}, err
	}
	metadata, err := readWorkspaceMetadata(filepath.Join(dataPath, "workspace.json"))
	if err != nil {
		return backup.Info{}, fmt.Errorf("backup workspace metadata: %w", err)
	}
	databaseSchema := 0
	databasePath := filepath.Join(dataPath, "learning.db")
	if _, statErr := os.Stat(databasePath); statErr == nil {
		if service.validator == nil {
			return backup.Info{}, errors.New("database backup validation is unavailable")
		}
		databaseSchema, err = service.validator.Validate(ctx, databasePath)
		if err != nil {
			return backup.Info{}, fmt.Errorf("validate staged backup database: %w", err)
		}
	} else if !errors.Is(statErr, fs.ErrNotExist) {
		return backup.Info{}, fmt.Errorf("inspect staged backup database: %w", statErr)
	}

	reason := strings.TrimSpace(options.Reason)
	if reason == "" {
		reason = "manual"
	}
	manifest := backup.Manifest{
		FormatVersion: backup.FormatVersion, ID: id, CreatedAt: createdAt,
		Reason: reason, AppVersion: service.appVersion,
		WorkspaceID: metadata.WorkspaceID, WorkspaceSchemaVersion: metadata.SchemaVersion,
		DatabaseSchemaVersion: databaseSchema, Files: files,
	}
	if err := writeManifest(filepath.Join(staging, manifestName), manifest); err != nil {
		return backup.Info{}, err
	}
	destination := filepath.Join(backupDir, id)
	if err := service.rename(staging, destination); err != nil {
		return backup.Info{}, fmt.Errorf("commit backup %s: %w", id, err)
	}
	staging = ""
	if err := service.prune(ctx, backupDir, options.Retention); err != nil {
		return backup.Info{}, err
	}
	return backup.InfoFromManifest(manifest), nil
}

func (service *Service) List(ctx context.Context, root string) ([]backup.Info, error) {
	_, _, backupDir, err := workspacePaths(root)
	if err != nil {
		return nil, err
	}
	entries, err := os.ReadDir(backupDir)
	if errors.Is(err, fs.ErrNotExist) {
		return []backup.Info{}, nil
	}
	if err != nil {
		return nil, fmt.Errorf("list workspace backups: %w", err)
	}
	infos := make([]backup.Info, 0, len(entries))
	for _, entry := range entries {
		if err := ctx.Err(); err != nil {
			return nil, err
		}
		if !entry.IsDir() || strings.HasPrefix(entry.Name(), ".") {
			continue
		}
		manifest, err := service.loadAndValidate(ctx, filepath.Join(backupDir, entry.Name()), entry.Name())
		if err != nil {
			return nil, err
		}
		infos = append(infos, backup.InfoFromManifest(manifest))
	}
	sort.Slice(infos, func(i, j int) bool {
		if infos[i].CreatedAt.Equal(infos[j].CreatedAt) {
			return infos[i].ID > infos[j].ID
		}
		return infos[i].CreatedAt.After(infos[j].CreatedAt)
	})
	return infos, nil
}

func (service *Service) Restore(ctx context.Context, root, id string) (result backup.Info, err error) {
	if !validID(id) {
		return backup.Info{}, fmt.Errorf("%w: %s", backup.ErrNotFound, id)
	}
	root, internal, backupDir, err := workspacePaths(root)
	if err != nil {
		return backup.Info{}, err
	}
	backupPath := filepath.Join(backupDir, id)
	manifest, err := service.loadAndValidate(ctx, backupPath, id)
	if err != nil {
		return backup.Info{}, err
	}
	currentMetadata, err := readWorkspaceMetadata(filepath.Join(internal, "workspace.json"))
	if err != nil {
		return backup.Info{}, fmt.Errorf("read current workspace metadata: %w", err)
	}
	if currentMetadata.WorkspaceID != manifest.WorkspaceID {
		return backup.Info{}, fmt.Errorf("%w: backup belongs to workspace %s", backup.ErrCorrupt, manifest.WorkspaceID)
	}

	staging, err := os.MkdirTemp(internal, ".restore-stage-*")
	if err != nil {
		return backup.Info{}, fmt.Errorf("create restore staging directory: %w", err)
	}
	defer func() { _ = service.removeAll(staging) }()
	if err := os.MkdirAll(filepath.Join(staging, "state"), 0o700); err != nil {
		return backup.Info{}, fmt.Errorf("create restore state staging: %w", err)
	}
	for _, file := range manifest.Files {
		if err := copyVerified(ctx, filepath.Join(backupPath, dataDirectory, filepath.FromSlash(file.Path)), filepath.Join(staging, filepath.FromSlash(file.Path)), file); err != nil {
			return backup.Info{}, err
		}
	}
	if manifest.DatabaseSchemaVersion > 0 {
		if service.validator == nil {
			return backup.Info{}, errors.New("database restore validation is unavailable")
		}
		version, err := service.validator.Validate(ctx, filepath.Join(staging, "learning.db"))
		if err != nil {
			return backup.Info{}, fmt.Errorf("validate restore database in staging: %w", err)
		}
		if version != manifest.DatabaseSchemaVersion {
			return backup.Info{}, fmt.Errorf("%w: database schema is %d, manifest records %d", backup.ErrCorrupt, version, manifest.DatabaseSchemaVersion)
		}
		if service.reconciler == nil {
			return backup.Info{}, errors.New("database restore artifact reconciliation is unavailable")
		}
		if err := service.reconciler.ReconcileUnbackedArtifacts(
			ctx,
			filepath.Join(internal, "learning.db"),
			filepath.Join(staging, "learning.db"),
		); err != nil {
			return backup.Info{}, fmt.Errorf("reconcile unbacked artifacts in restore database: %w", err)
		}
		version, err = service.validator.Validate(ctx, filepath.Join(staging, "learning.db"))
		if err != nil {
			return backup.Info{}, fmt.Errorf("validate reconciled restore database in staging: %w", err)
		}
		if version != manifest.DatabaseSchemaVersion {
			return backup.Info{}, fmt.Errorf("%w: reconciled database schema is %d, manifest records %d", backup.ErrCorrupt, version, manifest.DatabaseSchemaVersion)
		}
	}
	stagedMetadata, err := readWorkspaceMetadata(filepath.Join(staging, "workspace.json"))
	if err != nil || stagedMetadata.WorkspaceID != manifest.WorkspaceID || stagedMetadata.SchemaVersion != manifest.WorkspaceSchemaVersion {
		return backup.Info{}, fmt.Errorf("%w: staged workspace metadata does not match manifest", backup.ErrCorrupt)
	}

	recovery, err := os.MkdirTemp(internal, ".restore-original-*")
	if err != nil {
		return backup.Info{}, fmt.Errorf("create restore rollback directory: %w", err)
	}
	removeRecovery := true
	defer func() {
		if removeRecovery {
			_ = service.removeAll(recovery)
		}
	}()
	components := append(append([]string(nil), rootFiles...), "state")
	committed := make([]restoreMove, 0, len(components))
	for _, name := range components {
		move, moveErr := service.replaceComponent(filepath.Join(internal, name), filepath.Join(staging, name), filepath.Join(recovery, name))
		if moveErr != nil {
			if move.target != "" {
				committed = append(committed, move)
			}
			rollbackErr := service.rollback(committed)
			if rollbackErr != nil {
				removeRecovery = false
				return backup.Info{}, errors.Join(moveErr, rollbackErr, fmt.Errorf("original files retained at %s", recovery))
			}
			return backup.Info{}, moveErr
		}
		committed = append(committed, move)
	}
	return backup.InfoFromManifest(manifest), nil
}

type restoreMove struct {
	target      string
	original    string
	hadOriginal bool
	hadStaged   bool
}

func (service *Service) replaceComponent(target, staged, original string) (restoreMove, error) {
	move := restoreMove{target: target, original: original}
	if _, err := os.Lstat(target); err == nil {
		move.hadOriginal = true
		if err := service.rename(target, original); err != nil {
			return restoreMove{}, fmt.Errorf("preserve original %s: %w", target, err)
		}
	} else if !errors.Is(err, fs.ErrNotExist) {
		return restoreMove{}, fmt.Errorf("inspect restore target %s: %w", target, err)
	}
	if _, err := os.Lstat(staged); err == nil {
		move.hadStaged = true
		if err := service.rename(staged, target); err != nil {
			move.hadStaged = false
			return move, fmt.Errorf("commit restored component %s: %w", target, err)
		}
	} else if !errors.Is(err, fs.ErrNotExist) {
		return move, fmt.Errorf("inspect staged restore component %s: %w", staged, err)
	}
	return move, nil
}

func (service *Service) rollback(moves []restoreMove) error {
	var rollbackErr error
	for index := len(moves) - 1; index >= 0; index-- {
		move := moves[index]
		if move.hadStaged {
			if err := service.removeAll(move.target); err != nil {
				rollbackErr = errors.Join(rollbackErr, fmt.Errorf("remove failed restore target %s: %w", move.target, err))
				continue
			}
		}
		if move.hadOriginal {
			if err := service.rename(move.original, move.target); err != nil {
				rollbackErr = errors.Join(rollbackErr, fmt.Errorf("restore original %s: %w", move.target, err))
			}
		}
	}
	return rollbackErr
}

func (service *Service) copyAllowlisted(ctx context.Context, internal, destination string) ([]backup.File, error) {
	var files []backup.File
	for _, name := range rootFiles {
		source := filepath.Join(internal, name)
		info, err := os.Lstat(source)
		if errors.Is(err, fs.ErrNotExist) {
			continue
		}
		if err != nil {
			return nil, fmt.Errorf("inspect backup source %s: %w", source, err)
		}
		if !info.Mode().IsRegular() {
			return nil, fmt.Errorf("backup source %s is not a regular file", source)
		}
		record, err := copyFile(ctx, source, filepath.Join(destination, name), name)
		if err != nil {
			return nil, err
		}
		files = append(files, record)
	}
	stateRoot := filepath.Join(internal, "state")
	err := filepath.WalkDir(stateRoot, func(path string, entry fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if err := ctx.Err(); err != nil {
			return err
		}
		if path == stateRoot {
			return nil
		}
		if entry.Type()&os.ModeSymlink != 0 {
			return fmt.Errorf("backup state path %s is a symbolic link", path)
		}
		relative, err := filepath.Rel(internal, path)
		if err != nil {
			return err
		}
		if entry.IsDir() {
			return os.MkdirAll(filepath.Join(destination, relative), 0o700)
		}
		if !entry.Type().IsRegular() {
			return fmt.Errorf("backup state path %s is not a regular file", path)
		}
		record, err := copyFile(ctx, path, filepath.Join(destination, relative), filepath.ToSlash(relative))
		if err != nil {
			return err
		}
		files = append(files, record)
		return nil
	})
	if err != nil {
		return nil, fmt.Errorf("copy workspace state: %w", err)
	}
	sort.Slice(files, func(i, j int) bool { return files[i].Path < files[j].Path })
	return files, nil
}

func copyFile(ctx context.Context, source, destination, manifestPath string) (record backup.File, err error) {
	if err := ctx.Err(); err != nil {
		return backup.File{}, err
	}
	if err := os.MkdirAll(filepath.Dir(destination), 0o700); err != nil {
		return backup.File{}, fmt.Errorf("create backup parent: %w", err)
	}
	input, err := os.Open(source)
	if err != nil {
		return backup.File{}, fmt.Errorf("open backup source %s: %w", source, err)
	}
	defer input.Close()
	output, err := os.OpenFile(destination, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0o600)
	if err != nil {
		return backup.File{}, fmt.Errorf("create backup file %s: %w", destination, err)
	}
	defer func() {
		if closeErr := output.Close(); err == nil && closeErr != nil {
			err = closeErr
		}
	}()
	digest := sha256.New()
	written, err := io.Copy(io.MultiWriter(output, digest), input)
	if err != nil {
		return backup.File{}, fmt.Errorf("copy backup file %s: %w", source, err)
	}
	if err := output.Sync(); err != nil {
		return backup.File{}, fmt.Errorf("sync backup file %s: %w", destination, err)
	}
	return backup.File{Path: filepath.ToSlash(manifestPath), Size: written, SHA256: hex.EncodeToString(digest.Sum(nil))}, nil
}

func copyVerified(ctx context.Context, source, destination string, expected backup.File) error {
	record, err := copyFile(ctx, source, destination, expected.Path)
	if err != nil {
		return fmt.Errorf("%w: %v", backup.ErrCorrupt, err)
	}
	if record.Size != expected.Size || record.SHA256 != expected.SHA256 {
		return fmt.Errorf("%w: checksum mismatch for %s", backup.ErrCorrupt, expected.Path)
	}
	return nil
}

func (service *Service) loadAndValidate(ctx context.Context, path, expectedID string) (backup.Manifest, error) {
	encoded, err := os.ReadFile(filepath.Join(path, manifestName))
	if errors.Is(err, fs.ErrNotExist) {
		return backup.Manifest{}, fmt.Errorf("%w: %s", backup.ErrNotFound, expectedID)
	}
	if err != nil {
		return backup.Manifest{}, fmt.Errorf("read backup manifest %s: %w", expectedID, err)
	}
	if len(encoded) > maxManifest {
		return backup.Manifest{}, fmt.Errorf("%w: manifest exceeds %d bytes", backup.ErrCorrupt, maxManifest)
	}
	var manifest backup.Manifest
	if err := json.Unmarshal(encoded, &manifest); err != nil {
		return backup.Manifest{}, fmt.Errorf("%w: decode manifest %s: %v", backup.ErrCorrupt, expectedID, err)
	}
	if err := validateManifest(manifest, expectedID); err != nil {
		return backup.Manifest{}, err
	}
	for _, file := range manifest.Files {
		if err := verifyFile(ctx, filepath.Join(path, dataDirectory, filepath.FromSlash(file.Path)), file); err != nil {
			return backup.Manifest{}, err
		}
	}
	return manifest, nil
}

func validateManifest(manifest backup.Manifest, expectedID string) error {
	if manifest.FormatVersion != backup.FormatVersion || manifest.ID != expectedID || !validID(manifest.ID) ||
		manifest.CreatedAt.IsZero() || strings.TrimSpace(manifest.Reason) == "" || strings.TrimSpace(manifest.AppVersion) == "" ||
		strings.TrimSpace(manifest.WorkspaceID) == "" || manifest.WorkspaceSchemaVersion < 1 || manifest.DatabaseSchemaVersion < 0 {
		return fmt.Errorf("%w: invalid manifest for %s", backup.ErrCorrupt, expectedID)
	}
	seen := make(map[string]bool, len(manifest.Files))
	hasMetadata := false
	hasDatabase := false
	for _, file := range manifest.Files {
		if !allowedManifestPath(file.Path) || seen[file.Path] || file.Size < 0 || len(file.SHA256) != sha256.Size*2 {
			return fmt.Errorf("%w: invalid file record %q", backup.ErrCorrupt, file.Path)
		}
		if _, err := hex.DecodeString(file.SHA256); err != nil {
			return fmt.Errorf("%w: invalid checksum for %s", backup.ErrCorrupt, file.Path)
		}
		seen[file.Path] = true
		hasMetadata = hasMetadata || file.Path == "workspace.json"
		hasDatabase = hasDatabase || file.Path == "learning.db"
	}
	if !hasMetadata || (manifest.DatabaseSchemaVersion > 0) != hasDatabase {
		return fmt.Errorf("%w: manifest contents are incomplete", backup.ErrCorrupt)
	}
	return nil
}

func verifyFile(ctx context.Context, path string, expected backup.File) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	info, err := os.Lstat(path)
	if err != nil || !info.Mode().IsRegular() {
		return fmt.Errorf("%w: missing or invalid file %s", backup.ErrCorrupt, expected.Path)
	}
	input, err := os.Open(path)
	if err != nil {
		return fmt.Errorf("%w: open %s: %v", backup.ErrCorrupt, expected.Path, err)
	}
	defer input.Close()
	digest := sha256.New()
	size, err := io.Copy(digest, input)
	if err != nil {
		return fmt.Errorf("%w: read %s: %v", backup.ErrCorrupt, expected.Path, err)
	}
	if size != expected.Size || hex.EncodeToString(digest.Sum(nil)) != expected.SHA256 {
		return fmt.Errorf("%w: checksum mismatch for %s", backup.ErrCorrupt, expected.Path)
	}
	return nil
}

func (service *Service) prune(ctx context.Context, backupDir string, retention int) error {
	entries, err := os.ReadDir(backupDir)
	if err != nil {
		return fmt.Errorf("read backups for retention: %w", err)
	}
	type candidate struct {
		path    string
		created time.Time
		id      string
	}
	var backups []candidate
	for _, entry := range entries {
		if !entry.IsDir() || strings.HasPrefix(entry.Name(), ".") {
			continue
		}
		encoded, err := os.ReadFile(filepath.Join(backupDir, entry.Name(), manifestName))
		if err != nil {
			return fmt.Errorf("read backup %s for retention: %w", entry.Name(), err)
		}
		var manifest backup.Manifest
		if err := json.Unmarshal(encoded, &manifest); err != nil || manifest.ID != entry.Name() || manifest.CreatedAt.IsZero() {
			return fmt.Errorf("%w: invalid manifest for %s during retention", backup.ErrCorrupt, entry.Name())
		}
		backups = append(backups, candidate{path: filepath.Join(backupDir, entry.Name()), created: manifest.CreatedAt, id: entry.Name()})
	}
	sort.Slice(backups, func(i, j int) bool {
		if backups[i].created.Equal(backups[j].created) {
			return backups[i].id < backups[j].id
		}
		return backups[i].created.Before(backups[j].created)
	})
	for len(backups) > retention {
		if err := ctx.Err(); err != nil {
			return err
		}
		if err := service.removeAll(backups[0].path); err != nil {
			return fmt.Errorf("remove expired backup %s: %w", backups[0].id, err)
		}
		backups = backups[1:]
	}
	return nil
}

func workspacePaths(root string) (normalized, internal, backupDir string, err error) {
	normalized, err = platform.NormalizePath(root)
	if err != nil {
		return "", "", "", err
	}
	internal, err = platform.WorkspaceInternalDir(normalized)
	if err != nil {
		return "", "", "", err
	}
	info, err := os.Stat(internal)
	if err != nil {
		return "", "", "", fmt.Errorf("inspect workspace internals %s: %w", internal, err)
	}
	if !info.IsDir() {
		return "", "", "", fmt.Errorf("workspace internals %s is not a directory", internal)
	}
	backupDir, err = platform.WorkspaceBackupDir(normalized)
	return normalized, internal, backupDir, err
}

func writeManifest(path string, manifest backup.Manifest) error {
	encoded, err := json.MarshalIndent(manifest, "", "  ")
	if err != nil {
		return fmt.Errorf("encode backup manifest: %w", err)
	}
	encoded = append(encoded, '\n')
	file, err := os.OpenFile(path, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0o600)
	if err != nil {
		return fmt.Errorf("create backup manifest: %w", err)
	}
	if _, err := file.Write(encoded); err != nil {
		_ = file.Close()
		return fmt.Errorf("write backup manifest: %w", err)
	}
	if err := file.Sync(); err != nil {
		_ = file.Close()
		return fmt.Errorf("sync backup manifest: %w", err)
	}
	if err := file.Close(); err != nil {
		return fmt.Errorf("close backup manifest: %w", err)
	}
	return nil
}

func readWorkspaceMetadata(path string) (workspace.Metadata, error) {
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

func allowedManifestPath(path string) bool {
	if strings.ContainsAny(path, ":\x00") {
		return false
	}
	if path == "learning.db" || path == "config.toml" || path == "workspace.json" {
		return true
	}
	if !strings.HasPrefix(path, "state/") {
		return false
	}
	clean := filepath.ToSlash(filepath.Clean(filepath.FromSlash(path)))
	return clean == path && path != "state/" && !strings.Contains(path, "\\")
}

func validID(id string) bool {
	if len(id) < 2 || len(id) > 96 || strings.HasPrefix(id, ".") {
		return false
	}
	for _, character := range id {
		if (character >= 'a' && character <= 'z') || (character >= 'A' && character <= 'Z') ||
			(character >= '0' && character <= '9') || character == '-' || character == '_' || character == '.' {
			continue
		}
		return false
	}
	return true
}

func (service *Service) newID(createdAt time.Time) (string, error) {
	var suffix [4]byte
	if _, err := io.ReadFull(service.random, suffix[:]); err != nil {
		return "", fmt.Errorf("generate backup ID: %w", err)
	}
	return createdAt.UTC().Format("20060102T150405.000000000Z") + "-" + hex.EncodeToString(suffix[:]), nil
}

var _ backup.Service = (*Service)(nil)
