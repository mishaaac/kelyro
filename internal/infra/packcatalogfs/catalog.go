// Package packcatalogfs reads Pack Catalog v1 metadata documents and keeps the
// last validated snapshot in Foundation's global cache directory.
package packcatalogfs

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"time"

	"github.com/mishaaac/kelyro/internal/curriculum"
	curriculumapp "github.com/mishaaac/kelyro/internal/curriculum/application"
	"github.com/mishaaac/kelyro/internal/platform"
)

const maximumCatalogBytes = 8 << 20

type Cache struct{ path func() (string, error) }

func NewCache() *Cache { return &Cache{path: platform.GlobalPackCatalogCachePath} }

func (cache *Cache) Load(ctx context.Context) (curriculum.PackCatalogSnapshot, error) {
	if cache == nil || cache.path == nil {
		return curriculum.PackCatalogSnapshot{}, curriculumapp.Classify(curriculumapp.ErrorUnavailable, "load pack catalog cache", fmt.Errorf("catalog cache is not configured"))
	}
	path, err := cache.path()
	if err != nil {
		return curriculum.PackCatalogSnapshot{}, err
	}
	snapshot, err := loadDocument(ctx, path)
	if errors.Is(err, fs.ErrNotExist) {
		generated, _ := curriculum.NewTimestamp(time.Unix(0, 0).UTC())
		return curriculum.PackCatalogSnapshot{SchemaVersion: curriculum.PackCatalogSchemaVersionV1, GeneratedAt: generated, Entries: []curriculum.PackCatalogEntry{}}, nil
	}
	return snapshot, err
}

func (cache *Cache) Replace(ctx context.Context, snapshot curriculum.PackCatalogSnapshot) error {
	if cache == nil || cache.path == nil {
		return curriculumapp.Classify(curriculumapp.ErrorUnavailable, "replace pack catalog cache", fmt.Errorf("catalog cache is not configured"))
	}
	snapshot = curriculum.SortPackCatalog(snapshot)
	if err := snapshot.Validate(); err != nil {
		return curriculumapp.Invalid("replace pack catalog cache", err)
	}
	path, err := cache.path()
	if err != nil {
		return err
	}
	encoded, err := encodeDocument(snapshot)
	if err != nil {
		return err
	}
	if err := ctx.Err(); err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return err
	}
	temporary, err := os.CreateTemp(filepath.Dir(path), ".pack-catalog-*.tmp")
	if err != nil {
		return err
	}
	temporaryPath := temporary.Name()
	defer os.Remove(temporaryPath)
	writeErr := temporary.Chmod(0o600)
	if writeErr == nil {
		_, writeErr = temporary.Write(encoded)
		if writeErr == nil {
			writeErr = temporary.Sync()
		}
	}
	closeErr := temporary.Close()
	if writeErr == nil {
		writeErr = closeErr
	}
	if writeErr == nil {
		writeErr = os.Rename(temporaryPath, path)
	}
	return writeErr
}

type DocumentSource struct{ Path string }

func (source DocumentSource) Load(ctx context.Context) (curriculum.PackCatalogSnapshot, error) {
	if source.Path == "" {
		return curriculum.PackCatalogSnapshot{}, fmt.Errorf("pack catalog source path is empty")
	}
	return loadDocument(ctx, source.Path)
}

type catalogDocument struct {
	SchemaVersion string          `json:"schema_version"`
	GeneratedAt   string          `json:"generated_at"`
	Entries       []entryDocument `json:"entries"`
}

type entryDocument struct {
	PackID      string            `json:"pack_id"`
	Name        string            `json:"name"`
	Description string            `json:"description"`
	Domain      string            `json:"domain"`
	Target      string            `json:"target"`
	Maintainer  string            `json:"maintainer"`
	Versions    []versionDocument `json:"versions"`
	Source      sourceDocument    `json:"source"`
}

type versionDocument struct {
	Version              string `json:"version"`
	Status               string `json:"status"`
	MinimumKelyroVersion string `json:"minimum_kelyro_version"`
	Compatibility        string `json:"compatibility"`
	Artifact             string `json:"artifact"`
}

type sourceDocument struct {
	ID      string `json:"id"`
	Name    string `json:"name"`
	Locator string `json:"locator"`
	Trust   string `json:"trust"`
}

