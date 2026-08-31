package researchsearch

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"strings"
	"testing"

	"github.com/mishaaac/kelyro/internal/config"
	"github.com/mishaaac/kelyro/internal/research/application"
	"github.com/mishaaac/kelyro/internal/storage"
)

func TestNewBraveFromSecretsResolvesOnlyDocumentedReference(t *testing.T) {
	t.Parallel()
	const secret = "fixture-secret-value"
	secrets := &recordingSecretReader{value: secret}
	var sentToken string
	client := roundTripFunc(func(request *http.Request) (*http.Response, error) {
		sentToken = request.Header.Get(credentialHeader)
		return jsonResponse(http.StatusOK, `{"web":{"results":[]}}`, nil), nil
	})

	provider, readiness, err := NewBraveFromSecrets(client, secrets)
	if err != nil {
		t.Fatalf("NewBraveFromSecrets() error = %v", err)
	}
	if provider == nil {
		t.Fatal("NewBraveFromSecrets() provider is nil")
	}
	if len(secrets.names) != 1 || secrets.names[0] != BraveAPIKeySecretName {
		t.Fatalf("secret references = %v", secrets.names)
	}
	if got := readiness.String(); got != "Provider: configured\nCredential: available" {
		t.Fatalf("readiness = %q", got)
	}
	availability := readiness.ConfigAvailability()
	search := config.ResearchSearchConfig{Provider: ProviderID, MaxResultsPerQuery: 8, MaxQueriesPerRun: 4}
	if state := search.State(availability); state != config.ResearchSearchConfigured {
		t.Fatalf("configuration state = %q", state)
	}

	if _, err := provider.Search(context.Background(), validQuery(t), application.SearchOptions{Limit: 1}); err != nil {
		t.Fatal(err)
	}
	if sentToken != secret {
		t.Fatal("resolved credential was not supplied to the adapter")
	}
	if strings.Contains(readiness.String(), secret) {
		t.Fatal("safe readiness exposed the credential")
	}
	if rendered := fmt.Sprintf("%v %+v %#v", provider, provider, provider); strings.Contains(rendered, secret) {
		t.Fatalf("provider diagnostic formatting exposed the credential: %q", rendered)
	}
}

func TestNewBraveFromSecretsClassifiesMissingUnavailableAndInvalidCredentials(t *testing.T) {
	t.Parallel()
	const secret = "backend-echoed-secret"
	tests := []struct {
		name       string
		reader     SecretReader
		wantError  error
		wantState  CredentialState
		wantConfig config.ResearchSearchProviderState
	}{
		{name: "missing", reader: &recordingSecretReader{err: storage.ErrSecretNotFound}, wantError: ErrCredentialMissing, wantState: CredentialMissing, wantConfig: config.ResearchSearchMissingCredential},
		{name: "empty", reader: &recordingSecretReader{}, wantError: ErrCredentialMissing, wantState: CredentialMissing, wantConfig: config.ResearchSearchMissingCredential},
		{name: "backend unavailable", reader: &recordingSecretReader{err: fmtError(storage.ErrSecretStoreUnavailable, secret)}, wantError: ErrCredentialUnavailable, wantState: CredentialUnavailable, wantConfig: config.ResearchSearchMissingCredential},
		{name: "unknown backend error", reader: &recordingSecretReader{err: errors.New(secret)}, wantError: ErrCredentialUnavailable, wantState: CredentialUnavailable, wantConfig: config.ResearchSearchMissingCredential},
		{name: "invalid whitespace", reader: &recordingSecretReader{value: "invalid secret"}, wantError: ErrCredentialInvalid, wantState: CredentialInvalid, wantConfig: config.ResearchSearchMissingCredential},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			provider, readiness, err := NewBraveFromSecrets(staticClient(nil), test.reader)
			if provider != nil || !errors.Is(err, test.wantError) {
				t.Fatalf("provider/error = %v/%v, want nil/%v", provider, err, test.wantError)
			}
			if readiness.CredentialState() != test.wantState {
				t.Fatalf("credential state = %q, want %q", readiness.CredentialState(), test.wantState)
			}
			search := config.ResearchSearchConfig{Provider: ProviderID, MaxResultsPerQuery: 8, MaxQueriesPerRun: 4}
			if state := search.State(readiness.ConfigAvailability()); state != test.wantConfig {
				t.Fatalf("configuration state = %q, want %q", state, test.wantConfig)
			}
			if strings.Contains(err.Error(), secret) || strings.Contains(readiness.String(), secret) {
				t.Fatalf("credential escaped through error/readiness: %v / %q", err, readiness)
			}
		})
	}
}

func TestNewBraveFromSecretsDoesNotReadStoreWithoutProviderTransport(t *testing.T) {
	t.Parallel()
	secrets := &recordingSecretReader{value: "fixture-secret"}
	provider, readiness, err := NewBraveFromSecrets(nil, secrets)
	if err == nil || provider != nil {
		t.Fatalf("provider/error = %v/%v", provider, err)
	}
	if readiness.IsProviderConfigured() || len(secrets.names) != 0 {
		t.Fatalf("readiness/names = %+v/%v", readiness, secrets.names)
	}
	if got := readiness.String(); got != "Provider: unavailable\nCredential: unavailable" {
		t.Fatalf("readiness = %q", got)
	}

	_, readiness, err = NewBraveFromSecrets(staticClient(nil), nil)
	if !errors.Is(err, ErrCredentialUnavailable) || readiness.String() != "Provider: configured\nCredential: unavailable" {
		t.Fatalf("nil store readiness/error = %q/%v", readiness, err)
	}
}

type recordingSecretReader struct {
	value string
	err   error
	names []string
}

func (reader *recordingSecretReader) Get(name string) (string, error) {
	reader.names = append(reader.names, name)
	return reader.value, reader.err
}

type wrappedError struct {
	target error
	text   string
}

func (err wrappedError) Error() string { return err.text }
func (err wrappedError) Unwrap() error { return err.target }

func fmtError(target error, text string) error {
	return wrappedError{target: target, text: text}
}
