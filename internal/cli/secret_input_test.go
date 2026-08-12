package cli

import (
	"bytes"
	"os"
	"strings"
	"testing"
)

func TestTerminalSecretReaderSupportsRedirectedInputWithoutEchoing(t *testing.T) {
	secret := "redirected-sensitive-value"
	input, err := os.CreateTemp(t.TempDir(), "secret-input-*")
	if err != nil {
		t.Fatalf("CreateTemp() error = %v", err)
	}
	defer input.Close()
	if _, err := input.WriteString(secret + "\n"); err != nil {
		t.Fatalf("WriteString() error = %v", err)
	}
	if _, err := input.Seek(0, 0); err != nil {
		t.Fatalf("Seek() error = %v", err)
	}

	var output bytes.Buffer
	reader := NewTerminalSecretReader(input, &output)
	value, err := reader.ReadSecret("Secret value: ")
	if err != nil {
		t.Fatalf("ReadSecret() error = %v", err)
	}
	if value != secret {
		t.Fatalf("ReadSecret() = %q", value)
	}
	if strings.Contains(output.String(), secret) {
		t.Fatal("terminal reader echoed secret")
	}
	if output.String() != "Secret value: \n" {
		t.Fatalf("prompt output = %q", output.String())
	}
}
