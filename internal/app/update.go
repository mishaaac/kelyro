package app

import (
	"context"
	"errors"
	"fmt"

	"github.com/mishaaac/kelyro/internal/config"
	"github.com/mishaaac/kelyro/internal/update"
)

var ErrUpdateUnsupported = errors.New("automatic update is unavailable until signed release artifacts and checksum verification are implemented")

func (service *Service) executeUpdate(ctx context.Context, command Command) (Result, error) {
	switch command.UpdateOperation {
	case "install":
		return Result{}, ErrUpdateUnsupported
	case "check":
	default:
		return Result{}, fmt.Errorf("unsupported update operation %q", command.UpdateOperation)
	}
	if service.configs == nil {
		return Result{}, errors.New("configuration store is unavailable")
	}
	settings, err := service.resolvedConfig(command)
	if err != nil {
		return Result{}, err
	}
	enabled, ok := settings[config.KeyUpdateCheck].BoolField()
	if !ok {
		return Result{}, errors.New("updates.check configuration is invalid")
	}
	if !enabled {
		return Result{Message: "Update checks are disabled by updates.check."}, nil
	}
	configuredChannel, ok := settings[config.KeyUpdateChannel].StringField()
	if !ok {
		return Result{}, errors.New("updates.channel configuration is invalid")
	}
	channel := update.Channel(configuredChannel)
	if !channel.Valid() {
		return Result{}, fmt.Errorf("updates.channel configuration %q is invalid", configuredChannel)
	}
	if service.updates == nil {
		return Result{}, errors.New("update checker is unavailable")
	}
	gate, err := service.networkGate(settings, command)
	if err != nil {
		return Result{}, err
	}
	checked, err := service.updates.Check(ctx, channel, gate)
	if err != nil {
		return Result{}, err
	}
	return Result{Update: &checked}, nil
}
