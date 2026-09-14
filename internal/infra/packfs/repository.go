// Package packfs persists immutable Learning Pack artifacts globally and one
// active pack reference per workspace.
package packfs

import (
	"context"
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

	"github.com/mishaaac/kelyro/internal/curriculum"
	curriculumapp "github.com/mishaaac/kelyro/internal/curriculum/application"
	"github.com/mishaaac/kelyro/internal/platform"
)

const (
	archiveName  = "pack.zip"
	metadataName = "installation.json"
)

type validationService interface {
	Validate(context.Context, curriculumapp.PackSource) (curriculumapp.PackValidationResult, error)
}

type Repository struct {
	root      func() (string, error)
	validator validationService
}

func NewRepository(validator validationService) *Repository {
	return &Repository{root: platform.GlobalPackDir, validator: validator}
}

type installationDocument struct {
	PackID      string `json:"pack_id"`
	Version     string `json:"version"`
	ContentHash string `json:"content_hash"`
	InstalledAt string `json:"installed_at"`
}

type activationDocument struct {
	PackID      string `json:"pack_id"`
	Version     string `json:"version"`
	ContentHash string `json:"content_hash"`
	ActivatedAt string `json:"activated_at"`
}

func (repository *Repository) Add(ctx context.Context, artifact curriculumapp.PackInstallationArtifact) error {
	const operation = "store installed Learning Pack"
	if repository == nil || repository.root == nil || repository.validator == nil {
		return curriculumapp.Classify(curriculumapp.ErrorUnavailable, operation, fmt.Errorf("pack filesystem repository is not configured"))
	}
	if err := ctx.Err(); err != nil {
		return curriculumapp.Classify(curriculumapp.ErrorUnavailable, operation, err)
	}
	if err := artifact.Validate(); err != nil {
		return curriculumapp.Invalid(operation, err)
	}
	root, target, err := repository.packPath(artifact.Pack.Manifest.ID, artifact.Pack.Manifest.Version)
	if err != nil {
		return curriculumapp.Classify(curriculumapp.ErrorPersistenceFailure, operation, err)
	}
	if _, err := os.Lstat(target); err == nil {
		return curriculumapp.Classify(curriculumapp.ErrorConflict, operation, fmt.Errorf("pack version already exists"))
	} else if !errors.Is(err, fs.ErrNotExist) {
		return curriculumapp.Classify(curriculumapp.ErrorPersistenceFailure, operation, err)
	}
	if err := os.MkdirAll(filepath.Dir(target), 0o700); err != nil {
		return curriculumapp.Classify(curriculumapp.ErrorPersistenceFailure, operation, err)
	}
	staging, err := os.MkdirTemp(root, ".install-*")
	if err != nil {
		return curriculumapp.Classify(curriculumapp.ErrorPersistenceFailure, operation, err)
	}
	defer os.RemoveAll(staging)
	archivePath := filepath.Join(staging, archiveName)
	if err := os.WriteFile(archivePath, artifact.PortableArchive, 0o600); err != nil {
		return curriculumapp.Classify(curriculumapp.ErrorPersistenceFailure, operation, err)
	}
	reloaded, err := repository.validator.Validate(ctx, curriculumapp.PackSource{Path: archivePath})
	if err != nil {
		return curriculumapp.ExternalError(operation, err)
	}
	if err := sameValidatedArtifact(artifact, reloaded); err != nil {
		return curriculumapp.Invalid(operation, err)
	}
	document := installationDocument{
		PackID: artifact.Pack.Manifest.ID.String(), Version: artifact.Pack.Manifest.Version.String(),
		ContentHash: artifact.ContentHash, InstalledAt: artifact.InstalledAt.Time().Format(time.RFC3339Nano),
	}
	if err := writeJSONFile(filepath.Join(staging, metadataName), document); err != nil {
		return curriculumapp.Classify(curriculumapp.ErrorPersistenceFailure, operation, err)
	}
	if err := os.Rename(staging, target); err != nil {
		if _, statErr := os.Lstat(target); statErr == nil {
			return curriculumapp.Classify(curriculumapp.ErrorConflict, operation, fmt.Errorf("pack version already exists"))
		}
		return curriculumapp.Classify(curriculumapp.ErrorPersistenceFailure, operation, err)
	}
	return nil
}

