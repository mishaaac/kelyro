package main

import (
	"context"
	"os"

	"github.com/mishaaac/kelyro/internal/app"
	"github.com/mishaaac/kelyro/internal/cli"
	"github.com/mishaaac/kelyro/internal/infra/workspacefs"
	"github.com/mishaaac/kelyro/internal/version"
)

func main() {
	workspaces := workspacefs.New(version.Version)
	service := app.NewService(workspaces, os.Getwd)
	runner := cli.NewRunner(service, os.Stdout, os.Stderr)
	os.Exit(runner.Run(context.Background(), os.Args[1:]))
}
