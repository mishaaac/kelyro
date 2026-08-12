package cli

import (
	"bufio"
	"errors"
	"fmt"
	"io"
	"os"
	"strings"
)

// SecretReader obtains a sensitive value through a presentation-specific
// channel. Implementations must not echo or log the returned value.
type SecretReader interface {
	ReadSecret(prompt string) (string, error)
}

// TerminalSecretReader reads one line from a terminal while disabling echo.
// Redirected stdin is already non-echoing and remains supported for automation.
type TerminalSecretReader struct {
	input  *os.File
	output io.Writer
}

// NewTerminalSecretReader creates the production secret-input adapter.
func NewTerminalSecretReader(input *os.File, output io.Writer) TerminalSecretReader {
	return TerminalSecretReader{input: input, output: output}
}

func (reader TerminalSecretReader) ReadSecret(prompt string) (string, error) {
	if reader.input == nil || reader.output == nil {
		return "", fmt.Errorf("terminal input is unavailable")
	}
	if _, err := fmt.Fprint(reader.output, prompt); err != nil {
		return "", fmt.Errorf("write secret prompt: %w", err)
	}
	restore, err := disableEcho(reader.input)
	if err != nil {
		return "", err
	}
	encoded, readErr := bufio.NewReader(reader.input).ReadString('\n')
	restoreErr := restore()
	_, newlineErr := fmt.Fprintln(reader.output)
	if readErr != nil && !(errors.Is(readErr, io.EOF) && encoded != "") {
		return "", fmt.Errorf("read terminal input: %w", readErr)
	}
	if restoreErr != nil {
		return "", fmt.Errorf("restore terminal echo: %w", restoreErr)
	}
	if newlineErr != nil {
		return "", fmt.Errorf("finish secret prompt: %w", newlineErr)
	}
	encoded = strings.TrimSuffix(encoded, "\n")
	encoded = strings.TrimSuffix(encoded, "\r")
	return encoded, nil
}

var _ SecretReader = TerminalSecretReader{}
