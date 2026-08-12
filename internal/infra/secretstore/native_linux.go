//go:build linux

package secretstore

import (
	"bytes"
	"fmt"
	"os/exec"
	"strings"

	"github.com/mishaaac/kelyro/internal/storage"
)

type linuxSecretService struct {
	path string
}

func newNativeBackend() (nativeBackend, error) {
	path, err := exec.LookPath("secret-tool")
	if err != nil {
		return nil, fmt.Errorf("Secret Service client secret-tool was not found (install libsecret tools in a graphical session)")
	}
	return linuxSecretService{path: path}, nil
}

func (backend linuxSecretService) Get(name string) (string, error) {
	command := exec.Command(backend.path, "lookup", "service", serviceName, "account", name)
	output, err := command.CombinedOutput()
	if err != nil {
		return "", commandError("read", output, err)
	}
	value := trimLineEnding(string(output))
	if value == "" {
		return "", storage.ErrSecretNotFound
	}
	return value, nil
}

func (backend linuxSecretService) Set(name, value string) error {
	command := exec.Command(backend.path, "store", "--label=Kelyro secret "+name, "service", serviceName, "account", name)
	command.Stdin = bytes.NewBufferString(value + "\n")
	output, err := command.CombinedOutput()
	if err != nil {
		return commandError("store", output, err)
	}
	return nil
}

func (backend linuxSecretService) Delete(name string) error {
	command := exec.Command(backend.path, "clear", "service", serviceName, "account", name)
	output, err := command.CombinedOutput()
	if err != nil {
		return commandError("delete", output, err)
	}
	return nil
}

func commandError(operation string, output []byte, err error) error {
	detail := strings.TrimSpace(string(output))
	if detail == "" {
		detail = err.Error()
	}
	return fmt.Errorf("Linux Secret Service could not %s the credential: %s (is a Secret Service session running?)", operation, detail)
}

var _ nativeBackend = linuxSecretService{}
