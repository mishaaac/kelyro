// Package config defines configuration values and persistence contracts.
package config

// Settings is the format-independent representation of Kelyro configuration.
type Settings map[string]string

// Store persists global and project-scoped settings. Project methods receive a
// workspace root; resolving platform-specific locations remains an adapter
// concern.
type Store interface {
	LoadGlobal() (Settings, error)
	LoadProject(root string) (Settings, error)
	SaveGlobal(settings Settings) error
	SaveProject(root string, settings Settings) error
}
