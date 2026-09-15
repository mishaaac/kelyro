package main

import (
	"context"
	"fmt"
	"os"
	"path/filepath"

	"github.com/mishaaac/kelyro/internal/infra/referencepack"
)

func main() {
	if len(os.Args) != 2 {
		fmt.Fprintln(os.Stderr, "usage: go run ./internal/infra/referencepack/cmd <output.zip>")
		os.Exit(2)
	}
	result, err := referencepack.BuildBackendGo(context.Background())
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	target := filepath.Clean(os.Args[1])
	if err := os.MkdirAll(filepath.Dir(target), 0o755); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	if err := os.WriteFile(target, result.PortableArchive, 0o644); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	fmt.Printf("wrote %s (%s)\n", target, result.ContentHash)
}