func loadDocument(ctx context.Context, path string) (curriculum.PackCatalogSnapshot, error) {
	if err := ctx.Err(); err != nil {
		return curriculum.PackCatalogSnapshot{}, err
	}
	info, err := os.Lstat(path)
	if err != nil {
		return curriculum.PackCatalogSnapshot{}, err
	}
	if !info.Mode().IsRegular() || info.Mode()&os.ModeSymlink != 0 {
		return curriculum.PackCatalogSnapshot{}, fmt.Errorf("pack catalog %s is not a regular file", path)
	}
	if info.Size() > maximumCatalogBytes {
		return curriculum.PackCatalogSnapshot{}, fmt.Errorf("pack catalog exceeds %d bytes", maximumCatalogBytes)
	}
	encoded, err := os.ReadFile(path)
	if err != nil {
		return curriculum.PackCatalogSnapshot{}, err
	}
	var document catalogDocument
	decoder := json.NewDecoder(bytes.NewReader(encoded))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&document); err != nil {
		return curriculum.PackCatalogSnapshot{}, fmt.Errorf("decode pack catalog: %w", err)
	}
	var trailing any
	if err := decoder.Decode(&trailing); !errors.Is(err, io.EOF) {
		if err == nil {
			err = fmt.Errorf("trailing JSON value")
		}
		return curriculum.PackCatalogSnapshot{}, fmt.Errorf("decode pack catalog: %w", err)
	}
	snapshot, err := decodeDocument(document)
	if err != nil {
		return curriculum.PackCatalogSnapshot{}, fmt.Errorf("decode pack catalog: %w", err)
	}
	return snapshot, nil
}

func decodeDocument(document catalogDocument) (curriculum.PackCatalogSnapshot, error) {
	generatedAt, err := time.Parse(time.RFC3339Nano, document.GeneratedAt)
	if err != nil || generatedAt.Location() != time.UTC {
		return curriculum.PackCatalogSnapshot{}, fmt.Errorf("generated_at must be RFC3339 UTC")
	}
	generated, _ := curriculum.NewTimestamp(generatedAt)
	snapshot := curriculum.PackCatalogSnapshot{SchemaVersion: document.SchemaVersion, GeneratedAt: generated, Entries: make([]curriculum.PackCatalogEntry, len(document.Entries))}
	for index, raw := range document.Entries {
		id, err := curriculum.NewID(raw.PackID)
		if err != nil {
			return curriculum.PackCatalogSnapshot{}, err
		}
		sourceID, err := curriculum.NewID(raw.Source.ID)
		if err != nil {
			return curriculum.PackCatalogSnapshot{}, err
		}
		entry := curriculum.PackCatalogEntry{
			PackID: id, Name: raw.Name, Description: raw.Description, Domain: raw.Domain, Target: raw.Target, Maintainer: raw.Maintainer,
			Source: curriculum.PackCatalogSourceMetadata{ID: sourceID, Name: raw.Source.Name, Locator: raw.Source.Locator, Trust: curriculum.PackCatalogTrust(raw.Source.Trust)},
		}
		for _, rawVersion := range raw.Versions {
			version, err := curriculum.NewPackVersion(rawVersion.Version)
			if err != nil {
				return curriculum.PackCatalogSnapshot{}, err
			}
			minimum, err := curriculum.NewPackVersion(rawVersion.MinimumKelyroVersion)
			if err != nil {
				return curriculum.PackCatalogSnapshot{}, err
			}
			entry.Versions = append(entry.Versions, curriculum.PackCatalogVersion{
				Version: version, Status: curriculum.PackStatus(rawVersion.Status), MinimumKelyroVersion: minimum,
				Compatibility: curriculum.PackCompatibility(rawVersion.Compatibility), Artifact: rawVersion.Artifact,
			})
		}
		snapshot.Entries[index] = entry
	}
	snapshot = curriculum.SortPackCatalog(snapshot)
	if err := snapshot.Validate(); err != nil {
		return curriculum.PackCatalogSnapshot{}, err
	}
	return snapshot, nil
}

func encodeDocument(snapshot curriculum.PackCatalogSnapshot) ([]byte, error) {
	document := catalogDocument{SchemaVersion: snapshot.SchemaVersion, GeneratedAt: snapshot.GeneratedAt.Time().Format(time.RFC3339Nano), Entries: make([]entryDocument, len(snapshot.Entries))}
	for index, entry := range snapshot.Entries {
		raw := entryDocument{
			PackID: entry.PackID.String(), Name: entry.Name, Description: entry.Description, Domain: entry.Domain, Target: entry.Target, Maintainer: entry.Maintainer,
			Source: sourceDocument{ID: entry.Source.ID.String(), Name: entry.Source.Name, Locator: entry.Source.Locator, Trust: string(entry.Source.Trust)},
		}
		for _, version := range entry.Versions {
			raw.Versions = append(raw.Versions, versionDocument{
				Version: version.Version.String(), Status: string(version.Status), MinimumKelyroVersion: version.MinimumKelyroVersion.String(),
				Compatibility: string(version.Compatibility), Artifact: version.Artifact,
			})
		}
		document.Entries[index] = raw
	}
	encoded, err := json.MarshalIndent(document, "", "  ")
	if err != nil {
		return nil, err
	}
	return append(encoded, '\n'), nil
}

var (
	_ curriculumapp.PackCatalogCache  = (*Cache)(nil)
	_ curriculumapp.PackCatalogSource = DocumentSource{}
)
