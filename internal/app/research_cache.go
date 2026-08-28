package app

import (
	"context"
	"errors"
	"fmt"
)

func (service *Service) executeResearchCache(ctx context.Context, command Command) (Result, error) {
	if service.researchCaches == nil {
		return Result{}, errors.New("research cache is unavailable")
	}
	found, err := service.discoverWorkspace(command)
	if err != nil {
		return Result{}, err
	}
	cache, err := service.researchCaches.Open(ctx, found.Root)
	if err != nil {
		return Result{}, fmt.Errorf("open research cache: %w", err)
	}
	switch command.ResearchCacheOperation {
	case "status":
		status, statusErr := cache.Status(ctx)
		if statusErr != nil {
			return Result{}, statusErr
		}
		return Result{ResearchCacheStatus: &status}, nil
	case "clear":
		cleared, clearErr := cache.Clear(ctx)
		if clearErr != nil {
			return Result{}, clearErr
		}
		return Result{ResearchCacheCleared: &cleared}, nil
	default:
		return Result{}, fmt.Errorf("unsupported research cache operation %q", command.ResearchCacheOperation)
	}
}
