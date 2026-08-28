// Package platformos implements operating-system operations behind the
// platform boundary.
package platformos

import (
	"errors"
	"fmt"
	"net/url"
	"os"
	"os/exec"
	"runtime"
	"strings"

	"github.com/mishaaac/kelyro/internal/platform"
)

type command struct {
	executable string
	args       []string
}

type lookupFunc func(string) (string, error)
type runFunc func(command) error

// Platform provides the native implementation used by incoming adapters.
type Platform struct {
	goos   string
	lookup lookupFunc
	run    runFunc
}

func New() *Platform {
	return &Platform{
		goos:   runtime.GOOS,
		lookup: exec.LookPath,
		run: func(specification command) error {
			return exec.Command(specification.executable, specification.args...).Run()
		},
	}
}

func (service *Platform) Name() string { return service.goos }

func (*Platform) UserHomeDir() (string, error)   { return os.UserHomeDir() }
func (*Platform) UserConfigDir() (string, error) { return os.UserConfigDir() }
func (*Platform) UserCacheDir() (string, error)  { return os.UserCacheDir() }

func (service *Platform) CommandPath(name string) (string, bool) {
	path, err := service.lookup(name)
	return path, err == nil
}

func (service *Platform) OpenPath(path string) error {
	if strings.TrimSpace(path) == "" {
		return errors.New("path to open must not be empty")
	}
	return service.open(path)
}

func (service *Platform) OpenURL(rawURL string) error {
	parsed, err := url.Parse(rawURL)
	if err != nil || parsed.Host == "" || parsed.User != nil || (parsed.Scheme != "http" && parsed.Scheme != "https") {
		return fmt.Errorf("URL to open must be absolute HTTP(S)")
	}
	return service.open(parsed.String())
}

func (service *Platform) open(target string) error {
	name := "xdg-open"
	args := []string{target}
	switch service.goos {
	case "darwin":
		name = "open"
	case "windows":
		name = "rundll32.exe"
		args = []string{"url.dll,FileProtocolHandler", target}
	}
	executable, err := service.lookup(name)
	if err != nil {
		return fmt.Errorf("system opener %q was not found: %w", name, err)
	}
	if err := service.run(command{executable: executable, args: args}); err != nil {
		return fmt.Errorf("open %q with system default: %w", target, err)
	}
	return nil
}

var _ platform.Platform = (*Platform)(nil)
