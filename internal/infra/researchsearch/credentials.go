package researchsearch

import (
	"errors"
	"fmt"

	"github.com/mishaaac/kelyro/internal/config"
	"github.com/mishaaac/kelyro/internal/storage"
)

const BraveAPIKeySecretName = "research.search.brave.api_key"

var (
	ErrCredentialMissing     = errors.New("brave search credential is missing")
	ErrCredentialUnavailable = errors.New("brave search credential store is unavailable")
	ErrCredentialInvalid     = errors.New("brave search credential is invalid")
)

// SecretReader is the narrow Foundation Secrets boundary needed to construct
// the provider. The secret value never crosses back out of this package.
type SecretReader interface {
	Get(string) (string, error)
}

type CredentialState string

const (
	CredentialAvailable   CredentialState = "available"
	CredentialMissing     CredentialState = "missing"
	CredentialUnavailable CredentialState = "unavailable"
	CredentialInvalid     CredentialState = "invalid"
)

// BraveReadiness contains capability states only. String deliberately renders
// the two safe lines allowed at presentation and diagnostic boundaries.
type BraveReadiness struct {
	providerConfigured bool
	credential         CredentialState
}

func (readiness BraveReadiness) IsProviderConfigured() bool {
	return readiness.providerConfigured
}

func (readiness BraveReadiness) CredentialState() CredentialState {
	return readiness.credential
}

func (readiness BraveReadiness) String() string {
	provider := "unavailable"
	if readiness.providerConfigured {
		provider = "configured"
	}
	credential := CredentialUnavailable
	switch readiness.credential {
	case CredentialAvailable, CredentialMissing, CredentialUnavailable, CredentialInvalid:
		credential = readiness.credential
	}
	return fmt.Sprintf("Provider: %s\nCredential: %s", provider, credential)
}

// ConfigAvailability projects safe capabilities into the provider-neutral
// configuration readiness model without carrying the credential value.
func (readiness BraveReadiness) ConfigAvailability() config.ResearchSearchAvailability {
	return config.ResearchSearchAvailability{
		AdapterAvailable:    readiness.providerConfigured,
		CredentialRequired:  true,
		CredentialAvailable: readiness.credential == CredentialAvailable,
	}
}

// NewBraveFromSecrets resolves the one documented secret reference and keeps
// its value only in the in-memory adapter. It never enumerates or renders the
// secret store and sanitizes backend errors before returning them.
func NewBraveFromSecrets(client HTTPClient, secrets SecretReader, options ...Option) (*Brave, BraveReadiness, error) {
	readiness := BraveReadiness{providerConfigured: client != nil, credential: CredentialUnavailable}
	if client == nil {
		return nil, readiness, errors.New("brave search HTTP client is unavailable")
	}
	if secrets == nil {
		return nil, readiness, ErrCredentialUnavailable
	}

	token, err := secrets.Get(BraveAPIKeySecretName)
	if err != nil {
		switch {
		case errors.Is(err, storage.ErrSecretNotFound):
			readiness.credential = CredentialMissing
			return nil, readiness, fmt.Errorf("%w: %w", ErrCredentialMissing, storage.ErrSecretNotFound)
		case errors.Is(err, storage.ErrSecretStoreUnavailable):
			return nil, readiness, fmt.Errorf("%w: %w", ErrCredentialUnavailable, storage.ErrSecretStoreUnavailable)
		default:
			return nil, readiness, ErrCredentialUnavailable
		}
	}
	if token == "" {
		readiness.credential = CredentialMissing
		return nil, readiness, ErrCredentialMissing
	}
	if err := validateToken(token); err != nil {
		readiness.credential = CredentialInvalid
		return nil, readiness, ErrCredentialInvalid
	}

	provider, err := NewBrave(client, token, options...)
	if err != nil {
		readiness.credential = CredentialInvalid
		return nil, readiness, ErrCredentialInvalid
	}
	readiness.credential = CredentialAvailable
	return provider, readiness, nil
}
