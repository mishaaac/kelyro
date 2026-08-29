// Command quality runs Kelyro's local quality gates without requiring Make or
// platform-specific shell scripts.
package main

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
)

const usage = `Usage: go run ./tools/quality <gate>

Gates:
  test         Run all tests
  e2e          Run isolated Foundation, Student Core, and Research Engine tests
  vet          Run Go static analysis
  race         Run all tests with the race detector
  build-smoke  Build the CLI and run version/help smoke tests
  all          Run every gate in the order shown above
`

type command struct {
	name string
	args []string
}

type executor func(context.Context, string, io.Writer, io.Writer, command) error

func main() {
	os.Exit(run(context.Background(), os.Args[1:], os.Stdout, os.Stderr, execute))
}

func run(ctx context.Context, args []string, stdout, stderr io.Writer, executeCommand executor) int {
	if len(args) != 1 {
		fmt.Fprint(stderr, usage)
		return 2
	}

	root, err := moduleRoot()
	if err != nil {
		fmt.Fprintf(stderr, "quality: %v\n", err)
		return 1
	}

	binary := "kelyro"
	if runtime.GOOS == "windows" {
		binary += ".exe"
	}
	buildDir, err := os.MkdirTemp("", "kelyro-quality-")
	if err != nil {
		fmt.Fprintf(stderr, "quality: create build directory: %v\n", err)
		return 1
	}
	defer os.RemoveAll(buildDir)

	commands, err := plan(args[0], filepath.Join(buildDir, binary))
	if err != nil {
		fmt.Fprintf(stderr, "quality: %v\n\n%s", err, usage)
		return 2
	}
	for _, command := range commands {
		fmt.Fprintf(stdout, "==> %s %s\n", command.name, strings.Join(command.args, " "))
		if err := executeCommand(ctx, root, stdout, stderr, command); err != nil {
			fmt.Fprintf(stderr, "quality: gate %q failed: %v\n", args[0], err)
			return 1
		}
	}

	return 0
}

func plan(gate, binary string) ([]command, error) {
	test := []command{{name: "go", args: []string{"test", "./..."}}}
	e2e := []command{{name: "go", args: []string{"test", "-tags=e2e", "./tests/e2e"}}}
	vet := []command{{name: "go", args: []string{"vet", "./..."}}}
	race := []command{{name: "go", args: []string{"test", "-race", "./..."}}}
	buildSmoke := []command{
		{name: "go", args: []string{"build", "-o", binary, "./cmd/kelyro"}},
		{name: binary, args: []string{"--version"}},
		{name: binary, args: []string{"--help"}},
	}

	switch gate {
	case "test":
		return test, nil
	case "e2e":
		return e2e, nil
	case "vet":
		return vet, nil
	case "race":
		return race, nil
	case "build-smoke":
		return buildSmoke, nil
	case "all":
		commands := append(test, e2e...)
		commands = append(commands, vet...)
		commands = append(commands, race...)
		return append(commands, buildSmoke...), nil
	default:
		return nil, fmt.Errorf("unknown gate %q", gate)
	}
}

func moduleRoot() (string, error) {
	directory, err := os.Getwd()
	if err != nil {
		return "", fmt.Errorf("get working directory: %w", err)
	}
	for {
		_, err := os.Stat(filepath.Join(directory, "go.mod"))
		if err == nil {
			return directory, nil
		}
		if !errors.Is(err, os.ErrNotExist) {
			return "", fmt.Errorf("inspect go.mod: %w", err)
		}
		parent := filepath.Dir(directory)
		if parent == directory {
			return "", errors.New("go.mod not found in current directory or its parents")
		}
		directory = parent
	}
}

func execute(ctx context.Context, root string, stdout, stderr io.Writer, command command) error {
	process := exec.CommandContext(ctx, command.name, command.args...)
	process.Dir = root
	process.Stdout = stdout
	process.Stderr = stderr
	return process.Run()
}
