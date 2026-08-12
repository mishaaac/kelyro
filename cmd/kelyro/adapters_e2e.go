//go:build e2e

package main

import (
	"context"
	"errors"

	"github.com/mishaaac/kelyro/internal/storage"
	"github.com/mishaaac/kelyro/internal/update"
)

const e2eSecretValue = "e2e-secret-value-never-render"

// The E2E executable keeps every user-facing layer real while replacing the
// two host integrations that tests must never contact.
type e2eSecretStore struct{}

func (e2eSecretStore) Get(string) (string, error) { return e2eSecretValue, nil }
func (e2eSecretStore) Set(string, string) error   { return nil }
func (e2eSecretStore) Delete(string) error        { return nil }
func (e2eSecretStore) Availability() error        { return nil }
func (e2eSecretStore) Status() ([]storage.SecretStatus, error) {
	return []storage.SecretStatus{{Name: "e2e", Reference: "fake:e2e", Configured: true}}, nil
}

type e2eReleaseProvider struct{}

func (e2eReleaseProvider) Latest(context.Context, update.Channel) (update.Release, bool, error) {
	return update.Release{}, false, errors.New("E2E network adapter was invoked despite offline policy")
}

func newSecretStore() storage.SecretStore        { return e2eSecretStore{} }
func newReleaseProvider() update.ReleaseProvider { return e2eReleaseProvider{} }

var _ storage.SecretStore = e2eSecretStore{}
var _ update.ReleaseProvider = e2eReleaseProvider{}