func (repository *Repository) Get(ctx context.Context, id curriculum.ID, version curriculum.PackVersion) (curriculumapp.InstalledPack, error) {
	const operation = "load installed Learning Pack"
	if repository == nil || repository.root == nil || repository.validator == nil {
		return curriculumapp.InstalledPack{}, curriculumapp.Classify(curriculumapp.ErrorUnavailable, operation, fmt.Errorf("pack filesystem repository is not configured"))
	}
	if err := id.Validate(); err != nil {
		return curriculumapp.InstalledPack{}, curriculumapp.Invalid(operation, err)
	}
	if err := version.Validate(); err != nil {
		return curriculumapp.InstalledPack{}, curriculumapp.Invalid(operation, err)
	}
	_, target, err := repository.packPath(id, version)
	if err != nil {
		return curriculumapp.InstalledPack{}, curriculumapp.Classify(curriculumapp.ErrorPersistenceFailure, operation, err)
	}
	return repository.load(ctx, target, id, version)
}

func (repository *Repository) List(ctx context.Context) ([]curriculumapp.InstalledPack, error) {
	const operation = "list installed Learning Packs"
	if repository == nil || repository.root == nil || repository.validator == nil {
		return nil, curriculumapp.Classify(curriculumapp.ErrorUnavailable, operation, fmt.Errorf("pack filesystem repository is not configured"))
	}
	root, err := repository.root()
	if err != nil {
		return nil, curriculumapp.Classify(curriculumapp.ErrorPersistenceFailure, operation, err)
	}
	packDirs, err := os.ReadDir(root)
	if errors.Is(err, fs.ErrNotExist) {
		return []curriculumapp.InstalledPack{}, nil
	}
	if err != nil {
		return nil, curriculumapp.Classify(curriculumapp.ErrorPersistenceFailure, operation, err)
	}
	var result []curriculumapp.InstalledPack
	for _, packDir := range packDirs {
		if err := ctx.Err(); err != nil {
			return nil, curriculumapp.Classify(curriculumapp.ErrorUnavailable, operation, err)
		}
		if strings.HasPrefix(packDir.Name(), ".") {
			continue
		}
		if packDir.Type()&os.ModeSymlink != 0 || !packDir.IsDir() {
			return nil, curriculumapp.Classify(curriculumapp.ErrorPersistenceFailure, operation, fmt.Errorf("unexpected entry %q in pack store", packDir.Name()))
		}
		id, err := curriculum.NewID(packDir.Name())
		if err != nil {
			return nil, curriculumapp.Classify(curriculumapp.ErrorPersistenceFailure, operation, err)
		}
		versions, err := os.ReadDir(filepath.Join(root, packDir.Name()))
		if err != nil {
			return nil, curriculumapp.Classify(curriculumapp.ErrorPersistenceFailure, operation, err)
		}
		for _, versionDir := range versions {
			if versionDir.Type()&os.ModeSymlink != 0 || !versionDir.IsDir() {
				return nil, curriculumapp.Classify(curriculumapp.ErrorPersistenceFailure, operation, fmt.Errorf("unexpected entry %q for pack %q", versionDir.Name(), id))
			}
			version, err := curriculum.NewPackVersion(versionDir.Name())
			if err != nil {
				return nil, curriculumapp.Classify(curriculumapp.ErrorPersistenceFailure, operation, err)
			}
			installed, err := repository.load(ctx, filepath.Join(root, packDir.Name(), versionDir.Name()), id, version)
			if err != nil {
				return nil, err
			}
			result = append(result, installed)
		}
	}
	sort.Slice(result, func(i, j int) bool {
		left, right := result[i].Pack.Manifest, result[j].Pack.Manifest
		if left.ID != right.ID {
			return left.ID.String() < right.ID.String()
		}
		return left.Version.String() < right.Version.String()
	})
	return result, nil
}

func (repository *Repository) Activate(ctx context.Context, workspaceRoot string, activation curriculumapp.PackActivation) error {
	const operation = "store active Learning Pack"
	if err := activation.Validate(); err != nil {
		return curriculumapp.Invalid(operation, err)
	}
	installed, err := repository.Get(ctx, activation.PackID, activation.Version)
	if err != nil {
		return err
	}
	if activation.ActivatedAt.Time().Before(installed.InstalledAt.Time()) {
		return curriculumapp.Invalid(operation, fmt.Errorf("activation precedes installation"))
	}
	path, err := platform.WorkspacePackActivationPath(workspaceRoot)
	if err != nil {
		return curriculumapp.Classify(curriculumapp.ErrorPersistenceFailure, operation, err)
	}
	parent := filepath.Dir(path)
	info, err := os.Lstat(parent)
	if err != nil || !info.IsDir() || info.Mode()&os.ModeSymlink != 0 {
		return curriculumapp.Classify(curriculumapp.ErrorPersistenceFailure, operation, fmt.Errorf("workspace state directory is unavailable"))
	}
	document := activationDocument{
		PackID: activation.PackID.String(), Version: activation.Version.String(), ContentHash: installed.ContentHash,
		ActivatedAt: activation.ActivatedAt.Time().Format(time.RFC3339Nano),
	}
	return writeAtomicJSON(path, document)
}

