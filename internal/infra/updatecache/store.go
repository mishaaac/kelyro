// Package updatecache persists disposable update-check metadata in the native
// user cache directory.
package updatecache

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

	"github.com/mishaaac/kelyro/internal/platform"
	"github.com/mishaaac/kelyro/internal/update"
)

const (
	schemaVersion = 1
	maxCacheSize  = 64 * 1024
)

type document struct {
	SchemaVersion int                                   `json:"schema_version"`
	Checks        map[update.Channel]update.CachedCheck `json:"checks"`
}

// Store implements update.Cache using one bounded, versioned JSON document.
type Store struct {
	path     func() (string, error)
	readFile func(string) ([]byte, error)
	rename   func(string, string) error
}

func New() *Store {
	return &Store{path: platform.GlobalUpdateCachePath, readFile: os.ReadFile, rename: replaceFile}
}

func (store *Store) Load(ctx context.Context, channel update.Channel) (update.CachedCheck, bool, error) {
	if err := ctx.Err(); err != nil {
		return update.CachedCheck{}, false, err
	}
	if !channel.Valid() {
		return update.CachedCheck{}, false, fmt.Errorf("invalid update cache channel %q", channel)
	}
	path, err := store.path()
	if err != nil {
		return update.CachedCheck{}, false, err
	}
	doc, found, err := store.loadDocument(path)
	if err != nil || !found {
		return update.CachedCheck{}, false, err
	}
	check, found := doc.Checks[channel]
	if !found {
		return update.CachedCheck{}, false, nil
	}
	if err := validateCheck(check); err != nil {
		return update.CachedCheck{}, false, fmt.Errorf("invalid update cache %s: %w", path, err)
	}
	return check, true, nil
}

func (store *Store) Save(ctx context.Context, check update.CachedCheck) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	if err := validateCheck(check); err != nil {
		return err
	}
	path, err := store.path()
	if err != nil {
		return err
	}
	doc, found, loadErr := store.loadDocument(path)
	if loadErr != nil || !found {
		doc = document{SchemaVersion: schemaVersion, Checks: make(map[update.Channel]update.CachedCheck)}
	}
	if doc.Checks == nil {
		doc.Checks = make(map[update.Channel]update.CachedCheck)
	}
	doc.Checks[check.Channel] = check
	encoded, err := json.MarshalIndent(doc, "", "  ")
	if err != nil {
		return fmt.Errorf("encode update cache: %w", err)
	}
	encoded = append(encoded, '\n')
	return store.writeAtomic(path, encoded)
}

func (store *Store) loadDocument(path string) (document, bool, error) {
	encoded, err := store.readFile(path)
	if errors.Is(err, fs.ErrNotExist) {
		return document{}, false, nil
	}
	if err != nil {
		return document{}, false, fmt.Errorf("read update cache %s: %w", path, err)
	}
	if len(encoded) > maxCacheSize {
		return document{}, false, fmt.Errorf("read update cache %s: file exceeds %d bytes", path, maxCacheSize)
	}
	decoder := json.NewDecoder(bytes.NewReader(encoded))
	decoder.DisallowUnknownFields()
	var doc document
	if err := decoder.Decode(&doc); err != nil {
		return document{}, false, fmt.Errorf("decode update cache %s: %w", path, err)
	}
	if err := ensureJSONEOF(decoder); err != nil {
		return document{}, false, fmt.Errorf("decode update cache %s: %w", path, err)
	}
	if doc.SchemaVersion != schemaVersion {
		return document{}, false, fmt.Errorf("unsupported update cache schema_version %d", doc.SchemaVersion)
	}
	return doc, true, nil
}

func validateCheck(check update.CachedCheck) error {
	if !check.Channel.Valid() {
		return fmt.Errorf("invalid update cache channel %q", check.Channel)
	}
	if check.CheckedAt.IsZero() {
		return errors.New("update cache timestamp is empty")
	}
	if check.Found && check.Release.Version == "" {
		return errors.New("cached release version is empty")
	}
	return nil
}

func ensureJSONEOF(decoder *json.Decoder) error {
	var extra any
	err := decoder.Decode(&extra)
	if errors.Is(err, io.EOF) {
		return nil
	}
	if err == nil {
		return errors.New("unexpected data after JSON document")
	}
	return err
}

func (store *Store) writeAtomic(path string, encoded []byte) (err error) {
	directory := filepath.Dir(path)
	if err := os.MkdirAll(directory, 0o700); err != nil {
		return fmt.Errorf("create update cache directory: %w", err)
	}
	temporary, err := os.CreateTemp(directory, ".updates-*.tmp")
	if err != nil {
		return fmt.Errorf("create update cache staging file: %w", err)
	}
	temporaryPath := temporary.Name()
	defer func() {
		_ = temporary.Close()
		_ = os.Remove(temporaryPath)
	}()
	if err := temporary.Chmod(0o600); err != nil {
		return fmt.Errorf("secure update cache staging file: %w", err)
	}
	if _, err := temporary.Write(encoded); err != nil {
		return fmt.Errorf("write update cache staging file: %w", err)
	}
	if err := temporary.Sync(); err != nil {
		return fmt.Errorf("sync update cache staging file: %w", err)
	}
	if err := temporary.Close(); err != nil {
		return fmt.Errorf("close update cache staging file: %w", err)
	}
	if err := store.rename(temporaryPath, path); err != nil {
		return fmt.Errorf("commit update cache: %w", err)
	}
	return nil
}
