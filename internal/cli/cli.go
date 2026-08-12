// Package cli provides the minimal command-line entry point for Kelyro.
package cli

import (
	"fmt"
	"io"

	"github.com/mishaaac/kelyro/internal/version"
)

const help = `Kelyro is a local-first learning workspace.

Usage:
  kelyro --help
  kelyro --version

Options:
  -h, --help  Show this help message
      --version  Show build version information
`

// Run executes the bootstrap command-line interface and returns a process exit
// code. The complete command router is intentionally deferred to a later step.
func Run(args []string, stdout, stderr io.Writer) int {
	if len(args) == 0 {
		fmt.Fprint(stdout, help)
		return 0
	}

	if len(args) == 1 {
		switch args[0] {
		case "-h", "--help":
			fmt.Fprint(stdout, help)
			return 0
		case "--version":
			fmt.Fprintf(stdout, "kelyro %s\n", version.Current())
			return 0
		}
	}

	fmt.Fprintf(stderr, "kelyro: unsupported arguments: %v\n", args)
	fmt.Fprintln(stderr, "Run 'kelyro --help' for usage.")
	return 2
}