func (repository *Repository) Active(ctx context.Context, workspaceRoot string) (curriculumapp.InstalledPack, error) {
	const operation = "load active Learning Pack"
	path, err := platform.WorkspacePackActivationPath(workspaceRoot)
	if err != nil {
		return curriculumapp.InstalledPack{}, curriculumapp.Classify(curriculumapp.ErrorPersistenceFailure, operation, err)
	}
	var document activationDocument
	if err := readStrictJSON(path, &document); errors.Is(err, fs.ErrNotExist) {
		return curriculumapp.InstalledPack{}, curriculumapp.Classify(curriculumapp.ErrorNotFound, operation, fmt.Errorf("workspace has no active pack"))
	} else if err != nil {
		return curriculumapp.InstalledPack{}, curriculumapp.Classify(curriculumapp.ErrorPersistenceFailure, operation, err)
	}
	id, err := curriculum.NewID(document.PackID)
	if err != nil {
		return curriculumapp.InstalledPack{}, curriculumapp.Classify(curriculumapp.ErrorPersistenceFailure, operation, err)
	}
	version, err := curriculum.NewPackVersion(document.Version)
	if err != nil {
		return curriculumapp.InstalledPack{}, curriculumapp.Classify(curriculumapp.ErrorPersistenceFailure, operation, err)
	}
	installed, err := repository.Get(ctx, id, version)
	if err != nil {
		return curriculumapp.InstalledPack{}, err
	}
	if installed.ContentHash != document.ContentHash {
		return curriculumapp.InstalledPack{}, curriculumapp.Classify(curriculumapp.ErrorPersistenceFailure, operation, fmt.Errorf("active pack content hash mismatch"))
	}
	if _, err := time.Parse(time.RFC3339Nano, document.ActivatedAt); err != nil {
		return curriculumapp.InstalledPack{}, curriculumapp.Classify(curriculumapp.ErrorPersistenceFailure, operation, fmt.Errorf("invalid activation time: %w", err))
	}
	return installed, nil
}

func (repository *Repository) load(ctx context.Context, directory string, expectedID curriculum.ID, expectedVersion curriculum.PackVersion) (curriculumapp.InstalledPack, error) {
	const operation = "load installed Learning Pack"
	info, err := os.Lstat(directory)
	if errors.Is(err, fs.ErrNotExist) {
		return curriculumapp.InstalledPack{}, curriculumapp.Classify(curriculumapp.ErrorNotFound, operation, fmt.Errorf("pack is not installed"))
	}
	if err != nil || !info.IsDir() || info.Mode()&os.ModeSymlink != 0 {
		return curriculumapp.InstalledPack{}, curriculumapp.Classify(curriculumapp.ErrorPersistenceFailure, operation, fmt.Errorf("installed pack directory is invalid"))
	}
	var document installationDocument
	if err := readStrictJSON(filepath.Join(directory, metadataName), &document); err != nil {
		return curriculumapp.InstalledPack{}, curriculumapp.Classify(curriculumapp.ErrorPersistenceFailure, operation, err)
	}
	installedAt, err := time.Parse(time.RFC3339Nano, document.InstalledAt)
	if err != nil {
		return curriculumapp.InstalledPack{}, curriculumapp.Classify(curriculumapp.ErrorPersistenceFailure, operation, fmt.Errorf("invalid installation time: %w", err))
	}
	timestamp, _ := curriculum.NewTimestamp(installedAt)
	archivePath := filepath.Join(directory, archiveName)
	archiveInfo, err := os.Lstat(archivePath)
	if err != nil || !archiveInfo.Mode().IsRegular() || archiveInfo.Mode()&os.ModeSymlink != 0 {
		return curriculumapp.InstalledPack{}, curriculumapp.Classify(curriculumapp.ErrorPersistenceFailure, operation, fmt.Errorf("installed pack archive is invalid"))
	}
	validation, err := repository.validator.Validate(ctx, curriculumapp.PackSource{Path: archivePath})
	if err != nil {
		return curriculumapp.InstalledPack{}, curriculumapp.ExternalError(operation, err)
	}
	if len(validation.Errors) > 0 || validation.Pack == nil {
		return curriculumapp.InstalledPack{}, curriculumapp.Classify(curriculumapp.ErrorPersistenceFailure, operation, fmt.Errorf("installed pack no longer validates"))
	}
	if document.PackID != expectedID.String() || document.Version != expectedVersion.String() ||
		validation.Pack.Manifest.ID != expectedID || validation.Pack.Manifest.Version != expectedVersion ||
		document.ContentHash != validation.ContentHash {
		return curriculumapp.InstalledPack{}, curriculumapp.Classify(curriculumapp.ErrorPersistenceFailure, operation, fmt.Errorf("installed pack identity or content hash mismatch"))
	}
	installed := curriculumapp.InstalledPack{Pack: *validation.Pack, ContentHash: validation.ContentHash, InstalledAt: timestamp}
	if err := installed.Validate(); err != nil {
		return curriculumapp.InstalledPack{}, curriculumapp.Classify(curriculumapp.ErrorPersistenceFailure, operation, err)
	}
	return installed, nil
}

