//go:build !e2e

package main

import (
	"github.com/mishaaac/kelyro/internal/infra/secretstore"
	"github.com/mishaaac/kelyro/internal/infra/updategithub"
	"github.com/mishaaac/kelyro/internal/storage"
	"github.com/mishaaac/kelyro/internal/update"
	"github.com/mishaaac/kelyro/internal/version"
)

func newSecretStore() storage.SecretStore {
	return secretstore.New()
}

func newReleaseProvider() update.ReleaseProvider {
	return updategithub.New(version.Version)
}
