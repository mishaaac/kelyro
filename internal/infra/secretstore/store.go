// Package secretstore provides environment and operating-system credential
// adapters for Kelyro's neutral secret storage contract.
package secretstore

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"sort"
	"strings"
	"sync"
	"unicode"

	"github.com/mishaaac/kelyro/internal/storage"
)

const (
	serviceName          = "kelyro"
	environmentPrefix    = "KELYRO_SECRET_"
	environmentPattern   = environmentPrefix + "<NAME>"
	secretIndexReference = "__kelyro_secret_index__"
)

type nativeBackend interface {
	Get(name string) (string, error)
	Set(name, value string) error
	Delete(name string) error
}

// Store resolves environment variables before the native keychain. Values are
// never written to files; the native keychain also holds the non-sensitive
// index used by status commands.
type Store struct {
	native      nativeBackend
	nativeErr   error
	lookupEnv   func(string) (string, bool)
	environment func() []string
	statusMu    sync.RWMutex
	statusErr   error
}

// New selects the credential backend native to the current operating system.
// An unavailable backend is retained as actionable status rather than causing
// application startup to fail, so environment-only use remains possible.
func New() *Store {
	native, err := newNativeBackend()
	return &Store{
		native:      native,
		nativeErr:   err,
		lookupEnv:   os.LookupEnv,
		environment: os.Environ,
	}
}

func newStore(native nativeBackend, nativeErr error, lookup func(string) (string, bool), environment func() []string) *Store {
	return &Store{native: native, nativeErr: nativeErr, lookupEnv: lookup, environment: environment}
}

// EnvironmentReference returns the deterministic environment-variable name
// for a validated secret name.
func EnvironmentReference(name string) string {
	var reference strings.Builder
	reference.WriteString(environmentPrefix)
	for _, character := range name {
		switch {
		case unicode.IsLetter(character), unicode.IsDigit(character):
			reference.WriteRune(unicode.ToUpper(character))
		default:
			reference.WriteByte('_')
		}
	}
	return reference.String()
}

func (store *Store) Get(name string) (string, error) {
	if err := validateName(name); err != nil {
		return "", err
	}
	if value, configured := store.lookupEnv(EnvironmentReference(name)); configured && value != "" {
		return value, nil
	}
	if store.native == nil {
		return "", store.unavailable(name)
	}
	value, err := store.native.Get(name)
	if err != nil {
		if !errors.Is(err, storage.ErrSecretNotFound) {
			return "", store.backendUnavailable(name, err)
		}
		return "", fmt.Errorf("read secret %q from OS keychain: %w", name, err)
	}
	return value, nil
}

func (store *Store) Set(name, value string) error {
	if err := validateName(name); err != nil {
		return err
	}
	if value == "" {
		return fmt.Errorf("secret value must not be empty")
	}
	if store.native == nil {
		return store.unavailable(name)
	}

	names, err := store.index()
	if err != nil {
		var accessErr nativeAccessError
		if errors.As(err, &accessErr) {
			return store.backendUnavailable(name, accessErr.err)
		}
		return fmt.Errorf("read secret reference index: %w", err)
	}
	previous, previousErr := store.native.Get(name)
	hadPrevious := previousErr == nil
	if previousErr != nil && !errors.Is(previousErr, storage.ErrSecretNotFound) {
		return store.backendUnavailable(name, previousErr)
	}
	if err := store.native.Set(name, value); err != nil {
		return store.backendUnavailable(name, redactedError(err, value))
	}
	if contains(names, name) {
		return nil
	}

	updated := append(append([]string(nil), names...), name)
	sort.Strings(updated)
	if err := store.saveIndex(updated); err != nil {
		rollbackErr := store.native.Delete(name)
		if hadPrevious {
			rollbackErr = store.native.Set(name, previous)
		}
		if rollbackErr != nil {
			cause := redactedError(fmt.Errorf("update reference index: %v (also failed to restore prior keychain state: %v)", err, rollbackErr), value, previous)
			return store.backendUnavailable(name, cause)
		}
		return store.backendUnavailable(name, err)
	}
	return nil
}

func (store *Store) Delete(name string) error {
	if err := validateName(name); err != nil {
		return err
	}
	if store.native == nil {
		return store.unavailable(name)
	}
	if err := store.native.Delete(name); err != nil && !errors.Is(err, storage.ErrSecretNotFound) {
		return store.backendUnavailable(name, err)
	}

	names, err := store.index()
	if err != nil {
		var accessErr nativeAccessError
		if errors.As(err, &accessErr) {
			return store.backendUnavailable(name, accessErr.err)
		}
		return fmt.Errorf("read secret reference index: %w", err)
	}
	filtered := make([]string, 0, len(names))
	for _, candidate := range names {
		if candidate != name {
			filtered = append(filtered, candidate)
		}
	}
	if len(filtered) != len(names) {
		if err := store.saveIndex(filtered); err != nil {
			return store.backendUnavailable(name, err)
		}
	}
	return nil
}

