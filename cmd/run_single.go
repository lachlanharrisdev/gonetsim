package cmd

import (
	"context"
	"log/slog"

	appconfig "github.com/lachlanharrisdev/gonetsim/internal/config"
	"github.com/lachlanharrisdev/gonetsim/internal/observability"
	"github.com/lachlanharrisdev/gonetsim/internal/service"
	"github.com/lachlanharrisdev/gonetsim/internal/utils"
	"github.com/spf13/cobra"
)

type ServiceFactory func(cfg appconfig.Config, logger *slog.Logger) (service.Service, error)

func runSingleServiceCommand(cmd *cobra.Command, overrides []flagOverride, factory ServiceFactory) error {
	cfgRes, err := loadConfigWithFlagOverrides(cmd, overrides)
	if err != nil {
		return err
	}
	cfg := cfgRes.Config
	if err := cfg.Validate(); err != nil {
		return err
	}

	ctx, stop := utils.SignalContext(context.Background())
	defer stop()

	logger, err := observability.NewLogger(cfg.Logging)
	if err != nil {
		return err
	}
	slog.SetDefault(logger)
	if cfgRes.Created {
		logger.Info("config created", "path", cfgRes.Path)
	}
	logger.Info("config loaded", "path", cfgRes.Path)

	svc, err := factory(cfg, logger)
	if err != nil {
		return err
	}

	manager := service.NewManager(cfg.General.ShutdownTimeout, logger)
	return manager.RunSingleService(ctx, svc)
}
