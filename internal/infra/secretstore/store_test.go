package secretstore

import (
	"errors"
	"strings"
	"testing"

	"github.com/mishaaac/kelyro/internal/storage"
)

func TestEnvironmentValueOverridesNativeKeychain(t *testing.T) {
	t.Parallel()

	native := newMemoryBackend()
	native.values["openai"] = "keychain-value"
	store := newStore(native, nil, func(name string) (string, bool) {
		if name == "KELYRO_SECRET_OPENAI" {
			return "environment-value", true
		}
		return "", false
	}, func() []string { return []string{"KELYRO_SECRET_OPENAI=environment-value"} })

	value, err := store.Get("openai")
	if err != nil {
		t.Fatalf("Get() error = %v", err)
	}
	if value != "environment-value" {
		t.Fatalf("Get() = %q, want environment value", value)
	}
	if native.gets["openai"] != 0 {
		t.Fatalf("native Get(openai) calls = %d, want 0", native.gets["openai"])
	}
}

func TestSetAndDeleteKeepReferenceIndexInKeychain(t *testing.T) {
	t.Parallel()

	native := newMemoryBackend()
	store := newStore(native, nil, noEnvironment, emptyEnvironment)
	secret := "sensitive-test-token"
	if err := store.Set("provider.token", secret); err != nil {
		t.Fatalf("Set() error = %v", err)
	}
	if native.values["provider.token"] != secret {
		t.Fatal("Set() did not persist value in fake keychain")
	}
	if index := native.values[secretIndexReference]; index != `["provider.token"]` {
		t.Fatalf("index = %q", index)
	}

	statuses, err := store.Status()
	if err != nil {
		t.Fatalf("Status() error = %v", err)
	}
	if len(statuses) != 1 || !statuses[0].Configured || statuses[0].Reference != "keychain:kelyro/provider.token" {
		t.Fatalf("Status() = %#v", statuses)
	}
	if strings.Contains(statuses[0].Reference, secret) {
		t.Fatal("status reference contains secret value")
	}

	if err := store.Delete("provider.token"); err != nil {
		t.Fatalf("Delete() error = %v", err)
	}
	if len(native.values) != 0 {
		t.Fatalf("fake keychain values after delete = %#v", native.values)
	}
}

func TestStatusReportsOnlyStateAndReferencesForEnvironment(t *testing.T) {
	t.Parallel()

	secret := "fixture-value-that-must-not-render"
	store := newStore(nil, errors.New("headless session"), func(name string) (string, bool) {
		if name == "KELYRO_SECRET_GITHUB_TOKEN" {
			return secret, true
		}
		return "", false
	}, func() []string { return []string{"KELYRO_SECRET_GITHUB_TOKEN=" + secret} })

	statuses, err := store.Status()
	if err != nil {
		t.Fatalf("Status() error = %v", err)
	}
	if len(statuses) != 1 || statuses[0].Name != "github_token" || !statuses[0].Configured || statuses[0].Reference != "KELYRO_SECRET_GITHUB_TOKEN" {
		t.Fatalf("Status() = %#v", statuses)
	}
	if strings.Contains(statuses[0].Reference, secret) {
		t.Fatal("status exposed secret value")
	}
	if err := store.Availability(); !errors.Is(err, storage.ErrSecretStoreUnavailable) || !strings.Contains(err.Error(), environmentPattern) {
		t.Fatalf("Availability() error = %v", err)
	}
}

func TestUnavailableStoreExplainsEnvironmentFallback(t *testing.T) {
	t.Parallel()

	store := newStore(nil, errors.New("secret-tool not found"), noEnvironment, emptyEnvironment)
	err := store.Set("openai", "never-persist-this")
	if !errors.Is(err, storage.ErrSecretStoreUnavailable) {
		t.Fatalf("Set() error = %v", err)
	}
	if !strings.Contains(err.Error(), "KELYRO_SECRET_OPENAI") {
		t.Fatalf("Set() error lacks environment hint: %v", err)
	}
	if strings.Contains(err.Error(), "never-persist-this") {
		t.Fatal("Set() error exposed secret")
	}
}

func TestStatusDegradesRuntimeKeychainFailureToEnvironmentGuidance(t *testing.T) {
	t.Parallel()

	native := newMemoryBackend()
	native.getErr[secretIndexReference] = errors.New("session bus is unavailable")
	store := newStore(native, nil, noEnvironment, emptyEnvironment)
	statuses, err := store.Status()
	if err != nil {
		t.Fatalf("Status() error = %v", err)
	}
	if len(statuses) != 1 || statuses[0].Configured || statuses[0].Reference != environmentPattern {
		t.Fatalf("Status() = %#v", statuses)
	}
	if err := store.Availability(); !errors.Is(err, storage.ErrSecretStoreUnavailable) || !strings.Contains(err.Error(), environmentPattern) {
		t.Fatalf("Availability() error = %v", err)
	}
}

func TestSetRedactsBackendErrors(t *testing.T) {
	t.Parallel()

	secret := "backend-echoed-sensitive-value"
	native := newMemoryBackend()
	native.setErr = errors.New("rejected " + secret)
	store := newStore(native, nil, noEnvironment, emptyEnvironment)
	err := store.Set("openai", secret)
	if err == nil {
		t.Fatal("Set() error = nil")
	}
	if strings.Contains(err.Error(), secret) || !strings.Contains(err.Error(), "[REDACTED]") {
		t.Fatalf("Set() error = %q", err)
	}
}

func TestSecretNamesAreValidated(t *testing.T) {
	t.Parallel()

	store := newStore(newMemoryBackend(), nil, noEnvironment, emptyEnvironment)
	for _, name := range []string{"", "../token", "token with spaces", "_reserved"} {
		if err := store.Set(name, "value"); err == nil {
			t.Errorf("Set(%q) error = nil", name)
		}
	}
}

type memoryBackend struct {
	values map[string]string
	gets   map[string]int
	getErr map[string]error
	setErr error
}

func newMemoryBackend() *memoryBackend {
	return &memoryBackend{values: make(map[string]string), gets: make(map[string]int), getErr: make(map[string]error)}
}

func (backend *memoryBackend) Get(name string) (string, error) {
	backend.gets[name]++
	if err := backend.getErr[name]; err != nil {
		return "", err
	}
	value, found := backend.values[name]
	if !found {
		return "", storage.ErrSecretNotFound
	}
	return value, nil
}

func (backend *memoryBackend) Set(name, value string) error {
	if backend.setErr != nil {
		return backend.setErr
	}
	backend.values[name] = value
	return nil
}

func (backend *memoryBackend) Delete(name string) error {
	if _, found := backend.values[name]; !found {
		return storage.ErrSecretNotFound
	}
	delete(backend.values, name)
	return nil
}

func noEnvironment(string) (string, bool) { return "", false }
func emptyEnvironment() []string          { return nil }
