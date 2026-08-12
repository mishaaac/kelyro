package main

import (
	"context"
	"os"

	"github.com/mishaaac/kelyro/internal/app"
	"github.com/mishaaac/kelyro/internal/cli"
	"github.com/mishaaac/kelyro/internal/doctor"
	"github.com/mishaaac/kelyro/internal/infra/artifactfs"
	"github.com/mishaaac/kelyro/internal/infra/auditsqlite"
	"github.com/mishaaac/kelyro/internal/infra/configfs"
	"github.com/mishaaac/kelyro/internal/infra/doctoros"
	"github.com/mishaaac/kelyro/internal/infra/doctorsqlite"
	"github.com/mishaaac/kelyro/internal/infra/editoros"
	"github.com/mishaaac/kelyro/internal/infra/logfs"
	"github.com/mishaaac/kelyro/internal/infra/secretstore"
	"github.com/mishaaac/kelyro/internal/infra/sessiondb"
	"github.com/mishaaac/kelyro/internal/infra/workspacefs"
	"github.com/mishaaac/kelyro/internal/tui"
	"github.com/mishaaac/kelyro/internal/version"
)

func main() {
	workspaces := workspacefs.New(version.Version)
	service := app.NewService(workspaces, os.Getwd).
		WithConfig(configfs.New()).
		WithSecrets(secretstore.New()).
		WithArtifactStores(artifactfs.NewFactory(version.Version)).
		WithSessionStores(sessiondb.NewFactory(version.Version)).
		WithEditor(editoros.New()).
		WithDoctor(doctor.New(doctoros.New(), doctorsqlite.New(), doctor.DefaultRegistry())).
		WithLogging(logfs.New()).
		WithAudit(auditsqlite.NewFactory(version.Version))
	runner := cli.NewRunner(service, os.Stdout, os.Stderr).
		WithSecretReader(cli.NewTerminalSecretReader(os.Stdin, os.Stderr)).
		WithInteractive(tui.NewRunner(service, os.Stdin, os.Stdout))
	os.Exit(runner.Run(context.Background(), os.Args[1:]))
}