func (repository *Repository) packPath(id curriculum.ID, version curriculum.PackVersion) (string, string, error) {
	if filepath.Base(id.String()) != id.String() || id.String() == "." || id.String() == ".." {
		return "", "", fmt.Errorf("pack id is not a portable path component")
	}
	root, err := repository.root()
	if err != nil {
		return "", "", err
	}
	root, err = filepath.Abs(root)
	if err != nil {
		return "", "", err
	}
	target := filepath.Join(root, id.String(), version.String())
	relative, err := filepath.Rel(root, target)
	if err != nil || relative == ".." || strings.HasPrefix(relative, ".."+string(filepath.Separator)) {
		return "", "", fmt.Errorf("pack path escapes global store")
	}
	return root, target, nil
}

func sameValidatedArtifact(expected curriculumapp.PackInstallationArtifact, actual curriculumapp.PackValidationResult) error {
	if len(actual.Errors) > 0 || actual.Pack == nil {
		return fmt.Errorf("stored archive failed validation")
	}
	if actual.ContentHash != expected.ContentHash || actual.Pack.Manifest.ID != expected.Pack.Manifest.ID || actual.Pack.Manifest.Version != expected.Pack.Manifest.Version {
		return fmt.Errorf("stored archive differs from validated artifact")
	}
	return nil
}

func writeJSONFile(path string, value any) error {
	encoded, err := json.MarshalIndent(value, "", "  ")
	if err != nil {
		return err
	}
	encoded = append(encoded, '\n')
	return os.WriteFile(path, encoded, 0o600)
}

func writeAtomicJSON(path string, value any) error {
	directory := filepath.Dir(path)
	temporary, err := os.CreateTemp(directory, ".active-pack-*.tmp")
	if err != nil {
		return curriculumapp.Classify(curriculumapp.ErrorPersistenceFailure, "store active Learning Pack", err)
	}
	temporaryPath := temporary.Name()
	defer os.Remove(temporaryPath)
	encoded, err := json.MarshalIndent(value, "", "  ")
	if err == nil {
		encoded = append(encoded, '\n')
		_, err = temporary.Write(encoded)
	}
	if err == nil {
		err = temporary.Chmod(0o600)
	}
	if err == nil {
		err = temporary.Sync()
	}
	closeErr := temporary.Close()
	if err == nil {
		err = closeErr
	}
	if err == nil {
		err = os.Rename(temporaryPath, path)
	}
	if err != nil {
		return curriculumapp.Classify(curriculumapp.ErrorPersistenceFailure, "store active Learning Pack", err)
	}
	return nil
}

func readStrictJSON(path string, target any) error {
	info, err := os.Lstat(path)
	if err != nil {
		return err
	}
	if !info.Mode().IsRegular() || info.Mode()&os.ModeSymlink != 0 {
		return fmt.Errorf("%s is not a regular file", path)
	}
	if info.Size() > 1<<20 {
		return fmt.Errorf("%s exceeds metadata size limit", path)
	}
	file, err := os.Open(path)
	if err != nil {
		return err
	}
	defer file.Close()
	decoder := json.NewDecoder(file)
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(target); err != nil {
		return err
	}
	var trailing any
	if err := decoder.Decode(&trailing); err == nil {
		return fmt.Errorf("%s contains trailing JSON", path)
	} else if !errors.Is(err, io.EOF) {
		return err
	}
	return nil
}

var _ curriculumapp.PackInstallationRepository = (*Repository)(nil)