func (store *Store) Status() ([]storage.SecretStatus, error) {
	names := make(map[string]string)
	nativeUsable := store.native != nil
	if store.native != nil {
		indexed, err := store.index()
		if err != nil {
			var accessErr nativeAccessError
			if errors.As(err, &accessErr) {
				nativeUsable = false
				store.recordStatusError(accessErr.err)
			} else {
				return nil, fmt.Errorf("read secret reference index: %w", err)
			}
		} else {
			store.recordStatusError(nil)
		}
		for _, name := range indexed {
			names[name] = ""
		}
	}
	for _, assignment := range store.environment() {
		reference, _, found := strings.Cut(assignment, "=")
		if !found || !strings.HasPrefix(reference, environmentPrefix) {
			continue
		}
		name := strings.ToLower(strings.TrimPrefix(reference, environmentPrefix))
		for indexed := range names {
			if EnvironmentReference(indexed) == reference {
				name = indexed
				break
			}
		}
		if validateName(name) == nil {
			names[name] = reference
		}
	}

	if len(names) == 0 {
		return []storage.SecretStatus{{Name: "<name>", Reference: environmentPattern}}, nil
	}
	ordered := make([]string, 0, len(names))
	for name := range names {
		ordered = append(ordered, name)
	}
	sort.Strings(ordered)

	statuses := make([]storage.SecretStatus, 0, len(ordered))
	for _, name := range ordered {
		reference := names[name]
		if reference == "" {
			reference = EnvironmentReference(name)
		}
		if value, configured := store.lookupEnv(reference); configured && value != "" {
			statuses = append(statuses, storage.SecretStatus{Name: name, Reference: reference, Configured: true})
			continue
		}
		configured := false
		if nativeUsable {
			_, err := store.native.Get(name)
			switch {
			case err == nil:
				configured = true
				reference = "keychain:" + serviceName + "/" + name
			case errors.Is(err, storage.ErrSecretNotFound):
			default:
				nativeUsable = false
				store.recordStatusError(err)
			}
		}
		statuses = append(statuses, storage.SecretStatus{Name: name, Reference: reference, Configured: configured})
	}
	return statuses, nil
}

func (store *Store) Availability() error {
	if store.native == nil {
		return store.unavailable("<name>")
	}
	store.statusMu.RLock()
	err := store.statusErr
	store.statusMu.RUnlock()
	if err != nil {
		return store.backendUnavailable("<name>", err)
	}
	return nil
}

func (store *Store) index() ([]string, error) {
	encoded, err := store.native.Get(secretIndexReference)
	if errors.Is(err, storage.ErrSecretNotFound) {
		return nil, nil
	}
	if err != nil {
		return nil, nativeAccessError{err: err}
	}
	var names []string
	if err := json.Unmarshal([]byte(encoded), &names); err != nil {
		return nil, fmt.Errorf("invalid keychain index: %w", err)
	}
	for _, name := range names {
		if err := validateName(name); err != nil {
			return nil, fmt.Errorf("invalid keychain index entry: %w", err)
		}
	}
	return names, nil
}

func (store *Store) saveIndex(names []string) error {
	if len(names) == 0 {
		err := store.native.Delete(secretIndexReference)
		if errors.Is(err, storage.ErrSecretNotFound) {
			return nil
		}
		return err
	}
	encoded, err := json.Marshal(names)
	if err != nil {
		return fmt.Errorf("encode keychain index: %w", err)
	}
	return store.native.Set(secretIndexReference, string(encoded))
}

func (store *Store) unavailable(name string) error {
	cause := store.nativeErr
	if cause == nil {
		cause = storage.ErrSecretStoreUnavailable
	}
	return store.backendUnavailable(name, cause)
}

func (store *Store) backendUnavailable(name string, cause error) error {
	reference := EnvironmentReference(name)
	if name == "<name>" {
		reference = environmentPattern
	}
	return fmt.Errorf("%w: %v; use environment variable %s instead", storage.ErrSecretStoreUnavailable, cause, reference)
}

func (store *Store) recordStatusError(err error) {
	store.statusMu.Lock()
	store.statusErr = err
	store.statusMu.Unlock()
}

func validateName(name string) error {
	if len(name) == 0 || len(name) > 64 {
		return fmt.Errorf("secret name must contain between 1 and 64 characters")
	}
	for index, character := range name {
		valid := character >= 'a' && character <= 'z' || character >= 'A' && character <= 'Z' || character >= '0' && character <= '9' || character == '.' || character == '_' || character == '-'
		if !valid || index == 0 && (character == '.' || character == '_' || character == '-') {
			return fmt.Errorf("invalid secret name %q: use letters, digits, dots, underscores, or hyphens", name)
		}
	}
	return nil
}

func contains(values []string, candidate string) bool {
	for _, value := range values {
		if value == candidate {
			return true
		}
	}
	return false
}

func trimLineEnding(value string) string {
	value = strings.TrimSuffix(value, "\n")
	return strings.TrimSuffix(value, "\r")
}

type safeError string

func (err safeError) Error() string { return string(err) }

func redactedError(err error, sensitive ...string) error {
	return safeError(storage.Redact(err.Error(), sensitive...))
}

type nativeAccessError struct {
	err error
}

func (err nativeAccessError) Error() string { return err.err.Error() }
func (err nativeAccessError) Unwrap() error { return err.err }

var _ storage.SecretStore = (*Store)(nil)
