//go:build linux

package secretstore

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/mishaaac/kelyro/internal/storage"
)

func TestLinuxSecretServiceTreatsEmptyExitOneAsNotFound(t *testing.T) {
	backend := linuxSecretService{path: writeSecretToolFixture(t, `#!/bin/sh
exit 1
`)}

	if _, err := backend.Get("missing"); !errors.Is(err, storage.ErrSecretNotFound) {
		t.Fatalf("Get() error = %v, want secret not found", err)
	}
	if err := backend.Delete("missing"); !errors.Is(err, storage.ErrSecretNotFound) {
		t.Fatalf("Delete() error = %v, want secret not found", err)
	}
}

func TestLinuxSecretServicePreservesDiagnosticFailures(t *testing.T) {
	backend := linuxSecretService{path: writeSecretToolFixture(t, `#!/bin/sh
echo 'session bus unavailable' >&2
exit 1
`)}

	_, err := backend.Get("missing")
	if err == nil || errors.Is(err, storage.ErrSecretNotFound) || !strings.Contains(err.Error(), "session bus unavailable") {
		t.Fatalf("Get() error = %v, want actionable backend failure", err)
	}
}

func TestLinuxSecretStoreCanWriteFirstEntry(t *testing.T) {
	backend := linuxSecretService{path: writeSecretToolFixture(t, `#!/bin/sh
if [ "$1" = "lookup" ]; then
  exit 1
fi
exit 0
`)}
	store := newStore(backend, nil, noEnvironment, emptyEnvironment)

	if err := store.Set("provider.token", "fixture-token"); err != nil {
		t.Fatalf("Set() first keychain entry error = %v", err)
	}
}

func writeSecretToolFixture(t *testing.T, contents string) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "secret-tool")
	if err := os.WriteFile(path, []byte(contents), 0o700); err != nil {
		t.Fatal(err)
	}
	return path
}
