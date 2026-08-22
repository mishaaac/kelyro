package app

import (
	"context"
	"errors"
	"fmt"
	"path/filepath"
	"strings"

	"github.com/mishaaac/kelyro/internal/artifacts"
	artifactmarkdown "github.com/mishaaac/kelyro/internal/artifacts/markdown"
)

func (service *Service) executeProgressExport(ctx context.Context, command Command) (Result, error) {
	if service.profiles == nil {
		return Result{}, errors.New("student core store is unavailable")
	}
	if service.artifactStores == nil {
		return Result{}, errors.New("workspace artifact store is unavailable")
	}
	found, err := service.discoverWorkspace(command)
	if err != nil {
		return Result{}, err
	}

	learningStore, err := service.profiles.Open(ctx, found.Root)
	if err != nil {
		return Result{}, fmt.Errorf("open student core: %w", err)
	}
	if learningStore.Dashboard() == nil {
		_ = learningStore.Close()
		return Result{}, errors.New("progress dashboard service is unavailable")
	}
	dashboard, dashboardErr := learningStore.Dashboard().Show(ctx)
	if closeErr := learningStore.Close(); closeErr != nil {
		dashboardErr = errors.Join(dashboardErr, fmt.Errorf("close student core: %w", closeErr))
	}
	if dashboardErr != nil {
		return Result{}, dashboardErr
	}

	documents, err := artifactmarkdown.GenerateProgress(dashboard)
	if err != nil {
		return Result{}, fmt.Errorf("render learning progress artifacts: %w", err)
	}
	artifactStore, err := service.artifactStores.Open(ctx, found.Root)
	if err != nil {
		return Result{}, fmt.Errorf("open workspace artifact store: %w", err)
	}

	paths := make([]string, 0, len(documents))
	var writeErr error
	for _, document := range documents {
		_, writeErr = artifactStore.Write(ctx, artifacts.WriteRequest{
			Path: document.Path, Ownership: artifacts.SystemGeneratedHumanReadable,
			CreatedBy: artifactmarkdown.ProgressCreator, Content: document.Content,
			ExpectedVersion: document.TemplateVersion,
		})
		if writeErr != nil {
			writeErr = fmt.Errorf("generate learning progress artifact %s: %w", filepath.ToSlash(document.Path), writeErr)
			break
		}
		paths = append(paths, filepath.ToSlash(document.Path))
	}
	if closeErr := artifactStore.Close(); closeErr != nil {
		writeErr = errors.Join(writeErr, fmt.Errorf("close workspace artifact store: %w", closeErr))
	}
	if writeErr != nil {
		return Result{}, writeErr
	}
	return Result{Message: "Updated learning progress artifacts:\n- " + strings.Join(paths, "\n- ")}, nil
}
