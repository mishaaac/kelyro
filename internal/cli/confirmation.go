package cli

import (
	"bufio"
	"fmt"
	"io"
	"strings"
)

type textConfirmer struct {
	input  *bufio.Reader
	output io.Writer
}

// NewTextConfirmer creates a conservative y/yes confirmation prompt.
func NewTextConfirmer(input io.Reader, output io.Writer) Confirmer {
	return &textConfirmer{input: bufio.NewReader(input), output: output}
}

func (confirmer *textConfirmer) Confirm(prompt string) (bool, error) {
	if _, err := fmt.Fprint(confirmer.output, prompt); err != nil {
		return false, err
	}
	answer, err := confirmer.input.ReadString('\n')
	if err != nil && len(answer) == 0 {
		return false, err
	}
	answer = strings.ToLower(strings.TrimSpace(answer))
	return answer == "y" || answer == "yes", nil
}
