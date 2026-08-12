//go:build darwin

package secretstore

import (
	"bytes"
	"fmt"
	"os/exec"
	"strings"

	"github.com/mishaaac/kelyro/internal/storage"
)

type macOSKeychain struct {
	path string
}

func newNativeBackend() (nativeBackend, error) {
	path, err := exec.LookPath("security")
	if err != nil {
		return nil, fmt.Errorf("macOS security command was not found")
	}
	return macOSKeychain{path: path}, nil
}

func (backend macOSKeychain) Get(name string) (string, error) {
	command := exec.Command(backend.path, "find-generic-password", "-s", serviceName, "-a", name, "-w")
	output, err := command.CombinedOutput()
	if err != nil {
		if strings.Contains(string(output), "could not be found") {
			return "", storage.ErrSecretNotFound
		}
		return "", macOSCommandError("read", output, err)
	}
	return trimLineEnding(string(output)), nil
}

func (backend macOSKeychain) Set(name, value string) error {
	// A trailing -w asks security(1) to read the password interactively. Feeding
	// that prompt over stdin keeps the value out of the child process arguments.
	command := exec.Command(backend.path, "add-generic-password", "-U", "-s", serviceName, "-a", name, "-w")
	command.Stdin = bytes.NewBufferString(value + "\n")
	output, err := command.CombinedOutput()
	if err != nil {
		return macOSCommandError("store", output, err)
	}
	return nil
}

func (backend macOSKeychain) Delete(name string) error {
	command := exec.Command(backend.path, "delete-generic-password", "-s", serviceName, "-a", name)
	output, err := command.CombinedOutput()
	if err != nil {
		if strings.Contains(string(output), "could not be found") {
			return storage.ErrSecretNotFound
		}
		return macOSCommandError("delete", output, err)
	}
	return nil
}

func macOSCommandError(operation string, output []byte, err error) error {
	detail := strings.TrimSpace(string(output))
	if detail == "" {
		detail = err.Error()
	}
	return fmt.Errorf("macOS Keychain could not %s the credential: %s", operation, detail)
}

var _ nativeBackend = macOSKeychain{}
