// Package platform defines the boundary for operating-system-dependent work.
package platform

// Platform exposes the operating-system information and operations needed by
// application services. Implementations belong to infrastructure code.
type Platform interface {
	Name() string
	UserHomeDir() (string, error)
	UserConfigDir() (string, error)
	UserCacheDir() (string, error)
	CommandPath(name string) (string, bool)
	OpenPath(path string) error
	OpenURL(url string) error
}
